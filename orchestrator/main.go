package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/affanhamid/editor/orchestrator/internal/dag"
	"github.com/affanhamid/editor/orchestrator/internal/db"
	"github.com/affanhamid/editor/orchestrator/internal/monitor"
	"github.com/affanhamid/editor/orchestrator/internal/spawn"
)

func main() {
	projectDir := flag.String("project", ".", "Path to the git repository")
	dbURL := flag.String("db", "postgres://architect:architect_local@localhost:5432/architect?sslmode=disable", "PostgreSQL connection string")
	mcpBinary := flag.String("mcp-pg", "./mcp-pg/mcp-pg", "Path to the mcp-pg binary")
	prompt := flag.String("prompt", "", "The user prompt to decompose and execute")
	flag.Parse()

	if *prompt == "" {
		fmt.Fprintln(os.Stderr, "error: --prompt is required")
		flag.Usage()
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals for graceful shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("received signal %v, shutting down...", sig)
		cancel()
	}()

	// Connect to Postgres.
	pool, err := db.NewPool(ctx, *dbURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Start LISTEN/NOTIFY listener.
	eventCh := make(chan db.Event, 100)
	go func() {
		if err := db.StartListener(ctx, *dbURL, eventCh); err != nil {
			log.Printf("listener error: %v", err)
			cancel()
		}
	}()

	// Start heartbeat monitor.
	go monitor.MonitorHeartbeats(ctx, pool, 2*time.Minute)

	// Decompose prompt into DAG.
	log.Printf("decomposing prompt: %q", *prompt)
	taskDAG, err := dag.DecomposePrompt(*prompt, *projectDir)
	if err != nil {
		log.Fatalf("failed to decompose prompt: %v", err)
	}
	log.Printf("decomposed into %d tasks", len(taskDAG.Tasks))

	// Write DAG to Postgres.
	idMap := make(map[int64]int64) // original ID → Postgres ID
	for _, task := range taskDAG.Tasks {
		pgID, err := db.InsertTask(ctx, pool, task.Title, task.Description, task.RiskLevel)
		if err != nil {
			log.Fatalf("failed to insert task %q: %v", task.Title, err)
		}
		idMap[task.ID] = pgID
		log.Printf("  task %d → pg:%d: %s", task.ID, pgID, task.Title)
	}
	for _, edge := range taskDAG.Edges {
		if err := db.InsertEdge(ctx, pool, idMap[edge.From], idMap[edge.To]); err != nil {
			log.Fatalf("failed to insert edge %d→%d: %v", edge.From, edge.To, err)
		}
	}

	// Read main CLAUDE.md if it exists.
	mainClaudeMD := readMainClaudeMD(*projectDir)

	// Spawn sessions for immediately-ready tasks.
	config := spawn.Config{
		MCPPgBinary:  *mcpBinary,
		DBURL:        *dbURL,
		MainClaudeMD: mainClaudeMD,
	}
	ready, err := dag.ReadyTasks(ctx, pool)
	if err != nil {
		log.Fatalf("failed to find ready tasks: %v", err)
	}
	log.Printf("spawning %d initial sessions", len(ready))
	for _, task := range ready {
		if err := spawn.SpawnSession(ctx, pool, task, *projectDir, config); err != nil {
			log.Printf("error spawning session for task %d: %v", task.ID, err)
		}
	}

	// Process events until all tasks done or context cancelled.
	log.Println("entering event loop...")
	monitor.HandleEvents(ctx, pool, eventCh, *projectDir, config)
	log.Println("orchestrator shutdown complete")
}

func readMainClaudeMD(projectDir string) string {
	data, err := os.ReadFile(projectDir + "/CLAUDE.md")
	if err != nil {
		return ""
	}
	return string(data)
}
