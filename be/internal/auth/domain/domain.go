// Package domain holds credentials, sessions, one-time tokens and 2FA:
// entities, errors and the ports the auth use cases need.
package domain

import (
	"context"
	"time"

	"govite/internal/kernel"
)

var (
	ErrInvalidCredentials = kernel.E(kernel.Unauthenticated, "INVALID_CREDENTIALS", "Email or password is incorrect")
	ErrInvalidRefresh     = kernel.E(kernel.Unauthenticated, "INVALID_REFRESH_TOKEN", "Refresh token is invalid or expired")
	ErrReuseDetected      = kernel.E(kernel.Unauthenticated, "REFRESH_TOKEN_REUSE_DETECTED", "Refresh token reuse detected; session revoked")
	ErrMissingRefresh     = kernel.E(kernel.Unauthenticated, "MISSING_REFRESH_TOKEN", "No refresh token cookie present")
	ErrSessionUser        = kernel.E(kernel.Unauthenticated, "SESSION_USER_NOT_FOUND", "The signed-in user no longer exists or is inactive")
	ErrUnauthorized       = kernel.E(kernel.Unauthenticated, "UNAUTHORIZED", "Unauthorized")
	ErrInvalidToken       = kernel.E(kernel.Invalid, "INVALID_TOKEN", "Token is invalid or expired")
	ErrInvalidTOTP        = kernel.E(kernel.Unauthenticated, "INVALID_TWO_FACTOR_CODE", "Invalid two-factor code")
	ErrTOTPEnabled        = kernel.E(kernel.Conflict, "TWO_FACTOR_ALREADY_ENABLED", "Two-factor authentication is already enabled")
	ErrTOTPNotEnabled     = kernel.E(kernel.Conflict, "TWO_FACTOR_NOT_ENABLED", "Two-factor authentication is not enabled")
	ErrTOTPRequired       = kernel.E(kernel.Forbidden, "TWO_FACTOR_REQUIRED", "Your role requires two-factor authentication")
	ErrSessionNotFound    = kernel.E(kernel.NotFound, "SESSION_NOT_FOUND", "Session not found")
)

// One-time token purposes and channels.
const (
	PurposeTwoFactor     = "two_factor"
	PurposePasswordReset = "password_reset"
	PurposeInvite        = "invite"
	PurposeEmailVerify   = "email_verify"
	ChannelWeb           = "web"
	ChannelMobile        = "mobile"
)

type User struct {
	ID              string
	Email           string
	FullName        *string
	RoleID          string
	RoleName        string
	PasswordHash    *string
	AvatarKey       *string
	IsActive        bool
	EmailVerifiedAt *time.Time
}

// CanAuthenticate: active and has accepted the invite (has a password).
func (u User) CanAuthenticate() bool { return u.IsActive && u.PasswordHash != nil }

type Client struct{ UserAgent, IP string }

// Session is a fresh login: access token plus the opaque refresh token.
type Session struct {
	AccessToken  string
	ExpiresIn    int
	RefreshToken string
	User         User
	BackupCodes  []string
}

// Challenge is returned instead of a session when 2FA is required.
type Challenge struct {
	ChallengeToken string `json:"challengeToken"`
	SetupRequired  bool   `json:"setupRequired"`
	ManualKey      string `json:"manualKey,omitempty"`
	OtpauthURL     string `json:"otpauthUrl,omitempty"`
}

type RefreshToken struct {
	ID, FamilyID, UserID string
	ExpiresAt            time.Time
	UsedAt, RevokedAt    *time.Time
}

type Family struct {
	ID, UserID string
	RevokedAt  *time.Time
}

type SessionInfo struct {
	ID         string    `json:"id"`
	UserAgent  *string   `json:"userAgent"`
	IPAddress  *string   `json:"ipAddress"`
	CreatedAt  time.Time `json:"createdAt"`
	LastUsedAt time.Time `json:"lastUsedAt"`
	Current    bool      `json:"current"`
}

