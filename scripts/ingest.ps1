$ErrorActionPreference = "Stop"

$Manifest = if ($args[0]) { $args[0] } else { "manifest.json" }
$Filter = if ($args[1]) { $args[1] } else { "" }

if (-not (Test-Path $Manifest)) {
    Write-Host "ERROR: manifest file '$Manifest' not found" -ForegroundColor Red
    exit 1
}

Write-Host "[ingestor] Manifest: $Manifest" -ForegroundColor Green

if ($Filter) {
    Write-Host "[ingestor] Filtering to agency slug: $Filter" -ForegroundColor Green
    go run cmd/ingestor/main.go $Manifest $Filter
} else {
    Write-Host "[ingestor] Ingesting all agencies..." -ForegroundColor Green
    go run cmd/ingestor/main.go $Manifest
}

Write-Host "[ingestor] Ingestion complete" -ForegroundColor Green
