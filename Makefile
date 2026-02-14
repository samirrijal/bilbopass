.PHONY: dev test lint build clean docker-up docker-down ingest realtime fmt vet

# ---- Development ----

dev: docker-up  ## Start infra + API server
	go run cmd/api/main.go

docker-up:  ## Start infrastructure containers
	docker compose up -d timescale nats valkey tempo loki prometheus grafana
	@echo "Waiting for TimescaleDB..."
	@until docker compose exec -T timescale pg_isready -U transit -d bilbopass >/dev/null 2>&1; do sleep 1; done
	@echo "TimescaleDB ready"

docker-down:  ## Stop all containers
	docker compose down

ingest:  ## Ingest GTFS data (usage: make ingest FILTER=metro_bilbao)
	go run cmd/ingestor/main.go manifest.json $(FILTER)

realtime:  ## Start GTFS-RT poller
	go run cmd/realtime/main.go

# ---- Quality ----

test:  ## Run all tests
	go test ./... -count=1 -race

test-cover:  ## Run tests with coverage
	go test ./... -count=1 -race -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

lint:  ## Run golangci-lint
	golangci-lint run --timeout=5m

fmt:  ## Format code
	gofmt -w -s .
	goimports -w .

vet:  ## Run go vet
	go vet ./...

# ---- Build ----

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

build:  ## Build all binaries
	go build -ldflags="-s -w -X main.version=$(VERSION)" -o bin/api ./cmd/api
	go build -ldflags="-s -w" -o bin/ingestor ./cmd/ingestor
	go build -ldflags="-s -w" -o bin/realtime ./cmd/realtime

build-docker:  ## Build Docker image
	docker build -f deployments/docker/Dockerfile -t bilbopass-api:$(VERSION) --build-arg VERSION=$(VERSION) .

clean:  ## Remove build artifacts
	rm -rf bin/ coverage.out coverage.html

# ---- Database ----

migrate:  ## Run SQL migrations
	@for f in migrations/*.sql; do \
		echo "Applying $$f..."; \
		docker compose exec -T timescale psql -U transit -d bilbopass -f /dev/stdin < $$f; \
	done

db-shell:  ## Open psql shell
	docker compose exec timescale psql -U transit -d bilbopass

db-stats:  ## Show table row counts
	@docker compose exec -T timescale psql -U transit -d bilbopass -c \
		"SELECT 'agencies' as tbl, count(*) FROM agencies UNION ALL \
		 SELECT 'stops', count(*) FROM stops UNION ALL \
		 SELECT 'routes', count(*) FROM routes UNION ALL \
		 SELECT 'trips', count(*) FROM trips UNION ALL \
		 SELECT 'stop_times', count(*) FROM stop_times ORDER BY tbl;"

# ---- Help ----

help:  ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
