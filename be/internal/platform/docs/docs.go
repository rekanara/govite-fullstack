// Package docs embeds and serves the OpenAPI contract and Swagger UI.
package docs

import (
	_ "embed"
	"encoding/json"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/swaggest/swgui/v5emb"
	"go.yaml.in/yaml/v3"
)

//go:embed openapi.json
var spec []byte

// specYAML is the same contract rendered as YAML, built once at first use.
var specYAML = sync.OnceValues(func() ([]byte, error) {
	var doc any
	if err := json.Unmarshal(spec, &doc); err != nil {
		return nil, err
	}
	return yaml.Marshal(doc)
})

// Routes mounts docs outside the /api/v1 prefix.
func Routes(app *fiber.App) {
	app.Get("/api/docs-json", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
		return c.Send(spec)
	})
	app.Get("/api/docs-yaml", func(c *fiber.Ctx) error {
		out, err := specYAML()
		if err != nil {
			return err
		}
		c.Set(fiber.HeaderContentType, "application/yaml; charset=utf-8")
		return c.Send(out)
	})

	ui := adaptor.HTTPHandler(v5emb.New("Govite Fullstack API", "/api/docs-json", "/api/docs"))
	app.Get("/api/docs", ui)
	app.All("/api/docs/*", ui)
}
