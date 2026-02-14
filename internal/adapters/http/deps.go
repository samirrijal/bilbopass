package http

import (
	"github.com/nats-io/nats.go"
	"github.com/samirrijal/bilbopass/internal/adapters/postgres"
	"github.com/samirrijal/bilbopass/internal/adapters/valkey"
	"github.com/samirrijal/bilbopass/internal/core/usecases"
)

// Dependencies holds all services needed by HTTP handlers.
type Dependencies struct {
	Stops         *usecases.StopService
	Routes        *usecases.RouteService
	Agencies      *usecases.AgencyService
	Departures    *usecases.DepartureService
	Trips         *usecases.TripService
	Journeys      *usecases.JourneyService
	Realtime      *usecases.RealtimeService
	Compensations *usecases.CompensationService
	NATS          *nats.Conn
	DB            *postgres.DB
	Cache         *valkey.Cache
}
