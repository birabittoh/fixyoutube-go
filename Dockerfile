# syntax=docker/dockerfile:1

FROM golang:1.23-alpine AS builder

RUN apk add --no-cache build-base

WORKDIR /build

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download

# Transfer source code
COPY templates ./templates
COPY invidious ./invidious
COPY *.go ./

# Build
RUN CGO_ENABLED=0 go build -trimpath -o /dist/app

# Test
FROM build-stage AS run-test-stage
RUN go test -v ./...

FROM scratch AS build-release-stage

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /dist /

ENTRYPOINT ["/app"]
