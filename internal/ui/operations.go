package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/krisitan/tasks-go/internal/storage"
	"github.com/krisitan/tasks-go/internal/task"
)

func (m *TaskViewModel) saveEditedTask(content, description, dueValue, recurValue, estValue, startValue string) tea.Cmd {
	return func() tea.Msg {
		if m.cursor >= len(m.items) {
			return nil
		}

		item := m.items[m.cursor]

		// Handle upcoming items — save to source list
		if item.IsUpcoming {
			dueOffset, specificDate := parseDueValue(dueValue)
			recurDays := parseRecurValue(recurValue)
			estimate := parseEstimateValue(estValue)
			startTime := parseStartTimeValue(startValue)
			list, err := m.storage.LoadList(item.ListName)
			if err != nil {
				return nil
			}
			originalContent := strings.TrimSpace(item.Task.Content)
			for _, t := range list.Tasks {
				if strings.TrimSpace(t.Content) == originalContent {
					t.Content = content
					t.Description = description
					t.RecurDays = recurDays
					t.Estimate = estimate
					t.StartTime = startTime
					if specificDate != nil {
						t.DueDate = specificDate
					} else if dueOffset > 0 {
						due := time.Now().AddDate(0, 0, dueOffset)
						t.DueDate = &due
					} else if dueValue == "" {
						t.DueDate = nil
					}
					break
				}
			}
			if err := m.storage.SaveList(list); err != nil {
				debugLog.Printf("Error saving list %s: %v", list.Name, err)
			}
			m.inputMode = InputNormal
			return m.loadTasks()()
		}

		// Parse form values
		dueOffset, specificDate := parseDueValue(dueValue)
		recurDays := parseRecurValue(recurValue)
		estimate := parseEstimateValue(estValue)
		startTime := parseStartTimeValue(startValue)

		// Update the task in-place (item.Task is the same pointer as in m.taskList)
		item.Task.Content = content
		item.Task.Description = description
		item.Task.RecurDays = recurDays
		item.Task.Estimate = estimate
		item.Task.StartTime = startTime

		if specificDate != nil {
			item.Task.DueDate = specificDate
		} else if dueOffset > 0 {
			due := time.Now().AddDate(0, 0, dueOffset)
			item.Task.DueDate = &due
		} else if dueValue == "" {
			item.Task.DueDate = nil
		}

		listName := m.listName
		if m.viewMode == ViewAllPending {
			listName = item.ListName
		}

		// ViewSingleList: item.Task is the same pointer as m.taskList.Tasks[item.Index],
		// so the in-place mutation above already updated m.taskList. Save it directly
		// and return a lightweight refresh instead of a full disk reload so the cursor
		// stays on the edited task and the timeline updates immediately.
		if m.viewMode == ViewSingleList && m.taskList != nil {
			if err := m.storage.SaveList(m.taskList); err != nil {
				debugLog.Printf("Error saving list %s: %v", m.taskList.Name, err)
			}

			if listName == "today" && item.Task.IsReference() {
				m.syncMetadataToSource(item.Task)

				if !item.Task.IsDueToday() {
					// Task is no longer due today — remove stub and do a full reload
					// (cursor reset is acceptable since the task disappears from the list)
					m.taskList.Delete(item.Index)
					if err := m.storage.SaveList(m.taskList); err != nil {
						debugLog.Printf("Error saving list %s: %v", m.taskList.Name, err)
					}
					m.inputMode = InputNormal
					return m.loadTasks()()
				}
			}

			m.inputMode = InputNormal
			return refreshDisplayMsg{}
		}

		// ViewAllPending: m.taskList is nil, so load the list fresh from disk
		list, err := m.storage.LoadList(listName)
		if err != nil {
			return nil
		}

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
			if err := m.storage.SaveList(list); err != nil {
				debugLog.Printf("Error saving list %s: %v", list.Name, err)
			}
		} else if item.Index < len(list.Tasks) {
			list.Tasks[item.Index].Content = content
			list.Tasks[item.Index].Description = description
			list.Tasks[item.Index].DueDate = item.Task.DueDate
			list.Tasks[item.Index].RecurDays = recurDays
			list.Tasks[item.Index].Estimate = estimate
			list.Tasks[item.Index].StartTime = startTime
			if err := m.storage.SaveList(list); err != nil {
				debugLog.Printf("Error saving list %s: %v", list.Name, err)
			}
		}

		if listName == "today" && item.Task.IsReference() {
			m.syncMetadataToSource(item.Task)

			if !item.Task.IsDueToday() {
				if item.Index < len(list.Tasks) {
					list.Delete(item.Index)
					if err := m.storage.SaveList(list); err != nil {
						debugLog.Printf("Error saving list %s: %v", list.Name, err)
					}
				}
			}
		}

		m.inputMode = InputNormal
		return m.loadTasks()()
	}
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

		if _, err := m.storage.AddSubtask(m.targetForItem(parentItem), subtask); err != nil {
			debugLog.Printf("Error adding subtask: %v", err)
		}

		return m.loadTasks()()
	}
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

