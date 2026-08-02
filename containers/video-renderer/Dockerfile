# Video renderer image for the GenerateSocialVideos Fargate task (arm64).
# Build context is the repo root:
#   docker build -f containers/video-renderer/Dockerfile -t <ecr>:<tag> .
# syntax=docker/dockerfile:1

FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -o /out/video-renderer ./cmd/video-renderer

FROM debian:bookworm-slim
# FFmpeg does the encoding; fonts-dejavu-core provides the drawtext font;
# librsvg2-bin (rsvg-convert) rasterizes the architecture-diagram.svg background;
# ca-certificates lets the AWS SDK reach S3/Polly over TLS.
RUN apt-get update && apt-get install -y --no-install-recommends \
      ffmpeg fonts-dejavu-core librsvg2-bin ca-certificates \
  && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/video-renderer /usr/local/bin/video-renderer
ENTRYPOINT ["/usr/local/bin/video-renderer"]
