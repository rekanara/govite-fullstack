// Package docs embeds and serves the OpenAPI contract and Swagger UI.
package docs

import (
	_ "embed"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/swaggest/swgui/v5emb"
)

//go:embed openapi.json
var spec []byte

// Routes mounts docs outside the /api/v1 prefix. JSON is valid YAML 1.2, so
// docs-yaml can expose the same canonical bytes without a second generated file.
func Routes(app *fiber.App) {
	app.Get("/api/docs-json", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
		return c.Send(spec)
	})
	app.Get("/api/docs-yaml", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, "application/yaml; charset=utf-8")
		return c.Send(spec)
	})

	ui := adaptor.HTTPHandler(v5emb.New("Govite Fullstack API", "/api/docs-json", "/api/docs"))
	app.Get("/api/docs", ui)
	app.All("/api/docs/*", ui)
}
