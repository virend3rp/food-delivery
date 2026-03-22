package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/virend3rp/food-delivery/internal/events"
)

type eventPublisher interface {
	Publish(ctx context.Context, routingKey string, payload any) error
}

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	pub   eventPublisher
	store orderStore
}

func NewHandler(pub eventPublisher, store orderStore) *Handler {
	return &Handler{pub: pub, store: store}
}

func (h *Handler) CreateOrder(c *gin.Context) {
	var req struct {
		CustomerID   string        `json:"customer_id" binding:"required"`
		RestaurantID string        `json:"restaurant_id" binding:"required"`
		Items        []events.Item `json:"items" binding:"required,min=1"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var total float64
	for _, item := range req.Items {
		total += item.Price * float64(item.Quantity)
	}

	event := events.OrderCreatedEvent{
		BaseEvent: events.BaseEvent{
			ID:        uuid.New().String(),
			Type:      events.OrderCreated,
			Timestamp: time.Now().UTC(),
		},
		OrderID:      uuid.New().String(),
		CustomerID:   req.CustomerID,
		RestaurantID: req.RestaurantID,
		Items:        req.Items,
		TotalPrice:   total,
	}

	if err := h.store.Save(c.Request.Context(), event); err != nil {
		log.Printf("[api-gateway] db save failed: %v", err)
		// non-fatal: continue to publish
	}

	if err := h.pub.Publish(c.Request.Context(), string(events.OrderCreated), event); err != nil {
		log.Printf("[api-gateway] publish failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to queue order"})
		return
	}

	log.Printf("[api-gateway] order created: %s (total: $%.2f)", event.OrderID, total)
	c.JSON(http.StatusAccepted, gin.H{
		"order_id": event.OrderID,
		"status":   "pending",
		"message":  "your order is being processed",
	})
}

func (h *Handler) GetOrder(c *gin.Context) {
	order, err := h.store.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}
	c.JSON(http.StatusOK, order)
}
