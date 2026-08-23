package machine

import (
	"time"

	"github.com/krisitan/tasks-go/internal/task"
)

func (s *Service) createTask(request Request) error {
	listName, err := taskListName(request.List)
	if err != nil {
		return err
	}
	if !s.Store.ListExists(listName) {
		return apiError(CodeNotFound, "list %q not found", listName)
	}
	list, err := s.Store.LoadList(listName)
	if err != nil {
		return err
	}
	now := time.Now()
	item := &task.Task{ID: task.NewID(), CreatedAt: &now}
	if err := applyChanges(item, request.Changes); err != nil {
		return err
	}
	if item.Content == "" {
		return apiError(CodeBadRequest, "title is required")
	}
	if listName == ListToday {
		item.EnsureDueToday()
	}
	list.Add(item)
	return s.Store.SaveList(list)
}

func (s *Service) mutateTask(request Request) error {
	located, err := s.locateCanonical(request.TaskID)
	if err != nil {
		return err
	}
	if located.IsSubtask {
		return apiError(CodeBadRequest, "operation requires a top-level task")
	}
	switch request.Operation {
	case OpTaskUpdate:
		err = applyChanges(located.Task, request.Changes)
	case OpTaskSetCompleted:
		err = s.completeTask(located, request.Changes)
	case OpTaskDelete:
		return s.deleteTask(located)
	}
	if err != nil {
		return err
	}
	return s.saveCanonical(located)
}

func (s *Service) completeTask(located *locatedTask, changes TaskChanges) error {
	if changes.Completed == nil {
		return apiError(CodeBadRequest, "changes.completed is required")
	}
	wasCompleted := located.Task.Completed
	setCompleted(located.Task, *changes.Completed)
	if !wasCompleted && located.Task.Completed {
		s.Store.HandleRecurrence(located.Task, located.List)
	}
	return nil
}

func (s *Service) saveCanonical(located *locatedTask) error {
	if err := s.Store.SaveList(located.List); err != nil {
		return err
	}
	if located.ListName != ListToday {
		s.Store.SyncCompletionToToday(located.ListName, located.Task)
	}
	return nil
}

func (s *Service) deleteTask(located *locatedTask) error {
	located.List.Delete(located.Index)
	if err := s.Store.SaveList(located.List); err != nil {
		return err
	}
	if located.ListName == ListToday {
		return nil
	}
	err := s.removeTodayReference(located.Task.ID)
	if typed, ok := err.(*Error); ok && typed.Code == CodeNotFound {
		return nil
	}
	return err
}

func (s *Service) mutatePlacement(request Request) error {
	if request.Operation == OpTaskRemoveFromToday {
		return s.removeTodayReference(request.TaskID)
	}
	located, err := s.locateCanonical(request.TaskID)
	if err != nil {
		return err
	}
	if located.ListName == ListToday {
		return nil
	}
	return s.addTodayReference(located.Task, located.ListName)
}

func (s *Service) addTodayReference(item *task.Task, source string) error {
	today, err := s.Store.LoadList(ListToday)
	if err != nil {
		return err
	}
	for _, existing := range today.Tasks {
		if existing.ID == item.ID {
			return nil
		}
	}
	today.Add(task.NewReferenceStub(item, source))
	return s.Store.SaveList(today)
}

func (s *Service) removeTodayReference(id string) error {
	today, err := s.Store.LoadList(ListToday)
	if err != nil {
		return err
	}
	for index, item := range today.Tasks {
		if item.ID != id {
			continue
		}
		if item.Source == "" {
			return apiError(CodeBadRequest, "task %s is direct in Today; use task.delete", id)
		}
		today.Delete(index)
		return s.Store.SaveList(today)
	}
	return apiError(CodeNotFound, "task %s is not in Today", id)
}
