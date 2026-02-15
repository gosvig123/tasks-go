package ui

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/krisitan/tasks-go/internal/storage"
	"github.com/krisitan/tasks-go/internal/task"
)

var debugLog *log.Logger

func init() {
	f, err := os.OpenFile("/tmp/tasks-debug.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		debugLog = log.New(os.Stderr, "", 0)
	} else {
		debugLog = log.New(f, "", log.Lmicroseconds)
	}
}

// Styles - consistent across all views
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#e5c07b"))

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#61afef")).
			Bold(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#abb2bf"))

	completedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5c6370")).
			Faint(true)

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5c6370"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5c6370"))

	separatorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5c6370")).
			Faint(true)

	// Panel styles for split-screen view
	panelBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#5c6370")).
				Padding(0, 1)

	focusedPanelBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#61afef")).
				Padding(0, 1)

	addPanelTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#98c379"))
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

// TaskViewModel is the unified model for all task views
type TaskViewModel struct {
	storage *storage.Storage

	// View configuration
	viewMode  ViewMode
	listName  string // for single list mode
	showLists bool   // show list column

	// Task data
	items    []TaskItem
	allItems []TaskItem
	taskList *task.TaskList // for single list mode

	// UI state
	cursor       int
	selected     map[int]bool // for picker mode
	inputMode    InputMode
	textInput    textinput.Model
	descInput    textinput.Model // description input
	dueInput     textinput.Model // due date input (days offset or YYYY-MM-DD)
	recurInput   textinput.Model // recurrence days input
	estInput     textinput.Model // estimate input (e.g. "2h", "30m", "1h30m")
	startInput   textinput.Model // start time input (e.g. "09:00")
	focusedField int             // 0=title, 1=desc, 2=due, 3=recur, 4=est, 5=start
	searchInput  textinput.Model
	searchQuery  string

	// List switcher
	lists      []string
	listCursor int

	// Pending task (for list selection before adding)
	pendingTaskContent string
	pendingTaskDesc    string
	pendingTaskDue     string
	pendingTaskRecur   string
	pendingTaskEst     string
	pendingTaskStart   string

	// Subtask state
	subtaskParentIdx int // index in items[] of the parent being added to

	// Terminal size
	width  int
	height int

	// State
	quitting        bool
	wantsSwitchList bool   // exit to switch list
	added           int    // for picker mode: count of added items
	statusMsg       string // temporary status message

	// Timeline state
	showTimeline   bool            // whether timeline panel is visible
	timelineFocus  bool            // whether the right panel (timeline) has focus
	timelineCursor int             // cursor index within timeline-ordered tasks
	timelineLayout *TimelineLayout // computed timeline layout
}

// NewTaskViewModel creates a new task view model
func NewTaskViewModel(store *storage.Storage, mode ViewMode) *TaskViewModel {
	ti := textinput.New()
	ti.Placeholder = "Enter task..."
	ti.CharLimit = 200
	ti.Width = 50

	si := textinput.New()
	si.Placeholder = "Search..."
	si.CharLimit = 100
	si.Width = 40

	di := textinput.New()
	di.Placeholder = "Description (optional)..."
	di.CharLimit = 500
	di.Width = 50

	dueIn := textinput.New()
	dueIn.Placeholder = "+7 or 2024-12-31"
	dueIn.CharLimit = 20
	dueIn.Width = 20

	recurIn := textinput.New()
	recurIn.Placeholder = "e.g. 7 for weekly"
	recurIn.CharLimit = 10
	recurIn.Width = 20

	estIn := textinput.New()
	estIn.Placeholder = "e.g. 2h, 30m, 1h30m"
	estIn.CharLimit = 10
	estIn.Width = 20

	startIn := textinput.New()
	startIn.Placeholder = "e.g. 09:00, 14:30"
	startIn.CharLimit = 5
	startIn.Width = 20

	return &TaskViewModel{
		storage:     store,
		viewMode:    mode,
		selected:    make(map[int]bool),
		textInput:   ti,
		descInput:   di,
		dueInput:    dueIn,
		recurInput:  recurIn,
		estInput:    estIn,
		startInput:  startIn,
		searchInput: si,
		inputMode:   InputNormal,
		showLists:   mode != ViewSingleList,
	}
}

func (m *TaskViewModel) Init() tea.Cmd {
	return m.loadTasks()
}

// Message types
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

type AddedToTodayMsg struct {
	TaskName string
}

// timelineRefreshMsg signals that the timeline layout should be recomputed
// from the current in-memory items (no disk reload needed).
type timelineRefreshMsg struct {
	cursorIdx int // timeline cursor to restore after refresh (-1 = don't change)
}

func (m *TaskViewModel) loadTasks() tea.Cmd {
	return func() tea.Msg {
		switch m.viewMode {
		case ViewSingleList:
			return m.loadSingleList()
		case ViewAllPending:
			return m.loadAllPending()
		case ViewPicker:
			return m.loadPicker()
		}
		return nil
	}
}

// sortByDueDate sorts TaskItems so tasks with dates appear first (earliest first),
// then tasks without dates.
func sortByDueDate(items []TaskItem) {
	sort.Slice(items, func(i, j int) bool {
		di, dj := items[i].Task.DueDate, items[j].Task.DueDate
		// Both have dates: sort by date
		if di != nil && dj != nil {
			return di.Before(*dj)
		}
		// Only i has date: i comes first
		if di != nil && dj == nil {
			return true
		}
		// Only j has date: j comes first
		if di == nil && dj != nil {
			return false
		}
		// Neither has date: keep original order
		return false
	})
}

func (m *TaskViewModel) loadSingleList() tea.Msg {
	listName := m.storage.GetCurrentList()

	isToday := listName == "today"
	if isToday {
		m.storage.ResetTodayList()
	}

	taskList, err := m.storage.LoadList(listName)
	if err != nil {
		return nil
	}

	// Separate uncompleted and completed
	var uncompleted, completed []TaskItem
	for i, t := range taskList.Tasks {
		// For today list, use Source as the list name to show origin
		itemListName := listName
		if isToday && t.Source != "" {
			itemListName = t.Source
		}
		parentItem := TaskItem{
			Task:     t,
			ListName: itemListName,
			Index:    i,
			SubIndex: -1,
		}
		if t.Completed {
			completed = append(completed, parentItem)
		} else {
			uncompleted = append(uncompleted, parentItem)
		}
		// Flatten subtasks immediately after parent
		for si, sub := range t.Subtasks {
			subItemListName := itemListName
			if isToday && sub.Source != "" {
				subItemListName = sub.Source
			}
			subItem := TaskItem{
				Task:      sub,
				ListName:  subItemListName,
				Index:     i,
				SubIndex:  si,
				IsSubtask: true,
			}
			// Group with parent (same bucket)
			if t.Completed {
				completed = append(completed, subItem)
			} else {
				uncompleted = append(uncompleted, subItem)
			}
		}
	}

	// Uncompleted first, then completed (ensure non-nil so View() distinguishes "loaded empty" from "not loaded")
	items := make([]TaskItem, 0, len(uncompleted)+len(completed))
	items = append(items, uncompleted...)
	items = append(items, completed...)

	return tasksLoadedMsg{
		items:        items,
		listName:     listName,
		taskList:     taskList,
		showLists:    &isToday,
		showTimeline: isToday,
	}
}

func (m *TaskViewModel) loadAllPending() tea.Msg {
	lists, err := m.storage.GetAllLists()
	if err != nil {
		return nil
	}

	m.storage.ResetTodayList()

	var uncompleted, completed []TaskItem
	for _, listName := range lists {
		list, err := m.storage.LoadList(listName)
		if err != nil {
			continue
		}

		for i, t := range list.Tasks {
			parentItem := TaskItem{
				Task:     t,
				ListName: listName,
				Index:    i,
				SubIndex: -1,
			}
			if t.Completed {
				completed = append(completed, parentItem)
			} else {
				uncompleted = append(uncompleted, parentItem)
			}
			// Flatten subtasks immediately after parent
			for si, sub := range t.Subtasks {
				subItem := TaskItem{
					Task:      sub,
					ListName:  listName,
					Index:     i,
					SubIndex:  si,
					IsSubtask: true,
				}
				if t.Completed {
					completed = append(completed, subItem)
				} else {
					uncompleted = append(uncompleted, subItem)
				}
			}
		}
	}

	// Uncompleted first, then completed (ensure non-nil so View() distinguishes "loaded empty" from "not loaded")
	items := make([]TaskItem, 0, len(uncompleted)+len(completed))
	items = append(items, uncompleted...)
	items = append(items, completed...)

	return tasksLoadedMsg{items: items}
}

func (m *TaskViewModel) loadPicker() tea.Msg {
	lists, err := m.storage.GetAllLists()
	if err != nil {
		return nil
	}

	items := make([]TaskItem, 0)
	for _, listName := range lists {
		if listName == "today" {
			continue
		}

		list, err := m.storage.LoadList(listName)
		if err != nil {
			continue
		}

		for i, t := range list.Tasks {
			if !t.Completed {
				items = append(items, TaskItem{
					Task:     t,
					ListName: listName,
					Index:    i,
					SubIndex: -1,
				})
				// Flatten subtasks immediately after parent
				for si, sub := range t.Subtasks {
					if !sub.Completed {
						items = append(items, TaskItem{
							Task:      sub,
							ListName:  listName,
							Index:     i,
							SubIndex:  si,
							IsSubtask: true,
						})
					}
				}
			}
		}
	}

	return tasksLoadedMsg{items: items}
}

func (m *TaskViewModel) loadLists() tea.Cmd {
	return func() tea.Msg {
		lists, err := m.storage.GetAllLists()
		if err != nil {
			return nil
		}
		return listsLoadedMsg{lists: lists}
	}
}

