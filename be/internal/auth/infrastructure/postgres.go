// Package infrastructure implements the auth ports: Postgres, crypto,
// TOTP and the transactional mails.
package infrastructure

import (
	"context"
	"time"

	"govite/internal/auth/domain"
	"govite/internal/kernel"
	"govite/internal/platform/postgres"
	"govite/internal/platform/postgres/store"
)

type Repository struct{ DB *postgres.DB }

func toUser(u store.User, role string) domain.User {
	return domain.User{ID: u.ID, Email: u.Email, FullName: u.FullName, RoleID: u.RoleID, RoleName: role, PasswordHash: u.PasswordHash, AvatarKey: u.AvatarKey, IsActive: u.IsActive, EmailVerifiedAt: u.EmailVerifiedAt}
}

func (r Repository) UserByEmail(ctx context.Context, email string) (domain.User, error) {
	u, err := r.DB.Q(ctx).GetUserByEmail(ctx, email)
	return toUser(u, ""), postgres.Err(err)
}

func (r Repository) UserByID(ctx context.Context, id string) (domain.User, error) {
	u, err := r.DB.Q(ctx).GetUserByID(ctx, id)
	return toUser(u, ""), postgres.Err(err)
}

func (r Repository) UserWithRole(ctx context.Context, id string) (domain.User, error) {
	u, err := r.DB.Q(ctx).GetUser(ctx, id)
	return toUser(u.User, u.RoleName), postgres.Err(err)
}

func (r Repository) RoleRequiresTwoFactor(ctx context.Context, roleID string) (bool, error) {
	role, err := r.DB.Q(ctx).GetRole(ctx, roleID)
	return role.Role.RequireTwoFactor, postgres.Err(err)
}

func (r Repository) TouchLastLogin(ctx context.Context, id string) error {
	return r.DB.Q(ctx).TouchLastLogin(ctx, id)
}

func (r Repository) SetPassword(ctx context.Context, userID, hash string) error {
	return r.DB.Q(ctx).SetPassword(ctx, store.SetPasswordParams{ID: userID, PasswordHash: &hash})
}

func (r Repository) AcceptInvite(ctx context.Context, userID, fullName, hash string) error {
	return r.DB.Q(ctx).AcceptInvite(ctx, store.AcceptInviteParams{ID: userID, FullName: &fullName, PasswordHash: &hash})
}

func (r Repository) MarkEmailVerified(ctx context.Context, userID string) error {
	return r.DB.Q(ctx).MarkEmailVerified(ctx, userID)
}

func (r Repository) TwoFactor(ctx context.Context, userID string) (domain.TwoFactor, error) {
	tf, err := r.DB.Q(ctx).GetTwoFactor(ctx, userID)
	return domain.TwoFactor{SecretEncrypted: tf.SecretEncrypted, EnabledAt: tf.EnabledAt}, postgres.Err(err)
}

func (r Repository) SetTwoFactorSecret(ctx context.Context, userID, encrypted string) error {
	return r.DB.Q(ctx).SetTwoFactorSecret(ctx, store.SetTwoFactorSecretParams{UserID: userID, SecretEncrypted: encrypted})
}

func (r Repository) EnableTwoFactor(ctx context.Context, userID string) error {
	return r.DB.Q(ctx).EnableTwoFactor(ctx, userID)
}

func (r Repository) DeleteTwoFactor(ctx context.Context, userID string) error {
	return r.DB.Q(ctx).DeleteTwoFactor(ctx, userID)
}

func (r Repository) ReplaceBackupCodes(ctx context.Context, userID string, hashes []string) error {
	q := r.DB.Q(ctx)
	if err := q.DeleteBackupCodes(ctx, userID); err != nil {
		return err
	}
	for _, h := range hashes {
		if err := q.InsertBackupCode(ctx, store.InsertBackupCodeParams{ID: kernel.NewID(), UserID: userID, CodeHash: h}); err != nil {
			return err
		}
	}
	return nil
}

func (r Repository) UseBackupCode(ctx context.Context, userID, hash string) (bool, error) {
	n, err := r.DB.Q(ctx).UseBackupCode(ctx, store.UseBackupCodeParams{UserID: userID, CodeHash: hash})
	return n > 0, err
}

func (r Repository) CountBackupCodes(ctx context.Context, userID string) (int64, error) {
	return r.DB.Q(ctx).CountBackupCodes(ctx, userID)
}

