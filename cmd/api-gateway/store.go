package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/virend3rp/food-delivery/internal/events"
)

// Order is the persisted representation of a customer order.
type Order struct {
	ID           string  `json:"id"`
	CustomerID   string  `json:"customer_id"`
	RestaurantID string  `json:"restaurant_id"`
	TotalPrice   float64 `json:"total_price"`
	Status       string  `json:"status"`
}

// OrderEvent is one entry in an order's timeline.
type OrderEvent struct {
	EventType  string    `json:"event"`
	OccurredAt time.Time `json:"occurred_at"`
}

// orderStore is the persistence interface for orders.
type orderStore interface {
	Save(ctx context.Context, e events.OrderCreatedEvent) error
	GetByID(ctx context.Context, id string) (*Order, error)
	UpdateStatus(ctx context.Context, orderID, status string) error
	ListByCustomer(ctx context.Context, customerID string) ([]Order, error)
	RecordEvent(ctx context.Context, orderID, eventType string) error
	GetTimeline(ctx context.Context, orderID string) ([]OrderEvent, error)
}

// PostgresOrderStore implements orderStore using PostgreSQL.
type PostgresOrderStore struct {
	db *pgxpool.Pool
}

func NewPostgresOrderStore(db *pgxpool.Pool) *PostgresOrderStore {
	return &PostgresOrderStore{db: db}
}

func (s *PostgresOrderStore) Migrate(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS orders (
			id            TEXT PRIMARY KEY,
			customer_id   TEXT NOT NULL,
			restaurant_id TEXT NOT NULL,
			total_price   NUMERIC(10,2) NOT NULL,
			status        TEXT NOT NULL DEFAULT 'pending',
			created_at    TIMESTAMPTZ DEFAULT NOW()
		);
		CREATE TABLE IF NOT EXISTS order_items (
			id        BIGSERIAL PRIMARY KEY,
			order_id  TEXT REFERENCES orders(id),
			name      TEXT NOT NULL,
			quantity  INT  NOT NULL,
			price     NUMERIC(10,2) NOT NULL
		);
		CREATE TABLE IF NOT EXISTS order_events (
			id          BIGSERIAL PRIMARY KEY,
			order_id    TEXT NOT NULL,
			event_type  TEXT NOT NULL,
			occurred_at TIMESTAMPTZ DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_orders_customer_id    ON orders(customer_id);
		CREATE INDEX IF NOT EXISTS idx_order_events_order_id ON order_events(order_id);
	`)
	return err
}

func (s *PostgresOrderStore) Save(ctx context.Context, e events.OrderCreatedEvent) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err = tx.Exec(ctx, `
		INSERT INTO orders (id, customer_id, restaurant_id, total_price)
		VALUES ($1, $2, $3, $4)
	`, e.OrderID, e.CustomerID, e.RestaurantID, e.TotalPrice); err != nil {
		return fmt.Errorf("insert order: %w", err)
	}

	for _, item := range e.Items {
		if _, err = tx.Exec(ctx, `
			INSERT INTO order_items (order_id, name, quantity, price)
			VALUES ($1, $2, $3, $4)
		`, e.OrderID, item.Name, item.Quantity, item.Price); err != nil {
			return fmt.Errorf("insert item: %w", err)
		}
	}

	return tx.Commit(ctx)
}

func (s *PostgresOrderStore) GetByID(ctx context.Context, id string) (*Order, error) {
	var o Order
	err := s.db.QueryRow(ctx, `
		SELECT id, customer_id, restaurant_id, total_price, status
		FROM orders WHERE id = $1
	`, id).Scan(&o.ID, &o.CustomerID, &o.RestaurantID, &o.TotalPrice, &o.Status)
	if err != nil {
		return nil, fmt.Errorf("query order %s: %w", id, err)
	}
	return &o, nil
}

func (s *PostgresOrderStore) UpdateStatus(ctx context.Context, orderID, status string) error {
	_, err := s.db.Exec(ctx, `
		UPDATE orders SET status = $1 WHERE id = $2
	`, status, orderID)
	return err
}

func (s *PostgresOrderStore) ListByCustomer(ctx context.Context, customerID string) ([]Order, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, customer_id, restaurant_id, total_price, status
		FROM orders WHERE customer_id = $1 ORDER BY created_at DESC
	`, customerID)
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.CustomerID, &o.RestaurantID, &o.TotalPrice, &o.Status); err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

func (s *PostgresOrderStore) RecordEvent(ctx context.Context, orderID, eventType string) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO order_events (order_id, event_type) VALUES ($1, $2)
	`, orderID, eventType)
	return err
}

func (s *PostgresOrderStore) GetTimeline(ctx context.Context, orderID string) ([]OrderEvent, error) {
	rows, err := s.db.Query(ctx, `
		SELECT event_type, occurred_at FROM order_events
		WHERE order_id = $1 ORDER BY occurred_at ASC
	`, orderID)
	if err != nil {
		return nil, fmt.Errorf("get timeline: %w", err)
	}
	defer rows.Close()

	var timeline []OrderEvent
	for rows.Next() {
		var e OrderEvent
		if err := rows.Scan(&e.EventType, &e.OccurredAt); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		timeline = append(timeline, e)
	}
	return timeline, rows.Err()
}
