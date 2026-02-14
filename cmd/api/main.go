package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/samirrijal/bilbopass/internal/adapters/http"
	natsadapter "github.com/samirrijal/bilbopass/internal/adapters/nats"
	"github.com/samirrijal/bilbopass/internal/adapters/postgres"
	"github.com/samirrijal/bilbopass/internal/adapters/valkey"
	"github.com/samirrijal/bilbopass/internal/core/usecases"
	"github.com/samirrijal/bilbopass/internal/pkg/config"
	"github.com/samirrijal/bilbopass/internal/pkg/logging"
	"github.com/samirrijal/bilbopass/internal/pkg/telemetry"
)

func main() {
	cfg, err := config.Load("bilbopass-api")
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// Structured logging
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	logging.Setup(logLevel, "json")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Telemetry
	if cfg.Telemetry.Enabled {
		shutdown, err := telemetry.InitTracer(ctx, cfg.Telemetry.ServiceName, cfg.Telemetry.TempoAddr)
		if err != nil {
			slog.Warn("telemetry init failed", "error", err)
		} else {
			defer shutdown()
		}
	}

	// Database
	db, err := postgres.New(ctx, cfg.Database.DSN())
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	// Cache
	cache, err := valkey.New(cfg.Valkey.Addr)
	if err != nil {
		slog.Warn("valkey unavailable", "error", err)
	} else {
		defer cache.Close()
	}

	// NATS
	nc, err := natsadapter.NewPublisher(cfg.NATS.URL)
	if err != nil {
		slog.Warn("nats unavailable", "error", err)
	} else {
		defer nc.Close()
	}

	// Raw NATS connection for WebSocket relay
	natsConn, err := natsadapter.RawConn(cfg.NATS.URL)
	if err != nil {
		slog.Warn("nats ws conn unavailable", "error", err)
	}

	// Repos
	agencyRepo := postgres.NewAgencyRepo(db)
	stopRepo := postgres.NewStopRepo(db)
	routeRepo := postgres.NewRouteRepo(db)
	vehicleRepo := postgres.NewVehiclePositionRepo(db)
	tripRepo := postgres.NewTripRepo(db)
	journeyRepo := postgres.NewJourneyRepo(db)

	// Use cases
	agencySvc := usecases.NewAgencyService(agencyRepo)
	stopSvc := usecases.NewStopService(stopRepo, cache)
	routeSvc := usecases.NewRouteService(routeRepo, vehicleRepo)
	departureSvc := usecases.NewDepartureService(tripRepo)
	tripSvc := usecases.NewTripService(tripRepo)
	realtimeSvc := usecases.NewRealtimeService(vehicleRepo, routeRepo, nc)
	journeySvc := usecases.NewJourneyService(journeyRepo, stopRepo)

	deps := &http.Dependencies{
		Agencies:   agencySvc,
		Stops:      stopSvc,
		Routes:     routeSvc,
		Departures: departureSvc,
		Trips:      tripSvc,
		Realtime:   realtimeSvc,
		Journeys:   journeySvc,
		NATS:       natsConn,
		DB:         db,
		Cache:      cache,
	}

	// Fiber
	app := fiber.New(fiber.Config{
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		BodyLimit:    1024 * 1024, // 1 MB max request body
		AppName:      "BilboPass API",
	})
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "http://localhost:3000, http://localhost:5173, https://*.bilbopass.eus",
		AllowMethods:     "GET,POST,OPTIONS",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowCredentials: false,
		MaxAge:           3600,
	}))

	http.SetupRoutes(app, deps)

	// Graceful shutdown
	go func() {
		addr := fmt.Sprintf(":%d", cfg.Server.Port)
		slog.Info("API server starting", "addr", addr)
		if err := app.Listen(addr); err != nil {
			log.Fatalf("listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	slog.Info("shutdown signal received, draining connections...", "signal", sig.String())

	// Give in-flight requests up to 10s to complete
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		slog.Error("forced shutdown", "error", err)
	}

	slog.Info("server stopped")
}
