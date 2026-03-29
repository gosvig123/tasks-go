package task

import (
	"testing"
)

func TestAddSubtask(t *testing.T) {
	list := NewTaskList("test")
	parent := &Task{Content: "Parent"}
	list.Add(parent)

	sub := &Task{Content: "Subtask"}
	result := list.AddSubtask(0, sub)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(parent.Subtasks) != 1 {
		t.Fatalf("expected 1 subtask, got %d", len(parent.Subtasks))
	}
	if parent.Subtasks[0].Parent != parent {
		t.Error("expected subtask Parent to point to parent")
	}
}

func TestAddSubtaskInvalidIndex(t *testing.T) {
	list := NewTaskList("test")
	sub := &Task{Content: "Subtask"}
	result := list.AddSubtask(99, sub)
	if result != nil {
		t.Error("expected nil for invalid parent index")
	}
}

func TestDeleteSubtask(t *testing.T) {
	list := NewTaskList("test")
	parent := &Task{Content: "Parent"}
	list.Add(parent)
	parent.Subtasks = []*Task{
		{Content: "Sub1", Parent: parent},
		{Content: "Sub2", Parent: parent},
	}

	deleted := list.DeleteSubtask(0, 0)
	if deleted == nil || deleted.Content != "Sub1" {
		t.Error("expected to delete Sub1")
	}
	if len(parent.Subtasks) != 1 {
		t.Errorf("expected 1 remaining subtask, got %d", len(parent.Subtasks))
	}
}

func TestToggleSubtaskAutoComplete(t *testing.T) {
	list := NewTaskList("test")
	parent := &Task{Content: "Parent"}
	list.Add(parent)
	parent.Subtasks = []*Task{
		{Content: "Sub1", Completed: true, Parent: parent},
		{Content: "Sub2", Completed: false, Parent: parent},
	}

	// Toggle the last incomplete subtask
	list.ToggleSubtask(0, 1)

	if !parent.Subtasks[1].Completed {
		t.Error("expected Sub2 to be completed")
	}
	if !parent.Completed {
		t.Error("expected parent to auto-complete when all subtasks completed")
	}
}

func TestToggleParentCascade(t *testing.T) {
	list := NewTaskList("test")
	parent := &Task{Content: "Parent"}
	list.Add(parent)
	parent.Subtasks = []*Task{
		{Content: "Sub1", Completed: false, Parent: parent},
		{Content: "Sub2", Completed: false, Parent: parent},
	}

	// Toggle parent to completed — should cascade
	list.Toggle(0)

	if !parent.Completed {
		t.Error("expected parent to be completed")
	}
	for i, sub := range parent.Subtasks {
		if !sub.Completed {
			t.Errorf("expected subtask %d to be completed via cascade", i)
		}
	}
}

func TestPendingCountWithSubtasks(t *testing.T) {
	list := NewTaskList("test")
	parent := &Task{Content: "Parent", Subtasks: []*Task{
		{Content: "Sub1", Completed: false},
		{Content: "Sub2", Completed: true},
	}}
	list.Add(parent)

	// 1 parent pending + 1 subtask pending = 2
	if list.PendingCount() != 2 {
		t.Errorf("expected 2 pending, got %d", list.PendingCount())
	}
	// 1 subtask completed = 1
	if list.CompletedCount() != 1 {
		t.Errorf("expected 1 completed, got %d", list.CompletedCount())
	}
}

func TestToggleSetsCompletedAt(t *testing.T) {
	list := NewTaskList("test")
	list.Add(&Task{Content: "Do thing"})

	toggled := list.Toggle(0)

	if toggled == nil {
		t.Fatal("expected toggled task")
	}
	if !toggled.Completed {
		t.Error("expected task to be completed")
	}
	if toggled.CompletedAt == nil {
		t.Error("expected CompletedAt to be set when completing")
	}
}

func TestToggleClearsCompletedAt(t *testing.T) {
	list := NewTaskList("test")
	list.Add(&Task{Content: "Do thing", Completed: true})

	toggled := list.Toggle(0)

	if toggled.Completed {
		t.Error("expected task to be incomplete")
	}
	if toggled.CompletedAt != nil {
		t.Error("expected CompletedAt to be cleared when un-completing")
	}
}

func TestToggleSubtaskSetsCompletedAt(t *testing.T) {
	list := NewTaskList("test")
	parent := &Task{Content: "Parent"}
	list.Add(parent)
	parent.Subtasks = []*Task{
		{Content: "Sub1", Completed: false, Parent: parent},
	}

	sub := list.ToggleSubtask(0, 0)

	if sub == nil {
		t.Fatal("expected toggled subtask")
	}
	if sub.CompletedAt == nil {
		t.Error("expected CompletedAt to be set on subtask when completing")
	}
}

func TestToggleCascadeSetsCompletedAt(t *testing.T) {
	list := NewTaskList("test")
	parent := &Task{Content: "Parent"}
	list.Add(parent)
	parent.Subtasks = []*Task{
		{Content: "Sub1", Completed: false, Parent: parent},
		{Content: "Sub2", Completed: false, Parent: parent},
	}

	list.Toggle(0)

	for i, sub := range parent.Subtasks {
		if sub.CompletedAt == nil {
			t.Errorf("expected CompletedAt on subtask %d after cascade complete", i)
		}
	}
}

func TestToggleParentToIncompleteNoReverse(t *testing.T) {
	list := NewTaskList("test")
	parent := &Task{Content: "Parent", Completed: true}
	list.Add(parent)
	parent.Subtasks = []*Task{
		{Content: "Sub1", Completed: true, Parent: parent},
		{Content: "Sub2", Completed: true, Parent: parent},
	}

	// Toggle parent back to incomplete — subtasks should stay completed
	list.Toggle(0)

	if parent.Completed {
		t.Error("expected parent to be incomplete")
	}
	for i, sub := range parent.Subtasks {
		if !sub.Completed {
			t.Errorf("expected subtask %d to remain completed", i)
		}
	}
}
