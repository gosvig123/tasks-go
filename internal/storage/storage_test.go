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

func TestSyncSubtasksToSourceAddsNewSubtask(t *testing.T) {
	store := newTestStorage(t)

	parent := &task.Task{Content: "Project tasks"}
	sourceList := task.NewTaskList("work")
	sourceList.Add(parent)
	if err := store.SaveList(sourceList); err != nil {
		t.Fatalf("SaveList(work): %v", err)
	}

	todayList := task.NewTaskList("today")
	stub := task.NewReferenceStub(parent, "work")
	todayList.Add(stub)
	if err := store.SaveList(todayList); err != nil {
		t.Fatalf("SaveList(today): %v", err)
	}

	now := time.Now()
	stub.Subtasks = append(stub.Subtasks, &task.Task{
		Content:   "Design mockup",
		CreatedAt: &now,
		Parent:    stub,
		Source:    "work",
	})
	store.SyncSubtasksToSource(stub)

	reloaded, err := store.LoadList("work")
	if err != nil {
		t.Fatalf("LoadList(work): %v", err)
	}
	if len(reloaded.Tasks[0].Subtasks) != 1 {
		t.Fatalf("expected 1 subtask in source, got %d", len(reloaded.Tasks[0].Subtasks))
	}
	if reloaded.Tasks[0].Subtasks[0].Content != "Design mockup" {
		t.Errorf("expected synced subtask content, got %q", reloaded.Tasks[0].Subtasks[0].Content)
	}
}

