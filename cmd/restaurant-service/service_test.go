package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/virend3rp/food-delivery/internal/events"
)

// --- mocks ---

type mockPublisher struct {
	calls []string
	err   error
}

func (m *mockPublisher) Publish(_ context.Context, routingKey string, _ any) error {
	if m.err != nil {
		return m.err
	}
	m.calls = append(m.calls, routingKey)
	return nil
}

type mockRestaurantStore struct {
	saved []RestaurantOrder
	err   error
}

func (m *mockRestaurantStore) Save(_ context.Context, record RestaurantOrder) error {
	if m.err != nil {
		return m.err
	}
	m.saved = append(m.saved, record)
	return nil
}

// --- helpers ---

func noSleep(_ time.Duration) {}

func newTestService(pub publisher, store restaurantStore) *Service {
	svc := NewService(pub, store)
	svc.sleepFn = noSleep
	return svc
}

func marshalOrderCreated(t *testing.T) []byte {
	t.Helper()
	e := events.OrderCreatedEvent{
		BaseEvent:    events.BaseEvent{ID: uuid.New().String(), Type: events.OrderCreated, Timestamp: time.Now()},
		OrderID:      uuid.New().String(),
		CustomerID:   "cust-001",
		RestaurantID: "rest-001",
		Items:        []events.Item{{Name: "Burger", Quantity: 2, Price: 9.99}},
		TotalPrice:   19.98,
	}
	b, _ := json.Marshal(e)
	return b
}

// --- tests ---

func TestHandleOrderCreated_Success(t *testing.T) {
	pub := &mockPublisher{}
	store := &mockRestaurantStore{}
	svc := newTestService(pub, store)

	if err := svc.HandleOrderCreated(context.Background(), marshalOrderCreated(t)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pub.calls) != 1 || pub.calls[0] != string(events.OrderAccepted) {
		t.Errorf("expected order.accepted publish, got %v", pub.calls)
	}
	if len(store.saved) != 1 {
		t.Errorf("expected 1 DB save, got %d", len(store.saved))
	}
	if store.saved[0].Status != "accepted" {
		t.Errorf("status: got %q, want 'accepted'", store.saved[0].Status)
	}
	if store.saved[0].EstimatedTime != 30 {
		t.Errorf("ETA: got %d, want 30", store.saved[0].EstimatedTime)
	}
}

func TestHandleOrderCreated_InvalidJSON(t *testing.T) {
	svc := newTestService(&mockPublisher{}, &mockRestaurantStore{})

	if err := svc.HandleOrderCreated(context.Background(), []byte("not json")); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestHandleOrderCreated_PublishError(t *testing.T) {
	pub := &mockPublisher{err: errors.New("broker down")}
	store := &mockRestaurantStore{}
	svc := newTestService(pub, store)

	if err := svc.HandleOrderCreated(context.Background(), marshalOrderCreated(t)); err == nil {
		t.Error("expected error when publish fails")
	}
}

func TestHandleOrderCreated_DBError_StillPublishes(t *testing.T) {
	// DB failure should be non-fatal
	pub := &mockPublisher{}
	store := &mockRestaurantStore{err: errors.New("db down")}
	svc := newTestService(pub, store)

	if err := svc.HandleOrderCreated(context.Background(), marshalOrderCreated(t)); err != nil {
		t.Errorf("unexpected error on DB failure: %v", err)
	}
	if len(pub.calls) != 1 {
		t.Error("expected publish despite DB error")
	}
}
