package usecases_test

import (
	"context"
	"testing"

	"github.com/samirrijal/bilbopass/internal/core/domain"
	"github.com/samirrijal/bilbopass/internal/core/usecases"
)

// --- Mock RouteRepository ---

type mockRouteRepo struct {
	getByIDFn      func(ctx context.Context, id string) (*domain.Route, error)
	listByAgencyFn func(ctx context.Context, agencyID string) ([]domain.Route, error)
}

func (m *mockRouteRepo) Upsert(ctx context.Context, r *domain.Route) error        { return nil }
func (m *mockRouteRepo) UpsertBatch(ctx context.Context, rs []domain.Route) error { return nil }

func (m *mockRouteRepo) GetByID(ctx context.Context, id string) (*domain.Route, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockRouteRepo) ListByAgency(ctx context.Context, agencyID string) ([]domain.Route, error) {
	if m.listByAgencyFn != nil {
		return m.listByAgencyFn(ctx, agencyID)
	}
	return nil, nil
}

func (m *mockRouteRepo) ListByStop(ctx context.Context, stopUUID string) ([]domain.Route, error) {
	return nil, nil
}

// --- Mock VehiclePositionRepository ---

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

func TestRouteService_GetByID(t *testing.T) {
	repo := &mockRouteRepo{
		getByIDFn: func(ctx context.Context, id string) (*domain.Route, error) {
			return &domain.Route{ID: id, ShortName: "L1", LongName: "Etxebarri-Ibarbengoa"}, nil
		},
	}

	svc := usecases.NewRouteService(repo, &mockVehicleRepo{})
	route, err := svc.GetByID(context.Background(), "route-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if route.ShortName != "L1" {
		t.Errorf("expected L1, got %s", route.ShortName)
	}
}

func TestRouteService_ListByAgency(t *testing.T) {
	repo := &mockRouteRepo{
		listByAgencyFn: func(ctx context.Context, agencyID string) ([]domain.Route, error) {
			return []domain.Route{
				{ShortName: "L1"},
				{ShortName: "L2"},
			}, nil
		},
	}

	svc := usecases.NewRouteService(repo, &mockVehicleRepo{})
	routes, err := svc.ListByAgency(context.Background(), "metro_bilbao")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(routes))
	}
}

func TestRouteService_GetLiveVehicles(t *testing.T) {
	vRepo := &mockVehicleRepo{
		latestByRouteFn: func(ctx context.Context, routeID string) ([]domain.VehiclePosition, error) {
			return []domain.VehiclePosition{
				{VehicleID: "v1", Location: domain.GeoPoint{Lat: 43.26, Lon: -2.93}},
			}, nil
		},
	}

	svc := usecases.NewRouteService(&mockRouteRepo{}, vRepo)
	vehicles, err := svc.GetLiveVehicles(context.Background(), "route-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vehicles) != 1 {
		t.Fatalf("expected 1 vehicle, got %d", len(vehicles))
	}
}
