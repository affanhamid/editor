package spawn

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// AgentHandle holds the stdin pipe and PID for a running agent process.
type AgentHandle struct {
	Stdin io.WriteCloser
	PID   int
}

// AgentRegistry is a thread-safe map of agentID → AgentHandle.
type AgentRegistry struct {
	mu     sync.RWMutex
	agents map[string]*AgentHandle
}

// NewAgentRegistry creates an empty registry.
func NewAgentRegistry() *AgentRegistry {
	return &AgentRegistry{agents: make(map[string]*AgentHandle)}
}

// Register adds an agent to the registry.
func (r *AgentRegistry) Register(agentID string, stdin io.WriteCloser, pid int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[agentID] = &AgentHandle{Stdin: stdin, PID: pid}
}

// Deregister removes an agent from the registry.
func (r *AgentRegistry) Deregister(agentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.agents, agentID)
}

// streamMessage is the NDJSON format expected by claude --input-format stream-json.
type streamMessage struct {
	Type    string        `json:"type"`
	Message streamContent `json:"message"`
}

type streamContent struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Send writes a message to an agent's stdin pipe as a stream-json user message.
func (r *AgentRegistry) Send(agentID string, message string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	handle, ok := r.agents[agentID]
	if !ok {
		return fmt.Errorf("agent %s not found in registry", agentID)
	}
	msg := streamMessage{
		Type: "user",
		Message: streamContent{
			Role:    "user",
			Content: message,
		},
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal stream message: %w", err)
	}
	data = append(data, '\n')
	_, err = handle.Stdin.Write(data)
	return err
}

// IsAlive returns true if the agent is registered (process still running).
func (r *AgentRegistry) IsAlive(agentID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.agents[agentID]
	return ok
}
