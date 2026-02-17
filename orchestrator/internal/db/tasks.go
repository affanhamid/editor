package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// InsertTask creates a new task in Postgres and returns the assigned ID.
func InsertTask(ctx context.Context, pool *pgxpool.Pool, title, description, riskLevel string) (int64, error) {
	var id int64
	err := pool.QueryRow(ctx,
		`INSERT INTO tasks (title, description, risk_level, status)
		 VALUES ($1, $2, $3, 'pending')
		 RETURNING id`,
		title, description, riskLevel,
	).Scan(&id)
	return id, err
}

// InsertEdge creates a dependency edge between two tasks.
func InsertEdge(ctx context.Context, pool *pgxpool.Pool, fromTask, toTask int64) error {
	_, err := pool.Exec(ctx,
		`INSERT INTO task_edges (from_task, to_task, edge_type) VALUES ($1, $2, 'blocks')`,
		fromTask, toTask,
	)
	return err
}

// UpdateTaskStatus updates the status of a task and optionally assigns it.
func UpdateTaskStatus(ctx context.Context, pool *pgxpool.Pool, taskID int64, status string, assignedTo *string) error {
	if assignedTo != nil {
		_, err := pool.Exec(ctx,
			`UPDATE tasks SET status = $1, assigned_to = $2 WHERE id = $3`,
			status, *assignedTo, taskID,
		)
		return err
	}
	_, err := pool.Exec(ctx,
		`UPDATE tasks SET status = $1 WHERE id = $2`,
		status, taskID,
	)
	return err
}

// ReclaimTask resets a task to pending and clears its assignment.
func ReclaimTask(ctx context.Context, pool *pgxpool.Pool, taskID int64) error {
	_, err := pool.Exec(ctx,
		`UPDATE tasks SET status = 'pending', assigned_to = NULL WHERE id = $1`,
		taskID,
	)
	return err
}
