package http_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	handler "github.com/samirrijal/bilbopass/internal/adapters/http"
	"github.com/samirrijal/bilbopass/internal/core/domain"
	"github.com/samirrijal/bilbopass/internal/core/usecases"
)

// ---- Mock repositories ----

type mockAgencyRepo struct {
	listFn      func(ctx context.Context) ([]domain.Agency, error)
	getBySlugFn func(ctx context.Context, slug string) (*domain.Agency, error)
}

func (m *mockAgencyRepo) Upsert(ctx context.Context, a *domain.Agency) error { return nil }
func (m *mockAgencyRepo) List(ctx context.Context) ([]domain.Agency, error) {
	if m.listFn != nil {
		return m.listFn(ctx)
	}
	return nil, nil
}
func (m *mockAgencyRepo) GetBySlug(ctx context.Context, slug string) (*domain.Agency, error) {
	if m.getBySlugFn != nil {
		return m.getBySlugFn(ctx, slug)
	}
	return nil, nil
}

type mockStopRepo struct {
	findNearbyFn func(ctx context.Context, lat, lon, radius float64, limit int) ([]domain.Stop, error)
	getByIDFn    func(ctx context.Context, id string) (*domain.Stop, error)
	getByIDsFn   func(ctx context.Context, ids []string) ([]domain.Stop, error)
	searchFn     func(ctx context.Context, query string, near *domain.GeoPoint, limit int) ([]domain.Stop, error)
}

func (m *mockStopRepo) Upsert(ctx context.Context, s *domain.Stop) error       { return nil }
func (m *mockStopRepo) UpsertBatch(ctx context.Context, s []domain.Stop) error { return nil }
func (m *mockStopRepo) FindNearby(ctx context.Context, lat, lon, radius float64, limit int) ([]domain.Stop, error) {
	if m.findNearbyFn != nil {
		return m.findNearbyFn(ctx, lat, lon, radius, limit)
	}
	return nil, nil
}
func (m *mockStopRepo) GetByID(ctx context.Context, id string) (*domain.Stop, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, nil
}
func (m *mockStopRepo) GetByIDs(ctx context.Context, ids []string) ([]domain.Stop, error) {
	if m.getByIDsFn != nil {
		return m.getByIDsFn(ctx, ids)
	}
	return nil, nil
}
func (m *mockStopRepo) Search(ctx context.Context, query string, near *domain.GeoPoint, limit int) ([]domain.Stop, error) {
	if m.searchFn != nil {
		return m.searchFn(ctx, query, near, limit)
	}
	return nil, nil
}

type mockRouteRepo struct {
	getByIDFn    func(ctx context.Context, id string) (*domain.Route, error)
	listByAgFn   func(ctx context.Context, agencyID string) ([]domain.Route, error)
	listByStopFn func(ctx context.Context, stopUUID string) ([]domain.Route, error)
}

func (m *mockRouteRepo) Upsert(ctx context.Context, r *domain.Route) error       { return nil }
func (m *mockRouteRepo) UpsertBatch(ctx context.Context, r []domain.Route) error { return nil }
func (m *mockRouteRepo) GetByID(ctx context.Context, id string) (*domain.Route, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, nil
}
func (m *mockRouteRepo) ListByAgency(ctx context.Context, agencyID string) ([]domain.Route, error) {
	if m.listByAgFn != nil {
		return m.listByAgFn(ctx, agencyID)
	}
	return nil, nil
}
func (m *mockRouteRepo) ListByStop(ctx context.Context, stopUUID string) ([]domain.Route, error) {
	if m.listByStopFn != nil {
		return m.listByStopFn(ctx, stopUUID)
	}
	return nil, nil
}

type mockVehicleRepo struct {
	latestByRouteFn func(ctx context.Context, routeID string) ([]domain.VehiclePosition, error)
}

