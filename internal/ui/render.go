package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/krisitan/tasks-go/internal/task"
)

func (m *TaskViewModel) calcSplitWidths() (int, int) {
	w := m.width
	if w < 60 {
		w = 80
	}
	left := (w * 55) / 100
	right := w - left - 3
	if left < 30 {
		left = 30
	}
	if right < 25 {
		right = 25
	}
	return left, right
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
		leftWidth, rightWidth := m.calcSplitWidths()

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
		leftWidth, rightWidth := m.calcSplitWidths()
		leftContent := m.renderTaskListPanel(leftWidth)
		leftPanel := panelBorderStyle.Width(leftWidth).Render(leftContent)

		parentName := ""
		if m.subtaskParentIdx < len(m.items) {
			parentName = m.items[m.subtaskParentIdx].Task.Content
		}
		rightContent := m.renderTaskFormPanel(fmt.Sprintf("Add Subtask to: %s", parentName), "add subtask")
		rightPanel := panelBorderStyle.Width(rightWidth).Render(rightContent)

		splitView := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, " ", rightPanel)
		sb.WriteString(splitView)

		return sb.String()

	case InputEditTask, InputViewTask:
		// Split-screen view: tasks on left, edit form or details on right
		leftWidth, rightWidth := m.calcSplitWidths()

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

	// Fixed widths: cursor(2) + checkbox(2) + date(18) + padding(4)
	fixedWidth := 26
	contentWidth := termWidth - fixedWidth - listColWidth
	if contentWidth < 20 {
		contentWidth = 20
	}

	// Header
	if m.showLists {
		header := fmt.Sprintf("   %-*s  %-*s  %s", listColWidth-2, "List", contentWidth, "Task", "Date")
		sb.WriteString(headerStyle.Render(header))
	} else {
		header := fmt.Sprintf("   %-*s  %s", contentWidth, "Task", "Date")
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

	termWidth := m.width
	if termWidth < 80 {
		termWidth = 80
	}
	termHeight := m.height
	if termHeight < 15 {
		termHeight = 30
	}

	// Width split: task list on left (55%), timeline on right (45%)
	leftWidth := (termWidth * 55) / 100
	rightWidth := termWidth - leftWidth - 3 // 3 = 1 gap char + 2 margin
	if leftWidth < 30 {
		leftWidth = 30
	}
	if rightWidth < 30 {
		rightWidth = 30
	}

	// Panel height: full terminal minus overhead rows (title + blank + \n + help)
	overhead := 5
	if m.inputMode == InputSearch {
		overhead += 2
	}
	if m.statusMsg != "" {
		overhead += 1
	}
	panelHeight := termHeight - overhead
	if panelHeight < 5 {
		panelHeight = 5
	}

	debugLog.Printf("renderTimelineSplitView: termW=%d termH=%d leftW=%d rightW=%d panelH=%d", termWidth, termHeight, leftWidth, rightWidth, panelHeight)

	// Left panel: task list
	leftStyle := panelBorderStyle
	if !m.timelineFocus {
		leftStyle = focusedPanelBorderStyle
	}
	leftContent := m.renderTaskListPanel(leftWidth)
	leftPanel := leftStyle.Width(leftWidth).Height(panelHeight).Render(leftContent)

	// Right panel: timeline
	var selectedTask *task.Task
	if m.cursor >= 0 && m.cursor < len(m.items) {
		selectedTask = m.items[m.cursor].Task
	}
	if m.timelineLayout == nil {
		m.timelineLayout = computeTimelineLayout(m.items, 8, 18)
	}
	timelineContentWidth := rightWidth - 2
	if timelineContentWidth < 10 {
		timelineContentWidth = 10
	}
	timelineContentHeight := panelHeight - 2
	if timelineContentHeight < 1 {
		timelineContentHeight = 1
	}
	timelineContent := renderTimeline(m.timelineLayout, timelineContentWidth, timelineContentHeight, selectedTask, time.Now())

	rightStyle := panelBorderStyle
	if m.timelineFocus {
		rightStyle = focusedPanelBorderStyle
	}
	rightPanel := rightStyle.Width(rightWidth).Height(panelHeight).Render(timelineContent)

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

	// Panels side by side
	sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, " ", rightPanel))

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

	fixedWidth := 26
	contentWidth := panelWidth - fixedWidth - listColWidth
	if contentWidth < 15 {
		contentWidth = 15
	}

	// Header
	if m.showLists {
		header := fmt.Sprintf("   %-*s  %-*s  %s", listColWidth-2, "List", contentWidth, "Task", "Date")
		sb.WriteString(headerStyle.Render(header))
	} else {
		header := fmt.Sprintf("   %-*s  %s", contentWidth, "Task", "Date")
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

	// Completion date
	if t.Completed && t.CompletedAt != nil {
		sb.WriteString(headerStyle.Render("Completed on: "))
		sb.WriteString(normalStyle.Render(t.CompletedAt.Format("2006-01-02")))
		sb.WriteString("\n\n")
	}

	// Content
	sb.WriteString(headerStyle.Render("Task:"))
	sb.WriteString("\n")
	// Word wrap content to fit panel
	content := t.Content
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
	textRunes := []rune(text)
	if len(textRunes) <= maxWidth {
		return text
	}

	var result strings.Builder
	words := strings.Fields(text)
	lineLen := 0

	for i, word := range words {
		wordLen := len([]rune(word))
		if i > 0 {
			if lineLen+1+wordLen > maxWidth {
				result.WriteString("\n")
				lineLen = 0
			} else {
				result.WriteString(" ")
				lineLen++
			}
		}
		result.WriteString(word)
		lineLen += wordLen
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
	if item.Task.Completed && item.Task.CompletedAt != nil {
		due = "Done: " + item.Task.CompletedAt.Format("2006-01-02")
	} else if item.Task.DueDate != nil {
		due = item.Task.DueDate.Format("2006-01-02")
	}

	content := item.Task.Content

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
		contentRunes := []rune(content)
		if len(contentRunes) > maxContent && maxContent > 1 {
			content = string(contentRunes[:maxContent-1]) + "…"
		}
		listNameWidth := listColWidth - 2
		return fmt.Sprintf("%s%s%-*s %s %-*s  %s", indent, cursor, listNameWidth, item.ListName, check, maxContent, content, due)
	}

	// Single list view
	maxContent := contentWidth - 2
	if item.IsSubtask {
		maxContent -= 4
	}
	contentRunes := []rune(content)
	if len(contentRunes) > maxContent && maxContent > 1 {
		content = string(contentRunes[:maxContent-1]) + "…"
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
	descRunes := []rune(desc)
	if len(descRunes) > maxDesc {
		desc = string(descRunes[:maxDesc-1]) + "…"
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
