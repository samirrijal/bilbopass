# BilboPass

Real-time transit platform for Bilbao and the Basque Country. Ingests GTFS/GTFS-RT feeds from **35 transit agencies** via Open Data Euskadi, serving stop search, departures, live vehicle positions, and delay detection through REST, GraphQL, and WebSocket APIs.

## Architecture

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│  GTFS Feeds │    │  GTFS-RT    │    │  Clients    │
│  (35 zips)  │    │  (28 feeds) │    │  REST/GQL/WS│
└──────┬──────┘    └──────┬──────┘    └──────┬──────┘
       │                  │                  │
  ┌────▼──────┐    ┌──────▼──────┐    ┌──────▼──────┐
  │ Ingestor  │    │  Realtime   │    │  API Server │
  │ cmd/ingest│    │  cmd/realtime│   │  cmd/api    │
  └────┬──────┘    └──────┬──────┘    └──────┬──────┘
       │                  │                  │
       │           ┌──────▼──────┐           │
       │           │    NATS     │◄──────────┤
       │           │  JetStream  │           │
       │           └─────────────┘           │
       │                                     │
  ┌────▼─────────────────────────────────────▼──┐
  │           TimescaleDB + PostGIS              │
  │  stops│routes│trips│stop_times│vehicles│delays│
  └──────────────────────────────────────────────┘
       │                                     │
  ┌────▼──────┐                        ┌─────▼─────┐
  │  Valkey   │                        │  Grafana   │
  │  Cache    │                        │ Prometheus │
  └───────────┘                        │ Tempo/Loki │
                                       └───────────┘
```

**Stack:** Go 1.24 · Fiber · pgx/v5 · TimescaleDB+PostGIS · NATS JetStream · Valkey · graphql-go · OpenTelemetry · Prometheus · Temporal

## Quick Start

### Prerequisites

- Go 1.24+
- Docker & Docker Compose

### 1. Start Infrastructure

```bash
cp .env.example .env   # edit DB_PASSWORD if desired
docker compose up -d
docker compose exec timescale pg_isready -U transit -d bilbopass
```

### 2. Ingest GTFS Data

```bash
# All 35 Basque Country agencies
go run cmd/ingestor/main.go manifest.json

# Or a single agency
go run cmd/ingestor/main.go manifest.json metro_bilbao
```

### 3. Start Services

```bash
# API server (port 8080)
go run cmd/api/main.go