func TestSyncCompletionToTodayAddsMissingSubtask(t *testing.T) {
	store := newTestStorage(t)

	parent := &task.Task{Content: "Project tasks"}
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

	sourceList.Tasks[0].Subtasks = append(sourceList.Tasks[0].Subtasks, &task.Task{
		Content: "Write tests",
		Parent:  sourceList.Tasks[0],
	})
	if err := store.SaveList(sourceList); err != nil {
		t.Fatalf("SaveList(work) after subtask add: %v", err)
	}
	store.SyncCompletionToToday("work", sourceList.Tasks[0])

	reloaded, err := store.LoadList("today")
	if err != nil {
		t.Fatalf("LoadList(today): %v", err)
	}
	if len(reloaded.Tasks[0].Subtasks) != 1 {
		t.Fatalf("expected 1 subtask on today stub, got %d", len(reloaded.Tasks[0].Subtasks))
	}
	if reloaded.Tasks[0].Subtasks[0].Content != "Write tests" {
		t.Errorf("expected propagated subtask, got %q", reloaded.Tasks[0].Subtasks[0].Content)
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

func TestPruneCompletedTasksBefore(t *testing.T) {
	store := newTestStorage(t)
	now := time.Date(2026, 4, 15, 9, 0, 0, 0, time.Local)
	oldDone := time.Date(2026, 4, 1, 18, 30, 0, 0, time.Local)
	recentDone := time.Date(2026, 4, 2, 8, 0, 0, 0, time.Local)

	work := task.NewTaskList("work")
	work.Add(&task.Task{Content: "old completed", Completed: true, CompletedAt: &oldDone})
	work.Add(&task.Task{Content: "recent completed", Completed: true, CompletedAt: &recentDone})
	work.Add(&task.Task{Content: "pending task"})
	work.Add(&task.Task{Content: "legacy completed", Completed: true})
	if err := store.SaveList(work); err != nil {
		t.Fatalf("SaveList(work): %v", err)
	}

	today := task.NewTaskList("today")
	today.Add(&task.Task{Content: "today completed", Completed: true, CompletedAt: &oldDone})
	if err := store.SaveList(today); err != nil {
		t.Fatalf("SaveList(today): %v", err)
	}

	pruned, err := store.pruneCompletedTasksBefore(now)
	if err != nil {
		t.Fatalf("pruneCompletedTasksBefore: %v", err)
	}
	if pruned != 1 {
		t.Fatalf("expected 1 pruned task, got %d", pruned)
	}

	workReloaded, err := store.loadListRaw("work")
	if err != nil {
		t.Fatalf("loadListRaw(work): %v", err)
	}
	if workReloaded.Len() != 3 {
		t.Fatalf("expected 3 tasks in work after prune, got %d", workReloaded.Len())
	}
	if workReloaded.Tasks[0].Content != "recent completed" {
		t.Errorf("expected recent completed first, got %q", workReloaded.Tasks[0].Content)
	}
	if workReloaded.Tasks[1].Content != "pending task" {
		t.Errorf("expected pending task kept, got %q", workReloaded.Tasks[1].Content)
	}
	if workReloaded.Tasks[2].Content != "legacy completed" {
		t.Errorf("expected completed task without CompletedAt kept, got %q", workReloaded.Tasks[2].Content)
	}

	todayReloaded, err := store.loadListRaw("today")
	if err != nil {
		t.Fatalf("loadListRaw(today): %v", err)
	}
	if todayReloaded.Len() != 1 {
		t.Fatalf("expected today list to be skipped, got %d tasks", todayReloaded.Len())
	}
	if todayReloaded.Tasks[0].Content != "today completed" {
		t.Errorf("expected today task unchanged, got %q", todayReloaded.Tasks[0].Content)
	}
}

func TestToggleTaskTodayReferenceRecursBeforeSourceSync(t *testing.T) {
	store := newTestStorage(t)
	work := task.NewTaskList("work")
	work.Add(&task.Task{Content: "Standup", RecurDays: 7})
	if err := store.SaveList(work); err != nil {
		t.Fatalf("SaveList(work): %v", err)
	}

	today := task.NewTaskList("today")
	today.Add(task.NewReferenceStub(work.Tasks[0], "work"))
	if err := store.SaveList(today); err != nil {
		t.Fatalf("SaveList(today): %v", err)
	}

	result, err := store.ToggleTask(TaskTarget{ListName: "today", Index: 0, SubIndex: -1, Content: "Standup"})
	if err != nil {
		t.Fatalf("ToggleTask(today): %v", err)
	}
	if result.RecurrenceList != "work" || result.SyncedToSource != "work" {
		t.Fatalf("expected recurrence and source sync to work, got %#v", result)
	}

	reloaded, err := store.loadListRaw("work")
	if err != nil {
		t.Fatalf("loadListRaw(work): %v", err)
	}
	if reloaded.Len() != 2 {
		t.Fatalf("expected original plus recurrence, got %d", reloaded.Len())
	}
	if !reloaded.Tasks[0].Completed || reloaded.Tasks[1].Completed {
		t.Fatalf("expected completed original and pending recurrence")
	}
}

func TestToggleTaskSubtaskSyncsParentToSource(t *testing.T) {
	store := newTestStorage(t)
	parent := &task.Task{Content: "Project"}
	parent.Subtasks = []*task.Task{{Content: "Draft", Parent: parent}}
	work := task.NewTaskList("work")
	work.Add(parent)
	if err := store.SaveList(work); err != nil {
		t.Fatalf("SaveList(work): %v", err)
	}

	today := task.NewTaskList("today")
	today.Add(task.NewReferenceStub(parent, "work"))
	if err := store.SaveList(today); err != nil {
		t.Fatalf("SaveList(today): %v", err)
	}

	_, err := store.ToggleTask(TaskTarget{ListName: "today", Index: 0, SubIndex: 0, IsSubtask: true, Content: "Draft"})
	if err != nil {
		t.Fatalf("ToggleTask(subtask): %v", err)
	}

	reloaded, err := store.loadListRaw("work")
	if err != nil {
		t.Fatalf("loadListRaw(work): %v", err)
	}
	if !reloaded.Tasks[0].Subtasks[0].Completed {
		t.Fatal("expected source subtask completed")
	}
}

func TestDeleteTaskTodayReferenceOnlyRemovesStub(t *testing.T) {
	store := newTestStorage(t)
	work := task.NewTaskList("work")
	work.Add(&task.Task{Content: "Buy milk"})
	if err := store.SaveList(work); err != nil {
		t.Fatalf("SaveList(work): %v", err)
	}

	today := task.NewTaskList("today")
	today.Add(task.NewReferenceStub(work.Tasks[0], "work"))
	if err := store.SaveList(today); err != nil {
		t.Fatalf("SaveList(today): %v", err)
	}

	if _, err := store.DeleteTask(TaskTarget{ListName: "today", Index: 0, SubIndex: -1, Content: "Buy milk"}); err != nil {
		t.Fatalf("DeleteTask(today): %v", err)
	}

	workReloaded, err := store.loadListRaw("work")
	if err != nil {
		t.Fatalf("loadListRaw(work): %v", err)
	}
	todayReloaded, err := store.loadListRaw("today")
	if err != nil {
		t.Fatalf("loadListRaw(today): %v", err)
	}
	if workReloaded.Len() != 1 || todayReloaded.Len() != 0 {
		t.Fatalf("expected source kept and today cleared, got work=%d today=%d", workReloaded.Len(), todayReloaded.Len())
	}
}

func TestTaskMutationContentFallback(t *testing.T) {
	store := newTestStorage(t)
	work := task.NewTaskList("work")
	work.Add(&task.Task{Content: "Upcoming"})
	if err := store.SaveList(work); err != nil {
		t.Fatalf("SaveList(work): %v", err)
	}

	_, err := store.ToggleTask(TaskTarget{ListName: "work", Index: -1, SubIndex: -1, Content: "Upcoming"})
	if err != nil {
		t.Fatalf("ToggleTask(content fallback): %v", err)
	}

	reloaded, err := store.loadListRaw("work")
	if err != nil {
		t.Fatalf("loadListRaw(work): %v", err)
	}
	if !reloaded.Tasks[0].Completed {
		t.Fatal("expected fallback target toggled")
	}
}
