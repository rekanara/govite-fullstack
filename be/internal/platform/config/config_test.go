package config

import (
	"strings"
	"testing"
)

func TestProductionRejectsExamplePlaceholders(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("JWT_ACCESS_SECRET", "change-me-32-bytes-minimum-secret-value")
	t.Setenv("REFRESH_TOKEN_PEPPER", "change-me-refresh-pepper")

	t.Setenv("APP_ENV", "development")
	if _, err := Load(); err != nil {
		t.Fatalf("development must accept the example values: %v", err)
	}
	t.Setenv("APP_ENV", "production")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "JWT_ACCESS_SECRET still has") || !strings.Contains(err.Error(), "REFRESH_TOKEN_PEPPER still has") {
		t.Fatalf("production must reject placeholders, got %v", err)
	}
}
