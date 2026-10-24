// Package kernel holds the framework-free vocabulary every layer may import:
// domain errors, the authenticated principal, and ID generation.
package kernel

import (
	"context"

	"github.com/google/uuid"
)

// Kind classifies a domain error. Interface adapters map it to a transport
// status (HTTP, gRPC, CLI exit code); the domain never sees those codes.
type Kind int

const (
	Invalid Kind = iota + 1
	Unauthenticated
	Forbidden
	NotFound
	Conflict
	TooLarge
	Unavailable
)

// Error is a domain error: stable machine code plus human message.
type Error struct {
	Kind    Kind
	Code    string
	Message string
	Detail  string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func E(kind Kind, code, message string) *Error {
	return &Error{Kind: kind, Code: code, Message: message}
}

func (e *Error) WithDetail(d string) *Error {
	c := *e
	c.Detail = d
	return &c
}

// ErrNotFound is what repositories return for a missing row.
var ErrNotFound = E(NotFound, "NOT_FOUND", "Record not found")

// Object storage errors, shared by every feature that stores files.
var (
	ErrStorageUnavailable = E(Unavailable, "STORAGE_UNAVAILABLE", "Object storage is not configured")
	ErrImageRequired      = E(Invalid, "IMAGE_REQUIRED", "An image file is required")
	ErrImageTooLarge      = E(Invalid, "IMAGE_TOO_LARGE", "Image must be 5 MB or smaller")
	ErrInvalidImage       = E(Invalid, "INVALID_IMAGE", "Only JPEG, PNG or WebP images")
)

func Validation(detail string) *Error {
	return E(Invalid, "VALIDATION_ERROR", "Request validation failed").WithDetail(detail)
}

// Principal is the authenticated caller.
type Principal struct {
	ID        string
	RoleID    string
	SessionID string
}

type principalKey struct{}

// WithPrincipal carries the caller through application code without Fiber.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFrom returns the caller, or the zero Principal when anonymous.
func PrincipalFrom(ctx context.Context) Principal {
	p, _ := ctx.Value(principalKey{}).(Principal)
	return p
}

// Auditor records a mutation. Use cases depend on this port; the audit
// feature implements it. Failures are the implementation's to log.
type Auditor interface {
	Record(ctx context.Context, action, entityType, entityID string, before, after any)
}

// Tx runs fn atomically. Application use cases own transaction boundaries;
// infrastructure decides how (a Postgres transaction carried in ctx).
type Tx interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

// NewID returns a time-ordered UUIDv7.
func NewID() string { return uuid.Must(uuid.NewV7()).String() }

// Ptr returns nil for the zero value, else &v.
func Ptr[T comparable](v T) *T {
	var zero T
	if v == zero {
		return nil
	}
	return &v
}
