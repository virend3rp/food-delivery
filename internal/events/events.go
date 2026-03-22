package events

import "time"

type EventType string

const (
	OrderCreated   EventType = "order.created"
	OrderAccepted  EventType = "order.accepted"
	OrderRejected  EventType = "order.rejected"
	DriverAssigned EventType = "driver.assigned"
	OrderPickedUp  EventType = "order.picked_up"
	OrderDelivered EventType = "order.delivered"
)

type BaseEvent struct {
	ID        string    `json:"id"`
	Type      EventType `json:"type"`
	Timestamp time.Time `json:"timestamp"`
}

type Item struct {
	Name     string  `json:"name"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
}

type OrderCreatedEvent struct {
	BaseEvent
	OrderID      string  `json:"order_id"`
	CustomerID   string  `json:"customer_id"`
	RestaurantID string  `json:"restaurant_id"`
	Items        []Item  `json:"items"`
	TotalPrice   float64 `json:"total_price"`
}

type OrderAcceptedEvent struct {
	BaseEvent
	OrderID       string `json:"order_id"`
	RestaurantID  string `json:"restaurant_id"`
	EstimatedTime int    `json:"estimated_time_minutes"`
}

type OrderRejectedEvent struct {
	BaseEvent
	OrderID string `json:"order_id"`
	Reason  string `json:"reason"`
}

type DriverAssignedEvent struct {
	BaseEvent
	OrderID    string `json:"order_id"`
	DriverID   string `json:"driver_id"`
	DriverName string `json:"driver_name"`
}

type OrderPickedUpEvent struct {
	BaseEvent
	OrderID  string `json:"order_id"`
	DriverID string `json:"driver_id"`
}

type OrderDeliveredEvent struct {
	BaseEvent
	OrderID  string `json:"order_id"`
	DriverID string `json:"driver_id"`
}
