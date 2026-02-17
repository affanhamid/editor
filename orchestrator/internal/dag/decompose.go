package dag

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

// decompositionResponse is the expected JSON output from Claude Code.
type decompositionResponse struct {
	Tasks []Task `json:"tasks"`
}

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

	cmd := exec.Command("claude",
		"--print",
		"--output-format", "json",
		"--project", projectDir,
		"--prompt", plannerPrompt,
	)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("claude decompose: %w", err)
	}

	// Claude --output-format json wraps the response; extract the text content.
	var claudeResp struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal(output, &claudeResp); err != nil {
		// Fall back to treating the whole output as the JSON response.
		claudeResp.Result = string(output)
	}

	var resp decompositionResponse
	if err := json.Unmarshal([]byte(claudeResp.Result), &resp); err != nil {
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
		// Set initial status.
		t.Status = "pending"
	}
	// Ensure all tasks start as pending.
	for i := range d.Tasks {
		d.Tasks[i].Status = "pending"
	}
	return d
}
