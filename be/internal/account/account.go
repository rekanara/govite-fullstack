// Package account is self-service: any signed-in user, always on their own row.
package account

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"govite/internal/app"
	"govite/internal/audit"
	"govite/internal/auth"
	"govite/internal/httpx"
	"govite/internal/store"
)

type Service struct {
	*app.Deps
	Auth  *auth.Service
	Audit *audit.Log
}

type passwordReq struct {
	CurrentPassword string `json:"currentPassword" validate:"required"`
}

// Routes mounts /account behind Authenticate only (no permission needed).
func (s *Service) Routes(r fiber.Router) {
	g := r.Group("/account")

	g.Get("/notifications/preferences", func(c *fiber.Ctx) error {
		u, err := s.Q.GetUserByID(c.UserContext(), app.CurrentUser(c).ID)
		if err != nil {
			return err
		}
		return httpx.OK(c, fiber.Map{"pushEnabled": u.NotificationPushEnabled})
	})

	g.Patch("/notifications/preferences", func(c *fiber.Ctx) error {
		var b struct {
			PushEnabled *bool `json:"pushEnabled" validate:"required"`
		}
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		id := app.CurrentUser(c).ID
		if err := s.Q.SetPushEnabled(c.UserContext(), store.SetPushEnabledParams{ID: id, NotificationPushEnabled: *b.PushEnabled}); err != nil {
			return err
		}
		res := fiber.Map{"pushEnabled": *b.PushEnabled}
		s.Audit.From(c, "account.notifications.update", "user", id, nil, res)
		return httpx.OK(c, res)
	})

	g.Patch("/profile", func(c *fiber.Ctx) error {
		var b struct {
			FullName string `json:"fullName" validate:"required,min=1,max=100"`
		}
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		id, name := app.CurrentUser(c).ID, strings.TrimSpace(b.FullName)
		if err := s.Q.UpdateProfile(c.UserContext(), store.UpdateProfileParams{ID: id, FullName: &name}); err != nil {
			return err
		}
		res := fiber.Map{"id": id, "fullName": name}
		s.Audit.From(c, "account.profile.update", "user", id, nil, res)
		return httpx.OK(c, res)
	})

	g.Post("/password", func(c *fiber.Ctx) error {
		var b struct {
			CurrentPassword string `json:"currentPassword" validate:"required"`
			NewPassword     string `json:"newPassword" validate:"required,password"`
		}
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		ctx, id := c.UserContext(), app.CurrentUser(c).ID
		if _, err := s.Auth.CheckPassword(ctx, id, b.CurrentPassword); err != nil {
			return err
		}
		hash, err := auth.HashPassword(b.NewPassword)
		if err != nil {
			return err
		}
		err = s.Tx(ctx, func(q *store.Queries) error {
			if err := q.SetPassword(ctx, store.SetPasswordParams{ID: id, PasswordHash: &hash}); err != nil {
				return err
			}
			return q.RevokeUserFamilies(ctx, id)
		})
		if err != nil {
			return err
		}
		s.Audit.From(c, "account.password.change", "user", id, nil, nil)
		return httpx.NoContent(c)
	})

	g.Get("/two-factor", func(c *fiber.Ctx) error {
		ctx, id := c.UserContext(), app.CurrentUser(c).ID
		u, err := s.Q.GetUserByID(ctx, id)
		if err != nil {
			return err
		}
		role, err := s.Q.GetRole(ctx, u.RoleID)
		if err != nil {
			return err
		}
		res := fiber.Map{"enabled": false, "required": role.Role.RequireTwoFactor, "enabledAt": nil, "backupCodesRemaining": 0}
		if tf, err := s.Q.GetTwoFactor(ctx, id); err == nil && tf.EnabledAt != nil {
			res["enabled"], res["enabledAt"] = true, tf.EnabledAt
		}
		if n, err := s.Q.CountBackupCodes(ctx, id); err == nil {
			res["backupCodesRemaining"] = n
		}
		return httpx.OK(c, res)
	})

	g.Post("/two-factor/setup", func(c *fiber.Ctx) error {
		ctx, id := c.UserContext(), app.CurrentUser(c).ID
		u, err := s.Q.GetUserByID(ctx, id)
		if err != nil {
			return err
		}
		key, url, err := s.Auth.BeginSetup(ctx, id, u.Email)
		if err != nil {
			return err
		}
		return httpx.OK(c, fiber.Map{"manualKey": key, "otpauthUrl": url})
	})

	g.Post("/two-factor/enable", func(c *fiber.Ctx) error {
		var b struct {
			Code string `json:"code" validate:"required,numeric,len=6"`
		}
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		id := app.CurrentUser(c).ID
		codes, err := s.Auth.ConfirmSetup(c.UserContext(), id, b.Code)
		if err != nil {
			return err
		}
		s.Audit.From(c, "account.two_factor.enable", "user", id, nil, nil)
		return httpx.OK(c, fiber.Map{"backupCodes": codes})
	})

	g.Post("/two-factor/backup-codes", func(c *fiber.Ctx) error {
		var b passwordReq
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		ctx, id := c.UserContext(), app.CurrentUser(c).ID
		if _, err := s.Auth.CheckPassword(ctx, id, b.CurrentPassword); err != nil {
			return err
		}
		if tf, err := s.Q.GetTwoFactor(ctx, id); err != nil || tf.EnabledAt == nil {
			return auth.ErrTOTPNotEnabled
		}
		codes, err := s.Auth.RegenerateBackupCodes(ctx, id)
		if err != nil {
			return err
		}
		s.Audit.From(c, "account.two_factor.backup_codes", "user", id, nil, nil)
		return httpx.OK(c, fiber.Map{"backupCodes": codes})
	})

	g.Post("/two-factor/reset", func(c *fiber.Ctx) error {
		return s.resetTwoFactor(c, false)
	})
	g.Post("/two-factor/disable", func(c *fiber.Ctx) error {
		return s.resetTwoFactor(c, true)
	})

	g.Post("/verify-email", func(c *fiber.Ctx) error {
		ctx := c.UserContext()
		u, err := s.Q.GetUserByID(ctx, app.CurrentUser(c).ID)
		if err != nil {
			return err
		}
		if u.EmailVerifiedAt == nil {
			if err := s.Auth.SendVerification(ctx, u.ID, u.Email); err != nil {
				return err
			}
		}
		return httpx.NoContent(c)
	})

	g.Get("/sessions", func(c *fiber.Ctx) error {
		p := app.CurrentUser(c)
		rows, err := s.Q.ListSessions(c.UserContext(), p.ID)
		if err != nil {
			return err
		}
		out := make([]fiber.Map, len(rows))
		for i, r := range rows {
			out[i] = fiber.Map{"id": r.ID, "userAgent": r.UserAgent, "ipAddress": r.IpAddress, "createdAt": r.CreatedAt, "lastUsedAt": r.LastUsedAt, "current": r.ID == p.SessionID}
		}
		return httpx.OK(c, out)
	})

	g.Delete("/sessions", func(c *fiber.Ctx) error {
		p := app.CurrentUser(c)
		if err := s.Q.RevokeOtherFamilies(c.UserContext(), store.RevokeOtherFamiliesParams{UserID: p.ID, ID: p.SessionID}); err != nil {
			return err
		}
		s.Audit.From(c, "account.sessions.revoke_others", "session", p.SessionID, nil, nil)
		return httpx.NoContent(c)
	})

	g.Delete("/sessions/:id", func(c *fiber.Ctx) error {
		id := c.Params("id")
		if _, err := uuid.Parse(id); err != nil {
			return httpx.Validation("id: must be a UUID")
		}
		ctx, p := c.UserContext(), app.CurrentUser(c)
		f, err := s.Q.GetFamily(ctx, id)
		if err != nil || f.UserID != p.ID || f.RevokedAt != nil {
			return auth.ErrSessionNotFound
		}
		if err := s.Q.RevokeFamily(ctx, id); err != nil {
			return err
		}
		s.Audit.From(c, "account.session.revoke", "session", id, nil, nil)
		return httpx.NoContent(c)
	})

}

