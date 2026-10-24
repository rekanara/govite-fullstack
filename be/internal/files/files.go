// Package files serves avatar and private attachment uploads for the caller.
package files

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"govite/internal/app"
	"govite/internal/audit"
	"govite/internal/httpx"
	"govite/internal/storage"
	"govite/internal/store"
)

var (
	ErrInvalidAvatar     = httpx.E(400, "INVALID_AVATAR", "Avatar must be an image up to 2MB")
	ErrInvalidAttachment = httpx.E(400, "INVALID_ATTACHMENT", "Attachment is missing or too large")
	ErrAttachmentGone    = httpx.E(404, "ATTACHMENT_NOT_FOUND", "Attachment not found")
)

const (
	maxAvatar     = 2_000_000
	maxAttachment = 10_000_000
	signedTTL     = time.Hour
)

type Service struct {
	*app.Deps
	Audit *audit.Log
}

type attachment struct {
	ID          string    `json:"id"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"contentType"`
	Size        int32     `json:"size"`
	URL         string    `json:"url"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (s *Service) view(c *fiber.Ctx, a store.Attachment) (attachment, error) {
	url, err := s.Files.SignedURL(c.UserContext(), a.Key, a.Filename, signedTTL)
	return attachment{a.ID, a.Filename, a.ContentType, a.Size, url, a.CreatedAt}, err
}

// Routes mounts avatar and attachments behind Authenticate only.
func (s *Service) Routes(r fiber.Router) {
	r.Post("/account/avatar", func(c *fiber.Ctx) error {
		ctx, id := c.UserContext(), app.CurrentUser(c).ID
		if !s.Files.Enabled() {
			return storage.ErrUnavailable
		}
		u, err := s.Q.GetUserByID(ctx, id)
		if err != nil {
			return err
		}
		key, url, err := s.Files.UploadImage(c, "file", "avatars", id, maxAvatar)
		switch err {
		case nil:
		case storage.ErrImageRequired, storage.ErrImageTooLarge, storage.ErrInvalidImage:
			return ErrInvalidAvatar
		default:
			return err
		}
		if err := s.Q.SetAvatarKey(ctx, store.SetAvatarKeyParams{ID: id, AvatarKey: &key}); err != nil {
			return err
		}
		if u.AvatarKey != nil && *u.AvatarKey != key { // new file stored first, so a failed upload never loses the old one
			_ = s.Files.Remove(ctx, *u.AvatarKey)
		}
		s.Audit.From(c, "account.avatar.update", "user", id, nil, fiber.Map{"avatarUrl": url})
		return httpx.Created(c, fiber.Map{"avatarUrl": url})
	})

	r.Get("/attachments", func(c *fiber.Ctx) error {
		if !s.Files.Enabled() {
			return httpx.OK(c, []attachment{})
		}
		rows, err := s.Q.ListAttachments(c.UserContext(), app.CurrentUser(c).ID)
		if err != nil {
			return err
		}
		out := make([]attachment, len(rows))
		for i, a := range rows {
			if out[i], err = s.view(c, a); err != nil {
				return err
			}
		}
		return httpx.OK(c, out)
	})

	r.Post("/attachments", func(c *fiber.Ctx) error {
		if !s.Files.Enabled() {
			return storage.ErrUnavailable
		}
		body, name, err := storage.ReadFile(c, "file", maxAttachment)
		if err == fiber.ErrRequestEntityTooLarge || len(body) == 0 {
			return ErrInvalidAttachment
		}
		if err != nil {
			return err
		}
		ctype := c.FormValue("contentType")
		if fh, ferr := c.FormFile("file"); ferr == nil {
			ctype = fh.Header.Get("Content-Type")
		}
		if ctype == "" {
			ctype = "application/octet-stream"
		}
		ctx, owner := c.UserContext(), app.CurrentUser(c).ID
		key := storage.Key("misc", owner, uuid.Must(uuid.NewV7()).String(), extFor(ctype), time.Now())
		if err := s.Files.Put(ctx, key, body, ctype); err != nil {
			return err
		}
		name = truncate(name, 255)
		id := app.NewID()
		if err := s.Q.CreateAttachment(ctx, store.CreateAttachmentParams{ID: id, OwnerUserID: owner, Key: key, Filename: name, ContentType: ctype, Size: int32(len(body))}); err != nil {
			_ = s.Files.Remove(ctx, key)
			return err
		}
		a, err := s.Q.GetAttachment(ctx, id)
		if err != nil {
			return err
		}
		v, err := s.view(c, a)
		if err != nil {
			return err
		}
		return httpx.Created(c, v)
	})

	r.Delete("/attachments/:id", func(c *fiber.Ctx) error {
		id := c.Params("id")
		if _, err := uuid.Parse(id); err != nil {
			return httpx.Validation("id: must be a UUID")
		}
		ctx := c.UserContext()
		a, err := s.Q.GetAttachment(ctx, id)
		if err != nil || a.OwnerUserID != app.CurrentUser(c).ID {
			return ErrAttachmentGone
		}
		if err := s.Q.DeleteAttachment(ctx, id); err != nil {
			return err
		}
		if s.Files.Enabled() {
			_ = s.Files.Remove(ctx, a.Key)
		}
		return httpx.NoContent(c)
	})
}

var extensions = map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp", "application/pdf": "pdf", "text/csv": "csv"}

func extFor(contentType string) string {
	if e, ok := extensions[strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))]; ok {
		return e
	}
	return "bin"
}

func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}
