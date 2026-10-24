package application

import (
	"context"
	"crypto/subtle"
	"strings"
	"time"

	"govite/internal/auth/domain"
	"govite/internal/kernel"
	"govite/internal/policy"
)

func principal(ctx context.Context) kernel.Principal { return kernel.PrincipalFrom(ctx) }

func (s *Service) webLink(path, token string) string { return s.WebURL + path + "?token=" + token }

// IssueLink stores a web link token, invalidating older ones of the purpose.
func (s *Service) IssueLink(ctx context.Context, userID, purpose string, ttl time.Duration) (string, error) {
	tok := s.Crypto.Token(32)
	return tok, s.Repo.IssueToken(ctx, userID, purpose, domain.ChannelWeb, s.Crypto.HMAC(tok), time.Now().Add(ttl))
}

func (s *Service) issueOTP(ctx context.Context, userID, purpose string, ttl time.Duration) (string, error) {
	otp := s.Crypto.Digits(6)
	return otp, s.Repo.IssueToken(ctx, userID, purpose, domain.ChannelMobile, s.otpHash(userID, otp), time.Now().Add(ttl))
}

func (s *Service) FindLink(ctx context.Context, purpose, token string) (domain.OneTimeToken, error) {
	return s.Repo.FindToken(ctx, purpose, s.Crypto.HMAC(token))
}

// findOTP checks a mobile code; each miss counts toward MaxCodeAttempts.
func (s *Service) findOTP(ctx context.Context, userID, purpose, otp string) (domain.OneTimeToken, bool) {
	rec, err := s.Repo.LatestOTP(ctx, userID, purpose, policy.Security.MaxCodeAttempts)
	if err != nil {
		return rec, false
	}
	if subtle.ConstantTimeCompare([]byte(rec.TokenHash), []byte(s.otpHash(userID, otp))) == 1 {
		return rec, true
	}
	_ = s.Repo.BumpAttempts(ctx, rec.ID)
	return rec, false
}

func (s *Service) otpHash(userID, otp string) string { return s.Crypto.HMAC(userID + ":" + otp) }

// RequestPasswordReset mails a link (web) or a 6-digit code (mobile).
// Unknown emails return nil so callers can't probe accounts.
func (s *Service) RequestPasswordReset(ctx context.Context, email, channel string) error {
	email = normEmail(email)
	u, err := s.Repo.UserByEmail(ctx, email)
	if notFound(err) || (err == nil && !u.CanAuthenticate()) {
		return nil
	}
	if err != nil {
		return err
	}
	if channel == domain.ChannelMobile {
		ttl := policy.Security.PasswordResetCodeTTL
		otp, err := s.issueOTP(ctx, u.ID, domain.PurposePasswordReset, ttl)
		if err != nil {
			return err
		}
		return s.Mail.PasswordResetCode(ctx, email, otp, ttl)
	}
	ttl := policy.Security.PasswordResetLinkTTL
	tok, err := s.IssueLink(ctx, u.ID, domain.PurposePasswordReset, ttl)
	if err != nil {
		return err
	}
	return s.Mail.PasswordResetLink(ctx, email, s.webLink("/reset-password", tok), ttl)
}

// ResetPassword applies a new password from a link token or mobile OTP and
// signs out every session. Invalid tokens are silently ignored, like Nest.
func (s *Service) ResetPassword(ctx context.Context, channel, token, email, otp, newPassword string) error {
	var rec domain.OneTimeToken
	if channel == domain.ChannelWeb {
		var err error
		if rec, err = s.FindLink(ctx, domain.PurposePasswordReset, token); err != nil {
			return nil
		}
	} else {
		u, err := s.Repo.UserByEmail(ctx, normEmail(email))
		if err != nil {
			return nil
		}
		var ok bool
		if rec, ok = s.findOTP(ctx, u.ID, domain.PurposePasswordReset, otp); !ok {
			return nil
		}
	}
	hash, err := s.Crypto.HashPassword(newPassword)
	if err != nil {
		return err
	}
	return s.Tx.Do(ctx, func(ctx context.Context) error {
		if ok, err := s.Repo.UseToken(ctx, rec.ID); err != nil || !ok {
			return err
		}
		if err := s.Repo.SetPassword(ctx, rec.UserID, hash); err != nil {
			return err
		}
		return s.Repo.RevokeUserFamilies(ctx, rec.UserID)
	})
}

