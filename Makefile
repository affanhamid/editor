PREFIX := $(HOME)/.architect

.PHONY: all clean install orchestrator dashboard mcp-pg architect-bridge

all: orchestrator dashboard mcp-pg architect-bridge

orchestrator:
	go build -C orchestrator -o ../orchestrator/orchestrator .

dashboard:
	go build -C orchestrator -o ../orchestrator/dashboard ./cmd/dashboard/

mcp-pg:
	go build -C mcp-pg -o ../mcp-pg/mcp-pg .

architect-bridge:
	go build -C architect-bridge -o ../architect-bridge/architect-bridge .

install: all
	mkdir -p $(PREFIX)/bin
	cp orchestrator/orchestrator $(PREFIX)/bin/architect
	cp orchestrator/dashboard $(PREFIX)/bin/architect-dashboard
	cp mcp-pg/mcp-pg $(PREFIX)/bin/architect-mcp-pg
	cp architect-bridge/architect-bridge $(PREFIX)/bin/architect-bridge
	codesign -s - $(PREFIX)/bin/architect
	codesign -s - $(PREFIX)/bin/architect-dashboard
	codesign -s - $(PREFIX)/bin/architect-mcp-pg
	codesign -s - $(PREFIX)/bin/architect-bridge
	@echo "Installed to $(PREFIX)/bin/"
	@echo "Add to PATH: export PATH=\"$(PREFIX)/bin:\$$PATH\""

clean:
	rm -f orchestrator/orchestrator orchestrator/dashboard mcp-pg/mcp-pg architect-bridge/architect-bridge

db-setup:
	@for f in schema/migrations/*.sql; do psql "postgres://architect:architect_local@localhost:5432/architect_meta?sslmode=disable" -f "$$f"; done
	@for f in schema/triggers/*.sql; do psql "postgres://architect:architect_local@localhost:5432/architect_meta?sslmode=disable" -f "$$f"; done
	@echo "DB setup complete."

db-clean:
	./schema/clean.sh
