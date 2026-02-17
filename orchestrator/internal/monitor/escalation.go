package monitor

import (
	"context"
	"fmt"
	"log"

	"github.com/affanhamid/editor/orchestrator/internal/db"
	"github.com/affanhamid/editor/orchestrator/internal/spawn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// HandleBlocker processes a blocker message from an agent.
// Fetches the full message content from the database, then responds via the agent's stdin.
func HandleBlocker(ctx context.Context, pool *pgxpool.Pool, registry *spawn.AgentRegistry, msg MessagePayload) {
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

	// Send acknowledgement back via stdin.
	response := fmt.Sprintf("The orchestrator received your blocker: %q. "+
		"Please continue with what you can and skip the blocked part for now.", content)
	if err := registry.Send(msg.AgentID, response); err != nil {
		log.Printf("error sending response to agent %s: %v", msg.AgentID[:8], err)
	}
}
