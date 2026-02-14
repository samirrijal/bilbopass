package usecases_test

import (
	"context"
	"testing"

	"github.com/samirrijal/bilbopass/internal/core/domain"
	"github.com/samirrijal/bilbopass/internal/core/usecases"
)

// --- Mock StopRepository ---

type mockStopRepo struct {
	findNearbyFn func(ctx context.Context, lat, lon, radius float64, limit int) ([]domain.Stop, error)
	getByIDFn    func(ctx context.Context, id string) (*domain.Stop, error)
	getByIDsFn   func(ctx context.Context, ids []string) ([]domain.Stop, error)
	searchFn     func(ctx context.Context, query string, near *domain.GeoPoint, limit int) ([]domain.Stop, error)
}

func (m *mockStopRepo) Upsert(ctx context.Context, stop *domain.Stop) error        { return nil }
func (m *mockStopRepo) UpsertBatch(ctx context.Context, stops []domain.Stop) error { return nil }

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

// --- Tests ---

func TestStopService_FindNearby(t *testing.T) {
	repo := &mockStopRepo{
		findNearbyFn: func(ctx context.Context, lat, lon, radius float64, limit int) ([]domain.Stop, error) {
			return []domain.Stop{
				{ID: "1", Name: "Abando", Location: domain.GeoPoint{Lat: 43.263, Lon: -2.935}},
				{ID: "2", Name: "Moyua", Location: domain.GeoPoint{Lat: 43.264, Lon: -2.934}},
			}, nil
		},
	}

	svc := usecases.NewStopService(repo, nil)

	stops, err := svc.FindNearby(context.Background(), 43.263, -2.935, 500, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stops) != 2 {
		t.Fatalf("expected 2 stops, got %d", len(stops))
	}
	if stops[0].Name != "Abando" {
		t.Errorf("expected Abando, got %s", stops[0].Name)
	}
}

func TestStopService_FindNearby_ClampLimit(t *testing.T) {
	called := false
	repo := &mockStopRepo{
		findNearbyFn: func(ctx context.Context, lat, lon, radius float64, limit int) ([]domain.Stop, error) {
			called = true
			if limit != 50 {
				t.Errorf("expected limit clamped to 50, got %d", limit)
			}
			return nil, nil
		},
	}

	svc := usecases.NewStopService(repo, nil)
	_, _ = svc.FindNearby(context.Background(), 43.0, -2.0, 500, 999)
	if !called {
		t.Error("repo was not called")
	}
}

func TestStopService_Search_EmptyQuery(t *testing.T) {
	svc := usecases.NewStopService(&mockStopRepo{}, nil)
	_, err := svc.Search(context.Background(), "", nil, 10)
	if err == nil {
		t.Error("expected error for empty query")
	}
}

func TestStopService_Search_Success(t *testing.T) {
	repo := &mockStopRepo{
		searchFn: func(ctx context.Context, query string, near *domain.GeoPoint, limit int) ([]domain.Stop, error) {
			if query != "Abando" {
				t.Errorf("expected query 'Abando', got '%s'", query)
			}
			return []domain.Stop{
				{ID: "1", Name: "Abando Indalecio Prieto"},
			}, nil
		},
	}

	svc := usecases.NewStopService(repo, nil)
	stops, err := svc.Search(context.Background(), "Abando", nil, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stops) != 1 {
		t.Fatalf("expected 1 stop, got %d", len(stops))
	}
}

func TestStopService_GetByID(t *testing.T) {
	repo := &mockStopRepo{
		getByIDFn: func(ctx context.Context, id string) (*domain.Stop, error) {
			return &domain.Stop{ID: id, Name: "Test Stop"}, nil
		},
	}

	svc := usecases.NewStopService(repo, nil)
	stop, err := svc.GetByID(context.Background(), "abc-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stop.ID != "abc-123" {
		t.Errorf("expected id abc-123, got %s", stop.ID)
	}
}
