package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Queries struct {
	Pool *pgxpool.Pool
}

// Message types

type Message struct {
	ID        int64     `json:"id"`
	AgentID   string    `json:"agent_id"`
	Channel   string    `json:"channel"`
	Content   string    `json:"content"`
	MsgType   string    `json:"msg_type"`
	RefTaskID *int64    `json:"ref_task_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type ContextEntry struct {
	ID         int64     `json:"id,omitempty"`
	AgentID    string    `json:"agent_id"`
	Domain     string    `json:"domain"`
	KeyName    string    `json:"key_name"`
	Value      string    `json:"value"`
	Confidence float64   `json:"confidence"`
	SourceFile *string   `json:"source_file,omitempty"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Task struct {
	ID          int64   `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Status      string  `json:"status"`
	AssignedTo  *string `json:"assigned_to,omitempty"`
	RiskLevel   string  `json:"risk_level"`
	Output      *string `json:"output,omitempty"`
	ParentID    *int64  `json:"parent_id,omitempty"`
}

type TaskEdge struct {
	FromTask int64  `json:"from_task"`
	ToTask   int64  `json:"to_task"`
	EdgeType string `json:"edge_type"`
}

type TasksResult struct {
	Tasks []Task     `json:"tasks"`
	Edges []TaskEdge `json:"edges"`
}

type Decision struct {
	ID        int64     `json:"id"`
	AgentID   string    `json:"agent_id"`
	Decision  string    `json:"decision"`
	Rationale string    `json:"rationale"`
	RiskLevel string    `json:"risk_level"`
	CreatedAt time.Time `json:"created_at"`
}

type Agent struct {
	AgentID       string    `json:"agent_id"`
	Status        string    `json:"status"`
	CurrentTaskID *int64    `json:"current_task_id,omitempty"`
	WorktreePath  *string   `json:"worktree_path,omitempty"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
}

// PostMessage inserts a message and returns its ID and timestamp.
func (q *Queries) PostMessage(ctx context.Context, agentID, channel, content, msgType string, refTaskID *int64) (*Message, error) {
	var msg Message
	err := q.Pool.QueryRow(ctx,
		`INSERT INTO messages (agent_id, channel, content, msg_type, ref_task_id)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, created_at`,
		agentID, channel, content, msgType, refTaskID,
	).Scan(&msg.ID, &msg.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("post_message: %w", err)
	}
	msg.AgentID = agentID
	msg.Channel = channel
	msg.Content = content
	msg.MsgType = msgType
	msg.RefTaskID = refTaskID
	return &msg, nil
}

// ReadMessages returns recent messages with optional channel and since filters.
func (q *Queries) ReadMessages(ctx context.Context, channel *string, since *time.Time, limit int) ([]Message, error) {
	rows, err := q.Pool.Query(ctx,
		`SELECT id, agent_id, channel, content, msg_type, ref_task_id, created_at
		 FROM messages
		 WHERE ($1::text IS NULL OR channel = $1)
		   AND ($2::timestamptz IS NULL OR created_at > $2)
		 ORDER BY created_at DESC
		 LIMIT $3`,
		channel, since, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("read_messages: %w", err)
	}
	defer rows.Close()

	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Message, error) {
		var m Message
		err := row.Scan(&m.ID, &m.AgentID, &m.Channel, &m.Content, &m.MsgType, &m.RefTaskID, &m.CreatedAt)
		return m, err
	})
}

// ReadContext returns context entries with optional domain filter.
func (q *Queries) ReadContext(ctx context.Context, domain *string) ([]ContextEntry, error) {
	rows, err := q.Pool.Query(ctx,
		`SELECT domain, key_name, value, confidence, agent_id, source_file, updated_at
		 FROM context
		 WHERE ($1::text IS NULL OR domain = $1)
		 ORDER BY domain, key_name`,
		domain,
	)
	if err != nil {
		return nil, fmt.Errorf("read_context: %w", err)
	}
	defer rows.Close()

	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (ContextEntry, error) {
		var c ContextEntry
		err := row.Scan(&c.Domain, &c.KeyName, &c.Value, &c.Confidence, &c.AgentID, &c.SourceFile, &c.UpdatedAt)
		return c, err
	})
}

// WriteContext upserts a context entry.
func (q *Queries) WriteContext(ctx context.Context, agentID, domain, key, value string, confidence float64, sourceFile *string) (int64, error) {
	var id int64
	err := q.Pool.QueryRow(ctx,
		`INSERT INTO context (agent_id, domain, key_name, value, confidence, source_file)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (domain, key_name) DO UPDATE
		   SET value = EXCLUDED.value,
		       confidence = EXCLUDED.confidence,
		       agent_id = EXCLUDED.agent_id,
		       source_file = EXCLUDED.source_file,
		       updated_at = NOW()
		 RETURNING id`,
		agentID, domain, key, value, confidence, sourceFile,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("write_context: %w", err)
	}
	return id, nil
}

