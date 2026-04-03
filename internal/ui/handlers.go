package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/krisitan/tasks-go/internal/storage"
)

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

	case "T":
		if m.selectedTaskContent == "" {
			return m, nil
		}
		sel, _ := m.storage.GetSelectedTask()
		if sel == nil {
			return m, nil
		}
		if sel.IsRunning() {
			// Pause: freeze accumulated, clear started_at, checkpoint to task
			sel.AccumulatedSecs += int(time.Since(*sel.StartedAt).Seconds())
			sel.StartedAt = nil
			m.timerRunning = false
			m.selectedTaskState = sel
			m.storage.SetSelectedTask(sel)
			if sel.AccumulatedSecs > 0 {
				tracked := time.Duration(sel.AccumulatedSecs) * time.Second
				m.storage.SaveTaskTracked(sel.List, sel.Content, sel.Source, tracked)
			}
			m.statusMsg = "Timer paused"
		} else {
			// Start: set started_at to now
			now := time.Now()
			sel.StartedAt = &now
			m.timerRunning = true
			m.selectedTaskState = sel
			m.storage.SetSelectedTask(sel)
			m.statusMsg = "Timer started"
			m.tickGen++
			gen := m.tickGen
			return m, tea.Tick(time.Second, func(t time.Time) tea.Msg { return timerTickMsg{gen: gen} })
		}

	case "S":
		if len(m.items) > 0 {
			item := m.items[m.cursor]
			if item.IsSubtask {
				return m, nil
			}
			content := strings.TrimSpace(item.Task.Content)
			listName := item.ListName
			if listName == "" {
				listName = m.listName
			}
			// Toggle: if already selected, deselect
			if m.selectedTaskContent == content && m.selectedTaskList == listName {
				// Save tracked time to task before clearing
				sel, _ := m.storage.GetSelectedTask()
				if sel != nil {
					totalSecs := sel.AccumulatedSecs
					if sel.IsRunning() {
						totalSecs += int(time.Since(*sel.StartedAt).Seconds())
					}
					if totalSecs > 0 {
						tracked := time.Duration(totalSecs) * time.Second
						m.storage.SaveTaskTracked(sel.List, sel.Content, sel.Source, tracked)
						item.Task.Tracked = &tracked
					}
				}
				m.timerRunning = false
				m.selectedTaskContent = ""
				m.selectedTaskList = ""
				m.selectedTaskState = nil
				m.storage.ClearSelectedTask()
				m.statusMsg = "Task deselected"
			} else {
				// Save tracked time on old task if switching selection
				if m.selectedTaskContent != "" {
					sel, _ := m.storage.GetSelectedTask()
					if sel != nil {
						totalSecs := sel.AccumulatedSecs
						if sel.IsRunning() {
							totalSecs += int(time.Since(*sel.StartedAt).Seconds())
						}
						if totalSecs > 0 {
							tracked := time.Duration(totalSecs) * time.Second
							m.storage.SaveTaskTracked(sel.List, sel.Content, sel.Source, tracked)
							// Update in-memory tracked time for old task
							for i := range m.items {
								oldList := m.items[i].ListName
								if oldList == "" {
									oldList = m.listName
								}
								if strings.TrimSpace(m.items[i].Task.Content) == sel.Content && oldList == sel.List {
									m.items[i].Task.Tracked = &tracked
									break
								}
							}
						}
					}
					m.timerRunning = false
				}
				// Select new task, loading any existing tracked time
				m.selectedTaskContent = content
				m.selectedTaskList = listName
				now := time.Now()
				var accSecs int
				if item.Task.Tracked != nil {
					accSecs = int(item.Task.Tracked.Seconds())
				}
				newSel := &storage.SelectedTask{
					List:            listName,
					Content:         content,
					Source:          item.Task.Source,
					StartedAt:       &now,
					AccumulatedSecs: accSecs,
				}
				m.storage.SetSelectedTask(newSel)
				m.selectedTaskState = newSel
				m.timerRunning = true
				m.statusMsg = "Task selected"
				m.tickGen++
				gen := m.tickGen
				return m, tea.Tick(time.Second, func(t time.Time) tea.Msg { return timerTickMsg{gen: gen} })
			}
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
		m.blurAllFields()
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
			m.blurAllFields()
			m.focusedField = 0
			return m, m.saveEditedTask(content, m.descInput.Value(), m.dueInput.Value(), m.recurInput.Value(), m.estInput.Value(), m.startInput.Value())
		}
		return m, nil
	}

	return m, m.updateFocusedField(msg)
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

func (m *TaskViewModel) blurAllFields() {
	m.textInput.Blur()
	m.descInput.Blur()
	m.dueInput.Blur()
	m.recurInput.Blur()
	m.estInput.Blur()
	m.startInput.Blur()
}

func (m *TaskViewModel) updateFocusedField(msg tea.Msg) tea.Cmd {
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
	return cmd
}

// focusFormField focuses the appropriate input based on field index
func (m *TaskViewModel) focusFormField(field int) tea.Cmd {
	m.blurAllFields()

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
		m.blurAllFields()
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

			m.blurAllFields()
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

	return m, m.updateFocusedField(msg)
}

func (m *TaskViewModel) handleAddSubtaskKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	const numFields = 6

	switch msg.String() {
	case "esc":
		m.inputMode = InputNormal
		m.blurAllFields()
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
			m.blurAllFields()
			m.focusedField = 0
			m.inputMode = InputNormal
			return m, m.addSubtaskToParent(content, m.descInput.Value(), m.dueInput.Value(), m.recurInput.Value(), m.estInput.Value(), m.startInput.Value())
		}
		return m, nil
	}

	return m, m.updateFocusedField(msg)
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
