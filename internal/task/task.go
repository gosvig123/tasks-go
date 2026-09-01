package task

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

type Task struct {
	ID          string
	Content     string
	Description string // Optional description for the task
	Completed   bool
	DueDate     *time.Time
	CompletedAt *time.Time // @done:YYYY-MM-DD — set when task is completed
	RecurDays   int
	CreatedAt   *time.Time
	Source      string         // For today's list: which list this task came from
	Estimate    *time.Duration // @est:2h, @est:30m, @est:1h30m
	Tracked     *time.Duration // @tracked:1h30m45s — accumulated time tracking
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
	idRegex      = regexp.MustCompile(`@id:([0-9a-fA-F-]{32,36})`)
	dueRegex     = regexp.MustCompile(`@due:(\d{4}-\d{2}-\d{2})`)
	doneRegex    = regexp.MustCompile(`@done:(\d{4}-\d{2}-\d{2})`)
	recurRegex   = regexp.MustCompile(`@recur:(\d+)`)
	sourceRegex  = regexp.MustCompile(`@source:(?:"([^"]+)"|([a-zA-Z0-9_-]+))`)
	estRegex     = regexp.MustCompile(`@est:(\d+h)?(\d+m)?`)
	trackedRegex = regexp.MustCompile(`@tracked:(\d+h)?(\d+m)?(\d+s)?`)
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

	// Extract stable identity.
	if match := idRegex.FindStringSubmatch(line); len(match) > 1 {
		task.ID = strings.ToLower(match[1])
		line = idRegex.ReplaceAllString(line, "")
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

	// Extract source (group 1 = quoted, group 2 = unquoted)
	if match := sourceRegex.FindStringSubmatch(line); len(match) > 1 {
		if match[1] != "" {
			task.Source = match[1]
		} else {
			task.Source = match[2]
		}
		line = sourceRegex.ReplaceAllString(line, "")
	}

	// Extract description
	if description, remainder, ok := extractDescription(line); ok {
		task.Description = description
		line = remainder
	}

	// Extract estimate
	if match := estRegex.FindStringSubmatch(line); match != nil && (match[1] != "" || match[2] != "") {
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

	// Extract tracked time
	if match := trackedRegex.FindStringSubmatch(line); match != nil && (match[1] != "" || match[2] != "" || match[3] != "") {
		var totalSeconds int
		if match[1] != "" {
			var h int
			fmt.Sscanf(match[1], "%dh", &h)
			totalSeconds += h * 3600
		}
		if match[2] != "" {
			var m int
			fmt.Sscanf(match[2], "%dm", &m)
			totalSeconds += m * 60
		}
		if match[3] != "" {
			var s int
			fmt.Sscanf(match[3], "%ds", &s)
			totalSeconds += s
		}
		if totalSeconds > 0 {
			d := time.Duration(totalSeconds) * time.Second
			task.Tracked = &d
		}
		line = trackedRegex.ReplaceAllString(line, "")
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
		parsed, ok := parseTaskLine(line)
		if !ok {
			continue
		}
		isIndented := strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ")
		if isIndented && currentParent != nil {
			parsed.Parent = currentParent
			currentParent.Subtasks = append(currentParent.Subtasks, parsed)
			continue
		}
		tasks = append(tasks, parsed)
		currentParent = parsed
	}
	return tasks
}

func parseTaskLine(line string) (*Task, bool) {
	trimmed := strings.TrimSpace(line)
	if !hasTaskMarker(trimmed) {
		return nil, false
	}
	parsed, err := Parse(trimmed)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: skipping malformed task line: %q\n", trimmed)
		return nil, false
	}
	return parsed, true
}

func hasTaskMarker(line string) bool {
	return strings.HasPrefix(line, "[x]") ||
		strings.HasPrefix(line, "[X]") ||
		strings.HasPrefix(line, "[ ]") ||
		strings.HasPrefix(line, "[]")
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

	if t.ID != "" {
		sb.WriteString(" @id:")
		sb.WriteString(t.ID)
	}

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
		if strings.ContainsAny(t.Source, " \t") {
			sb.WriteString(fmt.Sprintf(` @source:"%s"`, t.Source))
		} else {
			sb.WriteString(" @source:")
			sb.WriteString(t.Source)
		}
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

	if t.Tracked != nil {
		totalSeconds := int(t.Tracked.Seconds())
		hours := totalSeconds / 3600
		minutes := (totalSeconds % 3600) / 60
		seconds := totalSeconds % 60
		sb.WriteString(" @tracked:")
		if hours > 0 {
			sb.WriteString(fmt.Sprintf("%dh", hours))
		}
		if minutes > 0 {
			sb.WriteString(fmt.Sprintf("%dm", minutes))
		}
		if seconds > 0 || (hours == 0 && minutes == 0) {
			sb.WriteString(fmt.Sprintf("%ds", seconds))
		}
	}

	if t.StartTime != nil {
		sb.WriteString(" @at:")
		sb.WriteString(t.StartTime.String())
	}

	if t.Description != "" {
		sb.WriteString(" @desc:\"")
		sb.WriteString(encodeDescription(t.Description))
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

// TodayDate returns today's date at local midnight.
func TodayDate() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

// EnsureDueToday sets the task's due date to today if it's nil.
func (t *Task) EnsureDueToday() {
	if t.DueDate == nil {
		d := TodayDate()
		t.DueDate = &d
	}
}

// IsReference returns true if this task is a reference stub (has a source).
func (t *Task) IsReference() bool {
	return t.Source != ""
}

// NewReferenceStub creates a reference stub of t for today's list.
// All metadata is copied so it persists even if resolution fails.
func NewReferenceStub(t *Task, source string) *Task {
	stub := &Task{
		ID:          t.ID,
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
			ID:        sub.ID,
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
// Source is the single truth for scheduling/identity fields (DueDate, RecurDays,
// Description, CreatedAt, completion). These are always overwritten, even when
// the source value is nil/zero, so removals propagate to today stubs.
// Estimate and StartTime are authoritative on the source task, including clears.
func (t *Task) ResolveFrom(source *Task) {
	if t.ID == "" {
		t.ID = source.ID
	}
	t.Content = source.Content
	t.Completed = source.Completed
	t.CompletedAt = source.CompletedAt

	// Always sync from source (source is truth; removals must propagate)
	t.DueDate = source.DueDate
	t.RecurDays = source.RecurDays
	t.Description = source.Description
	t.CreatedAt = source.CreatedAt

	t.Estimate = source.Estimate
	t.StartTime = source.StartTime

	existing := make(map[string]*Task)
	for _, sub := range t.Subtasks {
		existing[subtaskKey(sub)] = sub
	}

	resolved := make([]*Task, 0, len(source.Subtasks))
	for _, srcSub := range source.Subtasks {
		sub, ok := existing[subtaskKey(srcSub)]
		if !ok {
			sub = &Task{ID: srcSub.ID, Content: srcSub.Content, Source: t.Source, Parent: t}
		} else {
			sub.Parent = t
			sub.ID = srcSub.ID
			if sub.Source == "" {
				sub.Source = t.Source
			}
		}

		sub.Content = srcSub.Content
		sub.Completed = srcSub.Completed
		sub.CompletedAt = srcSub.CompletedAt
		sub.DueDate = srcSub.DueDate
		sub.RecurDays = srcSub.RecurDays
		sub.Description = srcSub.Description
		sub.CreatedAt = srcSub.CreatedAt
		sub.Estimate = srcSub.Estimate
		sub.StartTime = srcSub.StartTime

		resolved = append(resolved, sub)
	}

	t.Subtasks = resolved
}

func subtaskKey(t *Task) string {
	if t.ID != "" {
		return "id:" + t.ID
	}
	return "content:" + strings.TrimSpace(t.Content)
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
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dueLocal := time.Date(t.DueDate.Year(), t.DueDate.Month(), t.DueDate.Day(), 0, 0, 0, 0, now.Location())
	return !dueLocal.After(today)
}

// IsDueWithin returns true if the task is due after today and within the given number of days
func (t *Task) IsDueWithin(days int) bool {
	if t.DueDate == nil || t.Completed {
		return false
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dueLocal := time.Date(t.DueDate.Year(), t.DueDate.Month(), t.DueDate.Day(), 0, 0, 0, 0, now.Location())
	if !dueLocal.After(today) {
		return false
	}
	cutoff := today.AddDate(0, 0, days)
	return !dueLocal.After(cutoff)
}

// CreateNextRecurrence creates the next occurrence of a recurring task
func (t *Task) CreateNextRecurrence() *Task {
	if t.RecurDays <= 0 {
		return nil
	}

	nextDue := time.Now().AddDate(0, 0, t.RecurDays)
	now := time.Now()

	return &Task{
		ID:          NewID(),
		Content:     t.Content,
		Description: t.Description,
		Completed:   false,
		DueDate:     &nextDue,
		RecurDays:   t.RecurDays,
		CreatedAt:   &now,
		Source:      "", // New occurrence goes to original list, not today
		Estimate:    t.Estimate,
		StartTime:   t.StartTime,
	}
}
