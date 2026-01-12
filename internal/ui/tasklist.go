package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/krisitan/tasks-go/internal/storage"
	"github.com/krisitan/tasks-go/internal/task"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#e5c07b"))

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#61afef")).
			Bold(true)

	completedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5c6370")).
			Strikethrough(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#abb2bf"))

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5c6370"))

	checkStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#98c379"))

	progressStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#c678dd"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5c6370"))
)

type Mode int

const (
	ModeNormal Mode = iota
	ModeAddTask
	ModeSwitchList
)

type TaskListModel struct {
	storage     *storage.Storage
	listName    string
	tasks       []*task.Task
	taskList    *task.TaskList
	cursor      int
	selected    map[int]bool
	mode        Mode
	textInput   textinput.Model
	lists       []string
	listCursor  int
	width       int
	height      int
	quitting    bool
	needsReload bool
}

func NewTaskListModel(store *storage.Storage) *TaskListModel {
	ti := textinput.New()
	ti.Placeholder = "Enter task..."
	ti.CharLimit = 200
	ti.Width = 60

	return &TaskListModel{
		storage:   store,
		selected:  make(map[int]bool),
		textInput: ti,
		mode:      ModeNormal,
	}
}

func (m *TaskListModel) Init() tea.Cmd {
	return m.loadCurrentList()
}

func (m *TaskListModel) loadCurrentList() tea.Cmd {
	return func() tea.Msg {
		listName := m.storage.GetCurrentList()

		// Reset today's list if needed
		if listName == "today" {
			m.storage.ResetTodayList()
		}

		taskList, err := m.storage.LoadList(listName)
		if err != nil {
			return errMsg{err}
		}

		return listLoadedMsg{
			name:     listName,
			taskList: taskList,
		}
	}
}

func (m *TaskListModel) loadLists() tea.Cmd {
	return func() tea.Msg {
		lists, err := m.storage.GetAllLists()
		if err != nil {
			return errMsg{err}
		}
		return listsLoadedMsg{lists: lists}
	}
}

type listLoadedMsg struct {
	name     string
	taskList *task.TaskList
}

type listsLoadedMsg struct {
	lists []string
}

type errMsg struct {
	err error
}

func (m *TaskListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case listLoadedMsg:
		m.listName = msg.name
		m.taskList = msg.taskList
		m.tasks = msg.taskList.SortedTasks()
		m.cursor = 0
		m.needsReload = false
		return m, nil

	case listsLoadedMsg:
		m.lists = msg.lists
		m.listCursor = 0
		// Find current list in the list
		for i, name := range m.lists {
			if name == m.listName {
				m.listCursor = i
				break
			}
		}
		return m, nil

	case errMsg:
		// Handle error
		return m, nil
	}

	if m.mode == ModeAddTask {
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *TaskListModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case ModeAddTask:
		return m.handleAddTaskKey(msg)
	case ModeSwitchList:
		return m.handleSwitchListKey(msg)
	default:
		return m.handleNormalKey(msg)
	}
}

func (m *TaskListModel) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.quitting = true
		return m, tea.Quit

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}

	case "down", "j":
		if m.cursor < len(m.tasks)-1 {
			m.cursor++
		}

	case "enter", " ":
		if len(m.tasks) > 0 {
			return m, m.toggleTask(m.cursor)
		}

	case "a", "A":
		m.mode = ModeAddTask
		m.textInput.SetValue("")
		m.textInput.Focus()
		return m, textinput.Blink

	case "L":
		m.mode = ModeSwitchList
		return m, m.loadLists()

	case "d":
		if len(m.tasks) > 0 {
			return m, m.deleteTask(m.cursor)
		}
	}

	return m, nil
}

func (m *TaskListModel) handleAddTaskKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = ModeNormal
		m.textInput.Blur()
		return m, nil

	case "enter":
		content := strings.TrimSpace(m.textInput.Value())
		if content != "" {
			m.mode = ModeNormal
			m.textInput.Blur()
			return m, m.addTask(content)
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m *TaskListModel) handleSwitchListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.mode = ModeNormal
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
			m.mode = ModeNormal
			return m, m.loadCurrentList()
		}
	}

	return m, nil
}

