# syntax=docker/dockerfile:1

FROM golang:alpine AS builder

RUN apk add --no-cache build-base

WORKDIR /build

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download

# Transfer source code
COPY templates ./templates
COPY invidious ./invidious
COPY volatile ./volatile
COPY *.go ./

# Build
RUN CGO_ENABLED=1 go build -ldflags='-s -w' -trimpath -o /dist/app
RUN ldd /dist/app | tr -s [:blank:] '\n' | grep ^/ | xargs -I % install -D % /dist/%
RUN ln -s ld-musl-x86_64.so.1 /dist/lib/libc.musl-x86_64.so.1

# Test
FROM build-stage AS run-test-stage
RUN go test -v ./...

FROM scratch AS build-release-stage

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /dist /

ENTRYPOINT ["/app"]
