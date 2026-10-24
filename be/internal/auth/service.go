package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pquerna/otp/totp"

	"govite/internal/app"
	"govite/internal/httpx"
	"govite/internal/policy"
	"govite/internal/store"
)

var (
	ErrInvalidCredentials = httpx.E(401, "INVALID_CREDENTIALS", "Email or password is incorrect")
	ErrInvalidRefresh     = httpx.E(401, "INVALID_REFRESH_TOKEN", "Refresh token is invalid or expired")
	ErrReuseDetected      = httpx.E(401, "REFRESH_TOKEN_REUSE_DETECTED", "Refresh token reuse detected; session revoked")
	ErrMissingRefresh     = httpx.E(401, "MISSING_REFRESH_TOKEN", "No refresh token cookie present")
	ErrSessionUser        = httpx.E(401, "SESSION_USER_NOT_FOUND", "The signed-in user no longer exists or is inactive")
	ErrInvalidToken       = httpx.E(400, "INVALID_TOKEN", "Token is invalid or expired")
	ErrInvalidTOTP        = httpx.E(401, "INVALID_TWO_FACTOR_CODE", "Invalid two-factor code")
	ErrTOTPEnabled        = httpx.E(409, "TWO_FACTOR_ALREADY_ENABLED", "Two-factor authentication is already enabled")
	ErrTOTPNotEnabled     = httpx.E(409, "TWO_FACTOR_NOT_ENABLED", "Two-factor authentication is not enabled")
	ErrTOTPRequired       = httpx.E(403, "TWO_FACTOR_REQUIRED", "Your role requires two-factor authentication")
	ErrSessionNotFound    = httpx.E(404, "SESSION_NOT_FOUND", "Session not found")
)

const totpIssuer = "Rekanara"

// Service holds auth logic shared by the auth, account and users packages.
type Service struct {
	*app.Deps
	Keys Keys
}

func NewService(d *app.Deps) *Service {
	return &Service{Deps: d, Keys: NewKeys(d.Cfg.JWTSecret, d.Cfg.RefreshPepper)}
}

func canAuthenticate(u store.User) bool { return u.IsActive && u.PasswordHash != nil }

type Client struct{ UserAgent, IP string }

// Session is a fresh login: access token plus the opaque refresh token.
type Session struct {
	AccessToken  string
	ExpiresIn    int
	RefreshToken string
	User         store.User
	BackupCodes  []string
}

// Challenge is returned instead of a session when 2FA is required.
type Challenge struct {
	ChallengeToken string `json:"challengeToken"`
	SetupRequired  bool   `json:"setupRequired"`
	ManualKey      string `json:"manualKey,omitempty"`
	OtpauthURL     string `json:"otpauthUrl,omitempty"`
}

// Login verifies the password; returns a session or a 2FA challenge.
func (s *Service) Login(ctx context.Context, email, password string, cl Client) (*Session, *Challenge, error) {
	u, err := s.Q.GetUserByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !canAuthenticate(u)) {
		VerifyPassword(dummyHash, password) // same cost as a real miss
		return nil, nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, nil, err
	}
	if !VerifyPassword(*u.PasswordHash, password) {
		return nil, nil, ErrInvalidCredentials
	}
	role, err := s.Q.GetRole(ctx, u.RoleID)
	if err != nil {
		return nil, nil, err
	}
	tf, err := s.Q.GetTwoFactor(ctx, u.ID)
	optedIn := err == nil && tf.EnabledAt != nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, err
	}
	if role.Role.RequireTwoFactor || optedIn {
		ch, err := s.challenge(ctx, u)
		return nil, ch, err
	}
	sess, err := s.issueSession(ctx, u, cl)
	return sess, nil, err
}

