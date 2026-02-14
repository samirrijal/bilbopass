package http

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
)

// AccessLogMiddleware logs HTTP requests with structured slog output.
// Logs: method, path, status, latency, bytes sent, request ID, and error (if any).
func AccessLogMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		path := c.Path()
		method := c.Method()

		// Get request ID if available
		requestID := c.Get(fiber.HeaderXRequestID, "unknown")

		// Call next handler
		err := c.Next()

		// Get response details
		status := c.Response().StatusCode()
		latency := time.Since(start)
		bytesOut := len(c.Response().Body())

		// Log attributes
		attrs := []slog.Attr{
			slog.String("method", method),
			slog.String("path", path),
			slog.Int("status", status),
			slog.String("latency", latency.String()),
			slog.Int("bytes_out", bytesOut),
			slog.String("request_id", requestID),
		}

		// Determine log level based on status code
		level := slog.LevelInfo
		if status >= 500 {
			level = slog.LevelError
		} else if status >= 400 {
			level = slog.LevelWarn
		}

		// Add error if one occurred
		if err != nil {
			attrs = append(attrs, slog.String("error", err.Error()))
			level = slog.LevelError
		}

		// Log the request
		slog.LogAttrs(c.Context(), level, fmt.Sprintf("%s %s", method, path), attrs...)

		return err
	}
}
