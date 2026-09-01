package storage

import (
	"testing"
	"time"

	"github.com/krisitan/tasks-go/internal/task"
)

func TestRecurringReferenceIgnoresHistoricalCompletion(t *testing.T) {
	store := newTestStorage(t)
	oldDue := time.Now().AddDate(0, 0, -7)
	currentDue := time.Now()
	historical := &task.Task{ID: task.NewID(), Content: "Standup", Completed: true, DueDate: &oldDue, RecurDays: 7}
	current := &task.Task{ID: task.NewID(), Content: "Standup", DueDate: &currentDue, RecurDays: 7}
	work := task.NewTaskList("work")
	work.Add(historical)
	work.Add(current)
	if err := store.SaveList(work); err != nil {
		t.Fatal(err)
	}
	today := task.NewTaskList("today")
	today.Add(task.NewReferenceStub(current, "work"))
	if err := store.SaveList(today); err != nil {
		t.Fatal(err)
	}
	_, err := store.ToggleTask(TaskTarget{ID: current.ID, ListName: "today", Index: -1, SubIndex: -1})
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := store.loadListRaw("work")
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Len() != 3 {
		t.Fatalf("expected next occurrence, got %d tasks", reloaded.Len())
	}
}
