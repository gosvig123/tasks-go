package task

import (
	"strings"
	"testing"
	"time"
)

func TestIDRoundTrip(t *testing.T) {
	id := "550e8400-e29b-41d4-a716-446655440000"
	parsed, err := Parse("[] Buy milk @id:" + id)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.ID != id {
		t.Fatalf("expected %s, got %s", id, parsed.ID)
	}
	if !strings.Contains(parsed.String(), "@id:"+id) {
		t.Fatalf("serialized task lost ID: %s", parsed.String())
	}
}

func TestReferenceKeepsTaskIDs(t *testing.T) {
	parent := &Task{ID: NewID(), Content: "Project"}
	parent.Subtasks = []*Task{{ID: NewID(), Content: "Draft"}}
	stub := NewReferenceStub(parent, "work")
	if stub.ID != parent.ID {
		t.Fatal("reference did not keep parent ID")
	}
	if stub.Subtasks[0].ID != parent.Subtasks[0].ID {
		t.Fatal("reference did not keep subtask ID")
	}
}

func TestResolveFromClearsScheduleMetadata(t *testing.T) {
	estimate := time.Hour
	reference := &Task{Content: "Plan", Estimate: &estimate, StartTime: &TimeOfDay{Hour: 9}}
	reference.ResolveFrom(&Task{Content: "Plan"})
	if reference.Estimate != nil || reference.StartTime != nil {
		t.Fatal("source clears did not propagate to reference")
	}
}

func TestRecurrenceGetsNewID(t *testing.T) {
	original := &Task{ID: NewID(), Content: "Plan", RecurDays: 1}
	next := original.CreateNextRecurrence()
	if next == nil || next.ID == "" || next.ID == original.ID {
		t.Fatalf("expected a new recurrence ID, got %#v", next)
	}
}
