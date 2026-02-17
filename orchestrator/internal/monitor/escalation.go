package monitor

import (
	"context"
	"log"

	"github.com/affanhamid/editor/orchestrator/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// HandleBlocker processes a blocker message from an agent.
// Fetches the full message content from the database since NOTIFY only includes the ID.
func HandleBlocker(ctx context.Context, pool *pgxpool.Pool, msg MessagePayload) {
	// Query the message content from the database.
	content, err := db.GetMessageContent(ctx, pool, msg.ID)
	if err != nil {
		log.Printf("error fetching blocker message %d: %v", msg.ID, err)
		content = "(could not fetch content)"
	}

	log.Printf("escalation: agent %s reported blocker on channel %s: %s",
		msg.AgentID[:8], msg.Channel, content)

	// Mark the agent as blocked in Postgres.
	_, err = pool.Exec(ctx,
		`UPDATE agents SET status = 'blocked' WHERE agent_id = $1`,
		msg.AgentID,
	)
	if err != nil {
		log.Printf("error marking agent %s as blocked: %v", msg.AgentID[:8], err)
	}
}
