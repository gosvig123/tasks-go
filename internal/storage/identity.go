package storage

import (
	"strings"

	"github.com/krisitan/tasks-go/internal/task"
)

func sameTask(left, right *task.Task) bool {
	if left == nil || right == nil {
		return false
	}
	if left.ID != "" && right.ID != "" {
		return left.ID == right.ID
	}
	return strings.TrimSpace(left.Content) == strings.TrimSpace(right.Content)
}

func identityKey(item *task.Task) string {
	if item.ID != "" {
		return "id:" + item.ID
	}
	return "content:" + strings.TrimSpace(item.Content)
}

func findTaskByIdentity(tasks []*task.Task, target *task.Task) *task.Task {
	for _, candidate := range tasks {
		if sameTask(candidate, target) {
			return candidate
		}
	}
	return nil
}

func findTaskByID(tasks []*task.Task, id string) (int, *task.Task) {
	for index, candidate := range tasks {
		if candidate.ID == id {
			return index, candidate
		}
	}
	return -1, nil
}

func recurrenceExists(tasks []*task.Task, next *task.Task) bool {
	for _, candidate := range tasks {
		if strings.TrimSpace(candidate.Content) == strings.TrimSpace(next.Content) &&
			sameDate(candidate.DueDate, next.DueDate) {
			return true
		}
	}
	return false
}

func findSubtaskByID(tasks []*task.Task, id string) (int, int, *task.Task) {
	for parentIndex, parent := range tasks {
		if subIndex, subtask := findTaskByID(parent.Subtasks, id); subtask != nil {
			return parentIndex, subIndex, subtask
		}
	}
	return -1, -1, nil
}
