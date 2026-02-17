package db

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultConnStr = "postgres://architect:architect_local@localhost:5432/architect?sslmode=disable"

func NewPool(ctx context.Context) (*pgxpool.Pool, error) {
	connStr := os.Getenv("ARCHITECT_DB_URL")
	if connStr == "" {
		connStr = defaultConnStr
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	return pool, nil
}
