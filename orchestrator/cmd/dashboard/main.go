package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ── DB types ────────────────────────────────────────────────────────────────

type Task struct {
	ID                 int64
	Title              string
	Status             string
	AssignedTo         *string
	RiskLevel          string
	ConsultationStatus *string
}

type TaskEdge struct {
	FromTask int64
	ToTask   int64
}

type Agent struct {
	AgentID       string
	Status        string
	CurrentTaskID *int64
	WorktreePath  *string
	StartedAt     time.Time
	LastHeartbeat time.Time
}

type Message struct {
	AgentID   string
	Channel   string
	MsgType   string
	Content   string
	CreatedAt time.Time
}

type ContextEntry struct {
	AgentID    string
	Domain     string
	KeyName    string
	Value      string
	Confidence float32
}

// ── Log types ───────────────────────────────────────────────────────────────

type LogLine struct {
	Type    string          `json:"type"`
	Subtype string          `json:"subtype,omitempty"`
	Message json.RawMessage `json:"message,omitempty"`

	// result fields
	DurationMs   float64 `json:"duration_ms,omitempty"`
	TotalCostUSD float64 `json:"total_cost_usd,omitempty"`
	NumTurns     int     `json:"num_turns,omitempty"`
}

type AssistantMessage struct {
	Content []ContentBlock `json:"content"`
}

type ContentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

type ToolResultMessage struct {
	Content interface{} `json:"content"`
}

// ── Status icons ────────────────────────────────────────────────────────────

func statusIcon(s string) string {
	switch s {
	case "completed":
		return "✓"
	case "in_progress":
		return "⏳"
	case "failed":
		return "✗"
	case "blocked":
		return "⊘"
	default:
		return "○"
	}
}

// ── DAG rendering ───────────────────────────────────────────────────────────

func renderDAG(tasks []Task, edges []TaskEdge) {
	fmt.Println("\n─── DAG ───────────────────────────────────────")

	taskMap := make(map[int64]Task)
	for _, t := range tasks {
		taskMap[t.ID] = t
	}

	// Build children map (from_task blocks to_task, so from_task -> to_task is the edge direction)
	children := make(map[int64][]int64)
	hasParent := make(map[int64]bool)
	for _, e := range edges {
		children[e.FromTask] = append(children[e.FromTask], e.ToTask)
		hasParent[e.ToTask] = true
	}

	// Find roots (tasks with no incoming edges)
	var roots []int64
	for _, t := range tasks {
		if !hasParent[t.ID] {
			roots = append(roots, t.ID)
		}
	}

	// If no edges, just list all tasks
	if len(edges) == 0 {
		for _, t := range tasks {
			fmt.Printf("  [%s] #%d %s\n", statusIcon(t.Status), t.ID, t.Title)
		}
		return
	}

	var printTree func(id int64, prefix string, isLast bool)
	printTree = func(id int64, prefix string, isLast bool) {
		t, ok := taskMap[id]
		if !ok {
			return
		}

		connector := "└──▶ "
		if !isLast {
			connector = "├──▶ "
		}
		if prefix == "" {
			fmt.Printf("  [%s] #%d %s\n", statusIcon(t.Status), t.ID, t.Title)
		} else {
			fmt.Printf("  %s%s[%s] #%d %s\n", prefix, connector, statusIcon(t.Status), t.ID, t.Title)
		}

		kids := children[id]
		childPrefix := prefix
		if prefix != "" {
			if isLast {
				childPrefix += "     "
			} else {
				childPrefix += "│    "
			}
		}

		for i, kid := range kids {
			printTree(kid, childPrefix, i == len(kids)-1)
		}
	}

	for _, root := range roots {
		printTree(root, "", true)
	}

	if len(tasks) == 0 {
		fmt.Println("  (no tasks)")
	}
}

// ── Agents rendering ────────────────────────────────────────────────────────

func renderAgents(agents []Agent, taskMap map[int64]Task) {
	fmt.Println("\n─── AGENTS ────────────────────────────────────")

	if len(agents) == 0 {
		fmt.Println("  (no agents)")
		return
	}

	for _, a := range agents {
		shortID := a.AgentID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}

		taskInfo := "—"
		if a.CurrentTaskID != nil {
			if t, ok := taskMap[*a.CurrentTaskID]; ok {
				title := t.Title
				if len(title) > 35 {
					title = title[:32] + "..."
				}
				taskInfo = fmt.Sprintf("task #%d  %q", *a.CurrentTaskID, title)
			} else {
				taskInfo = fmt.Sprintf("task #%d", *a.CurrentTaskID)
			}
		}

		dur := "—"
		if a.Status == "working" || a.Status == "starting" {
			elapsed := time.Since(a.StartedAt)
			if elapsed < time.Minute {
				dur = fmt.Sprintf("%.0fs", elapsed.Seconds())
			} else {
				dur = fmt.Sprintf("%.1fm", elapsed.Minutes())
			}
		}

		fmt.Printf("  %-10s %-9s %-45s %s\n", shortID, a.Status, taskInfo, dur)
	}
}

