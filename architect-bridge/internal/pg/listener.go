package pg

import (
	"context"
	"encoding/json"
	"log"

	"architect-bridge/internal/protocol"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var channels = []string{"agent_messages", "context_updates", "task_updates", "agent_updates"}

func StartListener(ctx context.Context, dbURL string, eventCh chan<- protocol.Event) {
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		log.Fatalf("listener: failed to connect: %v", err)
	}
	defer conn.Close(ctx)

	for _, ch := range channels {
		_, err := conn.Exec(ctx, "LISTEN "+ch)
		if err != nil {
			log.Fatalf("listener: failed to listen on %s: %v", ch, err)
		}
	}

	log.Printf("listener: listening on %v", channels)

	for {
		notification, err := conn.WaitForNotification(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("listener: error waiting for notification: %v", err)
			return
		}
		event := notificationToEvent(notification)
		eventCh <- event
	}
}

func notificationToEvent(n *pgconn.Notification) protocol.Event {
	var data interface{}
	if err := json.Unmarshal([]byte(n.Payload), &data); err != nil {
		data = map[string]interface{}{"raw": n.Payload}
	}

	typeMap := map[string]string{
		"agent_messages":  "new_message",
		"context_updates": "context_update",
		"task_updates":    "task_update",
		"agent_updates":   "agent_update",
	}

	eventType := typeMap[n.Channel]
	if eventType == "" {
		eventType = n.Channel
	}

	return protocol.Event{
		Type: eventType,
		Data: data,
	}
}
