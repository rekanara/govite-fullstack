package application

import (
	"context"
	"strings"

	"govite/internal/auth/domain"
	"govite/internal/policy"
)

func (s *Service) challenge(ctx context.Context, u domain.User) (*domain.Challenge, error) {
	tf, err := s.Repo.TwoFactor(ctx, u.ID)
	if err != nil && !notFound(err) {
		return nil, err
	}
	enrolled := err == nil
	ch := &domain.Challenge{SetupRequired: !enrolled || !tf.Enabled()}
	if ch.SetupRequired {
		var secret string
		if enrolled {
			if secret, err = s.Crypto.Decrypt(tf.SecretEncrypted); err != nil {
				return nil, err
			}
		} else {
			if secret, err = s.OTP.Generate(u.Email); err != nil {
				return nil, err
			}
			if err := s.Repo.SetTwoFactorSecret(ctx, u.ID, s.Crypto.Encrypt(secret)); err != nil {
				return nil, err
			}
		}
		ch.ManualKey, ch.OtpauthURL = secret, s.OTP.URL(u.Email, secret)
	}
	ch.ChallengeToken, err = s.IssueLink(ctx, u.ID, domain.PurposeTwoFactor, policy.Security.TwoFactorChallengeTTL)
	return ch, err
}

// VerifyTwoFactor completes a challenge with a TOTP code. First-time setup
// enables 2FA and returns backup codes once.
func (s *Service) VerifyTwoFactor(ctx context.Context, challengeToken, code string, cl domain.Client) (*domain.Session, error) {
	tok, err := s.FindLink(ctx, domain.PurposeTwoFactor, challengeToken)
	if err != nil {
		return nil, domain.ErrInvalidTOTP
	}
	tf, err := s.Repo.TwoFactor(ctx, tok.UserID)
	if err != nil {
		return nil, domain.ErrInvalidTOTP
	}
	secret, err := s.Crypto.Decrypt(tf.SecretEncrypted)
	if err != nil || !s.OTP.Validate(code, secret) {
		return nil, domain.ErrInvalidTOTP
	}
	var codes []string
	err = s.Tx.Do(ctx, func(ctx context.Context) error {
		if ok, err := s.Repo.UseToken(ctx, tok.ID); err != nil || !ok {
			return domain.ErrInvalidTOTP
		}
		if err := s.Repo.EnableTwoFactor(ctx, tok.UserID); err != nil {
			return err
		}
		if !tf.Enabled() {
			codes, err = s.replaceBackupCodes(ctx, tok.UserID)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	u, err := s.Repo.UserByID(ctx, tok.UserID)
	if err != nil || !u.CanAuthenticate() {
		return nil, domain.ErrInvalidCredentials
	}
	sess, err := s.issueSession(ctx, u, cl)
	if sess != nil {
		sess.BackupCodes = codes
	}
	return sess, err
}

// VerifyBackupCode completes a challenge with a single-use backup code.
func (s *Service) VerifyBackupCode(ctx context.Context, challengeToken, code string, cl domain.Client) (*domain.Session, error) {
	tok, err := s.FindLink(ctx, domain.PurposeTwoFactor, challengeToken)
	if err != nil {
		return nil, domain.ErrInvalidTOTP
	}
	if tf, err := s.Repo.TwoFactor(ctx, tok.UserID); err != nil || !tf.Enabled() {
		return nil, domain.ErrInvalidTOTP
	}
	err = s.Tx.Do(ctx, func(ctx context.Context) error {
		if ok, err := s.Repo.UseBackupCode(ctx, tok.UserID, s.backupHash(code)); err != nil || !ok {
			return domain.ErrInvalidTOTP
		}
		if ok, err := s.Repo.UseToken(ctx, tok.ID); err != nil || !ok {
			return domain.ErrInvalidTOTP
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	u, err := s.Repo.UserByID(ctx, tok.UserID)
	if err != nil || !u.CanAuthenticate() {
		return nil, domain.ErrInvalidCredentials
	}
	return s.issueSession(ctx, u, cl)
}

func (s *Service) backupHash(code string) string {
	return s.Crypto.SHA256(strings.ToLower(strings.TrimSpace(code)))
}

func (s *Service) replaceBackupCodes(ctx context.Context, userID string) ([]string, error) {
	codes, hashes := make([]string, 10), make([]string, 10)
	for i := range codes {
		codes[i] = s.Crypto.Digits(5) + "-" + s.Crypto.Digits(5)
		hashes[i] = s.backupHash(codes[i])
	}
	return codes, s.Repo.ReplaceBackupCodes(ctx, userID, hashes)
}

// BeginSetup starts account-initiated enrolment; inactive until ConfirmSetup.
func (s *Service) BeginSetup(ctx context.Context, userID string) (secret, url string, err error) {
	u, err := s.Repo.UserByID(ctx, userID)
	if err != nil {
		return "", "", err
	}
	if tf, err := s.Repo.TwoFactor(ctx, userID); err == nil && tf.Enabled() {
		return "", "", domain.ErrTOTPEnabled
	}
	if secret, err = s.OTP.Generate(u.Email); err != nil {
		return "", "", err
	}
	err = s.Repo.SetTwoFactorSecret(ctx, userID, s.Crypto.Encrypt(secret))
	return secret, s.OTP.URL(u.Email, secret), err
}

// ConfirmSetup enables 2FA and issues backup codes atomically.
func (s *Service) ConfirmSetup(ctx context.Context, userID, code string) ([]string, error) {
	tf, err := s.Repo.TwoFactor(ctx, userID)
	if err != nil || tf.Enabled() {
		return nil, domain.ErrInvalidTOTP
	}
	secret, err := s.Crypto.Decrypt(tf.SecretEncrypted)
	if err != nil || !s.OTP.Validate(code, secret) {
		return nil, domain.ErrInvalidTOTP
	}
	var codes []string
	err = s.Tx.Do(ctx, func(ctx context.Context) error {
		if err := s.Repo.EnableTwoFactor(ctx, userID); err != nil {
			return err
		}
		codes, err = s.replaceBackupCodes(ctx, userID)
		return err
	})
	if err == nil {
		s.Audit.Record(ctx, "account.two_factor.enable", "user", userID, nil, nil)
	}
	return codes, err
}

type TwoFactorStatus struct {
	Enabled              bool  `json:"enabled"`
	Required             bool  `json:"required"`
	EnabledAt            any   `json:"enabledAt"`
	BackupCodesRemaining int64 `json:"backupCodesRemaining"`
}

func (s *Service) TwoFactorStatus(ctx context.Context, userID string) (TwoFactorStatus, error) {
	u, err := s.Repo.UserByID(ctx, userID)
	if err != nil {
		return TwoFactorStatus{}, err
	}
	required, err := s.Repo.RoleRequiresTwoFactor(ctx, u.RoleID)
	if err != nil {
		return TwoFactorStatus{}, err
	}
	st := TwoFactorStatus{Required: required}
	if tf, err := s.Repo.TwoFactor(ctx, userID); err == nil && tf.Enabled() {
		st.Enabled, st.EnabledAt = true, tf.EnabledAt
	}
	if n, err := s.Repo.CountBackupCodes(ctx, userID); err == nil {
		st.BackupCodesRemaining = n
	}
	return st, nil
}

// RegenerateBackupCodes needs the current password and enabled 2FA.
func (s *Service) RegenerateBackupCodes(ctx context.Context, userID, password string) (codes []string, err error) {
	if _, err := s.CheckPassword(ctx, userID, password); err != nil {
		return nil, err
	}
	if tf, err := s.Repo.TwoFactor(ctx, userID); err != nil || !tf.Enabled() {
		return nil, domain.ErrTOTPNotEnabled
	}
	err = s.Tx.Do(ctx, func(ctx context.Context) error {
		codes, err = s.replaceBackupCodes(ctx, userID)
		return err
	})
	if err == nil {
		s.Audit.Record(ctx, "account.two_factor.backup_codes", "user", userID, nil, nil)
	}
	return codes, err
}

// ResetTwoFactor wipes the secret and codes and signs out other sessions.
// disable additionally refuses when the role requires 2FA.
func (s *Service) ResetTwoFactor(ctx context.Context, password string, disable bool) error {
	p := principal(ctx)
	u, err := s.CheckPassword(ctx, p.ID, password)
	if disable && u.RoleID != "" {
		if required, rerr := s.Repo.RoleRequiresTwoFactor(ctx, u.RoleID); rerr == nil && required {
			return domain.ErrTOTPRequired
		}
	}
	if err != nil {
		return err
	}
	err = s.Tx.Do(ctx, func(ctx context.Context) error {
		if err := s.Repo.DeleteTwoFactor(ctx, p.ID); err != nil {
			return err
		}
		if err := s.Repo.ReplaceBackupCodes(ctx, p.ID, nil); err != nil {
			return err
		}
		return s.Repo.RevokeOtherFamilies(ctx, p.ID, p.SessionID)
	})
	if err != nil {
		return err
	}
	action := "account.two_factor.reset"
	if disable {
		action = "account.two_factor.disable"
	}
	s.Audit.Record(ctx, action, "user", p.ID, nil, nil)
	return nil
}
