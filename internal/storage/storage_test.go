package storage

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/krisitan/tasks-go/internal/task"
)

func newTestStorage(t *testing.T) *Storage {
	t.Helper()
	dir := t.TempDir()
	return &Storage{
		TasksDir:        dir,
		CurrentListFile: filepath.Join(dir, ".current-list"),
		LastResetFile:   filepath.Join(dir, ".last-reset"),
		LastSyncFile:    filepath.Join(dir, ".last-sync"),
	}
}

func TestSyncCompletionToSource(t *testing.T) {
	store := newTestStorage(t)

	sourceList := task.NewTaskList("work")
	sourceList.Add(&task.Task{Content: "Buy milk"})
	if err := store.SaveList(sourceList); err != nil {
		t.Fatalf("SaveList(work): %v", err)
	}

	todayList := task.NewTaskList("today")
	todayList.Add(&task.Task{Content: "Buy milk", Source: "work"})
	if err := store.SaveList(todayList); err != nil {
		t.Fatalf("SaveList(today): %v", err)
	}

	todayList.Toggle(0)
	store.SyncCompletionToSource(todayList.Tasks[0])

	reloaded, err := store.loadListRaw("work")
	if err != nil {
		t.Fatalf("loadListRaw(work): %v", err)
	}
	if reloaded.Len() != 1 {
		t.Fatalf("expected 1 task in work, got %d", reloaded.Len())
	}
	if !reloaded.Tasks[0].Completed {
		t.Error("source task should be completed after SyncCompletionToSource")
	}
	if reloaded.Tasks[0].CompletedAt == nil {
		t.Error("source task CompletedAt should be set")
	}
}

func TestSyncCompletionToToday(t *testing.T) {
	store := newTestStorage(t)

	sourceList := task.NewTaskList("work")
	sourceList.Add(&task.Task{Content: "Buy milk"})
	if err := store.SaveList(sourceList); err != nil {
		t.Fatalf("SaveList(work): %v", err)
	}

	todayList := task.NewTaskList("today")
	todayList.Add(&task.Task{Content: "Buy milk", Source: "work"})
	if err := store.SaveList(todayList); err != nil {
		t.Fatalf("SaveList(today): %v", err)
	}

	sourceList.Toggle(0)
	store.SyncCompletionToToday("work", sourceList.Tasks[0])

	reloaded, err := store.loadListRaw("today")
	if err != nil {
		t.Fatalf("loadListRaw(today): %v", err)
	}
	if reloaded.Len() != 1 {
		t.Fatalf("expected 1 task in today, got %d", reloaded.Len())
	}
	if !reloaded.Tasks[0].Completed {
		t.Error("today stub should be completed after SyncCompletionToToday")
	}
	if reloaded.Tasks[0].CompletedAt == nil {
		t.Error("today stub CompletedAt should be set")
	}
}

func TestSyncCompletionToSourceWithSubtasks(t *testing.T) {
	store := newTestStorage(t)

	parent := &task.Task{Content: "Project tasks"}
	parent.Subtasks = []*task.Task{
		{Content: "Design mockup", Parent: parent},
		{Content: "Write tests", Parent: parent},
	}
	sourceList := task.NewTaskList("work")
	sourceList.Add(parent)
	if err := store.SaveList(sourceList); err != nil {
		t.Fatalf("SaveList(work): %v", err)
	}

	todayList := task.NewTaskList("today")
	todayList.Add(task.NewReferenceStub(parent, "work"))
	if err := store.SaveList(todayList); err != nil {
		t.Fatalf("SaveList(today): %v", err)
	}

	todayList.ToggleSubtask(0, 0)
	store.SyncCompletionToSource(todayList.Tasks[0])

	reloaded, err := store.loadListRaw("work")
	if err != nil {
		t.Fatalf("loadListRaw(work): %v", err)
	}
	srcParent := reloaded.Tasks[0]
	if len(srcParent.Subtasks) != 2 {
		t.Fatalf("expected 2 subtasks, got %d", len(srcParent.Subtasks))
	}
	if !srcParent.Subtasks[0].Completed {
		t.Error("first subtask should be completed after sync")
	}
	if srcParent.Subtasks[1].Completed {
		t.Error("second subtask should NOT be completed")
	}
}

