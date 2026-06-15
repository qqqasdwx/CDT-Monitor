# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS build

ARG VERSION=dev
ARG TARGETOS=linux
ARG TARGETARCH=amd64

WORKDIR /src

COPY backend/go.mod ./go.mod
RUN go mod download

COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/cdt-monitor ./cmd/server

FROM alpine:3.21

RUN addgroup -S app && adduser -S -G app app

COPY --from=build /out/cdt-monitor /usr/local/bin/cdt-monitor

USER app
EXPOSE 8080
ENV PORT=8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null || exit 1

ENTRYPOINT ["cdt-monitor"]
