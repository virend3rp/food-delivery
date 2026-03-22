package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/virend3rp/food-delivery/internal/config"
	"github.com/virend3rp/food-delivery/internal/db"
	"github.com/virend3rp/food-delivery/internal/logger"
	"github.com/virend3rp/food-delivery/internal/rabbitmq"
)

func main() {
	logger.Init("notification-service")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	rabbitURL, err := config.Required("RABBITMQ_URL")
	if err != nil {
		slog.Error("missing config", "err", err)
		os.Exit(1)
	}
	dbURL, err := config.Required("DATABASE_URL")
	if err != nil {
		slog.Error("missing config", "err", err)
		os.Exit(1)
	}

	conn, err := rabbitmq.NewConnection(rabbitURL)
	if err != nil {
		slog.Error("rabbitmq connection failed", "err", err)
		os.Exit(1)
	}
	defer conn.Close()

	consumer, err := rabbitmq.NewConsumer(conn)
	if err != nil {
		slog.Error("consumer init failed", "err", err)
		os.Exit(1)
	}
	defer consumer.Close()

	pool, err := db.New(ctx, dbURL)
	if err != nil {
		slog.Error("db connection failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	store := NewPostgresNotificationStore(pool)
	if err := store.Migrate(ctx); err != nil {
		slog.Error("migration failed", "err", err)
		os.Exit(1)
	}

	svc := NewService(store)

	if err := consumer.DeclareQueue("notification.queue", "order.*", "driver.*"); err != nil {
		slog.Error("declare queue failed", "err", err)
		os.Exit(1)
	}
	if err := consumer.Consume(ctx, "notification.queue", svc.HandleEvent); err != nil {
		slog.Error("consume failed", "err", err)
		os.Exit(1)
	}

	slog.Info("listening for all events")
	<-ctx.Done()
	slog.Info("shutting down")
}
