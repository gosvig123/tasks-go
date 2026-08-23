package machine

import (
	"time"

	"github.com/krisitan/tasks-go/internal/storage"
	"github.com/krisitan/tasks-go/internal/task"
)

type Service struct {
	Store *storage.Storage
}

func (s *Service) Snapshot(listName string) (*Snapshot, error) {
	if listName == "" {
		listName = ListToday
	}
	if err := validateListName(listName); err != nil {
		return nil, err
	}
	var result *Snapshot
	err := s.Store.WithLock(func() error {
		loaded, err := s.snapshotLocked(listName)
		result = loaded
		return err
	})
	return result, err
}

func (s *Service) snapshotLocked(listName string) (*Snapshot, error) {
	if _, err := s.Store.ResetTodayList(); err != nil {
		return nil, err
	}
	list, err := s.Store.LoadList(listName)
	if err != nil {
		return nil, err
	}
	if err := requireIDs(list.Tasks); err != nil {
		return nil, err
	}
	revision, err := s.Store.Revision()
	if err != nil {
		return nil, err
	}
	return buildSnapshot(list, revision), nil
}

func buildSnapshot(list *task.TaskList, revision string) *Snapshot {
	items := make([]TaskItem, 0, len(list.Tasks))
	for _, item := range list.SortedTasks() {
		items = append(items, taskItem(item, list.Name, false))
	}
	now := time.Now().Truncate(time.Second)
	return &Snapshot{
		SchemaVersion: SchemaVersion, GeneratedAt: now, Day: now.Format("2006-01-02"),
		Revision: revision, PendingCount: list.PendingCount(), CompletedCount: list.CompletedCount(), Tasks: items,
	}
}

func taskItem(item *task.Task, viewList string, subtask bool) TaskItem {
	result := TaskItem{
		ID: item.ID, Title: item.Content, Completed: item.Completed, Description: item.Description,
		RecurrenceDays: item.RecurDays, OwnerList: ownerList(item, viewList), SourceList: item.Source,
		Placement: placement(item), AvailableActions: actions(item, viewList, subtask), Subtasks: []TaskItem{},
	}
	if item.DueDate != nil {
		result.DueDate = item.DueDate.Format("2006-01-02")
	}
	if item.StartTime != nil {
		result.StartTime = item.StartTime.String()
	}
	if item.Estimate != nil {
		result.EstimateSeconds = int(item.Estimate.Seconds())
	}
	for _, child := range item.Subtasks {
		result.Subtasks = append(result.Subtasks, taskItem(child, result.OwnerList, true))
	}
	return result
}

func actions(item *task.Task, viewList string, subtask bool) []string {
	if subtask {
		return []string{OpSubtaskUpdate, OpSubtaskSetCompleted, OpSubtaskDelete}
	}
	result := []string{OpTaskUpdate, OpTaskSetCompleted, OpTaskDelete}
	if item.Source != "" {
		return append(result, OpTaskRemoveFromToday)
	}
	if viewList != ListToday {
		return append(result, OpTaskAddToToday)
	}
	return result
}

func ownerList(item *task.Task, viewList string) string {
	if item.Source != "" {
		return item.Source
	}
	return viewList
}

func placement(item *task.Task) string {
	if item.Source != "" {
		return "reference"
	}
	return "direct"
}

func requireIDs(tasks []*task.Task) error {
	for _, item := range tasks {
		if item.ID == "" {
			return apiError(CodeMigrationRequired,
				"task IDs missing; run 'tasks migrate ids --dry-run' then 'tasks migrate ids'")
		}
		if err := requireIDs(item.Subtasks); err != nil {
			return err
		}
	}
	return nil
}