func (m *TaskViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tasksLoadedMsg:
		m.items = msg.items
		m.allItems = msg.items
		m.listName = msg.listName
		m.taskList = msg.taskList
		if msg.showLists != nil {
			m.showLists = *msg.showLists
		}
		m.showTimeline = msg.showTimeline
		m.cursor = 0
		m.searchQuery = ""
		if m.showTimeline {
			m.timelineLayout = computeTimelineLayout(m.items, 8, 18)
		}
		return m, nil

	case listsLoadedMsg:
		m.lists = msg.lists
		m.listCursor = 0
		for i, name := range m.lists {
			if name == m.listName {
				m.listCursor = i
				break
			}
		}
		return m, nil

	case AddedToTodayMsg:
		m.statusMsg = fmt.Sprintf("Added to today: %s", msg.TaskName)
		return m, nil

	case timelineRefreshMsg:
		if m.showTimeline {
			m.timelineLayout = computeTimelineLayout(m.items, 8, 18)
			if msg.cursorIdx >= 0 {
				m.timelineCursor = msg.cursorIdx
			}
			m.syncListCursorFromTimeline()
		}
		return m, nil
	}

	// Handle text input updates for timeline inline inputs
	if m.inputMode == InputTimelineSetTime {
		var cmd tea.Cmd
		m.startInput, cmd = m.startInput.Update(msg)
		return m, cmd
	}

	if m.inputMode == InputTimelineSetEstimate {
		var cmd tea.Cmd
		m.estInput, cmd = m.estInput.Update(msg)
		return m, cmd
	}

	// Handle text input updates for form modes
	if m.inputMode == InputAddTask || m.inputMode == InputEditTask {
		var cmd tea.Cmd
		switch m.focusedField {
		case 0:
			m.textInput, cmd = m.textInput.Update(msg)
		case 1:
			m.descInput, cmd = m.descInput.Update(msg)
		case 2:
			m.dueInput, cmd = m.dueInput.Update(msg)
		case 3:
			m.recurInput, cmd = m.recurInput.Update(msg)
		case 4:
			m.estInput, cmd = m.estInput.Update(msg)
		case 5:
			m.startInput, cmd = m.startInput.Update(msg)
		}
		return m, cmd
	}

	if m.inputMode == InputSearch {
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		m.searchQuery = m.searchInput.Value()
		m.filterTasks()
		return m, cmd
	}

	return m, nil
}

func (m *TaskViewModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.inputMode {
	case InputAddTask:
		return m.handleAddTaskKey(msg)
	case InputEditTask:
		return m.handleEditTaskKey(msg)
	case InputViewTask:
		return m.handleViewTaskKey(msg)
	case InputSelectListForTask:
		return m.handleSelectListForTaskKey(msg)
	case InputSearch:
		return m.handleSearchKey(msg)
	case InputTimelineSetTime:
		return m.handleTimelineSetTimeKey(msg)
	case InputTimelineSetEstimate:
		return m.handleTimelineSetEstimateKey(msg)
	case InputAddSubtask:
		return m.handleAddSubtaskKey(msg)
	default:
		return m.handleNormalKey(msg)
	}
}

func (m *TaskViewModel) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Route to timeline key handler when timeline has focus
	if m.timelineFocus && m.showTimeline {
		return m.handleTimelineKey(msg)
	}

	// Clear any status message on key press
	m.statusMsg = ""

	switch msg.String() {
	case "q", "esc":
		m.quitting = true
		return m, tea.Quit

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}

	case "down", "j":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}

	case "enter":
		if m.viewMode == ViewPicker {
			return m, m.addSelectedToToday()
		}
		if len(m.items) > 0 {
			m.inputMode = InputViewTask
			return m, nil
		}

	case "tab":
		if len(m.items) > 0 {
			if m.viewMode == ViewPicker {
				m.selected[m.cursor] = !m.selected[m.cursor]
			} else {
				return m, m.toggleTask(m.cursor)
			}
		}

	case " ":
		if len(m.items) > 0 {
			if m.viewMode == ViewPicker {
				m.selected[m.cursor] = !m.selected[m.cursor]
			} else {
				return m, m.toggleTask(m.cursor)
			}
		}

	case "a":
		if m.viewMode == ViewPicker {
			// Select all visible
			for i := range m.items {
				m.selected[i] = true
			}
		} else if m.viewMode == ViewSingleList || m.viewMode == ViewAllPending {
			m.inputMode = InputAddTask
			m.clearFormFields()
			return m, m.focusFormField(0)
		}

	case "n":
		if m.viewMode == ViewPicker {
			// Deselect all
			m.selected = make(map[int]bool)
		}

	case "A":
		if (m.viewMode == ViewSingleList || m.viewMode == ViewAllPending) && len(m.items) > 0 {
			return m, m.addCurrentTaskToToday()
		}

	case "L":
		if m.viewMode == ViewSingleList {
			m.wantsSwitchList = true
			return m, tea.Quit
		}

	case "d":
		if (m.viewMode == ViewSingleList || m.viewMode == ViewAllPending) && len(m.items) > 0 {
			return m, m.deleteTask(m.cursor)
		}

	case "e":
		if (m.viewMode == ViewSingleList || m.viewMode == ViewAllPending) && len(m.items) > 0 {
			return m, m.startEditTask()
		}

	case "s":
		if (m.viewMode == ViewSingleList || m.viewMode == ViewAllPending) && len(m.items) > 0 {
			item := m.items[m.cursor]
			// Only allow on top-level tasks (not subtasks)
			if !item.IsSubtask {
				m.subtaskParentIdx = m.cursor
				m.inputMode = InputAddSubtask
				m.clearFormFields()
				return m, m.focusFormField(0)
			}
		}

	case "c":
		if m.listName == "today" || m.showTimeline {
			m.showTimeline = !m.showTimeline
			if !m.showTimeline {
				m.timelineFocus = false
			}
		}

	case "l", "right":
		if m.showTimeline && !m.timelineFocus {
			m.timelineFocus = true
			m.syncTimelineCursor()
		}

	case "h", "left":
		if m.timelineFocus {
			m.timelineFocus = false
		}

	case "/":
		m.inputMode = InputSearch
		m.searchInput.SetValue("")
		m.searchInput.Focus()
		return m, textinput.Blink
	}

	return m, nil
}

func (m *TaskViewModel) handleViewTaskKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.inputMode = InputNormal
		return m, nil

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case "down", "j":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
		return m, nil

	case "tab", " ":
		// Toggle task completion
		if len(m.items) > 0 {
			return m, m.toggleTask(m.cursor)
		}

	case "d":
		// Delete task
		if len(m.items) > 0 {
			cmd := m.deleteTask(m.cursor)
			// If we deleted the last task, go back to normal mode
			if len(m.items) <= 1 {
				m.inputMode = InputNormal
			}
			return m, cmd
		}

	case "A":
		// Add to today
		if len(m.items) > 0 {
			return m, m.addCurrentTaskToToday()
		}

	case "e":
		// Edit task
		if len(m.items) > 0 {
			return m, m.startEditTask()
		}
	}

	return m, nil
}

func (m *TaskViewModel) startEditTask() tea.Cmd {
	return func() tea.Msg {
		if m.cursor >= len(m.items) {
			return nil
		}
		item := m.items[m.cursor]

		// Populate form fields with existing task data
		m.textInput.SetValue(item.Task.Content)
		m.descInput.SetValue(item.Task.Description)

		// Format due date for display
		if item.Task.DueDate != nil {
			m.dueInput.SetValue(item.Task.DueDate.Format("2006-01-02"))
		} else {
			m.dueInput.SetValue("")
		}

		// Set recurrence
		if item.Task.RecurDays > 0 {
			m.recurInput.SetValue(fmt.Sprintf("%d", item.Task.RecurDays))
		} else {
			m.recurInput.SetValue("")
		}

		// Set estimate
		if item.Task.Estimate != nil {
			totalMinutes := int(item.Task.Estimate.Minutes())
			hours := totalMinutes / 60
			mins := totalMinutes % 60
			if hours > 0 && mins > 0 {
				m.estInput.SetValue(fmt.Sprintf("%dh%dm", hours, mins))
			} else if hours > 0 {
				m.estInput.SetValue(fmt.Sprintf("%dh", hours))
			} else {
				m.estInput.SetValue(fmt.Sprintf("%dm", mins))
			}
		} else {
			m.estInput.SetValue("")
		}

		// Set start time
		if item.Task.StartTime != nil {
			m.startInput.SetValue(item.Task.StartTime.String())
		} else {
			m.startInput.SetValue("")
		}

		m.focusedField = 0
		m.textInput.Focus()
		m.descInput.Blur()
		m.dueInput.Blur()
		m.recurInput.Blur()
		m.estInput.Blur()
		m.startInput.Blur()
		m.inputMode = InputEditTask
		return textinput.Blink()
	}
}

func (m *TaskViewModel) handleEditTaskKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	const numFields = 6

	switch msg.String() {
	case "esc":
		m.inputMode = InputNormal
		m.textInput.Blur()
		m.descInput.Blur()
		m.dueInput.Blur()
		m.recurInput.Blur()
		m.estInput.Blur()
		m.startInput.Blur()
		m.focusedField = 0
		return m, nil

	case "tab":
		nextField := (m.focusedField + 1) % numFields
		return m, m.focusFormField(nextField)

	case "shift+tab":
		prevField := (m.focusedField - 1 + numFields) % numFields
		return m, m.focusFormField(prevField)

	case "enter":
		content := strings.TrimSpace(m.textInput.Value())
		if content != "" {
			m.textInput.Blur()
			m.descInput.Blur()
			m.dueInput.Blur()
			m.recurInput.Blur()
			m.estInput.Blur()
			m.startInput.Blur()
			m.focusedField = 0
			return m, m.saveEditedTask(content, m.descInput.Value(), m.dueInput.Value(), m.recurInput.Value(), m.estInput.Value(), m.startInput.Value())
		}
		return m, nil
	}

	// Handle text input for focused field
	var cmd tea.Cmd
	switch m.focusedField {
	case 0:
		m.textInput, cmd = m.textInput.Update(msg)
	case 1:
		m.descInput, cmd = m.descInput.Update(msg)
	case 2:
		m.dueInput, cmd = m.dueInput.Update(msg)
	case 3:
		m.recurInput, cmd = m.recurInput.Update(msg)
	case 4:
		m.estInput, cmd = m.estInput.Update(msg)
	case 5:
		m.startInput, cmd = m.startInput.Update(msg)
	}
	return m, cmd
}

func (m *TaskViewModel) saveEditedTask(content, description, dueValue, recurValue, estValue, startValue string) tea.Cmd {
	return func() tea.Msg {
		if m.cursor >= len(m.items) {
			return nil
		}

		item := m.items[m.cursor]

		// Parse due date and recurrence
		dueOffset, specificDate := parseDueValue(dueValue)
		recurDays := parseRecurValue(recurValue)
		estimate := parseEstimateValue(estValue)
		startTime := parseStartTimeValue(startValue)

		// Update the task
		item.Task.Content = content
		item.Task.Description = description
		item.Task.RecurDays = recurDays
		item.Task.Estimate = estimate
		item.Task.StartTime = startTime

		// Update due date
		if specificDate != nil {
			item.Task.DueDate = specificDate
		} else if dueOffset > 0 {
			due := time.Now().AddDate(0, 0, dueOffset)
			item.Task.DueDate = &due
		} else if dueValue == "" {
			// Clear due date if field is empty
			item.Task.DueDate = nil
		}

		// Save to the appropriate list
		listName := m.listName
		if m.viewMode == ViewAllPending {
			listName = item.ListName
		}

		list, err := m.storage.LoadList(listName)
		if err != nil {
			return nil
		}

		// Find and update the task in the list
		if item.IsSubtask {
			if item.Index < len(list.Tasks) {
				parent := list.Tasks[item.Index]
				if item.SubIndex < len(parent.Subtasks) {
					sub := parent.Subtasks[item.SubIndex]
					sub.Content = content
					sub.Description = description
					sub.DueDate = item.Task.DueDate
					sub.RecurDays = recurDays
					sub.Estimate = estimate
					sub.StartTime = startTime
				}
			}
			m.storage.SaveList(list)
		} else if item.Index < len(list.Tasks) {
			list.Tasks[item.Index].Content = content
			list.Tasks[item.Index].Description = description
			list.Tasks[item.Index].DueDate = item.Task.DueDate
			list.Tasks[item.Index].RecurDays = recurDays
			list.Tasks[item.Index].Estimate = estimate
			list.Tasks[item.Index].StartTime = startTime
			m.storage.SaveList(list)
		}

		// For today reference tasks, propagate edits to the source list
		if listName == "today" && item.Task.IsReference() {
			m.syncMetadataToSource(item.Task)

			// If due date changed to no longer be today, remove from today list
			if !item.Task.IsDueToday() {
				if item.Index < len(list.Tasks) {
					list.Delete(item.Index)
					m.storage.SaveList(list)
				}
			}
		}

		m.inputMode = InputNormal
		return m.loadTasks()()
	}
}

