package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/krisitan/tasks-go/internal/task"
)

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

		m.saveAndSyncTimeline(t)
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

		m.saveAndSyncTimeline(t)
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

		m.saveAndSyncTimeline(t)
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
		m.saveAndSyncTimeline(t)
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

func (m *TaskViewModel) saveAndSyncTimeline(t *task.Task) {
	if m.taskList != nil {
		if err := m.storage.SaveList(m.taskList); err != nil {
			debugLog.Printf("Error saving list %s: %v", m.taskList.Name, err)
		}
	}
	if m.listName == "today" && t.IsReference() {
		m.syncMetadataToSource(t)
	}
}
