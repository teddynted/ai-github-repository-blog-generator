# Content runner image for the GenerateContent Fargate task (arm64).
# Build context is the repo root:
#   docker build -f containers/content-runner/Dockerfile -t <ecr>:<tag> .
# syntax=docker/dockerfile:1

# Build on the builder's native platform and cross-compile to the target arch
# (Go cross-compiles trivially), so the compile is NOT emulated under QEMU on an
# amd64 runner. $BUILDPLATFORM/$TARGETARCH are buildx-provided build args.
FROM --platform=$BUILDPLATFORM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -trimpath -o /out/content-runner ./cmd/content-runner

FROM debian:bookworm-slim
# The runner reads repos over the GitHub API and calls Bedrock/Anthropic + S3, so
# it needs no git or media tooling — only ca-certificates for the SDK's TLS calls.
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates \
  && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/content-runner /usr/local/bin/content-runner
ENTRYPOINT ["/usr/local/bin/content-runner"]
