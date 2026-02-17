package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/affanhamid/editor/orchestrator/internal/dag"
	"github.com/affanhamid/editor/orchestrator/internal/db"
	"github.com/affanhamid/editor/orchestrator/internal/monitor"
	"github.com/affanhamid/editor/orchestrator/internal/spawn"
)

func main() {
	projectDir := flag.String("project", ".", "Path to the git repository")
	dbURL := flag.String("db", "postgres://architect:architect_local@localhost:5432/architect_meta?sslmode=disable", "PostgreSQL connection string")
	mcpBinary := flag.String("mcp-pg", "", "Path to the mcp-pg binary (auto-detected if empty)")
	prompt := flag.String("prompt", "", "The user prompt to decompose and execute")
	promptFile := flag.String("prompt-file", "", "Path to a file containing the prompt (alternative to --prompt)")
	flag.Parse()

	// Resolve prompt from --prompt or --prompt-file.
	promptText := *prompt
	if promptText == "" && *promptFile != "" {
		data, err := os.ReadFile(*promptFile)
		if err != nil {
			log.Fatalf("failed to read prompt file: %v", err)
		}
		promptText = string(data)
	}
	if promptText == "" {
		fmt.Fprintln(os.Stderr, "error: --prompt or --prompt-file is required")
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

	// Auto-create database and run migrations.
	if err := db.EnsureDatabase(*dbURL); err != nil {
		log.Fatalf("failed to ensure database: %v", err)
	}

	// Connect to Postgres.
	pool, err := db.NewPool(ctx, *dbURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Run migrations.
	if err := db.RunMigrations(ctx, pool); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	// Start LISTEN/NOTIFY listener.
	eventCh := make(chan db.Event, 100)
	go func() {
		if err := db.StartListener(ctx, *dbURL, eventCh); err != nil {
			log.Printf("listener error: %v", err)
			cancel()
		}
	}()

	// Create agent registry for tracking live agent processes.
	registry := spawn.NewAgentRegistry()

	// Decompose prompt into DAG.
	log.Printf("decomposing prompt (%d chars)", len(promptText))
	taskDAG, err := dag.DecomposePrompt(promptText, *projectDir)
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

	// Resolve the mcp-pg binary path.
	resolvedMCPBinary := spawn.ResolveMCPPgBinary(*mcpBinary)
	log.Printf("using mcp-pg binary: %s", resolvedMCPBinary)

	// Spawn sessions for immediately-ready tasks.
	config := spawn.Config{
		MCPPgBinary:  resolvedMCPBinary,
		DBURL:        *dbURL,
		MainClaudeMD: mainClaudeMD,
	}
	ready, err := dag.ReadyTasks(ctx, pool)
	if err != nil {
		log.Fatalf("failed to find ready tasks: %v", err)
	}
	log.Printf("spawning %d initial sessions", len(ready))
	for _, task := range ready {
		if _, err := spawn.SpawnSession(ctx, pool, registry, task, *projectDir, config); err != nil {
			log.Printf("error spawning session for task %d: %v", task.ID, err)
		}
	}

	// Process events until all tasks done or context cancelled.
	log.Println("entering event loop...")
	monitor.HandleEvents(ctx, pool, registry, eventCh, *projectDir, config)
	log.Println("orchestrator shutdown complete")
}

func readMainClaudeMD(projectDir string) string {
	data, err := os.ReadFile(projectDir + "/CLAUDE.md")
	if err != nil {
		return ""
	}
	return string(data)
}
