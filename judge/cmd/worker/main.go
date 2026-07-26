package main

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"

	"github.com/caze/ascend/judge/internal/worker"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}

	client := redis.NewClient(&redis.Options{Addr: addr})

	if err := client.Ping(context.Background()).Err(); err != nil {
		slog.Error("failed to connect to redis", "addr", addr, "err", err)
		os.Exit(1)
	}
	slog.Info("connected to redis", "addr", addr)

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

	dockerExecutor := worker.NewDockerExecutor("docker")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	w := worker.New(client, db, dockerExecutor, slog.Default())
	w.Run(ctx)
	slog.Info("worker stopped")
}
