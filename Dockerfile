# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS builder

WORKDIR /build

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download

# Transfer source code
COPY templates ./templates
COPY ytdlp ./ytdlp
COPY *.go ./

# Build
RUN CGO_ENABLED=0 go build -trimpath -o /dist/fixyoutube-go

# Test
FROM build-stage AS run-test-stage
RUN go test -v ./...

FROM alpine:3 AS build-release-stage

RUN apk add --no-cache yt-dlp

WORKDIR /app

COPY --from=builder /dist .

ENTRYPOINT ["/app/fixyoutube-go"]
