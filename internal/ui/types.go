package ui

import (
	"github.com/krisitan/tasks-go/internal/storage"
	"github.com/krisitan/tasks-go/internal/task"
)

// ViewMode determines what tasks to show and how
type ViewMode int

const (
	ViewSingleList ViewMode = iota // Single list view (tasks l)
	ViewAllPending                 // All pending tasks (tasks la)
	ViewPicker                     // Multi-select picker (tasks add today)
)

// InputMode determines current input state
type InputMode int

const (
	InputNormal InputMode = iota
	InputAddTask
	InputEditTask          // Edit task in split panel
	InputViewTask          // View task details in split panel
	InputSelectListForTask // Select which list to add task to
	InputSearch
	InputTimelineSetTime     // Inline input for setting start time in timeline
	InputTimelineSetEstimate // Inline input for setting estimate in timeline
	InputAddSubtask          // Adding a subtask to focused parent
)

// TaskItem wraps a task with its source list info
type TaskItem struct {
	Task      *task.Task
	ListName  string
	Index     int  // original index in list (parent index for subtasks)
	SubIndex  int  // -1 for top-level, 0+ for subtask position
	IsSubtask bool // controls indented rendering
}

// RunTaskListResult indicates the result of running the task list
type RunTaskListResult int

const (
	ResultQuit RunTaskListResult = iota
	ResultSwitchList
)

type AddedToTodayMsg struct {
	TaskName string
}

type tasksLoadedMsg struct {
	items        []TaskItem
	listName     string
	taskList     *task.TaskList
	showLists    *bool // nil = don't change, non-nil = set to value
	showTimeline bool
}

type listsLoadedMsg struct {
	lists []string
}

// timelineRefreshMsg signals that the timeline layout should be recomputed
// from the current in-memory items (no disk reload needed).
type timelineRefreshMsg struct {
	cursorIdx int // timeline cursor to restore after refresh (-1 = don't change)
}

// refreshDisplayMsg signals the timeline to recompute from current in-memory
// state without resetting the list cursor. Used after in-place task edits.
type refreshDisplayMsg struct{}

// timerTickMsg is sent every second while the timer is running.
// gen prevents duplicate tick chains: only the latest generation is honoured.
type timerTickMsg struct{ gen int }

type listSwitcherLoadedMsg struct {
	lists     []string
	infoCache map[string]*storage.ListInfo
}
