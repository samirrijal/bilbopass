package http

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

// CachingMiddleware sets Cache-Control headers on GET responses based on endpoint.
// Adds sensible defaults if not already set by the handler.
func CachingMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		err := c.Next()

		// Only set on GET requests
		if c.Method() != "GET" {
			return err
		}

		// Don't override if already set
		if existing := c.Get("Cache-Control"); existing != "" {
			return err
		}

		path := c.Path()
		var ttl string

		// Default cache times by endpoint pattern
		switch {
		case path == "/v1/health" || path == "/v1/ready":
			ttl = "public, max-age=10" // Very short for system checks

		case strings.HasPrefix(path, "/v1/agencies"):
			ttl = "public, max-age=3600" // 1 hour for stable data

		case path == "/metrics":
			ttl = "no-cache" // Metrics are real-time

		case path == "/graphql":
			ttl = "private, max-age=0" // GraphQL varies wildly

		case strings.HasPrefix(path, "/v1/stops/nearby"):
			ttl = "public, max-age=300" // 5 min for location queries

		case strings.HasPrefix(path, "/v1/stops/search"):
			ttl = "public, max-age=300" // 5 min for search results

		case strings.Contains(path, "/stops/") && strings.Contains(path, "/"):
			ttl = "public, max-age=600" // 10 min for single stop

		case strings.Contains(path, "/routes/") && strings.Contains(path, "/"):
			ttl = "public, max-age=600" // 10 min for single route

		case strings.Contains(path, "/trips/") && strings.Contains(path, "/"):
			ttl = "public, max-age=600" // 10 min for single trip

		case path == "/v1/feeds/status":
			ttl = "public, max-age=60" // Feed stats: 1 min

		case strings.HasPrefix(path, "/v1/"):
			ttl = "public, max-age=300" // 5 min default for API endpoints
		}

		if ttl != "" {
			c.Set("Cache-Control", ttl)
		}

		return err
	}
}
