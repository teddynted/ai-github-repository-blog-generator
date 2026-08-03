# Content runner image for the GenerateContent Fargate task (arm64).
# Build context is the repo root:
#   docker build -f containers/content-runner/Dockerfile -t <ecr>:<tag> .
# syntax=docker/dockerfile:1

FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -o /out/content-runner ./cmd/content-runner

FROM debian:bookworm-slim
# The runner reads repos over the GitHub API and calls Bedrock/Anthropic + S3, so
# it needs no git or media tooling — only ca-certificates for the SDK's TLS calls.
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates \
  && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/content-runner /usr/local/bin/content-runner
ENTRYPOINT ["/usr/local/bin/content-runner"]
