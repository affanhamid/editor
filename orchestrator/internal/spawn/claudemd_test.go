package spawn

import (
	"strings"
	"testing"

	"github.com/affanhamid/editor/orchestrator/internal/dag"
)

func TestGenerateClaudeMD(t *testing.T) {
	task := dag.Task{
		ID:          42,
		Title:       "Implement auth",
		Description: "Build the authentication module with JWT support.",
	}
	agentID := "abc12345-6789-0000-0000-000000000000"
	branchName := "agent/abc12345/task-42"
	worktreePath := "/tmp/worktrees/agent-abc12345"
	mainClaudeMD := "Use Go 1.22+."

	result, err := GenerateClaudeMD(agentID, task, branchName, worktreePath, mainClaudeMD)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := string(result)

	checks := []string{
		agentID,
		"task #42",
		"Implement auth",
		"Build the authentication module",
		branchName,
		worktreePath,
		"Use Go 1.22+.",
		"architect-pg",
	}
	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("expected CLAUDE.md to contain %q", check)
		}
	}
}
