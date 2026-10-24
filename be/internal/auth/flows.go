package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"

	"govite/internal/mail"
	"govite/internal/policy"
	"govite/internal/store"
)

func (s *Service) webLink(path, token string) string {
	return s.Cfg.WebURL + path + "?token=" + token
}

// RequestPasswordReset mails a link (web) or a 6-digit code (mobile).
// Unknown emails return nil so callers can't probe accounts.
func (s *Service) RequestPasswordReset(ctx context.Context, email, channel string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	u, err := s.Q.GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !canAuthenticate(u)) {
		return nil
	}
	if err != nil {
		return err
	}
	if channel == "mobile" {
		ttl := policy.Security.PasswordResetCodeTTL
		otp, err := s.IssueOTP(ctx, u.ID, "password_reset", ttl)
		if err != nil {
			return err
		}
		exp := policy.FormatDuration(ttl)
		return s.Mail.Send(ctx, mail.Message{
			To: email, Subject: "Your password reset code",
			Text: "Your password reset code is " + otp + ". It expires in " + exp + ".",
			Content: &mail.Content{
				Preheader: "Your code is " + otp + ".", Heading: "Your password reset code",
				Paragraphs: []string{"Enter this code in the app to reset your password."}, Code: otp,
				Note: "This code expires in " + exp + ". If you didn't request it, you can ignore this email; your password stays the same.",
			},
		})
	}
	ttl := policy.Security.PasswordResetLinkTTL
	tok, err := s.IssueLink(ctx, s.Q, u.ID, "password_reset", ttl)
	if err != nil {
		return err
	}
	link, exp := s.webLink("/reset-password", tok), policy.FormatDuration(ttl)
	return s.Mail.Send(ctx, mail.Message{
		To: email, Subject: "Reset your password",
		Text: "Open this link to reset your password: " + link + "\n\nThis link expires in " + exp + ".",
		Content: &mail.Content{
			Preheader: "Reset your password.", Heading: "Reset your password",
			Paragraphs: []string{"We received a request to reset your password. Use the button below to choose a new one."},
			ButtonText: "Reset password", ButtonURL: link,
			Note: "This link expires in " + exp + ". If you didn't request it, you can ignore this email; your password stays the same.",
		},
	})
}

// ResetPassword applies a new password from a link token or mobile OTP and
// signs out every session. Invalid tokens are silently ignored, like Nest.
func (s *Service) ResetPassword(ctx context.Context, channel, token, email, otp, newPassword string) error {
	var rec store.UserToken
	if channel == "web" {
		var err error
		if rec, err = s.FindLink(ctx, "password_reset", token); err != nil {
			return nil
		}
	} else {
		u, err := s.Q.GetUserByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
		if err != nil {
			return nil
		}
		var ok bool
		if rec, ok = s.FindOTP(ctx, u.ID, "password_reset", otp); !ok {
			return nil
		}
	}
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	return s.Tx(ctx, func(q *store.Queries) error {
		if n, err := q.UseUserToken(ctx, rec.ID); err != nil || n == 0 {
			return err
		}
		if err := q.SetPassword(ctx, store.SetPasswordParams{ID: rec.UserID, PasswordHash: &hash}); err != nil {
			return err
		}
		return q.RevokeUserFamilies(ctx, rec.UserID)
	})
}

// SendInvite mails a set-password link to a password-less user.
func (s *Service) SendInvite(ctx context.Context, userID, email string) error {
	ttl := policy.Security.InviteTTL
	tok, err := s.IssueLink(ctx, s.Q, userID, "invite", ttl)
	if err != nil {
		return err
	}
	link, exp := s.webLink("/accept-invite", tok), policy.FormatDuration(ttl)
	return s.Mail.Send(ctx, mail.Message{
		To: email, Subject: "You have been invited",
		Text: "Set your password to activate your account: " + link + "\n\nThis link expires in " + exp + ".",
		Content: &mail.Content{
			Preheader: "Set your password to activate your account.", Heading: "You have been invited",
			Paragraphs: []string{"An account has been created for you. Set your password to activate it and sign in."},
			ButtonText: "Set your password", ButtonURL: link,
			Note: "This link expires in " + exp + ". If you weren't expecting this invite, you can ignore this email.",
		},
	})
}

func (s *Service) AcceptInvite(ctx context.Context, token, fullName, password string) error {
	rec, err := s.FindLink(ctx, "invite", token)
	if err != nil {
		return ErrInvalidToken
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	name := strings.TrimSpace(fullName)
	return s.Tx(ctx, func(q *store.Queries) error {
		if n, err := q.UseUserToken(ctx, rec.ID); err != nil || n == 0 {
			return ErrInvalidToken
		}
		return q.AcceptInvite(ctx, store.AcceptInviteParams{ID: rec.UserID, FullName: &name, PasswordHash: &hash})
	})
}

func (s *Service) SendVerification(ctx context.Context, userID, email string) error {
	ttl := policy.Security.EmailVerificationTTL
	tok, err := s.IssueLink(ctx, s.Q, userID, "email_verify", ttl)
	if err != nil {
		return err
	}
	link, exp := s.webLink("/verify-email", tok), policy.FormatDuration(ttl)
	return s.Mail.Send(ctx, mail.Message{
		To: email, Subject: "Verify your email",
		Text: "Confirm your email address: " + link + "\n\nThis link expires in " + exp + ".",
		Content: &mail.Content{
			Preheader: "Confirm your email address.", Heading: "Verify your email",
			Paragraphs: []string{"Confirm that this is your email address to finish securing your account."},
			ButtonText: "Verify email", ButtonURL: link,
			Note: "This link expires in " + exp + ". If you didn't request this, you can ignore this email.",
		},
	})
}

func (s *Service) VerifyEmail(ctx context.Context, token string) error {
	rec, err := s.FindLink(ctx, "email_verify", token)
	if err != nil {
		return ErrInvalidToken
	}
	return s.Tx(ctx, func(q *store.Queries) error {
		if n, err := q.UseUserToken(ctx, rec.ID); err != nil || n == 0 {
			return ErrInvalidToken
		}
		return q.MarkEmailVerified(ctx, rec.UserID)
	})
}

// CheckPassword returns ErrInvalidCredentials unless password matches.
func (s *Service) CheckPassword(ctx context.Context, userID, password string) (store.User, error) {
	u, err := s.Q.GetUserByID(ctx, userID)
	if err != nil {
		return u, err
	}
	if u.PasswordHash == nil || !VerifyPassword(*u.PasswordHash, password) {
		return u, ErrInvalidCredentials
	}
	return u, nil
}
