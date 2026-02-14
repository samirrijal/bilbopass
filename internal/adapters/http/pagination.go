package http

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// PaginatedResponse wraps list results with pagination metadata.
type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Pagination Pagination  `json:"pagination"`
}

// Pagination contains offset-based pagination info.
type Pagination struct {
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
	Total  int `json:"total"`
}

// SetLinkHeaders adds RFC 8288 Link headers for paginated responses.
// It uses the current request path and query parameters.
func SetLinkHeaders(c *fiber.Ctx, p Pagination) {
	base := c.Path()
	var links []string

	// first
	links = append(links, fmt.Sprintf(`<%s?offset=0&limit=%d>; rel="first"`, base, p.Limit))

	// prev
	if p.Offset > 0 {
		prev := p.Offset - p.Limit
		if prev < 0 {
			prev = 0
		}
		links = append(links, fmt.Sprintf(`<%s?offset=%d&limit=%d>; rel="prev"`, base, prev, p.Limit))
	}

	// next
	if p.Offset+p.Limit < p.Total {
		links = append(links, fmt.Sprintf(`<%s?offset=%d&limit=%d>; rel="next"`, base, p.Offset+p.Limit, p.Limit))
	}

	// last
	lastOffset := p.Total - p.Limit
	if lastOffset < 0 {
		lastOffset = 0
	}
	links = append(links, fmt.Sprintf(`<%s?offset=%d&limit=%d>; rel="last"`, base, lastOffset, p.Limit))

	c.Set("Link", strings.Join(links, ", "))
}
