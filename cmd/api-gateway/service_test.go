package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/virend3rp/food-delivery/internal/events"
)

// --- mocks ---

type mockPublisher struct {
	calls []string // routing keys published
	err   error
}

func (m *mockPublisher) Publish(_ context.Context, routingKey string, _ any) error {
	if m.err != nil {
		return m.err
	}
	m.calls = append(m.calls, routingKey)
	return nil
}

type mockOrderStore struct {
	saved []*events.OrderCreatedEvent
	order *Order
	err   error
}

func (m *mockOrderStore) Save(_ context.Context, e events.OrderCreatedEvent) error {
	if m.err != nil {
		return m.err
	}
	m.saved = append(m.saved, &e)
	return nil
}

func (m *mockOrderStore) GetByID(_ context.Context, _ string) (*Order, error) {
	if m.order == nil {
		return nil, errors.New("not found")
	}
	return m.order, nil
}

func (m *mockOrderStore) UpdateStatus(_ context.Context, _, _ string) error {
	return m.err
}

func (m *mockOrderStore) ListByCustomer(_ context.Context, _ string) ([]Order, error) {
	return []Order{}, m.err
}

func (m *mockOrderStore) RecordEvent(_ context.Context, _, _ string) error {
	return nil
}

func (m *mockOrderStore) GetTimeline(_ context.Context, _ string) ([]OrderEvent, error) {
	return []OrderEvent{}, m.err
}

// --- helpers ---

func newRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/orders", h.CreateOrder)
	r.GET("/orders/:id", h.GetOrder)
	return r
}

func postOrder(r *gin.Engine, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// --- tests ---

func TestCreateOrder_Success(t *testing.T) {
	pub := &mockPublisher{}
	store := &mockOrderStore{}
	r := newRouter(NewHandler(pub, store))

	w := postOrder(r, `{
		"customer_id":   "cust-001",
		"restaurant_id": "rest-001",
		"items": [{"name":"Burger","quantity":2,"price":9.99}]
	}`)

	if w.Code != http.StatusAccepted {
		t.Errorf("status: got %d, want 202", w.Code)
	}
	if len(pub.calls) != 1 || pub.calls[0] != string(events.OrderCreated) {
		t.Errorf("expected one order.created publish, got %v", pub.calls)
	}
	if len(store.saved) != 1 {
		t.Errorf("expected one DB save, got %d", len(store.saved))
	}
}

func TestCreateOrder_MissingCustomerID(t *testing.T) {
	pub := &mockPublisher{}
	store := &mockOrderStore{}
	r := newRouter(NewHandler(pub, store))

	w := postOrder(r, `{"restaurant_id":"rest-001","items":[{"name":"X","quantity":1,"price":1}]}`)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", w.Code)
	}
	if len(pub.calls) != 0 {
		t.Error("expected no publish on bad request")
	}
}

func TestCreateOrder_MissingItems(t *testing.T) {
	pub := &mockPublisher{}
	store := &mockOrderStore{}
	r := newRouter(NewHandler(pub, store))

	w := postOrder(r, `{"customer_id":"cust-001","restaurant_id":"rest-001","items":[]}`)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", w.Code)
	}
}

func TestCreateOrder_PublishError_Returns500(t *testing.T) {
	pub := &mockPublisher{err: errors.New("broker down")}
	store := &mockOrderStore{}
	r := newRouter(NewHandler(pub, store))

	w := postOrder(r, `{
		"customer_id":   "cust-001",
		"restaurant_id": "rest-001",
		"items": [{"name":"Burger","quantity":1,"price":9.99}]
	}`)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", w.Code)
	}
}

func TestCreateOrder_DBError_StillPublishes(t *testing.T) {
	// DB failure should be non-fatal — order should still be published
	pub := &mockPublisher{}
	store := &mockOrderStore{err: errors.New("db down")}
	r := newRouter(NewHandler(pub, store))

	w := postOrder(r, `{
		"customer_id":   "cust-001",
		"restaurant_id": "rest-001",
		"items": [{"name":"Burger","quantity":1,"price":9.99}]
	}`)

	if w.Code != http.StatusAccepted {
		t.Errorf("status: got %d, want 202 — DB error should be non-fatal", w.Code)
	}
	if len(pub.calls) != 1 {
		t.Error("expected publish despite DB error")
	}
}

func TestGetOrder_NotFound(t *testing.T) {
	pub := &mockPublisher{}
	store := &mockOrderStore{order: nil}
	r := newRouter(NewHandler(pub, store))

	req := httptest.NewRequest(http.MethodGet, "/orders/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", w.Code)
	}
}

func TestGetOrder_Found(t *testing.T) {
	pub := &mockPublisher{}
	store := &mockOrderStore{order: &Order{
		ID:           "order-123",
		CustomerID:   "cust-001",
		RestaurantID: "rest-001",
		TotalPrice:   19.98,
		Status:       "pending",
	}}
	r := newRouter(NewHandler(pub, store))

	req := httptest.NewRequest(http.MethodGet, "/orders/order-123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", w.Code)
	}
}

// --- HandleStatusUpdate tests ---

func marshalEvent(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func baseEvent(typ events.EventType) events.BaseEvent {
	return events.BaseEvent{ID: uuid.New().String(), Type: typ, Timestamp: time.Now()}
}

func TestHandleStatusUpdate_AllStatusTransitions(t *testing.T) {
	cases := []struct {
		name       string
		body       []byte
		wantStatus string
	}{
		{
			"accepted",
			marshalEvent(t, events.OrderAcceptedEvent{BaseEvent: baseEvent(events.OrderAccepted), OrderID: "o1"}),
			"accepted",
		},
		{
			"rejected",
			marshalEvent(t, events.OrderRejectedEvent{BaseEvent: baseEvent(events.OrderRejected), OrderID: "o1"}),
			"rejected",
		},
		{
			"driver_assigned",
			marshalEvent(t, events.DriverAssignedEvent{BaseEvent: baseEvent(events.DriverAssigned), OrderID: "o1"}),
			"driver_assigned",
		},
		{
			"out_for_delivery",
			marshalEvent(t, events.OrderPickedUpEvent{BaseEvent: baseEvent(events.OrderPickedUp), OrderID: "o1"}),
			"out_for_delivery",
		},
		{
			"delivered",
			marshalEvent(t, events.OrderDeliveredEvent{BaseEvent: baseEvent(events.OrderDelivered), OrderID: "o1"}),
			"delivered",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &mockOrderStore{}
			h := NewHandler(&mockPublisher{}, store)

			if err := h.HandleStatusUpdate(context.Background(), tc.body); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestHandleStatusUpdate_InvalidJSON(t *testing.T) {
	h := NewHandler(&mockPublisher{}, &mockOrderStore{})
	if err := h.HandleStatusUpdate(context.Background(), []byte("bad json")); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestHandleStatusUpdate_UnknownEvent_Ignored(t *testing.T) {
	// order.created should be ignored — api-gateway publishes it, doesn't consume it for status
	body := marshalEvent(t, events.BaseEvent{ID: "x", Type: "some.unknown", Timestamp: time.Now()})
	h := NewHandler(&mockPublisher{}, &mockOrderStore{})
	if err := h.HandleStatusUpdate(context.Background(), body); err != nil {
		t.Errorf("unknown events should be silently ignored, got: %v", err)
	}
}
