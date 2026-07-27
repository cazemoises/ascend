package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"

	appmw "github.com/caze/ascend/api/internal/middleware"
	"github.com/caze/ascend/api/internal/router"
	"github.com/caze/ascend/api/internal/store"
)

// runMigrations applies all pending schema migrations before the HTTP router
// starts. MIGRATIONS_PATH points at the SQL files (defaults to ./migrations
// for local `go run` from the repo root; the container image sets it to
// /migrations). golang-migrate takes a Postgres advisory lock, so concurrent
// api replicas won't race each other.
func runMigrations(dbURL string) error {
	path := os.Getenv("MIGRATIONS_PATH")
	if path == "" {
		path = "migrations"
	}
	m, err := migrate.New("file://"+path, dbURL)
	if err != nil {
		return fmt.Errorf("init migrator (path %q): %w", path, err)
	}
	defer m.Close()

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			slog.Info("database schema up to date")
			return nil
		}
		return fmt.Errorf("apply migrations: %w", err)
	}
	slog.Info("database migrations applied")
	return nil
}

// envInt reads an integer env var, falling back when unset or invalid.
func envInt(name string, fallback int) int {
	if v := os.Getenv(name); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
		slog.Warn("invalid env value, using default", "name", name, "default", fallback)
	}
	return fallback
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		slog.Error("DATABASE_URL is not set")
		os.Exit(1)
	}
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		slog.Error("failed to open database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		slog.Error("failed to connect to database", "err", err)
		os.Exit(1)
	}
	slog.Info("connected to database")

	if err := runMigrations(dbURL); err != nil {
		slog.Error("failed to run migrations", "err", err)
		os.Exit(1)
	}

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		slog.Error("REDIS_URL is not set")
		os.Exit(1)
	}
	redisOpts, err := redis.ParseURL(redisURL)
	if err != nil {
		slog.Error("invalid REDIS_URL", "err", err)
		os.Exit(1)
	}
	rdb := redis.NewClient(redisOpts)
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		slog.Error("failed to connect to Redis", "err", err)
		os.Exit(1)
	}
	defer rdb.Close()
	slog.Info("connected to Redis")

	s := store.New(db, rdb)

	devFakeEmail := os.Getenv("DEV_FAKE_EMAIL")
	if devFakeEmail != "" {
		slog.Warn("DEV_FAKE_EMAIL ativo — NÃO usar em produção", "email", devFakeEmail)
	}
	pa := appmw.NewPangolinAuth(s, devFakeEmail)

	rl := appmw.NewRateLimiter(rdb,
		envInt("SUBMIT_RATE_LIMIT", 10),
		time.Duration(envInt("SUBMIT_RATE_WINDOW_S", 60))*time.Second,
		appmw.UserKey)

	addr := os.Getenv("API_ADDR")
	if addr == "" {
		addr = "0.0.0.0:8080"
	}

	r := router.New(s, pa, rl)
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	go func() {
		slog.Info("server starting", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	slog.Info("server shutting down")
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("shutdown error", "err", err)
	}
	slog.Info("server stopped")
}
