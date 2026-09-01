package storage

import (
	"fmt"

	"github.com/krisitan/tasks-go/internal/task"
)

// TaskTarget identifies a top-level task or subtask for storage mutations.
type TaskTarget struct {
	ID        string
	ListName  string
	Index     int
	SubIndex  int
	IsSubtask bool
	Content   string
}

// TaskMutationResult describes side effects from a storage mutation.
type TaskMutationResult struct {
	Task           *task.Task
	Parent         *task.Task
	ListName       string
	RecurrenceList string
	SyncedToSource string
	SyncedToToday  bool
}

// ToggleTask toggles a task and owns recurrence plus today/source sync.
func (s *Storage) ToggleTask(target TaskTarget) (*TaskMutationResult, error) {
	var result *TaskMutationResult
	err := s.WithLock(func() error {
		var actionErr error
		result, actionErr = s.toggleTask(target)
		return actionErr
	})
	return result, err
}

func (s *Storage) toggleTask(target TaskTarget) (*TaskMutationResult, error) {
	list, target, err := s.loadMutationList(target)
	if err != nil {
		return nil, err
	}

	result := &TaskMutationResult{ListName: list.Name}
	if target.IsSubtask {
		s.toggleSubtask(list, target, result)
	} else {
		s.toggleTopLevel(list, target, result)
	}
	return s.finishMutation(list, result)
}

// DeleteTask deletes a task from its owning list.
func (s *Storage) DeleteTask(target TaskTarget) (*TaskMutationResult, error) {
	var result *TaskMutationResult
	err := s.WithLock(func() error {
		var actionErr error
		result, actionErr = s.deleteTask(target)
		return actionErr
	})
	return result, err
}

func (s *Storage) deleteTask(target TaskTarget) (*TaskMutationResult, error) {
	list, target, err := s.loadMutationList(target)
	if err != nil {
		return nil, err
	}

	result := &TaskMutationResult{ListName: list.Name}
	if target.IsSubtask {
		result.Parent = list.Get(target.Index)
		result.Task = list.DeleteSubtask(target.Index, target.SubIndex)
	} else {
		result.Task = list.Delete(target.Index)
	}
	return s.finishMutation(list, result)
}

// AddSubtask adds a subtask to a parent task and syncs the changed parent.
func (s *Storage) AddSubtask(target TaskTarget, subtask *task.Task) (*TaskMutationResult, error) {
	var result *TaskMutationResult
	err := s.WithLock(func() error {
		var actionErr error
		result, actionErr = s.addSubtask(target, subtask)
		return actionErr
	})
	return result, err
}

func (s *Storage) addSubtask(target TaskTarget, subtask *task.Task) (*TaskMutationResult, error) {
	if subtask == nil {
		return nil, fmt.Errorf("subtask is nil")
	}
	list, target, err := s.loadMutationList(target)
	if err != nil {
		return nil, err
	}

	result := &TaskMutationResult{Parent: list.Get(target.Index), ListName: list.Name}
	result.Task = list.AddSubtask(target.Index, subtask)
	if _, err := s.finishMutation(list, result); err != nil {
		return nil, err
	}
	s.syncSubtaskAddition(list.Name, result.Parent, result)
	return result, nil
}

func (s *Storage) finishMutation(list *task.TaskList, result *TaskMutationResult) (*TaskMutationResult, error) {
	if result.Task == nil {
		return nil, fmt.Errorf("task not found in list %s", list.Name)
	}
	if err := s.SaveList(list); err != nil {
		return nil, fmt.Errorf("saving list %s: %w", list.Name, err)
	}
	return result, nil
}

func (s *Storage) syncSubtaskAddition(listName string, parent *task.Task, result *TaskMutationResult) {
	if parent == nil {
		return
	}
	if parent.Source != "" {
		s.SyncSubtasksToSource(parent)
		result.SyncedToSource = parent.Source
		return
	}
	if listName != "today" {
		s.SyncCompletionToToday(listName, parent)
		result.SyncedToToday = true
	}
}

func (s *Storage) toggleTopLevel(list *task.TaskList, target TaskTarget, result *TaskMutationResult) {
	result.Task = list.Toggle(target.Index)
	result.RecurrenceList = s.HandleRecurrence(result.Task, list)
	s.syncTaskMutation(list.Name, result.Task, result)
}

func (s *Storage) toggleSubtask(list *task.TaskList, target TaskTarget, result *TaskMutationResult) {
	result.Parent = list.Get(target.Index)
	result.Task = list.ToggleSubtask(target.Index, target.SubIndex)
	s.syncParentMutation(list.Name, result.Parent, result)
}

func (s *Storage) syncTaskMutation(listName string, t *task.Task, result *TaskMutationResult) {
	if t == nil {
		return
	}
	if t.Source != "" {
		s.SyncCompletionToSource(t)
		result.SyncedToSource = t.Source
		return
	}
	if listName != "today" {
		s.SyncCompletionToToday(listName, t)
		result.SyncedToToday = true
	}
}

func (s *Storage) syncParentMutation(listName string, parent *task.Task, result *TaskMutationResult) {
	if parent == nil {
		return
	}
	if parent.Source != "" {
		s.SyncCompletionToSource(parent)
		result.SyncedToSource = parent.Source
		return
	}
	if listName != "today" {
		s.SyncCompletionToToday(listName, parent)
		result.SyncedToToday = true
	}
}
