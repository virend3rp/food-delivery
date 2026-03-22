package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Delivery tracks the state of a delivery.
type Delivery struct {
	OrderID     string
	DriverID    string
	DriverName  string
	Status      string
	AssignedAt  time.Time
	PickedUpAt  *time.Time
	DeliveredAt *time.Time
}

// deliveryStore is the persistence interface for deliveries.
type deliveryStore interface {
	Create(ctx context.Context, d Delivery) error
	UpdatePickedUp(ctx context.Context, orderID string) error
	UpdateDelivered(ctx context.Context, orderID string) error
}

// PostgresDeliveryStore implements deliveryStore.
type PostgresDeliveryStore struct {
	db *pgxpool.Pool
}

func NewPostgresDeliveryStore(db *pgxpool.Pool) *PostgresDeliveryStore {
	return &PostgresDeliveryStore{db: db}
}

func (s *PostgresDeliveryStore) Migrate(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS deliveries (
			order_id     TEXT PRIMARY KEY,
			driver_id    TEXT NOT NULL,
			driver_name  TEXT NOT NULL,
			status       TEXT NOT NULL DEFAULT 'assigned',
			assigned_at  TIMESTAMPTZ DEFAULT NOW(),
			picked_up_at TIMESTAMPTZ,
			delivered_at TIMESTAMPTZ
		)
	`)
	return err
}

func (s *PostgresDeliveryStore) Create(ctx context.Context, d Delivery) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO deliveries (order_id, driver_id, driver_name, status)
		VALUES ($1, $2, $3, 'assigned')
	`, d.OrderID, d.DriverID, d.DriverName)
	return err
}

func (s *PostgresDeliveryStore) UpdatePickedUp(ctx context.Context, orderID string) error {
	res, err := s.db.Exec(ctx, `
		UPDATE deliveries SET status = 'picked_up', picked_up_at = NOW()
		WHERE order_id = $1
	`, orderID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("delivery not found: %s", orderID)
	}
	return nil
}

func (s *PostgresDeliveryStore) UpdateDelivered(ctx context.Context, orderID string) error {
	res, err := s.db.Exec(ctx, `
		UPDATE deliveries SET status = 'delivered', delivered_at = NOW()
		WHERE order_id = $1
	`, orderID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("delivery not found: %s", orderID)
	}
	return nil
}
