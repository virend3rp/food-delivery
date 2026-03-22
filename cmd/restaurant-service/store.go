package main

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RestaurantOrder is the restaurant's view of an order.
type RestaurantOrder struct {
	OrderID       string
	RestaurantID  string
	Status        string
	EstimatedTime int
}

// restaurantStore is the persistence interface for restaurant orders.
type restaurantStore interface {
	Save(ctx context.Context, record RestaurantOrder) error
}

// PostgresRestaurantStore implements restaurantStore.
type PostgresRestaurantStore struct {
	db *pgxpool.Pool
}

func NewPostgresRestaurantStore(db *pgxpool.Pool) *PostgresRestaurantStore {
	return &PostgresRestaurantStore{db: db}
}

func (s *PostgresRestaurantStore) Migrate(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS restaurant_orders (
			order_id       TEXT PRIMARY KEY,
			restaurant_id  TEXT NOT NULL,
			status         TEXT NOT NULL,
			estimated_time INT,
			processed_at   TIMESTAMPTZ DEFAULT NOW()
		)
	`)
	return err
}

func (s *PostgresRestaurantStore) Save(ctx context.Context, record RestaurantOrder) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO restaurant_orders (order_id, restaurant_id, status, estimated_time)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (order_id) DO UPDATE SET status = EXCLUDED.status
	`, record.OrderID, record.RestaurantID, record.Status, record.EstimatedTime)
	return err
}
