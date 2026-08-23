package storage

import (
	"os"
	"strings"
	"testing"

	"github.com/krisitan/tasks-go/internal/task"
)

func TestSaveListPreservesMarkdownStructure(t *testing.T) {
	store := newTestStorage(t)
	firstID, secondID := task.NewID(), task.NewID()
	content := "# Goals\n\n## Health\n[] Run @id:" + firstID + "\n[] Rest @id:" + secondID + "\n"
	if err := os.WriteFile(store.ListPath("goals"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	list, err := store.LoadList("goals")
	if err != nil {
		t.Fatal(err)
	}
	list.Tasks[0].Content = "Run faster"
	list.Delete(1)
	list.Add(&task.Task{ID: task.NewID(), Content: "Recover"})
	if err := store.SaveList(list); err != nil {
		t.Fatal(err)
	}
	assertStructuredResult(t, store.ListPath("goals"))
}

func TestSaveListLockedRejectsStaleWrite(t *testing.T) {
	store := newTestStorage(t)
	list := task.NewTaskList("work")
	list.Add(&task.Task{ID: task.NewID(), Content: "Original"})
	if err := store.SaveList(list); err != nil {
		t.Fatal(err)
	}
	first, _ := store.LoadList("work")
	stale, _ := store.LoadList("work")
	first.Tasks[0].Content = "First edit"
	if err := store.SaveListLocked(first); err != nil {
		t.Fatal(err)
	}
	stale.Tasks[0].Content = "Stale edit"
	if err := store.SaveListLocked(stale); err == nil {
		t.Fatal("expected stale write rejection")
	}
}

func assertStructuredResult(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, expected := range []string{"# Goals", "## Health", "Run faster", "Recover"} {
		if !strings.Contains(content, expected) {
			t.Fatalf("missing %q in %q", expected, content)
		}
	}
	if strings.Contains(content, "[] Rest") {
		t.Fatalf("deleted task remains in %q", content)
	}
}
