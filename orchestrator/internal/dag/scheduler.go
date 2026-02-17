package dag

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ReadyTasks queries Postgres for tasks that are pending, unassigned,
// and have all blocking tasks completed.
func ReadyTasks(ctx context.Context, db *pgxpool.Pool) ([]Task, error) {
	query := `
		SELECT t.id, t.title, t.description, t.risk_level
		FROM tasks t
		WHERE t.status = 'pending'
		  AND t.assigned_to IS NULL
		  AND NOT EXISTS (
		      SELECT 1 FROM task_edges e
		      JOIN tasks blocker ON e.from_task = blocker.id
		      WHERE e.to_task = t.id
		        AND e.edge_type = 'blocks'
		        AND blocker.status != 'completed'
		  )
		ORDER BY t.id
	`

	rows, err := db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.RiskLevel); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}
