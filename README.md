# NomNom-Radar 🍔📡

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen.svg)](https://github.com/slighter12/NomNom-Radar)
[![Contributions Welcome](https://img.shields.io/badge/contributions-welcome-orange.svg)](https://github.com/slighter12/NomNom-Radar/issues)

Your personal foodie radar! Automatically detects nearby food trucks and sends out a "Nom Nom" alert.

NomNom-Radar is a real-time notification backend service. It utilizes PostgreSQL with PostGIS for high-performance geospatial queries to track mobile food vendors and notify users when deliciousness is near.

## ✨ Core Features

* **Real-time Vendor Tracking**: Allows food truck owners to update their location instantly via a simple API endpoint.
* **Dynamic Geofencing**: Users can define a custom notification radius (e.g., 500 meters, 1 km).
* **High-Performance Geospatial Queries**: Leverages the power of PostGIS (`ST_DWithin`) for efficient, low-latency proximity checks.
* **Road Network Routing**: Uses Contraction Hierarchies (CH) algorithm for accurate road network distance and ETA calculations, detecting unreachable locations (e.g., Taiwan ↔ Penghu islands).
* **Push Notification System**: Integrates with services like Firebase Cloud Messaging (FCM) to deliver instant alerts to users' devices.
* **Open & Share-Alike**: Licensed under AGPL-3.0 to encourage community contribution while protecting the project from being used in proprietary, closed-source commercial services.

## 🛠️ Tech Stack

* **Backend**: **Go** with **Echo** framework
* **Database**: PostgreSQL + PostGIS extension for geospatial data
* **Database Driver/ORM**: **GORM**
* **Routing Engine**: Custom CH implementation with [LdDl/ch](https://github.com/LdDl/ch) + Grid-based spatial index
* **Push Notifications**: Firebase Cloud Messaging (FCM)
* **Containerization**: Docker & Docker Compose

## 🚀 Getting Started

Follow these instructions to get a local copy up and running for development and testing purposes.

### Prerequisites

* [Go](https://go.dev/doc/install) (e.g., v1.25 or later)
* [Docker](https://www.docker.com/) and [Docker Compose](https://docs.docker.com/compose/)
* A [Firebase](https://firebase.google.com/) project for push notifications.

### Installation & Setup

1. **Clone the repository:**

```bash
git clone https://github.com/slighter12/NomNom-Radar.git
cd NomNom-Radar
```

2. **Install dependencies:**

```bash
go mod download
```

3. **Set up the database:**

Make sure you have PostgreSQL with PostGIS extension installed and running.

**Supabase Deployment - Required PostGIS Setup:**

If you're deploying on Supabase, run the following SQL in the SQL Editor to properly configure PostGIS in the `extensions` schema:

```sql
-- 1. Drop existing PostGIS extension (WARNING: This will also drop geometry columns in your tables!)
DROP EXTENSION IF EXISTS postgis CASCADE;

-- 2. Create a clean extensions schema
CREATE SCHEMA IF NOT EXISTS extensions;
GRANT USAGE ON SCHEMA extensions TO postgres, anon, authenticated, service_role;

-- 3. Set search path (so applications can find it without code changes)
ALTER DATABASE postgres SET search_path TO "$user", public, extensions;
ALTER ROLE authenticated SET search_path TO "$user", public, extensions;
ALTER ROLE anon SET search_path TO "$user", public, extensions;
ALTER ROLE service_role SET search_path TO "$user", public, extensions;

-- 4. Reinstall PostGIS in the new schema
CREATE EXTENSION postgis SCHEMA extensions;
```

4. **Configure environment variables:**

Copy the example config and update with your settings:

```bash
cp config.example.yaml config.yaml
# Edit config.yaml with your database credentials and Firebase settings
```

5. **Run the application:**

```bash
go run cmd/server/main.go
```

### Build Road PMTiles (Taiwan example)

Install the required tools:

```bash
brew install osmium-tool tippecanoe
```

Then generate a road-only GeoJSON and PMTiles:

```bash
# 1) Filter: keep only ways with the "highway" tag (w/highway = ways with highway tag)
# This produces a smaller .pbf containing only roads.
osmium tags-filter taiwan-latest.osm.pbf w/highway -o filtered-roads.osm.pbf --overwrite

# 2) Convert: turn the filtered PBF into GeoJSON
# osmium export converts OSM ways to LineString and preserves all tags.
osmium export filtered-roads.osm.pbf -o roads.geojson --overwrite

# 3) Build PMTiles (same parameters as before)
tippecanoe -o walking.pmtiles \
  -z15 -Z15 \
  --buffer=100 \
  --no-clipping \
  --layer=roads \
  roads.geojson
```

## 🧪 Testing

This project uses [mockery](https://github.com/vektra/mockery) to generate mocks. To regenerate mocks after interface changes:

```bash
mockery
```

## 🤔 How It Works

1. **Vendor Pings Location**: A food truck owner sends a `POST` request with their `vendorId` and current coordinates (`latitude`, `longitude`) to the `/api/vendors/location` endpoint.
2. **Location is Stored**: The Go service updates the vendor's location in the PostGIS database. The location is stored as a `GEOGRAPHY` or `GEOMETRY` type using a Go database driver.
3. **Proximity Check Job**: A background Goroutine or scheduled job runs periodically (e.g., every minute).
4. **Geospatial Query**: The worker queries the database, asking: "For each user, are there any vendors within their specified notification radius?" This is efficiently handled by the PostGIS `ST_DWithin` function.
5. **Notification Sent**: If a match is found, the service triggers a push notification via FCM to the relevant user's device, letting them know a favorite food truck is nearby.

## 🗺️ Routing Engine

NomNom-Radar includes a high-performance road network routing engine for accurate distance and ETA calculations.

### Features

* **Road Network Distance**: Calculate real driving distances instead of straight-line (Haversine) approximations
* **Island Detection**: Correctly identifies unreachable locations (e.g., Taiwan ↔ Penghu, Kinmen islands)
* **One-to-Many Queries**: Efficiently calculate distances from one merchant to thousands of users
* **GPS Snap Validation**: Detects GPS drift by validating snap distance to road network
* **Haversine Fallback**: Gracefully degrades to straight-line distance when routing data is unavailable

### CLI Tool

The `cmd/routing` CLI tool handles OSM data preprocessing:

```bash
# Build the routing CLI
go build -o bin/routing-cli ./cmd/routing

# Download and prepare Taiwan routing data
./bin/routing-cli prepare --region taiwan --output ./data/routing

# Or step-by-step:
./bin/routing-cli download --region taiwan --output /tmp
./bin/routing-cli convert --input /tmp/taiwan-latest.osm.pbf --output ./data/routing

# Validate data integrity
./bin/routing-cli validate --dir ./data/routing
```

### Configuration

```yaml
routing:
  enabled: true
  data_path: "./data/routing"
  max_snap_distance_km: 0.5        # Max GPS snap distance to road (500m)
  default_speed_kmh: 30            # Urban scooter speed for ETA
  query_pool_size: 8               # Concurrent query workers
  one_to_many_workers: 20          # Batch query workers
  pre_filter_radius_multiplier: 1.3
```

### Architecture

```text
internal/infra/routing/
├── ch/                     # CH algorithm implementation
│   ├── engine.go           # Main routing engine (Dijkstra placeholder)
│   ├── spatial.go          # Grid-based spatial index
│   └── engine_test.go      # Comprehensive tests
└── loader/                 # Data loading
    ├── csv_loader.go       # CSV parsing (vertices, edges, shortcuts)
    └── metadata.go         # Version tracking
```

## 🤝 Contributing

Contributions, issues, and feature requests are welcome! Feel free to check the [issues page](https://github.com/slighter12/NomNom-Radar/issues).

Please adhere to this project's `code of conduct`.

## 📄 License

This project, `NomNom-Radar`, is licensed under the **GNU Affero General Public License v3.0 (AGPL-3.0)**.

In simple terms, this means:

* **✓ Freedom to Use**: You are free to run, study, share, and modify the software.
* **🔗 Share Alike**: If you modify this project's code and make it available as a public network service (e.g., a website or API), you **must** also release your modified source code to all users of that service under the same AGPL-3.0 license.

The choice of AGPL-3.0 is intended to foster community collaboration while preventing this project from being used to create proprietary, closed-source commercial services. We welcome all contributions, but please ensure you understand and agree to the terms of this license.

See the [LICENSE](LICENSE) file for the full legal text.

---
*Copyright (c) 2025, [slighter12]*
