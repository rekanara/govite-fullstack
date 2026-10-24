package application

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"govite/internal/kernel"
	"govite/internal/users/domain"
)

func asKernel(err error, target **kernel.Error) bool { return errors.As(err, target) }

// ponytail: exports are built in memory; stream rows when this cap is hit.
const (
	ExportLimit = 10_000
	ImportLimit = 1000
)

func (s *Service) Export(ctx context.Context, f domain.Filter) ([]domain.User, error) {
	return s.Repo.List(ctx, f, ExportLimit, 0)
}

type PreviewRow struct {
	Row      int      `json:"row"`
	Email    string   `json:"email"`
	FullName string   `json:"fullName"`
	Role     string   `json:"role"`
	Errors   []string `json:"errors"`
}

type ImportRow struct {
	PreviewRow
	Input *CreateInput
}

// ValidateImport checks a parsed CSV table (header + rows) against roles,
// existing emails and duplicates within the file.
func (s *Service) ValidateImport(ctx context.Context, table [][]string) ([]ImportRow, error) {
	if len(table) == 0 {
		return nil, domain.ErrImport("header must have email,fullName,role")
	}
	cols := map[string]int{}
	for i, h := range table[0] {
		cols[strings.ToLower(strings.TrimSpace(h))] = i
	}
	for _, need := range []string{"email", "fullname", "role"} {
		if _, ok := cols[need]; !ok {
			return nil, domain.ErrImport("header must have email,fullName,role")
		}
	}
	body := table[1:]
	if len(body) > ImportLimit {
		return nil, domain.ErrImport(fmt.Sprintf("max %d rows", ImportLimit))
	}
	cell := func(r []string, name string) string {
		if i := cols[name]; i < len(r) {
			return strings.TrimSpace(r[i])
		}
		return ""
	}
	roleIDs, err := s.Repo.RoleIDsByName(ctx)
	if err != nil {
		return nil, err
	}
	emails := make([]string, len(body))
	for i, r := range body {
		emails[i] = strings.ToLower(cell(r, "email"))
	}
	existing, err := s.Repo.ExistingEmails(ctx, emails)
	if err != nil {
		return nil, err
	}
	taken := map[string]bool{}
	for _, e := range existing {
		taken[e] = true
	}
	out := make([]ImportRow, len(body))
	for i, r := range body {
		email, name, role := strings.ToLower(cell(r, "email")), cell(r, "fullname"), cell(r, "role")
		row := ImportRow{PreviewRow: PreviewRow{Row: i + 2, Email: email, FullName: name, Role: role, Errors: []string{}}}
		if _, err := mail.ParseAddress(email); err != nil || !strings.Contains(email, "@") {
			row.Errors = append(row.Errors, "email: invalid email")
		}
		if name == "" || len([]rune(name)) > 100 {
			row.Errors = append(row.Errors, "fullName: must be 1-100 characters")
		}
		roleID, ok := roleIDs[strings.ToLower(role)]
		if !ok {
			row.Errors = append(row.Errors, fmt.Sprintf("unknown role %q", role))
		}
		if len(row.Errors) == 0 && taken[email] {
			row.Errors = append(row.Errors, "email already exists")
		}
		if len(row.Errors) == 0 {
			taken[email] = true
			row.Input = &CreateInput{Email: email, FullName: name, RoleID: roleID}
		}
		out[i] = row
	}
	return out, nil
}

type ImportError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

func (s *Service) Import(ctx context.Context, table [][]string) (created int, errs []ImportError, err error) {
	rows, err := s.ValidateImport(ctx, table)
	if err != nil {
		return 0, nil, err
	}
	errs = []ImportError{}
	for _, r := range rows {
		if r.Input == nil {
			errs = append(errs, ImportError{r.Row, strings.Join(r.Errors, "; ")})
			continue
		}
		if _, err := s.Create(ctx, *r.Input); err != nil {
			return created, errs, err
		}
		created++
	}
	return created, errs, nil
}
