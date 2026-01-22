package task

import (
	"sort"
	"time"
)

type TaskList struct {
	Name  string
	Tasks []*Task
}

func NewTaskList(name string) *TaskList {
	return &TaskList{
		Name:  name,
		Tasks: make([]*Task, 0),
	}
}

func (l *TaskList) Add(task *Task) {
	l.Tasks = append(l.Tasks, task)
}

func (l *TaskList) AddContent(content string, description string, dueOffset int, recurDays int) *Task {
	now := time.Now()
	task := &Task{
		Content:     content,
		Description: description,
		Completed:   false,
		RecurDays:   recurDays,
		CreatedAt:   &now,
	}

	if dueOffset > 0 || recurDays > 0 {
		offset := dueOffset
		if offset == 0 && recurDays > 0 {
			offset = recurDays
		}
		due := now.AddDate(0, 0, offset)
		task.DueDate = &due
	}

	l.Tasks = append(l.Tasks, task)
	return task
}

func (l *TaskList) Delete(index int) *Task {
	if index < 0 || index >= len(l.Tasks) {
		return nil
	}
	task := l.Tasks[index]
	l.Tasks = append(l.Tasks[:index], l.Tasks[index+1:]...)
	return task
}

func (l *TaskList) Toggle(index int) *Task {
	if index < 0 || index >= len(l.Tasks) {
		return nil
	}
	l.Tasks[index].Completed = !l.Tasks[index].Completed
	return l.Tasks[index]
}

func (l *TaskList) Get(index int) *Task {
	if index < 0 || index >= len(l.Tasks) {
		return nil
	}
	return l.Tasks[index]
}

func (l *TaskList) Len() int {
	return len(l.Tasks)
}

func (l *TaskList) PendingCount() int {
	count := 0
	for _, t := range l.Tasks {
		if !t.Completed {
			count++
		}
	}
	return count
}

func (l *TaskList) CompletedCount() int {
	count := 0
	for _, t := range l.Tasks {
		if t.Completed {
			count++
		}
	}
	return count
}

// Clear removes all tasks
func (l *TaskList) Clear() {
	l.Tasks = make([]*Task, 0)
}

// GetDueTasks returns tasks that are due today or overdue
func (l *TaskList) GetDueTasks() []*Task {
	var due []*Task
	for _, t := range l.Tasks {
		if !t.Completed && t.IsDueToday() {
			due = append(due, t)
		}
	}
	return due
}

// SortedTasks returns tasks sorted: pending first (by due date), then completed
func (l *TaskList) SortedTasks() []*Task {
	pending := make([]*Task, 0)
	completed := make([]*Task, 0)

	for _, t := range l.Tasks {
		if t.Completed {
			completed = append(completed, t)
		} else {
			pending = append(pending, t)
		}
	}

	// Sort pending by due date (earliest first, nil dates last)
	sort.Slice(pending, func(i, j int) bool {
		if pending[i].DueDate == nil && pending[j].DueDate == nil {
			return false
		}
		if pending[i].DueDate == nil {
			return false
		}
		if pending[j].DueDate == nil {
			return true
		}
		return pending[i].DueDate.Before(*pending[j].DueDate)
	})

	// Sort completed by due date as well
	sort.Slice(completed, func(i, j int) bool {
		if completed[i].DueDate == nil && completed[j].DueDate == nil {
			return false
		}
		if completed[i].DueDate == nil {
			return false
		}
		if completed[j].DueDate == nil {
			return true
		}
		return completed[i].DueDate.Before(*completed[j].DueDate)
	})

	return append(pending, completed...)
}

// OriginalIndex returns the original index of a task in the unsorted list
func (l *TaskList) OriginalIndex(task *Task) int {
	// First try pointer comparison (most reliable when pointers are valid)
	for i, t := range l.Tasks {
		if t == task {
			return i
		}
	}
	// Fallback to content-based matching if pointer comparison fails
	for i, t := range l.Tasks {
		if t.Content == task.Content && t.Completed == task.Completed {
			// Additional check: due dates should match if both exist
			if (t.DueDate == nil && task.DueDate == nil) ||
				(t.DueDate != nil && task.DueDate != nil && t.DueDate.Equal(*task.DueDate)) {
				return i
			}
		}
	}
	return -1
}
