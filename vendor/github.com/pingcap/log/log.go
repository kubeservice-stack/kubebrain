// Copyright 2019 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package log

import (
	"errors"
	"os"
	"sync"
	"sync/atomic"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

var globalMu sync.Mutex
var globalLogger, globalProperties, globalSugarLogger atomic.Value

var registerOnce sync.Once

func init() {
	conf := &Config{Level: "info", File: FileLogConfig{}}
	logger, props, _ := InitLogger(conf)
	ReplaceGlobals(logger, props)
}

// InitLogger initializes a zap logger.
func InitLogger(cfg *Config, opts ...zap.Option) (*zap.Logger, *ZapProperties, error) {
	var output zapcore.WriteSyncer
	var errOutput zapcore.WriteSyncer
	if len(cfg.File.Filename) > 0 {
		lg, err := initFileLog(&cfg.File)
		if err != nil {
			return nil, nil, err
		}
		output = zapcore.AddSync(lg)
	} else {
		stdOut, _, err := zap.Open([]string{"stdout"}...)
		if err != nil {
			return nil, nil, err
		}
		output = stdOut
	}
	if len(cfg.ErrorOutputPath) > 0 {
		errOut, _, err := zap.Open([]string{cfg.ErrorOutputPath}...)
		if err != nil {
			return nil, nil, err
		}
		errOutput = errOut
	} else {
		errOutput = output
	}

	return InitLoggerWithWriteSyncer(cfg, output, errOutput, opts...)
}

func InitTestLogger(t zaptest.TestingT, cfg *Config, opts ...zap.Option) (*zap.Logger, *ZapProperties, error) {
	writer := newTestingWriter(t)
	zapOptions := []zap.Option{
		// Send zap errors to the same writer and mark the test as failed if
		// that happens.
		zap.ErrorOutput(writer.WithMarkFailed(true)),
	}
	opts = append(zapOptions, opts...)
	return InitLoggerWithWriteSyncer(cfg, writer, writer, opts...)
}

// InitLoggerWithWriteSyncer initializes a zap logger with specified write syncer.
func InitLoggerWithWriteSyncer(cfg *Config, output, errOutput zapcore.WriteSyncer, opts ...zap.Option) (*zap.Logger, *ZapProperties, error) {
	level := zap.NewAtomicLevel()
	err := level.UnmarshalText([]byte(cfg.Level))
	if err != nil {
		return nil, nil, err
	}
	encoder := NewTextEncoder(cfg)
	registerOnce.Do(func() {
		err = zap.RegisterEncoder(ZapEncodingName, func(zapcore.EncoderConfig) (zapcore.Encoder, error) {
			return encoder, nil
		})
	})
	if err != nil {
		return nil, nil, err
	}
	core := NewTextCore(encoder, output, level)
	opts = append(cfg.buildOptions(errOutput), opts...)
	lg := zap.New(core, opts...)
	r := &ZapProperties{
		Core:   core,
		Syncer: output,
		Level:  level,
	}
	return lg, r, nil
}

// initFileLog initializes file based logging options.
func initFileLog(cfg *FileLogConfig) (*lumberjack.Logger, error) {
	if st, err := os.Stat(cfg.Filename); err == nil {
		if st.IsDir() {
			return nil, errors.New("can't use directory as log file name")
		}
	}
	if cfg.MaxSize == 0 {
		cfg.MaxSize = defaultLogMaxSize
	}

	// use lumberjack to logrotate
	return &lumberjack.Logger{
		Filename:   cfg.Filename,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxDays,
		LocalTime:  true,
	}, nil
}

// L returns the global Logger, which can be reconfigured with ReplaceGlobals.
// It's safe for concurrent use.
func L() *zap.Logger {
	return globalLogger.Load().(*zap.Logger)
}

// S returns the global SugaredLogger, which can be reconfigured with
// ReplaceGlobals. It's safe for concurrent use.
func S() *zap.SugaredLogger {
	return globalSugarLogger.Load().(*zap.SugaredLogger)
}

// ReplaceGlobals replaces the global Logger and SugaredLogger, and returns a
// function to restore the original values. It's safe for concurrent use.
func ReplaceGlobals(logger *zap.Logger, props *ZapProperties) func() {
	// TODO: This globalMu can be replaced by atomic.Swap(), available since go1.17.
	globalMu.Lock()
	prevLogger := globalLogger.Load()
	prevProps := globalProperties.Load()
	globalLogger.Store(logger)
	globalSugarLogger.Store(logger.Sugar())
	globalProperties.Store(props)
	globalMu.Unlock()

	if prevLogger == nil || prevProps == nil {
		// When `ReplaceGlobals` is called first time, atomic.Value is empty.
		return func() {}
	}
	return func() {
		ReplaceGlobals(prevLogger.(*zap.Logger), prevProps.(*ZapProperties))
	}
}

// Sync flushes any buffered log entries.
func Sync() error {
	err := L().Sync()
	if err != nil {
		return err
	}
	return S().Sync()
}
