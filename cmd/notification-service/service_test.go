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

type mockNotificationStore struct {
	logs []NotificationLog
	err  error
}

func (m *mockNotificationStore) Log(_ context.Context, entry NotificationLog) error {
	if m.err != nil {
		return m.err
	}
	m.logs = append(m.logs, entry)
	return nil
}

// --- helpers ---

func baseEvent(t events.EventType) events.BaseEvent {
	return events.BaseEvent{ID: uuid.New().String(), Type: t, Timestamp: time.Now()}
}

func marshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// --- tests ---

func TestHandleEvent_OrderCreated(t *testing.T) {
	store := &mockNotificationStore{}
	svc := NewService(store)

	body := marshal(t, events.OrderCreatedEvent{
		BaseEvent:  baseEvent(events.OrderCreated),
		OrderID:    "order-001",
		CustomerID: "cust-001",
		TotalPrice: 19.98,
		Items:      []events.Item{{Name: "Burger", Quantity: 1, Price: 19.98}},
	})

	if err := svc.HandleEvent(context.Background(), body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(store.logs))
	}
	if store.logs[0].EventType != string(events.OrderCreated) {
		t.Errorf("event type: got %q, want %q", store.logs[0].EventType, events.OrderCreated)
	}
	if store.logs[0].OrderID != "order-001" {
		t.Errorf("order ID: got %q, want 'order-001'", store.logs[0].OrderID)
	}
}

func TestHandleEvent_AllEventTypes(t *testing.T) {
	cases := []struct {
		name    string
		payload any
		wantKey events.EventType
	}{
		{
			"OrderAccepted",
			events.OrderAcceptedEvent{BaseEvent: baseEvent(events.OrderAccepted), OrderID: "o1", EstimatedTime: 30},
			events.OrderAccepted,
		},
		{
			"OrderRejected",
			events.OrderRejectedEvent{BaseEvent: baseEvent(events.OrderRejected), OrderID: "o2", Reason: "closed"},
			events.OrderRejected,
		},
		{
			"DriverAssigned",
			events.DriverAssignedEvent{BaseEvent: baseEvent(events.DriverAssigned), OrderID: "o3", DriverName: "Alice"},
			events.DriverAssigned,
		},
		{
			"OrderPickedUp",
			events.OrderPickedUpEvent{BaseEvent: baseEvent(events.OrderPickedUp), OrderID: "o4"},
			events.OrderPickedUp,
		},
		{
			"OrderDelivered",
			events.OrderDeliveredEvent{BaseEvent: baseEvent(events.OrderDelivered), OrderID: "o5"},
			events.OrderDelivered,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &mockNotificationStore{}
			svc := NewService(store)

			if err := svc.HandleEvent(context.Background(), marshal(t, tc.payload)); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(store.logs) != 1 {
				t.Fatalf("expected 1 log, got %d", len(store.logs))
			}
			if store.logs[0].EventType != string(tc.wantKey) {
				t.Errorf("event type: got %q, want %q", store.logs[0].EventType, tc.wantKey)
			}
		})
	}
}

func TestHandleEvent_InvalidJSON(t *testing.T) {
	svc := NewService(&mockNotificationStore{})

	if err := svc.HandleEvent(context.Background(), []byte("not json")); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestHandleEvent_UnknownEventType_NoError(t *testing.T) {
	store := &mockNotificationStore{}
	svc := NewService(store)

	body := marshal(t, events.BaseEvent{ID: "x", Type: "some.unknown.event", Timestamp: time.Now()})

	if err := svc.HandleEvent(context.Background(), body); err != nil {
		t.Errorf("unknown event type should not return error, got: %v", err)
	}
	if len(store.logs) != 0 {
		t.Error("unknown events should not be logged")
	}
}

func TestHandleEvent_DBError_NoHandlerError(t *testing.T) {
	// DB log failure should not propagate as a handler error
	store := &mockNotificationStore{err: errors.New("db down")}
	svc := NewService(store)

	body := marshal(t, events.OrderDeliveredEvent{
		BaseEvent: baseEvent(events.OrderDelivered),
		OrderID:   "order-99",
	})

	if err := svc.HandleEvent(context.Background(), body); err != nil {
		t.Errorf("DB error should be non-fatal, got: %v", err)
	}
}
