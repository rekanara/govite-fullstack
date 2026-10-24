package server

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"govite/internal/platform/httpx"
)

func TestRecoverPanicRendersEnvelope(t *testing.T) {
	f := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	f.Use(recoverPanic)
	f.Get("/boom", func(*fiber.Ctx) error { panic("boom") })
	res, err := f.Test(httptest.NewRequest("GET", "/boom", nil))
	if err != nil || res.StatusCode != 500 {
		t.Fatalf("want 500, got %v %v", res.StatusCode, err)
	}
}
