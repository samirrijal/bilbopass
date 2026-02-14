#!/usr/bin/env bash
set -euo pipefail

MANIFEST="${1:-manifest.json}"
FILTER="${2:-}"

GREEN='\033[0;32m'
NC='\033[0m'

log() { echo -e "${GREEN}[ingestor]${NC} $1"; }

if [ ! -f "$MANIFEST" ]; then
  echo "ERROR: manifest file '$MANIFEST' not found"
  exit 1
fi

AGENCY_COUNT=$(jq '.agencies | length' "$MANIFEST" 2>/dev/null || echo "?")
log "Manifest: $MANIFEST ($AGENCY_COUNT agencies)"

if [ -n "$FILTER" ]; then
  log "Filtering to agency slug: $FILTER"
  go run cmd/ingestor/main.go "$MANIFEST" "$FILTER"
else
  log "Ingesting all agencies..."
  go run cmd/ingestor/main.go "$MANIFEST"
fi

log "Ingestion complete"

# Print stats
docker compose exec -T timescale psql -U transit -d bilbopass -c "
  SELECT 'agencies' as type, count(*) as total FROM agencies
  UNION ALL
  SELECT 'stops', count(*) FROM stops
  UNION ALL
  SELECT 'routes', count(*) FROM routes
  UNION ALL
  SELECT 'trips', count(*) FROM trips
  UNION ALL
  SELECT 'stop_times', count(*) FROM stop_times
  ORDER BY type;
" 2>/dev/null || true
