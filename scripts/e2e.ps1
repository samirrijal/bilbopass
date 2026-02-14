# scripts/e2e.ps1 — End-to-end smoke tests for the BilboPass API (PowerShell)
# Expects the API to be running on localhost:8080.
$ErrorActionPreference = "Continue"

$Base = "http://localhost:8080"
$Pass = 0
$Fail = 0
$Total = 0

function Assert-Status {
    param([string]$Label, [string]$Url, [int]$Expected)
    $script:Total++
    try {
        $resp = Invoke-WebRequest -Uri $Url -UseBasicParsing -ErrorAction Stop
        $code = $resp.StatusCode
    } catch {
        $code = [int]$_.Exception.Response.StatusCode
    }
    if ($code -eq $Expected) {
        Write-Host "  + $Label (HTTP $code)" -ForegroundColor Green
        $script:Pass++
    } else {
        Write-Host "  x $Label - expected $Expected, got $code" -ForegroundColor Red
        $script:Fail++
    }
}

function Assert-JsonField {
    param([string]$Label, [string]$Url, [string]$Field)
    $script:Total++
    try {
        $resp = Invoke-WebRequest -Uri $Url -UseBasicParsing -ErrorAction Stop
        $json = $resp.Content | ConvertFrom-Json
        if ($null -ne $json.$Field) {
            Write-Host "  + $Label (has '$Field')" -ForegroundColor Green
            $script:Pass++
        } else {
            Write-Host "  x $Label - missing field '$Field'" -ForegroundColor Red
            $script:Fail++
        }
    } catch {
        Write-Host "  x $Label - request failed: $_" -ForegroundColor Red
        $script:Fail++
    }
}

function Assert-NonEmptyArray {
    param([string]$Label, [string]$Url)
    $script:Total++
    try {
        $resp = Invoke-WebRequest -Uri $Url -UseBasicParsing -ErrorAction Stop
        $json = $resp.Content | ConvertFrom-Json
        $count = 0
        if ($json -is [System.Array]) {
            $count = $json.Count
        } elseif ($null -ne $json.data) {
            $count = $json.data.Count
        }
        if ($count -gt 0) {
            Write-Host "  + $Label ($count items)" -ForegroundColor Green
            $script:Pass++
        } else {
            Write-Host "  x $Label - expected non-empty array" -ForegroundColor Red
            $script:Fail++
        }
    } catch {
        Write-Host "  x $Label - request failed: $_" -ForegroundColor Red
        $script:Fail++
    }
}

function Assert-Header {
    param([string]$Label, [string]$Url, [string]$Header)
    $script:Total++
    try {
        $resp = Invoke-WebRequest -Uri $Url -UseBasicParsing -ErrorAction Stop
        if ($resp.Headers[$Header]) {
            Write-Host "  + $Label ($Header = $($resp.Headers[$Header]))" -ForegroundColor Green
            $script:Pass++
        } else {
            Write-Host "  x $Label - header '$Header' missing" -ForegroundColor Red
            $script:Fail++
        }
    } catch {
        Write-Host "  x $Label - request failed: $_" -ForegroundColor Red
        $script:Fail++
    }
}

# ──────────────────────────────────────────────
Write-Host ""
Write-Host "BilboPass E2E Smoke Tests" -ForegroundColor White
Write-Host "=========================" -ForegroundColor White
Write-Host ""

# System
Write-Host "[System]" -ForegroundColor Cyan
Assert-Status      "Health endpoint"      "$Base/v1/health"          200
Assert-JsonField   "Health has status"    "$Base/v1/health"          "status"
Assert-Status      "Ready endpoint"       "$Base/v1/ready"           200
Assert-Status      "Metrics endpoint"     "$Base/metrics"            200
Assert-Status      "Swagger UI"           "$Base/docs"               200
Assert-Status      "OpenAPI spec"         "$Base/docs/openapi.yaml"  200