func (m *TaskViewModel) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.inputMode = InputNormal
		m.searchInput.Blur()
		m.searchQuery = ""
		m.items = m.allItems
		m.cursor = 0
		return m, nil

	case "enter":
		m.inputMode = InputNormal
		m.searchInput.Blur()
		return m, nil

	case "up", "down":
		if msg.String() == "up" && m.cursor > 0 {
			m.cursor--
		} else if msg.String() == "down" && m.cursor < len(m.items)-1 {
			m.cursor++
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	m.searchQuery = m.searchInput.Value()
	m.filterTasks()
	return m, cmd
}

// Timeline key handling and helpers

func (m *TaskViewModel) handleTimelineKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.statusMsg = ""

	switch msg.String() {
	case "q", "esc":
		m.quitting = true
		return m, tea.Quit

	case "h", "left":
		m.timelineFocus = false
		return m, nil

	case "up", "k":
		if m.timelineCursor > 0 {
			m.timelineCursor--
			m.syncListCursorFromTimeline()
		}
		return m, nil

	case "down", "j":
		totalItems := len(m.timelineLayout.Slots) + len(m.timelineLayout.Unscheduled)
		if m.timelineCursor < totalItems-1 {
			m.timelineCursor++
			m.syncListCursorFromTimeline()
		}
		return m, nil

	case "K":
		// Move task up: floating tasks reorder, pinned tasks nudge -30m
		return m, m.timelineMoveTask(-1)

	case "J":
		// Move task down: floating tasks reorder, pinned tasks nudge +30m
		return m, m.timelineMoveTask(1)

	case "s":
		// Set start time (pin the task)
		if m.timelineLayout != nil {
			totalItems := len(m.timelineLayout.Slots) + len(m.timelineLayout.Unscheduled)
			if m.timelineCursor < totalItems {
				m.inputMode = InputTimelineSetTime
				m.startInput.SetValue("")
				m.startInput.Focus()
				return m, textinput.Blink
			}
		}

	case "S":
		// Unpin (remove StartTime)
		return m, m.timelineUnpinTask()

	case "e":
		// Set/change estimate
		m.inputMode = InputTimelineSetEstimate
		m.estInput.SetValue("")
		m.estInput.Focus()
		return m, textinput.Blink

	case "tab", " ":
		// Toggle completion
		if len(m.items) > 0 {
			return m, m.toggleTask(m.cursor)
		}

	case "d":
		// Delete task
		if len(m.items) > 0 {
			return m, m.deleteTask(m.cursor)
		}

	case "a":
		// Add task — switch to list panel and open add form
		m.timelineFocus = false
		m.inputMode = InputAddTask
		m.textInput.SetValue("")
		m.descInput.SetValue("")
		m.dueInput.SetValue("")
		m.recurInput.SetValue("")
		m.estInput.SetValue("")
		m.startInput.SetValue("")
		m.focusedField = 0
		m.textInput.Focus()
		return m, textinput.Blink

	case "enter":
		// View task details
		if m.cursor >= 0 && m.cursor < len(m.items) {
			m.timelineFocus = false
			m.inputMode = InputViewTask
			return m, nil
		}
	}

	return m, nil
}

func (m *TaskViewModel) handleTimelineSetTimeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.inputMode = InputNormal
		m.startInput.Blur()
		return m, nil
	case "enter":
		value := strings.TrimSpace(m.startInput.Value())
		m.startInput.Blur()
		m.inputMode = InputNormal
		if value != "" {
			return m, m.timelineSetStartTime(value)
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.startInput, cmd = m.startInput.Update(msg)
	return m, cmd
}

func (m *TaskViewModel) handleTimelineSetEstimateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.inputMode = InputNormal
		m.estInput.Blur()
		return m, nil
	case "enter":
		value := strings.TrimSpace(m.estInput.Value())
		m.estInput.Blur()
		m.inputMode = InputNormal
		if value != "" {
			return m, m.timelineSetEstimate(value)
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.estInput, cmd = m.estInput.Update(msg)
	return m, cmd
}

func (m *TaskViewModel) getTimelineTask() *task.Task {
	if m.timelineLayout == nil {
		return nil
	}
	if m.timelineCursor < len(m.timelineLayout.Slots) {
		return m.timelineLayout.Slots[m.timelineCursor].Item.Task
	}
	unschedIdx := m.timelineCursor - len(m.timelineLayout.Slots)
	if unschedIdx >= 0 && unschedIdx < len(m.timelineLayout.Unscheduled) {
		return m.timelineLayout.Unscheduled[unschedIdx].Task
	}
	return nil
}

func (m *TaskViewModel) timelineSetStartTime(value string) tea.Cmd {
	return func() tea.Msg {
		t := m.getTimelineTask()
		if t == nil {
			return nil
		}
		startTime := parseStartTimeValue(value)
		if startTime == nil {
			return nil
		}
		t.StartTime = startTime

		if m.taskList != nil {
			m.storage.SaveList(m.taskList)
		}
		if m.listName == "today" && t.IsReference() {
			m.syncMetadataToSource(t)
		}
		return timelineRefreshMsg{cursorIdx: m.timelineCursor}
	}
}

func (m *TaskViewModel) timelineSetEstimate(value string) tea.Cmd {
	return func() tea.Msg {
		t := m.getTimelineTask()
		if t == nil {
			return nil
		}
		est := parseEstimateValue(value)
		if est == nil {
			return nil
		}
		t.Estimate = est

		if m.taskList != nil {
			m.storage.SaveList(m.taskList)
		}
		if m.listName == "today" && t.IsReference() {
			m.syncMetadataToSource(t)
		}
		return timelineRefreshMsg{cursorIdx: m.timelineCursor}
	}
}

func (m *TaskViewModel) timelineUnpinTask() tea.Cmd {
	return func() tea.Msg {
		t := m.getTimelineTask()
		if t == nil {
			return nil
		}
		t.StartTime = nil

		if m.taskList != nil {
			m.storage.SaveList(m.taskList)
		}
		if m.listName == "today" && t.IsReference() {
			m.syncMetadataToSource(t)
		}
		return timelineRefreshMsg{cursorIdx: m.timelineCursor}
	}
}

func (m *TaskViewModel) timelineMoveTask(direction int) tea.Cmd {
	return func() tea.Msg {
		if m.timelineLayout == nil || m.timelineCursor >= len(m.timelineLayout.Slots) {
			return nil
		}

		slot := m.timelineLayout.Slots[m.timelineCursor]
		t := slot.Item.Task
		newCursor := m.timelineCursor

		if slot.IsPinned && t.StartTime != nil {
			// Pinned task: nudge by 30 minutes
			newMinutes := t.StartTime.ToMinutes() + direction*30
			if newMinutes < 0 {
				newMinutes = 0
			}
			if newMinutes > 23*60+30 {
				newMinutes = 23*60 + 30
			}
			t.StartTime = &task.TimeOfDay{
				Hour:   newMinutes / 60,
				Minute: newMinutes % 60,
			}
		} else {
			// Floating task: swap with adjacent floating task in the slots
			targetIdx := m.timelineCursor + direction
			if targetIdx < 0 || targetIdx >= len(m.timelineLayout.Slots) {
				return nil
			}
			targetSlot := m.timelineLayout.Slots[targetIdx]
			if targetSlot.IsPinned {
				return nil // Can't swap with pinned
			}
			// Swap the tasks in the actual task list
			if m.taskList != nil {
				idx1 := m.taskList.OriginalIndex(slot.Item.Task)
				idx2 := m.taskList.OriginalIndex(targetSlot.Item.Task)
				if idx1 >= 0 && idx2 >= 0 && idx1 < len(m.taskList.Tasks) && idx2 < len(m.taskList.Tasks) {
					m.taskList.Tasks[idx1], m.taskList.Tasks[idx2] = m.taskList.Tasks[idx2], m.taskList.Tasks[idx1]
				}
			}
			newCursor = targetIdx
		}

		// Save
		if m.taskList != nil {
			m.storage.SaveList(m.taskList)
		}
		if m.listName == "today" && t.IsReference() {
			m.syncMetadataToSource(t)
		}
		return timelineRefreshMsg{cursorIdx: newCursor}
	}
}

func (m *TaskViewModel) syncTimelineCursor() {
	if m.timelineLayout == nil || m.cursor >= len(m.items) {
		return
	}
	currentTask := m.items[m.cursor].Task
	for i, slot := range m.timelineLayout.Slots {
		if slot.Item.Task == currentTask {
			m.timelineCursor = i
			return
		}
	}
	// Check unscheduled
	for i, item := range m.timelineLayout.Unscheduled {
		if item.Task == currentTask {
			m.timelineCursor = len(m.timelineLayout.Slots) + i
			return
		}
	}
}

func (m *TaskViewModel) syncListCursorFromTimeline() {
	if m.timelineLayout == nil {
		return
	}
	var targetTask *task.Task
	if m.timelineCursor < len(m.timelineLayout.Slots) {
		targetTask = m.timelineLayout.Slots[m.timelineCursor].Item.Task
	} else {
		unschedIdx := m.timelineCursor - len(m.timelineLayout.Slots)
		if unschedIdx >= 0 && unschedIdx < len(m.timelineLayout.Unscheduled) {
			targetTask = m.timelineLayout.Unscheduled[unschedIdx].Task
		}
	}
	if targetTask != nil {
		for i, item := range m.items {
			if item.Task == targetTask {
				m.cursor = i
				return
			}
		}
	}
}

// focusFormField focuses the appropriate input based on field index
func (m *TaskViewModel) focusFormField(field int) tea.Cmd {
	m.textInput.Blur()
	m.descInput.Blur()
	m.dueInput.Blur()
	m.recurInput.Blur()
	m.estInput.Blur()
	m.startInput.Blur()

	m.focusedField = field
	switch field {
	case 0:
		m.textInput.Focus()
	case 1:
		m.descInput.Focus()
	case 2:
		m.dueInput.Focus()
	case 3:
		m.recurInput.Focus()
	case 4:
		m.estInput.Focus()
	case 5:
		m.startInput.Focus()
	}
	return textinput.Blink
}