// GetTasks returns tasks with optional status and assigned_to filters, plus edges.
func (q *Queries) GetTasks(ctx context.Context, status, assignedTo *string) (*TasksResult, error) {
	rows, err := q.Pool.Query(ctx,
		`SELECT id, title, description, status, assigned_to, risk_level, output, parent_id
		 FROM tasks
		 WHERE ($1::text IS NULL OR status = $1)
		   AND ($2::text IS NULL OR assigned_to = $2)
		 ORDER BY id`,
		status, assignedTo,
	)
	if err != nil {
		return nil, fmt.Errorf("get_tasks: %w", err)
	}
	defer rows.Close()

	tasks, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Task, error) {
		var t Task
		err := row.Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.AssignedTo, &t.RiskLevel, &t.Output, &t.ParentID)
		return t, err
	})
	if err != nil {
		return nil, fmt.Errorf("get_tasks scan: %w", err)
	}

	if len(tasks) == 0 {
		return &TasksResult{Tasks: tasks, Edges: nil}, nil
	}

	// Collect task IDs for edge query
	taskIDs := make([]int64, len(tasks))
	for i, t := range tasks {
		taskIDs[i] = t.ID
	}

	edgeRows, err := q.Pool.Query(ctx,
		`SELECT from_task, to_task, edge_type
		 FROM task_edges
		 WHERE from_task = ANY($1) OR to_task = ANY($1)`,
		taskIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("get_tasks edges: %w", err)
	}
	defer edgeRows.Close()

	edges, err := pgx.CollectRows(edgeRows, func(row pgx.CollectableRow) (TaskEdge, error) {
		var e TaskEdge
		err := row.Scan(&e.FromTask, &e.ToTask, &e.EdgeType)
		return e, err
	})
	if err != nil {
		return nil, fmt.Errorf("get_tasks edges scan: %w", err)
	}

	return &TasksResult{Tasks: tasks, Edges: edges}, nil
}

// ClaimTask attempts to claim an unassigned task. Returns task ID and title, or error if already claimed.
func (q *Queries) ClaimTask(ctx context.Context, agentID string, taskID int64) (*Task, error) {
	var t Task
	err := q.Pool.QueryRow(ctx,
		`UPDATE tasks
		 SET assigned_to = $1, status = 'in_progress', updated_at = NOW()
		 WHERE id = $2 AND assigned_to IS NULL
		 RETURNING id, title`,
		agentID, taskID,
	).Scan(&t.ID, &t.Title)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("task %d is already claimed or does not exist", taskID)
	}
	if err != nil {
		return nil, fmt.Errorf("claim_task: %w", err)
	}
	return &t, nil
}

// UpdateTask updates the status and output of a task owned by the given agent.
func (q *Queries) UpdateTask(ctx context.Context, agentID string, taskID int64, status string, output *string) error {
	var id int64
	err := q.Pool.QueryRow(ctx,
		`UPDATE tasks
		 SET status = $1, output = $2, updated_at = NOW()
		 WHERE id = $3 AND assigned_to = $4
		 RETURNING id`,
		status, output, taskID, agentID,
	).Scan(&id)
	if err == pgx.ErrNoRows {
		return fmt.Errorf("task %d is not assigned to you or does not exist", taskID)
	}
	if err != nil {
		return fmt.Errorf("update_task: %w", err)
	}
	return nil
}

// WriteDecision records an architectural decision.
func (q *Queries) WriteDecision(ctx context.Context, agentID, branch, domain, decision, rationale string, alternatives *string, riskLevel string) (int64, error) {
	var id int64
	err := q.Pool.QueryRow(ctx,
		`INSERT INTO decisions (agent_id, branch, domain, decision, rationale, alternatives_considered, risk_level)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id`,
		agentID, branch, domain, decision, rationale, alternatives, riskLevel,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("write_decision: %w", err)
	}
	return id, nil
}

// CheckDecisions returns existing decisions for a domain.
func (q *Queries) CheckDecisions(ctx context.Context, domain string) ([]Decision, error) {
	rows, err := q.Pool.Query(ctx,
		`SELECT id, agent_id, decision, rationale, risk_level, created_at
		 FROM decisions
		 WHERE domain = $1
		 ORDER BY created_at DESC`,
		domain,
	)
	if err != nil {
		return nil, fmt.Errorf("check_decisions: %w", err)
	}
	defer rows.Close()

	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Decision, error) {
		var d Decision
		err := row.Scan(&d.ID, &d.AgentID, &d.Decision, &d.Rationale, &d.RiskLevel, &d.CreatedAt)
		return d, err
	})
}

// Heartbeat updates the agent's last heartbeat timestamp.
func (q *Queries) Heartbeat(ctx context.Context, agentID string) error {
	_, err := q.Pool.Exec(ctx,
		`UPDATE agents SET last_heartbeat = NOW() WHERE agent_id = $1`,
		agentID,
	)
	if err != nil {
		return fmt.Errorf("heartbeat: %w", err)
	}
	return nil
}

// GetAgents returns active agents with optional status filter.
func (q *Queries) GetAgents(ctx context.Context, status *string) ([]Agent, error) {
	rows, err := q.Pool.Query(ctx,
		`SELECT agent_id, status, current_task_id, worktree_path, last_heartbeat
		 FROM agents
		 WHERE ($1::text IS NULL OR status = $1)
		 ORDER BY started_at`,
		status,
	)
	if err != nil {
		return nil, fmt.Errorf("get_agents: %w", err)
	}
	defer rows.Close()

	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Agent, error) {
		var a Agent
		err := row.Scan(&a.AgentID, &a.Status, &a.CurrentTaskID, &a.WorktreePath, &a.LastHeartbeat)
		return a, err
	})
}

// ToJSON is a helper to marshal any value to a JSON string.
func ToJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"error": "failed to marshal: %s"}`, err)
	}
	return string(b)
}
