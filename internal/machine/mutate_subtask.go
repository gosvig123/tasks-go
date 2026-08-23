package machine

import (
	"time"

	"github.com/krisitan/tasks-go/internal/task"
)

func (s *Service) createSubtask(request Request) error {
	parentID := request.TaskID
	if request.Changes.ParentID != nil {
		parentID = *request.Changes.ParentID
	}
	parent, err := s.locateCanonical(parentID)
	if err != nil {
		return err
	}
	if parent.IsSubtask {
		return apiError(CodeBadRequest, "subtasks support one level")
	}
	now := time.Now()
	child := &task.Task{ID: task.NewID(), CreatedAt: &now}
	if err := applyChanges(child, request.Changes); err != nil {
		return err
	}
	if child.Content == "" {
		return apiError(CodeBadRequest, "subtask title is required")
	}
	parent.List.AddSubtask(parent.Index, child)
	return s.saveParent(parent)
}

func (s *Service) mutateSubtask(request Request) error {
	located, err := s.locateCanonical(request.TaskID)
	if err != nil {
		return err
	}
	if !located.IsSubtask {
		return apiError(CodeBadRequest, "operation requires a subtask")
	}
	switch request.Operation {
	case OpSubtaskUpdate:
		err = applyChanges(located.Task, request.Changes)
	case OpSubtaskSetCompleted:
		err = completeSubtask(located, request.Changes)
	case OpSubtaskDelete:
		located.List.DeleteSubtask(located.Index, located.SubIndex)
	}
	if err != nil {
		return err
	}
	return s.saveParent(located)
}

func completeSubtask(located *locatedTask, changes TaskChanges) error {
	if changes.Completed == nil {
		return apiError(CodeBadRequest, "changes.completed is required")
	}
	setCompleted(located.Task, *changes.Completed)
	if *changes.Completed && located.Parent.AllSubtasksCompleted() {
		setCompleted(located.Parent, true)
	}
	return nil
}

func (s *Service) saveParent(located *locatedTask) error {
	if err := s.Store.SaveList(located.List); err != nil {
		return err
	}
	parent := located.Parent
	if parent == nil {
		parent = located.Task
	}
	if located.ListName != ListToday {
		s.Store.SyncCompletionToToday(located.ListName, parent)
	}
	return nil
}
