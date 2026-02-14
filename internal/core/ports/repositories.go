package ports

import (
	"context"
	"time"

	"github.com/samirrijal/bilbopass/internal/core/domain"
)

// AgencyRepository persists agencies.
type AgencyRepository interface {
	Upsert(ctx context.Context, agency *domain.Agency) error
	GetBySlug(ctx context.Context, slug string) (*domain.Agency, error)
	List(ctx context.Context) ([]domain.Agency, error)
}

// StopRepository persists stops.
type StopRepository interface {
	Upsert(ctx context.Context, stop *domain.Stop) error
	UpsertBatch(ctx context.Context, stops []domain.Stop) error
	GetByID(ctx context.Context, id string) (*domain.Stop, error)
	GetByIDs(ctx context.Context, ids []string) ([]domain.Stop, error)
	FindNearby(ctx context.Context, lat, lon, radiusMeters float64, limit int) ([]domain.Stop, error)
	Search(ctx context.Context, query string, near *domain.GeoPoint, limit int) ([]domain.Stop, error)
}

// RouteRepository persists routes.
type RouteRepository interface {
	Upsert(ctx context.Context, route *domain.Route) error
	UpsertBatch(ctx context.Context, routes []domain.Route) error
	GetByID(ctx context.Context, id string) (*domain.Route, error)
	ListByAgency(ctx context.Context, agencyID string) ([]domain.Route, error)
	ListByStop(ctx context.Context, stopUUID string) ([]domain.Route, error)
}

// TripRepository persists trips and stop-times.
type TripRepository interface {
	Upsert(ctx context.Context, trip *domain.Trip) error
	UpsertBatch(ctx context.Context, trips []domain.Trip) error
	GetByID(ctx context.Context, id string) (*domain.Trip, error)
	UpsertStopTimes(ctx context.Context, stopTimes []domain.StopTime) error
	GetStopTimes(ctx context.Context, tripID string) ([]domain.StopTime, error)
	NextDeparturesAtStop(ctx context.Context, stopUUID string, limit int) ([]domain.Departure, error)
}

// VehiclePositionRepository persists real-time vehicle positions.
type VehiclePositionRepository interface {
	Insert(ctx context.Context, vp *domain.VehiclePosition) error
	InsertBatch(ctx context.Context, vps []domain.VehiclePosition) error
	LatestByRoute(ctx context.Context, routeID string) ([]domain.VehiclePosition, error)
}

// DelayEventRepository persists delay events.
type DelayEventRepository interface {
	Insert(ctx context.Context, event *domain.DelayEvent) error
	GetByID(ctx context.Context, id string) (*domain.DelayEvent, error)
	MarkCompensated(ctx context.Context, id string) error
}

// AffiliateRepository persists affiliate shops.
type AffiliateRepository interface {
	FindNearby(ctx context.Context, lat, lon float64, limit int) ([]domain.Affiliate, error)
	GetByID(ctx context.Context, id string) (*domain.Affiliate, error)
}

// CompensationRepository persists compensation coupons.
type CompensationRepository interface {
	Create(ctx context.Context, comp *domain.Compensation) error
	GetByCode(ctx context.Context, code string) (*domain.Compensation, error)
	Redeem(ctx context.Context, code string) error
	Delete(ctx context.Context, code string) error
}

// JourneyRepository finds routes between stops.
type JourneyRepository interface {
	// FindJourneys returns possible journeys from one stop to another at a given time.
	FindJourneys(ctx context.Context, fromStopID, toStopID string, departAfter time.Time, maxTransfers int, limit int) ([]domain.Journey, error)
}
