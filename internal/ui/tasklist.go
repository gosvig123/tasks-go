package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/krisitan/tasks-go/internal/storage"
	"github.com/krisitan/tasks-go/internal/task"
)

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
	InputSwitchList
	InputSearch
)

// TaskItem wraps a task with its source list info
type TaskItem struct {
	Task     *task.Task
	ListName string
	Index    int // original index in list
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
	cursor      int
	selected    map[int]bool // for picker mode
	inputMode   InputMode
	textInput   textinput.Model
	searchInput textinput.Model
	searchQuery string

	// List switcher
	lists      []string
	listCursor int

	// State
	quitting bool
	added    int // for picker mode: count of added items
}

// NewTaskViewModel creates a new task view model
func NewTaskViewModel(store *storage.Storage, mode ViewMode) *TaskViewModel {
	ti := textinput.New()
	ti.Placeholder = "Enter task..."
	ti.CharLimit = 200
	ti.Width = 60

	si := textinput.New()
	si.Placeholder = "Search..."
	si.CharLimit = 100
	si.Width = 40

	return &TaskViewModel{
		storage:     store,
		viewMode:    mode,
		selected:    make(map[int]bool),
		textInput:   ti,
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
	items    []TaskItem
	listName string
	taskList *task.TaskList
}

type listsLoadedMsg struct {
	lists []string
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

func (m *TaskViewModel) loadSingleList() tea.Msg {
	listName := m.storage.GetCurrentList()

	if listName == "today" {
		m.storage.ResetTodayList()
	}

	taskList, err := m.storage.LoadList(listName)
	if err != nil {
		return nil
	}

	// Separate uncompleted and completed
	var uncompleted, completed []TaskItem
	for i, t := range taskList.Tasks {
		item := TaskItem{
			Task:     t,
			ListName: listName,
			Index:    i,
		}
		if t.Completed {
			completed = append(completed, item)
		} else {
			uncompleted = append(uncompleted, item)
		}
	}

	// Uncompleted first, then completed
	items := append(uncompleted, completed...)

	return tasksLoadedMsg{
		items:    items,
		listName: listName,
		taskList: taskList,
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
			item := TaskItem{
				Task:     t,
				ListName: listName,
				Index:    i,
			}
			if t.Completed {
				completed = append(completed, item)
			} else {
				uncompleted = append(uncompleted, item)
			}
		}
	}

	// Sort each group by list name
	sort.Slice(uncompleted, func(i, j int) bool {
		return uncompleted[i].ListName < uncompleted[j].ListName
	})
	sort.Slice(completed, func(i, j int) bool {
		return completed[i].ListName < completed[j].ListName
	})

	// Uncompleted first, then completed
	items := append(uncompleted, completed...)

	return tasksLoadedMsg{items: items}
}

func (m *TaskViewModel) loadPicker() tea.Msg {
	lists, err := m.storage.GetAllLists()
	if err != nil {
		return nil
	}

	var items []TaskItem
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
				})
			}
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].ListName < items[j].ListName
	})

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
		return m, nil

	case tasksLoadedMsg:
		m.items = msg.items
		m.allItems = msg.items
		m.listName = msg.listName
		m.taskList = msg.taskList
		m.cursor = 0
		m.searchQuery = ""
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
	}

	// Handle text input updates
	if m.inputMode == InputAddTask {
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
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
	case InputSwitchList:
		return m.handleSwitchListKey(msg)
	case InputSearch:
		return m.handleSearchKey(msg)
	default:
		return m.handleNormalKey(msg)
	}
}

func (m *TaskViewModel) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
			return m, m.toggleTask(m.cursor)
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
		} else if m.viewMode == ViewSingleList {
			m.inputMode = InputAddTask
			m.textInput.SetValue("")
			m.textInput.Focus()
			return m, textinput.Blink
		}

	case "n":
		if m.viewMode == ViewPicker {
			// Deselect all
			m.selected = make(map[int]bool)
		}

	case "A":
		if m.viewMode == ViewSingleList {
			m.inputMode = InputAddTask
			m.textInput.SetValue("")
			m.textInput.Focus()
			return m, textinput.Blink
		}

	case "L":
		if m.viewMode == ViewSingleList {
			m.inputMode = InputSwitchList
			return m, m.loadLists()
		}

	case "d":
		if m.viewMode == ViewSingleList && len(m.items) > 0 {
			return m, m.deleteTask(m.cursor)
		}

	case "/":
		m.inputMode = InputSearch
		m.searchInput.SetValue("")
		m.searchInput.Focus()
		return m, textinput.Blink
	}

	return m, nil
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

