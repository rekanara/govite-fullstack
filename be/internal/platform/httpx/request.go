package httpx

import (
	"io"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"govite/internal/kernel"
)

// SetPrincipal stores the caller in the request's user context, where
// application code reads it with kernel.PrincipalFrom.
func SetPrincipal(c *fiber.Ctx, p kernel.Principal) {
	c.SetUserContext(kernel.WithPrincipal(c.UserContext(), p))
}

// Principal returns the caller; only valid behind authentication.
func Principal(c *fiber.Ctx) kernel.Principal { return kernel.PrincipalFrom(c.UserContext()) }

// ID returns the named path param when it is a UUID.
func ID(c *fiber.Ctx, name string) (string, error) {
	id := c.Params(name)
	if _, err := uuid.Parse(id); err != nil {
		return "", Validation(name + ": must be a UUID")
	}
	return id, nil
}

// Upload is a multipart file read into memory.
type Upload struct {
	Body        []byte
	Name        string
	ContentType string
}

// ErrTooLarge marks an upload over its byte cap.
var ErrTooLarge = fiber.ErrRequestEntityTooLarge

// FormFile reads a multipart field, refusing anything over max bytes.
// A missing field returns an empty Upload and nil error.
func FormFile(c *fiber.Ctx, field string, max int64) (Upload, error) {
	fh, err := c.FormFile(field)
	if err != nil {
		return Upload{}, nil
	}
	up := Upload{Name: fh.Filename, ContentType: fh.Header.Get("Content-Type")}
	if fh.Size > max {
		return up, ErrTooLarge
	}
	f, err := fh.Open()
	if err != nil {
		return up, err
	}
	defer f.Close()
	up.Body, err = io.ReadAll(io.LimitReader(f, max+1))
	if int64(len(up.Body)) > max {
		return Upload{Name: fh.Filename}, ErrTooLarge
	}
	return up, err
}

// StatusOf is the HTTP status an error renders as (for access logs).
func StatusOf(err error) int {
	switch e := err.(type) {
	case *fiber.Error:
		return e.Code
	case *Error:
		return e.Status
	case *kernel.Error:
		if s := kindStatus[e.Kind]; s != 0 {
			return s
		}
	}
	return 500
}
