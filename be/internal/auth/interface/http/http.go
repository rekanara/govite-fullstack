// Package http mounts /auth and the account security endpoints.
package http

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"govite/internal/auth/application"
	"govite/internal/auth/domain"
	"govite/internal/platform/httpx"
	"govite/internal/policy"
	rbac "govite/internal/rbac/domain"
)

const (
	refreshCookie = "refresh_token"
	cookiePath    = "/api/v1/auth"
)

type Handler struct {
	Svc *application.Service
	// Grants resolves the caller's permissions for /auth/me.
	Grants func(c *fiber.Ctx, roleID string) ([]rbac.Grant, error)
	// AvatarURL turns a stored key into a public URL ("" when none).
	AvatarURL func(key string) string
}

// Authenticate requires a valid Bearer access token and sets the Principal.
func (h Handler) Authenticate(c *fiber.Ctx) error {
	tok, ok := strings.CutPrefix(c.Get(fiber.HeaderAuthorization), "Bearer ")
	if !ok {
		return domain.ErrUnauthorized
	}
	p, err := h.Svc.Authenticate(tok)
	if err != nil {
		return err
	}
	httpx.SetPrincipal(c, p)
	return c.Next()
}

// Native apps send `X-Client: mobile`: refresh token travels in the JSON body.
func isMobile(c *fiber.Ctx) bool { return strings.EqualFold(c.Get("X-Client"), "mobile") }

func setRefreshCookie(c *fiber.Ctx, token string) {
	c.Cookie(&fiber.Cookie{
		Name: refreshCookie, Value: token, Path: cookiePath, HTTPOnly: true, Secure: true, SameSite: "Strict",
		MaxAge: int(policy.Security.RefreshTokenTTL.Seconds()), Expires: time.Now().Add(policy.Security.RefreshTokenTTL),
	})
}

func clearRefreshCookie(c *fiber.Ctx) {
	c.Cookie(&fiber.Cookie{Name: refreshCookie, Value: "", Path: cookiePath, HTTPOnly: true, Secure: true, SameSite: "Strict", MaxAge: -1, Expires: time.Unix(0, 0)})
}

func readRefresh(c *fiber.Ctx) string {
	if isMobile(c) {
		var b struct {
			RefreshToken string `json:"refreshToken"`
		}
		_ = c.BodyParser(&b)
		return b.RefreshToken
	}
	return c.Cookies(refreshCookie)
}

func client(c *fiber.Ctx) domain.Client {
	return domain.Client{UserAgent: c.Get(fiber.HeaderUserAgent), IP: c.IP()}
}

type sessionResponse struct {
	RequiresTwoFactor           bool        `json:"requiresTwoFactor"`
	AccessToken                 string      `json:"accessToken"`
	AccessTokenExpiresInSeconds int         `json:"accessTokenExpiresInSeconds"`
	User                        sessionUser `json:"user"`
	RefreshToken                string      `json:"refreshToken,omitempty"`
	BackupCodes                 []string    `json:"backupCodes,omitempty"`
}

type sessionUser struct {
	ID     string `json:"id"`
	Email  string `json:"email"`
	RoleID string `json:"roleId"`
}

func deliver(c *fiber.Ctx, sess *domain.Session, ch *domain.Challenge) error {
	if ch != nil {
		return httpx.Created(c, struct {
			RequiresTwoFactor bool `json:"requiresTwoFactor"`
			*domain.Challenge
		}{true, ch})
	}
	res := sessionResponse{
		AccessToken: sess.AccessToken, AccessTokenExpiresInSeconds: sess.ExpiresIn,
		User:        sessionUser{sess.User.ID, sess.User.Email, sess.User.RoleID},
		BackupCodes: sess.BackupCodes,
	}
	if isMobile(c) {
		res.RefreshToken = sess.RefreshToken
	} else {
		setRefreshCookie(c, sess.RefreshToken)
	}
	return httpx.Created(c, res)
}

type loginReq struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,password"`
}

type verifyReq struct {
	ChallengeToken string `json:"challengeToken" validate:"required,min=32,max=256"`
	Code           string `json:"code" validate:"required,numeric,len=6"`
}

type backupReq struct {
	ChallengeToken string `json:"challengeToken" validate:"required,min=32,max=256"`
	BackupCode     string `json:"backupCode" validate:"required,min=1,max=32"`
}