func (m *TaskViewModel) handleAddTaskKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.inputMode = InputNormal
		m.textInput.Blur()
		return m, nil

	case "enter":
		content := strings.TrimSpace(m.textInput.Value())
		if content != "" {
			m.inputMode = InputNormal
			m.textInput.Blur()
			return m, m.addTask(content)
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m *TaskViewModel) handleSwitchListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.inputMode = InputNormal
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
		if len(m.lists) > 0 {
			selectedList := m.lists[m.listCursor]
			m.storage.SetCurrentList(selectedList)
			m.inputMode = InputNormal
			return m, m.loadTasks()
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
			// Toggle in current list
			originalIndex := m.taskList.OriginalIndex(item.Task)
			if originalIndex < 0 {
				return nil
			}

			toggled := m.taskList.Toggle(originalIndex)
			if toggled != nil && toggled.Completed && toggled.RecurDays > 0 {
				nextTask := toggled.CreateNextRecurrence()
				if nextTask != nil {
					if toggled.Source != "" {
						sourceList, err := m.storage.LoadList(toggled.Source)
						if err == nil {
							sourceList.Add(nextTask)
							m.storage.SaveList(sourceList)
						}
					} else {
						m.taskList.Add(nextTask)
					}
				}
			}

			if m.listName == "today" && toggled != nil && toggled.Source != "" {
				m.syncToSource(toggled)
			}

			m.storage.SaveList(m.taskList)
		} else {
			// Toggle in source list
			list, err := m.storage.LoadList(item.ListName)
			if err != nil {
				return nil
			}

			toggled := list.Toggle(item.Index)
			if toggled != nil && toggled.Completed && toggled.RecurDays > 0 {
				nextTask := toggled.CreateNextRecurrence()
				if nextTask != nil {
					list.Add(nextTask)
				}
			}

			m.storage.SaveList(list)
		}

		return m.loadTasks()()
	}
}

func (m *TaskViewModel) syncToSource(todayTask *task.Task) {
	sourceList, err := m.storage.LoadList(todayTask.Source)
	if err != nil {
		return
	}

	for _, t := range sourceList.Tasks {
		if t.Content == todayTask.Content {
			t.Completed = todayTask.Completed
			break
		}
	}

	m.storage.SaveList(sourceList)
}

func (m *TaskViewModel) deleteTask(idx int) tea.Cmd {
	return func() tea.Msg {
		if idx < 0 || idx >= len(m.items) || m.taskList == nil {
			return nil
		}

		item := m.items[idx]
		originalIndex := m.taskList.OriginalIndex(item.Task)
		if originalIndex < 0 {
			return nil
		}

		m.taskList.Delete(originalIndex)
		m.storage.SaveList(m.taskList)

		return m.loadTasks()()
	}
}

func (m *TaskViewModel) addTask(content string) tea.Cmd {
	return func() tea.Msg {
		if m.taskList == nil {
			return nil
		}

		dueOffset := 0
		recurDays := 0

		if idx := strings.LastIndex(content, " +"); idx > 0 {
			suffix := content[idx+2:]
			if strings.HasSuffix(suffix, "r") {
				fmt.Sscanf(suffix[:len(suffix)-1], "%d", &recurDays)
				dueOffset = recurDays
				content = strings.TrimSpace(content[:idx])
			} else {
				fmt.Sscanf(suffix, "%d", &dueOffset)
				if dueOffset > 0 {
					content = strings.TrimSpace(content[:idx])
				}
			}
		}

		m.taskList.AddContent(content, dueOffset, recurDays)
		m.storage.SaveList(m.taskList)

		return m.loadTasks()()
	}
}

