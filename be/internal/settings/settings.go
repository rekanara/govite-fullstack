// Package settings stores the single app_setting row "app".
package settings

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"

	"govite/internal/app"
	"govite/internal/audit"
	"govite/internal/httpx"
	"govite/internal/mail"
	"govite/internal/rbac"
	"govite/internal/storage"
	"govite/internal/store"
)

const rowID = "app"

// Settings mirrors AppSettingsSchema in fe/packages/contracts.
type Settings struct {
	AppName                  string   `json:"appName" validate:"required,min=1,max=100"`
	Tagline                  *string  `json:"tagline" validate:"omitempty,max=150"`
	Description              *string  `json:"description" validate:"omitempty,max=1000"`
	SupportEmail             *string  `json:"supportEmail" validate:"omitempty,email"`
	SupportPhone             *string  `json:"supportPhone" validate:"omitempty,max=30"`
	LogoURL                  *string  `json:"logoUrl" validate:"omitempty,url"`
	BannerURL                *string  `json:"bannerUrl" validate:"omitempty,url"`
	LoginNotice              *string  `json:"loginNotice" validate:"omitempty,max=500"`
	EmailFooter              *string  `json:"emailFooter" validate:"omitempty,max=1000"`
	Timezone                 string   `json:"timezone" validate:"required,min=1,max=64"`
	Currency                 string   `json:"currency" validate:"required,len=3"`
	Address                  *string  `json:"address" validate:"omitempty,max=200"`
	District                 *string  `json:"district" validate:"omitempty,max=100"`
	City                     *string  `json:"city" validate:"omitempty,max=100"`
	Province                 *string  `json:"province" validate:"omitempty,max=100"`
	Country                  *string  `json:"country" validate:"omitempty,max=100"`
	PostalCode               *string  `json:"postalCode" validate:"omitempty,max=20"`
	Latitude                 *float64 `json:"latitude" validate:"omitempty,min=-90,max=90"`
	Longitude                *float64 `json:"longitude" validate:"omitempty,min=-180,max=180"`
	MaintenanceRetentionDays int      `json:"maintenanceRetentionDays" validate:"min=7,max=3650"`
	PushNotificationsEnabled bool     `json:"pushNotificationsEnabled"`
}

func defaults() Settings {
	return Settings{AppName: "Rekanara", Timezone: "Asia/Jakarta", Currency: "IDR", MaintenanceRetentionDays: 90, PushNotificationsEnabled: true}
}

// normalize trims strings and turns empty optionals into null.
func (s *Settings) normalize() {
	s.AppName, s.Timezone = strings.TrimSpace(s.AppName), strings.TrimSpace(s.Timezone)
	s.Currency = strings.ToUpper(strings.TrimSpace(s.Currency))
	for _, p := range []**string{&s.Tagline, &s.Description, &s.SupportEmail, &s.SupportPhone, &s.LogoURL, &s.BannerURL, &s.LoginNotice, &s.EmailFooter, &s.Address, &s.District, &s.City, &s.Province, &s.Country, &s.PostalCode} {
		if *p != nil {
			if t := strings.TrimSpace(**p); t == "" {
				*p = nil
			} else {
				*p = &t
			}
		}
	}
}

type Service struct {
	*app.Deps
	Audit *audit.Log
}

// Get returns stored settings over defaults; missing keys keep defaults.
func (s *Service) Get(ctx context.Context) (Settings, error) {
	v := defaults()
	raw, err := s.Q.GetSetting(ctx, rowID)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, nil
	}
	if err != nil {
		return v, err
	}
	_ = json.Unmarshal(raw, &v)
	return v, nil
}

// Brand feeds the mailer; falls back to defaults on any error.
func (s *Service) Brand(ctx context.Context) mail.Brand {
	v, _ := s.Get(ctx)
	str := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	var addr []string
	for _, p := range []*string{v.Address, v.District, v.City, v.Province, v.Country, v.PostalCode} {
		if p != nil {
			addr = append(addr, *p)
		}
	}
	return mail.Brand{AppName: v.AppName, Tagline: str(v.Tagline), LogoURL: str(v.LogoURL), SupportEmail: str(v.SupportEmail), SupportPhone: str(v.SupportPhone), EmailFooter: str(v.EmailFooter), Address: strings.Join(addr, ", ")}
}

// PublicRoutes: branding shown before sign-in.
func (s *Service) PublicRoutes(r fiber.Router) {
	r.Get("/settings/public", func(c *fiber.Ctx) error {
		v, err := s.Get(c.UserContext())
		if err != nil {
			return err
		}
		return httpx.OK(c, fiber.Map{"appName": v.AppName, "tagline": v.Tagline, "logoUrl": v.LogoURL, "bannerUrl": v.BannerURL, "supportEmail": v.SupportEmail, "loginNotice": v.LoginNotice})
	})
}

func (s *Service) Routes(g rbac.Guarded) {
	g.Get("/settings", "settings.manage", func(c *fiber.Ctx) error {
		v, err := s.Get(c.UserContext())
		if err != nil {
			return err
		}
		return httpx.OK(c, v)
	})

	g.Put("/settings", "settings.manage", func(c *fiber.Ctx) error {
		v := defaults()
		if err := json.Unmarshal(c.Body(), &v); err != nil {
			return httpx.Validation("body: invalid JSON")
		}
		v.normalize()
		if err := httpx.Check(&v); err != nil {
			return err
		}
		previous, _ := s.Get(c.UserContext())
		raw, _ := json.Marshal(v)
		if err := s.Q.PutSetting(c.UserContext(), store.PutSettingParams{ID: rowID, Value: raw}); err != nil {
			return err
		}
		if previous.LogoURL != nil && (v.LogoURL == nil || *previous.LogoURL != *v.LogoURL) {
			s.Files.RemoveOwned(c.UserContext(), *previous.LogoURL, "branding")
		}
		if previous.BannerURL != nil && (v.BannerURL == nil || *previous.BannerURL != *v.BannerURL) {
			s.Files.RemoveOwned(c.UserContext(), *previous.BannerURL, "branding")
		}
		s.Audit.From(c, "settings.update", "settings", rowID, previous, v)
		return httpx.OK(c, v)
	})

	g.Post("/settings/image", "settings.manage", func(c *fiber.Ctx) error {
		_, url, err := s.Files.UploadImage(c, "image", "branding", "", storage.MaxImageBytes)
		if err != nil {
			return err
		}
		return httpx.Created(c, fiber.Map{"url": url})
	})
}
