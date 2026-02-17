package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Message represents a message from an agent.
type Message struct {
	ID      int64
	AgentID string
	Channel string
	MsgType string
	Content string
}

// RecentMessages fetches the most recent messages from a channel.
func RecentMessages(ctx context.Context, pool *pgxpool.Pool, channel string, limit int) ([]Message, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, agent_id, channel, msg_type, content
		FROM messages
		WHERE channel = $1
		ORDER BY created_at DESC
		LIMIT $2`, channel, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.AgentID, &m.Channel, &m.MsgType, &m.Content); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// GetMessageContent fetches the content of a single message by ID.
func GetMessageContent(ctx context.Context, pool *pgxpool.Pool, messageID int64) (string, error) {
	var content string
	err := pool.QueryRow(ctx, `SELECT content FROM messages WHERE id = $1`, messageID).Scan(&content)
	return content, err
}
