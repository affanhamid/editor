package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"architect-bridge/internal/pg"
	"architect-bridge/internal/protocol"
	"architect-bridge/internal/state"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dbURL := flag.String("db", "postgres://architect:architect_local@localhost:5432/architect?sslmode=disable", "PostgreSQL connection string")
	socketPath := flag.String("socket", "/tmp/architect-bridge.sock", "Unix socket path")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := pgxpool.New(ctx, *dbURL)
	if err != nil {
		log.Fatalf("failed to connect to postgres: %v", err)
	}
	defer pool.Close()

	eventCh := make(chan protocol.Event, 100)
	go pg.StartListener(ctx, *dbURL, eventCh)

	server := protocol.NewSocketServer(*socketPath)
	go server.Start(ctx)

	// Forward PG events to all connected Lua clients
	go func() {
		for event := range eventCh {
			server.Broadcast(event)
		}
	}()

	// Handle incoming commands from Lua
	go func() {
		for cmd := range server.Commands() {
			handleCommand(ctx, pool, cmd)
		}
	}()

	// Block until signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	cancel()
}

func handleCommand(ctx context.Context, pool *pgxpool.Pool, cmd protocol.Command) {
	switch cmd.Type {
	case "submit_prompt":
		prompt, _ := cmd.DataString("prompt")
		_, err := pool.Exec(ctx,
			`INSERT INTO messages (agent_id, channel, content, msg_type) VALUES ('human', 'general', $1, 'decision')`,
			prompt)
		if err != nil {
			log.Printf("failed to submit prompt: %v", err)
		}

	case "approve_consultation":
		taskID, _ := cmd.DataInt("task_id")
		approved, _ := cmd.DataBool("approved")
		status := "approved"
		if !approved {
			status = "rejected"
		}
		_, err := pool.Exec(ctx,
			`UPDATE tasks SET consultation_status = $1 WHERE id = $2`,
			status, taskID)
		if err != nil {
			log.Printf("failed to update consultation: %v", err)
		}

	case "kill_agent":
		agentID, _ := cmd.DataString("agent_id")
		_, err := pool.Exec(ctx,
			`UPDATE agents SET status = 'dead' WHERE agent_id = $1`,
			agentID)
		if err != nil {
			log.Printf("failed to kill agent: %v", err)
		}

	case "post_message":
		channel, _ := cmd.DataString("channel")
		content, _ := cmd.DataString("content")
		msgType, _ := cmd.DataString("msg_type")
		_, err := pool.Exec(ctx,
			`INSERT INTO messages (agent_id, channel, content, msg_type) VALUES ('human', $1, $2, $3)`,
			channel, content, msgType)
		if err != nil {
			log.Printf("failed to post message: %v", err)
		}

	case "request_snapshot":
		snapshot, err := state.GetSnapshot(ctx, pool)
		if err != nil {
			log.Printf("failed to get snapshot: %v", err)
			return
		}
		if cmd.Conn != nil {
			cmd.Conn.Send(protocol.Event{Type: "snapshot", Data: snapshot})
		}

	default:
		log.Printf("unknown command type: %q", cmd.Type)
	}
}
