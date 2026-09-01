package machine

import (
	"os"
	"testing"

	"github.com/krisitan/tasks-go/internal/task"
)

func TestCreateSubtaskKeepsStructuredParent(t *testing.T) {
	store, _ := testStore(t)
	targetID, wrongID := task.NewID(), task.NewID()
	content := "# Gappd\n[] Google Calendar @id:" + targetID +
		"\n[] Diagnose deployment @id:" + wrongID + "\n"
	if err := os.WriteFile(store.ListPath("gappd"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := (&Service{Store: store}).Snapshot("gappd")
	if err != nil {
		t.Fatal(err)
	}
	title := "Release step"
	request := Request{SchemaVersion: 1, Operation: OpSubtaskCreate, TaskID: targetID,
		ExpectedRevision: snapshot.Revision, Changes: TaskChanges{Title: &title}}
	if _, err := (&Service{Store: store}).Execute(request); err != nil {
		t.Fatal(err)
	}
	assertSubtaskParent(t, store, targetID, wrongID)
}

func assertSubtaskParent(t *testing.T, store interface {
	LoadList(string) (*task.TaskList, error)
}, targetID, wrongID string) {
	t.Helper()
	list, err := store.LoadList("gappd")
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range list.Tasks {
		if item.ID == targetID && len(item.Subtasks) != 1 {
			t.Fatal("target parent did not receive subtask")
		}
		if item.ID == wrongID && len(item.Subtasks) != 0 {
			t.Fatal("subtask moved to following task")
		}
	}
}