func (r Repository) CreateFamily(ctx context.Context, id, userID string, userAgent, ip *string) error {
	return r.DB.Q(ctx).CreateFamily(ctx, store.CreateFamilyParams{ID: id, UserID: userID, UserAgent: userAgent, IpAddress: ip})
}

func (r Repository) AddRefreshToken(ctx context.Context, familyID, hash string, expires time.Time) error {
	return r.DB.Q(ctx).AddRefreshToken(ctx, store.AddRefreshTokenParams{ID: kernel.NewID(), FamilyID: familyID, TokenHash: hash, ExpiresAt: expires})
}

func (r Repository) FindRefreshToken(ctx context.Context, hash string) (domain.RefreshToken, error) {
	t, err := r.DB.Q(ctx).FindRefreshToken(ctx, hash)
	return domain.RefreshToken{ID: t.ID, FamilyID: t.FamilyID, UserID: t.UserID, ExpiresAt: t.ExpiresAt, UsedAt: t.UsedAt, RevokedAt: t.RevokedAt}, postgres.Err(err)
}

func (r Repository) MarkRefreshTokenUsed(ctx context.Context, id, replacedBy string) (bool, error) {
	n, err := r.DB.Q(ctx).MarkRefreshTokenUsed(ctx, store.MarkRefreshTokenUsedParams{ID: id, ReplacedByTokenHash: &replacedBy})
	return n > 0, err
}

func (r Repository) Family(ctx context.Context, id string) (domain.Family, error) {
	f, err := r.DB.Q(ctx).GetFamily(ctx, id)
	return domain.Family{ID: f.ID, UserID: f.UserID, RevokedAt: f.RevokedAt}, postgres.Err(err)
}

func (r Repository) Sessions(ctx context.Context, userID string) ([]domain.SessionInfo, error) {
	rows, err := r.DB.Q(ctx).ListSessions(ctx, userID)
	out := make([]domain.SessionInfo, len(rows))
	for i, s := range rows {
		out[i] = domain.SessionInfo{ID: s.ID, UserAgent: s.UserAgent, IPAddress: s.IpAddress, CreatedAt: s.CreatedAt, LastUsedAt: s.LastUsedAt}
	}
	return out, err
}

func (r Repository) RevokeFamily(ctx context.Context, id string) error {
	return r.DB.Q(ctx).RevokeFamily(ctx, id)
}

func (r Repository) RevokeUserFamilies(ctx context.Context, userID string) error {
	return r.DB.Q(ctx).RevokeUserFamilies(ctx, userID)
}

func (r Repository) RevokeOtherFamilies(ctx context.Context, userID, keepID string) error {
	return r.DB.Q(ctx).RevokeOtherFamilies(ctx, store.RevokeOtherFamiliesParams{UserID: userID, ID: keepID})
}

func (r Repository) IssueToken(ctx context.Context, userID, purpose, channel, hash string, expires time.Time) error {
	q := r.DB.Q(ctx)
	if err := q.InvalidateUserTokens(ctx, store.InvalidateUserTokensParams{UserID: userID, Purpose: purpose, Channel: channel}); err != nil {
		return err
	}
	return q.CreateUserToken(ctx, store.CreateUserTokenParams{ID: kernel.NewID(), UserID: userID, Purpose: purpose, Channel: channel, TokenHash: hash, ExpiresAt: expires})
}

func (r Repository) FindToken(ctx context.Context, purpose, hash string) (domain.OneTimeToken, error) {
	t, err := r.DB.Q(ctx).FindUserToken(ctx, store.FindUserTokenParams{TokenHash: hash, Purpose: purpose})
	return domain.OneTimeToken{ID: t.ID, UserID: t.UserID, TokenHash: t.TokenHash}, postgres.Err(err)
}

func (r Repository) LatestOTP(ctx context.Context, userID, purpose string, maxAttempts int) (domain.OneTimeToken, error) {
	t, err := r.DB.Q(ctx).LatestOtp(ctx, store.LatestOtpParams{UserID: userID, Purpose: purpose, Attempts: int32(maxAttempts)})
	return domain.OneTimeToken{ID: t.ID, UserID: t.UserID, TokenHash: t.TokenHash}, postgres.Err(err)
}

func (r Repository) BumpAttempts(ctx context.Context, id string) error {
	return r.DB.Q(ctx).BumpUserTokenAttempts(ctx, id)
}

func (r Repository) UseToken(ctx context.Context, id string) (bool, error) {
	n, err := r.DB.Q(ctx).UseUserToken(ctx, id)
	return n > 0, err
}
