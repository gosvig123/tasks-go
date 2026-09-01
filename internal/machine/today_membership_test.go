package machine

import (
	"testing"
	"time"

	"github.com/krisitan/tasks-go/internal/storage"
	"github.com/krisitan/tasks-go/internal/task"
)

func TestUpdateFutureDueDateRemovesTodayReference(t *testing.T) {
	updated := updateReferencedDueDate(t, 1)
	if len(updated.Tasks) != 0 {
		t.Fatalf("future task remained in Today: %+v", updated.Tasks)
	}
}

func TestUpdateOverdueDueDateKeepsTodayReference(t *testing.T) {
	updated := updateReferencedDueDate(t, -1)
	if len(updated.Tasks) != 1 {
		t.Fatalf("overdue task left Today: %+v", updated.Tasks)
	}
}

func TestClearDueDateRemovesTodayReference(t *testing.T) {
	updated := updateReferencedDueDateValue(t, "")
	if len(updated.Tasks) != 0 {
		t.Fatalf("task without due date remained in Today: %+v", updated.Tasks)
	}
}

func updateReferencedDueDate(t *testing.T, days int) *Snapshot {
	t.Helper()
	value := time.Now().AddDate(0, 0, days).Format("2006-01-02")
	return updateReferencedDueDateValue(t, value)
}

func updateReferencedDueDateValue(t *testing.T, value string) *Snapshot {
	t.Helper()
	store, item := referencedTestStore(t)
	service := &Service{Store: store}
	before, err := service.Snapshot(ListToday)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := service.Execute(Request{
		SchemaVersion: SchemaVersion, Operation: OpTaskUpdate, TaskID: item.ID,
		ExpectedRevision: before.Revision, Changes: TaskChanges{DueDate: &value},
	})
	if err != nil {
		t.Fatal(err)
	}
	return updated
}

func referencedTestStore(t *testing.T) (*storage.Storage, *task.Task) {
	t.Helper()
	store, item := testStore(t)
	due := time.Now()
	item.DueDate = &due
	work := task.NewTaskList("work")
	work.Add(item)
	if err := store.SaveList(work); err != nil {
		t.Fatal(err)
	}
	today := task.NewTaskList(ListToday)
	today.Add(task.NewReferenceStub(item, "work"))
	if err := store.SaveList(today); err != nil {
		t.Fatal(err)
	}
	return store, item
}
