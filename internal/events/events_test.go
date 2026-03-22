package events_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/virend3rp/food-delivery/internal/events"
)

func TestEventTypes(t *testing.T) {
	cases := []struct {
		name     string
		got      events.EventType
		expected string
	}{
		{"OrderCreated", events.OrderCreated, "order.created"},
		{"OrderAccepted", events.OrderAccepted, "order.accepted"},
		{"OrderRejected", events.OrderRejected, "order.rejected"},
		{"DriverAssigned", events.DriverAssigned, "driver.assigned"},
		{"OrderPickedUp", events.OrderPickedUp, "order.picked_up"},
		{"OrderDelivered", events.OrderDelivered, "order.delivered"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if string(tc.got) != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, tc.got)
			}
		})
	}
}

func TestOrderCreatedEvent_JSONRoundTrip(t *testing.T) {
	original := events.OrderCreatedEvent{
		BaseEvent: events.BaseEvent{
			ID:        "event-001",
			Type:      events.OrderCreated,
			Timestamp: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		},
		OrderID:      "order-123",
		CustomerID:   "cust-001",
		RestaurantID: "rest-001",
		Items: []events.Item{
			{Name: "Burger", Quantity: 2, Price: 9.99},
			{Name: "Fries", Quantity: 1, Price: 3.49},
		},
		TotalPrice: 23.47,
	}

	b, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded events.OrderCreatedEvent
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.OrderID != original.OrderID {
		t.Errorf("OrderID: got %q, want %q", decoded.OrderID, original.OrderID)
	}
	if decoded.CustomerID != original.CustomerID {
		t.Errorf("CustomerID: got %q, want %q", decoded.CustomerID, original.CustomerID)
	}
	if decoded.TotalPrice != original.TotalPrice {
		t.Errorf("TotalPrice: got %f, want %f", decoded.TotalPrice, original.TotalPrice)
	}
	if len(decoded.Items) != len(original.Items) {
		t.Errorf("Items length: got %d, want %d", len(decoded.Items), len(original.Items))
	}
	if decoded.Type != events.OrderCreated {
		t.Errorf("Type: got %q, want %q", decoded.Type, events.OrderCreated)
	}
}

func TestItem_TotalPrice(t *testing.T) {
	cases := []struct {
		name     string
		items    []events.Item
		expected float64
	}{
		{"single item", []events.Item{{Name: "Burger", Quantity: 1, Price: 9.99}}, 9.99},
		{"multiple items", []events.Item{
			{Name: "Burger", Quantity: 2, Price: 9.99},
			{Name: "Fries", Quantity: 1, Price: 3.49},
		}, 23.47},
		{"zero quantity", []events.Item{{Name: "Burger", Quantity: 0, Price: 9.99}}, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var total float64
			for _, item := range tc.items {
				total += item.Price * float64(item.Quantity)
			}
			if total != tc.expected {
				t.Errorf("total: got %f, want %f", total, tc.expected)
			}
		})
	}
}

func TestOrderAcceptedEvent_JSONRoundTrip(t *testing.T) {
	original := events.OrderAcceptedEvent{
		BaseEvent:     events.BaseEvent{ID: "evt-002", Type: events.OrderAccepted, Timestamp: time.Now().UTC()},
		OrderID:       "order-123",
		RestaurantID:  "rest-001",
		EstimatedTime: 30,
	}

	b, _ := json.Marshal(original)

	var decoded events.OrderAcceptedEvent
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.EstimatedTime != 30 {
		t.Errorf("EstimatedTime: got %d, want 30", decoded.EstimatedTime)
	}
}
