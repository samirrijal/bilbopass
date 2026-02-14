package usecases

import (
	"context"

	"github.com/samirrijal/bilbopass/internal/core/domain"
	"github.com/samirrijal/bilbopass/internal/core/ports"
)

// DepartureService computes next departures at a stop.
type DepartureService struct {
	trips ports.TripRepository
}

// NewDepartureService creates a new DepartureService.
func NewDepartureService(trips ports.TripRepository) *DepartureService {
	return &DepartureService{trips: trips}
}

// NextDeparturesAtStop returns the next scheduled departures at a stop.
func (s *DepartureService) NextDeparturesAtStop(ctx context.Context, stopUUID string, limit int) ([]domain.Departure, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	return s.trips.NextDeparturesAtStop(ctx, stopUUID, limit)
}
