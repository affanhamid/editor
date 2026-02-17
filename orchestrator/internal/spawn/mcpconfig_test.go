package spawn

import (
	"encoding/json"
	"testing"
)

func TestGenerateMCPConfig(t *testing.T) {
	result, err := GenerateMCPConfig(
		"agent-123",
		"agent/abc12345/task-1",
		"/usr/local/bin/mcp-pg",
		"postgres://localhost/test",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var config map[string]any
	if err := json.Unmarshal(result, &config); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	servers, ok := config["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal("missing mcpServers key")
	}

	pg, ok := servers["architect-pg"].(map[string]any)
	if !ok {
		t.Fatal("missing architect-pg server")
	}

	if pg["command"] != "/usr/local/bin/mcp-pg" {
		t.Errorf("unexpected command: %v", pg["command"])
	}

	env, ok := pg["env"].(map[string]any)
	if !ok {
		t.Fatal("missing env")
	}
	if env["ARCHITECT_AGENT_ID"] != "agent-123" {
		t.Errorf("unexpected agent ID: %v", env["ARCHITECT_AGENT_ID"])
	}
	if env["ARCHITECT_BRANCH"] != "agent/abc12345/task-1" {
		t.Errorf("unexpected branch: %v", env["ARCHITECT_BRANCH"])
	}
}