func (m *TaskViewModel) targetForItem(item TaskItem) storage.TaskTarget {
	listName := item.ListName
	if listName == "" || (m.viewMode == ViewSingleList && !item.IsUpcoming) {
		listName = m.listName
	}
	return storage.TaskTarget{
		ListName:  listName,
		Index:     item.Index,
		SubIndex:  item.SubIndex,
		IsSubtask: item.IsSubtask,
		Content:   item.Task.Content,
	}
}

func (m *TaskViewModel) toggleTask(idx int) tea.Cmd {
	return func() tea.Msg {
		if idx < 0 || idx >= len(m.items) {
			return nil
		}

		if _, err := m.storage.ToggleTask(m.targetForItem(m.items[idx])); err != nil {
			debugLog.Printf("Error toggling task: %v", err)
		}
		return m.loadTasks()()
	}
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

	if err := m.storage.SaveList(sourceList); err != nil {
		debugLog.Printf("Error saving list %s: %v", sourceList.Name, err)
	}
}

func (m *TaskViewModel) deleteTask(idx int) tea.Cmd {
	return func() tea.Msg {
		if idx < 0 || idx >= len(m.items) {
			return nil
		}

		if _, err := m.storage.DeleteTask(m.targetForItem(m.items[idx])); err != nil {
			debugLog.Printf("Error deleting task: %v", err)
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
			// Auto-assign today's date when adding to the today list with no due date
			if m.listName == "today" {
				t.EnsureDueToday()
			}
			t.Estimate = estimate
			t.StartTime = startTime
			if err := m.storage.SaveList(m.taskList); err != nil {
				debugLog.Printf("Error saving list %s: %v", m.taskList.Name, err)
			}
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
			// Auto-assign today's date when adding to the today list with no due date
			if item.ListName == "today" {
				t.EnsureDueToday()
			}
			t.Estimate = estimate
			t.StartTime = startTime
			if err := m.storage.SaveList(list); err != nil {
				debugLog.Printf("Error saving list %s: %v", list.Name, err)
			}
		} else {
			return nil
		}

		return m.loadTasks()()
	}
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
		// Auto-assign today's date when adding to the today list with no due date
		if listName == "today" {
			t.EnsureDueToday()
		}
		t.Estimate = estimate
		t.StartTime = startTime
		if err := m.storage.SaveList(list); err != nil {
			debugLog.Printf("Error saving list %s: %v", list.Name, err)
		}

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

// ensureDueDateOnSource sets today's date on the source task if it has no due date,
// then persists the change to the source list.
func (m *TaskViewModel) ensureDueDateOnSource(t *task.Task, sourceList string) {
	if t.DueDate != nil {
		return
	}
	t.EnsureDueToday()

	list, err := m.storage.LoadList(sourceList)
	if err != nil {
		return
	}
	content := strings.TrimSpace(t.Content)
	for _, src := range list.Tasks {
		if strings.TrimSpace(src.Content) == content {
			src.DueDate = t.DueDate
			break
		}
	}
	if err := m.storage.SaveList(list); err != nil {
		debugLog.Printf("Error saving list %s: %v", list.Name, err)
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
			m.ensureDueDateOnSource(item.Task, item.ListName)
			todayList.Add(task.NewReferenceStub(item.Task, item.ListName))
			added++
		}

		if added > 0 {
			if err := m.storage.SaveList(todayList); err != nil {
				debugLog.Printf("Error saving list %s: %v", todayList.Name, err)
			}
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

		m.ensureDueDateOnSource(item.Task, sourceList)

		todayList := m.getOrCreateTodayList()
		todayList.Add(task.NewReferenceStub(item.Task, sourceList))
		if err := m.storage.SaveList(todayList); err != nil {
			debugLog.Printf("Error saving list %s: %v", todayList.Name, err)
		}

		return AddedToTodayMsg{TaskName: item.Task.Content}
	}
}