// clearFormFields resets all form inputs
func (m *TaskViewModel) clearFormFields() {
	m.textInput.SetValue("")
	m.descInput.SetValue("")
	m.dueInput.SetValue("")
	m.recurInput.SetValue("")
	m.estInput.SetValue("")
	m.startInput.SetValue("")
	m.focusedField = 0
}

func (m *TaskViewModel) handleAddTaskKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	const numFields = 6

	switch msg.String() {
	case "esc":
		m.inputMode = InputNormal
		m.textInput.Blur()
		m.descInput.Blur()
		m.dueInput.Blur()
		m.recurInput.Blur()
		m.estInput.Blur()
		m.startInput.Blur()
		m.focusedField = 0
		return m, nil

	case "tab":
		// Move to next field (wrap around)
		nextField := (m.focusedField + 1) % numFields
		return m, m.focusFormField(nextField)

	case "shift+tab":
		// Move to previous field (wrap around)
		prevField := (m.focusedField - 1 + numFields) % numFields
		return m, m.focusFormField(prevField)

	case "enter":
		content := strings.TrimSpace(m.textInput.Value())
		if content != "" {
			description := m.descInput.Value()
			dueValue := strings.TrimSpace(m.dueInput.Value())
			recurValue := strings.TrimSpace(m.recurInput.Value())
			estValue := strings.TrimSpace(m.estInput.Value())
			startValue := strings.TrimSpace(m.startInput.Value())

			m.textInput.Blur()
			m.descInput.Blur()
			m.dueInput.Blur()
			m.recurInput.Blur()
			m.estInput.Blur()
			m.startInput.Blur()
			m.focusedField = 0

			if m.viewMode == ViewAllPending {
				// In all-tasks view, show list picker before adding
				m.pendingTaskContent = content
				m.pendingTaskDesc = description
				m.pendingTaskDue = dueValue
				m.pendingTaskRecur = recurValue
				m.pendingTaskEst = estValue
				m.pendingTaskStart = startValue
				m.inputMode = InputSelectListForTask
				// Pre-select the list of currently selected task
				if m.cursor < len(m.items) {
					defaultList := m.items[m.cursor].ListName
					for i, list := range m.lists {
						if list == defaultList {
							m.listCursor = i
							break
						}
					}
				}
				return m, m.loadLists()
			}

			m.inputMode = InputNormal
			return m, m.addTaskWithOptions(content, description, dueValue, recurValue, estValue, startValue)
		}
		return m, nil
	}

	// Update the focused input
	var cmd tea.Cmd
	switch m.focusedField {
	case 0:
		m.textInput, cmd = m.textInput.Update(msg)
	case 1:
		m.descInput, cmd = m.descInput.Update(msg)
	case 2:
		m.dueInput, cmd = m.dueInput.Update(msg)
	case 3:
		m.recurInput, cmd = m.recurInput.Update(msg)
	case 4:
		m.estInput, cmd = m.estInput.Update(msg)
	case 5:
		m.startInput, cmd = m.startInput.Update(msg)
	}
	return m, cmd
}

func (m *TaskViewModel) handleAddSubtaskKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	const numFields = 6

	switch msg.String() {
	case "esc":
		m.inputMode = InputNormal
		m.textInput.Blur()
		m.descInput.Blur()
		m.dueInput.Blur()
		m.recurInput.Blur()
		m.estInput.Blur()
		m.startInput.Blur()
		m.focusedField = 0
		return m, nil

	case "tab":
		nextField := (m.focusedField + 1) % numFields
		return m, m.focusFormField(nextField)

	case "shift+tab":
		prevField := (m.focusedField - 1 + numFields) % numFields
		return m, m.focusFormField(prevField)

	case "enter":
		content := strings.TrimSpace(m.textInput.Value())
		if content != "" {
			m.textInput.Blur()
			m.descInput.Blur()
			m.dueInput.Blur()
			m.recurInput.Blur()
			m.estInput.Blur()
			m.startInput.Blur()
			m.focusedField = 0
			m.inputMode = InputNormal
			return m, m.addSubtaskToParent(content, m.descInput.Value(), m.dueInput.Value(), m.recurInput.Value(), m.estInput.Value(), m.startInput.Value())
		}
		return m, nil
	}

	var cmd tea.Cmd
	switch m.focusedField {
	case 0:
		m.textInput, cmd = m.textInput.Update(msg)
	case 1:
		m.descInput, cmd = m.descInput.Update(msg)
	case 2:
		m.dueInput, cmd = m.dueInput.Update(msg)
	case 3:
		m.recurInput, cmd = m.recurInput.Update(msg)
	case 4:
		m.estInput, cmd = m.estInput.Update(msg)
	case 5:
		m.startInput, cmd = m.startInput.Update(msg)
	}
	return m, cmd
}

func (m *TaskViewModel) addSubtaskToParent(content, description, dueValue, recurValue, estValue, startValue string) tea.Cmd {
	return func() tea.Msg {
		if m.subtaskParentIdx < 0 || m.subtaskParentIdx >= len(m.items) {
			return nil
		}

		parentItem := m.items[m.subtaskParentIdx]
		estimate := parseEstimateValue(estValue)
		startTime := parseStartTimeValue(startValue)
		dueOffset, specificDate := parseDueValue(dueValue)
		recurDays := parseRecurValue(recurValue)

		now := time.Now()
		subtask := &task.Task{
			Content:     content,
			Description: strings.TrimSpace(description),
			Completed:   false,
			RecurDays:   recurDays,
			Estimate:    estimate,
			StartTime:   startTime,
			CreatedAt:   &now,
		}

		if dueOffset > 0 || recurDays > 0 {
			offset := dueOffset
			if offset == 0 && recurDays > 0 {
				offset = recurDays
			}
			due := now.AddDate(0, 0, offset)
			subtask.DueDate = &due
		}
		if specificDate != nil {
			subtask.DueDate = specificDate
		}

		if m.viewMode == ViewSingleList && m.taskList != nil {
			originalIndex := m.taskList.OriginalIndex(parentItem.Task)
			if originalIndex < 0 {
				return nil
			}
			m.taskList.AddSubtask(originalIndex, subtask)
			m.storage.SaveList(m.taskList)
		} else {
			// All-tasks view
			list, err := m.storage.LoadList(parentItem.ListName)
			if err != nil {
				return nil
			}
			list.AddSubtask(parentItem.Index, subtask)
			m.storage.SaveList(list)
		}

		return m.loadTasks()()
	}
}

func (m *TaskViewModel) handleSelectListForTaskKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.inputMode = InputNormal
		m.pendingTaskContent = ""
		m.pendingTaskDesc = ""
		return m, nil

	case "up", "k":
		if m.listCursor > 0 {
			m.listCursor--
		}

	case "down", "j":
		if m.listCursor < len(m.lists)-1 {
			m.listCursor++
		}

	case "enter":
		if len(m.lists) > 0 && m.pendingTaskContent != "" {
			selectedList := m.lists[m.listCursor]
			m.inputMode = InputNormal
			return m, m.addTaskToList(m.pendingTaskContent, m.pendingTaskDesc, selectedList)
		}
	}

	return m, nil
}

func (m *TaskViewModel) filterTasks() {
	if m.searchQuery == "" {
		m.items = m.allItems
		return
	}

	query := strings.ToLower(m.searchQuery)
	var filtered []TaskItem
	for _, item := range m.allItems {
		if strings.Contains(strings.ToLower(item.Task.Content), query) ||
			strings.Contains(strings.ToLower(item.ListName), query) {
			filtered = append(filtered, item)
		}
	}
	m.items = filtered
	if m.cursor >= len(m.items) {
		m.cursor = max(0, len(m.items)-1)
	}
}

func (m *TaskViewModel) toggleTask(idx int) tea.Cmd {
	return func() tea.Msg {
		if idx < 0 || idx >= len(m.items) {
			return nil
		}

		item := m.items[idx]

		if m.viewMode == ViewSingleList && m.taskList != nil {
			if item.IsSubtask {
				// Toggle subtask
				parentIdx := m.findParentItemIdx(idx)
				originalParentIndex := m.taskList.OriginalIndex(m.items[parentIdx].Task)
				if originalParentIndex < 0 {
					return nil
				}
				m.taskList.ToggleSubtask(originalParentIndex, item.SubIndex)

				// Sync parent to source if on today list
				if m.listName == "today" {
					parent := m.taskList.Get(originalParentIndex)
					if parent != nil && parent.Source != "" {
						m.syncToSource(parent)
					}
				}
			} else {
				// Toggle parent task (existing logic)
				originalIndex := m.taskList.OriginalIndex(item.Task)
				if originalIndex < 0 {
					return nil
				}

				toggled := m.taskList.Toggle(originalIndex)
				m.handleRecurrence(toggled, m.taskList)

				if m.listName == "today" && toggled != nil && toggled.Source != "" {
					m.syncToSource(toggled)
				}
			}

			m.storage.SaveList(m.taskList)
		} else {
			// All-tasks view
			list, err := m.storage.LoadList(item.ListName)
			if err != nil {
				return nil
			}

			if item.IsSubtask {
				list.ToggleSubtask(item.Index, item.SubIndex)
			} else {
				toggled := list.Toggle(item.Index)
				m.handleRecurrence(toggled, list)
			}

			m.storage.SaveList(list)
		}

		return m.loadTasks()()
	}
}

// findParentItemIdx walks backwards from a subtask item to find its parent TaskItem index.
func (m *TaskViewModel) findParentItemIdx(subtaskIdx int) int {
	for i := subtaskIdx - 1; i >= 0; i-- {
		if !m.items[i].IsSubtask {
			return i
		}
	}
	return 0
}

func (m *TaskViewModel) syncToSource(todayTask *task.Task) {
	sourceList, err := m.storage.LoadList(todayTask.Source)
	if err != nil {
		return
	}

	todayContent := strings.TrimSpace(todayTask.Content)
	for _, t := range sourceList.Tasks {
		if strings.TrimSpace(t.Content) == todayContent {
			t.Completed = todayTask.Completed
			// Sync subtask completion states
			for _, todaySub := range todayTask.Subtasks {
				subContent := strings.TrimSpace(todaySub.Content)
				for _, srcSub := range t.Subtasks {
					if strings.TrimSpace(srcSub.Content) == subContent {
						srcSub.Completed = todaySub.Completed
						break
					}
				}
			}
			break
		}
	}

	m.storage.SaveList(sourceList)
}

