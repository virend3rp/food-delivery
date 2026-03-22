package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/virend3rp/food-delivery/internal/config"
	"github.com/virend3rp/food-delivery/internal/db"
	"github.com/virend3rp/food-delivery/internal/events"
	"github.com/virend3rp/food-delivery/internal/logger"
	"github.com/virend3rp/food-delivery/internal/rabbitmq"
)

func main() {
	logger.Init("api-gateway")

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

	store := NewPostgresOrderStore(pool)
	if err := store.Migrate(ctx); err != nil {
		slog.Error("migration failed", "err", err)
		os.Exit(1)
	}

	h := NewHandler(pub, store)

	consumer, err := rabbitmq.NewConsumer(conn)
	if err != nil {
		slog.Error("consumer init failed", "err", err)
		os.Exit(1)
	}
	defer consumer.Close()

	if err := consumer.DeclareQueue("api-gateway.status.queue",
		string(events.OrderAccepted),
		string(events.OrderRejected),
		string(events.DriverAssigned),
		string(events.OrderPickedUp),
		string(events.OrderDelivered),
	); err != nil {
		slog.Error("declare status queue failed", "err", err)
		os.Exit(1)
	}
	if err := consumer.Consume(ctx, "api-gateway.status.queue", h.HandleStatusUpdate); err != nil {
		slog.Error("consume failed", "err", err)
		os.Exit(1)
	}

	r := gin.Default()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.POST("/orders", h.CreateOrder)
	r.GET("/orders", h.ListOrders)
	r.GET("/orders/:id", h.GetOrder)
	r.GET("/orders/:id/timeline", h.GetTimeline)

	slog.Info("starting", "addr", ":8080")
	if err := r.Run(":8080"); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