func TestSyncCompletionRoundTrip(t *testing.T) {
	store := newTestStorage(t)

	sourceList := task.NewTaskList("work")
	sourceList.Add(&task.Task{Content: "Buy milk"})
	if err := store.SaveList(sourceList); err != nil {
		t.Fatalf("SaveList(work): %v", err)
	}

	todayList := task.NewTaskList("today")
	todayList.Add(&task.Task{Content: "Buy milk", Source: "work"})
	if err := store.SaveList(todayList); err != nil {
		t.Fatalf("SaveList(today): %v", err)
	}

	todayList.Toggle(0)
	store.SyncCompletionToSource(todayList.Tasks[0])

	if err := store.SaveList(todayList); err != nil {
		t.Fatalf("SaveList(today) after toggle: %v", err)
	}

	workReloaded, err := store.LoadList("work")
	if err != nil {
		t.Fatalf("LoadList(work): %v", err)
	}
	if !workReloaded.Tasks[0].Completed {
		t.Error("source task should be completed after round-trip")
	}

	todayReloaded, err := store.LoadList("today")
	if err != nil {
		t.Fatalf("LoadList(today): %v", err)
	}
	if !todayReloaded.Tasks[0].Completed {
		t.Error("today task should still be completed after LoadList with ResolveReferences")
	}
}

func TestSyncCompletionToSourceContentMismatch(t *testing.T) {
	store := newTestStorage(t)

	sourceList := task.NewTaskList("work")
	sourceList.Add(&task.Task{Content: "Buy milk"})
	if err := store.SaveList(sourceList); err != nil {
		t.Fatalf("SaveList(work): %v", err)
	}

	todayList := task.NewTaskList("today")
	todayList.Add(&task.Task{Content: "Buy  milk", Source: "work"})
	if err := store.SaveList(todayList); err != nil {
		t.Fatalf("SaveList(today): %v", err)
	}

	todayList.Toggle(0)
	store.SyncCompletionToSource(todayList.Tasks[0])

	reloaded, err := store.loadListRaw("work")
	if err != nil {
		t.Fatalf("loadListRaw(work): %v", err)
	}
	if reloaded.Tasks[0].Completed {
		t.Error("source task should NOT be completed when content doesn't match")
	}
}

func TestSyncCompletionToSourceUncomplete(t *testing.T) {
	store := newTestStorage(t)

	sourceList := task.NewTaskList("work")
	sourceList.Add(&task.Task{Content: "Buy milk"})
	if err := store.SaveList(sourceList); err != nil {
		t.Fatalf("SaveList(work): %v", err)
	}

	todayList := task.NewTaskList("today")
	todayList.Add(&task.Task{Content: "Buy milk", Source: "work"})
	if err := store.SaveList(todayList); err != nil {
		t.Fatalf("SaveList(today): %v", err)
	}

	todayList.Toggle(0)
	store.SyncCompletionToSource(todayList.Tasks[0])

	afterComplete, err := store.loadListRaw("work")
	if err != nil {
		t.Fatalf("loadListRaw(work) after complete: %v", err)
	}
	if !afterComplete.Tasks[0].Completed {
		t.Fatal("source task should be completed before uncomplete test")
	}

	todayList.Toggle(0)
	store.SyncCompletionToSource(todayList.Tasks[0])

	afterUncomplete, err := store.loadListRaw("work")
	if err != nil {
		t.Fatalf("loadListRaw(work) after uncomplete: %v", err)
	}
	if afterUncomplete.Tasks[0].Completed {
		t.Error("source task should be uncompleted after toggle back")
	}
	if afterUncomplete.Tasks[0].CompletedAt != nil {
		t.Error("source task CompletedAt should be nil after uncomplete")
	}
}

func TestResolveReferencesOverwritesCompletion(t *testing.T) {
	store := newTestStorage(t)

	sourceList := task.NewTaskList("work")
	sourceList.Add(&task.Task{Content: "Buy milk", Completed: false})
	if err := store.SaveList(sourceList); err != nil {
		t.Fatalf("SaveList(work): %v", err)
	}

	doneAt := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	todayList := task.NewTaskList("today")
	todayList.Add(&task.Task{
		Content:     "Buy milk",
		Source:      "work",
		Completed:   true,
		CompletedAt: &doneAt,
	})
	if err := store.SaveList(todayList); err != nil {
		t.Fatalf("SaveList(today): %v", err)
	}

	reloaded, err := store.LoadList("today")
	if err != nil {
		t.Fatalf("LoadList(today): %v", err)
	}
	if reloaded.Len() != 1 {
		t.Fatalf("expected 1 task in today, got %d", reloaded.Len())
	}
	if reloaded.Tasks[0].Completed {
		t.Error("today stub should be UNCOMPLETED after ResolveReferences (source wins)")
	}
	if reloaded.Tasks[0].CompletedAt != nil {
		t.Error("today stub CompletedAt should be nil after ResolveReferences")
	}
}
