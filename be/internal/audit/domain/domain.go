// Package domain holds the audit entry and the redaction rule.
package domain

import (
	"context"
	"encoding/json"
	"time"
)

type Entry struct {
	ID          string          `json:"id"`
	ActorUserID *string         `json:"actorUserId"`
	Action      string          `json:"action"`
	EntityType  string          `json:"entityType"`
	EntityID    string          `json:"entityId"`
	Before      json.RawMessage `json:"before"`
	After       json.RawMessage `json:"after"`
	CreatedAt   time.Time       `json:"createdAt"`
}

// Filter narrows the log. Nil fields match everything; To is exclusive.
type Filter struct {
	ActorUserID, UserID, EntityType, EntityID, Action *string
	From, To                                          *time.Time
}

type Repository interface {
	Insert(ctx context.Context, e Entry) error
	Count(ctx context.Context, f Filter) (int64, error)
	List(ctx context.Context, f Filter, limit, offset int32) ([]Entry, error)
}

var sensitive = map[string]bool{
	"passwordHash": true, "password": true, "tokenHash": true, "token": true,
	"accessToken": true, "refreshToken": true, "backupCodes": true, "currentPassword": true,
	"newPassword": true, "manualKey": true, "otpauthUrl": true, "key": true,
}

// Redact round-trips v through JSON and drops secret keys at any depth.
func Redact(v any) json.RawMessage {
	if v == nil {
		return nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var generic any
	if json.Unmarshal(raw, &generic) != nil {
		return raw
	}
	out, _ := json.Marshal(strip(generic))
	return out
}

func strip(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if sensitive[k] {
				delete(t, k)
			} else {
				t[k] = strip(val)
			}
		}
	case []any:
		for i := range t {
			t[i] = strip(t[i])
		}
	}
	return v
}
