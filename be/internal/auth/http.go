package auth

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"govite/internal/app"
	"govite/internal/httpx"
	"govite/internal/policy"
	"govite/internal/rbac"
)

const (
	refreshCookie = "refresh_token"
	cookiePath    = "/api/v1/auth"
)

// Authenticate requires a valid Bearer access token and sets the Principal.
func (s *Service) Authenticate(c *fiber.Ctx) error {
	h := c.Get(fiber.HeaderAuthorization)
	tok, ok := strings.CutPrefix(h, "Bearer ")
	if !ok {
		return httpx.E(401, "UNAUTHORIZED", "Unauthorized")
	}
	claims, err := s.Keys.ParseAccess(tok)
	if err != nil {
		return httpx.E(401, "UNAUTHORIZED", "Unauthorized")
	}
	app.SetPrincipal(c, app.Principal{ID: claims.Subject, RoleID: claims.RoleID, SessionID: claims.SID})
	return c.Next()
}

// Native apps send `X-Client: mobile`: refresh token travels in the JSON body.
func isMobile(c *fiber.Ctx) bool { return strings.EqualFold(c.Get("X-Client"), "mobile") }

func (s *Service) setRefreshCookie(c *fiber.Ctx, token string) {
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

func client(c *fiber.Ctx) Client { return Client{UserAgent: c.Get(fiber.HeaderUserAgent), IP: c.IP()} }

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

func (s *Service) deliver(c *fiber.Ctx, sess *Session, ch *Challenge) error {
	if ch != nil {
		return httpx.Created(c, struct {
			RequiresTwoFactor bool `json:"requiresTwoFactor"`
			*Challenge
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
		s.setRefreshCookie(c, sess.RefreshToken)
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

// Routes mounts /auth. loginLimit throttles credential endpoints separately
// from the global limit, so loosening THROTTLE_LIMIT never loosens login.
func (s *Service) Routes(r fiber.Router, rb *rbac.RBAC, loginLimit fiber.Handler) {
	g := r.Group("/auth")

	g.Post("/login", loginLimit, func(c *fiber.Ctx) error {
		var b loginReq
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		sess, ch, err := s.Login(c.UserContext(), b.Email, b.Password, client(c))
		if err != nil {
			return err
		}
		return s.deliver(c, sess, ch)
	})

	g.Post("/2fa/verify", loginLimit, func(c *fiber.Ctx) error {
		var b verifyReq
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		sess, err := s.VerifyTwoFactor(c.UserContext(), b.ChallengeToken, b.Code, client(c))
		if err != nil {
			return err
		}
		return s.deliver(c, sess, nil)
	})

	g.Post("/2fa/verify-backup", loginLimit, func(c *fiber.Ctx) error {
		var b backupReq
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		sess, err := s.VerifyBackupCode(c.UserContext(), b.ChallengeToken, b.BackupCode, client(c))
		if err != nil {
			return err
		}
		return s.deliver(c, sess, nil)
	})

	g.Post("/refresh", func(c *fiber.Ctx) error {
		tok := readRefresh(c)
		if tok == "" {
			clearRefreshCookie(c)
			return ErrMissingRefresh
		}
		sess, err := s.Refresh(c.UserContext(), tok)
		if err != nil {
			if err == ErrInvalidRefresh || err == ErrReuseDetected {
				clearRefreshCookie(c)
			}
			return err
		}
		res := fiber.Map{"accessToken": sess.AccessToken, "accessTokenExpiresInSeconds": sess.ExpiresIn}
		if isMobile(c) {
			res["refreshToken"] = sess.RefreshToken
		} else {
			s.setRefreshCookie(c, sess.RefreshToken)
		}
		return httpx.OK(c, res)
	})

	g.Post("/logout", func(c *fiber.Ctx) error {
		if tok := readRefresh(c); tok != "" {
			if err := s.Logout(c.UserContext(), tok); err != nil {
				return err
			}
		}
		clearRefreshCookie(c)
		return httpx.NoContent(c)
	})

	g.Post("/forgot-password", loginLimit, func(c *fiber.Ctx) error {
		var b forgotReq
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		if err := s.RequestPasswordReset(c.UserContext(), b.Email, b.Channel); err != nil {
			return err
		}
		return httpx.NoContent(c)
	})

	g.Post("/reset-password", loginLimit, func(c *fiber.Ctx) error {
		var b resetReq
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		if err := s.ResetPassword(c.UserContext(), b.Channel, b.Token, b.Email, b.OTP, b.NewPassword); err != nil {
			return err
		}
		return httpx.NoContent(c)
	})

	g.Post("/accept-invite", loginLimit, func(c *fiber.Ctx) error {
		var b acceptInviteReq
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		if err := s.AcceptInvite(c.UserContext(), b.Token, b.FullName, b.Password); err != nil {
			return err
		}
		return httpx.NoContent(c)
	})

	g.Post("/verify-email", loginLimit, func(c *fiber.Ctx) error {
		var b tokenReq
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		if err := s.VerifyEmail(c.UserContext(), b.Token); err != nil {
			return err
		}
		return httpx.NoContent(c)
	})

	g.Get("/me", s.Authenticate, func(c *fiber.Ctx) error {
		p := app.CurrentUser(c)
		u, err := s.Q.GetUser(c.UserContext(), p.ID)
		if err != nil || !u.User.IsActive || u.User.PasswordHash == nil {
			return ErrSessionUser
		}
		grants, err := rb.Grants(c.UserContext(), u.User.RoleID)
		if err != nil {
			return err
		}
		var avatar any
		if u.User.AvatarKey != nil && s.Files.Enabled() {
			if v := s.Files.URL(*u.User.AvatarKey); v != "" {
				avatar = v
			}
		}
		return httpx.OK(c, fiber.Map{
			"id": u.User.ID, "email": u.User.Email, "fullName": u.User.FullName,
			"avatarUrl": avatar, "roleId": u.User.RoleID, "roleName": u.RoleName,
			"emailVerified": u.User.EmailVerifiedAt != nil, "permissions": grants,
		})
	})
}
