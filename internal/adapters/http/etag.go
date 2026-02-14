package http

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/gofiber/fiber/v2"
)

// ETagMiddleware computes a weak ETag from the response body
// and returns 304 Not Modified if the client already has it.
func ETagMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Process request first
		if err := c.Next(); err != nil {
			return err
		}

		// Only apply to successful GET responses with a body
		if c.Method() != fiber.MethodGet || c.Response().StatusCode() != 200 {
			return nil
		}

		body := c.Response().Body()
		if len(body) == 0 {
			return nil
		}

		// Compute weak ETag from SHA-256 of body (first 16 hex chars)
		h := sha256.Sum256(body)
		etag := `W/"` + hex.EncodeToString(h[:8]) + `"`

		c.Set("ETag", etag)

		// Check If-None-Match
		ifNoneMatch := c.Get("If-None-Match")
		if ifNoneMatch == etag {
			c.Status(304)
			c.Response().ResetBody()
			return nil
		}

		return nil
	}
}