// syncMetadataToSource propagates metadata changes from a today reference
// task back to the source list (the single source of truth).
func (m *TaskViewModel) syncMetadataToSource(todayTask *task.Task) {
	if todayTask.Source == "" {
		return
	}

	sourceList, err := m.storage.LoadList(todayTask.Source)
	if err != nil {
		return
	}

	todayContent := strings.TrimSpace(todayTask.Content)
	for _, t := range sourceList.Tasks {
		if strings.TrimSpace(t.Content) == todayContent {
			t.Estimate = todayTask.Estimate
			t.StartTime = todayTask.StartTime
			t.Description = todayTask.Description
			t.DueDate = todayTask.DueDate
			t.RecurDays = todayTask.RecurDays
			break
		}
	}

	m.storage.SaveList(sourceList)
}

// handleRecurrence creates the next occurrence for a recurring task.
// For tasks from today list (with Source), it checks if source is already completed
// to avoid duplicate recurrences. Returns true if recurrence was added.
func (m *TaskViewModel) handleRecurrence(toggled *task.Task, currentList *task.TaskList) bool {
	if toggled == nil || !toggled.Completed || toggled.RecurDays <= 0 {
		return false
	}

	nextTask := toggled.CreateNextRecurrence()
	if nextTask == nil {
		return false
	}

	if toggled.Source != "" {
		// Task is from today list - add recurrence to source list
		// but only if source task isn't already completed (to avoid duplicates)
		sourceList, err := m.storage.LoadList(toggled.Source)
		if err != nil {
			return false
		}

		for _, t := range sourceList.Tasks {
			if t.Content == toggled.Content && t.Completed {
				// Source already completed, recurrence was already created
				return false
			}
		}

		sourceList.Add(nextTask)
		m.storage.SaveList(sourceList)
		return true
	}

	// Task is in its home list - add recurrence to current list
	currentList.Add(nextTask)
	return true
}

func (m *TaskViewModel) deleteTask(idx int) tea.Cmd {
	return func() tea.Msg {
		if idx < 0 || idx >= len(m.items) {
			return nil
		}

		item := m.items[idx]

		if m.viewMode == ViewSingleList && m.taskList != nil {
			if item.IsSubtask {
				parentIdx := m.findParentItemIdx(idx)
				originalParentIndex := m.taskList.OriginalIndex(m.items[parentIdx].Task)
				if originalParentIndex < 0 {
					return nil
				}
				m.taskList.DeleteSubtask(originalParentIndex, item.SubIndex)
			} else {
				originalIndex := m.taskList.OriginalIndex(item.Task)
				if originalIndex < 0 {
					return nil
				}
				m.taskList.Delete(originalIndex)
			}
			m.storage.SaveList(m.taskList)
		} else {
			list, err := m.storage.LoadList(item.ListName)
			if err != nil {
				return nil
			}

			if item.IsSubtask {
				list.DeleteSubtask(item.Index, item.SubIndex)
			} else {
				list.Delete(item.Index)
			}
			m.storage.SaveList(list)
		}

		return m.loadTasks()()
	}
}

// parseDueValue parses a due date value which can be:
// - empty (no due date)
// - "+N" or "N" (days from now)
// - "YYYY-MM-DD" (specific date)
// Returns the due date offset in days (0 means no due date)
func parseDueValue(value string) (dueOffset int, specificDate *time.Time) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}

	// Try parsing as specific date first
	if t, err := time.Parse("2006-01-02", value); err == nil {
		return 0, &t
	}

	// Parse as days offset (+N or just N)
	value = strings.TrimPrefix(value, "+")
	var days int
	if _, err := fmt.Sscanf(value, "%d", &days); err == nil && days > 0 {
		return days, nil
	}

	return 0, nil
}

// parseRecurValue parses a recurrence value (days between occurrences)
func parseRecurValue(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}

	var days int
	fmt.Sscanf(value, "%d", &days)
	return days
}

func parseEstimateValue(value string) *time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	var totalMinutes int
	// Parse hours
	if idx := strings.Index(value, "h"); idx > 0 {
		var hours int
		fmt.Sscanf(value[:idx], "%d", &hours)
		totalMinutes += hours * 60
		value = value[idx+1:]
	}
	// Parse minutes
	value = strings.TrimSuffix(value, "m")
	if value != "" {
		var mins int
		fmt.Sscanf(value, "%d", &mins)
		totalMinutes += mins
	}
	if totalMinutes > 0 {
		d := time.Duration(totalMinutes) * time.Minute
		return &d
	}
	return nil
}

func parseStartTimeValue(value string) *task.TimeOfDay {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	var hour, minute int
	n, _ := fmt.Sscanf(value, "%d:%d", &hour, &minute)
	if n == 2 && hour >= 0 && hour <= 23 && minute >= 0 && minute <= 59 {
		return &task.TimeOfDay{Hour: hour, Minute: minute}
	}
	return nil
}

func (m *TaskViewModel) addTaskWithOptions(content, description, dueValue, recurValue, estValue, startValue string) tea.Cmd {
	return func() tea.Msg {
		dueOffset, specificDate := parseDueValue(dueValue)
		recurDays := parseRecurValue(recurValue)
		estimate := parseEstimateValue(estValue)
		startTime := parseStartTimeValue(startValue)

		// If recur is set but no due date, use recur as due offset
		if recurDays > 0 && dueOffset == 0 && specificDate == nil {
			dueOffset = recurDays
		}

		if m.viewMode == ViewSingleList && m.taskList != nil {
			t := m.taskList.AddContent(content, strings.TrimSpace(description), dueOffset, recurDays)
			// If specific date was provided, override the calculated due date
			if specificDate != nil {
				t.DueDate = specificDate
			}
			t.Estimate = estimate
			t.StartTime = startTime
			m.storage.SaveList(m.taskList)
		} else if m.viewMode == ViewAllPending && m.cursor < len(m.items) {
			item := m.items[m.cursor]
			list, err := m.storage.LoadList(item.ListName)
			if err != nil {
				return nil
			}
			t := list.AddContent(content, strings.TrimSpace(description), dueOffset, recurDays)
			if specificDate != nil {
				t.DueDate = specificDate
			}
			t.Estimate = estimate
			t.StartTime = startTime
			m.storage.SaveList(list)
		} else {
			return nil
		}

		return m.loadTasks()()
	}
}

// addTask is kept for backward compatibility (CLI usage)
func (m *TaskViewModel) addTask(content string, description string) tea.Cmd {
	return m.addTaskWithOptions(content, description, "", "", "", "")
}

func (m *TaskViewModel) addTaskToList(content string, description string, listName string) tea.Cmd {
	return func() tea.Msg {
		// Use pending values for due, recur, est, start
		dueOffset, specificDate := parseDueValue(m.pendingTaskDue)
		recurDays := parseRecurValue(m.pendingTaskRecur)
		estimate := parseEstimateValue(m.pendingTaskEst)
		startTime := parseStartTimeValue(m.pendingTaskStart)

		// If recur is set but no due date, use recur as due offset
		if recurDays > 0 && dueOffset == 0 && specificDate == nil {
			dueOffset = recurDays
		}

		list, err := m.storage.LoadList(listName)
		if err != nil {
			return nil
		}

		t := list.AddContent(content, strings.TrimSpace(description), dueOffset, recurDays)
		if specificDate != nil {
			t.DueDate = specificDate
		}
		t.Estimate = estimate
		t.StartTime = startTime
		m.storage.SaveList(list)

		// Clear pending task
		m.pendingTaskContent = ""
		m.pendingTaskDesc = ""
		m.pendingTaskDue = ""
		m.pendingTaskRecur = ""
		m.pendingTaskEst = ""
		m.pendingTaskStart = ""

		return m.loadTasks()()
	}
}

// getOrCreateTodayList loads the today list, creating it if needed
func (m *TaskViewModel) getOrCreateTodayList() *task.TaskList {
	todayList, err := m.storage.LoadList("today")
	if err != nil {
		m.storage.CreateList("today")
		todayList, _ = m.storage.LoadList("today")
	}
	return todayList
}

// copyTaskForToday creates a reference stub for adding to today's list.
// The source list remains the single truth for all metadata.
func copyTaskForToday(t *task.Task, source string) *task.Task {
	return &task.Task{
		Content: t.Content,
		Source:  source,
	}
}

func (m *TaskViewModel) addSelectedToToday() tea.Cmd {
	return func() tea.Msg {
		todayList := m.getOrCreateTodayList()

		added := 0
		for idx, sel := range m.selected {
			if !sel || idx >= len(m.items) {
				continue
			}
			item := m.items[idx]
			todayList.Add(copyTaskForToday(item.Task, item.ListName))
			added++
		}

		if added > 0 {
			m.storage.SaveList(todayList)
		}

		m.added = added
		m.quitting = true
		return tea.Quit()
	}
}

func (m *TaskViewModel) addCurrentTaskToToday() tea.Cmd {
	return func() tea.Msg {
		if m.cursor >= len(m.items) {
			return nil
		}

		item := m.items[m.cursor]

		// Determine source list name
		sourceList := m.listName
		if m.viewMode == ViewAllPending {
			sourceList = item.ListName
		}

		// Don't add if already in today list
		if sourceList == "today" {
			return nil
		}

		todayList := m.getOrCreateTodayList()
		todayList.Add(copyTaskForToday(item.Task, sourceList))
		m.storage.SaveList(todayList)

		return AddedToTodayMsg{TaskName: item.Task.Content}
	}
}

