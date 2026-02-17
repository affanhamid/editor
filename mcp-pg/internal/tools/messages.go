package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/affanhamid/editor/mcp-pg/internal/db"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerMessageTools(s *server.MCPServer, cfg *Config) {
	postMessage := mcp.NewTool("post_message",
		mcp.WithDescription("Post a message to the agent communication channel. Use this to share updates, ask questions, report blockers, or broadcast discoveries to other agents."),
		mcp.WithString("channel",
			mcp.Description("Channel name: 'general', 'blockers', 'discoveries', or a task-specific channel like 'task-42'"),
			mcp.DefaultString("general"),
		),
		mcp.WithString("content",
			mcp.Description("The message content"),
			mcp.Required(),
		),
		mcp.WithString("msg_type",
			mcp.Description("Type of message"),
			mcp.Required(),
			mcp.Enum("update", "question", "answer", "blocker", "discovery", "decision"),
		),
		mcp.WithNumber("ref_task_id",
			mcp.Description("Optional: link this message to a specific task ID"),
		),
	)

	readMessages := mcp.NewTool("read_messages",
		mcp.WithDescription("Read recent messages from the agent communication channels. Use this on startup to get context, and periodically to check for updates from other agents."),
		mcp.WithString("channel",
			mcp.Description("Filter by channel. Omit to read all channels."),
		),
		mcp.WithString("since",
			mcp.Description("ISO 8601 timestamp. Only return messages after this time."),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of messages to return"),
			mcp.DefaultNumber(50),
		),
	)

	s.AddTool(postMessage, makePostMessageHandler(cfg))
	s.AddTool(readMessages, makeReadMessagesHandler(cfg))
}

func makePostMessageHandler(cfg *Config) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		content := request.GetString("content", "")
		msgType := request.GetString("msg_type", "update")
		channel := request.GetString("channel", "general")

		var refTaskID *int64
		if v := request.GetFloat("ref_task_id", 0); v != 0 {
			id := int64(v)
			refTaskID = &id
		}

		msg, err := cfg.Queries.PostMessage(ctx, cfg.AgentID, channel, content, msgType, refTaskID)
		if err != nil {
			return errorResult(err), nil
		}

		return textResult(fmt.Sprintf("Message posted: id=%d at %s", msg.ID, msg.CreatedAt.Format(time.RFC3339))), nil
	}
}

func makeReadMessagesHandler(cfg *Config) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var channel *string
		if v := request.GetString("channel", ""); v != "" {
			channel = &v
		}

		var since *time.Time
		if v := request.GetString("since", ""); v != "" {
			t, err := time.Parse(time.RFC3339, v)
			if err != nil {
				return errorResult(fmt.Errorf("invalid 'since' timestamp: %w", err)), nil
			}
			since = &t
		}

		limit := int(request.GetFloat("limit", 50))

		messages, err := cfg.Queries.ReadMessages(ctx, channel, since, limit)
		if err != nil {
			return errorResult(err), nil
		}

		return textResult(db.ToJSON(messages)), nil
	}
}
