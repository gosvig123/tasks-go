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
	t := l.Tasks[index]
	t.Completed = !t.Completed

	now := time.Now()
	if t.Completed {
		t.CompletedAt = &now
		for _, sub := range t.Subtasks {
			sub.Completed = true
			sub.CompletedAt = &now
		}
	} else {
		t.CompletedAt = nil
	}

	return t
}

// AddSubtask appends a subtask to the parent at the given index.
func (l *TaskList) AddSubtask(parentIndex int, subtask *Task) *Task {
	parent := l.Get(parentIndex)
	if parent == nil {
		return nil
	}
	subtask.Parent = parent
	parent.Subtasks = append(parent.Subtasks, subtask)
	return subtask
}

// DeleteSubtask removes a subtask from the parent at the given indices.
func (l *TaskList) DeleteSubtask(parentIndex, subIndex int) *Task {
	parent := l.Get(parentIndex)
	if parent == nil || subIndex < 0 || subIndex >= len(parent.Subtasks) {
		return nil
	}
	removed := parent.Subtasks[subIndex]
	parent.Subtasks = append(parent.Subtasks[:subIndex], parent.Subtasks[subIndex+1:]...)
	return removed
}

// ToggleSubtask toggles a subtask and auto-completes the parent if all subtasks are done.
func (l *TaskList) ToggleSubtask(parentIndex, subIndex int) *Task {
	parent := l.Get(parentIndex)
	if parent == nil || subIndex < 0 || subIndex >= len(parent.Subtasks) {
		return nil
	}
	sub := parent.Subtasks[subIndex]
	sub.Completed = !sub.Completed

	now := time.Now()
	if sub.Completed {
		sub.CompletedAt = &now
		if parent.AllSubtasksCompleted() {
			parent.Completed = true
			parent.CompletedAt = &now
		}
	} else {
		sub.CompletedAt = nil
	}

	return sub
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
		for _, sub := range t.Subtasks {
			if !sub.Completed {
				count++
			}
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
		for _, sub := range t.Subtasks {
			if sub.Completed {
				count++
			}
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

// GetUpcomingTasks returns incomplete tasks due after today and within the given number of days, sorted by due date
func (l *TaskList) GetUpcomingTasks(days int) []*Task {
	var upcoming []*Task
	for _, t := range l.Tasks {
		if t.IsDueWithin(days) {
			upcoming = append(upcoming, t)
		}
		for _, sub := range t.Subtasks {
			if sub.IsDueWithin(days) {
				upcoming = append(upcoming, sub)
			}
		}
	}
	sortByDueDate(upcoming)
	return upcoming
}

// sortByDueDate sorts tasks by due date (earliest first, nil dates last)
func sortByDueDate(tasks []*Task) {
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].DueDate == nil && tasks[j].DueDate == nil {
			return false
		}
		if tasks[i].DueDate == nil {
			return false
		}
		if tasks[j].DueDate == nil {
			return true
		}
		return tasks[i].DueDate.Before(*tasks[j].DueDate)
	})
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

	sortByDueDate(pending)
	sortByDueDate(completed)

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
