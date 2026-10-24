package server_test

// End-to-end tests against a real Postgres. Skipped unless TEST_DATABASE_URL
// is set (make test-e2e sets it to the docker-compose postgres_test).

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"io"
	"maps"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pquerna/otp/totp"

	"govite/internal/kernel"
	"govite/internal/platform/config"
	"govite/internal/platform/crypto"
	"govite/internal/platform/fcm"
	"govite/internal/platform/mail"
	"govite/internal/platform/postgres"
	"govite/internal/platform/postgres/store"
	"govite/internal/platform/storage"
	"govite/internal/policy"
	"govite/internal/server"
)

const password = "Password123!"

type env struct {
	t    *testing.T
	f    *fiber.App
	q    *store.Queries
	pool *pgxpool.Pool
	mail *mail.Memory
	push *fcm.Memory
}

func setup(t *testing.T) *env {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := postgres.Migrate(ctx, pool, "up"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE role, permission, "user", audit_log, app_setting, notification, announcement, webhook_event, api_key, attachment, fcm_device CASCADE`); err != nil {
		t.Fatal(err)
	}
	m := &mail.Memory{}
	cfg := config.Config{Env: "test", WebURL: "http://web.test", JWTSecret: strings.Repeat("s", 32), RefreshPepper: strings.Repeat("p", 16), ThrottleLimit: 100000, LoginThrottleLimit: 100000, LogLevel: "silent", MidtransServerKey: "test-server-key"}
	push := &fcm.Memory{}
	d := server.Deps{Cfg: cfg, Pool: pool, Mail: m, Push: push}
	if endpoint := os.Getenv("TEST_MINIO_ENDPOINT"); endpoint != "" {
		d.Files, err = storage.New(storage.Config{Endpoint: endpoint, AccessKey: "minio", SecretKey: "minio12345", Bucket: "app-test", PublicEndpoint: "http://" + endpoint})
		if err != nil {
			t.Fatal(err)
		}
		if err := d.Files.EnsureBucket(ctx); err != nil {
			t.Fatal(err)
		}
	}
	f, _ := server.New(d)
	return &env{t, f, store.New(pool), pool, m, push}
}

// seedUser creates a role holding perms (scope all) and an active user in it.
func (e *env) seedUser(email string, perms ...policy.Permission) (userID, roleID string) {
	ctx := context.Background()
	roleID, userID = kernel.NewID(), kernel.NewID()
	must(e.t, e.q.CreateRole(ctx, store.CreateRoleParams{ID: roleID, Name: "role-" + email}))
	for _, p := range perms {
		must(e.t, e.q.InsertRolePermission(ctx, store.InsertRolePermissionParams{ID: kernel.NewID(), RoleID: roleID, Resource: p.Resource, Action: p.Action, Scope: "all"}))
	}
	hash, _ := crypto.HashPassword(password)
	name := "Test"
	must(e.t, e.q.CreateUser(ctx, store.CreateUserParams{ID: userID, Email: email, FullName: &name, RoleID: roleID, PasswordHash: &hash}))
	return userID, roleID
}

type res struct {
	Code    int
	Body    map[string]any
	Cookies []*http.Cookie
	Header  http.Header
	Raw     []byte
}

func (r res) data() map[string]any { m, _ := r.Body["data"].(map[string]any); return m }
func (r res) errCode() string {
	m, _ := r.Body["error"].(map[string]any)
	s, _ := m["code"].(string)
	return s
}

type opt func(*http.Request)

func bearer(tok string) opt {
	return func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+tok) }
}
func cookie(c *http.Cookie) opt { return func(r *http.Request) { r.AddCookie(c) } }

func (e *env) do(method, path string, body any, opts ...opt) res {
	e.t.Helper()
	var rd io.Reader
	ct := ""
	switch b := body.(type) {
	case nil:
	case *multipart.Writer:
		e.t.Fatal("use upload")
	case []byte:
		rd = bytes.NewReader(b)
	default:
		raw, _ := json.Marshal(b)
		rd, ct = bytes.NewReader(raw), "application/json"
	}
	req := httptest.NewRequest(method, "/api/v1"+path, rd)
	if ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	for _, o := range opts {
		o(req)
	}
	return e.send(req)
}

func (e *env) send(req *http.Request) res {
	e.t.Helper()
	resp, err := e.f.Test(req, 10_000)
	if err != nil {
		e.t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	r := res{Code: resp.StatusCode, Cookies: resp.Cookies(), Header: resp.Header, Raw: raw}
	_ = json.Unmarshal(raw, &r.Body)
	return r
}

func (e *env) upload(path, field, filename string, content []byte, tok string) res {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormFile(field, filename)
	_, _ = fw.Write(content)
	_ = w.Close()
	req := httptest.NewRequest("POST", "/api/v1"+path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+tok)
	return e.send(req)
}

func (e *env) login(email string) (token string, refresh *http.Cookie) {
	e.t.Helper()
	r := e.do("POST", "/auth/login", map[string]string{"email": email, "password": password})
	if r.Code != 201 {
		e.t.Fatalf("login %s: %d %s", email, r.Code, r.Raw)
	}
	for _, c := range r.Cookies {
		if c.Name == "refresh_token" {
			refresh = c
		}
	}
	return r.data()["accessToken"].(string), refresh
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func expect(t *testing.T, r res, code int, errCode ...string) {
	t.Helper()
	if r.Code != code {
		t.Fatalf("want %d got %d: %s", code, r.Code, r.Raw)
	}
	if len(errCode) > 0 && r.errCode() != errCode[0] {
		t.Fatalf("want error %s got %s: %s", errCode[0], r.errCode(), r.Raw)
	}
}

var all = policy.Permissions

func TestHealthEnvelope(t *testing.T) {
	e := setup(t)
	r := e.do("GET", "/health", nil)
	expect(t, r, 200)
	st := r.Body["status"].(map[string]any)
	if st["type"] != "SUCCESS" || st["messageInd"] != "OK" || !strings.HasSuffix(st["timeStamp"].(string), "+07:00") {
		t.Fatalf("bad status %v", st)
	}
	if r.Header.Get("X-Request-Id") == "" {
		t.Fatal("missing x-request-id")
	}
}

func TestRBAC(t *testing.T) {
	e := setup(t)
	expect(t, e.do("GET", "/users", nil), 401)
	e.seedUser("none@test.local")
	e.seedUser("admin@test.local", all...)
	none, _ := e.login("none@test.local")
	admin, _ := e.login("admin@test.local")

	r := e.do("GET", "/users", nil, bearer(none))
	expect(t, r, 403, "PERMISSION_NOT_GRANTED")
	if r.Body["status"].(map[string]any)["messageInd"] != "Permission tidak diberikan" {
		t.Fatal("messageInd")
	}

	r = e.do("GET", "/users", nil, bearer(admin))
	expect(t, r, 200)
	if n := len(r.Body["data"].([]any)); n != 2 {
		t.Fatalf("want 2 users got %d", n)
	}
	meta := r.Body["meta"].(map[string]any)
	if meta["total"].(float64) != 2 || meta["totalPages"].(float64) != 1 || meta["limit"].(float64) != 20 {
		t.Fatalf("meta %v", meta)
	}

	// self-service works without permission
	expect(t, e.do("GET", "/auth/me", nil, bearer(none)), 200)
	expect(t, e.do("GET", "/account/sessions", nil, bearer(none)), 200)
}

func TestRefreshRotationAndReuse(t *testing.T) {
	e := setup(t)
	e.seedUser("u@test.local")
	_, c1 := e.login("u@test.local")
	if c1 == nil || !c1.HttpOnly || c1.Path != "/api/v1/auth" {
		t.Fatalf("bad cookie %+v", c1)
	}
	r := e.do("POST", "/auth/refresh", nil, cookie(c1))
	expect(t, r, 200)
	if r.data()["accessToken"] == "" {
		t.Fatal("no token")
	}
	var c2 *http.Cookie
	for _, c := range r.Cookies {
		if c.Name == "refresh_token" {
			c2 = c
		}
	}
	// replaying the old token revokes the family, killing the new one too
	expect(t, e.do("POST", "/auth/refresh", nil, cookie(c1)), 401, "REFRESH_TOKEN_REUSE_DETECTED")
	expect(t, e.do("POST", "/auth/refresh", nil, cookie(c2)), 401, "INVALID_REFRESH_TOKEN")
	expect(t, e.do("POST", "/auth/refresh", nil), 401, "MISSING_REFRESH_TOKEN")

	// mobile clients get the token in the body
	req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(`{"email":"u@test.local","password":"`+password+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client", "mobile")
	m := e.send(req)
	expect(t, m, 201)
	if m.data()["refreshToken"] == nil || len(m.Cookies) != 0 {
		t.Fatal("mobile login should return body token, no cookie")
	}
}

func TestLoginFailures(t *testing.T) {
	e := setup(t)
	e.seedUser("u@test.local")
	expect(t, e.do("POST", "/auth/login", map[string]string{"email": "u@test.local", "password": "wrong-password"}), 401, "INVALID_CREDENTIALS")
	expect(t, e.do("POST", "/auth/login", map[string]string{"email": "ghost@test.local", "password": password}), 401, "INVALID_CREDENTIALS")
	expect(t, e.do("POST", "/auth/login", map[string]string{"email": "bad"}), 400, "VALIDATION_ERROR")
}

func TestRoleRequiredTwoFactorAndBackupCodes(t *testing.T) {
	e := setup(t)
	_, roleID := e.seedUser("totp@test.local", policy.Permission{Resource: "users", Action: "manage"})
	_, err := e.pool.Exec(context.Background(), `UPDATE role SET require_two_factor = true WHERE id = $1`, roleID)
	must(t, err)

	r := e.do("POST", "/auth/login", map[string]string{"email": "totp@test.local", "password": password})
	expect(t, r, 201)
	d := r.data()
	if d["requiresTwoFactor"] != true || d["setupRequired"] != true || len(r.Cookies) != 0 {
		t.Fatalf("expected challenge: %s", r.Raw)
	}
	code, _ := totp.GenerateCode(d["manualKey"].(string), time.Now())
	r = e.do("POST", "/auth/2fa/verify", map[string]string{"challengeToken": d["challengeToken"].(string), "code": code})
	expect(t, r, 201)
	codes := r.data()["backupCodes"].([]any)
	if len(codes) != 10 || r.data()["accessToken"] == nil {
		t.Fatalf("verify: %s", r.Raw)
	}

	// second login: challenge without setup, finish with a backup code (single use)
	r = e.do("POST", "/auth/login", map[string]string{"email": "totp@test.local", "password": password})
	ch := r.data()["challengeToken"].(string)
	if r.data()["setupRequired"] != false || r.data()["manualKey"] != nil {
		t.Fatalf("no setup expected: %s", r.Raw)
	}
	r = e.do("POST", "/auth/2fa/verify-backup", map[string]string{"challengeToken": ch, "backupCode": codes[0].(string)})
	expect(t, r, 201)
	tok := r.data()["accessToken"].(string)

	r = e.do("POST", "/auth/login", map[string]string{"email": "totp@test.local", "password": password})
	expect(t, e.do("POST", "/auth/2fa/verify-backup", map[string]string{"challengeToken": r.data()["challengeToken"].(string), "backupCode": codes[0].(string)}), 401, "INVALID_TWO_FACTOR_CODE")

	st := e.do("GET", "/account/two-factor", nil, bearer(tok))
	expect(t, st, 200)
	if st.data()["required"] != true || st.data()["backupCodesRemaining"].(float64) != 9 {
		t.Fatalf("status: %s", st.Raw)
	}
	expect(t, e.do("POST", "/account/two-factor/disable", map[string]string{"currentPassword": password}, bearer(tok)), 403, "TWO_FACTOR_REQUIRED")
}

func TestInviteAndPasswordReset(t *testing.T) {
	e := setup(t)
	_, roleID := e.seedUser("admin@test.local", all...)
	admin, _ := e.login("admin@test.local")

	r := e.do("POST", "/users", map[string]string{"email": "New@Test.local", "fullName": "New", "roleId": roleID}, bearer(admin))
	expect(t, r, 201)
	if r.data()["invited"] != true || r.data()["email"] != "new@test.local" {
		t.Fatalf("create: %s", r.Raw)
	}
	expect(t, e.do("POST", "/users", map[string]string{"email": "new@test.local", "fullName": "Dup", "roleId": roleID}, bearer(admin)), 409, "EMAIL_TAKEN")

	msg, _ := e.mail.Last()
	tok := tokenFrom(t, msg.Text)
	expect(t, e.do("POST", "/auth/accept-invite", map[string]string{"token": tok, "fullName": "Neo", "password": password}), 204)
	expect(t, e.do("POST", "/auth/accept-invite", map[string]string{"token": tok, "fullName": "Neo", "password": password}), 400, "INVALID_TOKEN")
	e.login("new@test.local")

	n := e.mail.Count()
	expect(t, e.do("POST", "/auth/forgot-password", map[string]string{"email": "new@test.local"}), 204)
	msg = e.nextMail(n)
	tok = tokenFrom(t, msg.Text)
	expect(t, e.do("POST", "/auth/reset-password", map[string]string{"channel": "web", "token": tok, "newPassword": "NewPassword123!"}), 204)
	expect(t, e.do("POST", "/auth/login", map[string]string{"email": "new@test.local", "password": "NewPassword123!"}), 201)

	// mobile OTP
	n = e.mail.Count()
	expect(t, e.do("POST", "/auth/forgot-password", map[string]string{"email": "new@test.local", "channel": "mobile"}), 204)
	msg = e.nextMail(n)
	if len(msg.Content.Code) != 6 {
		t.Fatalf("otp mail: %+v", msg)
	}
	expect(t, e.do("POST", "/auth/reset-password", map[string]string{"channel": "mobile", "email": "new@test.local", "otp": msg.Content.Code, "newPassword": password}), 204)
	e.login("new@test.local")
}

// nextMail waits for a message beyond the first n; reset mail is sent async.
func (e *env) nextMail(n int) mail.Message {
	e.t.Helper()
	for range 100 {
		if m, ok := e.mail.Last(); ok && e.mail.Count() > n {
			return m
		}
		time.Sleep(10 * time.Millisecond)
	}
	e.t.Fatal("no mail sent")
	return mail.Message{}
}

func tokenFrom(t *testing.T, text string) string {
	t.Helper()
	_, after, ok := strings.Cut(text, "?token=")
	if !ok {
		t.Fatalf("no token in %q", text)
	}
	tok, _, _ := strings.Cut(after, "\n")
	return strings.TrimSpace(tok)
}

func TestUserInvariantsAndAudit(t *testing.T) {
	e := setup(t)
	adminID, roleID := e.seedUser("admin@test.local", all...)
	otherID, _ := e.seedUser("other@test.local")
	admin, _ := e.login("admin@test.local")
	_, otherRefresh := e.login("other@test.local")

	f := false
	expect(t, e.do("PATCH", "/users/"+adminID, map[string]any{"isActive": &f}, bearer(admin)), 403, "CANNOT_MODIFY_SELF")
	expect(t, e.do("DELETE", "/users/"+adminID, nil, bearer(admin)), 403, "CANNOT_MODIFY_SELF")

	r := e.do("DELETE", "/users/"+otherID, nil, bearer(admin))
	expect(t, r, 200)
	if r.data()["deletedAt"] == nil || r.data()["isActive"] != false {
		t.Fatalf("delete: %s", r.Raw)
	}
	expect(t, e.do("POST", "/auth/refresh", nil, cookie(otherRefresh)), 401, "INVALID_REFRESH_TOKEN")
	if n := len(e.do("GET", "/users", nil, bearer(admin)).Body["data"].([]any)); n != 1 {
		t.Fatalf("deleted users must be hidden, got %d", n)
	}
	if n := len(e.do("GET", "/users?deleted=true", nil, bearer(admin)).Body["data"].([]any)); n != 1 {
		t.Fatalf("deleted list got %d", n)
	}
	expect(t, e.do("POST", "/users/"+otherID+"/restore", nil, bearer(admin)), 201)

	r = e.do("POST", "/users/bulk", map[string]any{"ids": []string{otherID, adminID}, "action": "deactivate"}, bearer(admin))
	expect(t, r, 200)
	if len(r.data()["succeeded"].([]any)) != 1 || len(r.data()["failed"].([]any)) != 1 {
		t.Fatalf("bulk: %s", r.Raw)
	}

	// last holder of users.manage cannot lose it
	expect(t, e.do("PUT", "/roles/"+roleID+"/permissions", map[string]any{"permissions": []any{}}, bearer(admin)), 409, "LAST_PERMISSION_HOLDER")

	r = e.do("GET", "/audit?action=user.delete", nil, bearer(admin))
	expect(t, r, 200)
	rows := r.Body["data"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["entityId"] != otherID {
		t.Fatalf("audit: %s", r.Raw)
	}
}

func TestExportImport(t *testing.T) {
	e := setup(t)
	e.seedUser("admin@test.local", all...)
	admin, _ := e.login("admin@test.local")

	r := e.do("GET", "/users/export?format=csv", nil, bearer(admin))
	expect(t, r, 200)
	if !strings.Contains(string(r.Raw), "admin@test.local") || !strings.HasPrefix(r.Header.Get("Content-Disposition"), "attachment") {
		t.Fatalf("csv: %s", r.Raw)
	}
	r = e.do("GET", "/users/export?format=xlsx", nil, bearer(admin))
	expect(t, r, 200)
	if !bytes.HasPrefix(r.Raw, []byte("PK")) {
		t.Fatal("xlsx is not a zip")
	}

	csv := []byte("email,fullName,role\nok@test.local,Ok,role-admin@test.local\nbad,Bad,nope\n")
	r = e.upload("/users/import/preview", "file", "u.csv", csv, admin)
	expect(t, r, 200)
	rows := r.Body["data"].([]any)
	if len(rows) != 2 || len(rows[0].(map[string]any)["errors"].([]any)) != 0 || len(rows[1].(map[string]any)["errors"].([]any)) == 0 {
		t.Fatalf("preview: %s", r.Raw)
	}
	r = e.upload("/users/import", "file", "u.csv", csv, admin)
	expect(t, r, 201)
	if r.data()["created"].(float64) != 1 {
		t.Fatalf("import: %s", r.Raw)
	}
}

func TestSettingsAndRoles(t *testing.T) {
	e := setup(t)
	e.seedUser("admin@test.local", all...)
	admin, _ := e.login("admin@test.local")

	r := e.do("GET", "/settings/public", nil)
	expect(t, r, 200)
	if r.data()["appName"] != "Rekanara" {
		t.Fatalf("defaults: %s", r.Raw)
	}
	cur := e.do("GET", "/settings", nil, bearer(admin)).data()
	cur["appName"] = "Govite"
	cur["supportEmail"] = "  "
	expect(t, e.do("PUT", "/settings", cur, bearer(admin)), 200)
	if e.do("GET", "/settings/public", nil).data()["appName"] != "Govite" {
		t.Fatal("settings not saved")
	}

	r = e.do("POST", "/roles", map[string]any{"name": "Editor"}, bearer(admin))
	expect(t, r, 201)
	id := r.data()["id"].(string)
	expect(t, e.do("POST", "/roles", map[string]any{"name": "Editor"}, bearer(admin)), 409, "ROLE_NAME_TAKEN")
	r = e.do("PUT", "/roles/"+id+"/permissions", map[string]any{"permissions": []map[string]string{{"resource": "audit", "action": "view", "scope": "all"}}}, bearer(admin))
	expect(t, r, 200)
	expect(t, e.do("PUT", "/roles/"+id+"/permissions", map[string]any{"permissions": []map[string]string{{"resource": "audit", "action": "nope", "scope": "all"}}}, bearer(admin)), 400, "VALIDATION_ERROR")
	expect(t, e.do("DELETE", "/roles/"+id, nil, bearer(admin)), 200)
	expect(t, e.do("GET", "/permissions", nil, bearer(admin)), 200)
}

func TestAnnouncementsNotificationsAndAPIKeys(t *testing.T) {
	e := setup(t)
	e.seedUser("admin@test.local", all...)
	e.seedUser("reader@test.local", policy.Permission{Resource: "announcements", Action: "view"})
	admin, _ := e.login("admin@test.local")
	reader, _ := e.login("reader@test.local")

	// Draft is visible to managers but hidden from readers.
	r := e.do("POST", "/announcements", map[string]any{"title": "Draft", "content": "Not yet", "status": "draft"}, bearer(admin))
	expect(t, r, 201)
	draftID := r.data()["id"].(string)
	expect(t, e.do("GET", "/announcements/"+draftID, nil, bearer(reader)), 404, "ANNOUNCEMENT_NOT_FOUND")
	expect(t, e.do("GET", "/announcements/manage/"+draftID, nil, bearer(admin)), 200)

	// Publishing writes one notification for every active user, once.
	r = e.do("PATCH", "/announcements/"+draftID, map[string]string{"status": "published"}, bearer(admin))
	expect(t, r, 200)
	expect(t, e.do("PATCH", "/announcements/"+draftID, map[string]string{"title": "Edited"}, bearer(admin)), 200)
	inbox := e.do("GET", "/notifications", nil, bearer(reader))
	expect(t, inbox, 200)
	if len(inbox.Body["data"].([]any)) != 1 || inbox.Body["meta"].(map[string]any)["unreadCount"].(float64) != 1 {
		t.Fatalf("inbox: %s", inbox.Raw)
	}
	id := inbox.Body["data"].([]any)[0].(map[string]any)["id"].(string)
	expect(t, e.do("POST", "/notifications/"+id+"/read", nil, bearer(reader)), 201)
	expect(t, e.do("POST", "/notifications/read-all", nil, bearer(reader)), 204)

	// API key plaintext is shown once; list only carries its prefix/hash metadata.
	r = e.do("POST", "/api-keys", map[string]string{"name": "CI"}, bearer(admin))
	expect(t, r, 201)
	keyID, plain := r.data()["id"].(string), r.data()["key"].(string)
	if !strings.HasPrefix(plain, "rk_") || len(plain) < 40 {
		t.Fatalf("api key: %s", r.Raw)
	}
	r = e.do("GET", "/api-keys", nil, bearer(admin))
	expect(t, r, 200)
	if strings.Contains(string(r.Raw), plain) {
		t.Fatal("plaintext API key leaked in list")
	}
	expect(t, e.do("PATCH", "/api-keys/"+keyID, map[string]bool{"isActive": false}, bearer(admin)), 200)
	expect(t, e.do("DELETE", "/api-keys/"+keyID, nil, bearer(admin)), 204)
}

func TestMidtransWebhook(t *testing.T) {
	e := setup(t)
	e.seedUser("admin@test.local", all...)
	admin, _ := e.login("admin@test.local")
	p := map[string]string{"order_id": "o1", "status_code": "200", "gross_amount": "10000.00", "transaction_status": "settlement", "transaction_id": "t1"}
	sum := sha512.Sum512([]byte(p["order_id"] + p["status_code"] + p["gross_amount"] + "test-server-key"))
	p["signature_key"] = hex.EncodeToString(sum[:])

	r := e.do("POST", "/webhooks/midtrans", p)
	expect(t, r, 200)
	if r.data()["duplicate"] != false {
		t.Fatalf("first: %s", r.Raw)
	}
	r = e.do("POST", "/webhooks/midtrans", p)
	expect(t, r, 200)
	if r.data()["duplicate"] != true {
		t.Fatalf("duplicate: %s", r.Raw)
	}
	bad := maps.Clone(p)
	bad["signature_key"] = strings.Repeat("a", 128)
	expect(t, e.do("POST", "/webhooks/midtrans", bad), 401, "INVALID_WEBHOOK_SIGNATURE")
	r = e.do("GET", "/webhooks/events", nil, bearer(admin))
	expect(t, r, 200)
	if len(r.Body["data"].([]any)) != 1 || r.Body["data"].([]any)[0].(map[string]any)["status"] != "processed" {
		t.Fatalf("events: %s", r.Raw)
	}
}

func TestStorageAvatarAttachmentsAndBranding(t *testing.T) {
	e := setup(t)
	if e.f == nil || os.Getenv("TEST_MINIO_ENDPOINT") == "" {
		t.Skip("TEST_MINIO_ENDPOINT not set")
	}
	e.seedUser("admin@test.local", all...)
	admin, _ := e.login("admin@test.local")
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0, 0}

	r := e.upload("/account/avatar", "file", "avatar.png", png, admin)
	expect(t, r, 201)
	if !strings.Contains(r.data()["avatarUrl"].(string), "/avatars/public/") {
		t.Fatalf("avatar: %s", r.Raw)
	}
	expect(t, e.upload("/account/avatar", "file", "x.svg", []byte("<svg/>"), admin), 400, "INVALID_AVATAR")

	r = e.upload("/attachments", "file", "note.txt", []byte("private"), admin)
	expect(t, r, 201)
	attID := r.data()["id"].(string)
	if !strings.Contains(r.data()["url"].(string), "X-Amz-Signature") {
		t.Fatalf("attachment URL is not signed: %s", r.Raw)
	}
	expect(t, e.do("DELETE", "/attachments/"+attID, nil, bearer(admin)), 204)

	// Settings and announcement images use magic-byte detection and public URLs.
	r = e.upload("/settings/image", "image", "logo.png", png, admin)
	expect(t, r, 201)
	if !strings.Contains(r.data()["url"].(string), "/branding/public/") {
		t.Fatalf("branding: %s", r.Raw)
	}
	r = e.upload("/announcements/image", "image", "cover.png", png, admin)
	expect(t, r, 201)
	if !strings.Contains(r.data()["url"].(string), "/announcements/public/") {
		t.Fatalf("announcement image: %s", r.Raw)
	}
}

func TestMetricsAndCleanup(t *testing.T) {
	e := setup(t)
	e.seedUser("admin@test.local", all...)
	admin, _ := e.login("admin@test.local")

	r := e.do("GET", "/metrics", nil)
	expect(t, r, 200)
	if !strings.Contains(string(r.Raw), "go_goroutines") || !strings.Contains(r.Header.Get("Content-Type"), "text/plain") {
		t.Fatalf("metrics: %s", r.Raw)
	}
	// Expired/used token rows are removed by the manual cleanup endpoint.
	_, err := e.pool.Exec(context.Background(), `INSERT INTO user_token(id,user_id,purpose,channel,token_hash,expires_at,used_at) SELECT $1,id,'x','web',$2,now()-interval '100 days',now() FROM "user" LIMIT 1`, kernel.NewID(), crypto.RandomToken(32))
	must(t, err)
	r = e.do("POST", "/system/maintenance/cleanup", nil, bearer(admin))
	expect(t, r, 201)
	if r.data()["ran"] != true || r.data()["deleted"].(map[string]any)["userTokens"].(float64) < 1 {
		t.Fatalf("cleanup: %s", r.Raw)
	}
}

func TestOpenAPICoversRuntimeRoutes(t *testing.T) {
	e := setup(t)
	r := e.doAbsolute("GET", "/api/docs-json", nil)
	expect(t, r, 200)
	var spec struct {
		OpenAPI string                                `json:"openapi"`
		Paths   map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(r.Raw, &spec); err != nil || spec.OpenAPI != "3.0.0" {
		t.Fatalf("bad OpenAPI document: %v", err)
	}
	if y := e.doAbsolute("GET", "/api/docs-yaml", nil); y.Code != 200 || bytes.HasPrefix(y.Raw, []byte("{")) || !strings.Contains(string(y.Raw), "\nopenapi: 3.0.0\n") {
		t.Fatalf("docs-yaml is not YAML: %.80q", y.Raw)
	}

	documented := map[string]bool{}
	for path, methods := range spec.Paths {
		for method := range methods {
			switch method {
			case "get", "post", "put", "patch", "delete":
				documented[strings.ToUpper(method)+" "+path] = true
			}
		}
	}
	for _, methodRoutes := range e.f.Stack() {
		for _, route := range methodRoutes {
			if route.Method == "HEAD" || route.Path == "/" || route.Path == "/*" || route.Path == "/api/v1" || strings.HasPrefix(route.Path, "/api/docs") {
				continue
			}
			path := regexp.MustCompile(`:([A-Za-z0-9_]+)`).ReplaceAllString(route.Path, `{$1}`)
			key := route.Method + " " + path
			if !documented[key] {
				t.Errorf("runtime route missing from OpenAPI: %s", key)
			}
			delete(documented, key)
		}
	}
	for route := range documented {
		t.Errorf("OpenAPI route missing at runtime: %s", route)
	}

	ui := e.doAbsolute("GET", "/api/docs/", nil)
	for range 5 {
		if ui.Code != 307 && ui.Code != 308 {
			break
		}
		ui = e.doAbsolute("GET", ui.Header.Get("Location"), nil)
	}
	expect(t, ui, 200)
	if !bytes.Contains(ui.Raw, []byte("Govite Fullstack API")) {
		t.Fatal("Swagger UI title missing")
	}
}

func (e *env) doAbsolute(method, path string, body io.Reader) res {
	req := httptest.NewRequest(method, path, body)
	return e.send(req)
}
