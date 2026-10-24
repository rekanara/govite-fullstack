// Package domain is in-app notifications, devices and push delivery ports.
package domain

import (
	"context"
	"encoding/json"
	"time"

	"govite/internal/kernel"
)

var ErrNotFound = kernel.E(kernel.NotFound, "NOTIFICATION_NOT_FOUND", "Notification not found")

type Input struct {
	Type      string // system | security | admin | announcement
	Title     string
	Body      string
	Data      map[string]string
	DedupeKey string // one notification per user per key
	ImageURL  string
}

type Notification struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Title     string          `json:"title"`
	Body      string          `json:"body"`
	Data      json.RawMessage `json:"data"`
	ReadAt    *time.Time      `json:"readAt"`
	CreatedAt time.Time       `json:"createdAt"`
}

type Device struct{ ID, Token string }

type PushMessage struct {
	Title, Body, ImageURL string
	Data                  map[string]string
}

type Repository interface {
	ActiveUserIDs(ctx context.Context, after string, limit int32) ([]string, error)
	// Insert writes one row per user (ids aligned), skipping dedupe hits;
	// returns the users that got a new row.
	Insert(ctx context.Context, in Input, data json.RawMessage, ids, userIDs []string) ([]string, error)
	Devices(ctx context.Context, userIDs []string, after string, limit int32) ([]Device, error)
	DeleteDeviceTokens(ctx context.Context, tokens []string) error
	// RegisterDevice moves token to (userID, deviceID), removing it elsewhere.
	RegisterDevice(ctx context.Context, userID, deviceID, token string, device json.RawMessage) error
	DeleteDevice(ctx context.Context, userID, deviceID string) error
	Counts(ctx context.Context, userID string) (total, unread int64, err error)
	List(ctx context.Context, userID string, unreadOnly bool, limit, offset int32) ([]Notification, error)
	MarkAllRead(ctx context.Context, userID string) error
	MarkRead(ctx context.Context, id, userID string) (Notification, error)
}

// Pusher sends push messages and returns tokens the provider reported dead.
type Pusher interface {
	Send(ctx context.Context, tokens []string, m PushMessage) (invalid []string)
}
