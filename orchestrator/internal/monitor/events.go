package monitor

import (
	"context"
	"encoding/json"
	"log"

	"github.com/affanhamid/editor/orchestrator/internal/dag"
	"github.com/affanhamid/editor/orchestrator/internal/db"
	"github.com/affanhamid/editor/orchestrator/internal/spawn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TaskUpdatePayload is the JSON payload from task_updates notifications.
type TaskUpdatePayload struct {
	ID     int64  `json:"id"`
	Status string `json:"status"`
}

// MessagePayload is the JSON payload from agent_messages notifications.
type MessagePayload struct {
	ID      int64  `json:"id"`
	AgentID string `json:"agent_id"`
	Channel string `json:"channel"`
	MsgType string `json:"msg_type"`
}

// HandleEvents is the main event processing loop.
func HandleEvents(ctx context.Context, pool *pgxpool.Pool, eventCh <-chan db.Event, projectDir string, config spawn.Config) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-eventCh:
			if !ok {
				return
			}
			switch event.Channel {
			case "task_updates":
				var payload TaskUpdatePayload
				if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
					log.Printf("error parsing task_updates payload: %v", err)
					continue
				}
				if payload.Status == "completed" {
					log.Printf("task %d completed", payload.ID)
					ready, err := dag.ReadyTasks(ctx, pool)
					if err != nil {
						log.Printf("error finding ready tasks: %v", err)
						continue
					}
					for _, task := range ready {
						if err := spawn.SpawnSession(ctx, pool, task, projectDir, config); err != nil {
							log.Printf("error spawning session for task %d: %v", task.ID, err)
						}
					}
				}

			case "agent_messages":
				var payload MessagePayload
				if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
					log.Printf("error parsing agent_messages payload: %v", err)
					continue
				}
				if payload.MsgType == "blocker" {
					log.Printf("BLOCKER from agent %s (message %d)", payload.AgentID[:8], payload.ID)
					HandleBlocker(ctx, pool, payload)
				}

			case "agent_updates":
				log.Printf("agent update: %s", event.Payload)
			}
		}
	}
}
