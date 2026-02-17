package pg

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"architect-bridge/internal/protocol"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var channels = []string{"agent_messages", "context_updates", "task_updates", "agent_updates"}

func StartListener(ctx context.Context, dbURL string, eventCh chan<- protocol.Event) {
	for {
		if ctx.Err() != nil {
			return
		}
		err := listenLoop(ctx, dbURL, eventCh)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("listener: connection lost: %v, reconnecting in 2s...", err)
			time.Sleep(2 * time.Second)
		}
	}
}

func listenLoop(ctx context.Context, dbURL string, eventCh chan<- protocol.Event) error {
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close(ctx)

	for _, ch := range channels {
		_, err := conn.Exec(ctx, "LISTEN "+ch)
		if err != nil {
			return fmt.Errorf("failed to listen on %s: %w", ch, err)
		}
	}

	log.Printf("listener: listening on %v", channels)

	for {
		notification, err := conn.WaitForNotification(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("notification error: %w", err)
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