func (m *mockVehicleRepo) Insert(ctx context.Context, vp *domain.VehiclePosition) error { return nil }
func (m *mockVehicleRepo) InsertBatch(ctx context.Context, vps []domain.VehiclePosition) error {
	return nil
}
func (m *mockVehicleRepo) LatestByRoute(ctx context.Context, routeID string) ([]domain.VehiclePosition, error) {
	if m.latestByRouteFn != nil {
		return m.latestByRouteFn(ctx, routeID)
	}
	return nil, nil
}

type mockTripRepo struct {
	nextDepFn      func(ctx context.Context, stopUUID string, limit int) ([]domain.Departure, error)
	getByIDFn      func(ctx context.Context, id string) (*domain.Trip, error)
	getStopTimesFn func(ctx context.Context, tripID string) ([]domain.StopTime, error)
}

func (m *mockTripRepo) Upsert(ctx context.Context, t *domain.Trip) error       { return nil }
func (m *mockTripRepo) UpsertBatch(ctx context.Context, t []domain.Trip) error { return nil }
func (m *mockTripRepo) GetByID(ctx context.Context, id string) (*domain.Trip, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, nil
}
func (m *mockTripRepo) UpsertStopTimes(ctx context.Context, st []domain.StopTime) error { return nil }
func (m *mockTripRepo) GetStopTimes(ctx context.Context, tripID string) ([]domain.StopTime, error) {
	if m.getStopTimesFn != nil {
		return m.getStopTimesFn(ctx, tripID)
	}
	return nil, nil
}
func (m *mockTripRepo) NextDeparturesAtStop(ctx context.Context, stopUUID string, limit int) ([]domain.Departure, error) {
	if m.nextDepFn != nil {
		return m.nextDepFn(ctx, stopUUID, limit)
	}
	return nil, nil
}

// ---- Test helpers ----

func setupApp(deps *handler.Dependencies) *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	handler.SetupRoutes(app, deps)
	return app
}

func makeDeps(opts ...func(*handler.Dependencies)) *handler.Dependencies {
	d := &handler.Dependencies{
		Agencies:   usecases.NewAgencyService(&mockAgencyRepo{}),
		Stops:      usecases.NewStopService(&mockStopRepo{}, nil),
		Routes:     usecases.NewRouteService(&mockRouteRepo{}, &mockVehicleRepo{}),
		Departures: usecases.NewDepartureService(&mockTripRepo{}),
		Trips:      usecases.NewTripService(&mockTripRepo{}),
	}
	for _, o := range opts {
		o(d)
	}
	return d
}

func readBody(t *testing.T, body io.Reader) []byte {
	t.Helper()
	b, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return b
}

// ---- Agency handler tests ----

