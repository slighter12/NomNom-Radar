# Serverless Geospatial Notification System

## Objective

Refactor the existing synchronous notification system to an asynchronous Pub/Sub architecture, and replace the in-memory CH routing engine with PMTiles + MVT parsing.

---

## Architecture Overview

```text
┌─────────────────┐     ┌─────────────┐     ┌─────────────────┐
│   Service A     │     │   Pub/Sub   │     │   Service B     │
│   (Event API)   │────▶│   (Queue)   │────▶│   (Geo Worker)  │
│   cmd/radar     │     │             │     │   cmd/geoworker │
└─────────────────┘     └─────────────┘     └─────────────────┘
        │                                           │
        ▼                                           ▼
   ┌─────────┐                              ┌─────────────┐
   │ Supabase│                              │  PMTiles    │
   │   (DB)  │                              │  (GCS/Local)│
   └─────────┘                              └─────────────┘
```

**Service A (Event API)**: Receive requests → Write to DB → Filter subscribers → Publish events to Pub/Sub

**Service B (Geo Worker)**: Receive Pub/Sub → Read PMTiles → Calculate road network distance → Send FCM notifications

---

## Implementation Steps

### Phase 1: EventPublisher Infrastructure

#### 1.1 EventPublisher Interface

**File**: `internal/domain/service/event_publisher.go`

```go
type NotificationEvent struct {
    NotificationID string   `json:"notification_id"`
    MerchantID     string   `json:"merchant_id"`
    Latitude       float64  `json:"latitude"`
    Longitude      float64  `json:"longitude"`
    LocationName   string   `json:"location_name"`
    FullAddress    string   `json:"full_address"`
    HintMessage    string   `json:"hint_message,omitempty"`
    SubscriberIDs  []string `json:"subscriber_ids"` // Pre-filtered subscribers
}

type EventPublisher interface {
    PublishNotificationEvent(ctx context.Context, event *NotificationEvent) error
    Close() error
}
```

#### 1.2 LocalHttpPublisher (Local Development)

**File**: `internal/infra/pubsub/local_http_publisher.go`

- Direct HTTP POST to `http://localhost:8081/push`
- Simulates Pub/Sub push behavior
- Uses Base64 encoding for message data, compatible with Google Pub/Sub format

#### 1.3 GooglePubSubPublisher (Production)

**File**: `internal/infra/pubsub/google_publisher.go`

- Uses `cloud.google.com/go/pubsub`
- Supports graceful shutdown
- Automatically validates Topic existence

#### 1.4 Configuration Structure

**Modified**: `config/config.go`

```go
type PubSubConfig struct {
    Provider      string `json:"provider" yaml:"provider"`      // "local" or "google"
    ProjectID     string `json:"projectId" yaml:"projectId"`
    TopicID       string `json:"topicId" yaml:"topicId"`
    LocalEndpoint string `json:"localEndpoint" yaml:"localEndpoint"`
}
```

---

### Phase 2: PMTiles Routing Service

#### 2.1 PMTiles Routing Implementation

**File**: `internal/infra/routing/pmtiles/service.go`

Implements `RoutingUsecase` interface:

```go
type pmtilesRoutingService struct {
    source     string           // PMTiles URL (local or HTTP)
    roadLayer  string           // Road layer name in MVT
    zoomLevel  int              // Zoom level for queries
    logger     *slog.Logger
    httpClient *http.Client
    parser     *MVTParser
    tileCache  map[string]*RoadGraph
}

func (s *pmtilesRoutingService) OneToMany(ctx context.Context, source Coordinate, targets []Coordinate) (*OneToManyResult, error) {
    // 1. Calculate required tile range
    // 2. Fetch tiles via HTTP
    // 3. Parse MVT to extract road LineStrings
    // 4. Build local road network graph
    // 5. Calculate shortest path distance for each target
}
```

#### 2.2 MVT Parsing Logic

**File**: `internal/infra/routing/pmtiles/mvt_parser.go`

- Uses `github.com/paulmach/orb/encoding/mvt` to parse MVT
- Supports gzip-compressed MVT data
- Builds nodes and edges from road LineStrings
- Automatically projects to WGS84 coordinate system

#### 2.3 Local Pathfinding

**File**: `internal/infra/routing/pmtiles/pathfinder.go`

- Implements Dijkstra's shortest path algorithm
- Supports One-to-Many batch queries
- Uses Priority Queue for performance optimization
- Automatically falls back to Haversine if path exceeds tile boundaries

