package infrastructure

import (
	"context"
	"net/url"
	"time"

	"github.com/pquerna/otp/totp"

	"govite/internal/kernel"
	"govite/internal/platform/crypto"
	"govite/internal/platform/mail"
	"govite/internal/policy"
)

// Crypto adapts platform/crypto to domain.Crypto.
type Crypto struct{ Keys crypto.Keys }

func (Crypto) HashPassword(p string) (string, error) { return crypto.HashPassword(p) }
func (Crypto) VerifyPassword(h, p string) bool       { return crypto.VerifyPassword(h, p) }
func (Crypto) BurnPassword(p string)                 { crypto.VerifyPassword(crypto.DummyHash, p) }
func (c Crypto) HMAC(v string) string                { return c.Keys.HMAC(v) }
func (Crypto) SHA256(v string) string                { return crypto.SHA256Hex(v) }
func (c Crypto) Encrypt(p string) string             { return c.Keys.Encrypt(p) }
func (c Crypto) Decrypt(e string) (string, error)    { return c.Keys.Decrypt(e) }
func (Crypto) Token(n int) string                    { return crypto.RandomToken(n) }
func (Crypto) Digits(n int) string                   { return crypto.RandomDigits(n) }

func (c Crypto) SignAccess(userID, roleID, sid string) (string, int, error) {
	return c.Keys.SignAccess(userID, roleID, sid)
}

func (c Crypto) ParseAccess(token string) (kernel.Principal, error) {
	cl, err := c.Keys.ParseAccess(token)
	if err != nil {
		return kernel.Principal{}, err
	}
	return kernel.Principal{ID: cl.Subject, RoleID: cl.RoleID, SessionID: cl.SID}, nil
}

const totpIssuer = "Rekanara"

// TOTP adapts pquerna/otp to domain.OTP.
type TOTP struct{}

func (TOTP) Generate(account string) (string, error) {
	key, err := totp.Generate(totp.GenerateOpts{Issuer: totpIssuer, AccountName: account})
	if err != nil {
		return "", err
	}
	return key.Secret(), nil
}

func (TOTP) URL(account, secret string) string {
	v := url.Values{"secret": {secret}, "issuer": {totpIssuer}, "algorithm": {"SHA1"}, "digits": {"6"}, "period": {"30"}}
	return "otpauth://totp/" + url.PathEscape(totpIssuer+":"+account) + "?" + v.Encode()
}

func (TOTP) Validate(code, secret string) bool { return totp.Validate(code, secret) }

// Mailer renders the auth emails over a mail.Sender.
type Mailer struct{ Sender mail.Sender }

func (m Mailer) PasswordResetCode(ctx context.Context, to, code string, ttl time.Duration) error {
	exp := policy.FormatDuration(ttl)
	return m.Sender.Send(ctx, mail.Message{
		To: to, Subject: "Your password reset code",
		Text: "Your password reset code is " + code + ". It expires in " + exp + ".",
		Content: &mail.Content{
			Preheader: "Your code is " + code + ".", Heading: "Your password reset code",
			Paragraphs: []string{"Enter this code in the app to reset your password."}, Code: code,
			Note: "This code expires in " + exp + ". If you didn't request it, you can ignore this email; your password stays the same.",
		},
	})
}

func (m Mailer) PasswordResetLink(ctx context.Context, to, link string, ttl time.Duration) error {
	exp := policy.FormatDuration(ttl)
	return m.Sender.Send(ctx, mail.Message{
		To: to, Subject: "Reset your password",
		Text: "Open this link to reset your password: " + link + "\n\nThis link expires in " + exp + ".",
		Content: &mail.Content{
			Preheader: "Reset your password.", Heading: "Reset your password",
			Paragraphs: []string{"We received a request to reset your password. Use the button below to choose a new one."},
			ButtonText: "Reset password", ButtonURL: link,
			Note: "This link expires in " + exp + ". If you didn't request it, you can ignore this email; your password stays the same.",
		},
	})
}

func (m Mailer) Invite(ctx context.Context, to, link string, ttl time.Duration) error {
	exp := policy.FormatDuration(ttl)
	return m.Sender.Send(ctx, mail.Message{
		To: to, Subject: "You have been invited",
		Text: "Set your password to activate your account: " + link + "\n\nThis link expires in " + exp + ".",
		Content: &mail.Content{
			Preheader: "Set your password to activate your account.", Heading: "You have been invited",
			Paragraphs: []string{"An account has been created for you. Set your password to activate it and sign in."},
			ButtonText: "Set your password", ButtonURL: link,
			Note: "This link expires in " + exp + ". If you weren't expecting this invite, you can ignore this email.",
		},
	})
}

func (m Mailer) VerifyEmail(ctx context.Context, to, link string, ttl time.Duration) error {
	exp := policy.FormatDuration(ttl)
	return m.Sender.Send(ctx, mail.Message{
		To: to, Subject: "Verify your email",
		Text: "Confirm your email address: " + link + "\n\nThis link expires in " + exp + ".",
		Content: &mail.Content{
			Preheader: "Confirm your email address.", Heading: "Verify your email",
			Paragraphs: []string{"Confirm that this is your email address to finish securing your account."},
			ButtonText: "Verify email", ButtonURL: link,
			Note: "This link expires in " + exp + ". If you didn't request this, you can ignore this email.",
		},
	})
}
