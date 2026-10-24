// Package application reads and replaces the app settings.
package application

import (
	"context"
	"encoding/json"
	"errors"

	"govite/internal/kernel"
	"govite/internal/settings/domain"
)

const rowID = "app"

type Service struct {
	Repo   domain.Repository
	Images domain.Images
	Audit  kernel.Auditor
}

// Get returns stored settings over defaults; missing keys keep defaults.
func (s *Service) Get(ctx context.Context) (domain.Settings, error) {
	v := domain.Defaults()
	raw, err := s.Repo.Get(ctx)
	if errors.Is(err, kernel.ErrNotFound) {
		return v, nil
	}
	if err != nil {
		return v, err
	}
	_ = json.Unmarshal(raw, &v)
	return v, nil
}

// Put replaces the settings (already normalized and validated) and drops
// branding images no longer referenced.
func (s *Service) Put(ctx context.Context, v domain.Settings) error {
	previous, _ := s.Get(ctx)
	raw, _ := json.Marshal(v)
	if err := s.Repo.Put(ctx, raw); err != nil {
		return err
	}
	if domain.Replaced(previous.LogoURL, v.LogoURL) {
		s.Images.RemoveOwned(ctx, *previous.LogoURL, "branding")
	}
	if domain.Replaced(previous.BannerURL, v.BannerURL) {
		s.Images.RemoveOwned(ctx, *previous.BannerURL, "branding")
	}
	s.Audit.Record(ctx, "settings.update", "settings", rowID, previous, v)
	return nil
}

func (s *Service) UploadImage(ctx context.Context, body []byte) (string, error) {
	_, url, err := s.Images.PutImage(ctx, body, "branding", "")
	return url, err
}
