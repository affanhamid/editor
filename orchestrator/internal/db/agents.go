package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RegisterAgent inserts a new agent record into Postgres.
func RegisterAgent(ctx context.Context, pool *pgxpool.Pool, agentID string, taskID int64, worktreePath string) error {
	_, err := pool.Exec(ctx,
		`INSERT INTO agents (agent_id, pid, status, current_task_id, worktree_path)
		 VALUES ($1, 0, 'starting', $2, $3)`,
		agentID, taskID, worktreePath,
	)
	return err
}

// UpdateAgentPID sets the PID and marks the agent as working.
func UpdateAgentPID(ctx context.Context, pool *pgxpool.Pool, agentID string, pid int) error {
	_, err := pool.Exec(ctx,
		`UPDATE agents SET pid = $1, status = 'working' WHERE agent_id = $2`,
		pid, agentID,
	)
	return err
}

// UpdateAgentStatus sets the agent's status.
func UpdateAgentStatus(ctx context.Context, pool *pgxpool.Pool, agentID string, status string) error {
	_, err := pool.Exec(ctx,
		`UPDATE agents SET status = $1 WHERE agent_id = $2`,
		status, agentID,
	)
	return err
}

// DeadAgent holds info about an agent detected as dead.
type DeadAgent struct {
	AgentID string
	TaskID  *int64
}

// MarkDeadAgents finds agents with stale heartbeats and marks them dead,
// returning their info so tasks can be reclaimed.
func MarkDeadAgents(ctx context.Context, pool *pgxpool.Pool, timeoutInterval string) ([]DeadAgent, error) {
	rows, err := pool.Query(ctx, `
		UPDATE agents SET status = 'dead'
		WHERE status IN ('working', 'blocked')
		  AND last_heartbeat < NOW() - $1::interval
		RETURNING agent_id, current_task_id`, timeoutInterval)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dead []DeadAgent
	for rows.Next() {
		var d DeadAgent
		if err := rows.Scan(&d.AgentID, &d.TaskID); err != nil {
			return nil, err
		}
		dead = append(dead, d)
	}
	return dead, rows.Err()
}
