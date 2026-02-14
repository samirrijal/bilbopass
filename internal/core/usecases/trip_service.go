package usecases

import (
	"context"

	"github.com/samirrijal/bilbopass/internal/core/domain"
	"github.com/samirrijal/bilbopass/internal/core/ports"
)

// TripService handles trip lookups.
type TripService struct {
	trips ports.TripRepository
}

// NewTripService creates a new TripService.
func NewTripService(trips ports.TripRepository) *TripService {
	return &TripService{trips: trips}
}

// GetByID returns a single trip by its UUID.
func (s *TripService) GetByID(ctx context.Context, id string) (*domain.Trip, error) {
	return s.trips.GetByID(ctx, id)
}

// GetStopTimes returns the ordered stop-times for a trip.
func (s *TripService) GetStopTimes(ctx context.Context, tripID string) ([]domain.StopTime, error) {
	return s.trips.GetStopTimes(ctx, tripID)
}
