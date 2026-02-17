package spawn

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// CreateWorktree creates a git worktree for an agent.
// If parentWorktrees is non-empty, it branches from the first parent and
// merges the rest, so the agent starts with all dependency work.
func CreateWorktree(projectDir string, agentID string, taskID int64, parentWorktrees []string) (string, string, error) {
	branchName := fmt.Sprintf("agent/%s/task-%d", agentID[:8], taskID)
	worktreePath := filepath.Join(projectDir, ".worktrees", fmt.Sprintf("agent-%s", agentID[:8]))

	// Determine base ref: first parent's branch, or the repo's default branch
	baseRef := defaultBranch(projectDir)
	if len(parentWorktrees) > 0 {
		// Extract branch name from the first parent worktree
		branch, err := worktreeBranch(projectDir, parentWorktrees[0])
		if err == nil && branch != "" {
			baseRef = branch
		}
	}

	cmd := exec.Command("git", "worktree", "add", worktreePath, "-b", branchName, baseRef)
	cmd.Dir = projectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("git worktree add: %w\n%s", err, out)
	}

	// Merge remaining parent branches into the new worktree
	for _, pw := range parentWorktrees[min(1, len(parentWorktrees)):] {
		branch, err := worktreeBranch(projectDir, pw)
		if err != nil || branch == "" || branch == baseRef {
			continue
		}
		mergeCmd := exec.Command("git", "merge", "--no-edit", branch)
		mergeCmd.Dir = worktreePath
		if mergeOut, mergeErr := mergeCmd.CombinedOutput(); mergeErr != nil {
			// Abort failed merge and continue — better to have partial context than fail
			abortCmd := exec.Command("git", "merge", "--abort")
			abortCmd.Dir = worktreePath
			_ = abortCmd.Run()
			fmt.Printf("warning: merge of %s failed, skipping: %s\n", branch, mergeOut)
		}
	}

	return worktreePath, branchName, nil
}

// worktreeBranch returns the branch name checked out in a worktree path.
func worktreeBranch(projectDir string, worktreePath string) (string, error) {
	absPath := worktreePath
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(projectDir, absPath)
	}
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = absPath
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// defaultBranch returns the current branch of the repo (HEAD), falling back to "main".
func defaultBranch(projectDir string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = projectDir
	out, err := cmd.Output()
	if err != nil {
		return "main"
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" {
		return "main"
	}
	return branch
}

// RemoveWorktree cleans up after an agent is done.
func RemoveWorktree(projectDir string, worktreePath string) error {
	cmd := exec.Command("git", "worktree", "remove", worktreePath)
	cmd.Dir = projectDir
	return cmd.Run()
}
