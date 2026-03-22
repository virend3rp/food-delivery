package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/virend3rp/food-delivery/internal/config"
	"github.com/virend3rp/food-delivery/internal/db"
	"github.com/virend3rp/food-delivery/internal/events"
	"github.com/virend3rp/food-delivery/internal/logger"
	"github.com/virend3rp/food-delivery/internal/rabbitmq"
)

func main() {
	logger.Init("restaurant-service")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	rabbitURL := config.WithDefault("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")
	dbURL := config.WithDefault("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/restaurant_db")

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

	pub, err := rabbitmq.NewPublisher(conn)
	if err != nil {
		slog.Error("publisher init failed", "err", err)
		os.Exit(1)
	}
	defer pub.Close()

	pool, err := db.New(ctx, dbURL)
	if err != nil {
		slog.Error("db connection failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	store := NewPostgresRestaurantStore(pool)
	if err := store.Migrate(ctx); err != nil {
		slog.Error("migration failed", "err", err)
		os.Exit(1)
	}

	svc := NewService(pub, store)

	if err := consumer.DeclareQueue("restaurant.queue", string(events.OrderCreated)); err != nil {
		slog.Error("declare queue failed", "err", err)
		os.Exit(1)
	}
	if err := consumer.Consume(ctx, "restaurant.queue", svc.HandleOrderCreated); err != nil {
		slog.Error("consume failed", "err", err)
		os.Exit(1)
	}

	slog.Info("waiting for orders")
	<-ctx.Done()
	slog.Info("shutting down")
}
