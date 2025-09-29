# Multi-stage build - Optimize image size
FROM golang:alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Copy dependency files first to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Static compilation to reduce dependencies
# The flags -a and -installsuffix cgo are removed as they are unnecessary with CGO_ENABLED=0
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o main ./cmd/radar

# Runtime stage - Use distroless image
FROM gcr.io/distroless/static-debian11:nonroot AS deployment

# Copy necessary files
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/main /app/main

WORKDIR /app
EXPOSE 8080

ENTRYPOINT ["/app/main"]
