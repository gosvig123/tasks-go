package storage

import (
	"os"
	"strings"
	"testing"

	"github.com/krisitan/tasks-go/internal/task"
)

func TestMigrateIDsDryRunAndApply(t *testing.T) {
	store := migrationStore(t)
	before, _ := os.ReadFile(store.ListPath("work"))
	dry, err := store.MigrateIDs(true)
	if err != nil || dry.Assigned != 2 {
		t.Fatalf("dry run: report=%+v err=%v", dry, err)
	}
	afterDry, _ := os.ReadFile(store.ListPath("work"))
	if string(before) != string(afterDry) {
		t.Fatal("dry run changed the task file")
	}
	applied, err := store.MigrateIDs(false)
	if err != nil {
		t.Fatal(err)
	}
	if applied.Backup == "" {
		t.Fatal("migration did not create a backup")
	}
	assertMigratedReference(t, store)
}

func migrationStore(t *testing.T) *Storage {
	t.Helper()
	store := newTestStorage(t)
	work := task.NewTaskList("work")
	work.Add(&task.Task{Content: "Buy milk", Subtasks: []*task.Task{{Content: "Choose brand"}}})
	if err := store.SaveList(work); err != nil {
		t.Fatal(err)
	}
	today := task.NewTaskList("today")
	today.Add(&task.Task{Content: "Buy milk", Source: "work", Subtasks: []*task.Task{{Content: "Choose brand"}}})
	if err := store.SaveList(today); err != nil {
		t.Fatal(err)
	}
	return store
}

func assertMigratedReference(t *testing.T, store *Storage) {
	t.Helper()
	work, err := store.LoadList("work")
	if err != nil {
		t.Fatal(err)
	}
	today, err := store.LoadList("today")
	if err != nil {
		t.Fatal(err)
	}
	if work.Tasks[0].ID == "" || work.Tasks[0].Subtasks[0].ID == "" {
		t.Fatal("source IDs were not assigned")
	}
	if today.Tasks[0].ID != work.Tasks[0].ID {
		t.Fatal("Today reference did not inherit source ID")
	}
}

func TestMigrateIDsPreservesNonTaskLines(t *testing.T) {
	store := newTestStorage(t)
	content := "# Goals\n\n## Health\n[] Run a race\n"
	if err := os.WriteFile(store.ListPath("goals"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.MigrateIDs(false); err != nil {
		t.Fatal(err)
	}
	migrated, err := os.ReadFile(store.ListPath("goals"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(migrated)
	if !strings.Contains(text, "# Goals\n\n## Health\n") || !strings.Contains(text, "@id:") {
		t.Fatalf("non-task lines or ID missing: %q", text)
	}
}

func TestMigrateIDsRejectsAmbiguousReference(t *testing.T) {
	store := newTestStorage(t)
	work := task.NewTaskList("work")
	work.Add(&task.Task{Content: "Duplicate"})
	work.Add(&task.Task{Content: "Duplicate"})
	if err := store.SaveList(work); err != nil {
		t.Fatal(err)
	}
	today := task.NewTaskList("today")
	today.Add(&task.Task{Content: "Duplicate", Source: "work"})
	if err := store.SaveList(today); err != nil {
		t.Fatal(err)
	}
	if _, err := store.MigrateIDs(true); err == nil {
		t.Fatal("expected ambiguous reference error")
	}
}
