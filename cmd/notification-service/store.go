package main

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NotificationLog is a persisted record of a notification sent.
type NotificationLog struct {
	OrderID   string
	EventType string
	Message   string
}

// notificationStore is the persistence interface for notification logs.
type notificationStore interface {
	Log(ctx context.Context, entry NotificationLog) error
}

// PostgresNotificationStore implements notificationStore.
type PostgresNotificationStore struct {
	db *pgxpool.Pool
}

func NewPostgresNotificationStore(db *pgxpool.Pool) *PostgresNotificationStore {
	return &PostgresNotificationStore{db: db}
}

func (s *PostgresNotificationStore) Migrate(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS notification_logs (
			id         BIGSERIAL PRIMARY KEY,
			order_id   TEXT NOT NULL,
			event_type TEXT NOT NULL,
			message    TEXT NOT NULL,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)
	`)
	return err
}

func (s *PostgresNotificationStore) Log(ctx context.Context, entry NotificationLog) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO notification_logs (order_id, event_type, message)
		VALUES ($1, $2, $3)
	`, entry.OrderID, entry.EventType, entry.Message)
	return err
}
