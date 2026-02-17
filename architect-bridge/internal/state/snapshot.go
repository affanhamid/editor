package state

import (
	"context"
	"time"

	"architect-bridge/internal/protocol"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Agent struct {
	AgentID       string     `json:"agent_id"`
	Status        string     `json:"status"`
	CurrentTaskID *int       `json:"current_task_id"`
	WorktreePath  *string    `json:"worktree_path"`
	LastHeartbeat *time.Time `json:"last_heartbeat"`
}

type Task struct {
	ID         int     `json:"id"`
	Title      string  `json:"title"`
	Status     string  `json:"status"`
	AssignedTo *string `json:"assigned_to"`
	RiskLevel  *string `json:"risk_level"`
	ParentID   *int    `json:"parent_id"`
}

type Message struct {
	ID        int       `json:"id"`
	AgentID   string    `json:"agent_id"`
	Channel   string    `json:"channel"`
	Content   string    `json:"content"`
	MsgType   string    `json:"msg_type"`
	CreatedAt time.Time `json:"created_at"`
}

type Snapshot struct {
	Agents   []Agent   `json:"agents"`
	Tasks    []Task    `json:"tasks"`
	Messages []Message `json:"messages"`
}

func GetSnapshot(ctx context.Context, db *pgxpool.Pool) (*Snapshot, error) {
	snapshot := &Snapshot{}

	// Get all agents
	rows, err := db.Query(ctx, `SELECT agent_id, status, current_task_id, worktree_path, last_heartbeat FROM agents ORDER BY started_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var a Agent
		if err := rows.Scan(&a.AgentID, &a.Status, &a.CurrentTaskID, &a.WorktreePath, &a.LastHeartbeat); err != nil {
			return nil, err
		}
		snapshot.Agents = append(snapshot.Agents, a)
	}

	// Get all tasks
	rows, err = db.Query(ctx, `SELECT id, title, status, assigned_to, risk_level, parent_id FROM tasks ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.Title, &t.Status, &t.AssignedTo, &t.RiskLevel, &t.ParentID); err != nil {
			return nil, err
		}
		snapshot.Tasks = append(snapshot.Tasks, t)
	}

	// Get recent messages
	rows, err = db.Query(ctx, `SELECT id, agent_id, channel, content, msg_type, created_at FROM messages ORDER BY created_at DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.AgentID, &m.Channel, &m.Content, &m.MsgType, &m.CreatedAt); err != nil {
			return nil, err
		}
		snapshot.Messages = append(snapshot.Messages, m)
	}

	return snapshot, nil
}

func SnapshotEvent(s *Snapshot) protocol.Event {
	return protocol.Event{
		Type: "snapshot",
		Data: s,
	}
}
