package http

import (
	"context"
	"log/slog"

	"github.com/gofiber/fiber/v2"
)

type ctxKey string

const requestIDKey ctxKey = "request_id"

// RequestIDLogMiddleware copies the Fiber request ID into the context so that
// slog.Default().With("request_id", ...) can be used downstream, and also
// injects a per-request *slog.Logger into the context via slog attributes.
func RequestIDLogMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		rid := c.Locals("requestid")
		if rid == nil {
			return c.Next()
		}

		ridStr, ok := rid.(string)
		if !ok || ridStr == "" {
			return c.Next()
		}

		// Build a request-scoped logger with the request ID baked in.
		reqLogger := slog.Default().With("request_id", ridStr)

		// Store logger in Go context so usecases/repos can retrieve it.
		ctx := context.WithValue(c.Context(), requestIDKey, ridStr)
		ctx = context.WithValue(ctx, ctxKey("logger"), reqLogger)
		c.SetUserContext(ctx)

		return c.Next()
	}
}

// LoggerFromCtx extracts the per-request slog.Logger from a context.
// Falls back to the default logger if none is set.
func LoggerFromCtx(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKey("logger")).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}
