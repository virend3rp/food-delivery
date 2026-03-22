package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/virend3rp/food-delivery/internal/db"
	"github.com/virend3rp/food-delivery/internal/events"
	"github.com/virend3rp/food-delivery/internal/rabbitmq"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	rabbitURL := getenv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")
	dbURL := getenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/delivery_db")

	conn, err := rabbitmq.NewConnection(rabbitURL)
	if err != nil {
		log.Fatalf("[delivery-service] rabbitmq: %v", err)
	}
	defer conn.Close()

	consumer, err := rabbitmq.NewConsumer(conn)
	if err != nil {
		log.Fatalf("[delivery-service] consumer: %v", err)
	}
	defer consumer.Close()

	pub, err := rabbitmq.NewPublisher(conn)
	if err != nil {
		log.Fatalf("[delivery-service] publisher: %v", err)
	}
	defer pub.Close()

	pool, err := db.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("[delivery-service] db: %v", err)
	}
	defer pool.Close()

	store := NewPostgresDeliveryStore(pool)
	if err := store.Migrate(ctx); err != nil {
		log.Fatalf("[delivery-service] migrate: %v", err)
	}

	svc := NewService(pub, store)

	if err := consumer.DeclareQueue("delivery.queue", string(events.OrderAccepted)); err != nil {
		log.Fatalf("[delivery-service] declare queue: %v", err)
	}
	if err := consumer.Consume(ctx, "delivery.queue", svc.HandleOrderAccepted); err != nil {
		log.Fatalf("[delivery-service] consume: %v", err)
	}

	log.Println("[delivery-service] waiting for accepted orders...")
	<-ctx.Done()
	log.Println("[delivery-service] shutting down")
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
