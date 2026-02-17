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

// ClaimTask atomically assigns a task to an agent, returning false if already claimed.
func ClaimTask(ctx context.Context, pool *pgxpool.Pool, taskID int64, status string, agentID string) (bool, error) {
	tag, err := pool.Exec(ctx,
		`UPDATE tasks SET status = $1, assigned_to = $2
		 WHERE id = $3 AND assigned_to IS NULL`,
		status, agentID, taskID,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// UpdateTaskStatus updates the status of a task.
func UpdateTaskStatus(ctx context.Context, pool *pgxpool.Pool, taskID int64, status string) error {
	_, err := pool.Exec(ctx,
		`UPDATE tasks SET status = $1 WHERE id = $2`,
		status, taskID,
	)
	return err
}

// CompleteTask marks a task as completed by its assigned agent.
func CompleteTask(ctx context.Context, pool *pgxpool.Pool, taskID int64, agentID string) error {
	_, err := pool.Exec(ctx,
		`UPDATE tasks SET status = 'completed', updated_at = NOW()
		 WHERE id = $1 AND assigned_to = $2`,
		taskID, agentID,
	)
	return err
}

// ParentBranches returns the worktree_path values of agents that completed
// the direct dependencies (blocking tasks) of the given task.
func ParentBranches(ctx context.Context, pool *pgxpool.Pool, taskID int64) ([]string, error) {
	rows, err := pool.Query(ctx,
		`SELECT DISTINCT a.worktree_path
		 FROM task_edges e
		 JOIN tasks t ON e.from_task = t.id
		 JOIN agents a ON t.assigned_to = a.agent_id
		 WHERE e.to_task = $1
		   AND e.edge_type = 'blocks'
		   AND t.status = 'completed'
		   AND a.worktree_path IS NOT NULL`,
		taskID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}

// ReclaimTask resets a task to pending and clears its assignment.
func ReclaimTask(ctx context.Context, pool *pgxpool.Pool, taskID int64) error {
	_, err := pool.Exec(ctx,
		`UPDATE tasks SET status = 'pending', assigned_to = NULL WHERE id = $1`,
		taskID,
	)
	return err
}
