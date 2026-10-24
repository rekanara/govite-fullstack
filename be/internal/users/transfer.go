package users

import (
	"fmt"
	"io"
	"net/mail"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"govite/internal/httpx"
	"govite/internal/xport"
)

// ponytail: exports are built in memory; stream rows when this cap is hit.
const (
	exportLimit = 10_000
	importLimit = 1000
	importBytes = 1_000_000
)

func importTemplate() []byte {
	return xport.CSV([][]any{{"email", "fullName", "role"}, {"jane@example.com", "Jane Doe", "Admin"}})
}

func iso(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func (s *Service) export(c *fiber.Ctx) error {
	var q struct {
		listQuery
		Format string `query:"format" validate:"omitempty,oneof=csv xlsx"`
	}
	if err := httpx.Query(c, &q); err != nil {
		return err
	}
	q.normalize()
	rows, err := s.Q.ListUsers(c.UserContext(), q.params(exportLimit, 0))
	if err != nil {
		return err
	}
	table := [][]any{{"email", "fullName", "role", "status", "emailVerifiedAt", "lastLoginAt", "createdAt"}}
	for _, r := range rows {
		v := s.toView(r.User, r.RoleName)
		status := "inactive"
		if v.Invited {
			status = "invited"
		} else if v.IsActive {
			status = "active"
		}
		table = append(table, []any{v.Email, v.FullName, v.RoleName, status, iso(v.EmailVerifiedAt), iso(v.LastLoginAt), iso(&v.CreatedAt)})
	}
	stamp := time.Now().UTC().Format(time.DateOnly)
	s.Audit.From(c, "user.export", "user", "", c.AllParams(), fiber.Map{"file": true})
	if q.Format == "xlsx" {
		return httpx.File(c, "users-"+stamp+".xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", xport.XLSX(table, "Users"))
	}
	return httpx.File(c, "users-"+stamp+".csv", "text/csv; charset=utf-8", xport.CSV(table))
}

type previewRow struct {
	Row      int      `json:"row"`
	Email    string   `json:"email"`
	FullName string   `json:"fullName"`
	Role     string   `json:"role"`
	Errors   []string `json:"errors"`
}

type importRow struct {
	previewRow
	input *CreateInput
}

func readUpload(c *fiber.Ctx) ([]byte, error) {
	fh, err := c.FormFile("file")
	if err != nil {
		return nil, errImport("file is empty")
	}
	if fh.Size > importBytes {
		return nil, fiber.ErrRequestEntityTooLarge
	}
	f, err := fh.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, importBytes+1))
}

func (s *Service) validateImport(c *fiber.Ctx) ([]importRow, error) {
	data, err := readUpload(c)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, errImport("file is empty")
	}
	table, err := xport.ParseCSV(data)
	if err != nil {
		return nil, errImport(err.Error())
	}
	if len(table) == 0 {
		return nil, errImport("header must have email,fullName,role")
	}
	cols := map[string]int{}
	for i, h := range table[0] {
		cols[strings.ToLower(strings.TrimSpace(h))] = i
	}
	for _, need := range []string{"email", "fullname", "role"} {
		if _, ok := cols[need]; !ok {
			return nil, errImport("header must have email,fullName,role")
		}
	}
	body := table[1:]
	if len(body) > importLimit {
		return nil, errImport(fmt.Sprintf("max %d rows", importLimit))
	}
	cell := func(r []string, name string) string {
		if i := cols[name]; i < len(r) {
			return strings.TrimSpace(r[i])
		}
		return ""
	}
	ctx := c.UserContext()
	roles, err := s.Q.RoleNames(ctx)
	if err != nil {
		return nil, err
	}
	roleIDs := map[string]string{}
	for _, r := range roles {
		roleIDs[strings.ToLower(r.Name)] = r.ID
	}
	emails := make([]string, len(body))
	for i, r := range body {
		emails[i] = strings.ToLower(cell(r, "email"))
	}
	existing, err := s.Q.ExistingEmails(ctx, emails)
	if err != nil {
		return nil, err
	}
	taken := map[string]bool{}
	for _, e := range existing {
		taken[e] = true
	}

	out := make([]importRow, len(body))
	for i, r := range body {
		email, name, role := strings.ToLower(cell(r, "email")), cell(r, "fullname"), cell(r, "role")
		row := importRow{previewRow: previewRow{Row: i + 2, Email: email, FullName: name, Role: role, Errors: []string{}}}
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
			row.input = &CreateInput{Email: email, FullName: name, RoleID: roleID}
		}
		out[i] = row
	}
	return out, nil
}

func (s *Service) importUsers(c *fiber.Ctx) error {
	rows, err := s.validateImport(c)
	if err != nil {
		return err
	}
	type rowErr struct {
		Row     int    `json:"row"`
		Message string `json:"message"`
	}
	created, errs := 0, []rowErr{}
	for _, r := range rows {
		if r.input == nil {
			errs = append(errs, rowErr{r.Row, strings.Join(r.Errors, "; ")})
			continue
		}
		if _, err := s.Create(c.UserContext(), *r.input); err != nil {
			return err
		}
		created++
	}
	res := fiber.Map{"created": created, "errors": errs}
	s.Audit.From(c, "user.import", "user", "", c.AllParams(), res)
	return httpx.Created(c, res)
}
