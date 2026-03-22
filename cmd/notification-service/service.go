package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/virend3rp/food-delivery/internal/events"
)

// Service holds dependencies for the notification service handler.
type Service struct {
	store notificationStore
}

func NewService(store notificationStore) *Service {
	return &Service{store: store}
}

// HandleEvent routes any food-delivery event to a structured log + DB entry.
func (s *Service) HandleEvent(ctx context.Context, body []byte) error {
	var base events.BaseEvent
	if err := json.Unmarshal(body, &base); err != nil {
		return fmt.Errorf("unmarshal base event: %w", err)
	}

	var orderID, message string

	switch base.Type {
	case events.OrderCreated:
		var e events.OrderCreatedEvent
		json.Unmarshal(body, &e)
		orderID = e.OrderID
		message = fmt.Sprintf("Order placed by %s — $%.2f", e.CustomerID, e.TotalPrice)
		slog.Info("order placed", "order_id", e.OrderID, "customer_id", e.CustomerID, "total", e.TotalPrice)

	case events.OrderAccepted:
		var e events.OrderAcceptedEvent
		json.Unmarshal(body, &e)
		orderID = e.OrderID
		message = fmt.Sprintf("Order accepted — ETA %d mins", e.EstimatedTime)
		slog.Info("order accepted", "order_id", e.OrderID, "eta_mins", e.EstimatedTime)

	case events.OrderRejected:
		var e events.OrderRejectedEvent
		json.Unmarshal(body, &e)
		orderID = e.OrderID
		message = fmt.Sprintf("Order rejected — %s", e.Reason)
		slog.Info("order rejected", "order_id", e.OrderID, "reason", e.Reason)

	case events.DriverAssigned:
		var e events.DriverAssignedEvent
		json.Unmarshal(body, &e)
		orderID = e.OrderID
		message = fmt.Sprintf("Driver %s assigned", e.DriverName)
		slog.Info("driver assigned", "order_id", e.OrderID, "driver", e.DriverName)

	case events.OrderPickedUp:
		var e events.OrderPickedUpEvent
		json.Unmarshal(body, &e)
		orderID = e.OrderID
		message = "Order picked up — on the way!"
		slog.Info("order picked up", "order_id", e.OrderID)

	case events.OrderDelivered:
		var e events.OrderDeliveredEvent
		json.Unmarshal(body, &e)
		orderID = e.OrderID
		message = "Order delivered — enjoy your meal!"
		slog.Info("order delivered", "order_id", e.OrderID)

	default:
		slog.Warn("unknown event type", "type", base.Type)
		return nil
	}

	if err := s.store.Log(ctx, NotificationLog{
		OrderID:   orderID,
		EventType: string(base.Type),
		Message:   message,
	}); err != nil {
		slog.Error("db log failed", "order_id", orderID, "err", err)
	}

	return nil
}
