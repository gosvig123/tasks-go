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
	CompletedAt *time.Time // @done:YYYY-MM-DD — set when task is completed
	RecurDays   int
	CreatedAt   *time.Time
	Source      string         // For today's list: which list this task came from
	Estimate    *time.Duration // @est:2h, @est:30m, @est:1h30m
	StartTime   *TimeOfDay     // @at:09:00, @at:14:30
	Subtasks    []*Task        // child tasks (single level only)
	Parent      *Task          // back-pointer to parent (nil for top-level, not serialized)
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
	doneRegex    = regexp.MustCompile(`@done:(\d{4}-\d{2}-\d{2})`)
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

	// Extract completion date
	if match := doneRegex.FindStringSubmatch(line); len(match) > 1 {
		if t, err := time.Parse("2006-01-02", match[1]); err == nil {
			task.CompletedAt = &t
		}
		line = doneRegex.ReplaceAllString(line, "")
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

// ParseLines parses multiple lines with indentation-aware subtask grouping.
// Lines indented with 2 spaces are attached as subtasks to the preceding top-level task.
// Orphan indented lines (no preceding parent) are treated as top-level tasks.
func ParseLines(lines []string) []*Task {
	var tasks []*Task
	var currentParent *Task

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		isIndented := strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ")

		if isIndented && currentParent != nil {
			parsed, err := Parse(strings.TrimPrefix(line, "  "))
			if err != nil {
				continue
			}
			parsed.Parent = currentParent
			currentParent.Subtasks = append(currentParent.Subtasks, parsed)
		} else {
			trimmed := strings.TrimSpace(line)
			parsed, err := Parse(trimmed)
			if err != nil {
				continue
			}
			tasks = append(tasks, parsed)
			currentParent = parsed
		}
	}

	return tasks
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

	if t.CompletedAt != nil {
		sb.WriteString(" @done:")
		sb.WriteString(t.CompletedAt.Format("2006-01-02"))
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

	result := sb.String()
	if len(t.Subtasks) > 0 {
		for _, sub := range t.Subtasks {
			result += "\n  " + sub.String()
		}
	}
	return result
}

// IsReference returns true if this task is a reference stub (has a source).
func (t *Task) IsReference() bool {
	return t.Source != ""
}

// NewReferenceStub creates a reference stub of t for today's list.
// All metadata is copied so it persists even if resolution fails.
func NewReferenceStub(t *Task, source string) *Task {
	stub := &Task{
		Content:     t.Content,
		Source:      source,
		DueDate:     t.DueDate,
		Estimate:    t.Estimate,
		StartTime:   t.StartTime,
		Description: t.Description,
		RecurDays:   t.RecurDays,
		CreatedAt:   t.CreatedAt,
	}
	for _, sub := range t.Subtasks {
		stubSub := &Task{
			Content:   sub.Content,
			Completed: sub.Completed,
			Source:    source,
			Parent:    stub,
		}
		stub.Subtasks = append(stub.Subtasks, stubSub)
	}
	return stub
}

// ResolveFrom merges metadata from a source task into this reference.
// Source is the single truth: completion status and all metadata are
// copied unconditionally. Nil/zero source values for optional metadata
// do not overwrite existing stub values (preserving locally-set fields
// like @est or @at assigned directly on the today task).
func (t *Task) ResolveFrom(source *Task) {
	t.Completed = source.Completed
	t.CompletedAt = source.CompletedAt

	if source.DueDate != nil {
		t.DueDate = source.DueDate
	}
	if source.RecurDays > 0 {
		t.RecurDays = source.RecurDays
	}
	if source.Estimate != nil {
		t.Estimate = source.Estimate
	}
	if source.StartTime != nil {
		t.StartTime = source.StartTime
	}
	if source.Description != "" {
		t.Description = source.Description
	}
	if source.CreatedAt != nil {
		t.CreatedAt = source.CreatedAt
	}

	// Resolve subtask metadata + completion by matching content
	for _, sub := range t.Subtasks {
		subContent := strings.TrimSpace(sub.Content)
		for _, srcSub := range source.Subtasks {
			if strings.TrimSpace(srcSub.Content) == subContent {
				sub.Completed = srcSub.Completed
				sub.CompletedAt = srcSub.CompletedAt
				if srcSub.DueDate != nil {
					sub.DueDate = srcSub.DueDate
				}
				if srcSub.RecurDays > 0 {
					sub.RecurDays = srcSub.RecurDays
				}
				if srcSub.Estimate != nil {
					sub.Estimate = srcSub.Estimate
				}
				if srcSub.StartTime != nil {
					sub.StartTime = srcSub.StartTime
				}
				if srcSub.Description != "" {
					sub.Description = srcSub.Description
				}
				if srcSub.CreatedAt != nil {
					sub.CreatedAt = srcSub.CreatedAt
				}
				break
			}
		}
	}
}

// AllSubtasksCompleted returns true if all subtasks are completed (or there are none).
func (t *Task) AllSubtasksCompleted() bool {
	for _, sub := range t.Subtasks {
		if !sub.Completed {
			return false
		}
	}
	return true
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
