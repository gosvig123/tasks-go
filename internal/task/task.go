package task

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type Task struct {
	Content   string
	Completed bool
	DueDate   *time.Time
	RecurDays int
	CreatedAt *time.Time
	Source    string // For today's list: which list this task came from
}

var (
	dueRegex     = regexp.MustCompile(`@due:(\d{4}-\d{2}-\d{2})`)
	recurRegex   = regexp.MustCompile(`@recur:(\d+)`)
	sourceRegex  = regexp.MustCompile(`@source:([a-zA-Z0-9_-]+)`)
	createdRegex = regexp.MustCompile(`\|\| (\d{4}-\d{2}-\d{2})$`)
)

// Parse parses a task line from the file format
// Format: [] task content @due:2024-01-15 @recur:7 @source:work || 2024-01-10
// or:     [x] completed task content
func Parse(line string) (*Task, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, fmt.Errorf("empty line")
	}

	task := &Task{}

	// Check completion status
	if strings.HasPrefix(line, "[x] ") {
		task.Completed = true
		line = strings.TrimPrefix(line, "[x] ")
	} else if strings.HasPrefix(line, "[] ") {
		task.Completed = false
		line = strings.TrimPrefix(line, "[] ")
	} else {
		return nil, fmt.Errorf("invalid task format: %s", line)
	}

	// Extract created date (at the end after ||)
	if match := createdRegex.FindStringSubmatch(line); len(match) > 1 {
		if t, err := time.Parse("2006-01-02", match[1]); err == nil {
			task.CreatedAt = &t
		}
		line = createdRegex.ReplaceAllString(line, "")
		line = strings.TrimSpace(line)
	}

	// Extract due date
	if match := dueRegex.FindStringSubmatch(line); len(match) > 1 {
		if t, err := time.Parse("2006-01-02", match[1]); err == nil {
			task.DueDate = &t
		}
		line = dueRegex.ReplaceAllString(line, "")
	}

	// Extract recur days
	if match := recurRegex.FindStringSubmatch(line); len(match) > 1 {
		fmt.Sscanf(match[1], "%d", &task.RecurDays)
		line = recurRegex.ReplaceAllString(line, "")
	}

	// Extract source
	if match := sourceRegex.FindStringSubmatch(line); len(match) > 1 {
		task.Source = match[1]
		line = sourceRegex.ReplaceAllString(line, "")
	}

	task.Content = strings.TrimSpace(line)
	return task, nil
}

// String converts the task back to file format
func (t *Task) String() string {
	var sb strings.Builder

	if t.Completed {
		sb.WriteString("[x] ")
	} else {
		sb.WriteString("[] ")
	}

	sb.WriteString(t.Content)

	if t.DueDate != nil {
		sb.WriteString(" @due:")
		sb.WriteString(t.DueDate.Format("2006-01-02"))
	}

	if t.RecurDays > 0 {
		sb.WriteString(fmt.Sprintf(" @recur:%d", t.RecurDays))
	}

	if t.Source != "" {
		sb.WriteString(" @source:")
		sb.WriteString(t.Source)
	}

	if t.CreatedAt != nil {
		sb.WriteString(" || ")
		sb.WriteString(t.CreatedAt.Format("2006-01-02"))
	}

	return sb.String()
}

// DisplayContent returns the content without metadata for display
func (t *Task) DisplayContent() string {
	return t.Content
}

// IsDueToday returns true if the task is due today or overdue
func (t *Task) IsDueToday() bool {
	if t.DueDate == nil {
		return false
	}
	today := time.Now().Truncate(24 * time.Hour)
	due := t.DueDate.Truncate(24 * time.Hour)
	return !due.After(today)
}

// CreateNextRecurrence creates the next occurrence of a recurring task
func (t *Task) CreateNextRecurrence() *Task {
	if t.RecurDays <= 0 {
		return nil
	}

	nextDue := time.Now().AddDate(0, 0, t.RecurDays)
	now := time.Now()

	return &Task{
		Content:   t.Content,
		Completed: false,
		DueDate:   &nextDue,
		RecurDays: t.RecurDays,
		CreatedAt: &now,
		Source:    "", // New occurrence goes to original list, not today
	}
}
