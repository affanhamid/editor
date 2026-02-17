package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/term"
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


// ── View state ──────────────────────────────────────────────────────────────

const (
	viewMain  = 0
	viewAgent = 1
)

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

// ── Buffered rendering helpers ──────────────────────────────────────────────

func bprintf(buf *bytes.Buffer, format string, args ...any) {
	fmt.Fprintf(buf, format, args...)
}

func bprintln(buf *bytes.Buffer, s string) {
	buf.WriteString(s)
	buf.WriteByte('\n')
}

// ── DAG rendering ───────────────────────────────────────────────────────────

func renderDAG(buf *bytes.Buffer, tasks []Task, edges []TaskEdge) {
	bprintln(buf, "\n─── DAG ───────────────────────────────────────")

	taskMap := make(map[int64]Task)
	for _, t := range tasks {
		taskMap[t.ID] = t
	}

	children := make(map[int64][]int64)
	hasParent := make(map[int64]bool)
	for _, e := range edges {
		children[e.FromTask] = append(children[e.FromTask], e.ToTask)
		hasParent[e.ToTask] = true
	}

	var roots []int64
	for _, t := range tasks {
		if !hasParent[t.ID] {
			roots = append(roots, t.ID)
		}
	}

	if len(edges) == 0 {
		for _, t := range tasks {
			bprintf(buf, "  [%s] #%d %s\n", statusIcon(t.Status), t.ID, t.Title)
		}
		if len(tasks) == 0 {
			bprintln(buf, "  (no tasks)")
		}
		return
	}

	visited := make(map[int64]bool)

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
			bprintf(buf, "  [%s] #%d %s\n", statusIcon(t.Status), t.ID, t.Title)
		} else {
			bprintf(buf, "  %s%s[%s] #%d %s\n", prefix, connector, statusIcon(t.Status), t.ID, t.Title)
		}

		if visited[id] {
			return
		}
		visited[id] = true

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
		bprintln(buf, "  (no tasks)")
	}
}

// ── Agents rendering ────────────────────────────────────────────────────────

func renderAgents(buf *bytes.Buffer, agents []Agent, taskMap map[int64]Task) {
	bprintln(buf, "\n─── AGENTS ────────────────────────────────────")

	if len(agents) == 0 {
		bprintln(buf, "  (no agents)")
		return
	}

	for i, a := range agents {
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

		num := i + 1
		if num <= 9 {
			bprintf(buf, "  [%d] %-10s %-9s %-45s %s\n", num, shortID, a.Status, taskInfo, dur)
		} else {
			bprintf(buf, "      %-10s %-9s %-45s %s\n", shortID, a.Status, taskInfo, dur)
		}
	}
}

// ── Context rendering ───────────────────────────────────────────────────────

func renderContext(buf *bytes.Buffer, entries []ContextEntry) {
	bprintln(buf, "\n─── CONTEXT ───────────────────────────────────")

	if len(entries) == 0 {
		bprintln(buf, "  (no context)")
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
		bprintf(buf, "  %s/%s  →  %s  (by %s, confidence: %.1f)\n",
			c.Domain, c.KeyName, value, shortID, c.Confidence)
	}
}

// ── Agent conversation rendering ────────────────────────────────────────────

func renderConversation(buf *bytes.Buffer, agent Agent, taskMap map[int64]Task, logPath string) {
	shortID := agent.AgentID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}

	taskInfo := ""
	if agent.CurrentTaskID != nil {
		if t, ok := taskMap[*agent.CurrentTaskID]; ok {
			taskInfo = fmt.Sprintf(" | task #%d %q", *agent.CurrentTaskID, t.Title)
		} else {
			taskInfo = fmt.Sprintf(" | task #%d", *agent.CurrentTaskID)
		}
	}

	bprintf(buf, "\n─── AGENT %s%s | %s ───\n", shortID, taskInfo, agent.Status)

	if logPath == "" {
		bprintln(buf, "  (no log file found)")
		bprintln(buf, "\nPress [b] back, [q] quit")
		return
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		bprintf(buf, "  (cannot read log: %v)\n", err)
		bprintln(buf, "\nPress [b] back, [q] quit")
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
						bprintf(buf, "  🤖 %s\n", text)
					}
				case "tool_use":
					input := summarizeInput(block.Input)
					if input != "" {
						bprintf(buf, "  🔧 %s → %s\n", block.Name, input)
					} else {
						bprintf(buf, "  🔧 %s\n", block.Name)
					}
				}
			}

		case "user":
			if ll.Message == nil {
				continue
			}
			var raw map[string]any
			if err := json.Unmarshal(ll.Message, &raw); err != nil {
				continue
			}
			contentRaw, ok := raw["content"]
			if !ok {
				continue
			}
			switch v := contentRaw.(type) {
			case string:
				result := v
				if len(result) > 80 {
					result = result[:77] + "..."
				}
				bprintf(buf, "  📎 %s\n", result)
			case []any:
				for _, item := range v {
					if m, ok := item.(map[string]any); ok {
						if text, ok := m["text"].(string); ok {
							if len(text) > 80 {
								text = text[:77] + "..."
							}
							bprintf(buf, "  📎 %s\n", text)
						}
						if content, ok := m["content"].(string); ok {
							if len(content) > 80 {
								content = content[:77] + "..."
							}
							bprintf(buf, "  📎 %s\n", content)
						}
					}
				}
			}

		case "result":
			dur := ll.DurationMs / 1000.0
			bprintf(buf, "  ⏱  %.1fs | $%.2f | %d turns\n", dur, ll.TotalCostUSD, ll.NumTurns)
		}
	}

	bprintln(buf, "\nPress [b] back, [q] quit")
}

