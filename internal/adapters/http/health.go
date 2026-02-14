package http

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
)

// HealthHandler returns a basic liveness check.
func HealthHandler(deps *Dependencies) fiber.Handler {
	startedAt := time.Now()

	return func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "healthy",
			"uptime":  time.Since(startedAt).String(),
			"version": "dev",
		})
	}
}

// ReadyHandler checks DB, NATS, and cache connectivity.
func ReadyHandler(deps *Dependencies) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), 3*time.Second)
		defer cancel()

		checks := make(map[string]string)
		allOK := true

		// Database
		if deps.DB != nil {
			if err := deps.DB.Pool.Ping(ctx); err != nil {
				checks["database"] = "error: " + err.Error()
				allOK = false
			} else {
				checks["database"] = "ok"
			}
		} else {
			checks["database"] = "not configured"
			allOK = false
		}

		// NATS
		if deps.NATS != nil {
			if deps.NATS.IsConnected() {
				checks["nats"] = "ok"
			} else {
				checks["nats"] = "disconnected"
				allOK = false
			}
		} else {
			checks["nats"] = "not configured"
		}

		// Valkey cache
		if deps.Cache != nil {
			_, err := deps.Cache.Get(ctx, "__health_check__")
			// "valkey nil message" is expected for a missing key — that's fine
			if err != nil && err.Error() != "valkey nil message" {
				checks["cache"] = "error: " + err.Error()
				allOK = false
			} else {
				checks["cache"] = "ok"
			}
		} else {
			checks["cache"] = "not configured"
		}

		status := "ready"
		code := 200
		if !allOK {
			status = "not ready"
			code = 503
		}

		return c.Status(code).JSON(fiber.Map{
			"status": status,
			"checks": checks,
		})
	}
}
