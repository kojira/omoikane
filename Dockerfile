# syntax=docker/dockerfile:1.7
#
# omoikane container image.
#
# Multi-stage: build with the official Go alpine image (cgo + sqlite
# dev headers needed for the sqlite_fts5 build tag), then drop into a
# small alpine runtime. Final image ~25-30MB.
#
# Configuration is via env vars (see internal/config/config.go for the
# full list). The /data volume is the only stateful surface — bind
# mount or named-volume it in docker-compose.
#
# Build:
#   docker build -t omoikane .
# Run (example):
#   docker run --rm -p 8080:8080 -v $PWD/data:/data \
#     -e KB_OAUTH_GOOGLE_CLIENT_ID=... \
#     -e KB_OAUTH_GOOGLE_CLIENT_SECRET=... \
#     -e KB_OAUTH_REDIRECT_BASE=https://kb.example.com \
#     -e KB_AUTH_ALLOW_EMAILS=you@example.com \
#     omoikane

FROM golang:1.26-alpine AS build
RUN apk add --no-cache build-base sqlite-dev
WORKDIR /src
# Cache module downloads in a separate layer so source-only changes
# don't bust the dep cache.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# sqlite_fts5 tag is mandatory — without it, FTS index creation in
# migration 002 errors at startup ("no such module: fts5").
RUN CGO_ENABLED=1 go build -tags sqlite_fts5 -ldflags='-s -w' \
    -trimpath -o /out/kb-server ./cmd/kb-server

FROM alpine:3.20
RUN apk add --no-cache sqlite ca-certificates tzdata wget && \
    addgroup -S kb && adduser -S kb -G kb && \
    mkdir -p /data && chown kb:kb /data
COPY --from=build /out/kb-server /usr/local/bin/kb-server
USER kb
VOLUME ["/data"]
ENV KB_DB_PATH=/data/kb.db
ENV KB_HTTP_ADDR=:8080
EXPOSE 8080
# wget is used by docker-compose healthcheck; remove if you don't need it.
ENTRYPOINT ["/usr/local/bin/kb-server"]
