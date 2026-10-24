// Package policy is the single source of truth for the permission catalog and
// security limits. `make gen` mirrors it into fe/packages/contracts.
package policy

import (
	"strconv"
	"time"
)

var Scopes = []string{"self", "team", "all"}

var Actions = []string{"manage", "view", "create", "edit", "delete"}

var Resources = []string{
	"users",
	"roles",
	"permissions",
	"audit",
	"settings",
	"announcements",
	"webhooks",
	"apiKeys",
}

type Permission struct {
	Resource string `json:"resource"`
	Action   string `json:"action"`
}

func (p Permission) Key() string { return p.Resource + "." + p.Action }

// Add a pair here (then `make gen` and rerun seed) before using it in a route.
var Permissions = []Permission{
	{"users", "manage"},
	{"roles", "manage"},
	{"permissions", "view"},
	{"audit", "view"},
	{"settings", "manage"},
	{"announcements", "view"},
	{"announcements", "manage"},
	{"webhooks", "manage"},
	{"apiKeys", "manage"},
}

func IsKnown(resource, action string) bool {
	for _, p := range Permissions {
		if p.Resource == resource && p.Action == action {
			return true
		}
	}
	return false
}

// Revoking the last holder of these would lock every admin out.
var Guarded = []string{"users.manage", "roles.manage"}

// Security limits: the API enforces them, the UI displays them.
var Security = struct {
	PasswordMinLength     int
	PasswordMaxLength     int
	AccessTokenTTL        time.Duration
	RefreshTokenTTL       time.Duration
	PasswordResetLinkTTL  time.Duration
	PasswordResetCodeTTL  time.Duration
	InviteTTL             time.Duration
	EmailVerificationTTL  time.Duration
	TwoFactorChallengeTTL time.Duration
	MaxCodeAttempts       int
}{
	PasswordMinLength:     8,
	PasswordMaxLength:     128,
	AccessTokenTTL:        15 * time.Minute,
	RefreshTokenTTL:       7 * 24 * time.Hour,
	PasswordResetLinkTTL:  30 * time.Minute,
	PasswordResetCodeTTL:  10 * time.Minute,
	InviteTTL:             7 * 24 * time.Hour,
	EmailVerificationTTL:  24 * time.Hour,
	TwoFactorChallengeTTL: 5 * time.Minute,
	MaxCodeAttempts:       5,
}

// FormatDuration renders "30 minutes", "1 day", "7 days" for emails.
func FormatDuration(d time.Duration) string {
	units := []struct {
		name string
		size time.Duration
	}{{"day", 24 * time.Hour}, {"hour", time.Hour}, {"minute", time.Minute}}
	for _, u := range units {
		if d >= u.size && d%u.size == 0 {
			n := int(d / u.size)
			if n == 1 {
				return "1 " + u.name
			}
			return strconv.Itoa(n) + " " + u.name + "s"
		}
	}
	return strconv.Itoa(int(d.Seconds())) + " seconds"
}
