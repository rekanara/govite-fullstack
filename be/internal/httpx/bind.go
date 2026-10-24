package httpx

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"

	"govite/internal/policy"
)

var validate = func() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())
	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		for _, tag := range []string{"json", "query"} {
			if name, _, _ := strings.Cut(f.Tag.Get(tag), ","); name != "" && name != "-" {
				return name
			}
		}
		return f.Name
	})
	_ = v.RegisterValidation("password", func(fl validator.FieldLevel) bool {
		n := len([]rune(fl.Field().String()))
		return n >= policy.Security.PasswordMinLength && n <= policy.Security.PasswordMaxLength
	})
	return v
}()

// Body decodes JSON into dst and validates `validate` tags.
// An empty body counts as `{}` so all-optional payloads work.
func Body(c *fiber.Ctx, dst any) error {
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), dst); err != nil {
			return Validation("body: invalid JSON")
		}
	}
	return Check(dst)
}

// Query parses query params (`query` tags) into dst, then validates.
func Query(c *fiber.Ctx, dst any) error {
	if err := c.QueryParser(dst); err != nil {
		return Validation("query: " + err.Error())
	}
	return Check(dst)
}

// Check validates a struct and turns failures into a VALIDATION_ERROR.
func Check(dst any) error {
	err := validate.Struct(dst)
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		return err
	}
	parts := make([]string, 0, len(ve))
	for _, fe := range ve {
		path := fe.Namespace()
		if _, rest, ok := strings.Cut(path, "."); ok {
			path = rest
		}
		msg := fe.Tag()
		if fe.Param() != "" {
			msg += "=" + fe.Param()
		}
		parts = append(parts, path+": "+msg)
	}
	return Validation(strings.Join(parts, "; "))
}

// Pagination is embedded by list queries; Normalize applies defaults.
type Pagination struct {
	Page   int    `query:"page" validate:"omitempty,min=1"`
	Limit  int    `query:"limit" validate:"omitempty,min=1,max=100"`
	Search string `query:"search" validate:"max=100"`
}

func (p *Pagination) Normalize() {
	if p.Page == 0 {
		p.Page = 1
	}
	if p.Limit == 0 {
		p.Limit = 20
	}
	p.Search = strings.TrimSpace(p.Search)
}

func (p Pagination) Offset() int32 { return int32((p.Page - 1) * p.Limit) }

// SearchPtr is nil when empty, for sqlc nullable args.
func (p Pagination) SearchPtr() *string { return Ptr(p.Search) }

// Ptr returns nil for the zero value, else &v.
func Ptr[T comparable](v T) *T {
	var zero T
	if v == zero {
		return nil
	}
	return &v
}

// TrimPtr trims a string pointer in place.
func TrimPtr(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	return &t
}
