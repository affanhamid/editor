package main

import (
	"context"
	"log"

	"github.com/affanhamid/editor/mcp-pg/internal/db"
	mcpserver "github.com/affanhamid/editor/mcp-pg/internal/server"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	ctx := context.Background()

	pool, err := db.NewPool(ctx)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	queries := &db.Queries{Pool: pool}
	s := mcpserver.New(queries)

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("MCP server error: %v", err)
	}
}
