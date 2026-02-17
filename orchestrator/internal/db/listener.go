package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Event represents a LISTEN/NOTIFY event from Postgres.
type Event struct {
	Channel string
	Payload string
}

// Channels we listen on.
var ListenChannels = []string{
	"agent_messages",
	"context_updates",
	"task_updates",
	"agent_updates",
}

// StartListener opens a persistent connection and listens on all channels.
// Events are sent to eventCh. Blocks until context is cancelled or an error occurs.
func StartListener(ctx context.Context, connStr string, eventCh chan<- Event) error {
	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		return fmt.Errorf("listener connect: %w", err)
	}
	defer conn.Close(ctx)

	for _, ch := range ListenChannels {
		if _, err := conn.Exec(ctx, "LISTEN "+ch); err != nil {
			return fmt.Errorf("listen %s: %w", ch, err)
		}
	}

	for {
		notification, err := conn.WaitForNotification(ctx)
		if err != nil {
			return fmt.Errorf("wait for notification: %w", err)
		}
		eventCh <- Event{
			Channel: notification.Channel,
			Payload: notification.Payload,
		}
	}
}