// resetTwoFactor wipes the secret and codes and signs out other sessions.
// disable additionally refuses when the role requires 2FA.
func (s *Service) resetTwoFactor(c *fiber.Ctx, disable bool) error {
	var b passwordReq
	if err := httpx.Body(c, &b); err != nil {
		return err
	}
	ctx, p := c.UserContext(), app.CurrentUser(c)
	u, err := s.Auth.CheckPassword(ctx, p.ID, b.CurrentPassword)
	if disable {
		if u.RoleID != "" {
			if role, rerr := s.Q.GetRole(ctx, u.RoleID); rerr == nil && role.Role.RequireTwoFactor {
				return auth.ErrTOTPRequired
			}
		}
	}
	if err != nil {
		return err
	}
	err = s.Tx(ctx, func(q *store.Queries) error {
		if err := q.DeleteTwoFactor(ctx, p.ID); err != nil {
			return err
		}
		if err := q.DeleteBackupCodes(ctx, p.ID); err != nil {
			return err
		}
		return q.RevokeOtherFamilies(ctx, store.RevokeOtherFamiliesParams{UserID: p.ID, ID: p.SessionID})
	})
	if err != nil {
		return err
	}
	action := "account.two_factor.reset"
	if disable {
		action = "account.two_factor.disable"
	}
	s.Audit.From(c, action, "user", p.ID, nil, nil)
	return httpx.NoContent(c)
}
