package http

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/gofiber/fiber/v2/middleware/timeout"
	"github.com/gofiber/websocket/v2"
	"github.com/samirrijal/bilbopass/internal/pkg/metrics"
)

// SetupRoutes registers all REST, GraphQL, and WebSocket routes.
func SetupRoutes(app *fiber.App, deps *Dependencies) {
	// Prometheus metrics
	app.Use(metrics.Middleware())
	app.Get("/metrics", metrics.Handler())

	// Response compression (gzip)
	app.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed, // Balance speed vs compression ratio
	}))

	// Request ID
	app.Use(requestid.New())

	// Propagate request ID into slog context
	app.Use(RequestIDLogMiddleware())

	// Access logs (structured HTTP request logging)
	app.Use(AccessLogMiddleware())

	// Rate limiting: 120 requests per minute per IP
	app.Use(limiter.New(limiter.Config{
		Max:        120,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(429).JSON(fiber.Map{
				"error":   "rate limit exceeded",
				"message": "too many requests, please try again later",
			})
		},
		SkipFailedRequests: false,
	}))

	// Security headers + API version
	app.Use(func(c *fiber.Ctx) error {
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-XSS-Protection", "1; mode=block")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Set("X-API-Version", "1.0.0")
		return c.Next()
	})

	// ETag for conditional caching
	app.Use(ETagMiddleware())

	// Default Cache-Control headers
	app.Use(CachingMiddleware())

	// Health & readiness (no timeout — fast internal checks)
	app.Get("/v1/health", HealthHandler(deps))
	app.Get("/v1/ready", ReadyHandler(deps))

	// REST API v1 — 15s per-request timeout
	v1 := app.Group("/v1")
	v1.Get("/agencies", timeout.NewWithContext(ListAgenciesHandler(deps), 15*time.Second))
	v1.Get("/agencies/:slug", timeout.NewWithContext(GetAgencyHandler(deps), 15*time.Second))
	v1.Get("/agencies/:slug/routes", timeout.NewWithContext(AgencyRoutesHandler(deps), 15*time.Second))
	v1.Get("/stops/nearby", timeout.NewWithContext(NearbyStopsHandler(deps), 15*time.Second))
	v1.Get("/stops/search", timeout.NewWithContext(SearchStopsHandler(deps), 15*time.Second))
	v1.Get("/stops/batch", timeout.NewWithContext(BatchStopsHandler(deps), 15*time.Second))
	v1.Get("/stops/:id", timeout.NewWithContext(GetStopHandler(deps), 15*time.Second))
	v1.Get("/stops/:id/departures", timeout.NewWithContext(StopDeparturesHandler(deps), 15*time.Second))
	v1.Get("/stops/:id/routes", timeout.NewWithContext(StopRoutesHandler(deps), 15*time.Second))
	v1.Get("/routes", timeout.NewWithContext(ListRoutesHandler(deps), 15*time.Second))
	v1.Get("/routes/:id", timeout.NewWithContext(GetRouteHandler(deps), 15*time.Second))
	v1.Get("/routes/:id/vehicles", timeout.NewWithContext(GetRouteVehiclesHandler(deps), 15*time.Second))
	v1.Get("/trips/:id", timeout.NewWithContext(GetTripHandler(deps), 15*time.Second))
	v1.Get("/trips/:id/stop-times", timeout.NewWithContext(TripStopTimesHandler(deps), 15*time.Second))
	v1.Get("/feeds/status", timeout.NewWithContext(FeedStatsHandler(deps), 15*time.Second))

	// Journey planner (from/to)
	v1.Get("/journeys", timeout.NewWithContext(JourneyHandler(deps), 15*time.Second))

	// Enriched endpoints
	v1.Get("/agencies/:slug/stats", timeout.NewWithContext(AgencyStatsHandler(deps), 15*time.Second))
	v1.Get("/routes/:id/stops", timeout.NewWithContext(RouteStopsHandler(deps), 15*time.Second))

	// GraphQL
	app.Post("/graphql", GraphQLHandler(deps))

	// API documentation (Swagger UI)
	SetupDocs(app)

	// WebSocket
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	app.Get("/ws", websocket.New(WebSocketHandler(deps.NATS)))
}
