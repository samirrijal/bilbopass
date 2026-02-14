#!/usr/bin/env bash
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() { echo -e "${GREEN}[bilbopass]${NC} $1"; }
warn() { echo -e "${YELLOW}[bilbopass]${NC} $1"; }
err() { echo -e "${RED}[bilbopass]${NC} $1"; }

# Check prerequisites
for cmd in docker go; do
  if ! command -v "$cmd" &>/dev/null; then
    err "$cmd is required but not installed."
    exit 1
  fi
done

# Ensure .env exists
if [ ! -f .env ]; then
  cp .env.example .env
  warn "Created .env from .env.example — edit DB_PASSWORD if needed"
fi

# Start infrastructure
log "Starting infrastructure containers..."
docker compose up -d timescale nats valkey tempo loki prometheus grafana

# Wait for TimescaleDB to be ready
log "Waiting for TimescaleDB..."
until docker compose exec -T timescale pg_isready -U transit -d bilbopass 2>/dev/null; do
  sleep 1
done
log "TimescaleDB ready"

# Check if data is already ingested
STOP_COUNT=$(docker compose exec -T timescale psql -U transit -d bilbopass -t -c "SELECT count(*) FROM stops;" 2>/dev/null | tr -d ' ' || echo "0")
if [ "$STOP_COUNT" = "0" ] || [ "$STOP_COUNT" = "" ]; then
  warn "No stops found in database. Run: go run cmd/ingestor/main.go manifest.json"
fi

# Start API server
log "Starting API server on :8080..."
log "  REST:      http://localhost:8080/v1/"
log "  GraphQL:   http://localhost:8080/graphql"
log "  WebSocket: ws://localhost:8080/ws"
log "  Metrics:   http://localhost:8080/metrics"
log "  Grafana:   http://localhost:3000"
echo ""
go run cmd/api/main.go
