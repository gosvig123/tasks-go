package task

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type Task struct {
	Content     string
	Description string // Optional description for the task
	Completed   bool
	DueDate     *time.Time
	RecurDays   int
	CreatedAt   *time.Time
	Source      string         // For today's list: which list this task came from
	Estimate    *time.Duration // @est:2h, @est:30m, @est:1h30m
	StartTime   *TimeOfDay     // @at:09:00, @at:14:30
}

type TimeOfDay struct {
	Hour   int
	Minute int
}

func (t TimeOfDay) String() string {
	return fmt.Sprintf("%02d:%02d", t.Hour, t.Minute)
}

func (t TimeOfDay) ToMinutes() int {
	return t.Hour*60 + t.Minute
}

var (
	dueRegex     = regexp.MustCompile(`@due:(\d{4}-\d{2}-\d{2})`)
	recurRegex   = regexp.MustCompile(`@recur:(\d+)`)
	sourceRegex  = regexp.MustCompile(`@source:([a-zA-Z0-9_-]+)`)
	descRegex    = regexp.MustCompile(`@desc:"([^"]*)"`)
	estRegex     = regexp.MustCompile(`@est:(\d+h)?(\d+m)?`)
	startRegex   = regexp.MustCompile(`@at:(\d{1,2}:\d{2})`)
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
	if strings.HasPrefix(line, "[x] ") || strings.HasPrefix(line, "[X] ") {
		task.Completed = true
		line = strings.TrimPrefix(line, "[x] ")
		line = strings.TrimPrefix(line, "[X] ")
	} else if strings.HasPrefix(line, "[ ] ") {
		task.Completed = false
		line = strings.TrimPrefix(line, "[ ] ")
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

	// Extract description
	if match := descRegex.FindStringSubmatch(line); len(match) > 1 {
		task.Description = match[1]
		line = descRegex.ReplaceAllString(line, "")
	}

	// Extract estimate
	if match := estRegex.FindStringSubmatch(line); match != nil {
		var totalMinutes int
		if match[1] != "" {
			var h int
			fmt.Sscanf(match[1], "%dh", &h)
			totalMinutes += h * 60
		}
		if match[2] != "" {
			var m int
			fmt.Sscanf(match[2], "%dm", &m)
			totalMinutes += m
		}
		if totalMinutes > 0 {
			d := time.Duration(totalMinutes) * time.Minute
			task.Estimate = &d
		}
		line = estRegex.ReplaceAllString(line, "")
	}

	// Extract start time
	if match := startRegex.FindStringSubmatch(line); len(match) > 1 {
		var h, m int
		fmt.Sscanf(match[1], "%d:%d", &h, &m)
		if h >= 0 && h <= 23 && m >= 0 && m <= 59 {
			task.StartTime = &TimeOfDay{Hour: h, Minute: m}
		}
		line = startRegex.ReplaceAllString(line, "")
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

	if t.Estimate != nil {
		totalMinutes := int(t.Estimate.Minutes())
		hours := totalMinutes / 60
		minutes := totalMinutes % 60
		sb.WriteString(" @est:")
		if hours > 0 {
			sb.WriteString(fmt.Sprintf("%dh", hours))
		}
		if minutes > 0 {
			sb.WriteString(fmt.Sprintf("%dm", minutes))
		}
	}

	if t.StartTime != nil {
		sb.WriteString(" @at:")
		sb.WriteString(t.StartTime.String())
	}

	if t.Description != "" {
		sb.WriteString(" @desc:\"")
		sb.WriteString(t.Description)
		sb.WriteString("\"")
	}

	if t.CreatedAt != nil {
		sb.WriteString(" || ")
		sb.WriteString(t.CreatedAt.Format("2006-01-02"))
	}

	return sb.String()
}

// StubString serializes only the reference fields for today's list.
// The source list is the single truth for metadata like @est, @at, @due, etc.
func (t *Task) StubString() string {
	var sb strings.Builder

	if t.Completed {
		sb.WriteString("[x] ")
	} else {
		sb.WriteString("[] ")
	}

	sb.WriteString(t.Content)

	if t.Source != "" {
		sb.WriteString(" @source:")
		sb.WriteString(t.Source)
	}

	return sb.String()
}

// IsReference returns true if this task is a reference stub (has a source).
func (t *Task) IsReference() bool {
	return t.Source != ""
}

// ResolveFrom copies metadata from a source task into this reference.
func (t *Task) ResolveFrom(source *Task) {
	t.DueDate = source.DueDate
	t.RecurDays = source.RecurDays
	t.Estimate = source.Estimate
	t.StartTime = source.StartTime
	t.Description = source.Description
	t.CreatedAt = source.CreatedAt
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
		Content:     t.Content,
		Description: t.Description,
		Completed:   false,
		DueDate:     &nextDue,
		RecurDays:   t.RecurDays,
		CreatedAt:   &now,
		Source:      "", // New occurrence goes to original list, not today
		Estimate:    t.Estimate,
	}
}