# Realtime GTFS-RT poller (separate terminal)
go run cmd/realtime/main.go
```

### Windows (PowerShell)

```powershell
.\scripts\dev.ps1            # Start infra + API
.\scripts\ingest.ps1         # Ingest all agencies
```

## Data Sources

All data from [Open Data Euskadi](https://opendata.euskadi.eus) — 35 GTFS static feeds, 28 GTFS-RT feeds.

| Agency       | Type  | Stops  | Routes |
| ------------ | ----- | ------ | ------ |
| Metro Bilbao | Metro | 191    | ~3     |
| EuskoTran    | Tram  | ~50    | ~3     |
| Bizkaibus    | Bus   | ~2,500 | ~100+  |
| Lurraldebus  | Bus   | ~3,000 | ~100+  |
| + 31 more... | Mixed | ~3,800 | ~1,400 |

**Totals:** ~9,541 stops · ~1,656 routes · ~242,656 trips · ~3,598,294 stop_times

## API Reference

### REST Endpoints

| Method | Path                                        | Description                           | Cache    |
| ------ | ------------------------------------------- | ------------------------------------- | -------- |
| GET    | `/v1/health`                                | Health check                          | 10s      |
| GET    | `/v1/ready`                                 | Readiness check (DB/NATS/cache)       | no-store |
| GET    | `/v1/agencies`                              | List all transit agencies (paginated) | 1h       |
| GET    | `/v1/agencies/:slug`                        | Get agency by slug name               | 1h       |
| GET    | `/v1/agencies/:slug/routes`                 | List routes for agency (paginated)    | 1h       |
| GET    | `/v1/stops/nearby?lat=&lon=&radius=&limit=` | Find stops near location              | 5m       |
| GET    | `/v1/stops/search?q=&limit=`                | Fuzzy search stops by name            | 5m       |
| GET    | `/v1/stops/batch?ids=...`                   | Get multiple stops by IDs (max 100)   | 5m       |
| GET    | `/v1/stops/:id`                             | Get stop by ID                        | 10m      |
| GET    | `/v1/stops/:id/departures?limit=`           | Next departures at stop               | 10m      |
| GET    | `/v1/stops/:id/routes`                      | Routes serving this stop              | 1h       |
| GET    | `/v1/routes?agency_id=`                     | List routes by agency (paginated)     | 1h       |
| GET    | `/v1/routes/:id`                            | Get route by ID                       | 10m      |
| GET    | `/v1/routes/:id/vehicles`                   | Live vehicle positions for route      | no-cache |
| GET    | `/v1/trips/:id`                             | Get trip by ID                        | 10m      |
| GET    | `/v1/trips/:id/stop-times`                  | Ordered stop-times for trip           | 1h       |
| GET    | `/v1/feeds/status`                          | GTFS feed statistics (counts)         | 1m       |
| GET    | `/metrics`                                  | Prometheus metrics                    | no-cache |
| POST   | `/graphql`                                  | GraphQL endpoint                      | vary     |
| WS     | `/ws`                                       | WebSocket real-time stream            | —        |

**Features:**

- ✅ Pagination with RFC 8288 Link headers (`first`, `prev`, `next`, `last`)
- ✅ ETag support with 304 Not Modified responses
- ✅ Response compression (gzip)
- ✅ Request ID logging (correlation tracking)
- ✅ Per-endpoint rate limiting (120 req/min per IP)
- ✅ Request body size limit (1 MB)

### Example Requests

```bash
# List all agencies
curl "http://localhost:8080/v1/agencies"

# Get a specific agency
curl "http://localhost:8080/v1/agencies/metro_bilbao"

# Routes for an agency
curl "http://localhost:8080/v1/agencies/metro_bilbao/routes"

# Find stops near Bilbao city center
curl "http://localhost:8080/v1/stops/nearby?lat=43.263&lon=-2.935&radius=500&limit=10"

# Search for "Abando" stops
curl "http://localhost:8080/v1/stops/search?q=Abando"

# Get multiple stops efficiently
curl "http://localhost:8080/v1/stops/batch?ids=<id1>,<id2>,<id3>"

# Next departures at a stop
curl "http://localhost:8080/v1/stops/<stop-id>/departures?limit=5"

# Routes serving a specific stop
curl "http://localhost:8080/v1/stops/<stop-id>/routes"

# Get trip details
curl "http://localhost:8080/v1/trips/<trip-id>"

# Get stop-times for a trip
curl "http://localhost:8080/v1/trips/<trip-id>/stop-times"

# Feed statistics
curl "http://localhost:8080/v1/feeds/status"

# Batch requests support pagination
curl "http://localhost:8080/v1/agencies?offset=0&limit=5" -H "Accept-Encoding: gzip"
```

### GraphQL

```bash
curl -X POST http://localhost:8080/graphql \
  -H "Content-Type: application/json" \
  -d '{"query": "{ agencies { slug name } stopsNearby(lat: 43.263, lon: -2.935, radius: 500) { name location { lat lon } distance } }"}'
```

Available queries: `agencies`, `stopsNearby`, `searchStops`, `stop`, `route`, `routesByAgency`, `routeVehicles`, `stopDepartures`

### WebSocket

```javascript
const ws = new WebSocket("ws://localhost:8080/ws");