// ── Messages rendering ──────────────────────────────────────────────────────

func renderMessages(messages []Message) {
	fmt.Println("\n─── MESSAGES (recent) ─────────────────────────")

	if len(messages) == 0 {
		fmt.Println("  (no messages)")
		return
	}

	for _, m := range messages {
		shortID := m.AgentID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		content := m.Content
		if len(content) > 80 {
			content = content[:77] + "..."
		}
		fmt.Printf("  [%s] %s: %s\n", shortID, m.MsgType, content)
	}
}

// ── Context rendering ───────────────────────────────────────────────────────

func renderContext(entries []ContextEntry) {
	fmt.Println("\n─── CONTEXT ───────────────────────────────────")

	if len(entries) == 0 {
		fmt.Println("  (no context)")
		return
	}

	for _, c := range entries {
		shortID := c.AgentID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		value := c.Value
		if len(value) > 50 {
			value = value[:47] + "..."
		}
		fmt.Printf("  %s/%s  →  %s  (by %s, confidence: %.1f)\n",
			c.Domain, c.KeyName, value, shortID, c.Confidence)
	}
}

// ── Agent conversation rendering ────────────────────────────────────────────

func renderConversation(agentID string, logPath string) {
	shortID := agentID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	fmt.Printf("\n─── AGENT CONVERSATION: %s ──────────\n", shortID)

	data, err := os.ReadFile(logPath)
	if err != nil {
		fmt.Printf("  (cannot read log: %v)\n", err)
		return
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		var ll LogLine
		if err := json.Unmarshal([]byte(line), &ll); err != nil {
			continue
		}

		switch ll.Type {
		case "assistant":
			if ll.Message == nil {
				continue
			}
			var msg AssistantMessage
			if err := json.Unmarshal(ll.Message, &msg); err != nil {
				continue
			}
			for _, block := range msg.Content {
				switch block.Type {
				case "text":
					text := block.Text
					if len(text) > 100 {
						text = text[:97] + "..."
					}
					if text != "" {
						fmt.Printf("  🤖 %s\n", text)
					}
				case "tool_use":
					input := summarizeInput(block.Input)
					if input != "" {
						fmt.Printf("  🔧 %s → %s\n", block.Name, input)
					} else {
						fmt.Printf("  🔧 %s\n", block.Name)
					}
				}
			}

		case "user":
			if ll.Message == nil {
				continue
			}
			// Tool results - show brief summary
			var raw map[string]interface{}
			if err := json.Unmarshal(ll.Message, &raw); err != nil {
				continue
			}
			contentRaw, ok := raw["content"]
			if !ok {
				continue
			}
			// Could be string or array
			switch v := contentRaw.(type) {
			case string:
				result := v
				if len(result) > 80 {
					result = result[:77] + "..."
				}
				fmt.Printf("  📎 %s\n", result)
			case []interface{}:
				for _, item := range v {
					if m, ok := item.(map[string]interface{}); ok {
						if text, ok := m["text"].(string); ok {
							if len(text) > 80 {
								text = text[:77] + "..."
							}
							fmt.Printf("  📎 %s\n", text)
						}
						if content, ok := m["content"].(string); ok {
							if len(content) > 80 {
								content = content[:77] + "..."
							}
							fmt.Printf("  📎 %s\n", content)
						}
					}
				}
			}

		case "result":
			dur := ll.DurationMs / 1000.0
			fmt.Printf("  ⏱  %.1fs | $%.2f | %d turns\n", dur, ll.TotalCostUSD, ll.NumTurns)
		}
	}
}

func summarizeInput(raw json.RawMessage) string {
	if raw == nil {
		return ""
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}

	// Show key fields depending on tool
	if cmd, ok := m["command"].(string); ok {
		if len(cmd) > 60 {
			cmd = cmd[:57] + "..."
		}
		return cmd
	}
	if fp, ok := m["file_path"].(string); ok {
		return filepath.Base(fp)
	}
	if q, ok := m["query"].(string); ok {
		if len(q) > 60 {
			q = q[:57] + "..."
		}
		return q
	}
	if p, ok := m["pattern"].(string); ok {
		return p
	}
	// For MCP tools, show first string value
	for _, v := range m {
		if s, ok := v.(string); ok {
			if len(s) > 60 {
				s = s[:57] + "..."
			}
			return s
		}
	}
	return ""
}

// ── Main ────────────────────────────────────────────────────────────────────

