// Package domain is the caller's own profile and preferences.
package domain

import "context"

type Repository interface {
	PushEnabled(ctx context.Context, userID string) (bool, error)
	SetPushEnabled(ctx context.Context, userID string, on bool) error
	SetFullName(ctx context.Context, userID, name string) error
}
