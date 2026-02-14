package http

import "github.com/gofiber/fiber/v2"

// APIError is a structured error response.
type APIError struct {
	Status    int    `json:"status"`
	Code      string `json:"code"`    // Error code: bad_request, not_found, internal_error, etc.
	Message   string `json:"message"` // Human-readable message
	RequestID string `json:"request_id,omitempty"`
}

// newError builds a JSON error response with a request ID.
func newError(c *fiber.Ctx, status int, code string, message string) error {
	reqID, _ := c.Locals("requestid").(string)
	return c.Status(status).JSON(APIError{
		Status:    status,
		Code:      code,
		Message:   message,
		RequestID: reqID,
	})
}

// errBadRequest returns a 400 error.
func errBadRequest(c *fiber.Ctx, msg string) error {
	return newError(c, 400, "bad_request", msg)
}

// errNotFound returns a 404 error.
func errNotFound(c *fiber.Ctx, msg string) error {
	return newError(c, 404, "not_found", msg)
}

// errInternal returns a 500 error.
func errInternal(c *fiber.Ctx, msg string) error {
	return newError(c, 500, "internal_error", msg)
}

// errUnauthorized returns a 401 error.
func errUnauthorized(c *fiber.Ctx, msg string) error {
	return newError(c, 401, "unauthorized", msg)
}

// errForbidden returns a 403 error.
func errForbidden(c *fiber.Ctx, msg string) error {
	return newError(c, 403, "forbidden", msg)
}

// errConflict returns a 409 error.
func errConflict(c *fiber.Ctx, msg string) error {
	return newError(c, 409, "conflict", msg)
}
