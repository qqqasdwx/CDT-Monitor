# syntax=docker/dockerfile:1

FROM node:24-alpine AS frontend-build
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.24-alpine AS backend-build
RUN apk add --no-cache build-base
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/cdt-monitor ./cmd/server

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S cdt-monitor \
    && adduser -S -G cdt-monitor cdt-monitor \
    && mkdir -p /app/data \
    && chown -R cdt-monitor:cdt-monitor /app
WORKDIR /app
COPY --from=backend-build /out/cdt-monitor /app/cdt-monitor
COPY --from=frontend-build /src/frontend/dist /app/frontend

ENV CDTM_PORT=8080 \
    CDTM_DATABASE_PATH=/app/data/cdt-monitor.sqlite \
    CDTM_SECRET_KEY_PATH=/app/data/secret.key \
    CDTM_SESSION_KEY_PATH=/app/data/session.key \
    CDTM_FRONTEND_DIR=/app/frontend

USER cdt-monitor
EXPOSE 8080
VOLUME ["/app/data"]
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/app/cdt-monitor"]