func (m *TaskListModel) toggleTask(displayIndex int) tea.Cmd {
	return func() tea.Msg {
		if displayIndex < 0 || displayIndex >= len(m.tasks) {
			return nil
		}

		// Find original index
		selectedTask := m.tasks[displayIndex]
		originalIndex := m.taskList.OriginalIndex(selectedTask)
		if originalIndex < 0 {
			return nil
		}

		// Toggle the task
		toggled := m.taskList.Toggle(originalIndex)

		// Handle recurring tasks
		if toggled != nil && toggled.Completed && toggled.RecurDays > 0 {
			nextTask := toggled.CreateNextRecurrence()
			if nextTask != nil {
				// If from today's list with source, add to source list
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

		// Sync to source list if this is today's list
		if m.listName == "today" && toggled != nil && toggled.Source != "" {
			m.syncToSource(toggled)
		}

		// Save the list
		m.storage.SaveList(m.taskList)

		// Reload to refresh the view
		return listLoadedMsg{
			name:     m.listName,
			taskList: m.taskList,
		}
	}
}

func (m *TaskListModel) syncToSource(todayTask *task.Task) {
	sourceList, err := m.storage.LoadList(todayTask.Source)
	if err != nil {
		return
	}

	// Find matching task in source and sync completion status
	for _, t := range sourceList.Tasks {
		if t.Content == todayTask.Content {
			t.Completed = todayTask.Completed
			break
		}
	}

	m.storage.SaveList(sourceList)
}

func (m *TaskListModel) deleteTask(displayIndex int) tea.Cmd {
	return func() tea.Msg {
		if displayIndex < 0 || displayIndex >= len(m.tasks) {
			return nil
		}

		selectedTask := m.tasks[displayIndex]
		originalIndex := m.taskList.OriginalIndex(selectedTask)
		if originalIndex < 0 {
			return nil
		}

		m.taskList.Delete(originalIndex)
		m.storage.SaveList(m.taskList)

		return listLoadedMsg{
			name:     m.listName,
			taskList: m.taskList,
		}
	}
}

func (m *TaskListModel) addTask(content string) tea.Cmd {
	return func() tea.Msg {
		// Parse due date syntax: +N or +Nr
		dueOffset := 0
		recurDays := 0

		// Check for +Nr (recurring)
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

		return listLoadedMsg{
			name:     m.listName,
			taskList: m.taskList,
		}
	}
}

func (m *TaskListModel) View() string {
	if m.quitting {
		return ""
	}

	var sb strings.Builder

	// Header
	completed := m.taskList.CompletedCount()
	total := m.taskList.Len()
	title := fmt.Sprintf("📝 %s (%d/%d)", m.listName, completed, total)
	sb.WriteString(titleStyle.Render(title))
	sb.WriteString("\n\n")

	switch m.mode {
	case ModeAddTask:
		sb.WriteString("Add task: ")
		sb.WriteString(m.textInput.View())
		sb.WriteString("\n\n")
		sb.WriteString(helpStyle.Render("Enter: add • Esc: cancel"))

	case ModeSwitchList:
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

	default:
		// Task list header
		header := fmt.Sprintf(" %-5s │ %-50s │ %10s", "#", "Task", "Due")
		sb.WriteString(headerStyle.Render(header))
		sb.WriteString("\n")

		if len(m.tasks) == 0 {
			sb.WriteString(normalStyle.Render("  No tasks. Press 'a' to add one."))
			sb.WriteString("\n")
		} else {
			for i, t := range m.tasks {
				progress := fmt.Sprintf("%d/%d", i+1, total)

				cursor := "  "
				if i == m.cursor {
					cursor = "> "
				}

				check := "☐"
				if t.Completed {
					check = "☑"
				}

				due := ""
				if t.DueDate != nil {
					due = t.DueDate.Format("2006-01-02")
				}

				content := t.DisplayContent()
				if len(content) > 48 {
					content = content[:48] + "…"
				}

				line := fmt.Sprintf("%s%s %s │ %-50s │ %10s",
					cursor,
					progressStyle.Render(fmt.Sprintf("%-5s", progress)),
					checkStyle.Render(check),
					content,
					due,
				)

				if i == m.cursor {
					if t.Completed {
						sb.WriteString(completedStyle.Render(line))
					} else {
						sb.WriteString(selectedStyle.Render(line))
					}
				} else if t.Completed {
					sb.WriteString(completedStyle.Render(line))
				} else {
					sb.WriteString(normalStyle.Render(line))
				}
				sb.WriteString("\n")
			}
		}

		sb.WriteString("\n")
		sb.WriteString(helpStyle.Render("↑/↓: navigate • Enter: toggle • a: add • d: delete • L: switch list • q: quit"))
	}

	return sb.String()
}

func RunTaskList(store *storage.Storage) error {
	model := NewTaskListModel(store)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