type forgotReq struct {
	Email   string `json:"email" validate:"required,email"`
	Channel string `json:"channel" validate:"omitempty,oneof=web mobile"`
}

type resetReq struct {
	Channel     string `json:"channel" validate:"required,oneof=web mobile"`
	Token       string `json:"token" validate:"required_if=Channel web,omitempty,min=32,max=256"`
	Email       string `json:"email" validate:"required_if=Channel mobile,omitempty,email"`
	OTP         string `json:"otp" validate:"required_if=Channel mobile,omitempty,numeric,len=6"`
	NewPassword string `json:"newPassword" validate:"required,password"`
}

type acceptInviteReq struct {
	Token    string `json:"token" validate:"required,min=32,max=256"`
	FullName string `json:"fullName" validate:"required,min=1,max=100"`
	Password string `json:"password" validate:"required,password"`
}

type tokenReq struct {
	Token string `json:"token" validate:"required,min=32,max=256"`
}

// body binds and validates b, then runs fn.
func body[T any](fn func(c *fiber.Ctx, b T) error) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var b T
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		return fn(c, b)
	}
}

func noContent(c *fiber.Ctx, err error) error {
	if err != nil {
		return err
	}
	return httpx.NoContent(c)
}

// Routes mounts /auth. loginLimit throttles credential endpoints separately
// from the global limit, so loosening THROTTLE_LIMIT never loosens login.
func (h Handler) Routes(r fiber.Router, loginLimit, refreshLimit fiber.Handler) {
	g := r.Group("/auth")

	g.Post("/login", loginLimit, body(func(c *fiber.Ctx, b loginReq) error {
		sess, ch, err := h.Svc.Login(c.UserContext(), b.Email, b.Password, client(c))
		if err != nil {
			return err
		}
		return deliver(c, sess, ch)
	}))

	g.Post("/2fa/verify", loginLimit, body(func(c *fiber.Ctx, b verifyReq) error {
		sess, err := h.Svc.VerifyTwoFactor(c.UserContext(), b.ChallengeToken, b.Code, client(c))
		if err != nil {
			return err
		}
		return deliver(c, sess, nil)
	}))

	g.Post("/2fa/verify-backup", loginLimit, body(func(c *fiber.Ctx, b backupReq) error {
		sess, err := h.Svc.VerifyBackupCode(c.UserContext(), b.ChallengeToken, b.BackupCode, client(c))
		if err != nil {
			return err
		}
		return deliver(c, sess, nil)
	}))

	g.Post("/refresh", refreshLimit, func(c *fiber.Ctx) error {
		tok := readRefresh(c)
		if tok == "" {
			clearRefreshCookie(c)
			return domain.ErrMissingRefresh
		}
		sess, err := h.Svc.Refresh(c.UserContext(), tok)
		if err != nil {
			if err == domain.ErrInvalidRefresh || err == domain.ErrReuseDetected {
				clearRefreshCookie(c)
			}
			return err
		}
		res := fiber.Map{"accessToken": sess.AccessToken, "accessTokenExpiresInSeconds": sess.ExpiresIn}
		if isMobile(c) {
			res["refreshToken"] = sess.RefreshToken
		} else {
			setRefreshCookie(c, sess.RefreshToken)
		}
		return httpx.OK(c, res)
	})

	g.Post("/logout", refreshLimit, func(c *fiber.Ctx) error {
		if tok := readRefresh(c); tok != "" {
			if err := h.Svc.Logout(c.UserContext(), tok); err != nil {
				return err
			}
		}
		clearRefreshCookie(c)
		return httpx.NoContent(c)
	})

	g.Post("/forgot-password", loginLimit, body(func(c *fiber.Ctx, b forgotReq) error {
		return noContent(c, h.Svc.RequestPasswordReset(c.UserContext(), b.Email, b.Channel))
	}))
	g.Post("/reset-password", loginLimit, body(func(c *fiber.Ctx, b resetReq) error {
		return noContent(c, h.Svc.ResetPassword(c.UserContext(), b.Channel, b.Token, b.Email, b.OTP, b.NewPassword))
	}))
	g.Post("/accept-invite", loginLimit, body(func(c *fiber.Ctx, b acceptInviteReq) error {
		return noContent(c, h.Svc.AcceptInvite(c.UserContext(), b.Token, b.FullName, b.Password))
	}))
	g.Post("/verify-email", loginLimit, body(func(c *fiber.Ctx, b tokenReq) error {
		return noContent(c, h.Svc.VerifyEmail(c.UserContext(), b.Token))
	}))

	g.Get("/me", h.Authenticate, func(c *fiber.Ctx) error {
		u, err := h.Svc.Me(c.UserContext())
		if err != nil {
			return err
		}
		grants, err := h.Grants(c, u.RoleID)
		if err != nil {
			return err
		}
		var avatar any
		if u.AvatarKey != nil {
			if v := h.AvatarURL(*u.AvatarKey); v != "" {
				avatar = v
			}
		}
		return httpx.OK(c, fiber.Map{
			"id": u.ID, "email": u.Email, "fullName": u.FullName,
			"avatarUrl": avatar, "roleId": u.RoleID, "roleName": u.RoleName,
			"emailVerified": u.EmailVerifiedAt != nil, "permissions": grants,
		})
	})
}