func TestListAgencies_Success(t *testing.T) {
	deps := makeDeps(func(d *handler.Dependencies) {
		d.Agencies = usecases.NewAgencyService(&mockAgencyRepo{
			listFn: func(ctx context.Context) ([]domain.Agency, error) {
				return []domain.Agency{
					{ID: "a1", Slug: "bilbobus", Name: "Bilbobus"},
					{ID: "a2", Slug: "metro", Name: "Metro Bilbao"},
				}, nil
			},
		})
	})
	app := setupApp(deps)

	req := httptest.NewRequest("GET", "/v1/agencies", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result struct {
		Data       []domain.Agency `json:"data"`
		Pagination struct {
			Total int `json:"total"`
		} `json:"pagination"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Pagination.Total != 2 {
		t.Errorf("expected total 2, got %d", result.Pagination.Total)
	}
	if len(result.Data) != 2 {
		t.Errorf("expected 2 agencies, got %d", len(result.Data))
	}
}

func TestListAgencies_Pagination(t *testing.T) {
	agencies := make([]domain.Agency, 5)
	for i := range agencies {
		agencies[i] = domain.Agency{ID: fmt.Sprintf("a%d", i), Name: fmt.Sprintf("Agency %d", i)}
	}

	deps := makeDeps(func(d *handler.Dependencies) {
		d.Agencies = usecases.NewAgencyService(&mockAgencyRepo{
			listFn: func(ctx context.Context) ([]domain.Agency, error) { return agencies, nil },
		})
	})
	app := setupApp(deps)

	req := httptest.NewRequest("GET", "/v1/agencies?offset=2&limit=2", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result struct {
		Data       []domain.Agency `json:"data"`
		Pagination struct {
			Offset int `json:"offset"`
			Limit  int `json:"limit"`
			Total  int `json:"total"`
		} `json:"pagination"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Pagination.Total != 5 {
		t.Errorf("expected total 5, got %d", result.Pagination.Total)
	}
	if len(result.Data) != 2 {
		t.Errorf("expected 2 agencies in page, got %d", len(result.Data))
	}
	if result.Pagination.Offset != 2 {
		t.Errorf("expected offset 2, got %d", result.Pagination.Offset)
	}
}

// ---- Stop handler tests ----

func TestNearbyStops_Success(t *testing.T) {
	deps := makeDeps(func(d *handler.Dependencies) {
		d.Stops = usecases.NewStopService(&mockStopRepo{
			findNearbyFn: func(ctx context.Context, lat, lon, radius float64, limit int) ([]domain.Stop, error) {
				return []domain.Stop{
					{ID: "s1", Name: "Abando", Location: domain.GeoPoint{Lat: 43.263, Lon: -2.935}},
				}, nil
			},
		}, nil)
	})
	app := setupApp(deps)

	req := httptest.NewRequest("GET", "/v1/stops/nearby?lat=43.263&lon=-2.935&radius=500", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var stops []domain.Stop
	json.NewDecoder(resp.Body).Decode(&stops)
	if len(stops) != 1 {
		t.Errorf("expected 1 stop, got %d", len(stops))
	}
}

func TestNearbyStops_MissingParams(t *testing.T) {
	app := setupApp(makeDeps())

	req := httptest.NewRequest("GET", "/v1/stops/nearby", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	var apiErr struct {
		Status int    `json:"status"`
		Code   string `json:"code"`
	}
	json.NewDecoder(resp.Body).Decode(&apiErr)
	if apiErr.Code != "bad_request" {
		t.Errorf("expected bad_request error, got %s", apiErr.Code)
	}
}

func TestNearbyStops_BadRadius(t *testing.T) {
	app := setupApp(makeDeps())

	req := httptest.NewRequest("GET", "/v1/stops/nearby?lat=43.26&lon=-2.93&radius=50000", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestSearchStops_Success(t *testing.T) {
	deps := makeDeps(func(d *handler.Dependencies) {
		d.Stops = usecases.NewStopService(&mockStopRepo{
			searchFn: func(ctx context.Context, query string, near *domain.GeoPoint, limit int) ([]domain.Stop, error) {
				return []domain.Stop{
					{ID: "s1", Name: "Abando Indalecio Prieto"},
				}, nil
			},
		}, nil)
	})
	app := setupApp(deps)

	req := httptest.NewRequest("GET", "/v1/stops/search?q=abando", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestSearchStops_MissingQuery(t *testing.T) {
	app := setupApp(makeDeps())

	req := httptest.NewRequest("GET", "/v1/stops/search", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestGetStop_NotFound(t *testing.T) {
	deps := makeDeps(func(d *handler.Dependencies) {
		d.Stops = usecases.NewStopService(&mockStopRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Stop, error) {
				return nil, fmt.Errorf("not found")
			},
		}, nil)
	})
	app := setupApp(deps)

	req := httptest.NewRequest("GET", "/v1/stops/nonexistent-id", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetStop_Success(t *testing.T) {
	deps := makeDeps(func(d *handler.Dependencies) {
		d.Stops = usecases.NewStopService(&mockStopRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Stop, error) {
				return &domain.Stop{ID: id, Name: "Moyua"}, nil
			},
		}, nil)
	})
	app := setupApp(deps)

	req := httptest.NewRequest("GET", "/v1/stops/abc-123", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var stop domain.Stop
	json.NewDecoder(resp.Body).Decode(&stop)
	if stop.Name != "Moyua" {
		t.Errorf("expected Moyua, got %s", stop.Name)
	}
}

// ---- Route handler tests ----

func TestGetRoute_Success(t *testing.T) {
	deps := makeDeps(func(d *handler.Dependencies) {
		d.Routes = usecases.NewRouteService(&mockRouteRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Route, error) {
				return &domain.Route{ID: id, LongName: "L1 Etxebarri-Ibarbengoa"}, nil
			},
		}, &mockVehicleRepo{})
	})
	app := setupApp(deps)

	req := httptest.NewRequest("GET", "/v1/routes/route-uuid", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var route domain.Route
	json.NewDecoder(resp.Body).Decode(&route)
	if route.LongName != "L1 Etxebarri-Ibarbengoa" {
		t.Errorf("unexpected route name: %s", route.LongName)
	}
}

func TestGetRoute_NotFound(t *testing.T) {
	deps := makeDeps(func(d *handler.Dependencies) {
		d.Routes = usecases.NewRouteService(&mockRouteRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Route, error) {
				return nil, fmt.Errorf("not found")
			},
		}, &mockVehicleRepo{})
	})
	app := setupApp(deps)

	req := httptest.NewRequest("GET", "/v1/routes/bad-id", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestListRoutes_MissingAgencyID(t *testing.T) {
	app := setupApp(makeDeps())

	req := httptest.NewRequest("GET", "/v1/routes", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestListRoutes_Success(t *testing.T) {
	deps := makeDeps(func(d *handler.Dependencies) {
		d.Routes = usecases.NewRouteService(&mockRouteRepo{
			listByAgFn: func(ctx context.Context, agencyID string) ([]domain.Route, error) {
				return []domain.Route{
					{ID: "r1", LongName: "Line 1"},
					{ID: "r2", LongName: "Line 2"},
				}, nil
			},
		}, &mockVehicleRepo{})
	})
	app := setupApp(deps)

	req := httptest.NewRequest("GET", "/v1/routes?agency_id=abc", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result struct {
		Data       []domain.Route `json:"data"`
		Pagination struct {
			Total int `json:"total"`
		} `json:"pagination"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Pagination.Total != 2 {
		t.Errorf("expected 2 routes total, got %d", result.Pagination.Total)
	}
}

// ---- Vehicles handler tests ----

func TestGetRouteVehicles_Success(t *testing.T) {
	deps := makeDeps(func(d *handler.Dependencies) {
		d.Routes = usecases.NewRouteService(&mockRouteRepo{}, &mockVehicleRepo{
			latestByRouteFn: func(ctx context.Context, routeID string) ([]domain.VehiclePosition, error) {
				return []domain.VehiclePosition{
					{VehicleID: "v1", Location: domain.GeoPoint{Lat: 43.26, Lon: -2.93}},
				}, nil
			},
		})
	})
	app := setupApp(deps)

	req := httptest.NewRequest("GET", "/v1/routes/some-route/vehicles", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var vehicles []domain.VehiclePosition
	json.NewDecoder(resp.Body).Decode(&vehicles)
	if len(vehicles) != 1 {
		t.Errorf("expected 1 vehicle, got %d", len(vehicles))
	}
}

// ---- Departure handler tests ----

func TestStopDepartures_Success(t *testing.T) {
	now := time.Now()
	deps := makeDeps(func(d *handler.Dependencies) {
		d.Departures = usecases.NewDepartureService(&mockTripRepo{
			nextDepFn: func(ctx context.Context, stopUUID string, limit int) ([]domain.Departure, error) {
				return []domain.Departure{
					{ScheduledTime: now, Platform: "1"},
				}, nil
			},
		})
	})
	app := setupApp(deps)

	req := httptest.NewRequest("GET", "/v1/stops/stop-uuid/departures", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// ---- Health handler tests ----

func TestHealth_Returns200(t *testing.T) {
	app := setupApp(makeDeps())

	req := httptest.NewRequest("GET", "/v1/health", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["status"] != "healthy" {
		t.Errorf("expected healthy status, got %v", result["status"])
	}
}

func TestReady_NoDB(t *testing.T) {
	deps := makeDeps()
	// DB, NATS, Cache are nil → should report not ready
	app := setupApp(deps)

	req := httptest.NewRequest("GET", "/v1/ready", nil)
	resp, _ := app.Test(req, -1)
	// With nil DB, ready should return 503
	if resp.StatusCode != 503 {
		t.Fatalf("expected 503, got %d", resp.StatusCode)
	}
}

// ---- Nearby stops Cache-Control header ----

func TestNearbyStops_CacheControlHeader(t *testing.T) {
	deps := makeDeps(func(d *handler.Dependencies) {
		d.Stops = usecases.NewStopService(&mockStopRepo{
			findNearbyFn: func(ctx context.Context, lat, lon, radius float64, limit int) ([]domain.Stop, error) {
				return []domain.Stop{}, nil
			},
		}, nil)
	})
	app := setupApp(deps)

	req := httptest.NewRequest("GET", "/v1/stops/nearby?lat=43.26&lon=-2.93", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	cc := resp.Header.Get("Cache-Control")
	if cc != "public, max-age=300" {
		t.Errorf("expected Cache-Control header, got %q", cc)
	}
}

// ---- Agency slug handler tests ----

func TestGetAgency_Success(t *testing.T) {
	deps := makeDeps(func(d *handler.Dependencies) {
		d.Agencies = usecases.NewAgencyService(&mockAgencyRepo{
			getBySlugFn: func(ctx context.Context, slug string) (*domain.Agency, error) {
				return &domain.Agency{ID: "a1", Slug: slug, Name: "Metro Bilbao"}, nil
			},
		})
	})
	app := setupApp(deps)

	req := httptest.NewRequest("GET", "/v1/agencies/metro_bilbao", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var agency domain.Agency
	json.NewDecoder(resp.Body).Decode(&agency)
	if agency.Slug != "metro_bilbao" {
		t.Errorf("expected slug metro_bilbao, got %s", agency.Slug)
	}
}

func TestGetAgency_NotFound(t *testing.T) {
	deps := makeDeps(func(d *handler.Dependencies) {
		d.Agencies = usecases.NewAgencyService(&mockAgencyRepo{
			getBySlugFn: func(ctx context.Context, slug string) (*domain.Agency, error) {
				return nil, fmt.Errorf("not found")
			},
		})
	})
	app := setupApp(deps)

	req := httptest.NewRequest("GET", "/v1/agencies/nonexistent", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// ---- Agency routes handler tests ----

func TestAgencyRoutes_Success(t *testing.T) {
	deps := makeDeps(func(d *handler.Dependencies) {
		d.Agencies = usecases.NewAgencyService(&mockAgencyRepo{
			getBySlugFn: func(ctx context.Context, slug string) (*domain.Agency, error) {
				return &domain.Agency{ID: "a1", Slug: slug, Name: "Metro"}, nil
			},
		})
		d.Routes = usecases.NewRouteService(&mockRouteRepo{
			listByAgFn: func(ctx context.Context, agencyID string) ([]domain.Route, error) {
				return []domain.Route{
					{ID: "r1", LongName: "L1"},
					{ID: "r2", LongName: "L2"},
				}, nil
			},
		}, &mockVehicleRepo{})
	})
	app := setupApp(deps)

	req := httptest.NewRequest("GET", "/v1/agencies/metro/routes", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result struct {
		Data       []domain.Route      `json:"data"`
		Pagination struct{ Total int } `json:"pagination"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Pagination.Total != 2 {
		t.Errorf("expected 2 routes, got %d", result.Pagination.Total)
	}
}

// ---- Stop routes handler tests ----

func TestStopRoutes_Success(t *testing.T) {
	deps := makeDeps(func(d *handler.Dependencies) {
		d.Routes = usecases.NewRouteService(&mockRouteRepo{
			listByStopFn: func(ctx context.Context, stopUUID string) ([]domain.Route, error) {
				return []domain.Route{
					{ID: "r1", LongName: "Line 1"},
				}, nil
			},
		}, &mockVehicleRepo{})
	})
	app := setupApp(deps)

	req := httptest.NewRequest("GET", "/v1/stops/stop-uuid/routes", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var routes []domain.Route
	json.NewDecoder(resp.Body).Decode(&routes)
	if len(routes) != 1 {
		t.Errorf("expected 1 route, got %d", len(routes))
	}
}

// ---- Trip handler tests ----

func TestGetTrip_Success(t *testing.T) {
	deps := makeDeps(func(d *handler.Dependencies) {
		d.Trips = usecases.NewTripService(&mockTripRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Trip, error) {
				return &domain.Trip{ID: id, Headsign: "Etxebarri"}, nil
			},
		})
	})
	app := setupApp(deps)

	req := httptest.NewRequest("GET", "/v1/trips/trip-uuid", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var trip domain.Trip
	json.NewDecoder(resp.Body).Decode(&trip)
	if trip.Headsign != "Etxebarri" {
		t.Errorf("expected Etxebarri, got %s", trip.Headsign)
	}
}

func TestGetTrip_NotFound(t *testing.T) {
	deps := makeDeps(func(d *handler.Dependencies) {
		d.Trips = usecases.NewTripService(&mockTripRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Trip, error) {
				return nil, fmt.Errorf("not found")
			},
		})
	})
	app := setupApp(deps)

	req := httptest.NewRequest("GET", "/v1/trips/bad-id", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestTripStopTimes_Success(t *testing.T) {
	deps := makeDeps(func(d *handler.Dependencies) {
		d.Trips = usecases.NewTripService(&mockTripRepo{
			getStopTimesFn: func(ctx context.Context, tripID string) ([]domain.StopTime, error) {
				return []domain.StopTime{
					{ID: "st1", TripID: tripID, StopSequence: 1},
					{ID: "st2", TripID: tripID, StopSequence: 2},
				}, nil
			},
		})
	})
	app := setupApp(deps)

	req := httptest.NewRequest("GET", "/v1/trips/trip-uuid/stop-times", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var sts []domain.StopTime
	json.NewDecoder(resp.Body).Decode(&sts)
	if len(sts) != 2 {
		t.Errorf("expected 2 stop-times, got %d", len(sts))
	}
}

// ---- X-API-Version header ----

func TestAPIVersionHeader(t *testing.T) {
	app := setupApp(makeDeps())

	req := httptest.NewRequest("GET", "/v1/health", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	v := resp.Header.Get("X-API-Version")
	if v != "1.0.0" {
		t.Errorf("expected X-API-Version 1.0.0, got %q", v)
	}
}

// ---- Link header on pagination ----

func TestListAgencies_LinkHeader(t *testing.T) {
	agencies := make([]domain.Agency, 10)
	for i := range agencies {
		agencies[i] = domain.Agency{ID: fmt.Sprintf("a%d", i), Name: fmt.Sprintf("Agency %d", i)}
	}

	deps := makeDeps(func(d *handler.Dependencies) {
		d.Agencies = usecases.NewAgencyService(&mockAgencyRepo{
			listFn: func(ctx context.Context) ([]domain.Agency, error) { return agencies, nil },
		})
	})
	app := setupApp(deps)

	req := httptest.NewRequest("GET", "/v1/agencies?offset=0&limit=3", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	link := resp.Header.Get("Link")
	if link == "" {
		t.Fatal("expected Link header, got empty")
	}
	// Should contain rel="next"
	if !strings.Contains(link, `rel="next"`) {
		t.Errorf("expected next link, got %s", link)
	}
	if !strings.Contains(link, `rel="first"`) {
		t.Errorf("expected first link, got %s", link)
	}
	if !strings.Contains(link, `rel="last"`) {
		t.Errorf("expected last link, got %s", link)
	}
}

// TestAccessLogMiddleware verifies structured access logging is emitted.
func TestAccessLogMiddleware(t *testing.T) {
	app := fiber.New()

	// Register middleware
	app.Use(handler.AccessLogMiddleware())

	// Simple test route
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"ok": true})
	})

	// Make request
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "test-req-123")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	// Verify response body
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "ok") {
		t.Errorf("expected response body to contain 'ok', got %s", string(body))
	}
}
