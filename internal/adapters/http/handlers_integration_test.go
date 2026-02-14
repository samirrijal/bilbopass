//go:build integration
// +build integration

package http_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samirrijal/bilbopass/internal/adapters/http"
	"github.com/samirrijal/bilbopass/internal/adapters/postgres"
	"github.com/samirrijal/bilbopass/internal/core/domain"
	"github.com/samirrijal/bilbopass/internal/core/usecases"
	"github.com/samirrijal/bilbopass/internal/pkg/config"
)

// setupTestDB connects to the test database and returns a clean DB instance.
func setupTestDB(t *testing.T) *postgres.DB {
	cfg, err := config.Load("bilbopass-test")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	// Use a test-specific database name if needed
	dsn := cfg.Database.DSN()
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}

	db := &postgres.DB{Pool: pool}

	// Clear test data (optional — depends on schema design)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping db: %v", err)
	}

	return db
}

// setupTestDeps creates dependencies with real DB and repos, no cache.
func setupTestDeps(t *testing.T, db *postgres.DB) *http.Dependencies {
	agencyRepo := postgres.NewAgencyRepo(db)
	stopRepo := postgres.NewStopRepo(db)
	routeRepo := postgres.NewRouteRepo(db)
	tripRepo := postgres.NewTripRepo(db)
	vehicleRepo := postgres.NewVehicleRepo(db)

	return &http.Dependencies{
		Agencies:   usecases.NewAgencyService(agencyRepo),
		Stops:      usecases.NewStopService(stopRepo, nil),
		Routes:     usecases.NewRouteService(routeRepo, vehicleRepo),
		Departures: usecases.NewDepartureService(tripRepo),
		Trips:      usecases.NewTripService(tripRepo),
		DB:         db,
	}
}

// seedTestAgency inserts a test agency and returns its UUID.
func seedTestAgency(t *testing.T, db *postgres.DB, slug string) string {
	ctx := context.Background()
	agency := &domain.Agency{
		Slug:     slug,
		Name:     "Test Agency " + slug,
		Timezone: "Europe/Madrid",
	}
	if err := db.Pool.QueryRow(ctx, `
		INSERT INTO agencies (slug, name, timezone)
		VALUES ($1, $2, $3)
		ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name
		RETURNING id
	`, agency.Slug, agency.Name, agency.Timezone).Scan(&agency.ID); err != nil {
		t.Fatalf("seed agency: %v", err)
	}
	return agency.ID
}

// seedTestStop inserts a test stop and returns its UUID.
func seedTestStop(t *testing.T, db *postgres.DB, agencyID, stopID, name string) string {
	ctx := context.Background()
	var id string
	if err := db.Pool.QueryRow(ctx, `
		INSERT INTO stops (agency_id, stop_id, name, location)
		VALUES ($1, $2, $3, ST_Point(-2.935, 43.263, 4326))
		ON CONFLICT (agency_id, stop_id) DO UPDATE SET name = EXCLUDED.name
		RETURNING id
	`, agencyID, stopID, name).Scan(&id); err != nil {
		t.Fatalf("seed stop: %v", err)
	}
	return id
}

// seedTestRoute inserts a test route and returns its UUID.
func seedTestRoute(t *testing.T, db *postgres.DB, agencyID, routeID, shortName, longName string) string {
	ctx := context.Background()
	var id string
	if err := db.Pool.QueryRow(ctx, `
		INSERT INTO routes (agency_id, route_id, short_name, long_name, route_type, color)
		VALUES ($1, $2, $3, $4, 3, 'FF0000')
		ON CONFLICT (agency_id, route_id) DO UPDATE SET short_name = EXCLUDED.short_name
		RETURNING id
	`, agencyID, routeID, shortName, longName).Scan(&id); err != nil {
		t.Fatalf("seed route: %v", err)
	}
	return id
}

// TestListAgencies_Integration_WithRealDB tests agency listing against real database.
func TestListAgencies_Integration_WithRealDB(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	db := setupTestDB(t)
	defer db.Pool.Close()

	// Seed test data
	seedTestAgency(t, db, "test_metro")
	seedTestAgency(t, db, "test_bilbobus")

	// Create app with real repos
	deps := setupTestDeps(t, db)
	app := setupApp(deps)

	req := httptest.NewRequest("GET", "/v1/agencies", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result struct {
		Data       []domain.Agency     `json:"data"`
		Pagination struct{ Total int } `json:"pagination"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if result.Pagination.Total < 2 {
		t.Errorf("expected at least 2 agencies, got %d", result.Pagination.Total)
	}
}

// TestGetAgency_Integration tests agency lookup against real database.
func TestGetAgency_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	db := setupTestDB(t)
	defer db.Pool.Close()

	slug := "test_integ_" + time.Now().Format("20060102150405")
	seedTestAgency(t, db, slug)

	deps := setupTestDeps(t, db)
	app := setupApp(deps)

	req := httptest.NewRequest("GET", "/v1/agencies/"+slug, nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var agency domain.Agency
	if err := json.NewDecoder(resp.Body).Decode(&agency); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if agency.Slug != slug {
		t.Errorf("expected slug %s, got %s", slug, agency.Slug)
	}
}

// TestNearbyStops_Integration tests geospatial query against real database.
func TestNearbyStops_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	db := setupTestDB(t)
	defer db.Pool.Close()

	agencyID := seedTestAgency(t, db, "test_spatial")
	// Bilbao coordinates: 43.263, -2.935
	seedTestStop(t, db, agencyID, "stop1", "Abando")

	deps := setupTestDeps(t, db)
	app := setupApp(deps)

	// Search nearby Bilbao
	req := httptest.NewRequest("GET", "/v1/stops/nearby?lat=43.263&lon=-2.935&radius=5000", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var stops []domain.Stop
	if err := json.NewDecoder(resp.Body).Decode(&stops); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(stops) == 0 {
		t.Error("expected at least 1 nearby stop, got 0")
	}
}
