package tools

import (
	"context"
	"fmt"

	"github.com/affanhamid/editor/mcp-pg/internal/db"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerTaskTools(s *server.MCPServer, cfg *Config) {
	getTasks := mcp.NewTool("get_tasks",
		mcp.WithDescription("Get tasks from the DAG. Use to check what work is available or see the status of other tasks."),
		mcp.WithString("status",
			mcp.Description("Filter by status: 'pending', 'in_progress', 'completed', 'failed', 'blocked'"),
		),
		mcp.WithString("assigned_to",
			mcp.Description("Filter by agent ID"),
		),
	)

	claimTask := mcp.NewTool("claim_task",
		mcp.WithDescription("Claim a pending task to work on. Will fail if the task is already claimed by another agent."),
		mcp.WithNumber("task_id",
			mcp.Description("The task ID to claim"),
			mcp.Required(),
		),
	)

	updateTask := mcp.NewTool("update_task",
		mcp.WithDescription("Update the status of your current task. Use 'completed' when done, 'failed' if you cannot complete it, 'blocked' if you need something from another agent."),
		mcp.WithNumber("task_id",
			mcp.Description("The task ID"),
			mcp.Required(),
		),
		mcp.WithString("status",
			mcp.Description("New task status"),
			mcp.Required(),
			mcp.Enum("in_progress", "completed", "failed", "blocked"),
		),
		mcp.WithString("output",
			mcp.Description("Summary of what was accomplished or why it failed/is blocked"),
		),
	)

	s.AddTool(getTasks, makeGetTasksHandler(cfg))
	s.AddTool(claimTask, makeClaimTaskHandler(cfg))
	s.AddTool(updateTask, makeUpdateTaskHandler(cfg))
}

func makeGetTasksHandler(cfg *Config) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var status, assignedTo *string
		if v := request.GetString("status", ""); v != "" {
			status = &v
		}
		if v := request.GetString("assigned_to", ""); v != "" {
			assignedTo = &v
		}

		result, err := cfg.Queries.GetTasks(ctx, status, assignedTo)
		if err != nil {
			return errorResult(err), nil
		}

		return textResult(db.ToJSON(result)), nil
	}
}

func makeClaimTaskHandler(cfg *Config) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		taskID := int64(request.GetFloat("task_id", 0))
		if taskID == 0 {
			return errorResult(fmt.Errorf("task_id is required")), nil
		}

		task, err := cfg.Queries.ClaimTask(ctx, cfg.AgentID, taskID)
		if err != nil {
			return errorResult(err), nil
		}

		return textResult(fmt.Sprintf("Claimed task %d: %s", task.ID, task.Title)), nil
	}
}

func makeUpdateTaskHandler(cfg *Config) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		taskID := int64(request.GetFloat("task_id", 0))
		if taskID == 0 {
			return errorResult(fmt.Errorf("task_id is required")), nil
		}

		status := request.GetString("status", "")
		if status == "" {
			return errorResult(fmt.Errorf("status is required")), nil
		}

		var output *string
		if v := request.GetString("output", ""); v != "" {
			output = &v
		}

		err := cfg.Queries.UpdateTask(ctx, cfg.AgentID, taskID, status, output)
		if err != nil {
			return errorResult(err), nil
		}

		return textResult(fmt.Sprintf("Task %d updated to %s", taskID, status)), nil
	}
}
