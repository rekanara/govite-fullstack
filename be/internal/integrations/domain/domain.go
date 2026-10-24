// Package domain holds webhook events (verified, idempotent) and API keys.
package domain

import (
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"govite/internal/kernel"
)

var (
	ErrProviderDisabled = kernel.E(kernel.Unavailable, "WEBHOOK_PROVIDER_DISABLED", "Midtrans is not configured")
	ErrBadSignature     = kernel.E(kernel.Unauthenticated, "INVALID_WEBHOOK_SIGNATURE", "Invalid webhook signature")
	ErrEventNotFound    = kernel.E(kernel.NotFound, "WEBHOOK_EVENT_NOT_FOUND", "Webhook event not found")
	ErrUnsupported      = kernel.E(kernel.Invalid, "WEBHOOK_PROVIDER_UNSUPPORTED", "Webhook provider is not supported")
	ErrNotFailed        = kernel.E(kernel.Invalid, "WEBHOOK_REPLAY_NOT_FAILED", "Only failed webhook events can be replayed")
	ErrKeyNotFound      = kernel.E(kernel.NotFound, "API_KEY_NOT_FOUND", "API key not found")
)

type Event struct {
	ID          string          `json:"id"`
	Provider    string          `json:"provider"`
	EventKey    string          `json:"eventKey"`
	EventType   string          `json:"eventType"`
	Status      string          `json:"status"`
	Error       *string         `json:"error"`
	Payload     json.RawMessage `json:"payload"`
	ReceivedAt  time.Time       `json:"receivedAt"`
	ProcessedAt *time.Time      `json:"processedAt"`
}

// VerifyMidtrans checks sha512(order_id+status_code+gross_amount+serverKey).
func VerifyMidtrans(serverKey string, p map[string]string) bool {
	if serverKey == "" || p["signature_key"] == "" {
		return false
	}
	sum := sha512.Sum512([]byte(p["order_id"] + p["status_code"] + p["gross_amount"] + serverKey))
	return subtle.ConstantTimeCompare([]byte(hex.EncodeToString(sum[:])), []byte(strings.ToLower(p["signature_key"]))) == 1
}

// MidtransKey identifies one delivery for idempotency.
func MidtransKey(p map[string]string) string {
	return strings.Join([]string{p["order_id"], p["transaction_id"], p["transaction_status"], p["status_code"], p["gross_amount"]}, ":")
}

type APIKey struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	IsActive   bool       `json:"isActive"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
	CreatedAt  time.Time  `json:"createdAt"`
}

// HashAPIKey is how keys are stored; verify an incoming key by hashing it.
func HashAPIKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

type Repository interface {
	// InsertEvent returns the new id, or "" when (provider,key) already exists.
	InsertEvent(ctx context.Context, provider, key, eventType string, payload json.RawMessage) (string, error)
	EventByKey(ctx context.Context, provider, key string) (Event, error)
	Event(ctx context.Context, id string) (Event, error)
	// Claim moves an event to processing; false when someone else holds it.
	Claim(ctx context.Context, id string) (bool, error)
	MarkProcessed(ctx context.Context, id string) error
	MarkFailed(ctx context.Context, id, msg string) error
	CountEvents(ctx context.Context, provider, status *string) (int64, error)
	ListEvents(ctx context.Context, provider, status *string, limit, offset int32) ([]Event, error)

	ListKeys(ctx context.Context) ([]APIKey, error)
	CreateKey(ctx context.Context, name, prefix, hash string) (APIKey, error)
	UpdateKey(ctx context.Context, id string, name *string, active *bool) (APIKey, error)
	DeleteKey(ctx context.Context, id string) (bool, error)
}
