package storage

import (
	"fmt"
	"strings"

	"github.com/krisitan/tasks-go/internal/task"
)

func (s *Storage) loadMutationList(target TaskTarget) (*task.TaskList, TaskTarget, error) {
	if target.ListName == "" {
		return nil, target, fmt.Errorf("list name is required")
	}
	list, err := s.LoadList(target.ListName)
	if err != nil {
		return nil, target, fmt.Errorf("loading list %s: %w", target.ListName, err)
	}
	resolved, ok := resolveTaskTarget(list, target)
	if !ok {
		return nil, target, fmt.Errorf("task not found in list %s", target.ListName)
	}
	return list, resolved, nil
}

func resolveTaskTarget(list *task.TaskList, target TaskTarget) (TaskTarget, bool) {
	if target.IsSubtask {
		return resolveSubtaskTarget(list, target)
	}
	if target.ID != "" {
		if index, found := findTaskByID(list.Tasks, target.ID); found != nil {
			target.Index = index
			return target, true
		}
	}
	if target.Index >= 0 && target.Index < list.Len() {
		return target, true
	}
	idx := findTaskIndexByContent(list.Tasks, target.Content)
	if idx < 0 {
		return target, false
	}
	target.Index = idx
	return target, true
}

func resolveSubtaskTarget(list *task.TaskList, target TaskTarget) (TaskTarget, bool) {
	if target.ID != "" {
		parentIndex, subIndex, found := findSubtaskByID(list.Tasks, target.ID)
		if found != nil {
			target.Index, target.SubIndex = parentIndex, subIndex
			return target, true
		}
	}
	if hasSubtaskAt(list, target.Index, target.SubIndex) {
		return target, true
	}
	parentIdx, subIdx := findSubtaskIndexByContent(list.Tasks, target.Content)
	if parentIdx < 0 {
		return target, false
	}
	target.Index = parentIdx
	target.SubIndex = subIdx
	return target, true
}

func hasSubtaskAt(list *task.TaskList, parentIdx, subIdx int) bool {
	parent := list.Get(parentIdx)
	return parent != nil && subIdx >= 0 && subIdx < len(parent.Subtasks)
}

func findTaskIndexByContent(tasks []*task.Task, content string) int {
	needle := strings.TrimSpace(content)
	for i, t := range tasks {
		if needle != "" && strings.TrimSpace(t.Content) == needle {
			return i
		}
	}
	return -1
}

func findSubtaskIndexByContent(tasks []*task.Task, content string) (int, int) {
	needle := strings.TrimSpace(content)
	for i, t := range tasks {
		if idx := findTaskIndexByContent(t.Subtasks, needle); idx >= 0 {
			return i, idx
		}
	}
	return -1, -1
}
