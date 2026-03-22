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

var driverPool = []string{"Alice", "Bob", "Charlie", "Diana", "Eve"}

// Service holds dependencies for the delivery service handler.
type Service struct {
	pub     publisher
	store   deliveryStore
	sleepFn func(time.Duration)
}

func NewService(pub publisher, store deliveryStore) *Service {
	return &Service{pub: pub, store: store, sleepFn: time.Sleep}
}

// HandleOrderAccepted processes an order.accepted event.
func (s *Service) HandleOrderAccepted(ctx context.Context, body []byte) error {
	var order events.OrderAcceptedEvent
	if err := json.Unmarshal(body, &order); err != nil {
		return fmt.Errorf("unmarshal order.accepted: %w", err)
	}

	slog.Info("finding driver", "order_id", order.OrderID)

	driverID := uuid.New().String()[:8]
	driverName := driverPool[time.Now().UnixNano()%int64(len(driverPool))]

	if err := s.store.Create(ctx, Delivery{OrderID: order.OrderID, DriverID: driverID, DriverName: driverName}); err != nil {
		slog.Error("db create failed", "order_id", order.OrderID, "err", err)
	}

	if err := s.pub.Publish(ctx, string(events.DriverAssigned), events.DriverAssignedEvent{
		BaseEvent:  events.BaseEvent{ID: uuid.New().String(), Type: events.DriverAssigned, Timestamp: time.Now().UTC()},
		OrderID:    order.OrderID,
		DriverID:   driverID,
		DriverName: driverName,
	}); err != nil {
		return fmt.Errorf("publish driver.assigned: %w", err)
	}
	slog.Info("driver assigned", "order_id", order.OrderID, "driver", driverName, "driver_id", driverID)

	s.sleepFn(3 * time.Second)

	if err := s.store.UpdatePickedUp(ctx, order.OrderID); err != nil {
		slog.Error("db update picked_up failed", "order_id", order.OrderID, "err", err)
	}
	if err := s.pub.Publish(ctx, string(events.OrderPickedUp), events.OrderPickedUpEvent{
		BaseEvent: events.BaseEvent{ID: uuid.New().String(), Type: events.OrderPickedUp, Timestamp: time.Now().UTC()},
		OrderID:   order.OrderID,
		DriverID:  driverID,
	}); err != nil {
		return fmt.Errorf("publish order.picked_up: %w", err)
	}
	slog.Info("order picked up", "order_id", order.OrderID, "driver", driverName)

	s.sleepFn(3 * time.Second)

	if err := s.store.UpdateDelivered(ctx, order.OrderID); err != nil {
		slog.Error("db update delivered failed", "order_id", order.OrderID, "err", err)
	}
	if err := s.pub.Publish(ctx, string(events.OrderDelivered), events.OrderDeliveredEvent{
		BaseEvent: events.BaseEvent{ID: uuid.New().String(), Type: events.OrderDelivered, Timestamp: time.Now().UTC()},
		OrderID:   order.OrderID,
		DriverID:  driverID,
	}); err != nil {
		return fmt.Errorf("publish order.delivered: %w", err)
	}
	slog.Info("order delivered", "order_id", order.OrderID, "driver", driverName)

	return nil
}
