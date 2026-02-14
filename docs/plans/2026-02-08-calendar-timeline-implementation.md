# Calendar Timeline View - Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a timeline calendar view as a split-panel default for the today list, with time estimates and pinned start times on tasks.

**Architecture:** Extend the Task model with two new fields (Estimate, StartTime) and their file tags (@est, @at). Add a new timeline rendering panel to the existing TaskViewModel in the UI. The timeline is a right-side panel that renders a vertical day view with proportional task blocks.

**Tech Stack:** Go, Bubble Tea, Lipgloss, Bubbles (textinput)

---

### Task 1: Add TimeOfDay type and new fields to Task model

**Files:**
- Modify: `internal/task/task.go:1-26` (imports, struct, regex vars)

**Changes:**

Add after the `Task` struct (line 18), before the regex vars:

```go
// TimeOfDay represents a time within a day (no date component)
type TimeOfDay struct {
	Hour   int // 0-23
	Minute int // 0-59
}

// String returns the time in HH:MM format
func (t TimeOfDay) String() string {
	return fmt.Sprintf("%02d:%02d", t.Hour, t.Minute)
}

// ToMinutes returns the total minutes since midnight
func (t TimeOfDay) ToMinutes() int {
	return t.Hour*60 + t.Minute
}
```

Add two new fields to the `Task` struct:

```go
type Task struct {
	Content     string
	Description string
	Completed   bool
	DueDate     *time.Time
	RecurDays   int
	CreatedAt   *time.Time
	Source      string
	Estimate    *time.Duration // @est:2h, @est:30m, @est:1h30m
	StartTime   *TimeOfDay     // @at:09:00, @at:14:30
}
```

Add two new regex patterns after the existing ones:

```go
estRegex     = regexp.MustCompile(`@est:(\d+h)?(\d+m)?`)
startRegex   = regexp.MustCompile(`@at:(\d{1,2}:\d{2})`)
```

**Verify:** `go build ./internal/task/...`

---

### Task 2: Update Parse() to handle @est and @at tags

**Files:**
- Modify: `internal/task/task.go:28-91` (Parse function)

**Changes:**

Add after the `@desc` extraction block (after line 87), before `task.Content = strings.TrimSpace(line)`:

```go
// Extract estimate
if match := estRegex.FindStringSubmatch(line); len(match) > 0 {
	var totalMinutes int
	fullMatch := match[0]
	if match[1] != "" {
		var hours int
		fmt.Sscanf(match[1], "%dh", &hours)
		totalMinutes += hours * 60
	}
	if match[2] != "" {
		var mins int
		fmt.Sscanf(match[2], "%dm", &mins)
		totalMinutes += mins
	}
	if totalMinutes > 0 {
		d := time.Duration(totalMinutes) * time.Minute
		task.Estimate = &d
	}
	line = strings.Replace(line, fullMatch, "", 1)
}

// Extract start time
if match := startRegex.FindStringSubmatch(line); len(match) > 1 {
	var hour, minute int
	fmt.Sscanf(match[1], "%d:%d", &hour, &minute)
	if hour >= 0 && hour <= 23 && minute >= 0 && minute <= 59 {
		task.StartTime = &TimeOfDay{Hour: hour, Minute: minute}
	}
	line = startRegex.ReplaceAllString(line, "")
}
```

**Verify:** `go build ./internal/task/...`

---

### Task 3: Update String() to serialize @est and @at tags

**Files:**
- Modify: `internal/task/task.go:93-131` (String function)

**Changes:**

Add after the `@source` block (after line 117), before the `@desc` block:

```go
if t.Estimate != nil {
	totalMinutes := int(t.Estimate.Minutes())
	hours := totalMinutes / 60
	mins := totalMinutes % 60
	sb.WriteString(" @est:")
	if hours > 0 {
		sb.WriteString(fmt.Sprintf("%dh", hours))
	}
	if mins > 0 {
		sb.WriteString(fmt.Sprintf("%dm", mins))
	}
	if hours == 0 && mins == 0 {
		sb.WriteString("0m")
	}
}

if t.StartTime != nil {
	sb.WriteString(" @at:")
	sb.WriteString(t.StartTime.String())
}
```