#### 2.4 Configuration Structure

**Modified**: `config/config.go`

```go
type PMTilesConfig struct {
    Enabled   bool   `json:"enabled" yaml:"enabled"`
    Source    string `json:"source" yaml:"source"`       // "http://localhost:8080/map.pmtiles" or tile server URL
    RoadLayer string `json:"roadLayer" yaml:"roadLayer"` // MVT layer name
    ZoomLevel int    `json:"zoomLevel" yaml:"zoomLevel"` // Zoom level for queries
}
```

---

### Phase 3: Geo Worker Service

#### 3.1 Worker Entry Point

**File**: `cmd/geoworker/main.go`

- Uses FX dependency injection
- Starts HTTP Server listening on `/push` endpoint
- Injects PMTiles routing service and Firebase service

#### 3.2 Worker Handler

**File**: `internal/delivery/pubsub/worker.go`

```go
func (w *Worker) HandlePush(c echo.Context) error {
    // 1. Parse Pub/Sub message format
    // 2. Extract NotificationEvent
    // 3. Calculate road network distance for each subscriber
    // 4. Filter: distance <= subscription radius
    // 5. Get FCM tokens for valid subscribers
    // 6. Send Firebase notifications in batches
    // 7. Return 200 OK to confirm processing
}
```

---

### Phase 4: Modify Existing Notification Flow

#### 4.1 Modify NotificationService

**Modified**: `internal/usecase/impl/notification_service.go`

```go
func (s *notificationService) PublishLocationNotification(...) (*entity.MerchantLocationNotification, error) {
    // 1. Validate input
    // 2. Parse location information
    // 3. Create notification record (write to DB)

    // 4. Check if EventPublisher is available (async mode)
    if s.eventPublisher != nil {
        // 4a. Pre-filter subscribers using PostGIS (straight-line distance)
        subscribers := s.subscriptionRepo.FindSubscriberAddressesWithinRadius(...)

        // 4b. Publish event to Pub/Sub (async processing of road distance + sending notifications)
        event := &service.NotificationEvent{...}
        s.eventPublisher.PublishNotificationEvent(ctx, event)

        // 4c. Return immediately (don't wait for notification delivery)
        return notification, nil
    }

    // 5. Fallback: Synchronous processing (original logic)
    return s.publishSync(ctx, notification, ...)
}
```

---

### Phase 5: Local Development Setup

#### 5.1 Docker Compose Service Addition

**Modified**: `docker-compose.yml`

```yaml
services:
  # Local PMTiles server
  pmtiles-server:
    image: python:3.12-slim
    container_name: radar-pmtiles-server
    ports:
      - "8080:8080"
    volumes:
      - ./data/pmtiles:/data
    command: python -m http.server 8080 --directory /data
    networks:
      - radar-network
    profiles:
      - dev

  # Geo Worker
  geoworker:
    build:
      context: .
      dockerfile: Dockerfile
      target: geoworker
    container_name: radar-geoworker
    ports:
      - "8081:8081"
    environment:
      - PUBSUB_PROVIDER=local
      - PMTILES_SOURCE=http://pmtiles-server:8080/map.pmtiles
      - PMTILES_ENABLED=true
    depends_on:
      postgres-master:
        condition: service_healthy
    networks:
      - radar-network
    profiles:
      - dev
```

#### 5.2 Local Configuration File

**File**: `config/config.yaml` add sections

```yaml
pubsub:
  provider: "local"
  localEndpoint: "http://localhost:8081/push"

pmtiles:
  enabled: false
  source: "http://localhost:8080/map.pmtiles"
  roadLayer: "transportation"
  zoomLevel: 14
```

---

## File List

### New Files

| File Path | Description |
|-----------|-------------|
| `internal/domain/service/event_publisher.go` | EventPublisher interface |
| `internal/infra/pubsub/local_http_publisher.go` | Local HTTP Publisher for development |
| `internal/infra/pubsub/google_publisher.go` | Google Pub/Sub Publisher |
| `internal/infra/pubsub/provider.go` | FX Provider |
| `internal/infra/routing/pmtiles/service.go` | PMTiles routing service |
| `internal/infra/routing/pmtiles/mvt_parser.go` | MVT parser |
| `internal/infra/routing/pmtiles/pathfinder.go` | Pathfinding algorithm |
| `internal/delivery/worker/handler/push_handler.go` | Worker Handler |
| `internal/delivery/worker/server.go` | Worker HTTP Server |
| `cmd/geoworker/main.go` | Geo Worker entry point |