func main() {
	dbFlag := flag.String("db", "postgres://localhost:5432/architect?sslmode=disable", "Postgres connection string")
	projectFlag := flag.String("project", ".", "Project directory (to find .worktrees/)")
	agentFlag := flag.String("agent", "", "Show full conversation for specific agent ID (prefix match)")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, *dbFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to database: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error pinging database: %v\n", err)
		os.Exit(1)
	}

	// ── Header ──────────────────────────────────────────────────────────
	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Println("║          ARCHITECT DASHBOARD             ║")
	fmt.Println("╚══════════════════════════════════════════╝")

	// ── Query all data ──────────────────────────────────────────────────

	// Tasks
	tasks, err := queryTasks(ctx, pool)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error querying tasks: %v\n", err)
		os.Exit(1)
	}

	taskMap := make(map[int64]Task)
	for _, t := range tasks {
		taskMap[t.ID] = t
	}

	// Edges
	edges, err := queryEdges(ctx, pool)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error querying edges: %v\n", err)
		os.Exit(1)
	}

	// Agents
	agents, err := queryAgents(ctx, pool)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error querying agents: %v\n", err)
		os.Exit(1)
	}

	// Messages
	messages, err := queryMessages(ctx, pool)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error querying messages: %v\n", err)
		os.Exit(1)
	}

	// Context
	ctxEntries, err := queryContext(ctx, pool)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error querying context: %v\n", err)
		os.Exit(1)
	}

	// ── Render sections ─────────────────────────────────────────────────
	renderDAG(tasks, edges)
	renderAgents(agents, taskMap)
	renderMessages(messages)
	renderContext(ctxEntries)

	// ── Agent conversations ─────────────────────────────────────────────
	worktreeBase := filepath.Join(*projectFlag, ".worktrees")

	if *agentFlag != "" {
		// Show specific agent conversation
		logPath := findAgentLog(worktreeBase, *agentFlag)
		if logPath == "" {
			fmt.Fprintf(os.Stderr, "\nNo log found for agent %q in %s\n", *agentFlag, worktreeBase)
			os.Exit(1)
		}
		renderConversation(*agentFlag, logPath)
	} else {
		// Show summary for all agents
		for _, a := range agents {
			logPath := findAgentLog(worktreeBase, a.AgentID)
			if logPath != "" {
				renderConversation(a.AgentID, logPath)
			}
		}
	}

	fmt.Println()
}

// ── DB queries ──────────────────────────────────────────────────────────────

func queryTasks(ctx context.Context, pool *pgxpool.Pool) ([]Task, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, title, status, assigned_to, risk_level, consultation_status
		 FROM tasks ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.Title, &t.Status, &t.AssignedTo, &t.RiskLevel, &t.ConsultationStatus); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func queryEdges(ctx context.Context, pool *pgxpool.Pool) ([]TaskEdge, error) {
	rows, err := pool.Query(ctx, `SELECT from_task, to_task FROM task_edges`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var edges []TaskEdge
	for rows.Next() {
		var e TaskEdge
		if err := rows.Scan(&e.FromTask, &e.ToTask); err != nil {
			return nil, err
		}
		edges = append(edges, e)
	}
	return edges, rows.Err()
}

func queryAgents(ctx context.Context, pool *pgxpool.Pool) ([]Agent, error) {
	rows, err := pool.Query(ctx,
		`SELECT agent_id, status, current_task_id, worktree_path, started_at, last_heartbeat
		 FROM agents ORDER BY started_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []Agent
	for rows.Next() {
		var a Agent
		if err := rows.Scan(&a.AgentID, &a.Status, &a.CurrentTaskID, &a.WorktreePath, &a.StartedAt, &a.LastHeartbeat); err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

func queryMessages(ctx context.Context, pool *pgxpool.Pool) ([]Message, error) {
	rows, err := pool.Query(ctx,
		`SELECT agent_id, channel, msg_type, content, created_at
		 FROM messages ORDER BY created_at DESC LIMIT 10`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.AgentID, &m.Channel, &m.MsgType, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

func queryContext(ctx context.Context, pool *pgxpool.Pool) ([]ContextEntry, error) {
	rows, err := pool.Query(ctx,
		`SELECT agent_id, domain, key_name, value, confidence
		 FROM context ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []ContextEntry
	for rows.Next() {
		var c ContextEntry
		if err := rows.Scan(&c.AgentID, &c.Domain, &c.KeyName, &c.Value, &c.Confidence); err != nil {
			return nil, err
		}
		entries = append(entries, c)
	}
	return entries, rows.Err()
}

// ── Log file helpers ────────────────────────────────────────────────────────

func findAgentLog(worktreeBase string, agentID string) string {
	// Try exact match first
	logPath := filepath.Join(worktreeBase, "agent-"+agentID, "agent.log")
	if _, err := os.Stat(logPath); err == nil {
		return logPath
	}

	// Try prefix match
	entries, err := os.ReadDir(worktreeBase)
	if err != nil {
		return ""
	}

	shortID := agentID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, "agent-"+shortID) || strings.HasPrefix(name, "agent-"+agentID) {
			candidate := filepath.Join(worktreeBase, name, "agent.log")
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}
	return ""
}
