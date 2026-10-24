package httpx

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Error is a domain error: handlers return it, ErrorHandler renders it.
type Error struct {
	Status  int
	Code    string
	Message string
	Detail  string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func E(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

func (e *Error) WithDetail(d string) *Error {
	c := *e
	c.Detail = d
	return &c
}

var (
	ErrNotAuthenticated = E(401, "NOT_AUTHENTICATED", "No authenticated user on request")
	ErrNotFound         = E(404, "NOT_FOUND", "Record not found")
	ErrConflict         = E(409, "CONFLICT", "Duplicate value")
)

func Validation(detail string) *Error {
	return E(400, "VALIDATION_ERROR", "Request validation failed").WithDetail(detail)
}

// IsUnique reports a unique violation, optionally on a constraint containing `on`.
func IsUnique(err error, on string) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505" && strings.Contains(pg.ConstraintName, on)
}

// ErrorHandler wraps every error in the envelope. Unknown errors are logged and
// answered as a generic 500, never leaking internals.
func ErrorHandler(c *fiber.Ctx, err error) error {
	var (
		de *Error
		fe *fiber.Error
		pg *pgconn.PgError
	)
	switch {
	case errors.As(err, &de):
	case errors.As(err, &fe):
		de = E(fe.Code, strings.ToUpper(strings.ReplaceAll(statusName(fe.Code), " ", "_")), statusName(fe.Code))
		if fe.Message != statusName(fe.Code) {
			de.Detail = fe.Message
		}
	case errors.Is(err, pgx.ErrNoRows):
		de = ErrNotFound
	case errors.As(err, &pg) && pg.Code == "23505":
		de = ErrConflict
	case errors.As(err, &pg) && pg.Code == "23503":
		de = E(409, "CONFLICT", "Referenced record is in use or missing")
	default:
		sentry.CaptureException(err)
		slog.ErrorContext(c.UserContext(), "unhandled error", "err", err, "path", c.Path(), "requestId", c.Locals("requestid"))
		de = E(500, "INTERNAL_SERVER_ERROR", "Internal Server Error")
	}
	return c.Status(de.Status).JSON(Envelope{
		Data:   nil,
		Error:  &ErrorBody{Code: de.Code, Detail: de.Detail},
		Status: buildStatus(de.Status, "ERROR", de.Message, de.Code),
	})
}