# Security headers
Write-Host ""
Write-Host "[Security Headers]" -ForegroundColor Cyan
Assert-Header "X-Content-Type-Options" "$Base/v1/health" "X-Content-Type-Options"
Assert-Header "X-Frame-Options"        "$Base/v1/health" "X-Frame-Options"

# Agencies
Write-Host ""
Write-Host "[Agencies]" -ForegroundColor Cyan
Assert-Status         "List agencies"              "$Base/v1/agencies"                200
Assert-NonEmptyArray  "Agencies non-empty"         "$Base/v1/agencies"
Assert-JsonField      "Agencies paginated"         "$Base/v1/agencies"                "pagination"
Assert-Status         "Agencies with pagination"   "$Base/v1/agencies?offset=0&limit=3" 200

# Stops
Write-Host ""
Write-Host "[Stops]" -ForegroundColor Cyan
Assert-Status         "Nearby stops (Bilbao)"      "$Base/v1/stops/nearby?lat=43.263&lon=-2.935&radius=1000"  200
Assert-NonEmptyArray  "Nearby returns stops"        "$Base/v1/stops/nearby?lat=43.263&lon=-2.935&radius=1000"
Assert-Status         "Search stops"               "$Base/v1/stops/search?q=abando"  200
Assert-Status         "Nearby missing params"      "$Base/v1/stops/nearby"            400
Assert-Status         "Search missing query"       "$Base/v1/stops/search"            400
Assert-Status         "Bad radius"                 "$Base/v1/stops/nearby?lat=43.26&lon=-2.93&radius=50000" 400

# Routes
Write-Host ""
Write-Host "[Routes]" -ForegroundColor Cyan
Assert-Status "Routes missing agency_id" "$Base/v1/routes" 400

# GraphQL
Write-Host ""
Write-Host "[GraphQL]" -ForegroundColor Cyan
$script:Total++
try {
    $body = '{"query":"{ agencies { id slug name } }"}'
    $gqlResp = Invoke-WebRequest -Uri "$Base/graphql" -Method POST -Body $body -ContentType "application/json" -UseBasicParsing -ErrorAction Stop
    $gqlJson = $gqlResp.Content | ConvertFrom-Json
    $gqlCount = $gqlJson.data.agencies.Count
    if ($gqlCount -gt 0) {
        Write-Host "  + GraphQL agencies query ($gqlCount results)" -ForegroundColor Green
        $script:Pass++
    } else {
        Write-Host "  x GraphQL agencies query - no results" -ForegroundColor Red
        $script:Fail++
    }
} catch {
    Write-Host "  x GraphQL agencies query - request failed: $_" -ForegroundColor Red
    $script:Fail++
}

# Error format
Write-Host ""
Write-Host "[Error Format]" -ForegroundColor Cyan
$script:Total++
try {
    Invoke-WebRequest -Uri "$Base/v1/stops/nearby" -UseBasicParsing -ErrorAction Stop
    Write-Host "  x Expected 400 error" -ForegroundColor Red
    $script:Fail++
} catch {
    $errBody = $_.ErrorDetails.Message
    if (-not $errBody) {
        $errBody = $_.Exception.Response
    }
    # If we got a 400, that's the correct structured error
    $errCode = [int]$_.Exception.Response.StatusCode
    if ($errCode -eq 400) {
        Write-Host "  + Structured error response (400)" -ForegroundColor Green
        $script:Pass++
    } else {
        Write-Host "  x Expected 400, got $errCode" -ForegroundColor Red
        $script:Fail++
    }
}

# ──────────────────────────────────────────────
Write-Host ""
Write-Host "Results: $Pass/$Total passed, $Fail failed" -ForegroundColor White
if ($Fail -gt 0) {
    Write-Host "SOME TESTS FAILED" -ForegroundColor Red
    exit 1
} else {
    Write-Host "ALL TESTS PASSED" -ForegroundColor Green
}
