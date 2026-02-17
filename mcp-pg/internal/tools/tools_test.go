package tools_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/affanhamid/editor/mcp-pg/internal/db"
	"github.com/affanhamid/editor/mcp-pg/internal/tools"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const testDBURL = "postgres://architect:architect_local@localhost:5432/architect_test?sslmode=disable"

func setupTestDB(t *testing.T) (*db.Queries, func()) {
	t.Helper()

	connStr := os.Getenv("ARCHITECT_TEST_DB_URL")
	if connStr == "" {
		connStr = testDBURL
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Skipf("Skipping: cannot connect to test database: %v", err)
	}

	// Create tables for testing
	schema := []string{
		`CREATE TABLE IF NOT EXISTS messages (
			id BIGSERIAL PRIMARY KEY,
			agent_id VARCHAR(64) NOT NULL,
			channel VARCHAR(64) NOT NULL DEFAULT 'general',
			content TEXT NOT NULL,
			msg_type VARCHAR(32) NOT NULL,
			ref_task_id BIGINT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS context (
			id BIGSERIAL PRIMARY KEY,
			agent_id VARCHAR(64) NOT NULL,
			domain VARCHAR(128) NOT NULL,
			key_name VARCHAR(256) NOT NULL,
			value TEXT NOT NULL,
			confidence REAL NOT NULL DEFAULT 1.0,
			source_file VARCHAR(512) NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(domain, key_name)
		)`,
		`CREATE TABLE IF NOT EXISTS tasks (
			id BIGSERIAL PRIMARY KEY,
			parent_id BIGINT NULL REFERENCES tasks(id),
			title VARCHAR(512) NOT NULL,
			description TEXT NOT NULL,
			status VARCHAR(32) NOT NULL DEFAULT 'pending',
			assigned_to VARCHAR(64) NULL,
			risk_level VARCHAR(16) NOT NULL DEFAULT 'low',
			output TEXT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS task_edges (
			from_task BIGINT NOT NULL REFERENCES tasks(id),
			to_task BIGINT NOT NULL REFERENCES tasks(id),
			edge_type VARCHAR(16) NOT NULL DEFAULT 'blocks',
			PRIMARY KEY (from_task, to_task)
		)`,
		`CREATE TABLE IF NOT EXISTS decisions (
			id BIGSERIAL PRIMARY KEY,
			agent_id VARCHAR(64) NOT NULL,
			git_sha VARCHAR(40) NULL,
			branch VARCHAR(128),
			domain VARCHAR(128) NOT NULL,
			decision TEXT NOT NULL,
			rationale TEXT NOT NULL,
			alternatives_considered TEXT NULL,
			risk_level VARCHAR(16) NOT NULL DEFAULT 'low',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS agents (
			agent_id VARCHAR(64) PRIMARY KEY,
			pid INT NOT NULL,
			status VARCHAR(16) NOT NULL DEFAULT 'starting',
			current_task_id BIGINT NULL,
			worktree_path VARCHAR(512) NULL,
			started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			last_heartbeat TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
	}

	for _, s := range schema {
		if _, err := pool.Exec(ctx, s); err != nil {
			t.Skipf("Skipping: cannot create schema: %v", err)
		}
	}

	queries := &db.Queries{Pool: pool}

	cleanup := func() {
		tables := []string{"task_edges", "messages", "context", "tasks", "decisions", "agents"}
		for _, table := range tables {
			pool.Exec(ctx, fmt.Sprintf("TRUNCATE %s CASCADE", table))
		}
		pool.Close()
	}

	return queries, cleanup
}

func setupServer(t *testing.T) (*server.MCPServer, *db.Queries, func()) {
	t.Helper()
	queries, cleanup := setupTestDB(t)

	os.Setenv("ARCHITECT_AGENT_ID", "test-agent")
	os.Setenv("ARCHITECT_BRANCH", "feature/test")

	s := server.NewMCPServer("mcp-pg-test", "1.0.0", server.WithToolCapabilities(true))
	cfg := tools.NewConfig(queries)
	tools.RegisterAll(s, cfg)

	return s, queries, cleanup
}

func callTool(t *testing.T, s *server.MCPServer, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	ctx := context.Background()

	tool := s.GetTool(name)
	if tool == nil {
		t.Fatalf("tool %q not registered", name)
	}

	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args

	result, err := tool.Handler(ctx, req)
	if err != nil {
		t.Fatalf("tool %q returned error: %v", name, err)
	}
	return result
}

func getTextContent(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("result has no content")
	}
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	return tc.Text
}

func TestAllToolsRegistered(t *testing.T) {
	s := server.NewMCPServer("test", "1.0.0", server.WithToolCapabilities(true))
	queries := &db.Queries{} // nil pool is fine, we're just checking registration
	os.Setenv("ARCHITECT_AGENT_ID", "test")
	cfg := tools.NewConfig(queries)
	tools.RegisterAll(s, cfg)

	expected := []string{
		"post_message", "read_messages",
		"read_context", "write_context",
		"get_tasks", "claim_task", "update_task",
		"write_decision", "check_decisions",
		"heartbeat", "get_agents",
	}

	registered := s.ListTools()
	for _, name := range expected {
		if _, ok := registered[name]; !ok {
			t.Errorf("tool %q not registered", name)
		}
	}

	if len(registered) != len(expected) {
		t.Errorf("expected %d tools, got %d", len(expected), len(registered))
	}
}

func TestPostAndReadMessages(t *testing.T) {
	s, _, cleanup := setupServer(t)
	defer cleanup()

	// Post a message
	result := callTool(t, s, "post_message", map[string]any{
		"content":  "hello from test",
		"msg_type": "update",
		"channel":  "general",
	})
	if result.IsError {
		t.Fatalf("post_message failed: %s", getTextContent(t, result))
	}
	text := getTextContent(t, result)
	if text == "" {
		t.Fatal("expected non-empty response")
	}

	// Read messages
	result = callTool(t, s, "read_messages", map[string]any{
		"channel": "general",
		"limit":   float64(10),
	})
	if result.IsError {
		t.Fatalf("read_messages failed: %s", getTextContent(t, result))
	}

	var messages []db.Message
	if err := json.Unmarshal([]byte(getTextContent(t, result)), &messages); err != nil {
		t.Fatalf("failed to parse messages: %v", err)
	}
	if len(messages) == 0 {
		t.Fatal("expected at least one message")
	}
	if messages[0].Content != "hello from test" {
		t.Errorf("expected 'hello from test', got %q", messages[0].Content)
	}
}

func TestWriteAndReadContext(t *testing.T) {
	s, _, cleanup := setupServer(t)
	defer cleanup()

	result := callTool(t, s, "write_context", map[string]any{
		"domain":     "database",
		"key":        "orm_library",
		"value":      "pgx",
		"confidence": 0.9,
	})
	if result.IsError {
		t.Fatalf("write_context failed: %s", getTextContent(t, result))
	}

	result = callTool(t, s, "read_context", map[string]any{
		"domain": "database",
	})
	if result.IsError {
		t.Fatalf("read_context failed: %s", getTextContent(t, result))
	}

	var entries []db.ContextEntry
	if err := json.Unmarshal([]byte(getTextContent(t, result)), &entries); err != nil {
		t.Fatalf("failed to parse context: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one context entry")
	}
	if entries[0].Value != "pgx" {
		t.Errorf("expected 'pgx', got %q", entries[0].Value)
	}
}

func TestContextUpsert(t *testing.T) {
	s, _, cleanup := setupServer(t)
	defer cleanup()

	callTool(t, s, "write_context", map[string]any{
		"domain": "database",
		"key":    "orm_library",
		"value":  "gorm",
	})

	// Overwrite
	callTool(t, s, "write_context", map[string]any{
		"domain": "database",
		"key":    "orm_library",
		"value":  "pgx",
	})

	result := callTool(t, s, "read_context", map[string]any{
		"domain": "database",
	})

	var entries []db.ContextEntry
	json.Unmarshal([]byte(getTextContent(t, result)), &entries)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after upsert, got %d", len(entries))
	}
	if entries[0].Value != "pgx" {
		t.Errorf("expected 'pgx' after upsert, got %q", entries[0].Value)
	}
}

func TestClaimTaskRaceCondition(t *testing.T) {
	_, queries, cleanup := setupServer(t)
	defer cleanup()

	ctx := context.Background()
	// Insert a pending task
	var taskID int64
	err := queries.Pool.QueryRow(ctx,
		`INSERT INTO tasks (title, description, status) VALUES ('test task', 'desc', 'pending') RETURNING id`,
	).Scan(&taskID)
	if err != nil {
		t.Fatalf("failed to insert task: %v", err)
	}

	// Two agents try to claim concurrently
	results := make(chan error, 2)
	for _, agentID := range []string{"agent-1", "agent-2"} {
		go func(aid string) {
			_, err := queries.ClaimTask(ctx, aid, taskID)
			results <- err
		}(agentID)
	}

	var successes, failures int
	for range 2 {
		if err := <-results; err != nil {
			failures++
		} else {
			successes++
		}
	}

	if successes != 1 {
		t.Errorf("expected exactly 1 success, got %d", successes)
	}
	if failures != 1 {
		t.Errorf("expected exactly 1 failure, got %d", failures)
	}
}

func TestUpdateTaskOwnership(t *testing.T) {
	_, queries, cleanup := setupServer(t)
	defer cleanup()

	ctx := context.Background()
	var taskID int64
	err := queries.Pool.QueryRow(ctx,
		`INSERT INTO tasks (title, description, status, assigned_to) VALUES ('test', 'desc', 'in_progress', 'agent-1') RETURNING id`,
	).Scan(&taskID)
	if err != nil {
		t.Fatalf("failed to insert task: %v", err)
	}

	// Agent-2 should NOT be able to update agent-1's task
	err = queries.UpdateTask(ctx, "agent-2", taskID, "completed", nil)
	if err == nil {
		t.Error("expected error when non-owner updates task")
	}

	// Agent-1 should succeed
	err = queries.UpdateTask(ctx, "agent-1", taskID, "completed", nil)
	if err != nil {
		t.Errorf("owner should be able to update task: %v", err)
	}
}

func TestWriteAndCheckDecisions(t *testing.T) {
	s, _, cleanup := setupServer(t)
	defer cleanup()

	result := callTool(t, s, "write_decision", map[string]any{
		"domain":    "auth",
		"decision":  "Use JWT for authentication",
		"rationale": "Stateless, scalable",
	})
	if result.IsError {
		t.Fatalf("write_decision failed: %s", getTextContent(t, result))
	}

	result = callTool(t, s, "check_decisions", map[string]any{
		"domain": "auth",
	})
	if result.IsError {
		t.Fatalf("check_decisions failed: %s", getTextContent(t, result))
	}

	var decisions []db.Decision
	json.Unmarshal([]byte(getTextContent(t, result)), &decisions)
	if len(decisions) == 0 {
		t.Fatal("expected at least one decision")
	}
	if decisions[0].Decision != "Use JWT for authentication" {
		t.Errorf("unexpected decision: %q", decisions[0].Decision)
	}
}

func TestHeartbeat(t *testing.T) {
	s, queries, cleanup := setupServer(t)
	defer cleanup()

	ctx := context.Background()
	// Insert agent record first
	_, err := queries.Pool.Exec(ctx,
		`INSERT INTO agents (agent_id, pid, status) VALUES ('test-agent', 1234, 'working')`,
	)
	if err != nil {
		t.Fatalf("failed to insert agent: %v", err)
	}

	result := callTool(t, s, "heartbeat", map[string]any{})
	if result.IsError {
		t.Fatalf("heartbeat failed: %s", getTextContent(t, result))
	}
}

func TestGetAgents(t *testing.T) {
	s, queries, cleanup := setupServer(t)
	defer cleanup()

	ctx := context.Background()
	_, err := queries.Pool.Exec(ctx,
		`INSERT INTO agents (agent_id, pid, status) VALUES ('agent-1', 1111, 'working'), ('agent-2', 2222, 'idle')`,
	)
	if err != nil {
		t.Fatalf("failed to insert agents: %v", err)
	}

	result := callTool(t, s, "get_agents", map[string]any{})
	if result.IsError {
		t.Fatalf("get_agents failed: %s", getTextContent(t, result))
	}

	var agents []db.Agent
	json.Unmarshal([]byte(getTextContent(t, result)), &agents)
	if len(agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(agents))
	}

	// Filter by status
	result = callTool(t, s, "get_agents", map[string]any{"status": "working"})
	json.Unmarshal([]byte(getTextContent(t, result)), &agents)
	if len(agents) != 1 {
		t.Errorf("expected 1 working agent, got %d", len(agents))
	}
}
