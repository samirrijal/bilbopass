package usecases_test

import (
	"context"
	"testing"

	"github.com/samirrijal/bilbopass/internal/core/domain"
	"github.com/samirrijal/bilbopass/internal/core/usecases"
)

// --- Mock TripRepository ---

type mockTripRepo struct {
	nextDeparturesFn func(ctx context.Context, stopUUID string, limit int) ([]domain.Departure, error)
}

func (m *mockTripRepo) Upsert(ctx context.Context, trip *domain.Trip) error             { return nil }
func (m *mockTripRepo) UpsertBatch(ctx context.Context, trips []domain.Trip) error      { return nil }
func (m *mockTripRepo) GetByID(ctx context.Context, id string) (*domain.Trip, error)    { return nil, nil }
func (m *mockTripRepo) UpsertStopTimes(ctx context.Context, st []domain.StopTime) error { return nil }
func (m *mockTripRepo) GetStopTimes(ctx context.Context, tripID string) ([]domain.StopTime, error) {
	return nil, nil
}

func (m *mockTripRepo) NextDeparturesAtStop(ctx context.Context, stopUUID string, limit int) ([]domain.Departure, error) {
	if m.nextDeparturesFn != nil {
		return m.nextDeparturesFn(ctx, stopUUID, limit)
	}
	return nil, nil
}

func TestDepartureService_NextDepartures(t *testing.T) {
	repo := &mockTripRepo{
		nextDeparturesFn: func(ctx context.Context, stopUUID string, limit int) ([]domain.Departure, error) {
			return []domain.Departure{
				{Trip: &domain.Trip{TripID: "trip1", Headsign: "Basauri"}, Platform: "1"},
				{Trip: &domain.Trip{TripID: "trip2", Headsign: "Etxebarri"}, Platform: "2"},
			}, nil
		},
	}

	svc := usecases.NewDepartureService(repo)
	deps, err := svc.NextDeparturesAtStop(context.Background(), "stop-uuid", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deps) != 2 {
		t.Fatalf("expected 2 departures, got %d", len(deps))
	}
	if deps[0].Trip.Headsign != "Basauri" {
		t.Errorf("expected Basauri, got %s", deps[0].Trip.Headsign)
	}
}

func TestDepartureService_ClampLimit(t *testing.T) {
	repo := &mockTripRepo{
		nextDeparturesFn: func(ctx context.Context, stopUUID string, limit int) ([]domain.Departure, error) {
			if limit != 10 {
				t.Errorf("expected limit clamped to 10, got %d", limit)
			}
			return nil, nil
		},
	}

	svc := usecases.NewDepartureService(repo)
	_, _ = svc.NextDeparturesAtStop(context.Background(), "stop-uuid", -5)
}

func TestDepartureService_MaxLimit(t *testing.T) {
	repo := &mockTripRepo{
		nextDeparturesFn: func(ctx context.Context, stopUUID string, limit int) ([]domain.Departure, error) {
			if limit != 10 {
				t.Errorf("expected limit clamped to 10, got %d", limit)
			}
			return nil, nil
		},
	}

	svc := usecases.NewDepartureService(repo)
	_, _ = svc.NextDeparturesAtStop(context.Background(), "stop-uuid", 100)
}
