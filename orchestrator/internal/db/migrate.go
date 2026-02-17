package db

import (
	"context"
	"embed"
	"fmt"
	"log"
	"net/url"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

//go:embed triggers/*.sql
var triggersFS embed.FS

// EnsureDatabase connects to the "postgres" maintenance DB and creates the
// target database if it doesn't already exist.
func EnsureDatabase(connStr string) error {
	// Parse the connection string to extract the target DB name.
	u, err := url.Parse(connStr)
	if err != nil {
		return fmt.Errorf("parse connection string: %w", err)
	}
	targetDB := strings.TrimPrefix(u.Path, "/")
	if targetDB == "" {
		return fmt.Errorf("no database name in connection string")
	}

	// Build a connection string pointing to the "postgres" DB instead.
	adminURL := *u
	adminURL.Path = "/postgres"
	adminConn, err := pgx.Connect(context.Background(), adminURL.String())
	if err != nil {
		return fmt.Errorf("connect to postgres DB: %w", err)
	}
	defer adminConn.Close(context.Background())

	// Check if the target DB exists.
	var exists bool
	err = adminConn.QueryRow(context.Background(),
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", targetDB).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check database existence: %w", err)
	}

	if !exists {
		// CREATE DATABASE cannot use parameter substitution.
		_, err = adminConn.Exec(context.Background(),
			fmt.Sprintf("CREATE DATABASE %s", pgx.Identifier{targetDB}.Sanitize()))
		if err != nil {
			return fmt.Errorf("create database %s: %w", targetDB, err)
		}
		log.Printf("created database %q", targetDB)
	}

	return nil
}

// RunMigrations applies all pending migrations and re-applies all triggers.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	// Ensure the schema_migrations tracking table exists.
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename VARCHAR(256) PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	// Run migration files in order.
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()

		// Check if already applied.
		var applied bool
		err := pool.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE filename = $1)", name).Scan(&applied)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if applied {
			continue
		}

		sql, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin tx for %s: %w", name, err)
		}

		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("execute migration %s: %w", name, err)
		}

		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (filename) VALUES ($1)", name); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("record migration %s: %w", name, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}

		log.Printf("applied migration: %s", name)
	}

	// Re-apply all triggers (idempotent via CREATE OR REPLACE + DROP TRIGGER IF EXISTS).
	triggerEntries, err := triggersFS.ReadDir("triggers")
	if err != nil {
		return fmt.Errorf("read triggers dir: %w", err)
	}
	sort.Slice(triggerEntries, func(i, j int) bool {
		return triggerEntries[i].Name() < triggerEntries[j].Name()
	})

	for _, entry := range triggerEntries {
		if entry.IsDir() {
			continue
		}
		sql, err := triggersFS.ReadFile("triggers/" + entry.Name())
		if err != nil {
			return fmt.Errorf("read trigger %s: %w", entry.Name(), err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("apply trigger %s: %w", entry.Name(), err)
		}
	}

	return nil
}
