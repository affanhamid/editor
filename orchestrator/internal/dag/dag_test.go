package dag

import (
	"testing"
)

func TestReadyTasks_AllPending(t *testing.T) {
	d := &DAG{
		Tasks: []Task{
			{ID: 1, Title: "task1", Status: "pending"},
			{ID: 2, Title: "task2", Status: "pending"},
		},
	}
	ready := d.ReadyTasks()
	if len(ready) != 2 {
		t.Fatalf("expected 2 ready tasks, got %d", len(ready))
	}
}

func TestReadyTasks_WithDependencies(t *testing.T) {
	d := &DAG{
		Tasks: []Task{
			{ID: 1, Title: "task1", Status: "pending"},
			{ID: 2, Title: "task2", Status: "pending", BlockedBy: []int64{1}},
			{ID: 3, Title: "task3", Status: "pending", BlockedBy: []int64{1, 2}},
		},
		Edges: []Edge{
			{From: 1, To: 2},
			{From: 1, To: 3},
			{From: 2, To: 3},
		},
	}
	ready := d.ReadyTasks()
	if len(ready) != 1 {
		t.Fatalf("expected 1 ready task, got %d", len(ready))
	}
	if ready[0].ID != 1 {
		t.Fatalf("expected task 1 to be ready, got task %d", ready[0].ID)
	}
}

func TestReadyTasks_AfterCompletion(t *testing.T) {
	d := &DAG{
		Tasks: []Task{
			{ID: 1, Title: "task1", Status: "completed"},
			{ID: 2, Title: "task2", Status: "pending", BlockedBy: []int64{1}},
			{ID: 3, Title: "task3", Status: "pending", BlockedBy: []int64{1, 2}},
		},
	}
	ready := d.ReadyTasks()
	if len(ready) != 1 {
		t.Fatalf("expected 1 ready task, got %d", len(ready))
	}
	if ready[0].ID != 2 {
		t.Fatalf("expected task 2 to be ready, got task %d", ready[0].ID)
	}
}

func TestReadyTasks_SkipsAssigned(t *testing.T) {
	d := &DAG{
		Tasks: []Task{
			{ID: 1, Title: "task1", Status: "pending", AssignedTo: "agent-123"},
			{ID: 2, Title: "task2", Status: "pending"},
		},
	}
	ready := d.ReadyTasks()
	if len(ready) != 1 {
		t.Fatalf("expected 1 ready task, got %d", len(ready))
	}
	if ready[0].ID != 2 {
		t.Fatalf("expected task 2 to be ready, got task %d", ready[0].ID)
	}
}

func TestBuildDAG(t *testing.T) {
	tasks := []Task{
		{ID: 1, Title: "a", BlockedBy: nil},
		{ID: 2, Title: "b", BlockedBy: []int64{1}},
		{ID: 3, Title: "c", BlockedBy: []int64{1, 2}},
	}
	d := buildDAG(tasks)

	if len(d.Tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(d.Tasks))
	}
	if len(d.Edges) != 3 {
		t.Fatalf("expected 3 edges, got %d", len(d.Edges))
	}
	for _, task := range d.Tasks {
		if task.Status != "pending" {
			t.Fatalf("expected all tasks to be pending, got %q for task %d", task.Status, task.ID)
		}
	}
}
