package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/virend3rp/food-delivery/internal/db"
	"github.com/virend3rp/food-delivery/internal/rabbitmq"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	rabbitURL := getenv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")
	dbURL := getenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/notification_db")

	conn, err := rabbitmq.NewConnection(rabbitURL)
	if err != nil {
		log.Fatalf("[notification-service] rabbitmq: %v", err)
	}
	defer conn.Close()

	consumer, err := rabbitmq.NewConsumer(conn)
	if err != nil {
		log.Fatalf("[notification-service] consumer: %v", err)
	}
	defer consumer.Close()

	pool, err := db.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("[notification-service] db: %v", err)
	}
	defer pool.Close()

	store := NewPostgresNotificationStore(pool)
	if err := store.Migrate(ctx); err != nil {
		log.Fatalf("[notification-service] migrate: %v", err)
	}

	svc := NewService(store)

	if err := consumer.DeclareQueue("notification.queue", "order.*", "driver.*"); err != nil {
		log.Fatalf("[notification-service] declare queue: %v", err)
	}
	if err := consumer.Consume(ctx, "notification.queue", svc.HandleEvent); err != nil {
		log.Fatalf("[notification-service] consume: %v", err)
	}

	log.Println("[notification-service] listening for all events...")
	<-ctx.Done()
	log.Println("[notification-service] shutting down")
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
