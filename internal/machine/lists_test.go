package machine

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

func TestRunAPIListsReturnsStableJSON(t *testing.T) {
	store, _ := testStore(t)
	if err := store.CreateList("zeta"); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateList("alpha"); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := RunAPI(store, []string{"lists"}, bytes.NewReader(nil), &output); err != nil {
		t.Fatal(err)
	}
	var snapshot ListsSnapshot
	if err := json.Unmarshal(output.Bytes(), &snapshot); err != nil {
		t.Fatalf("invalid JSON: %v; output=%s", err, output.String())
	}
	want := []string{"alpha", "today", "zeta"}
	if snapshot.SchemaVersion != 1 || !reflect.DeepEqual(snapshot.Lists, want) {
		t.Fatalf("unexpected lists snapshot: %+v", snapshot)
	}
	if snapshot.Revision == "" || snapshot.CurrentList != ListToday {
		t.Fatalf("missing list metadata: %+v", snapshot)
	}
}

func TestListCRUDAndCurrentList(t *testing.T) {
	store, _ := testStore(t)
	service := &Service{Store: store}
	revision := listRevision(t, service)
	created := executeList(t, service, Request{Operation: OpListCreate,
		List: "work", ExpectedRevision: revision})
	selected := executeList(t, service, Request{Operation: OpListSetCurrent,
		List: "work", ExpectedRevision: created.Revision})
	if selected.Revision == created.Revision {
		t.Fatal("changing the current list did not change the revision")
	}
	renamed := executeList(t, service, Request{Operation: OpListRename,
		List: "work", NewList: "projects", ExpectedRevision: selected.Revision})
	if store.GetCurrentList() != "projects" || !store.ListExists("projects") {
		t.Fatal("rename did not update the current list")
	}
	assertExecuteCode(t, service, listRequest(OpListDelete, "projects", renamed.Revision), CodeBadRequest)
	selected = executeList(t, service, Request{Operation: OpListSetCurrent,
		List: ListToday, ExpectedRevision: renamed.Revision})
	executeList(t, service, Request{Operation: OpListDelete,
		List: "projects", ExpectedRevision: selected.Revision})
	if store.ListExists("projects") {
		t.Fatal("list was not deleted")
	}
}

func TestListMutationRejectsStaleRevision(t *testing.T) {
	store, _ := testStore(t)
	service := &Service{Store: store}
	request := listRequest(OpListCreate, "work", "sha256:stale")
	assertExecuteCode(t, service, request, CodeConflict)
	if store.ListExists("work") {
		t.Fatal("conflicting create changed storage")
	}
}

func TestListMutationProtectsToday(t *testing.T) {
	store, _ := testStore(t)
	service := &Service{Store: store}
	for _, operation := range []string{OpListRename, OpListDelete} {
		request := listRequest(operation, ListToday, "")
		request.NewList = "renamed"
		assertExecuteCode(t, service, request, CodeBadRequest)
	}
}

func executeList(t *testing.T, service *Service, request Request) *Snapshot {
	t.Helper()
	request.SchemaVersion = SchemaVersion
	request.Changes = TaskChanges{}
	snapshot, err := service.Execute(request)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func listRevision(t *testing.T, service *Service) string {
	t.Helper()
	lists, err := service.Lists()
	if err != nil {
		t.Fatal(err)
	}
	return lists.Revision
}

func listRequest(operation, list, revision string) Request {
	return Request{SchemaVersion: SchemaVersion, Operation: operation,
		List: list, ExpectedRevision: revision, Changes: TaskChanges{}}
}

func assertExecuteCode(t *testing.T, service *Service, request Request, code string) {
	t.Helper()
	_, err := service.Execute(request)
	typed, ok := err.(*Error)
	if !ok || typed.Code != code {
		t.Fatalf("expected %s, got %v", code, err)
	}
}
