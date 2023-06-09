# Build stage
FROM golang:1.18.5-alpine as builder

RUN apk add --no-cache gcc musl-dev libc6-compat build-base libc-dev git
WORKDIR /workspace
COPY go.mod go.mod
COPY go.sum go.sum
COPY Dockerfile Dockerfile

# Copy the go source
COPY cmd/ cmd/
COPY pkg/ pkg/
COPY vendor/ vendor/

ARG storage="tikv"

RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags "-linkmode external -extldflags -static -X github.com/kubewharf/kubebrain/cmd/version.Storage=$storage" -o kube-brain ./cmd/main.go


# Final image creation
FROM alpine:latest

COPY --from=builder /workspace/kube-brain .

ENTRYPOINT ["/kube-brain"]