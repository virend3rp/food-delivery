package db

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func New(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse DSN: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	for i := range 5 {
		if err = pool.Ping(ctx); err == nil {
			log.Println("[db] connected")
			return pool, nil
		}
		log.Printf("[db] ping attempt %d failed: %v", i+1, err)
		time.Sleep(time.Duration(i+1) * 2 * time.Second)
	}

	return nil, fmt.Errorf("postgres unreachable after retries: %w", err)
}
