package integrations

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"govite/internal/app"
	"govite/internal/auth"
	"govite/internal/httpx"
	"govite/internal/rbac"
	"govite/internal/store"
)

var ErrKeyNotFound = httpx.E(404, "API_KEY_NOT_FOUND", "API key not found")

type apiKey struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	IsActive   bool       `json:"isActive"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
	CreatedAt  time.Time  `json:"createdAt"`
}

func toKey(k store.ApiKey) apiKey {
	return apiKey{k.ID, k.Name, k.Prefix, k.IsActive, k.LastUsedAt, k.CreatedAt}
}

// HashAPIKey is how keys are stored; verify an incoming key by hashing it.
func HashAPIKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func (s *Service) APIKeyRoutes(g rbac.Guarded) {
	g.Get("/api-keys", "apiKeys.manage", func(c *fiber.Ctx) error {
		rows, err := s.Q.ListAPIKeys(c.UserContext())
		if err != nil {
			return err
		}
		out := make([]apiKey, len(rows))
		for i, k := range rows {
			out[i] = toKey(k)
		}
		return httpx.OK(c, out)
	})

	g.Post("/api-keys", "apiKeys.manage", func(c *fiber.Ctx) error {
		var b struct {
			Name string `json:"name" validate:"required,min=1,max=100"`
		}
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		secret := "rk_" + auth.RandomToken(32)
		k, err := s.Q.CreateAPIKey(c.UserContext(), store.CreateAPIKeyParams{ID: app.NewID(), Name: strings.TrimSpace(b.Name), Prefix: secret[:10], KeyHash: HashAPIKey(secret)})
		if err != nil {
			return err
		}
		s.Audit.From(c, "api_key.create", "api_key", k.ID, nil, toKey(k))
		// The plaintext key is shown exactly once.
		return httpx.Created(c, struct {
			apiKey
			Key string `json:"key"`
		}{toKey(k), secret})
	})

	g.Patch("/api-keys/:id", "apiKeys.manage", func(c *fiber.Ctx) error {
		id := c.Params("id")
		if _, err := uuid.Parse(id); err != nil {
			return httpx.Validation("id: must be a UUID")
		}
		var b struct {
			Name     *string `json:"name" validate:"omitempty,min=1,max=100"`
			IsActive *bool   `json:"isActive"`
		}
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		k, err := s.Q.UpdateAPIKey(c.UserContext(), store.UpdateAPIKeyParams{ID: id, Name: httpx.TrimPtr(b.Name), IsActive: b.IsActive})
		if err != nil {
			return ErrKeyNotFound
		}
		s.Audit.From(c, "api_key.update", "api_key", id, nil, toKey(k))
		return httpx.OK(c, toKey(k))
	})

	g.Delete("/api-keys/:id", "apiKeys.manage", func(c *fiber.Ctx) error {
		id := c.Params("id")
		if _, err := uuid.Parse(id); err != nil {
			return httpx.Validation("id: must be a UUID")
		}
		n, err := s.Q.DeleteAPIKey(c.UserContext(), id)
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrKeyNotFound
		}
		s.Audit.From(c, "api_key.delete", "api_key", id, nil, nil)
		return httpx.NoContent(c)
	})
}
