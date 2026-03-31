package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/krisitan/tasks-go/internal/storage"
)

// ListSwitcherModel - Interactive list selector
type ListSwitcherModel struct {
	storage       *storage.Storage
	lists         []string
	allLists      []string
	cursor        int
	quitting      bool
	selected      string
	inputMode     InputMode
	searchInput   textinput.Model
	addInput      textinput.Model
	searchQuery   string
	message       string // feedback message
	listInfoCache map[string]*storage.ListInfo
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

func (m *ListSwitcherModel) loadLists() tea.Cmd {
	return func() tea.Msg {
		lists, err := m.storage.GetAllLists()
		if err != nil {
			return nil
		}
		infoCache := make(map[string]*storage.ListInfo, len(lists))
		for _, name := range lists {
			if info, err := m.storage.GetListInfo(name); err == nil {
				infoCache[name] = info
			}
		}
		return listSwitcherLoadedMsg{lists: lists, infoCache: infoCache}
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
		m.listInfoCache = msg.infoCache
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
			info := m.listInfoCache[listName]
			if info == nil {
				info, _ = m.storage.GetListInfo(listName)
			}
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
