package machine

import (
	"fmt"
	"time"

	"github.com/krisitan/tasks-go/internal/task"
)

func (s *Service) Execute(request Request) (*Snapshot, error) {
	if err := validateRequest(request); err != nil {
		return nil, err
	}
	var result *Snapshot
	err := s.Store.WithLock(func() error {
		if _, err := s.Store.ResetTodayList(); err != nil {
			return err
		}
		if err := s.checkRevision(request.ExpectedRevision); err != nil {
			return err
		}
		if err := s.apply(request); err != nil {
			return err
		}
		return s.loadTodaySnapshot(&result)
	})
	return result, err
}

func validateRequest(request Request) error {
	if request.SchemaVersion != SchemaVersion {
		return apiError(CodeBadRequest, "unsupported schema version %d", request.SchemaVersion)
	}
	if request.Operation == "" {
		return apiError(CodeBadRequest, "operation is required")
	}
	return rejectUpdateCompletion(request)
}

func (s *Service) checkRevision(expected string) error {
	if expected == "" {
		return nil
	}
	actual, err := s.Store.Revision()
	if err != nil {
		return err
	}
	if actual != expected {
		return apiError(CodeConflict, "task data changed; refresh and try again")
	}
	return nil
}

func (s *Service) loadTodaySnapshot(destination **Snapshot) error {
	list, err := s.Store.LoadList(ListToday)
	if err != nil {
		return err
	}
	if err := requireIDs(list.Tasks); err != nil {
		return err
	}
	revision, err := s.Store.Revision()
	if err != nil {
		return err
	}
	*destination = buildSnapshot(list, revision)
	return nil
}

func (s *Service) apply(request Request) error {
	switch request.Operation {
	case OpTaskCreate:
		return s.createTask(request)
	case OpTaskUpdate, OpTaskSetCompleted, OpTaskDelete:
		return s.mutateTask(request)
	case OpTaskAddToToday, OpTaskRemoveFromToday:
		return s.mutatePlacement(request)
	case OpSubtaskCreate:
		return s.createSubtask(request)
	case OpSubtaskUpdate, OpSubtaskSetCompleted, OpSubtaskDelete:
		return s.mutateSubtask(request)
	case OpListCreate, OpListRename, OpListSetCurrent, OpListDelete:
		return s.mutateList(request)
	default:
		return apiError(CodeBadRequest, "unsupported operation %q", request.Operation)
	}
}

func setCompleted(item *task.Task, completed bool) {
	item.Completed = completed
	if !completed {
		item.CompletedAt = nil
		return
	}
	now := time.Now()
	item.CompletedAt = &now
	for _, child := range item.Subtasks {
		child.Completed = true
		child.CompletedAt = &now
	}
}

func missingID(id string) error {
	if id == "" {
		return apiError(CodeBadRequest, "taskId is required")
	}
	return fmt.Errorf("task %s not found", id)
}