func (m *TaskViewModel) View() string {
	if m.quitting {
		if m.viewMode == ViewPicker && m.added > 0 {
			return fmt.Sprintf("📅 Added %d task(s) to today's list\n", m.added)
		}
		return ""
	}

	if m.items == nil {
		return "Loading..."
	}

	// Timeline split view for today's list in normal/search/timeline-input modes
	if m.showTimeline && (m.inputMode == InputNormal || m.inputMode == InputSearch ||
		m.inputMode == InputTimelineSetTime || m.inputMode == InputTimelineSetEstimate) {
		return m.renderTimelineSplitView()
	}

	var sb strings.Builder

	// Title
	sb.WriteString(titleStyle.Render(m.getTitle()))
	sb.WriteString("\n\n")

	// Input modes
	switch m.inputMode {
	case InputAddTask:
		// Split-screen view: tasks on left, add form on right
		termWidth := m.width
		if termWidth < 60 {
			termWidth = 80
		}

		// Calculate panel widths (60% tasks, 40% add form)
		leftWidth := (termWidth * 55) / 100
		rightWidth := termWidth - leftWidth - 3 // account for gap

		if leftWidth < 30 {
			leftWidth = 30
		}
		if rightWidth < 25 {
			rightWidth = 25
		}

		// Render left panel (task list)
		leftContent := m.renderTaskListPanel(leftWidth)
		leftPanel := panelBorderStyle.Width(leftWidth).Render(leftContent)

		// Render right panel (add task form)
		rightContent := m.renderTaskFormPanel("Add New Task", "add task")
		rightPanel := panelBorderStyle.Width(rightWidth).Render(rightContent)

		// Join panels horizontally
		splitView := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, " ", rightPanel)
		sb.WriteString(splitView)

		return sb.String()

	case InputAddSubtask:
		// Same split-screen layout as InputAddTask
		termWidth := m.width
		if termWidth < 60 {
			termWidth = 80
		}
		leftWidth := (termWidth * 55) / 100
		rightWidth := termWidth - leftWidth - 3
		if leftWidth < 30 {
			leftWidth = 30
		}
		if rightWidth < 25 {
			rightWidth = 25
		}
		leftContent := m.renderTaskListPanel(leftWidth)
		leftPanel := panelBorderStyle.Width(leftWidth).Render(leftContent)

		parentName := ""
		if m.subtaskParentIdx < len(m.items) {
			parentName = m.items[m.subtaskParentIdx].Task.DisplayContent()
		}
		rightContent := m.renderTaskFormPanel(fmt.Sprintf("Add Subtask to: %s", parentName), "add subtask")
		rightPanel := panelBorderStyle.Width(rightWidth).Render(rightContent)

		splitView := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, " ", rightPanel)
		sb.WriteString(splitView)

		return sb.String()

	case InputEditTask, InputViewTask:
		// Split-screen view: tasks on left, edit form or details on right
		termWidth := m.width
		if termWidth < 60 {
			termWidth = 80
		}

		// Calculate panel widths (55% tasks, 45% details/edit)
		leftWidth := (termWidth * 55) / 100
		rightWidth := termWidth - leftWidth - 3

		if leftWidth < 30 {
			leftWidth = 30
		}
		if rightWidth < 25 {
			rightWidth = 25
		}

		// Render left panel (task list)
		leftContent := m.renderTaskListPanel(leftWidth)
		leftPanel := panelBorderStyle.Width(leftWidth).Render(leftContent)

		// Render right panel (edit form or task details)
		var rightContent string
		if m.inputMode == InputEditTask {
			rightContent = m.renderTaskFormPanel("Edit Task", "save")
		} else {
			rightContent = m.renderTaskDetailPanel(rightWidth)
		}
		rightPanel := panelBorderStyle.Width(rightWidth).Render(rightContent)

		// Join panels horizontally
		splitView := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, " ", rightPanel)
		sb.WriteString(splitView)

		return sb.String()

	case InputSelectListForTask:
		sb.WriteString(headerStyle.Render("Add task to list:\n"))
		sb.WriteString(normalStyle.Render(fmt.Sprintf("Task: %s\n\n", m.pendingTaskContent)))

		for i, listName := range m.lists {
			info, _ := m.storage.GetListInfo(listName)
			var line string
			if info != nil {
				line = fmt.Sprintf("%s (%d/%d)", listName, info.Completed, info.Total)
			} else {
				line = listName
			}

			if i == m.listCursor {
				sb.WriteString(selectedStyle.Render("> " + line))
			} else {
				sb.WriteString(normalStyle.Render("  " + line))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
		sb.WriteString(helpStyle.Render("↑/↓: navigate • Enter: add to list • Esc: cancel"))
		return sb.String()
	}

	// Search bar
	if m.inputMode == InputSearch {
		sb.WriteString("🔍 Search: ")
		sb.WriteString(m.searchInput.View())
		sb.WriteString(fmt.Sprintf("  (%d matches)", len(m.items)))
		sb.WriteString("\n\n")
	}

	// Calculate column widths based on terminal width
	termWidth := m.width
	if termWidth < 40 {
		termWidth = 80 // default fallback
	}

	// Calculate max list name length if showing lists
	listColWidth := 0
	if m.showLists {
		maxListLen := 4 // minimum "List" header
		for _, item := range m.items {
			if len(item.ListName) > maxListLen {
				maxListLen = len(item.ListName)
			}
		}
		listColWidth = maxListLen + 2 // add padding
	}

	// Fixed widths: cursor(2) + checkbox(2) + due(12) + padding(4)
	fixedWidth := 20
	contentWidth := termWidth - fixedWidth - listColWidth
	if contentWidth < 20 {
		contentWidth = 20
	}

	// Header
	if m.showLists {
		header := fmt.Sprintf("   %-*s  %-*s  %s", listColWidth-2, "List", contentWidth, "Task", "Due")
		sb.WriteString(headerStyle.Render(header))
	} else {
		header := fmt.Sprintf("   %-*s  %s", contentWidth, "Task", "Due")
		sb.WriteString(headerStyle.Render(header))
	}
	sb.WriteString("\n")
	sb.WriteString(headerStyle.Render(strings.Repeat("─", termWidth-2)))
	sb.WriteString("\n")

	// Task list
	if len(m.items) == 0 {
		sb.WriteString(normalStyle.Render("  No tasks."))
		sb.WriteString("\n")
	} else {
		showedCompletedSeparator := false
		completedShown := 0
		maxCompletedToShow := 5 // Only show a few completed tasks

		// Count completed tasks
		totalCompleted := 0
		for _, item := range m.items {
			if item.Task.Completed {
				totalCompleted++
			}
		}

		for i, item := range m.items {
			// Show separator before first completed task
			if item.Task.Completed && !showedCompletedSeparator && m.viewMode != ViewPicker {
				showedCompletedSeparator = true
				sb.WriteString("\n")
				sb.WriteString(separatorStyle.Render("  ── completed ──"))
				sb.WriteString("\n\n")
			}

			// Limit completed tasks shown (unless cursor is on them)
			if item.Task.Completed && m.viewMode != ViewPicker {
				completedShown++
				if completedShown > maxCompletedToShow && i != m.cursor {
					continue
				}
			}

			line := m.renderTaskLine(i, item, contentWidth, listColWidth)

			if i == m.cursor {
				sb.WriteString(selectedStyle.Render(line))
			} else if item.Task.Completed && m.viewMode != ViewPicker {
				sb.WriteString(completedStyle.Render(line))
			} else {
				sb.WriteString(normalStyle.Render(line))
			}
			sb.WriteString("\n")

			// Show description if present
			if item.Task.Description != "" {
				descLine := m.renderDescriptionLine(item, contentWidth, listColWidth)
				if i == m.cursor {
					sb.WriteString(selectedStyle.Render(descLine))
				} else if item.Task.Completed && m.viewMode != ViewPicker {
					sb.WriteString(completedStyle.Render(descLine))
				} else {
					sb.WriteString(helpStyle.Render(descLine))
				}
				sb.WriteString("\n")
			}
		}

		// Show count of hidden completed tasks
		if totalCompleted > maxCompletedToShow && m.viewMode != ViewPicker {
			hidden := totalCompleted - maxCompletedToShow
			sb.WriteString(separatorStyle.Render(fmt.Sprintf("  ... and %d more completed", hidden)))
			sb.WriteString("\n")
		}
	}

	// Status message
	if m.statusMsg != "" {
		sb.WriteString("\n")
		sb.WriteString(selectedStyle.Render(m.statusMsg))
	}

	// Help
	sb.WriteString("\n")
	sb.WriteString(helpStyle.Render(m.getHelp()))

	return sb.String()
}

// renderTimelineSplitView renders the split view with task list and timeline
func (m *TaskViewModel) renderTimelineSplitView() string {
	var sb strings.Builder

	// Title
	sb.WriteString(titleStyle.Render(m.getTitle()))
	sb.WriteString("\n\n")

	// Search bar (if searching)
	if m.inputMode == InputSearch {
		sb.WriteString("🔍 Search: ")
		sb.WriteString(m.searchInput.View())
		sb.WriteString(fmt.Sprintf("  (%d matches)", len(m.items)))
		sb.WriteString("\n\n")
	}

	termWidth := m.width
	if termWidth < 60 {
		termWidth = 80
	}
	termHeight := m.height
	if termHeight < 10 {
		termHeight = 30
	}

	// Full-width panel (minus small margin)
	panelWidth := termWidth - 4
	if panelWidth < 40 {
		panelWidth = 40
	}

	debugLog.Printf("renderTimelineSplitView: termW=%d termH=%d panelW=%d items=%d", termWidth, termHeight, panelWidth, len(m.items))

	// Calculate overhead: title(2) + search(2?) + status(1) + help(1) + gaps(2) + borders(4 for 2 panels)
	overhead := 10
	if m.inputMode == InputSearch {
		overhead += 2
	}
	if m.statusMsg != "" {
		overhead += 1
	}

	// Available height for both panels (subtract overhead for title, help, borders, gaps)
	usableHeight := termHeight - overhead

	// Render task list content and measure its natural height
	taskListContent := m.renderTaskListPanel(panelWidth)
	taskListLines := lipgloss.Height(taskListContent)
	if taskListLines < 1 {
		taskListLines = 1
	}

	// Cap task list to at most 40% of usable space so timeline always gets room
	maxTaskListHeight := usableHeight * 40 / 100
	if maxTaskListHeight < 3 {
		maxTaskListHeight = 3
	}
	taskListHeight := taskListLines
	if taskListHeight > maxTaskListHeight {
		taskListHeight = maxTaskListHeight
	}

	debugLog.Printf("  usableH=%d taskListLines=%d taskListH=%d maxTaskListH=%d", usableHeight, taskListLines, taskListHeight, maxTaskListHeight)

	// Timeline gets the remaining space (accounting for borders: 2 per panel + 1 gap)
	timelineHeight := usableHeight - taskListHeight - 5
	if timelineHeight < 5 {
		timelineHeight = 5
	}

	// Render task list panel
	topStyle := panelBorderStyle
	if !m.timelineFocus {
		topStyle = focusedPanelBorderStyle
	}
	topPanel := topStyle.Width(panelWidth).Height(taskListHeight).Render(taskListContent)

	// Render timeline panel
	var selectedTask *task.Task
	if m.cursor >= 0 && m.cursor < len(m.items) {
		selectedTask = m.items[m.cursor].Task
	}

	if m.timelineLayout == nil {
		m.timelineLayout = computeTimelineLayout(m.items, 8, 18)
	}

	debugLog.Printf("  timelineH=%d contentW=%d", timelineHeight, panelWidth-2)

	timelineContentHeight := timelineHeight - 2
	if timelineContentHeight < 1 {
		timelineContentHeight = 1
	}
	timelineContentWidth := panelWidth - 2
	if timelineContentWidth < 10 {
		timelineContentWidth = 10
	}
	timelineContent := renderTimeline(m.timelineLayout, timelineContentWidth, timelineContentHeight, selectedTask, time.Now())

	bottomStyle := panelBorderStyle
	if m.timelineFocus {
		bottomStyle = focusedPanelBorderStyle
	}
	bottomPanel := bottomStyle.Width(panelWidth).Height(timelineHeight).Render(timelineContent)

	// Stack vertically
	sb.WriteString(topPanel)
	sb.WriteString("\n")
	sb.WriteString(bottomPanel)

	// Status message
	if m.statusMsg != "" {
		sb.WriteString("\n")
		sb.WriteString(selectedStyle.Render(m.statusMsg))
	}

	// Help bar
	sb.WriteString("\n")
	switch m.inputMode {
	case InputTimelineSetTime:
		sb.WriteString(helpStyle.Render("Set start time: "))
		sb.WriteString(m.startInput.View())
		sb.WriteString(helpStyle.Render("  (Enter: confirm • Esc: cancel)"))
	case InputTimelineSetEstimate:
		sb.WriteString(helpStyle.Render("Set estimate: "))
		sb.WriteString(m.estInput.View())
		sb.WriteString(helpStyle.Render("  (Enter: confirm • Esc: cancel)"))
	default:
		if m.timelineFocus {
			sb.WriteString(helpStyle.Render("j/k: navigate • J/K: reorder • s: set time • S: unpin • e: est. • h/←: list"))
		} else {
			sb.WriteString(helpStyle.Render(m.getHelp() + " • l/→: timeline • c: hide timeline"))
		}
	}

	return sb.String()
}

