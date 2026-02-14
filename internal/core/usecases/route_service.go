package usecases

import (
	"context"

	"github.com/samirrijal/bilbopass/internal/core/domain"
	"github.com/samirrijal/bilbopass/internal/core/ports"
)

// RouteService handles route-related business logic.
type RouteService struct {
	routes   ports.RouteRepository
	vehicles ports.VehiclePositionRepository
}

// NewRouteService creates a new RouteService.
func NewRouteService(routes ports.RouteRepository, vehicles ports.VehiclePositionRepository) *RouteService {
	return &RouteService{routes: routes, vehicles: vehicles}
}

// GetByID returns a route by its UUID.
func (s *RouteService) GetByID(ctx context.Context, id string) (*domain.Route, error) {
	return s.routes.GetByID(ctx, id)
}

// ListByAgency returns all routes for a given agency.
func (s *RouteService) ListByAgency(ctx context.Context, agencyID string) ([]domain.Route, error) {
	return s.routes.ListByAgency(ctx, agencyID)
}

// GetLiveVehicles returns the latest vehicle positions on a route.
func (s *RouteService) GetLiveVehicles(ctx context.Context, routeID string) ([]domain.VehiclePosition, error) {
	return s.vehicles.LatestByRoute(ctx, routeID)
}

// ListByStop returns the distinct routes that serve a given stop.
func (s *RouteService) ListByStop(ctx context.Context, stopUUID string) ([]domain.Route, error) {
	return s.routes.ListByStop(ctx, stopUUID)
}