### Modified Files

| File Path | Changes |
|-----------|---------|
| `config/config.go` | Add PubSubConfig, PMTilesConfig |
| `config/config.yaml` | Add pubsub, pmtiles sections |
| `docker-compose.yml` | Add pmtiles-server, geoworker services |
| `cmd/radar/main.go` | Inject EventPublisher |
| `internal/usecase/impl/notification_service.go` | Change to publish events to Pub/Sub |
| `internal/domain/repository/subscription_repository.go` | Add FindSubscriberAddressesByUserIDs |
| `internal/infra/persistence/postgres/subscription_repository.go` | Implement FindSubscriberAddressesByUserIDs |
| `Dockerfile` | Support multiple binaries (radar, geoworker) |

---

## Dependencies

```go
require (
    cloud.google.com/go/pubsub v1.50.1        // Google Pub/Sub
    github.com/paulmach/orb v0.12.0           // MVT parsing + geometric operations
)
```

---

## Validation Steps

### Local Testing Flow

1. **Start infrastructure**

   ```bash
   docker compose --profile dev up -d
   ```

2. **Place PMTiles file**

   ```bash
   mkdir -p data/pmtiles
   cp your-map.pmtiles data/pmtiles/map.pmtiles
   ```

3. **Start API Server (Terminal 1)**

   ```bash
   go run cmd/radar/main.go
   ```

4. **Start Geo Worker (Terminal 2)**

   ```bash
   go run cmd/geoworker/main.go
   ```

5. **Send test request (Terminal 3)**

   ```bash
   curl -X POST http://localhost:4433/api/v1/notifications \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        -d '{"location_data": {"location_name": "Test", "full_address": "Test Address", "latitude": 25.03, "longitude": 121.56}}'
   ```

6. **Check Logs**
   - API Server: Should display `[LocalPubSub] Publishing event...`
   - Geo Worker: Should display `[Worker] Processing notification...`

---

## Production Setup

### Google Cloud Pub/Sub

1. **Create Topic**

   ```bash
   gcloud pubsub topics create notification-events
   ```

2. **Create Push Subscription**

  ```bash
   gcloud pubsub subscriptions create notification-push \
     --topic=notification-events \
     --push-endpoint=https://your-geoworker-url/push
  ```

3. **Configure environment variables**

  ```yaml
   pubsub:
     provider: "google"
     projectId: "your-project-id"
     topicId: "notification-events"
   ```

### PMTiles on GCS

1. **Upload PMTiles file**

  ```bash
   gsutil cp your-map.pmtiles gs://your-bucket/map.pmtiles
   ```

2. **Configure environment variables**

  ```yaml
   pmtiles:
     enabled: true
     source: "https://storage.googleapis.com/your-bucket"
     roadLayer: "transportation"
     zoomLevel: 14
  ```

---

## Important Notes

1. **PMTiles Data Preparation**: Requires PMTiles file containing road data, typically converted from OSM data (using tippecanoe or planetiler)

2. **MVT Layer Name**: Need to confirm the road layer name in your PMTiles (common: `transportation`, `roads`)

3. **Tile Boundaries**: Paths exceeding tile boundaries automatically fall back to Haversine straight-line distance

4. **Graceful Degradation**:
   - If Pub/Sub is unavailable, automatically falls back to synchronous processing
   - If PMTiles is unavailable, automatically falls back to Haversine

5. **Performance Considerations**:
   - Tiles are cached to avoid repeated reads
   - Uses Dijkstra One-to-Many to reduce redundant calculations
   - PostGIS pre-filtering reduces the number of targets requiring road network distance calculation

---

## Multi-stage Dockerfile Build

```dockerfile
# Build radar (main API service)
FROM golang:alpine AS builder
RUN CGO_ENABLED=0 go build -o radar ./cmd/radar
RUN CGO_ENABLED=0 go build -o geoworker ./cmd/geoworker

# radar image
FROM gcr.io/distroless/static-debian11:nonroot AS radar
COPY --from=builder /app/radar /app/radar
ENTRYPOINT ["/app/radar"]

# geoworker image
FROM gcr.io/distroless/static-debian11:nonroot AS geoworker
COPY --from=builder /app/geoworker /app/geoworker
ENTRYPOINT ["/app/geoworker"]
```

Usage:

```bash
# Build radar
docker build --target radar -t radar:latest .

# Build geoworker
docker build --target geoworker -t geoworker:latest .
```
