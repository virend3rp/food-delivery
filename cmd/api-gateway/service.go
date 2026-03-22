package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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
		slog.Error("db save failed", "order_id", event.OrderID, "err", err)
	} else {
		h.store.RecordEvent(c.Request.Context(), event.OrderID, string(events.OrderCreated))
	}

	if err := h.pub.Publish(c.Request.Context(), string(events.OrderCreated), event); err != nil {
		slog.Error("publish failed", "order_id", event.OrderID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to queue order"})
		return
	}

	slog.Info("order created", "order_id", event.OrderID, "customer_id", req.CustomerID, "total", total)
	c.JSON(http.StatusAccepted, gin.H{
		"order_id": event.OrderID,
		"status":   "pending",
		"message":  "your order is being processed",
	})
}

func (h *Handler) GetOrder(c *gin.Context) {
	id := c.Param("id")
	order, err := h.store.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}
	c.JSON(http.StatusOK, order)
}

func (h *Handler) ListOrders(c *gin.Context) {
	customerID := c.Query("customer_id")
	if customerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "customer_id query param is required"})
		return
	}

	orders, err := h.store.ListByCustomer(c.Request.Context(), customerID)
	if err != nil {
		slog.Error("list orders failed", "customer_id", customerID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch orders"})
		return
	}

	if orders == nil {
		orders = []Order{} // return [] not null
	}
	c.JSON(http.StatusOK, orders)
}

func (h *Handler) GetTimeline(c *gin.Context) {
	id := c.Param("id")
	timeline, err := h.store.GetTimeline(c.Request.Context(), id)
	if err != nil {
		slog.Error("get timeline failed", "order_id", id, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch timeline"})
		return
	}

	if timeline == nil {
		timeline = []OrderEvent{} // return [] not null
	}
	c.JSON(http.StatusOK, timeline)
}

// statusMap maps incoming event types to the order status stored in orders_db.
var statusMap = map[events.EventType]string{
	events.OrderAccepted:  "accepted",
	events.OrderRejected:  "rejected",
	events.DriverAssigned: "driver_assigned",
	events.OrderPickedUp:  "out_for_delivery",
	events.OrderDelivered: "delivered",
}

// HandleStatusUpdate consumes downstream events, updates order status,
// and records every event in the order timeline.
func (h *Handler) HandleStatusUpdate(ctx context.Context, body []byte) error {
	var base events.BaseEvent
	if err := json.Unmarshal(body, &base); err != nil {
		return fmt.Errorf("unmarshal base event: %w", err)
	}

	status, ok := statusMap[base.Type]
	if !ok {
		return nil
	}

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
		slog.Error("status update failed", "order_id", orderID, "status", status, "err", err)
		return err
	}

	h.store.RecordEvent(ctx, orderID, string(base.Type))

	slog.Info("order status updated", "order_id", orderID, "status", status)
	return nil
}
