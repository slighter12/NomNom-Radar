# Multi-stage build - Optimize image size and build independently
FROM golang:alpine AS base-builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Copy dependency files first to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download

# =============================================================================
# Radar Builder
# =============================================================================
FROM base-builder AS radar-builder

# Copy only radar source code
COPY ./cmd/radar ./cmd/radar
COPY ./internal ./internal
COPY ./config ./config

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
COPY ./internal ./internal
COPY ./config ./config

# Build geoworker
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o geoworker ./cmd/geoworker

# =============================================================================
# Runtime stage for radar (main API server)
# =============================================================================
FROM gcr.io/distroless/static-debian11:nonroot AS radar

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
FROM gcr.io/distroless/static-debian11:nonroot AS geoworker

COPY --from=geoworker-builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=geoworker-builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=geoworker-builder /app/geoworker /app/geoworker
COPY --from=geoworker-builder /app/config/config_demo.yaml /app/config/config.yaml

WORKDIR /app
EXPOSE 8081

ENTRYPOINT ["/app/geoworker"]