func (s *Service) challenge(ctx context.Context, u store.User) (*Challenge, error) {
	tf, err := s.Q.GetTwoFactor(ctx, u.ID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	enrolled := err == nil
	ch := &Challenge{SetupRequired: !enrolled || tf.EnabledAt == nil}
	if ch.SetupRequired {
		var secret string
		if enrolled {
			if secret, err = s.Keys.Decrypt(tf.SecretEncrypted); err != nil {
				return nil, err
			}
		} else {
			key, err := totp.Generate(totp.GenerateOpts{Issuer: totpIssuer, AccountName: u.Email})
			if err != nil {
				return nil, err
			}
			secret = key.Secret()
			if err := s.Q.SetTwoFactorSecret(ctx, store.SetTwoFactorSecretParams{UserID: u.ID, SecretEncrypted: s.Keys.Encrypt(secret)}); err != nil {
				return nil, err
			}
		}
		ch.ManualKey, ch.OtpauthURL = secret, otpauthURL(u.Email, secret)
	}
	ch.ChallengeToken, err = s.IssueLink(ctx, s.Q, u.ID, "two_factor", policy.Security.TwoFactorChallengeTTL)
	return ch, err
}

// VerifyTwoFactor completes a challenge with a TOTP code. First-time setup
// enables 2FA and returns backup codes once.
func (s *Service) VerifyTwoFactor(ctx context.Context, challengeToken, code string, cl Client) (*Session, error) {
	tok, err := s.FindLink(ctx, "two_factor", challengeToken)
	if err != nil {
		return nil, ErrInvalidTOTP
	}
	tf, err := s.Q.GetTwoFactor(ctx, tok.UserID)
	if err != nil {
		return nil, ErrInvalidTOTP
	}
	secret, err := s.Keys.Decrypt(tf.SecretEncrypted)
	if err != nil || !totp.Validate(code, secret) {
		return nil, ErrInvalidTOTP
	}
	firstTime := tf.EnabledAt == nil
	var codes []string
	err = s.Tx(ctx, func(q *store.Queries) error {
		if n, err := q.UseUserToken(ctx, tok.ID); err != nil || n == 0 {
			return ErrInvalidTOTP
		}
		if err := q.EnableTwoFactor(ctx, tok.UserID); err != nil {
			return err
		}
		if firstTime {
			codes, err = s.regenerateBackupCodes(ctx, q, tok.UserID)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	u, err := s.Q.GetUserByID(ctx, tok.UserID)
	if err != nil || !canAuthenticate(u) {
		return nil, ErrInvalidCredentials
	}
	sess, err := s.issueSession(ctx, u, cl)
	if sess != nil {
		sess.BackupCodes = codes
	}
	return sess, err
}

// VerifyBackupCode completes a challenge with a single-use backup code.
func (s *Service) VerifyBackupCode(ctx context.Context, challengeToken, code string, cl Client) (*Session, error) {
	tok, err := s.FindLink(ctx, "two_factor", challengeToken)
	if err != nil {
		return nil, ErrInvalidTOTP
	}
	tf, err := s.Q.GetTwoFactor(ctx, tok.UserID)
	if err != nil || tf.EnabledAt == nil {
		return nil, ErrInvalidTOTP
	}
	err = s.Tx(ctx, func(q *store.Queries) error {
		n, err := q.UseBackupCode(ctx, store.UseBackupCodeParams{UserID: tok.UserID, CodeHash: backupHash(code)})
		if err != nil || n == 0 {
			return ErrInvalidTOTP
		}
		if n, err := q.UseUserToken(ctx, tok.ID); err != nil || n == 0 {
			return ErrInvalidTOTP
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	u, err := s.Q.GetUserByID(ctx, tok.UserID)
	if err != nil || !canAuthenticate(u) {
		return nil, ErrInvalidCredentials
	}
	return s.issueSession(ctx, u, cl)
}

func (s *Service) issueSession(ctx context.Context, u store.User, cl Client) (*Session, error) {
	familyID := app.NewID()
	refresh := RandomToken(48)
	err := s.Tx(ctx, func(q *store.Queries) error {
		ua := cl.UserAgent
		if len(ua) > 512 {
			ua = ua[:512]
		}
		if err := q.CreateFamily(ctx, store.CreateFamilyParams{ID: familyID, UserID: u.ID, UserAgent: httpx.Ptr(ua), IpAddress: httpx.Ptr(cl.IP)}); err != nil {
			return err
		}
		if err := q.AddRefreshToken(ctx, store.AddRefreshTokenParams{ID: app.NewID(), FamilyID: familyID, TokenHash: s.Keys.HMAC(refresh), ExpiresAt: time.Now().Add(policy.Security.RefreshTokenTTL)}); err != nil {
			return err
		}
		return q.TouchLastLogin(ctx, u.ID)
	})
	if err != nil {
		return nil, err
	}
	access, exp, err := s.Keys.SignAccess(u.ID, u.RoleID, familyID)
	return &Session{AccessToken: access, ExpiresIn: exp, RefreshToken: refresh, User: u}, err
}

// Refresh rotates the refresh token. Replaying a used token revokes its family.
func (s *Service) Refresh(ctx context.Context, token string) (*Session, error) {
	rec, err := s.Q.FindRefreshToken(ctx, s.Keys.HMAC(token))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInvalidRefresh
	}
	if err != nil {
		return nil, err
	}
	// Reuse wins over expiry: a replayed old token is still an attack signal.
	if rec.UsedAt != nil {
		_ = s.Q.RevokeFamily(ctx, rec.FamilyID)
		return nil, ErrReuseDetected
	}
	if rec.ExpiresAt.Before(time.Now()) || rec.RevokedAt != nil {
		return nil, ErrInvalidRefresh
	}
	u, err := s.Q.GetUserByID(ctx, rec.UserID)
	if err != nil {
		return nil, ErrInvalidRefresh
	}
	if !u.IsActive {
		_ = s.Q.RevokeFamily(ctx, rec.FamilyID)
		return nil, ErrInvalidRefresh
	}
	next := RandomToken(48)
	nextHash := s.Keys.HMAC(next)
	n, err := s.Q.MarkRefreshTokenUsed(ctx, store.MarkRefreshTokenUsedParams{ID: rec.ID, ReplacedByTokenHash: &nextHash})
	if err != nil {
		return nil, err
	}
	if n == 0 { // lost a concurrent race: treat like a replay
		_ = s.Q.RevokeFamily(ctx, rec.FamilyID)
		return nil, ErrReuseDetected
	}
	if err := s.Q.AddRefreshToken(ctx, store.AddRefreshTokenParams{ID: app.NewID(), FamilyID: rec.FamilyID, TokenHash: nextHash, ExpiresAt: time.Now().Add(policy.Security.RefreshTokenTTL)}); err != nil {
		return nil, err
	}
	access, exp, err := s.Keys.SignAccess(u.ID, u.RoleID, rec.FamilyID)
	return &Session{AccessToken: access, ExpiresIn: exp, RefreshToken: next, User: u}, err
}

// Logout revokes the token's family. Unknown tokens are ignored (idempotent).
func (s *Service) Logout(ctx context.Context, token string) error {
	rec, err := s.Q.FindRefreshToken(ctx, s.Keys.HMAC(token))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.Q.RevokeFamily(ctx, rec.FamilyID)
}

// --- one-time tokens (password reset, invite, email verify, 2FA challenge) ---

// IssueLink stores a link token, invalidating older ones of the same purpose.
func (s *Service) IssueLink(ctx context.Context, q *store.Queries, userID, purpose string, ttl time.Duration) (string, error) {
	tok := RandomToken(32)
	return tok, s.issue(ctx, q, userID, purpose, "web", s.Keys.HMAC(tok), ttl)
}

func (s *Service) IssueOTP(ctx context.Context, userID, purpose string, ttl time.Duration) (string, error) {
	otp := RandomDigits(6)
	return otp, s.issue(ctx, s.Q, userID, purpose, "mobile", s.otpHash(userID, otp), ttl)
}

func (s *Service) issue(ctx context.Context, q *store.Queries, userID, purpose, channel, hash string, ttl time.Duration) error {
	if err := q.InvalidateUserTokens(ctx, store.InvalidateUserTokensParams{UserID: userID, Purpose: purpose, Channel: channel}); err != nil {
		return err
	}
	return q.CreateUserToken(ctx, store.CreateUserTokenParams{ID: app.NewID(), UserID: userID, Purpose: purpose, Channel: channel, TokenHash: hash, ExpiresAt: time.Now().Add(ttl)})
}

func (s *Service) FindLink(ctx context.Context, purpose, token string) (store.UserToken, error) {
	return s.Q.FindUserToken(ctx, store.FindUserTokenParams{TokenHash: s.Keys.HMAC(token), Purpose: purpose})
}

// FindOTP checks a mobile code; each miss counts toward MaxCodeAttempts.
func (s *Service) FindOTP(ctx context.Context, userID, purpose, otp string) (store.UserToken, bool) {
	rec, err := s.Q.LatestOtp(ctx, store.LatestOtpParams{UserID: userID, Purpose: purpose, Attempts: int32(policy.Security.MaxCodeAttempts)})
	if err != nil {
		return rec, false
	}
	if subtle.ConstantTimeCompare([]byte(rec.TokenHash), []byte(s.otpHash(userID, otp))) == 1 {
		return rec, true
	}
	_ = s.Q.BumpUserTokenAttempts(ctx, rec.ID)
	return rec, false
}

func (s *Service) otpHash(userID, otp string) string { return s.Keys.HMAC(userID + ":" + otp) }

// --- backup codes ---

func backupHash(code string) string { return sha256Hex(strings.ToLower(strings.TrimSpace(code))) }

func (s *Service) regenerateBackupCodes(ctx context.Context, q *store.Queries, userID string) ([]string, error) {
	if err := q.DeleteBackupCodes(ctx, userID); err != nil {
		return nil, err
	}
	codes := make([]string, 10)
	for i := range codes {
		codes[i] = RandomDigits(5) + "-" + RandomDigits(5)
		if err := q.InsertBackupCode(ctx, store.InsertBackupCodeParams{ID: app.NewID(), UserID: userID, CodeHash: backupHash(codes[i])}); err != nil {
			return nil, err
		}
	}
	return codes, nil
}

func (s *Service) RegenerateBackupCodes(ctx context.Context, userID string) (codes []string, err error) {
	err = s.Tx(ctx, func(q *store.Queries) error {
		codes, err = s.regenerateBackupCodes(ctx, q, userID)
		return err
	})
	return codes, err
}

// BeginSetup starts account-initiated enrolment; inactive until ConfirmSetup.
func (s *Service) BeginSetup(ctx context.Context, userID, email string) (secret, url string, err error) {
	if tf, err := s.Q.GetTwoFactor(ctx, userID); err == nil && tf.EnabledAt != nil {
		return "", "", ErrTOTPEnabled
	}
	key, err := totp.Generate(totp.GenerateOpts{Issuer: totpIssuer, AccountName: email})
	if err != nil {
		return "", "", err
	}
	err = s.Q.SetTwoFactorSecret(ctx, store.SetTwoFactorSecretParams{UserID: userID, SecretEncrypted: s.Keys.Encrypt(key.Secret())})
	return key.Secret(), otpauthURL(email, key.Secret()), err
}

func (s *Service) ConfirmSetup(ctx context.Context, userID, code string) ([]string, error) {
	tf, err := s.Q.GetTwoFactor(ctx, userID)
	if err != nil || tf.EnabledAt != nil {
		return nil, ErrInvalidTOTP
	}
	secret, err := s.Keys.Decrypt(tf.SecretEncrypted)
	if err != nil || !totp.Validate(code, secret) {
		return nil, ErrInvalidTOTP
	}
	if err := s.Q.EnableTwoFactor(ctx, userID); err != nil {
		return nil, err
	}
	return s.RegenerateBackupCodes(ctx, userID)
}

func otpauthURL(email, secret string) string {
	v := url.Values{"secret": {secret}, "issuer": {totpIssuer}, "algorithm": {"SHA1"}, "digits": {"6"}, "period": {"30"}}
	return "otpauth://totp/" + url.PathEscape(totpIssuer+":"+email) + "?" + v.Encode()
}
