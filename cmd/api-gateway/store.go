package main

import (
	"context"
	"fmt"

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

// orderStore is the persistence interface for orders.
type orderStore interface {
	Save(ctx context.Context, e events.OrderCreatedEvent) error
	GetByID(ctx context.Context, id string) (*Order, error)
	UpdateStatus(ctx context.Context, orderID, status string) error
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

func (s *PostgresOrderStore) UpdateStatus(ctx context.Context, orderID, status string) error {
	_, err := s.db.Exec(ctx, `
		UPDATE orders SET status = $1 WHERE id = $2
	`, status, orderID)
	return err
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
