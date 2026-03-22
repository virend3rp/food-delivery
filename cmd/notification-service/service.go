package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/virend3rp/food-delivery/internal/events"
)

type notificationPublisher interface {
	Publish(ctx context.Context, routingKey string, payload any) error
}

// Service holds dependencies for the notification service handler.
type Service struct {
	store notificationStore
}

func NewService(store notificationStore) *Service {
	return &Service{store: store}
}

// HandleEvent routes any food-delivery event to a log entry.
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
		log.Printf("[notification] ORDER PLACED    | order=%-36s customer=%s total=$%.2f", e.OrderID, e.CustomerID, e.TotalPrice)

	case events.OrderAccepted:
		var e events.OrderAcceptedEvent
		json.Unmarshal(body, &e)
		orderID = e.OrderID
		message = fmt.Sprintf("Order accepted — ETA %d mins", e.EstimatedTime)
		log.Printf("[notification] ORDER ACCEPTED  | order=%-36s eta=%d mins", e.OrderID, e.EstimatedTime)

	case events.OrderRejected:
		var e events.OrderRejectedEvent
		json.Unmarshal(body, &e)
		orderID = e.OrderID
		message = fmt.Sprintf("Order rejected — %s", e.Reason)
		log.Printf("[notification] ORDER REJECTED  | order=%-36s reason=%s", e.OrderID, e.Reason)

	case events.DriverAssigned:
		var e events.DriverAssignedEvent
		json.Unmarshal(body, &e)
		orderID = e.OrderID
		message = fmt.Sprintf("Driver %s assigned", e.DriverName)
		log.Printf("[notification] DRIVER ASSIGNED | order=%-36s driver=%s", e.OrderID, e.DriverName)

	case events.OrderPickedUp:
		var e events.OrderPickedUpEvent
		json.Unmarshal(body, &e)
		orderID = e.OrderID
		message = "Order picked up — on the way!"
		log.Printf("[notification] ORDER PICKED UP | order=%-36s on the way!", e.OrderID)

	case events.OrderDelivered:
		var e events.OrderDeliveredEvent
		json.Unmarshal(body, &e)
		orderID = e.OrderID
		message = "Order delivered — enjoy your meal!"
		log.Printf("[notification] ORDER DELIVERED | order=%-36s enjoy your meal!", e.OrderID)

	default:
		log.Printf("[notification] UNKNOWN EVENT   | type=%s", base.Type)
		return nil
	}

	if err := s.store.Log(ctx, NotificationLog{
		OrderID:   orderID,
		EventType: string(base.Type),
		Message:   message,
	}); err != nil {
		log.Printf("[notification] db log failed: %v", err)
	}

	return nil
}
