package http

import (
	"os"

	"github.com/gofiber/fiber/v2"
)

const swaggerUIHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>BilboPass API — Swagger UI</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css">
  <style>html{box-sizing:border-box}*,*::before,*::after{box-sizing:inherit}body{margin:0;background:#fafafa}</style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    SwaggerUIBundle({
      url: '/docs/openapi.yaml',
      dom_id: '#swagger-ui',
      deepLinking: true,
      presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.SwaggerUIStandalonePreset],
      layout: 'BaseLayout',
    });
  </script>
</body>
</html>`

// SetupDocs registers Swagger UI at /docs and the raw OpenAPI spec at /docs/openapi.yaml.
func SetupDocs(app *fiber.App) {
	app.Get("/docs", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.SendString(swaggerUIHTML)
	})

	app.Get("/docs/openapi.yaml", func(c *fiber.Ctx) error {
		data, err := os.ReadFile("api/openapi.yaml")
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "openapi.yaml not found"})
		}
		c.Set("Content-Type", "application/yaml")
		return c.Send(data)
	})
}
