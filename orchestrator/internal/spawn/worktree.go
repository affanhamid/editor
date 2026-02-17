package spawn

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// CreateWorktree creates a git worktree for an agent.
func CreateWorktree(projectDir string, agentID string, taskID int64) (string, string, error) {
	branchName := fmt.Sprintf("agent/%s/task-%d", agentID[:8], taskID)
	worktreePath := filepath.Join(projectDir, ".worktrees", fmt.Sprintf("agent-%s", agentID[:8]))

	cmd := exec.Command("git", "worktree", "add", worktreePath, "-b", branchName)
	cmd.Dir = projectDir
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("git worktree add: %w", err)
	}
	return worktreePath, branchName, nil
}

// RemoveWorktree cleans up after an agent is done.
func RemoveWorktree(projectDir string, worktreePath string) error {
	cmd := exec.Command("git", "worktree", "remove", worktreePath)
	cmd.Dir = projectDir
	return cmd.Run()
}