// renderTaskListPanel renders the task list for a given width (used in split view)
func (m *TaskViewModel) renderTaskListPanel(panelWidth int) string {
	var sb strings.Builder

	// Calculate column widths for the panel
	listColWidth := 0
	if m.showLists {
		maxListLen := 4
		for _, item := range m.items {
			if len(item.ListName) > maxListLen {
				maxListLen = len(item.ListName)
			}
		}
		listColWidth = maxListLen + 2
	}

	fixedWidth := 20
	contentWidth := panelWidth - fixedWidth - listColWidth
	if contentWidth < 15 {
		contentWidth = 15
	}

	// Header
	if m.showLists {
		header := fmt.Sprintf("   %-*s  %-*s  %s", listColWidth-2, "List", contentWidth, "Task", "Due")
		sb.WriteString(headerStyle.Render(header))
	} else {
		header := fmt.Sprintf("   %-*s  %s", contentWidth, "Task", "Due")
		sb.WriteString(headerStyle.Render(header))
	}
	sb.WriteString("\n")
	sb.WriteString(headerStyle.Render(strings.Repeat("─", panelWidth-4)))
	sb.WriteString("\n")

	// Task list
	if len(m.items) == 0 {
		sb.WriteString(normalStyle.Render("  No tasks."))
		sb.WriteString("\n")
	} else {
		showedCompletedSeparator := false
		completedShown := 0
		maxCompletedToShow := 3

		totalCompleted := 0
		for _, item := range m.items {
			if item.Task.Completed {
				totalCompleted++
			}
		}

		for i, item := range m.items {
			if item.Task.Completed && !showedCompletedSeparator && m.viewMode != ViewPicker {
				showedCompletedSeparator = true
				sb.WriteString("\n")
				sb.WriteString(separatorStyle.Render("  ── done ──"))
				sb.WriteString("\n\n")
			}

			if item.Task.Completed && m.viewMode != ViewPicker {
				completedShown++
				if completedShown > maxCompletedToShow && i != m.cursor {
					continue
				}
			}

			line := m.renderTaskLine(i, item, contentWidth, listColWidth)

			if i == m.cursor {
				sb.WriteString(selectedStyle.Render(line))
			} else if item.Task.Completed && m.viewMode != ViewPicker {
				sb.WriteString(completedStyle.Render(line))
			} else {
				sb.WriteString(normalStyle.Render(line))
			}
			sb.WriteString("\n")
		}

		if totalCompleted > maxCompletedToShow && m.viewMode != ViewPicker {
			hidden := totalCompleted - maxCompletedToShow
			sb.WriteString(separatorStyle.Render(fmt.Sprintf("  +%d more", hidden)))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// FormField represents a form field configuration
type FormField struct {
	Label string
	Input *textinput.Model
}

// renderFormField renders a single form field with cursor indicator
func renderFormField(label string, input textinput.Model, focused bool) string {
	cursor := "  "
	if focused {
		cursor = "> "
	}
	return fmt.Sprintf("%s%-6s %s", cursor, label+":", input.View())
}

// renderTaskFormPanel renders a task form panel for add/edit modes
func (m *TaskViewModel) renderTaskFormPanel(title, submitText string) string {
	var sb strings.Builder

	sb.WriteString(addPanelTitleStyle.Render(title))
	sb.WriteString("\n\n")

	// Task field
	sb.WriteString(renderFormField("Task", m.textInput, m.focusedField == 0))
	sb.WriteString("\n\n")

	// Description field
	sb.WriteString(renderFormField("Desc", m.descInput, m.focusedField == 1))
	sb.WriteString("\n\n")

	// Due date field
	sb.WriteString(renderFormField("Due", m.dueInput, m.focusedField == 2))
	sb.WriteString("\n\n")

	// Recurrence field
	sb.WriteString(renderFormField("Recur", m.recurInput, m.focusedField == 3))
	sb.WriteString("\n\n")

	// Estimate field
	sb.WriteString(renderFormField("Est", m.estInput, m.focusedField == 4))
	sb.WriteString("\n\n")

	// Start time field
	sb.WriteString(renderFormField("Start", m.startInput, m.focusedField == 5))
	sb.WriteString("\n\n")

	sb.WriteString(helpStyle.Render("Tab: next field • Shift+Tab: prev"))
	sb.WriteString("\n")
	sb.WriteString(helpStyle.Render("Enter: " + submitText))
	sb.WriteString("\n")
	sb.WriteString(helpStyle.Render("Esc: cancel"))

	return sb.String()
}

// renderTaskDetailPanel renders the task detail view panel
func (m *TaskViewModel) renderTaskDetailPanel(panelWidth int) string {
	var sb strings.Builder

	if m.cursor >= len(m.items) {
		return "No task selected"
	}

	item := m.items[m.cursor]
	t := item.Task

	// Title
	sb.WriteString(addPanelTitleStyle.Render("Task Details"))
	sb.WriteString("\n\n")

	// Status
	status := "Pending"
	if t.Completed {
		status = "Completed"
	}
	sb.WriteString(headerStyle.Render("Status: "))
	sb.WriteString(normalStyle.Render(status))
	sb.WriteString("\n\n")

	// Content
	sb.WriteString(headerStyle.Render("Task:"))
	sb.WriteString("\n")
	// Word wrap content to fit panel
	content := t.DisplayContent()
	maxWidth := panelWidth - 4
	if maxWidth < 20 {
		maxWidth = 20
	}
	wrapped := wrapText(content, maxWidth)
	sb.WriteString(selectedStyle.Render(wrapped))
	sb.WriteString("\n\n")

	// Description
	if t.Description != "" {
		sb.WriteString(headerStyle.Render("Description:"))
		sb.WriteString("\n")
		wrapped := wrapText(t.Description, maxWidth)
		sb.WriteString(normalStyle.Render(wrapped))
		sb.WriteString("\n\n")
	}

	// Due date
	if t.DueDate != nil {
		sb.WriteString(headerStyle.Render("Due: "))
		sb.WriteString(normalStyle.Render(t.DueDate.Format("2006-01-02")))
		sb.WriteString("\n\n")
	}

	// Source (if from another list)
	if t.Source != "" {
		sb.WriteString(headerStyle.Render("Source: "))
		sb.WriteString(normalStyle.Render(t.Source))
		sb.WriteString("\n\n")
	}

	// Recurrence
	if t.RecurDays > 0 {
		sb.WriteString(headerStyle.Render("Recurs: "))
		sb.WriteString(normalStyle.Render(fmt.Sprintf("every %d days", t.RecurDays)))
		sb.WriteString("\n\n")
	}

	// Estimate
	if t.Estimate != nil {
		sb.WriteString(headerStyle.Render("Estimate: "))
		totalMinutes := int(t.Estimate.Minutes())
		hours := totalMinutes / 60
		mins := totalMinutes % 60
		if hours > 0 && mins > 0 {
			sb.WriteString(normalStyle.Render(fmt.Sprintf("%dh%dm", hours, mins)))
		} else if hours > 0 {
			sb.WriteString(normalStyle.Render(fmt.Sprintf("%dh", hours)))
		} else {
			sb.WriteString(normalStyle.Render(fmt.Sprintf("%dm", mins)))
		}
		sb.WriteString("\n\n")
	}

	// Start time
	if t.StartTime != nil {
		sb.WriteString(headerStyle.Render("Starts at: "))
		sb.WriteString(normalStyle.Render(t.StartTime.String()))
		sb.WriteString("\n\n")
	}

	// Help
	sb.WriteString("\n")
	sb.WriteString(helpStyle.Render("Tab/Space: toggle"))
	sb.WriteString("\n")
	sb.WriteString(helpStyle.Render("e: edit"))
	sb.WriteString("\n")
	sb.WriteString(helpStyle.Render("d: delete"))
	sb.WriteString("\n")
	sb.WriteString(helpStyle.Render("Esc: close"))

	return sb.String()
}

// wrapText wraps text to fit within maxWidth
func wrapText(text string, maxWidth int) string {
	if len(text) <= maxWidth {
		return text
	}

	var result strings.Builder
	words := strings.Fields(text)
	lineLen := 0

	for i, word := range words {
		if i > 0 {
			if lineLen+1+len(word) > maxWidth {
				result.WriteString("\n")
				lineLen = 0
			} else {
				result.WriteString(" ")
				lineLen++
			}
		}
		result.WriteString(word)
		lineLen += len(word)
	}

	return result.String()
}

func (m *TaskViewModel) getTitle() string {
	pending := 0
	for _, item := range m.items {
		if !item.Task.Completed {
			pending++
		}
	}

	switch m.viewMode {
	case ViewSingleList:
		return fmt.Sprintf("📝 %s (%d pending)", m.listName, pending)
	case ViewAllPending:
		return fmt.Sprintf("📋 All Tasks (%d pending)", pending)
	case ViewPicker:
		selectedCount := 0
		for _, sel := range m.selected {
			if sel {
				selectedCount++
			}
		}
		return fmt.Sprintf("📅 Add to Today (%d selected)", selectedCount)
	}
	return "Tasks"
}

func (m *TaskViewModel) renderTaskLine(idx int, item TaskItem, contentWidth int, listColWidth int) string {
	cursor := "  "
	if idx == m.cursor {
		cursor = "> "
	}

	indent := ""
	if item.IsSubtask {
		indent = "    "
	}

	// Checkbox
	check := "☐"
	if m.viewMode == ViewPicker {
		if m.selected[idx] {
			check = "☑"
		}
	} else if item.Task.Completed {
		check = "✓"
	}

	due := ""
	if item.Task.DueDate != nil {
		due = item.Task.DueDate.Format("2006-01-02")
	}

	content := item.Task.DisplayContent()

	// Add subtask progress indicator for parent tasks
	if !item.IsSubtask && len(item.Task.Subtasks) > 0 {
		done := 0
		for _, sub := range item.Task.Subtasks {
			if sub.Completed {
				done++
			}
		}
		content = fmt.Sprintf("%s [%d/%d]", content, done, len(item.Task.Subtasks))
	}

	if m.showLists {
		// Account for checkbox space in content width
		maxContent := contentWidth - 2
		if item.IsSubtask {
			maxContent -= 4
		}
		if len(content) > maxContent && maxContent > 1 {
			content = content[:maxContent-1] + "…"
		}
		listNameWidth := listColWidth - 2
		return fmt.Sprintf("%s%s%-*s %s %-*s  %s", indent, cursor, listNameWidth, item.ListName, check, maxContent, content, due)
	}

	// Single list view
	maxContent := contentWidth - 2
	if item.IsSubtask {
		maxContent -= 4
	}
	if len(content) > maxContent && maxContent > 1 {
		content = content[:maxContent-1] + "…"
	}
	return fmt.Sprintf("%s%s%s %-*s  %s", indent, cursor, check, maxContent, content, due)
}

func (m *TaskViewModel) renderDescriptionLine(item TaskItem, contentWidth int, listColWidth int) string {
	desc := item.Task.Description

	// Calculate indent: cursor(2) + checkbox(2) = 4, or with list column
	indent := "     └ "
	if m.showLists {
		indent = strings.Repeat(" ", listColWidth) + "  └ "
	}

	// Truncate description if too long
	maxDesc := contentWidth - len(indent) - 4
	if maxDesc < 10 {
		maxDesc = 10
	}
	if len(desc) > maxDesc {
		desc = desc[:maxDesc-1] + "…"
	}

	return indent + desc
}

func (m *TaskViewModel) getHelp() string {
	switch m.viewMode {
	case ViewSingleList:
		if m.inputMode == InputSearch {
			return "Type to search • ↑/↓: navigate • Enter: keep filter • Esc: clear"
		}
		return "↑/↓: navigate • Enter: view • Tab: toggle • a: add • A: +today • e: edit • d: delete • L: lists • q: quit"
	case ViewAllPending:
		if m.inputMode == InputSearch {
			return "Type to search • ↑/↓: navigate • Enter: keep filter • Esc: clear"
		}
		return "↑/↓: navigate • Enter: view • Tab: toggle • a: add • A: +today • e: edit • d: delete • q: quit"
	case ViewPicker:
		if m.inputMode == InputSearch {
			return "Type to search • ↑/↓: navigate • Enter: keep filter • Esc: clear"
		}
		return "↑/↓: navigate • Space/Tab: select • a: all • n: none • Enter: add • /: search • q: quit"
	}
	return ""
}

func (m *TaskViewModel) Added() int {
	return m.added
}

// Public functions to run different views

// RunTaskListResult indicates the result of running the task list
type RunTaskListResult int

const (
	ResultQuit RunTaskListResult = iota
	ResultSwitchList
)

func RunTaskList(store *storage.Storage) (RunTaskListResult, error) {
	model := NewTaskViewModel(store, ViewSingleList)
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return ResultQuit, err
	}
	if m, ok := finalModel.(*TaskViewModel); ok && m.wantsSwitchList {
		return ResultSwitchList, nil
	}
	return ResultQuit, nil
}

func RunAllTasks(store *storage.Storage) error {
	model := NewTaskViewModel(store, ViewAllPending)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func RunAddToday(store *storage.Storage) (int, error) {
	model := NewTaskViewModel(store, ViewPicker)
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return 0, err
	}
	if m, ok := finalModel.(*TaskViewModel); ok {
		return m.Added(), nil
	}
	return 0, nil
}

// ListSwitcherModel - Interactive list selector
type ListSwitcherModel struct {
	storage     *storage.Storage
	lists       []string
	allLists    []string
	cursor      int
	quitting    bool
	selected    string
	inputMode   InputMode
	searchInput textinput.Model
	addInput    textinput.Model
	searchQuery string
	message     string // feedback message
}

func NewListSwitcherModel(store *storage.Storage) *ListSwitcherModel {
	si := textinput.New()
	si.Placeholder = "Search..."
	si.CharLimit = 50
	si.Width = 30

	ai := textinput.New()
	ai.Placeholder = "List name..."
	ai.CharLimit = 30
	ai.Width = 30

	return &ListSwitcherModel{
		storage:     store,
		searchInput: si,
		addInput:    ai,
	}
}

func (m *ListSwitcherModel) Init() tea.Cmd {
	return m.loadLists()
}

type listSwitcherLoadedMsg struct {
	lists []string
}

func (m *ListSwitcherModel) loadLists() tea.Cmd {
	return func() tea.Msg {
		lists, err := m.storage.GetAllLists()
		if err != nil {
			return nil
		}
		return listSwitcherLoadedMsg{lists: lists}
	}
}

func (m *ListSwitcherModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.inputMode {
		case InputSearch:
			return m.handleSearchKey(msg)
		case InputAddTask: // reuse for add list
			return m.handleAddKey(msg)
		default:
			return m.handleNormalKey(msg)
		}

	case listSwitcherLoadedMsg:
		m.lists = msg.lists
		m.allLists = msg.lists
		m.cursor = 0
		// Find current list
		currentList := m.storage.GetCurrentList()
		for i, name := range m.lists {
			if name == currentList {
				m.cursor = i
				break
			}
		}
		return m, nil
	}

	// Handle text input updates
	if m.inputMode == InputSearch {
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		m.searchQuery = m.searchInput.Value()
		m.filterLists()
		return m, cmd
	}

	if m.inputMode == InputAddTask {
		var cmd tea.Cmd
		m.addInput, cmd = m.addInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *ListSwitcherModel) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.quitting = true
		return m, tea.Quit

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}

	case "down", "j":
		if m.cursor < len(m.lists)-1 {
			m.cursor++
		}

	case "enter":
		if len(m.lists) > 0 {
			m.selected = m.lists[m.cursor]
			m.storage.SetCurrentList(m.selected)
			m.quitting = true
			return m, tea.Quit
		}

	case "/":
		m.inputMode = InputSearch
		m.searchInput.SetValue("")
		m.searchInput.Focus()
		return m, textinput.Blink

	case "a":
		m.inputMode = InputAddTask
		m.addInput.SetValue("")
		m.addInput.Focus()
		return m, textinput.Blink

	case "d", "x":
		if len(m.lists) > 0 {
			listName := m.lists[m.cursor]
			currentList := m.storage.GetCurrentList()
			if listName == currentList {
				m.message = "Cannot delete current list"
			} else {
				if err := m.storage.DeleteList(listName); err != nil {
					m.message = fmt.Sprintf("Error: %v", err)
				} else {
					m.message = fmt.Sprintf("Deleted '%s'", listName)
					return m, m.loadLists()
				}
			}
		}
	}

	return m, nil
}

