package spawn

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/affanhamid/editor/orchestrator/internal/dag"
	"github.com/affanhamid/editor/orchestrator/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds configuration for spawning sessions.
type Config struct {
	MCPPgBinary  string
	DBURL        string
	MainClaudeMD string
}

// SpawnSession creates a worktree, writes config files, and starts a Claude Code session.
func SpawnSession(ctx context.Context, pool *pgxpool.Pool, task dag.Task, projectDir string, config Config) error {
	agentID := uuid.New().String()

	// 1. Create git worktree
	worktreePath, branchName, err := CreateWorktree(projectDir, agentID, task.ID)
	if err != nil {
		return fmt.Errorf("create worktree: %w", err)
	}

	// 2. Register agent in Postgres
	if err := db.RegisterAgent(ctx, pool, agentID, task.ID, worktreePath); err != nil {
		return fmt.Errorf("register agent: %w", err)
	}

	// 3. Claim the task (atomic: fails if another agent claimed it first)
	claimed, err := db.ClaimTask(ctx, pool, task.ID, "in_progress", agentID)
	if err != nil {
		return fmt.Errorf("claim task: %w", err)
	}
	if !claimed {
		// Another agent already claimed this task; clean up worktree and skip.
		_ = RemoveWorktree(projectDir, worktreePath)
		log.Printf("task %d already claimed, skipping", task.ID)
		return nil
	}

	// 4. Write CLAUDE.md into worktree
	claudeMD, err := GenerateClaudeMD(agentID, task, branchName, worktreePath, config.MainClaudeMD)
	if err != nil {
		return fmt.Errorf("generate CLAUDE.md: %w", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, "CLAUDE.md"), claudeMD, 0644); err != nil {
		return fmt.Errorf("write CLAUDE.md: %w", err)
	}

	// 5. Write .mcp.json into worktree
	mcpJSON, err := GenerateMCPConfig(agentID, branchName, config.MCPPgBinary, config.DBURL)
	if err != nil {
		return fmt.Errorf("generate .mcp.json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, ".mcp.json"), mcpJSON, 0644); err != nil {
		return fmt.Errorf("write .mcp.json: %w", err)
	}

	// 6. Spawn Claude Code
	cmd := exec.CommandContext(ctx, "claude",
		"--print",
		"--project", worktreePath,
		"--prompt", task.Description,
	)
	cmd.Dir = worktreePath
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start claude: %w", err)
	}

	// 7. Update agent with PID
	if err := db.UpdateAgentPID(ctx, pool, agentID, cmd.Process.Pid); err != nil {
		log.Printf("warning: failed to update agent PID: %v", err)
	}

	// 8. Wait for completion in a goroutine
	go func() {
		err := cmd.Wait()
		bgCtx := context.Background()
		if err != nil {
			log.Printf("agent %s (task %d) failed: %v", agentID[:8], task.ID, err)
			_ = db.UpdateAgentStatus(bgCtx, pool, agentID, "dead")
		} else {
			log.Printf("agent %s (task %d) completed", agentID[:8], task.ID)
			_ = db.UpdateAgentStatus(bgCtx, pool, agentID, "idle")
		}
	}()

	log.Printf("spawned agent %s for task %d: %q", agentID[:8], task.ID, task.Title)
	return nil
}
