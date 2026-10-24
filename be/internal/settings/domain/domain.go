// Package domain is the single app settings document.
package domain

import (
	"context"
	"strings"
)

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

func Defaults() Settings {
	return Settings{AppName: "Rekanara", Timezone: "Asia/Jakarta", Currency: "IDR", MaintenanceRetentionDays: 90, PushNotificationsEnabled: true}
}

// Normalize trims strings and turns empty optionals into null.
func (s *Settings) Normalize() {
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

// FullAddress joins the address parts that are set.
func (s Settings) FullAddress() string {
	var parts []string
	for _, p := range []*string{s.Address, s.District, s.City, s.Province, s.Country, s.PostalCode} {
		if p != nil {
			parts = append(parts, *p)
		}
	}
	return strings.Join(parts, ", ")
}

// Replaced reports whether an image URL changed and the old one is orphaned.
func Replaced(prev, next *string) bool { return prev != nil && (next == nil || *prev != *next) }

type Repository interface {
	// Get returns the stored JSON, or kernel.ErrNotFound.
	Get(ctx context.Context) ([]byte, error)
	Put(ctx context.Context, raw []byte) error
}

// Images stores and removes branding images.
type Images interface {
	PutImage(ctx context.Context, body []byte, feature, scope string) (key, url string, err error)
	RemoveOwned(ctx context.Context, urlOrKey, feature string)
}
