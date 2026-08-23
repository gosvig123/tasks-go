package storage

import (
	"fmt"
	"strings"
	"time"

	"github.com/krisitan/tasks-go/internal/task"
)

func canonicalSubtask(tasks []*task.Task, reference *task.Task) (*task.Task, error) {
	if match := idMatch(tasks, reference.ID); match != nil {
		return match, nil
	}
	return uniqueTaskMatch(tasks, reference.Content)
}

func canonicalReference(list *task.TaskList, reference *task.Task) (*task.Task, error) {
	if list == nil {
		return nil, fmt.Errorf("source list not found")
	}
	if match := idMatch(list.Tasks, reference.ID); match != nil {
		return match, nil
	}
	return uniqueReferenceMatch(list.Tasks, reference)
}

func idMatch(tasks []*task.Task, id string) *task.Task {
	if id == "" {
		return nil
	}
	for _, candidate := range tasks {
		if candidate.ID == id {
			return candidate
		}
	}
	return nil
}

func uniqueReferenceMatch(tasks []*task.Task, reference *task.Task) (*task.Task, error) {
	matches := contentMatches(tasks, reference.Content)
	if len(matches) > 1 && reference.DueDate != nil {
		matches = dateMatches(matches, reference)
	}
	if len(matches) > 1 && reference.CreatedAt != nil {
		matches = createdMatches(matches, reference)
	}
	return requireUniqueMatch(matches)
}

func uniqueTaskMatch(tasks []*task.Task, content string) (*task.Task, error) {
	return requireUniqueMatch(contentMatches(tasks, content))
}

func requireUniqueMatch(matches []*task.Task) (*task.Task, error) {
	if len(matches) != 1 {
		return nil, fmt.Errorf("expected one source match, found %d", len(matches))
	}
	return matches[0], nil
}

func contentMatches(tasks []*task.Task, content string) []*task.Task {
	needle := strings.TrimSpace(content)
	matches := make([]*task.Task, 0)
	for _, candidate := range tasks {
		if strings.TrimSpace(candidate.Content) == needle {
			matches = append(matches, candidate)
		}
	}
	return matches
}

func dateMatches(tasks []*task.Task, reference *task.Task) []*task.Task {
	matches := make([]*task.Task, 0)
	for _, candidate := range tasks {
		if sameDate(candidate.DueDate, reference.DueDate) {
			matches = append(matches, candidate)
		}
	}
	return matches
}

func createdMatches(tasks []*task.Task, reference *task.Task) []*task.Task {
	matches := make([]*task.Task, 0)
	for _, candidate := range tasks {
		if sameDate(candidate.CreatedAt, reference.CreatedAt) {
			matches = append(matches, candidate)
		}
	}
	return matches
}

func sameDate(left, right *time.Time) bool {
	return left != nil && right != nil && left.Format("2006-01-02") == right.Format("2006-01-02")
}
