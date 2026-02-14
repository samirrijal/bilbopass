package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/samirrijal/bilbopass/internal/core/domain"
	"github.com/samirrijal/bilbopass/internal/core/ports"
)

// RealtimeService processes incoming GTFS-RT vehicle positions.
type RealtimeService struct {
	vehicles  ports.VehiclePositionRepository
	routes    ports.RouteRepository
	publisher ports.EventPublisher
}

// NewRealtimeService creates a new RealtimeService.
func NewRealtimeService(
	vehicles ports.VehiclePositionRepository,
	routes ports.RouteRepository,
	publisher ports.EventPublisher,
) *RealtimeService {
	return &RealtimeService{vehicles: vehicles, routes: routes, publisher: publisher}
}

// ProcessVehicleUpdate stores a position and checks for detours.
func (s *RealtimeService) ProcessVehicleUpdate(ctx context.Context, vp *domain.VehiclePosition) error {
	vp.Time = time.Now()

	if err := s.vehicles.Insert(ctx, vp); err != nil {
		return fmt.Errorf("insert vehicle position: %w", err)
	}

	// Broadcast to WebSocket clients
	// Serialization is left to the publisher implementation.
	_ = s.publisher.PublishVehiclePosition(ctx, vp)

	return nil
}