**Verify:** `go build ./internal/task/...`

---

### Task 4: Update CreateNextRecurrence to preserve Estimate

**Files:**
- Modify: `internal/task/task.go:148-166` (CreateNextRecurrence)

**Changes:**

Add `Estimate` to the returned Task struct in `CreateNextRecurrence()`:

```go
return &Task{
	Content:     t.Content,
	Description: t.Description,
	Completed:   false,
	DueDate:     &nextDue,
	RecurDays:   t.RecurDays,
	CreatedAt:   &now,
	Estimate:    t.Estimate,
	Source:      "",
}
```

**Verify:** `go build ./internal/task/...`

---

### Task 5: Update config with timeline defaults

**Files:**
- Modify: `internal/config/config.go`

**Changes:**

Add new fields to Config struct and defaults:

```go
type Config struct {
	TasksDir            string
	CurrentListFile     string
	LastResetFile       string
	DefaultList         string
	CleanupDays         int
	DayStartHour        int
	DayEndHour          int
	TimelineSlotMinutes int
}
```

Update `Default()`:

```go
DayStartHour:        8,
DayEndHour:          18,
TimelineSlotMinutes: 30,
```

**Verify:** `go build ./internal/config/...`

---

### Task 6: Add Estimate and StartTime fields to add/edit forms in UI

**Files:**
- Modify: `internal/ui/tasklist.go`

**Changes:**

This task updates the TaskViewModel to include `estInput` and `startInput` text inputs, updates `focusFormField`, `clearFormFields`, the add/edit key handlers (change `numFields` from 4 to 6), `renderTaskFormPanel`, and the save/add functions to handle the new fields.

Key areas to modify:
- Lines 100-104: Add `estInput` and `startInput` fields to TaskViewModel
- Lines 112-116: Add `pendingTaskEst` and `pendingTaskStart` fields  
- Lines 130-168: NewTaskViewModel — create estInput/startInput
- Lines 391-404: Update text input update handling for fields 4,5
- Lines 615-662: handleEditTaskKey — change numFields to 6
- Lines 664-715: saveEditedTask — handle est/start values
- Lines 748-767: focusFormField — add cases 4,5
- Lines 770-776: clearFormFields — clear est/start inputs
- Lines 778-853: handleAddTaskKey — change numFields to 6
- Lines 1557-1587: renderTaskFormPanel — add Est and Start fields
- Lines 580-613: startEditTask — populate est/start from task

**Verify:** `go build ./internal/ui/...`

---

### Task 7: Create timeline rendering engine (new file)

**Files:**
- Create: `internal/ui/timeline.go`

**Changes:**

Create a new file containing:

1. `TimelineSlot` struct — represents a task placed in the timeline with computed start/end times
2. `TimelineLayout` struct — holds the computed layout (slots, unscheduled tasks, time range)
3. `computeTimelineLayout(items []TaskItem, dayStart, dayEnd int) *TimelineLayout` — the layout algorithm
4. `renderTimeline(layout *TimelineLayout, panelWidth, panelHeight int, cursor int, focusedTaskIdx int, now time.Time) string` — renders the vertical timeline view

The layout algorithm:
- Separate tasks into pinned (has StartTime + Estimate), floating (has Estimate only), unscheduled
- Place pinned tasks at their start times
- Find gaps between pinned tasks, place floating tasks in gaps in order
- If floating tasks don't fit in gaps, stack after last pinned task
- Auto-expand dayStart/dayEnd if any task falls outside range
- Return computed layout with each task's row position

The rendering:
- Calculate rows per hour based on available panelHeight
- Draw time labels on the left (HH:MM │)
- Draw task blocks with proportional height
- Pinned blocks: solid border (┌─┐ │ │ └─┘)
- Floating blocks: shaded (░)
- Highlight the currently selected task
- Draw "now" marker line
- Draw unscheduled section at bottom

