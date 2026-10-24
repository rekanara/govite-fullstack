// Command api runs the HTTP server and its maintenance subcommands:
//
//	api                 serve (default)
//	api migrate [cmd]   goose command, default "up"
//	api seed            sync permissions, Super Admin role and user
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"govite/internal/app"
	"govite/internal/auth"
	"govite/internal/config"
	"govite/internal/db"
	"govite/internal/mail"
	"govite/internal/notify"
	"govite/internal/policy"
	"govite/internal/server"
	"govite/internal/settings"
	"govite/internal/storage"
	"govite/internal/store"
	"govite/internal/system"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	setupLogger(cfg.LogLevel)
	if cfg.SentryDSN != "" {
		if err := sentry.Init(sentry.ClientOptions{Dsn: cfg.SentryDSN, Environment: cfg.SentryEnvironment, EnableTracing: true}); err != nil {
			return err
		}
		defer sentry.Flush(2 * time.Second)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	switch cmd {
	case "migrate":
		goose := "up"
		if len(os.Args) > 2 {
			goose = os.Args[2]
		}
		return db.Migrate(ctx, pool, goose, os.Args[min(3, len(os.Args)):]...)
	case "seed":
		return seed(ctx, cfg, store.New(pool))
	case "serve":
		return serve(ctx, cfg, pool)
	default:
		return fmt.Errorf("unknown command %q (serve|migrate|seed)", cmd)
	}
}

func serve(ctx context.Context, cfg config.Config, pool *pgxpool.Pool) error {
	d := &app.Deps{Cfg: cfg, Pool: pool, Q: store.New(pool)}
	if cfg.RedisURL != "" {
		opt, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			return fmt.Errorf("REDIS_URL: %w", err)
		}
		d.Redis = redis.NewClient(opt)
		defer d.Redis.Close()
		if err := d.Redis.Ping(ctx).Err(); err != nil {
			slog.Warn("redis unreachable at boot; cache and limiter will retry", "err", err)
		}
	}
	brand := (&settings.Service{Deps: d}).Brand
	d.Mail = mail.NewSMTP(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPFrom, brand)
	var err error
	d.Files, err = storage.New(storage.Config{Endpoint: cfg.MinioEndpoint, AccessKey: cfg.MinioAccessKey, SecretKey: cfg.MinioSecretKey, Bucket: cfg.MinioBucket, UseSSL: cfg.MinioUseSSL, PublicEndpoint: cfg.MinioPublicEndpoint, PublicURL: cfg.StoragePublicURL})
	if err != nil {
		return err
	}
	if err := d.Files.EnsureBucket(ctx); err != nil {
		return err
	}
	push, err := notify.NewFCM(ctx, cfg.FirebaseServiceAccountPath)
	if err != nil {
		slog.Error("firebase init failed; push disabled", "err", err)
		push = nil
	}

	f := server.New(d, push)
	sys := &system.Service{Deps: d}
	sys.RetentionDays = func(ctx context.Context) int {
		v, _ := (&settings.Service{Deps: d}).Get(ctx)
		return v.MaintenanceRetentionDays
	}
	go sys.RunDaily(ctx)
	errc := make(chan error, 1)
	go func() { errc <- f.Listen(fmt.Sprintf(":%d", cfg.Port)) }()
	slog.Info("api listening", "port", cfg.Port, "env", cfg.Env)
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		slog.Info("shutting down")
		return f.ShutdownWithTimeout(10 * time.Second)
	}
}

// SuperAdminRoleID is fixed so seed is idempotent across environments.
const SuperAdminRoleID = "019a0000-0000-7000-8000-000000000001"

func seed(ctx context.Context, cfg config.Config, q *store.Queries) error {
	keys := make([]string, len(policy.Permissions))
	for i, p := range policy.Permissions {
		keys[i] = p.Key()
		if err := q.SyncPermission(ctx, store.SyncPermissionParams{ID: app.NewID(), Resource: p.Resource, Action: p.Action}); err != nil {
			return err
		}
	}
	if err := q.DeleteUnknownPermissions(ctx, keys); err != nil {
		return err
	}
	desc := "Full system access"
	if err := q.UpsertSystemRole(ctx, store.UpsertSystemRoleParams{ID: SuperAdminRoleID, Name: "Super Admin", Description: &desc}); err != nil {
		return err
	}
	for _, p := range policy.Permissions {
		if err := q.UpsertRolePermission(ctx, store.UpsertRolePermissionParams{ID: app.NewID(), RoleID: SuperAdminRoleID, Resource: p.Resource, Action: p.Action, Scope: "all"}); err != nil {
			return err
		}
	}
	if cfg.SuperAdminEmail == "" || cfg.SuperAdminPassword == "" {
		return fmt.Errorf("SUPER_ADMIN_EMAIL and SUPER_ADMIN_PASSWORD are required")
	}
	hash, err := auth.HashPassword(cfg.SuperAdminPassword)
	if err != nil {
		return err
	}
	if err := q.UpsertSuperAdmin(ctx, store.UpsertSuperAdminParams{ID: app.NewID(), Email: cfg.SuperAdminEmail, RoleID: SuperAdminRoleID, PasswordHash: &hash}); err != nil {
		return err
	}
	slog.Info("seed done", "permissions", len(keys), "superAdmin", cfg.SuperAdminEmail)
	return nil
}

func setupLogger(level string) {
	var l slog.Level
	switch strings.ToLower(level) {
	case "debug":
		l = slog.LevelDebug
	case "warn":
		l = slog.LevelWarn
	case "error", "silent":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: l})))
}
