package spawn

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/affanhamid/editor/orchestrator/internal/dag"
	"github.com/affanhamid/editor/orchestrator/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// filterEnv returns a copy of env with the named variable removed.
func filterEnv(env []string, name string) []string {
	prefix := name + "="
	out := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			out = append(out, e)
		}
	}
	return out
}

// Config holds configuration for spawning sessions.
type Config struct {
	MCPPgBinary  string
	DBURL        string
	MainClaudeMD string
}

// SpawnSession creates a worktree, writes config files, and starts an interactive Claude Code session.
// It returns the agentID so the caller can track it.
func SpawnSession(ctx context.Context, pool *pgxpool.Pool, registry *AgentRegistry,
	task dag.Task, projectDir string, config Config) (string, error) {

	agentID := uuid.New().String()

	// 1. Create git worktree
	worktreePath, branchName, err := CreateWorktree(projectDir, agentID, task.ID)
	if err != nil {
		return "", fmt.Errorf("create worktree: %w", err)
	}

	// 2. Register agent in Postgres
	if err := db.RegisterAgent(ctx, pool, agentID, task.ID, worktreePath); err != nil {
		return "", fmt.Errorf("register agent: %w", err)
	}

	// 3. Claim the task (atomic: fails if another agent claimed it first)
	claimed, err := db.ClaimTask(ctx, pool, task.ID, "in_progress", agentID)
	if err != nil {
		return "", fmt.Errorf("claim task: %w", err)
	}
	if !claimed {
		_ = RemoveWorktree(projectDir, worktreePath)
		log.Printf("task %d already claimed, skipping", task.ID)
		return "", nil
	}

	// 4. Write CLAUDE.md into worktree
	claudeMD, err := GenerateClaudeMD(agentID, task, branchName, worktreePath, config.MainClaudeMD)
	if err != nil {
		return "", fmt.Errorf("generate CLAUDE.md: %w", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, "CLAUDE.md"), claudeMD, 0644); err != nil {
		return "", fmt.Errorf("write CLAUDE.md: %w", err)
	}

	// 5. Write .mcp.json into worktree
	mcpJSON, err := GenerateMCPConfig(agentID, branchName, config.MCPPgBinary, config.DBURL)
	if err != nil {
		return "", fmt.Errorf("generate .mcp.json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, ".mcp.json"), mcpJSON, 0644); err != nil {
		return "", fmt.Errorf("write .mcp.json: %w", err)
	}

	// 6. Spawn Claude Code in interactive mode with scoped permissions
	cmd := exec.CommandContext(ctx, "claude",
		"--allowedTools", "Edit,Write,Read,Glob,Grep,Bash,mcp__architect-pg__*",
	)
	cmd.Dir = worktreePath
	cmd.Env = filterEnv(os.Environ(), "CLAUDECODE")

	// Hold stdin pipe for sending messages to the agent
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("stdin pipe: %w", err)
	}

	// Stdout/stderr to log file
	logFile, err := os.Create(filepath.Join(worktreePath, "agent.log"))
	if err != nil {
		return "", fmt.Errorf("create log: %w", err)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return "", fmt.Errorf("start claude: %w", err)
	}

	// 7. Update agent with PID
	if err := db.UpdateAgentPID(ctx, pool, agentID, cmd.Process.Pid); err != nil {
		log.Printf("warning: failed to update agent PID: %v", err)
	}

	// 8. Register in the agent registry
	registry.Register(agentID, stdinPipe, cmd.Process.Pid)

	// 9. Send initial prompt via stdin
	initialPrompt := fmt.Sprintf("You are working on task #%d: %q\n\n%s\n",
		task.ID, task.Title, task.Description)
	if _, err := stdinPipe.Write([]byte(initialPrompt)); err != nil {
		log.Printf("warning: failed to write initial prompt to agent %s: %v", agentID[:8], err)
	}

	// 10. Wait for completion in a goroutine
	go func() {
		err := cmd.Wait()
		logFile.Close()
		registry.Deregister(agentID)

		bgCtx := context.Background()
		if err != nil {
			log.Printf("agent %s (task %d) failed: %v", agentID[:8], task.ID, err)
			_ = db.UpdateAgentStatus(bgCtx, pool, agentID, "dead")
			_, _ = db.ClaimTask(bgCtx, pool, task.ID, "failed", agentID)
		} else {
			log.Printf("agent %s (task %d) completed", agentID[:8], task.ID)
			_ = db.UpdateAgentStatus(bgCtx, pool, agentID, "idle")
			_ = db.CompleteTask(bgCtx, pool, task.ID, agentID)
		}
	}()

	log.Printf("spawned agent %s for task %d: %q", agentID[:8], task.ID, task.Title)
	return agentID, nil
}