func (m *ListSwitcherModel) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.inputMode = InputNormal
		m.searchInput.Blur()
		m.searchQuery = ""
		m.lists = m.allLists
		m.cursor = 0
		return m, nil

	case "enter":
		m.inputMode = InputNormal
		m.searchInput.Blur()
		// Select if only one match
		if len(m.lists) == 1 {
			m.selected = m.lists[0]
			m.storage.SetCurrentList(m.selected)
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil

	case "up", "down":
		if msg.String() == "up" && m.cursor > 0 {
			m.cursor--
		} else if msg.String() == "down" && m.cursor < len(m.lists)-1 {
			m.cursor++
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	m.searchQuery = m.searchInput.Value()
	m.filterLists()
	return m, cmd
}

func (m *ListSwitcherModel) handleAddKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.inputMode = InputNormal
		m.addInput.Blur()
		return m, nil

	case "enter":
		name := strings.TrimSpace(m.addInput.Value())
		if name != "" {
			if err := m.storage.CreateList(name); err != nil {
				m.message = fmt.Sprintf("Error: %v", err)
			} else {
				m.message = fmt.Sprintf("Created '%s'", name)
				m.inputMode = InputNormal
				m.addInput.Blur()
				return m, m.loadLists()
			}
		}
		m.inputMode = InputNormal
		m.addInput.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.addInput, cmd = m.addInput.Update(msg)
	return m, cmd
}

func (m *ListSwitcherModel) filterLists() {
	if m.searchQuery == "" {
		m.lists = m.allLists
		return
	}

	query := strings.ToLower(m.searchQuery)
	var filtered []string
	for _, name := range m.allLists {
		if strings.Contains(strings.ToLower(name), query) {
			filtered = append(filtered, name)
		}
	}
	m.lists = filtered
	if m.cursor >= len(m.lists) {
		m.cursor = max(0, len(m.lists)-1)
	}
}

func (m *ListSwitcherModel) View() string {
	if m.quitting {
		if m.selected != "" {
			return fmt.Sprintf("✅ Switched to '%s'\n", m.selected)
		}
		return ""
	}

	if m.lists == nil && m.allLists == nil {
		return "Loading..."
	}

	var sb strings.Builder

	sb.WriteString(titleStyle.Render("📋 Select List"))
	sb.WriteString("\n\n")

	// Show input modes
	switch m.inputMode {
	case InputSearch:
		sb.WriteString("🔍 Search: ")
		sb.WriteString(m.searchInput.View())
		sb.WriteString(fmt.Sprintf("  (%d matches)", len(m.lists)))
		sb.WriteString("\n\n")

	case InputAddTask:
		sb.WriteString("New list: ")
		sb.WriteString(m.addInput.View())
		sb.WriteString("\n\n")
		sb.WriteString(helpStyle.Render("Enter: create • Esc: cancel"))
		return sb.String()
	}

	// Show message if any
	if m.message != "" {
		sb.WriteString(headerStyle.Render(m.message))
		sb.WriteString("\n\n")
	}

	currentList := m.storage.GetCurrentList()

	if len(m.lists) == 0 {
		sb.WriteString(normalStyle.Render("  No lists found."))
		sb.WriteString("\n")
	} else {
		for i, listName := range m.lists {
			info, _ := m.storage.GetListInfo(listName)
			var line string
			if info != nil {
				pending := info.Total - info.Completed
				line = fmt.Sprintf("%-15s  %d pending", listName, pending)
			} else {
				line = fmt.Sprintf("%-15s", listName)
			}

			if listName == currentList {
				line += "  ←"
			}

			if i == m.cursor {
				sb.WriteString(selectedStyle.Render("> " + line))
			} else {
				sb.WriteString(normalStyle.Render("  " + line))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
	if m.inputMode == InputSearch {
		sb.WriteString(helpStyle.Render("Type to search • ↑/↓: navigate • Enter: select • Esc: clear"))
	} else {
		sb.WriteString(helpStyle.Render("↑/↓: navigate • Enter: select • /: search • a: add • d: delete • q: quit"))
	}

	return sb.String()
}

func RunListSwitcher(store *storage.Storage) (string, error) {
	model := NewListSwitcherModel(store)
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return "", err
	}
	if m, ok := finalModel.(*ListSwitcherModel); ok {
		return m.selected, nil
	}
	return "", nil
}
