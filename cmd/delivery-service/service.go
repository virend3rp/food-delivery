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

var driverPool = []string{"Alice", "Bob", "Charlie", "Diana", "Eve"}

// Service holds dependencies for the delivery service handler.
type Service struct {
	pub     publisher
	store   deliveryStore
	sleepFn func(time.Duration)
}

func NewService(pub publisher, store deliveryStore) *Service {
	return &Service{
		pub:     pub,
		store:   store,
		sleepFn: time.Sleep,
	}
}

// HandleOrderAccepted processes an order.accepted event:
// assigns a driver, publishes driver.assigned → order.picked_up → order.delivered.
func (s *Service) HandleOrderAccepted(ctx context.Context, body []byte) error {
	var order events.OrderAcceptedEvent
	if err := json.Unmarshal(body, &order); err != nil {
		return fmt.Errorf("unmarshal order.accepted: %w", err)
	}

	log.Printf("[delivery-service] order %s accepted — finding driver", order.OrderID)

	driverID := uuid.New().String()[:8]
	driverName := driverPool[time.Now().UnixNano()%int64(len(driverPool))]

	if err := s.store.Create(ctx, Delivery{
		OrderID:    order.OrderID,
		DriverID:   driverID,
		DriverName: driverName,
	}); err != nil {
		log.Printf("[delivery-service] db create failed for order %s: %v", order.OrderID, err)
	}

	// driver assigned
	if err := s.pub.Publish(ctx, string(events.DriverAssigned), events.DriverAssignedEvent{
		BaseEvent:  events.BaseEvent{ID: uuid.New().String(), Type: events.DriverAssigned, Timestamp: time.Now().UTC()},
		OrderID:    order.OrderID,
		DriverID:   driverID,
		DriverName: driverName,
	}); err != nil {
		return fmt.Errorf("publish driver.assigned: %w", err)
	}
	log.Printf("[delivery-service] driver %s (%s) assigned to order %s", driverName, driverID, order.OrderID)

	// simulate driving to restaurant
	s.sleepFn(3 * time.Second)

	if err := s.store.UpdatePickedUp(ctx, order.OrderID); err != nil {
		log.Printf("[delivery-service] db update picked_up failed: %v", err)
	}
	if err := s.pub.Publish(ctx, string(events.OrderPickedUp), events.OrderPickedUpEvent{
		BaseEvent: events.BaseEvent{ID: uuid.New().String(), Type: events.OrderPickedUp, Timestamp: time.Now().UTC()},
		OrderID:   order.OrderID,
		DriverID:  driverID,
	}); err != nil {
		return fmt.Errorf("publish order.picked_up: %w", err)
	}
	log.Printf("[delivery-service] order %s picked up by %s", order.OrderID, driverName)

	// simulate driving to customer
	s.sleepFn(3 * time.Second)

	if err := s.store.UpdateDelivered(ctx, order.OrderID); err != nil {
		log.Printf("[delivery-service] db update delivered failed: %v", err)
	}
	if err := s.pub.Publish(ctx, string(events.OrderDelivered), events.OrderDeliveredEvent{
		BaseEvent: events.BaseEvent{ID: uuid.New().String(), Type: events.OrderDelivered, Timestamp: time.Now().UTC()},
		OrderID:   order.OrderID,
		DriverID:  driverID,
	}); err != nil {
		return fmt.Errorf("publish order.delivered: %w", err)
	}
	log.Printf("[delivery-service] order %s delivered by %s", order.OrderID, driverName)

	return nil
}
