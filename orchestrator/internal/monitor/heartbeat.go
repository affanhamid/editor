package monitor

import (
	"context"
	"log"
	"time"

	"github.com/affanhamid/editor/orchestrator/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MonitorHeartbeats runs periodically to detect dead agents.
func MonitorHeartbeats(ctx context.Context, pool *pgxpool.Pool, timeout time.Duration) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			dead, err := db.MarkDeadAgents(ctx, pool, timeout.String())
			if err != nil {
				log.Printf("error checking heartbeats: %v", err)
				continue
			}
			for _, d := range dead {
				log.Printf("agent %s detected as dead", d.AgentID[:8])
				if d.TaskID != nil {
					if err := db.ReclaimTask(ctx, pool, *d.TaskID); err != nil {
						log.Printf("error reclaiming task %d: %v", *d.TaskID, err)
					} else {
						log.Printf("reclaimed task %d from dead agent %s", *d.TaskID, d.AgentID[:8])
					}
				}
			}
		}
	}
}
