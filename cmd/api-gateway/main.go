package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/virend3rp/food-delivery/internal/db"
	"github.com/virend3rp/food-delivery/internal/events"
	"github.com/virend3rp/food-delivery/internal/rabbitmq"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	rabbitURL := getenv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")
	dbURL := getenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/orders_db")

	conn, err := rabbitmq.NewConnection(rabbitURL)
	if err != nil {
		log.Fatalf("[api-gateway] rabbitmq: %v", err)
	}
	defer conn.Close()

	pub, err := rabbitmq.NewPublisher(conn)
	if err != nil {
		log.Fatalf("[api-gateway] publisher: %v", err)
	}
	defer pub.Close()

	pool, err := db.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("[api-gateway] db: %v", err)
	}
	defer pool.Close()

	store := NewPostgresOrderStore(pool)
	if err := store.Migrate(ctx); err != nil {
		log.Fatalf("[api-gateway] migrate: %v", err)
	}

	h := NewHandler(pub, store)

	// Consumer: listen for downstream events and update order status
	consumer, err := rabbitmq.NewConsumer(conn)
	if err != nil {
		log.Fatalf("[api-gateway] consumer: %v", err)
	}
	defer consumer.Close()

	if err := consumer.DeclareQueue("api-gateway.status.queue",
		string(events.OrderAccepted),
		string(events.OrderRejected),
		string(events.DriverAssigned),
		string(events.OrderPickedUp),
		string(events.OrderDelivered),
	); err != nil {
		log.Fatalf("[api-gateway] declare status queue: %v", err)
	}
	if err := consumer.Consume(ctx, "api-gateway.status.queue", h.HandleStatusUpdate); err != nil {
		log.Fatalf("[api-gateway] consume: %v", err)
	}

	r := gin.Default()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.POST("/orders", h.CreateOrder)
	r.GET("/orders/:id", h.GetOrder)

	log.Println("[api-gateway] starting on :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("[api-gateway] server: %v", err)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