type OneTimeToken struct {
	ID, UserID, TokenHash string
}

type TwoFactor struct {
	SecretEncrypted string
	EnabledAt       *time.Time
}

func (t TwoFactor) Enabled() bool { return t.EnabledAt != nil }

// Repository persists everything auth owns. Missing rows are kernel.ErrNotFound.
type Repository interface {
	UserByEmail(ctx context.Context, email string) (User, error)
	UserByID(ctx context.Context, id string) (User, error)
	// UserWithRole also fills RoleName.
	UserWithRole(ctx context.Context, id string) (User, error)
	RoleRequiresTwoFactor(ctx context.Context, roleID string) (bool, error)
	TouchLastLogin(ctx context.Context, id string) error
	SetPassword(ctx context.Context, userID, hash string) error
	AcceptInvite(ctx context.Context, userID, fullName, hash string) error
	MarkEmailVerified(ctx context.Context, userID string) error

	TwoFactor(ctx context.Context, userID string) (TwoFactor, error)
	SetTwoFactorSecret(ctx context.Context, userID, encrypted string) error
	EnableTwoFactor(ctx context.Context, userID string) error
	DeleteTwoFactor(ctx context.Context, userID string) error
	ReplaceBackupCodes(ctx context.Context, userID string, hashes []string) error
	UseBackupCode(ctx context.Context, userID, hash string) (bool, error)
	CountBackupCodes(ctx context.Context, userID string) (int64, error)

	CreateFamily(ctx context.Context, id, userID string, userAgent, ip *string) error
	AddRefreshToken(ctx context.Context, familyID, hash string, expires time.Time) error
	FindRefreshToken(ctx context.Context, hash string) (RefreshToken, error)
	// MarkRefreshTokenUsed is compare-and-set: false when already used.
	MarkRefreshTokenUsed(ctx context.Context, id, replacedBy string) (bool, error)
	Family(ctx context.Context, id string) (Family, error)
	Sessions(ctx context.Context, userID string) ([]SessionInfo, error)
	RevokeFamily(ctx context.Context, id string) error
	RevokeUserFamilies(ctx context.Context, userID string) error
	RevokeOtherFamilies(ctx context.Context, userID, keepID string) error

	// IssueToken invalidates older tokens of the same purpose and channel.
	IssueToken(ctx context.Context, userID, purpose, channel, hash string, expires time.Time) error
	FindToken(ctx context.Context, purpose, hash string) (OneTimeToken, error)
	LatestOTP(ctx context.Context, userID, purpose string, maxAttempts int) (OneTimeToken, error)
	BumpAttempts(ctx context.Context, id string) error
	// UseToken is compare-and-set: false when already used.
	UseToken(ctx context.Context, id string) (bool, error)
}

// Crypto is the secrets port: hashing, peppered HMAC, AES-GCM, JWTs, randomness.
type Crypto interface {
	HashPassword(plain string) (string, error)
	VerifyPassword(hash, plain string) bool
	// BurnPassword spends the same time as a real verify (unknown users).
	BurnPassword(plain string)
	HMAC(v string) string
	SHA256(v string) string
	Encrypt(plain string) string
	Decrypt(enc string) (string, error)
	SignAccess(userID, roleID, sessionID string) (token string, expiresIn int, err error)
	ParseAccess(token string) (kernel.Principal, error)
	Token(bytes int) string
	Digits(n int) string
}

// OTP is TOTP enrolment and verification.
type OTP interface {
	Generate(account string) (secret string, err error)
	URL(account, secret string) string
	Validate(code, secret string) bool
}

// Mailer sends the auth emails; copy and layout live in the adapter.
type Mailer interface {
	PasswordResetCode(ctx context.Context, to, code string, ttl time.Duration) error
	PasswordResetLink(ctx context.Context, to, link string, ttl time.Duration) error
	Invite(ctx context.Context, to, link string, ttl time.Duration) error
	VerifyEmail(ctx context.Context, to, link string, ttl time.Duration) error
}
