$ErrorActionPreference = "Stop"

Write-Host "[bilbopass] Starting infrastructure containers..." -ForegroundColor Green

# Ensure .env exists
if (-not (Test-Path .env)) {
    Copy-Item .env.example .env
    Write-Host "[bilbopass] Created .env from .env.example" -ForegroundColor Yellow
}

docker compose up -d timescale nats valkey tempo loki prometheus grafana

Write-Host "[bilbopass] Waiting for TimescaleDB..." -ForegroundColor Green
do {
    Start-Sleep -Seconds 1
    docker compose exec -T timescale pg_isready -U transit -d bilbopass 2>$null
} while ($LASTEXITCODE -ne 0)

Write-Host "[bilbopass] TimescaleDB ready" -ForegroundColor Green

# Check ingested data
$stopCount = docker compose exec -T timescale psql -U transit -d bilbopass -t -c "SELECT count(*) FROM stops;" 2>$null
if ([string]::IsNullOrWhiteSpace($stopCount) -or $stopCount.Trim() -eq "0") {
    Write-Host "[bilbopass] No stops in database. Run: go run cmd/ingestor/main.go manifest.json" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "[bilbopass] Starting API server on :8080" -ForegroundColor Green
Write-Host "  REST:      http://localhost:8080/v1/"
Write-Host "  GraphQL:   http://localhost:8080/graphql"
Write-Host "  WebSocket: ws://localhost:8080/ws"
Write-Host "  Docs:      http://localhost:8080/docs"
Write-Host "  Metrics:   http://localhost:8080/metrics"
Write-Host "  Grafana:   http://localhost:3000"
Write-Host ""

go run cmd/api/main.go
