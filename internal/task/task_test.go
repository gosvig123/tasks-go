package task

import (
	"strings"
	"testing"
	"time"
)

func TestTaskSubtasksField(t *testing.T) {
	parent := &Task{Content: "Parent task"}
	child := &Task{Content: "Child task", Parent: parent}
	parent.Subtasks = append(parent.Subtasks, child)

	if len(parent.Subtasks) != 1 {
		t.Errorf("expected 1 subtask, got %d", len(parent.Subtasks))
	}
	if parent.Subtasks[0].Content != "Child task" {
		t.Errorf("expected subtask content 'Child task', got '%s'", parent.Subtasks[0].Content)
	}
	if child.Parent != parent {
		t.Error("expected child.Parent to point to parent")
	}
}

func TestAllSubtasksCompleted(t *testing.T) {
	t.Run("no subtasks returns true", func(t *testing.T) {
		parent := &Task{Content: "Parent"}
		if !parent.AllSubtasksCompleted() {
			t.Error("expected true when no subtasks")
		}
	})

	t.Run("all completed returns true", func(t *testing.T) {
		parent := &Task{Content: "Parent", Subtasks: []*Task{
			{Content: "Sub1", Completed: true},
			{Content: "Sub2", Completed: true},
		}}
		if !parent.AllSubtasksCompleted() {
			t.Error("expected true when all subtasks completed")
		}
	})

	t.Run("some incomplete returns false", func(t *testing.T) {
		parent := &Task{Content: "Parent", Subtasks: []*Task{
			{Content: "Sub1", Completed: true},
			{Content: "Sub2", Completed: false},
		}}
		if parent.AllSubtasksCompleted() {
			t.Error("expected false when some subtasks incomplete")
		}
	})
}

func TestStringWithSubtasks(t *testing.T) {
	parent := &Task{Content: "Write proposal"}
	parent.Subtasks = []*Task{
		{Content: "Research competitors"},
		{Content: "Draft outline", Completed: true},
	}

	result := parent.String()

	if !strings.HasPrefix(result, "[] Write proposal") {
		t.Errorf("expected parent line, got: %s", result)
	}
	if !strings.Contains(result, "\n  [] Research competitors") {
		t.Errorf("expected indented uncompleted subtask in output: %s", result)
	}
	if !strings.Contains(result, "\n  [x] Draft outline") {
		t.Errorf("expected indented completed subtask in output: %s", result)
	}
}

func TestStringWithoutSubtasks(t *testing.T) {
	task := &Task{Content: "Simple task"}
	result := task.String()
	expected := "[] Simple task"
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}

func TestStubStringWithSubtasks(t *testing.T) {
	parent := &Task{Content: "Write proposal", Source: "work"}
	parent.Subtasks = []*Task{
		{Content: "Research", Source: "work"},
		{Content: "Draft", Source: "work", Completed: true},
	}

	result := parent.StubString()

	if !strings.HasPrefix(result, "[] Write proposal @source:work") {
		t.Errorf("expected parent stub line, got: %s", result)
	}
	if !strings.Contains(result, "\n  [] Research @source:work") {
		t.Errorf("expected indented subtask stub: %s", result)
	}
	if !strings.Contains(result, "\n  [x] Draft @source:work") {
		t.Errorf("expected indented completed subtask stub: %s", result)
	}
}

func TestParseLines(t *testing.T) {
	t.Run("basic subtasks", func(t *testing.T) {
		lines := []string{
			"[] Write proposal @est:2h",
			"  [] Research competitors @est:45m",
			"  [x] Draft outline",
			"[] Buy groceries",
		}

		tasks := ParseLines(lines)
		if len(tasks) != 2 {
			t.Fatalf("expected 2 top-level tasks, got %d", len(tasks))
		}

		parent := tasks[0]
		if parent.Content != "Write proposal" {
			t.Errorf("expected 'Write proposal', got '%s'", parent.Content)
		}
		if len(parent.Subtasks) != 2 {
			t.Fatalf("expected 2 subtasks, got %d", len(parent.Subtasks))
		}
		if parent.Subtasks[0].Content != "Research competitors" {
			t.Errorf("expected subtask 'Research competitors', got '%s'", parent.Subtasks[0].Content)
		}
		if parent.Subtasks[0].Parent != parent {
			t.Error("expected subtask Parent to point to parent task")
		}
		if !parent.Subtasks[1].Completed {
			t.Error("expected second subtask to be completed")
		}

		if tasks[1].Content != "Buy groceries" {
			t.Errorf("expected 'Buy groceries', got '%s'", tasks[1].Content)
		}
		if len(tasks[1].Subtasks) != 0 {
			t.Errorf("expected no subtasks on second task, got %d", len(tasks[1].Subtasks))
		}
	})

	t.Run("orphan indented line becomes top-level", func(t *testing.T) {
		lines := []string{
			"  [] Orphan subtask",
			"[] Normal task",
		}

		tasks := ParseLines(lines)
		if len(tasks) != 2 {
			t.Fatalf("expected 2 top-level tasks, got %d", len(tasks))
		}
		if tasks[0].Content != "Orphan subtask" {
			t.Errorf("expected orphan as top-level, got '%s'", tasks[0].Content)
		}
	})

	t.Run("deep indent treated as top-level", func(t *testing.T) {
		lines := []string{
			"[] Parent task",
			"    [] Deeply indented task",
		}

		tasks := ParseLines(lines)
		if len(tasks) != 2 {
			t.Fatalf("expected 2 top-level tasks, got %d", len(tasks))
		}
		if len(tasks[0].Subtasks) != 0 {
			t.Errorf("expected no subtasks on parent, got %d", len(tasks[0].Subtasks))
		}
	})

	t.Run("empty lines skipped", func(t *testing.T) {
		lines := []string{
			"[] Task one",
			"",
			"  [] Subtask one",
			"[] Task two",
		}

		tasks := ParseLines(lines)
		if len(tasks) != 2 {
			t.Fatalf("expected 2 top-level tasks, got %d", len(tasks))
		}
		if len(tasks[0].Subtasks) != 1 {
			t.Fatalf("expected 1 subtask, got %d", len(tasks[0].Subtasks))
		}
	})
}

func TestResolveFromWithSubtasks(t *testing.T) {
	est := 45 * time.Minute
	source := &Task{
		Content: "Write proposal",
		Subtasks: []*Task{
			{Content: "Research", Estimate: &est},
			{Content: "Draft"},
		},
	}

	stub := &Task{
		Content: "Write proposal",
		Source:  "work",
		Subtasks: []*Task{
			{Content: "Research", Source: "work"},
			{Content: "Draft", Source: "work"},
		},
	}

	stub.ResolveFrom(source)

	if stub.Subtasks[0].Estimate == nil {
		t.Error("expected subtask estimate to be resolved from source")
	}
	if *stub.Subtasks[0].Estimate != est {
		t.Errorf("expected 45m estimate, got %v", *stub.Subtasks[0].Estimate)
	}
}