// Auto-receives all vehicle position updates
// Subscribe to specific channels:
ws.send(
  JSON.stringify({
    action: "subscribe",
    agency: "metro_bilbao",
    channel: "vehicles", // vehicles | alerts | delays
  }),
);
```

## Project Structure

```
bilbopass/
├── cmd/
│   ├── api/              # REST + GraphQL + WebSocket server
│   ├── ingestor/         # GTFS static data importer
│   ├── realtime/         # GTFS-RT stream processor
│   └── compensator/      # Temporal workflow worker
├── internal/
│   ├── core/
│   │   ├── domain/       # Entities & value objects
│   │   ├── ports/        # Repository & service interfaces
│   │   └── usecases/     # Business logic
│   ├── adapters/
│   │   ├── postgres/     # pgx repository implementations
│   │   ├── nats/         # JetStream publisher/subscriber
│   │   ├── valkey/       # Read-through cache layer
│   │   └── http/         # Fiber handlers, router, GraphQL, WebSocket
│   ├── gtfsrt/           # Generated protobuf bindings
│   ├── pkg/
│   │   ├── config/       # Viper configuration
│   │   ├── metrics/      # Prometheus metrics & middleware
│   │   ├── telemetry/    # OpenTelemetry tracing
│   │   └── geospatial/   # PostGIS helpers
│   └── workflows/        # Temporal compensation workflows
├── deployments/docker/   # Dockerfile & service compose
├── migrations/           # SQL migrations (auto-run by init)
├── observability/        # Grafana, Prometheus, Tempo, Loki configs
├── scripts/              # Dev & build scripts (bash + PowerShell)
├── manifest.json         # 35 agency GTFS feed URLs
├── config.yaml           # Local dev configuration
└── docker-compose.yml    # Infrastructure containers
```

## Configuration

Viper with `BILBOPASS_` prefix. Priority: env vars > config.yaml > defaults.

| Variable                      | Default               | Description          |
| ----------------------------- | --------------------- | -------------------- |
| `BILBOPASS_SERVER_PORT`       | 8080                  | API listen port      |
| `BILBOPASS_DATABASE_HOST`     | localhost             | TimescaleDB host     |
| `BILBOPASS_DATABASE_PORT`     | 5433                  | TimescaleDB port     |
| `BILBOPASS_DATABASE_USER`     | transit               | DB user              |
| `BILBOPASS_DATABASE_PASSWORD` | —                     | DB password          |
| `BILBOPASS_NATS_URL`          | nats://localhost:4222 | NATS server          |
| `BILBOPASS_VALKEY_ADDR`       | localhost:6379        | Valkey cache         |
| `BILBOPASS_TELEMETRY_ENABLED` | false                 | Enable OpenTelemetry |

## Observability

| Service      | URL                   | Purpose                      |
| ------------ | --------------------- | ---------------------------- |
| Grafana      | http://localhost:3000 | Dashboards (anonymous admin) |
| Prometheus   | http://localhost:9090 | Metrics scraping             |
| Tempo        | http://localhost:3200 | Distributed tracing          |
| Loki         | http://localhost:3100 | Log aggregation              |
| NATS Monitor | http://localhost:8222 | NATS server stats            |

Pre-provisioned dashboard: **BilboPass Transit Operations** — vehicle positions, delay events, per-agency stats, route delay rankings.

## Building Docker Images

```bash
bash scripts/build-images.sh

# Or build a single service
docker build --build-arg SERVICE=api -t bilbopass/api -f deployments/docker/Dockerfile .

# Run containerized
docker compose -f docker-compose.yml -f deployments/docker/docker-compose.services.yml up -d
```

## Performance Targets

| Metric             | Target  |
| ------------------ | ------- |
| API p95 latency    | < 100ms |
| WebSocket latency  | < 50ms  |
| DB query time      | < 10ms  |
| Stop search        | < 20ms  |
| GTFS-RT processing | < 5s    |

## License

Private — all rights reserved.
