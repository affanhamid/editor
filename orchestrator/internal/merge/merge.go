package merge

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/jackc/pgx/v5/pgxpool"
)

// BranchSummary holds info about a completed branch ready for review.
type BranchSummary struct {
	AgentID      string
	WorktreePath string
	TaskTitle    string
	Output       string
}

// MergeWorktree merges an agent's branch back to main.
func MergeWorktree(projectDir string, branchName string) error {
	cmd := exec.Command("git", "merge", "--no-ff", branchName,
		"-m", fmt.Sprintf("Merge %s (automated agent merge)", branchName))
	cmd.Dir = projectDir
	return cmd.Run()
}

// StageForReview lists completed branches for human review.
func StageForReview(ctx context.Context, pool *pgxpool.Pool) ([]BranchSummary, error) {
	rows, err := pool.Query(ctx, `
		SELECT a.agent_id, a.worktree_path, t.title, COALESCE(t.output, '')
		FROM agents a JOIN tasks t ON a.current_task_id = t.id
		WHERE t.status = 'completed'
		ORDER BY t.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []BranchSummary
	for rows.Next() {
		var s BranchSummary
		if err := rows.Scan(&s.AgentID, &s.WorktreePath, &s.TaskTitle, &s.Output); err != nil {
			return nil, err
		}
		summaries = append(summaries, s)
	}
	return summaries, rows.Err()
}
