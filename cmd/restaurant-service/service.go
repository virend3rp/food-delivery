package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/virend3rp/food-delivery/internal/events"
)

type publisher interface {
	Publish(ctx context.Context, routingKey string, payload any) error
}

// Service holds dependencies for the restaurant service handler.
type Service struct {
	pub     publisher
	store   restaurantStore
	sleepFn func(time.Duration)
}

func NewService(pub publisher, store restaurantStore) *Service {
	return &Service{pub: pub, store: store, sleepFn: time.Sleep}
}

// HandleOrderCreated processes an order.created event.
func (s *Service) HandleOrderCreated(ctx context.Context, body []byte) error {
	var order events.OrderCreatedEvent
	if err := json.Unmarshal(body, &order); err != nil {
		return fmt.Errorf("unmarshal order.created: %w", err)
	}

	slog.Info("order received", "order_id", order.OrderID, "items", len(order.Items), "total", order.TotalPrice)

	s.sleepFn(2 * time.Second)

	record := RestaurantOrder{
		OrderID:       order.OrderID,
		RestaurantID:  order.RestaurantID,
		Status:        "accepted",
		EstimatedTime: 30,
	}
	if err := s.store.Save(ctx, record); err != nil {
		slog.Error("db save failed", "order_id", order.OrderID, "err", err)
	}

	accepted := events.OrderAcceptedEvent{
		BaseEvent:     events.BaseEvent{ID: uuid.New().String(), Type: events.OrderAccepted, Timestamp: time.Now().UTC()},
		OrderID:       order.OrderID,
		RestaurantID:  order.RestaurantID,
		EstimatedTime: 30,
	}
	if err := s.pub.Publish(ctx, string(events.OrderAccepted), accepted); err != nil {
		return fmt.Errorf("publish order.accepted: %w", err)
	}

	slog.Info("order accepted", "order_id", order.OrderID, "eta_mins", 30)
	return nil
}
