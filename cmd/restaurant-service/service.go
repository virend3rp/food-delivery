package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	sleepFn func(time.Duration) // injectable for testing
}

func NewService(pub publisher, store restaurantStore) *Service {
	return &Service{
		pub:     pub,
		store:   store,
		sleepFn: time.Sleep,
	}
}

// HandleOrderCreated processes an order.created event:
// simulates kitchen review, saves to DB, publishes order.accepted.
func (s *Service) HandleOrderCreated(ctx context.Context, body []byte) error {
	var order events.OrderCreatedEvent
	if err := json.Unmarshal(body, &order); err != nil {
		return fmt.Errorf("unmarshal order.created: %w", err)
	}

	log.Printf("[restaurant-service] order %s received — %d item(s) — $%.2f",
		order.OrderID, len(order.Items), order.TotalPrice)

	s.sleepFn(2 * time.Second) // simulate kitchen review

	record := RestaurantOrder{
		OrderID:       order.OrderID,
		RestaurantID:  order.RestaurantID,
		Status:        "accepted",
		EstimatedTime: 30,
	}
	if err := s.store.Save(ctx, record); err != nil {
		log.Printf("[restaurant-service] db save failed for order %s: %v", order.OrderID, err)
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

	log.Printf("[restaurant-service] order %s accepted — ETA 30 mins", order.OrderID)
	return nil
}
