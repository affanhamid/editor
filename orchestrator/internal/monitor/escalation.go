package monitor

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

// HandleBlocker processes a blocker message from an agent.
// For now, this logs the blocker. Future: notify the user, escalate, or reassign.
func HandleBlocker(ctx context.Context, pool *pgxpool.Pool, msg MessagePayload) {
	log.Printf("escalation: agent %s reported blocker on channel %s: %s",
		msg.AgentID[:8], msg.Channel, msg.Body)

	// Mark the agent as blocked in Postgres.
	_, err := pool.Exec(ctx,
		`UPDATE agents SET status = 'blocked' WHERE agent_id = $1`,
		msg.AgentID,
	)
	if err != nil {
		log.Printf("error marking agent %s as blocked: %v", msg.AgentID[:8], err)
	}
}
