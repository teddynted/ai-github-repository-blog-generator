# Video renderer image for the GenerateSocialVideos Fargate task (arm64).
# Build context is the repo root:
#   docker build -f containers/video-renderer/Dockerfile -t <ecr>:<tag> .
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
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -trimpath -o /out/video-renderer ./cmd/video-renderer

FROM debian:bookworm-slim
# FFmpeg does the encoding; fonts-dejavu-core provides the drawtext font;
# ca-certificates lets the AWS SDK reach S3/Polly over TLS.
RUN apt-get update && apt-get install -y --no-install-recommends \
      ffmpeg fonts-dejavu-core ca-certificates \
  && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/video-renderer /usr/local/bin/video-renderer
ENTRYPOINT ["/usr/local/bin/video-renderer"]