**Verify:** `go build ./internal/ui/...`

---

### Task 8: Integrate timeline panel into TaskViewModel

**Files:**
- Modify: `internal/ui/tasklist.go`

**Changes:**

Add to TaskViewModel struct:
- `showTimeline bool` — whether timeline panel is visible (default true for today list)
- `timelineFocus bool` — whether the right panel (timeline) has focus
- `timelineCursor int` — cursor index in timeline order

Update `loadSingleList()` (~line 225): set `m.showTimeline = true` when `listName == "today"`.

Update `View()` for `InputNormal` mode (~line 1330): when `m.showTimeline` is true, render split view with task list on left and timeline on right using `lipgloss.JoinHorizontal`.

Update `handleNormalKey()` (~line 434):
- Add `"c"` key to toggle `m.showTimeline`
- Add `"h"`, `"left"` to set `m.timelineFocus = false`
- Add `"l"`, `"right"` to set `m.timelineFocus = true` (only when timeline is visible)
- When `m.timelineFocus` is true, route j/k to timeline navigation and add J/K/s/S/e handlers

Add cursor sync: when cursor changes in either panel, find the corresponding task in the other panel's ordering.

**Verify:** `go build ./internal/ui/...`

---

### Task 9: Add timeline-specific keybindings (J/K, s, S, e in timeline)

**Files:**
- Modify: `internal/ui/tasklist.go`

**Changes:**

Add a new method `handleTimelineKey(msg tea.KeyMsg)` that handles:
- `j/k`, `up/down`: navigate timeline cursor
- `J` (shift+j): move task down in timeline (floating: reorder, pinned: +30m)
- `K` (shift+k): move task up in timeline (floating: reorder, pinned: -30m)
- `s`: prompt for start time (show small inline input to pin the task)
- `S`: unpin task (remove StartTime, make floating)
- `e`: prompt for estimate (show small inline input)
- `tab/space`: toggle completion
- `d`: delete task

Add two new InputMode values:
- `InputTimelineSetTime` — inline input for setting start time
- `InputTimelineSetEstimate` — inline input for setting estimate

Route timeline key handling from `handleKey()` when `m.timelineFocus` is true.

**Verify:** `go build ./internal/ui/...`

---

### Task 10: Update copyTaskForToday to preserve Estimate and StartTime

**Files:**
- Modify: `internal/ui/tasklist.go:1154-1163`

**Changes:**

Add `Estimate` and `StartTime` to the copied task:

```go
func copyTaskForToday(t *task.Task, source string) *task.Task {
	return &task.Task{
		Content:     t.Content,
		Description: t.Description,
		Completed:   false,
		DueDate:     t.DueDate,
		RecurDays:   t.RecurDays,
		Source:      source,
		Estimate:    t.Estimate,
		StartTime:   t.StartTime,
	}
}
```

**Verify:** `go build ./internal/ui/...`

---

### Task 11: Add CLI command aliases for calendar view

**Files:**
- Modify: `main.go:34-108` (switch statement)

**Changes:**

The `"t"` alias is currently used for `toggle`. Change it:
- Add `"cal"` as a new command that switches to today list and runs interactive
- Update help text

In the switch statement, add:

```go
case "cal":
	store.SetCurrentList("today")
	runInteractive()
```

Update `showHelp()` to include the new command.

**Verify:** `go build .`

---

### Task 12: Build and integration test

**Commands:**
```bash
go vet ./...
go build -o tasks .
```

Manually test:
1. Add tasks with `@est:2h` and `@at:09:00` to a today list file
2. Run `tasks l` on the today list — verify split view renders
3. Test `c` toggle
4. Test h/l focus switching
5. Test J/K reordering in timeline
6. Test s/S to pin/unpin
7. Test add/edit forms with estimate and start time fields
