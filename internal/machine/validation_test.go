package machine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotRejectsInvalidListNames(t *testing.T) {
	store, _ := testStore(t)
	service := &Service{Store: store}
	outside := filepath.Join(filepath.Dir(store.TasksDir), "outside.md")
	if err := os.WriteFile(outside, []byte("[] secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../outside", "nested/list", ".hidden", "."} {
		_, err := service.Snapshot(name)
		assertAPIError(t, err, CodeBadRequest)
	}
	if err := store.CreateList("legacy work"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Snapshot("legacy work"); err != nil {
		t.Fatalf("safe legacy list name was rejected: %v", err)
	}
}

func TestCreateTaskRejectsInvalidListName(t *testing.T) {
	store, _ := testStore(t)
	service := &Service{Store: store}
	title := "escaped"
	request := Request{SchemaVersion: SchemaVersion, Operation: OpTaskCreate,
		List: "../outside", Changes: TaskChanges{Title: &title}}
	_, err := service.Execute(request)
	assertAPIError(t, err, CodeBadRequest)
	if store.ListExists("../outside") {
		t.Fatal("invalid task list was created")
	}
}

func TestListOperationsValidateNames(t *testing.T) {
	store, _ := testStore(t)
	service := &Service{Store: store}
	for _, operation := range []string{OpListCreate, OpListSetCurrent, OpListDelete} {
		request := listRequest(operation, "../outside", "")
		assertExecuteCode(t, service, request, CodeBadRequest)
	}
	request := listRequest(OpListRename, ListToday, "")
	request.NewList = "../outside"
	assertExecuteCode(t, service, request, CodeBadRequest)
}

func TestListOperationsSupportSafeLegacyNames(t *testing.T) {
	store, _ := testStore(t)
	if err := store.CreateList("legacy work"); err != nil {
		t.Fatal(err)
	}
	service := &Service{Store: store}
	executeList(t, service, listRequest(OpListSetCurrent, "legacy work", ""))
	assertExecuteCode(t, service, listRequest(OpListCreate, "new list", ""), CodeBadRequest)
}

func TestUpdateRejectsCompletionChanges(t *testing.T) {
	store, item := testStore(t)
	service := &Service{Store: store}
	completed := true
	for _, operation := range []string{OpTaskUpdate, OpSubtaskUpdate} {
		request := Request{SchemaVersion: SchemaVersion, Operation: operation,
			TaskID: item.ID, Changes: TaskChanges{Completed: &completed}}
		assertExecuteCode(t, service, request, CodeBadRequest)
	}
}

func TestRemoveFromTodayRejectsDirectTask(t *testing.T) {
	store, item := testStore(t)
	service := &Service{Store: store}
	request := Request{SchemaVersion: SchemaVersion, Operation: OpTaskRemoveFromToday,
		TaskID: item.ID, Changes: TaskChanges{}}
	assertExecuteCode(t, service, request, CodeBadRequest)
	snapshot, err := service.Snapshot(ListToday)
	if err != nil || len(snapshot.Tasks) != 1 {
		t.Fatalf("direct Today task changed: snapshot=%+v err=%v", snapshot, err)
	}
}

func TestListOperationsReturnNotFound(t *testing.T) {
	store, _ := testStore(t)
	service := &Service{Store: store}
	for _, operation := range []string{OpListSetCurrent, OpListDelete, OpListRename} {
		request := listRequest(operation, "missing", "")
		request.NewList = "renamed"
		assertExecuteCode(t, service, request, CodeNotFound)
	}
}

func assertAPIError(t *testing.T, err error, code string) {
	t.Helper()
	typed, ok := err.(*Error)
	if !ok || typed.Code != code {
		t.Fatalf("expected %s, got %v", code, err)
	}
}
