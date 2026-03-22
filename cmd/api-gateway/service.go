package main

import (
	"context"
	"encoding/json"
	"fmt"
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

// statusMap maps incoming event types to the order status string stored in orders_db.
var statusMap = map[events.EventType]string{
	events.OrderAccepted:  "accepted",
	events.OrderRejected:  "rejected",
	events.DriverAssigned: "driver_assigned",
	events.OrderPickedUp:  "out_for_delivery",
	events.OrderDelivered: "delivered",
}

// HandleStatusUpdate consumes downstream events and updates orders_db so
// GET /orders/:id always returns the current status.
func (h *Handler) HandleStatusUpdate(ctx context.Context, body []byte) error {
	var base events.BaseEvent
	if err := json.Unmarshal(body, &base); err != nil {
		return fmt.Errorf("unmarshal base event: %w", err)
	}

	status, ok := statusMap[base.Type]
	if !ok {
		return nil // ignore events we don't care about
	}

	// Extract the order ID from the specific event type
	var orderID string
	switch base.Type {
	case events.OrderAccepted:
		var e events.OrderAcceptedEvent
		json.Unmarshal(body, &e)
		orderID = e.OrderID
	case events.OrderRejected:
		var e events.OrderRejectedEvent
		json.Unmarshal(body, &e)
		orderID = e.OrderID
	case events.DriverAssigned:
		var e events.DriverAssignedEvent
		json.Unmarshal(body, &e)
		orderID = e.OrderID
	case events.OrderPickedUp:
		var e events.OrderPickedUpEvent
		json.Unmarshal(body, &e)
		orderID = e.OrderID
	case events.OrderDelivered:
		var e events.OrderDeliveredEvent
		json.Unmarshal(body, &e)
		orderID = e.OrderID
	}

	if err := h.store.UpdateStatus(ctx, orderID, status); err != nil {
		log.Printf("[api-gateway] status update failed for order %s: %v", orderID, err)
		return err
	}

	log.Printf("[api-gateway] order %s → %s", orderID, status)
	return nil
}

func (h *Handler) GetOrder(c *gin.Context) {
	order, err := h.store.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}
	c.JSON(http.StatusOK, order)
}