// SendInvite mails a set-password link to a password-less user.
func (s *Service) SendInvite(ctx context.Context, userID, email string) error {
	ttl := policy.Security.InviteTTL
	tok, err := s.IssueLink(ctx, userID, domain.PurposeInvite, ttl)
	if err != nil {
		return err
	}
	return s.Mail.Invite(ctx, email, s.webLink("/accept-invite", tok), ttl)
}

func (s *Service) AcceptInvite(ctx context.Context, token, fullName, password string) error {
	rec, err := s.FindLink(ctx, domain.PurposeInvite, token)
	if err != nil {
		return domain.ErrInvalidToken
	}
	hash, err := s.Crypto.HashPassword(password)
	if err != nil {
		return err
	}
	return s.Tx.Do(ctx, func(ctx context.Context) error {
		if ok, err := s.Repo.UseToken(ctx, rec.ID); err != nil || !ok {
			return domain.ErrInvalidToken
		}
		return s.Repo.AcceptInvite(ctx, rec.UserID, strings.TrimSpace(fullName), hash)
	})
}

// SendVerification mails a verify link to the caller unless already verified.
func (s *Service) SendVerification(ctx context.Context) error {
	u, err := s.Repo.UserByID(ctx, principal(ctx).ID)
	if err != nil || u.EmailVerifiedAt != nil {
		return err
	}
	ttl := policy.Security.EmailVerificationTTL
	tok, err := s.IssueLink(ctx, u.ID, domain.PurposeEmailVerify, ttl)
	if err != nil {
		return err
	}
	return s.Mail.VerifyEmail(ctx, u.Email, s.webLink("/verify-email", tok), ttl)
}

func (s *Service) VerifyEmail(ctx context.Context, token string) error {
	rec, err := s.FindLink(ctx, domain.PurposeEmailVerify, token)
	if err != nil {
		return domain.ErrInvalidToken
	}
	return s.Tx.Do(ctx, func(ctx context.Context) error {
		if ok, err := s.Repo.UseToken(ctx, rec.ID); err != nil || !ok {
			return domain.ErrInvalidToken
		}
		return s.Repo.MarkEmailVerified(ctx, rec.UserID)
	})
}

// ChangePassword checks the current one and signs out every session.
func (s *Service) ChangePassword(ctx context.Context, current, next string) error {
	id := principal(ctx).ID
	if _, err := s.CheckPassword(ctx, id, current); err != nil {
		return err
	}
	hash, err := s.Crypto.HashPassword(next)
	if err != nil {
		return err
	}
	err = s.Tx.Do(ctx, func(ctx context.Context) error {
		if err := s.Repo.SetPassword(ctx, id, hash); err != nil {
			return err
		}
		return s.Repo.RevokeUserFamilies(ctx, id)
	})
	if err == nil {
		s.Audit.Record(ctx, "account.password.change", "user", id, nil, nil)
	}
	return err
}

func (s *Service) Sessions(ctx context.Context) ([]domain.SessionInfo, error) {
	p := principal(ctx)
	rows, err := s.Repo.Sessions(ctx, p.ID)
	for i := range rows {
		rows[i].Current = rows[i].ID == p.SessionID
	}
	return rows, err
}

func (s *Service) RevokeOtherSessions(ctx context.Context) error {
	p := principal(ctx)
	if err := s.Repo.RevokeOtherFamilies(ctx, p.ID, p.SessionID); err != nil {
		return err
	}
	s.Audit.Record(ctx, "account.sessions.revoke_others", "session", p.SessionID, nil, nil)
	return nil
}

func (s *Service) RevokeSession(ctx context.Context, id string) error {
	f, err := s.Repo.Family(ctx, id)
	if err != nil || f.UserID != principal(ctx).ID || f.RevokedAt != nil {
		return domain.ErrSessionNotFound
	}
	if err := s.Repo.RevokeFamily(ctx, id); err != nil {
		return err
	}
	s.Audit.Record(ctx, "account.session.revoke", "session", id, nil, nil)
	return nil
}