func summarizeInput(raw json.RawMessage) string {
	if raw == nil {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}

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
	dbFlag := flag.String("db", "postgres://localhost:5432/architect_meta?sslmode=disable", "Postgres connection string")
	projectFlag := flag.String("project", ".", "Project directory (to find .worktrees/)")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

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

	// Enter raw terminal mode
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error entering raw mode: %v\n", err)
		os.Exit(1)
	}
	defer term.Restore(fd, oldState)

	// Read keypresses in a goroutine
	keys := make(chan byte, 16)
	go func() {
		buf := make([]byte, 1)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil || n == 0 {
				return
			}
			keys <- buf[0]
		}
	}()

	currentView := viewMain
	selectedAgent := 0
	var agents []Agent // cache for agent selection

	// Initial render
	agents = renderScreen(ctx, pool, *projectFlag, currentView, selectedAgent, agents)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case key := <-keys:
			switch {
			case key == 'q' || key == 3: // q or Ctrl-C
				return
			case key == 'b' && currentView == viewAgent:
				currentView = viewMain
				selectedAgent = 0
				agents = renderScreen(ctx, pool, *projectFlag, currentView, selectedAgent, agents)
			case key >= '1' && key <= '9' && currentView == viewMain:
				idx := int(key - '1')
				if idx < len(agents) {
					selectedAgent = idx
					currentView = viewAgent
					agents = renderScreen(ctx, pool, *projectFlag, currentView, selectedAgent, agents)
				}
			}
		case <-ticker.C:
			agents = renderScreen(ctx, pool, *projectFlag, currentView, selectedAgent, agents)
		}
	}
}

func renderScreen(ctx context.Context, pool *pgxpool.Pool, projectDir string, currentView int, selectedAgent int, prevAgents []Agent) []Agent {
	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var buf bytes.Buffer
	buf.WriteString("\033[2J\033[H")

	flush := func(agents []Agent) []Agent {
		out := bytes.ReplaceAll(buf.Bytes(), []byte("\n"), []byte("\r\n"))
		os.Stdout.Write(out)
		return agents
	}

	tasks, err := queryTasks(queryCtx, pool)
	if err != nil {
		bprintf(&buf, "error querying tasks: %v\n", err)
		return flush(prevAgents)
	}

	taskMap := make(map[int64]Task)
	for _, t := range tasks {
		taskMap[t.ID] = t
	}

	edges, err := queryEdges(queryCtx, pool)
	if err != nil {
		bprintf(&buf, "error querying edges: %v\n", err)
		return flush(prevAgents)
	}

	agents, err := queryAgents(queryCtx, pool)
	if err != nil {
		bprintf(&buf, "error querying agents: %v\n", err)
		return flush(prevAgents)
	}

	ctxEntries, err := queryContext(queryCtx, pool)
	if err != nil {
		bprintf(&buf, "error querying context: %v\n", err)
		return flush(prevAgents)
	}

	worktreeBase := filepath.Join(projectDir, ".worktrees")

	switch currentView {
	case viewMain:
		bprintln(&buf, "╔══════════════════════════════════════════╗")
		bprintln(&buf, "║          ARCHITECT DASHBOARD             ║")
		bprintln(&buf, "╚══════════════════════════════════════════╝")

		renderDAG(&buf, tasks, edges)
		renderAgents(&buf, agents, taskMap)
		renderContext(&buf, ctxEntries)

		bprintln(&buf, "\nPress [1-9] to view agent, [q] to quit")

	case viewAgent:
		if selectedAgent < len(agents) {
			a := agents[selectedAgent]
			logPath := findAgentLog(worktreeBase, a.AgentID)
			renderConversation(&buf, a, taskMap, logPath)
		} else {
			bprintln(&buf, "  (agent not found)")
			bprintln(&buf, "\nPress [b] back, [q] quit")
		}
	}

	return flush(agents)
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
	logPath := filepath.Join(worktreeBase, "agent-"+agentID, "agent.log")
	if _, err := os.Stat(logPath); err == nil {
		return logPath
	}

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