type passwordReq struct {
	CurrentPassword string `json:"currentPassword" validate:"required"`
}

// AccountRoutes mounts the caller's security endpoints under /account.
func (h Handler) AccountRoutes(r fiber.Router) {
	g := r.Group("/account")

	g.Post("/password", body(func(c *fiber.Ctx, b struct {
		CurrentPassword string `json:"currentPassword" validate:"required"`
		NewPassword     string `json:"newPassword" validate:"required,password"`
	}) error {
		return noContent(c, h.Svc.ChangePassword(c.UserContext(), b.CurrentPassword, b.NewPassword))
	}))

	g.Get("/two-factor", func(c *fiber.Ctx) error {
		st, err := h.Svc.TwoFactorStatus(c.UserContext(), httpx.Principal(c).ID)
		if err != nil {
			return err
		}
		return httpx.OK(c, st)
	})

	g.Post("/two-factor/setup", func(c *fiber.Ctx) error {
		key, url, err := h.Svc.BeginSetup(c.UserContext(), httpx.Principal(c).ID)
		if err != nil {
			return err
		}
		return httpx.OK(c, fiber.Map{"manualKey": key, "otpauthUrl": url})
	})

	g.Post("/two-factor/enable", body(func(c *fiber.Ctx, b struct {
		Code string `json:"code" validate:"required,numeric,len=6"`
	}) error {
		codes, err := h.Svc.ConfirmSetup(c.UserContext(), httpx.Principal(c).ID, b.Code)
		if err != nil {
			return err
		}
		return httpx.OK(c, fiber.Map{"backupCodes": codes})
	}))

	g.Post("/two-factor/backup-codes", body(func(c *fiber.Ctx, b passwordReq) error {
		codes, err := h.Svc.RegenerateBackupCodes(c.UserContext(), httpx.Principal(c).ID, b.CurrentPassword)
		if err != nil {
			return err
		}
		return httpx.OK(c, fiber.Map{"backupCodes": codes})
	}))

	g.Post("/two-factor/reset", body(func(c *fiber.Ctx, b passwordReq) error {
		return noContent(c, h.Svc.ResetTwoFactor(c.UserContext(), b.CurrentPassword, false))
	}))
	g.Post("/two-factor/disable", body(func(c *fiber.Ctx, b passwordReq) error {
		return noContent(c, h.Svc.ResetTwoFactor(c.UserContext(), b.CurrentPassword, true))
	}))

	g.Post("/verify-email", func(c *fiber.Ctx) error {
		return noContent(c, h.Svc.SendVerification(c.UserContext()))
	})

	g.Get("/sessions", func(c *fiber.Ctx) error {
		rows, err := h.Svc.Sessions(c.UserContext())
		if err != nil {
			return err
		}
		return httpx.OK(c, rows)
	})

	g.Delete("/sessions", func(c *fiber.Ctx) error {
		return noContent(c, h.Svc.RevokeOtherSessions(c.UserContext()))
	})

	g.Delete("/sessions/:id", func(c *fiber.Ctx) error {
		id, err := httpx.ID(c, "id")
		if err != nil {
			return err
		}
		return noContent(c, h.Svc.RevokeSession(c.UserContext(), id))
	})
}