func (m *TaskViewModel) addSelectedToToday() tea.Cmd {
	return func() tea.Msg {
		todayList, err := m.storage.LoadList("today")
		if err != nil {
			m.storage.CreateList("today")
			todayList, _ = m.storage.LoadList("today")
		}

		added := 0
		for idx, sel := range m.selected {
			if !sel || idx >= len(m.items) {
				continue
			}
			item := m.items[idx]

			todayTask := &task.Task{
				Content:   item.Task.Content,
				Completed: false,
				DueDate:   item.Task.DueDate,
				RecurDays: item.Task.RecurDays,
				Source:    item.ListName,
			}
			todayList.Add(todayTask)
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

	var sb strings.Builder

	// Title
	sb.WriteString(titleStyle.Render(m.getTitle()))
	sb.WriteString("\n\n")

	// Input modes
	switch m.inputMode {
	case InputAddTask:
		sb.WriteString("Add task: ")
		sb.WriteString(m.textInput.View())
		sb.WriteString("\n\n")
		sb.WriteString(helpStyle.Render("Enter: add • Esc: cancel"))
		return sb.String()

	case InputSwitchList:
		sb.WriteString(headerStyle.Render("Switch to list:\n\n"))
		currentList := m.storage.GetCurrentList()

		for i, listName := range m.lists {
			cursor := "  "
			if i == m.listCursor {
				cursor = "> "
			}

			info, _ := m.storage.GetListInfo(listName)
			var line string
			if info != nil {
				line = fmt.Sprintf("%s%s (%d/%d)", cursor, listName, info.Completed, info.Total)
			} else {
				line = fmt.Sprintf("%s%s", cursor, listName)
			}

			if listName == currentList {
				line += " *"
			}

			if i == m.listCursor {
				sb.WriteString(selectedStyle.Render(line))
			} else {
				sb.WriteString(normalStyle.Render(line))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
		sb.WriteString(helpStyle.Render("↑/↓: navigate • Enter: select • Esc: cancel"))
		return sb.String()
	}

	// Search bar
	if m.inputMode == InputSearch {
		sb.WriteString("🔍 Search: ")
		sb.WriteString(m.searchInput.View())
		sb.WriteString(fmt.Sprintf("  (%d matches)", len(m.items)))
		sb.WriteString("\n\n")
	}

	// Header
	if m.showLists {
		header := fmt.Sprintf("   %-12s  %-50s  %s", "List", "Task", "Due")
		sb.WriteString(headerStyle.Render(header))
	} else {
		header := fmt.Sprintf("   %-60s  %s", "Task", "Due")
		sb.WriteString(headerStyle.Render(header))
	}
	sb.WriteString("\n")
	sb.WriteString(headerStyle.Render(strings.Repeat("─", 75)))
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

			line := m.renderTaskLine(i, item)

			if i == m.cursor {
				sb.WriteString(selectedStyle.Render(line))
			} else if item.Task.Completed && m.viewMode != ViewPicker {
				sb.WriteString(completedStyle.Render(line))
			} else {
				sb.WriteString(normalStyle.Render(line))
			}
			sb.WriteString("\n")
		}

		// Show count of hidden completed tasks
		if totalCompleted > maxCompletedToShow && m.viewMode != ViewPicker {
			hidden := totalCompleted - maxCompletedToShow
			sb.WriteString(separatorStyle.Render(fmt.Sprintf("  ... and %d more completed", hidden)))
			sb.WriteString("\n")
		}
	}

	// Help
	sb.WriteString("\n")
	sb.WriteString(helpStyle.Render(m.getHelp()))

	return sb.String()
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

func (m *TaskViewModel) renderTaskLine(idx int, item TaskItem) string {
	cursor := "  "
	if idx == m.cursor {
		cursor = "> "
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
	if m.showLists {
		if len(content) > 46 {
			content = content[:46] + "…"
		}
		listName := item.ListName
		if len(listName) > 12 {
			listName = listName[:11] + "…"
		}
		return fmt.Sprintf("%s%-12s %s %-47s  %s", cursor, listName, check, content, due)
	}

	// Single list view
	if len(content) > 58 {
		content = content[:58] + "…"
	}
	return fmt.Sprintf("%s%s %-58s  %s", cursor, check, content, due)
}

func (m *TaskViewModel) getHelp() string {
	switch m.viewMode {
	case ViewSingleList:
		if m.inputMode == InputSearch {
			return "Type to search • ↑/↓: navigate • Enter: keep filter • Esc: clear"
		}
		return "↑/↓: navigate • Tab/Enter: toggle • a: add • d: delete • /: search • L: switch list • q: quit"
	case ViewAllPending:
		if m.inputMode == InputSearch {
			return "Type to search • ↑/↓: navigate • Enter: keep filter • Esc: clear"
		}
		return "↑/↓: navigate • Tab/Enter: toggle • /: search • q: quit"
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

func RunTaskList(store *storage.Storage) error {
	model := NewTaskViewModel(store, ViewSingleList)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
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
			cursor := "  "
			if i == m.cursor {
				cursor = "> "
			}

			info, _ := m.storage.GetListInfo(listName)
			var line string
			if info != nil {
				pending := info.Total - info.Completed
				line = fmt.Sprintf("%s%-15s  %d pending", cursor, listName, pending)
			} else {
				line = fmt.Sprintf("%s%-15s", cursor, listName)
			}

			if listName == currentList {
				line += "  ←"
			}

			if i == m.cursor {
				sb.WriteString(selectedStyle.Render(line))
			} else {
				sb.WriteString(normalStyle.Render(line))
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
