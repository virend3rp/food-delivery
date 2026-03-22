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

type mockDeliveryStore struct {
	created    []Delivery
	pickedUpIDs []string
	deliveredIDs []string
	err        error
}

func (m *mockDeliveryStore) Create(_ context.Context, d Delivery) error {
	if m.err != nil {
		return m.err
	}
	m.created = append(m.created, d)
	return nil
}

func (m *mockDeliveryStore) UpdatePickedUp(_ context.Context, orderID string) error {
	if m.err != nil {
		return m.err
	}
	m.pickedUpIDs = append(m.pickedUpIDs, orderID)
	return nil
}

func (m *mockDeliveryStore) UpdateDelivered(_ context.Context, orderID string) error {
	if m.err != nil {
		return m.err
	}
	m.deliveredIDs = append(m.deliveredIDs, orderID)
	return nil
}

// --- helpers ---

func noSleep(_ time.Duration) {}

func newTestService(pub publisher, store deliveryStore) *Service {
	svc := NewService(pub, store)
	svc.sleepFn = noSleep
	return svc
}

func marshalOrderAccepted(t *testing.T) []byte {
	t.Helper()
	e := events.OrderAcceptedEvent{
		BaseEvent:     events.BaseEvent{ID: uuid.New().String(), Type: events.OrderAccepted, Timestamp: time.Now()},
		OrderID:       uuid.New().String(),
		RestaurantID:  "rest-001",
		EstimatedTime: 30,
	}
	b, _ := json.Marshal(e)
	return b
}

// --- tests ---

func TestHandleOrderAccepted_Success(t *testing.T) {
	pub := &mockPublisher{}
	store := &mockDeliveryStore{}
	svc := newTestService(pub, store)

	if err := svc.HandleOrderAccepted(context.Background(), marshalOrderAccepted(t)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{
		string(events.DriverAssigned),
		string(events.OrderPickedUp),
		string(events.OrderDelivered),
	}
	if len(pub.calls) != len(want) {
		t.Fatalf("publish calls: got %v, want %v", pub.calls, want)
	}
	for i, key := range want {
		if pub.calls[i] != key {
			t.Errorf("call[%d]: got %q, want %q", i, pub.calls[i], key)
		}
	}

	if len(store.created) != 1 {
		t.Errorf("expected 1 delivery created, got %d", len(store.created))
	}
	if len(store.pickedUpIDs) != 1 {
		t.Errorf("expected 1 picked-up update, got %d", len(store.pickedUpIDs))
	}
	if len(store.deliveredIDs) != 1 {
		t.Errorf("expected 1 delivered update, got %d", len(store.deliveredIDs))
	}
}

func TestHandleOrderAccepted_InvalidJSON(t *testing.T) {
	svc := newTestService(&mockPublisher{}, &mockDeliveryStore{})

	if err := svc.HandleOrderAccepted(context.Background(), []byte("bad json")); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestHandleOrderAccepted_PublishError(t *testing.T) {
	pub := &mockPublisher{err: errors.New("broker down")}
	store := &mockDeliveryStore{}
	svc := newTestService(pub, store)

	if err := svc.HandleOrderAccepted(context.Background(), marshalOrderAccepted(t)); err == nil {
		t.Error("expected error when publish fails")
	}
}

func TestHandleOrderAccepted_DBError_StillPublishes(t *testing.T) {
	pub := &mockPublisher{}
	store := &mockDeliveryStore{err: errors.New("db down")}
	svc := newTestService(pub, store)

	if err := svc.HandleOrderAccepted(context.Background(), marshalOrderAccepted(t)); err != nil {
		t.Errorf("unexpected error on DB failure: %v", err)
	}
	if len(pub.calls) != 3 {
		t.Errorf("expected 3 publishes despite DB error, got %d", len(pub.calls))
	}
}
