package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/virend3rp/food-delivery/internal/db"
	"github.com/virend3rp/food-delivery/internal/rabbitmq"
)

func main() {
	ctx := context.Background()

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
