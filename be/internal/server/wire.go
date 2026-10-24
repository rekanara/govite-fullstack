package server

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	accountapp "govite/internal/account/application"
	accountinfra "govite/internal/account/infrastructure"
	accounthttp "govite/internal/account/interface/http"
	annapp "govite/internal/announcements/application"
	anninfra "govite/internal/announcements/infrastructure"
	annhttp "govite/internal/announcements/interface/http"
	auditapp "govite/internal/audit/application"
	auditinfra "govite/internal/audit/infrastructure"
	audithttp "govite/internal/audit/interface/http"
	authapp "govite/internal/auth/application"
	authinfra "govite/internal/auth/infrastructure"
	authhttp "govite/internal/auth/interface/http"
	filesapp "govite/internal/files/application"
	filesinfra "govite/internal/files/infrastructure"
	fileshttp "govite/internal/files/interface/http"
	intapp "govite/internal/integrations/application"
	intinfra "govite/internal/integrations/infrastructure"
	inthttp "govite/internal/integrations/interface/http"
	notifyapp "govite/internal/notify/application"
	notifydomain "govite/internal/notify/domain"
	notifyinfra "govite/internal/notify/infrastructure"
	notifyhttp "govite/internal/notify/interface/http"
	"govite/internal/platform/config"
	"govite/internal/platform/crypto"
	"govite/internal/platform/fcm"
	"govite/internal/platform/mail"
	"govite/internal/platform/postgres"
	"govite/internal/platform/storage"
	"govite/internal/rbac"
	rbacdomain "govite/internal/rbac/domain"
	rolesapp "govite/internal/roles/application"
	rolesinfra "govite/internal/roles/infrastructure"
	roleshttp "govite/internal/roles/interface/http"
	settingsapp "govite/internal/settings/application"
	settingsinfra "govite/internal/settings/infrastructure"
	settingshttp "govite/internal/settings/interface/http"
	sysapp "govite/internal/system/application"
	sysinfra "govite/internal/system/infrastructure"
	syshttp "govite/internal/system/interface/http"
	usersapp "govite/internal/users/application"
	usersinfra "govite/internal/users/infrastructure"
	usershttp "govite/internal/users/interface/http"
)

// Deps are the process-level adapters main builds once.
type Deps struct {
	Cfg   config.Config
	Pool  *pgxpool.Pool
	Redis *redis.Client // nil when REDIS_URL is unset
	Mail  mail.Sender
	Files *storage.Store // nil when object storage is not configured
	Push  fcm.Pusher     // nil when Firebase is not configured
}

// Settings reads the app settings without a full server, e.g. for the mail brand.
func Settings(pool *pgxpool.Pool) *settingsapp.Service {
	return &settingsapp.Service{Repo: settingsinfra.Repository{DB: postgres.New(pool)}}
}

// Brand feeds the mailer from app settings; defaults on any error.
func Brand(s *settingsapp.Service) func(context.Context) mail.Brand {
	str := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	return func(ctx context.Context) mail.Brand {
		v, _ := s.Get(ctx)
		return mail.Brand{AppName: v.AppName, Tagline: str(v.Tagline), LogoURL: str(v.LogoURL), SupportEmail: str(v.SupportEmail), SupportPhone: str(v.SupportPhone), EmailFooter: str(v.EmailFooter), Address: v.FullAddress()}
	}
}

// handlers is every feature wired to its adapters.
type handlers struct {
	rbac          *rbac.RBAC
	auth          authhttp.Handler
	account       accounthttp.Handler
	files         fileshttp.Handler
	notify        notifyhttp.Handler
	users         usershttp.Handler
	roles         roleshttp.Handler
	audit         audithttp.Handler
	settings      settingshttp.Handler
	announcements annhttp.Handler
	integrations  inthttp.Handler
	system        syshttp.Handler
}

func wire(d Deps) (handlers, *sysapp.Service) {
	db := postgres.New(d.Pool)
	keys := crypto.NewKeys(d.Cfg.JWTSecret, d.Cfg.RefreshPepper)
	files := filesinfra.Storage{Store: d.Files}

	auditor := &auditapp.Service{Repo: auditinfra.Repository{DB: db}}
	rb := rbac.New(db.Q(context.Background()), d.Redis)

	settings := &settingsapp.Service{Repo: settingsinfra.Repository{DB: db}, Images: d.Files, Audit: auditor}
	setting := func(ctx context.Context) (int, bool) {
		v, _ := settings.Get(ctx)
		return v.MaintenanceRetentionDays, v.PushNotificationsEnabled
	}

	auth := &authapp.Service{
		Repo: authinfra.Repository{DB: db}, Crypto: authinfra.Crypto{Keys: keys}, OTP: authinfra.TOTP{},
		Mail: authinfra.Mailer{Sender: d.Mail}, Tx: db, Audit: auditor, WebURL: d.Cfg.WebURL,
	}

	var pusher notifydomain.Pusher
	if d.Push != nil {
		pusher = notifyinfra.Pusher{Pusher: d.Push}
	}
	notify := &notifyapp.Service{Repo: notifyinfra.Repository{DB: db}, Tx: db, Push: pusher,
		PushAllowed: func(ctx context.Context) bool { _, on := setting(ctx); return on }}

	integrations := &intapp.Service{Repo: intinfra.Repository{DB: db}, Audit: auditor, Token: crypto.RandomToken, MidtransServerKey: d.Cfg.MidtransServerKey}

	var redisPing func(context.Context) error
	if d.Redis != nil {
		redisPing = func(ctx context.Context) error { return d.Redis.Ping(ctx).Err() }
	}
	system := &sysapp.Service{DB: d.Pool.Ping, Redis: redisPing, Cleaner: sysinfra.Cleaner{Pool: d.Pool},
		RetentionDays: func(ctx context.Context) int { days, _ := setting(ctx); return days }}

	return handlers{
		rbac: rb,
		auth: authhttp.Handler{
			Svc: auth,
			Grants: func(c *fiber.Ctx, roleID string) ([]rbacdomain.Grant, error) {
				return rb.Grants(c.UserContext(), roleID)
			},
			AvatarURL: d.Files.URL,
		},
		account: accounthttp.Handler{Svc: &accountapp.Service{Repo: accountinfra.Repository{DB: db}, Audit: auditor}},
		files:   fileshttp.Handler{Svc: &filesapp.Service{Repo: filesinfra.Repository{DB: db}, Store: files, Audit: auditor}},
		notify:  notifyhttp.Handler{Svc: notify},
		users: usershttp.Handler{
			Svc:   &usersapp.Service{Repo: usersinfra.Repository{DB: db, AvatarURL: d.Files.URL}, Creds: auth, Tx: db, Audit: auditor},
			Audit: auditor,
		},
		roles:    roleshttp.Handler{Svc: &rolesapp.Service{Repo: rolesinfra.Repository{DB: db}, Cache: rb, Tx: db, Audit: auditor}},
		audit:    audithttp.Handler{Svc: auditor},
		settings: settingshttp.Handler{Svc: settings},
		announcements: annhttp.Handler{Svc: &annapp.Service{
			Repo: anninfra.Repository{DB: db}, Images: d.Files, Push: anninfra.Broadcaster{Notify: notify}, Audit: auditor,
		}},
		integrations: inthttp.Handler{Svc: integrations},
		system: syshttp.Handler{Svc: system, Pool: d.Pool, Env: d.Cfg.Env, Integrations: syshttp.Integrations{
			Storage: d.Files.Enabled(), Push: d.Push != nil, Payment: integrations.PaymentEnabled(), Mailer: d.Cfg.SMTPHost != "",
		}},
	}, system
}
