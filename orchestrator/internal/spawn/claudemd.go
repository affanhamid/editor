package spawn

import (
	"bytes"
	"text/template"

	"github.com/affanhamid/editor/orchestrator/internal/dag"
)

const claudeMDTemplate = `# Agent Instructions

You are agent ` + "`{{.AgentID}}`" + ` working on task #{{.TaskID}}: "{{.TaskTitle}}"

## Your Task
{{.TaskDescription}}

## Communication Protocol
You have access to the ` + "`architect-pg`" + ` MCP server. Use it to communicate:

1. **On start:** Call ` + "`read_context`" + ` for your domain. Call ` + "`read_messages`" + ` to see recent chat.
2. **When you discover something:** Call ` + "`write_context`" + ` immediately.
3. **When you make a decision:** Call ` + "`write_decision`" + ` with rationale.
4. **When you're blocked:** Call ` + "`post_message`" + ` with channel='blockers' and msg_type='blocker'.
   The orchestrator will respond to you directly in this conversation.
5. **When you complete work:** Call ` + "`update_task`" + ` with status='completed' and a summary.
6. **To coordinate with other agents:** Use ` + "`read_messages`" + ` and ` + "`check_decisions`" + `.

## Important
- The orchestrator communicates with you through this conversation. Watch for messages.
- You do NOT need to call ` + "`heartbeat`" + `. The orchestrator tracks your liveness automatically.
- Focus on your task. Commit frequently. Use mcp-pg for structured coordination.

## Before Making Architectural Decisions
Always call ` + "`check_decisions`" + ` for the relevant domain first. If a conflicting decision exists,
call ` + "`post_message`" + ` with msg_type='question' rather than overriding.

## Git
You are working in worktree: ` + "`{{.WorktreePath}}`" + `
Branch: ` + "`{{.BranchName}}`" + `
Commit your work frequently with clear messages.

## Project Conventions
{{.MainClaudeMD}}
`

type claudeMDData struct {
	AgentID         string
	TaskID          int64
	TaskTitle       string
	TaskDescription string
	WorktreePath    string
	BranchName      string
	MainClaudeMD    string
}

// GenerateClaudeMD creates a per-agent CLAUDE.md from the embedded template.
func GenerateClaudeMD(agentID string, task dag.Task, branchName string, worktreePath string, mainClaudeMD string) ([]byte, error) {
	tmpl, err := template.New("claudemd").Parse(claudeMDTemplate)
	if err != nil {
		return nil, err
	}

	data := claudeMDData{
		AgentID:         agentID,
		TaskID:          task.ID,
		TaskTitle:       task.Title,
		TaskDescription: task.Description,
		WorktreePath:    worktreePath,
		BranchName:      branchName,
		MainClaudeMD:    mainClaudeMD,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
