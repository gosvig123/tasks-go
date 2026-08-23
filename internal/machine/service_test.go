package machine

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/krisitan/tasks-go/internal/storage"
	"github.com/krisitan/tasks-go/internal/task"
)

func TestSnapshotAndDesiredCompletion(t *testing.T) {
	store, item := testStore(t)
	service := &Service{Store: store}
	snapshot, err := service.Snapshot(ListToday)
	if err != nil {
		t.Fatal(err)
	}
	completed := true
	request := Request{SchemaVersion: 1, Operation: OpTaskSetCompleted, TaskID: item.ID,
		ExpectedRevision: snapshot.Revision, Changes: TaskChanges{Completed: &completed}}
	updated, err := service.Execute(request)
	if err != nil {
		t.Fatal(err)
	}
	if !updated.Tasks[0].Completed {
		t.Fatal("task was not completed")
	}
	request.ExpectedRevision = updated.Revision
	if _, err := service.Execute(request); err != nil {
		t.Fatalf("desired-state retry failed: %v", err)
	}
}

func TestRevisionConflict(t *testing.T) {
	store, item := testStore(t)
	service := &Service{Store: store}
	completed := true
	_, err := service.Execute(Request{SchemaVersion: 1, Operation: OpTaskSetCompleted,
		TaskID: item.ID, ExpectedRevision: "sha256:stale", Changes: TaskChanges{Completed: &completed}})
	if typed, ok := err.(*Error); !ok || typed.Code != CodeConflict {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestRunAPIExecReturnsJSONError(t *testing.T) {
	store, _ := testStore(t)
	input := bytes.NewBufferString(`{"schemaVersion":1,"operation":"unknown"}`)
	var output bytes.Buffer
	if err := RunAPI(store, []string{"exec"}, input, &output); err != nil {
		t.Fatal(err)
	}
	var response Response
	if err := json.Unmarshal(output.Bytes(), &response); err != nil {
		t.Fatalf("invalid JSON: %v; output=%s", err, output.String())
	}
	if response.Success || response.Error == nil || response.Error.Code != CodeBadRequest {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func testStore(t *testing.T) (*storage.Storage, *task.Task) {
	t.Helper()
	dir := t.TempDir()
	store := &storage.Storage{
		TasksDir: dir, LockFile: filepath.Join(dir, ".lock"),
		CurrentListFile: filepath.Join(dir, ".current"), LastResetFile: filepath.Join(dir, ".reset"),
		LastSyncFile: filepath.Join(dir, ".sync"), LocalStateFile: filepath.Join(dir, ".state"),
	}
	item := &task.Task{ID: task.NewID(), Content: "Plan tomorrow"}
	today := task.NewTaskList(ListToday)
	today.Add(item)
	if err := store.SaveList(today); err != nil {
		t.Fatal(err)
	}
	return store, item
}
