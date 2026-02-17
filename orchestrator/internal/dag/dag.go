package dag

// Task represents a single unit of work in the DAG.
type Task struct {
	ID          int64   `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	RiskLevel   string  `json:"risk_level"`
	BlockedBy   []int64 `json:"blocked_by"`
	AssignedTo  string  `json:"-"`
	Status      string  `json:"-"`
}

// Edge represents a dependency between two tasks.
type Edge struct {
	From int64
	To   int64
}

// DAG holds the full task graph.
type DAG struct {
	Tasks []Task
	Edges []Edge
}

// ReadyTasks returns tasks that have no unfinished blockers and are unassigned.
func (d *DAG) ReadyTasks() []Task {
	completed := make(map[int64]bool)
	for _, t := range d.Tasks {
		if t.Status == "completed" {
			completed[t.ID] = true
		}
	}

	var ready []Task
	for _, t := range d.Tasks {
		if t.Status != "pending" || t.AssignedTo != "" {
			continue
		}
		blocked := false
		for _, dep := range t.BlockedBy {
			if !completed[dep] {
				blocked = true
				break
			}
		}
		if !blocked {
			ready = append(ready, t)
		}
	}
	return ready
}
