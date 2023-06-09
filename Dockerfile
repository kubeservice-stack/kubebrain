# Build stage
FROM golang:1.18.5-alpine as builder

RUN apk add --no-cache gcc musl-dev libc6-compat build-base libc-dev
WORKDIR /workspace
COPY go.mod go.mod
COPY go.sum go.sum

# Copy the go source
COPY cmd/ cmd/
COPY pkg/ pkg/
COPY vendor/ vendor/

ARG storage="tikv"

RUN export version=$(git describe --abbrev=0 --tags || git rev-parse --abbrev-ref HEAD) && \
    export sha=$(git rev-parse --short HEAD) && \
    export go_version=$(go env GOVERSION) && \
    export go_os=$(go env GOOS) && \
    export go_arch=$(go env GOARCH) && \
    export go_os_arch="$go_os/$go_arch" && \
    CGO_ENABLED=0 GOOS=linux go build -a -ldflags "-linkmode external -extldflags -static -X github.com/kubewharf/kubebrain/cmd/version.Version=$version -X github.com/kubewharf/kubebrain/cmd/version.Storage=$storage -X github.com/kubewharf/kubebrain/cmd/version.GoOsArch=$go_os_arch -X github.com/kubewharf/kubebrain/cmd/version.GoVersion=$go_version -X github.com/kubewharf/kubebrain/cmd/version.GitSHA=$sha -X "github.com/kubewharf/kubebrain/cmd/version".Date=(date "+%Y-%m-%d-%H:%M:%S")" -o kube-brain ./cmd/main.go


# Final image creation
FROM alpine:latest

COPY --from=builder /workspace/kube-brain .

ENTRYPOINT ["/kube-brain"]