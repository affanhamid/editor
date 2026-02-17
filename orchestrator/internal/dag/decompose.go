package dag

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// filterEnv returns a copy of env with the named variable removed.
func filterEnv(env []string, name string) []string {
	prefix := name + "="
	out := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			out = append(out, e)
		}
	}
	return out
}

// decompositionResponse is the expected JSON output from Claude Code.
type decompositionResponse struct {
	Tasks []Task `json:"tasks"`
}

// jsonBlockRe matches a JSON object in Claude's text output.
var jsonBlockRe = regexp.MustCompile(`(?s)\{.*"tasks"\s*:\s*\[.*\]\s*\}`)

// DecomposePrompt takes a user prompt and returns a structured DAG
// by asking Claude Code to decompose it.
func DecomposePrompt(prompt string, projectDir string) (*DAG, error) {
	plannerPrompt := fmt.Sprintf(`You are a task decomposition agent. Given the following user request,
decompose it into a set of tasks that can be executed in parallel where possible.

Output ONLY valid JSON in this exact format:
{
  "tasks": [
    {
      "id": 1,
      "title": "short title",
      "description": "detailed description of what to implement",
      "risk_level": "low|medium|high",
      "blocked_by": []
    }
  ]
}

Rules:
- Each task should be independently implementable in its own git branch
- Use blocked_by to express dependencies (array of task IDs)
- Tasks with no blocked_by can run in parallel immediately
- Keep tasks focused: one module/feature per task
- Include verification/testing as separate tasks where appropriate

User request: %s`, prompt)

	cmd := exec.Command("claude", "--print", plannerPrompt)
	cmd.Dir = projectDir
	cmd.Env = filterEnv(os.Environ(), "CLAUDECODE")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("claude decompose: %w\nstderr: %s", err, stderr.String())
	}

	// Extract JSON block from Claude's text output.
	jsonBytes := jsonBlockRe.Find(output)
	if jsonBytes == nil {
		return nil, fmt.Errorf("no JSON block found in claude output: %s", output)
	}

	var resp decompositionResponse
	if err := json.Unmarshal(jsonBytes, &resp); err != nil {
		return nil, fmt.Errorf("parse decomposition JSON: %w", err)
	}

	return buildDAG(resp.Tasks), nil
}

// buildDAG converts a flat task list with blocked_by fields into a DAG with edges.
func buildDAG(tasks []Task) *DAG {
	d := &DAG{Tasks: tasks}
	for _, t := range tasks {
		for _, dep := range t.BlockedBy {
			d.Edges = append(d.Edges, Edge{From: dep, To: t.ID})
		}
	}
	for i := range d.Tasks {
		d.Tasks[i].Status = "pending"
	}
	return d
}
