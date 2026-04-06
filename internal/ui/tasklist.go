package ui

import (
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
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

	// Selected task (persisted locally)
	selectedTaskList    string
	selectedTaskContent string
	timerRunning        bool
	selectedTaskState   *storage.SelectedTask // cached for render (avoids disk I/O in View)
	tickGen             int                   // generation counter for timer tick chains

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
	showUpcoming   bool            // show upcoming tasks section in today view
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

	// Load upcoming tasks if toggle is on and we're on today list
	debugLog.Printf("loadSingleList: showUpcoming=%v isToday=%v", m.showUpcoming, isToday)
	if m.showUpcoming && isToday {
		upcomingTasks, err := m.storage.LoadUpcomingTasks(14)
		if err == nil && len(upcomingTasks) > 0 {
			for _, ut := range upcomingTasks {
				items = append(items, TaskItem{
					Task:       ut.Task,
					ListName:   ut.ListName,
					Index:      -1,
					SubIndex:   -1,
					IsUpcoming: true,
				})
			}
		}
	}

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
		sel, _ := m.storage.GetSelectedTask()
		if sel != nil {
			m.selectedTaskList = sel.List
			m.selectedTaskContent = sel.Content
			m.timerRunning = sel.IsRunning()
			m.selectedTaskState = sel
		} else {
			m.selectedTaskList = ""
			m.selectedTaskContent = ""
			m.timerRunning = false
			m.selectedTaskState = nil
		}
		if m.showTimeline {
			m.timelineLayout = computeTimelineLayout(m.items, 8, 18)
		}
		if m.timerRunning {
			m.tickGen++
			gen := m.tickGen
			return m, tea.Tick(time.Second, func(t time.Time) tea.Msg { return timerTickMsg{gen: gen} })
		}
		return m, nil

	case timerTickMsg:
		if m.timerRunning && msg.gen == m.tickGen {
			gen := m.tickGen
			return m, tea.Tick(time.Second, func(t time.Time) tea.Msg { return timerTickMsg{gen: gen} })
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

	case refreshDisplayMsg:
		if m.showTimeline {
			m.timelineLayout = computeTimelineLayout(m.items, 8, 18)
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
		return m, m.updateFocusedField(msg)
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

func (m *TaskViewModel) Added() int {
	return m.added
}
