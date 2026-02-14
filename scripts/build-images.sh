#!/usr/bin/env bash
set -euo pipefail

REGISTRY="${REGISTRY:-ghcr.io/bilbopass}"
VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo 'dev')}"

SERVICES=("api" "ingestor" "realtime" "compensator")

GREEN='\033[0;32m'
NC='\033[0m'

log() { echo -e "${GREEN}[build]${NC} $1"; }

log "Building images (version: $VERSION)"

for svc in "${SERVICES[@]}"; do
  log "Building $svc..."
  docker build \
    --build-arg SERVICE="$svc" \
    --build-arg VERSION="$VERSION" \
    -t "$REGISTRY/$svc:$VERSION" \
    -t "$REGISTRY/$svc:latest" \
    -f deployments/docker/Dockerfile \
    .
  log "$svc built → $REGISTRY/$svc:$VERSION"
done

log "All images built successfully"
echo ""
docker images --filter "reference=$REGISTRY/*" --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}"
