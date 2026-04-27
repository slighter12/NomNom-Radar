# Multi-stage build - Optimize image size and build independently
FROM golang:1.26-alpine AS base-builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Copy dependency files first to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download

# Copy shared packages (improves cache: cmd changes don't invalidate other builds)
COPY ./internal ./internal
COPY ./config ./config

# =============================================================================
# Radar Builder
# =============================================================================
FROM base-builder AS radar-builder

# Copy only radar source code
COPY ./cmd/radar ./cmd/radar

# Build radar
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o radar ./cmd/radar

# =============================================================================
# Geoworker Builder
# =============================================================================
FROM base-builder AS geoworker-builder

# Copy only geoworker source code
COPY ./cmd/geoworker ./cmd/geoworker

# Build geoworker
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o geoworker ./cmd/geoworker

# =============================================================================
# Device Cleanup Builder
# =============================================================================
FROM base-builder AS device-cleanup-builder

# Copy only device cleanup source code
COPY ./cmd/device-cleanup ./cmd/device-cleanup

# Build device cleanup job
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o device-cleanup ./cmd/device-cleanup

# =============================================================================
# Runtime stage for radar (main API server)
# =============================================================================
FROM gcr.io/distroless/static-debian13:nonroot AS radar

COPY --from=radar-builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=radar-builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=radar-builder /app/radar /app/radar
COPY --from=radar-builder /app/config/config_demo.yaml /app/config/config.yaml

WORKDIR /app
EXPOSE 8080

ENTRYPOINT ["/app/radar"]

# =============================================================================
# Runtime stage for geoworker
# =============================================================================
FROM gcr.io/distroless/static-debian13:nonroot AS geoworker

COPY --from=geoworker-builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=geoworker-builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=geoworker-builder /app/geoworker /app/geoworker
COPY --from=geoworker-builder /app/config/config_demo.yaml /app/config/config.yaml

WORKDIR /app
EXPOSE 8080

ENTRYPOINT ["/app/geoworker"]

# =============================================================================
# Runtime stage for device cleanup Cloud Run Job
# =============================================================================
FROM gcr.io/distroless/static-debian13:nonroot AS device-cleanup

COPY --from=device-cleanup-builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=device-cleanup-builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=device-cleanup-builder /app/device-cleanup /app/device-cleanup
COPY --from=device-cleanup-builder /app/config/config_demo.yaml /app/config/config.yaml

WORKDIR /app

ENTRYPOINT ["/app/device-cleanup"]
