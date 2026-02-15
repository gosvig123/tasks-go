package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/krisitan/tasks-go/internal/task"
)

// Timeline styles
var (
	nowMarkerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#e06c75"))

	// Pinned task block: teal/blue background
	pinnedBlockStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#1a3a4a")).
				Foreground(lipgloss.Color("#abb2bf"))

	pinnedBlockSelectedStyle = lipgloss.NewStyle().
					Background(lipgloss.Color("#1a4a5a")).
					Foreground(lipgloss.Color("#61afef")).
					Bold(true)

	// Floating task block: muted purple/grey background
	floatingBlockStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#2c2535")).
				Foreground(lipgloss.Color("#abb2bf"))

	floatingBlockSelectedStyle = lipgloss.NewStyle().
					Background(lipgloss.Color("#3c2545")).
					Foreground(lipgloss.Color("#61afef")).
					Bold(true)

	// Completed block: faint
	blockCompletedStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#1e2127")).
				Foreground(lipgloss.Color("#5c6370")).
				Faint(true)
)

// TimelineSlot represents a task placed in the timeline with computed position
type TimelineSlot struct {
	Item     TaskItem
	StartMin int  // start time in minutes from midnight
	EndMin   int  // end time in minutes from midnight
	IsPinned bool // true if task has @at, false if floating
}

// TimelineLayout holds the computed layout for the timeline
type TimelineLayout struct {
	Slots         []TimelineSlot // tasks placed in the timeline
	Unscheduled   []TaskItem     // tasks without @est
	DayStartMin   int            // earliest time shown (in minutes)
	DayEndMin     int            // latest time shown (in minutes)
	TotalEstimate time.Duration  // sum of all estimates
}

// computeTimelineLayout computes the timeline layout from a list of task items.
// dayStartHour and dayEndHour define the default visible range (e.g. 8, 18).
func computeTimelineLayout(items []TaskItem, dayStartHour, dayEndHour int) *TimelineLayout {
	layout := &TimelineLayout{
		DayStartMin: dayStartHour * 60,
		DayEndMin:   dayEndHour * 60,
	}

	var pinned []TimelineSlot
	var floating []TaskItem

	for _, item := range items {
		t := item.Task

		if t.Estimate == nil {
			// No estimate → unscheduled section
			layout.Unscheduled = append(layout.Unscheduled, item)
			continue
		}

		estMin := int(t.Estimate.Minutes())
		if !t.Completed {
			layout.TotalEstimate += *t.Estimate
		}

		if t.StartTime != nil {
			// Pinned: has StartTime AND Estimate
			startMin := t.StartTime.ToMinutes()
			pinned = append(pinned, TimelineSlot{
				Item:     item,
				StartMin: startMin,
				EndMin:   startMin + estMin,
				IsPinned: true,
			})
		} else {
			// Floating: has Estimate but no StartTime
			floating = append(floating, item)
		}
	}

	// Sort pinned tasks by StartTime
	sort.Slice(pinned, func(i, j int) bool {
		return pinned[i].StartMin < pinned[j].StartMin
	})

	// Resolve overlaps among pinned tasks: later tasks get pushed forward
	resolvePinnedOverlaps(pinned)

	// Add pinned tasks to layout
	layout.Slots = append(layout.Slots, pinned...)

	// Sort floating tasks by original file index so that toggling
	// completion (which reorders m.items) doesn't change their timeline position.
	sort.Slice(floating, func(i, j int) bool {
		return floating[i].Index < floating[j].Index
	})

	// Place floating tasks in gaps between pinned tasks
	placeFloatingTasks(layout, floating)

	// Expand DayStartMin and DayEndMin if any slot falls outside
	for _, slot := range layout.Slots {
		if slot.StartMin < layout.DayStartMin {
			layout.DayStartMin = slot.StartMin
		}
		if slot.EndMin > layout.DayEndMin {
			layout.DayEndMin = slot.EndMin
		}
	}

	// Snap DayStartMin down and DayEndMin up to nearest 30-minute boundary
	// so time labels always land on clean :00 or :30 marks
	layout.DayStartMin = (layout.DayStartMin / 30) * 30
	layout.DayEndMin = ((layout.DayEndMin + 29) / 30) * 30

	return layout
}

// resolvePinnedOverlaps pushes overlapping pinned tasks forward so they don't overlap.
// Tasks are already sorted by StartMin. When two tasks overlap, the later one
// is pushed to start right after the earlier one ends.
func resolvePinnedOverlaps(pinned []TimelineSlot) {
	for i := 1; i < len(pinned); i++ {
		prev := pinned[i-1]
		if pinned[i].StartMin < prev.EndMin {
			// Overlap: push this task to start after previous ends
			duration := pinned[i].EndMin - pinned[i].StartMin
			pinned[i].StartMin = prev.EndMin
			pinned[i].EndMin = pinned[i].StartMin + duration
		}
	}
}

// placeFloatingTasks fills gaps between pinned tasks with floating tasks.
func placeFloatingTasks(layout *TimelineLayout, floating []TaskItem) {
	if len(floating) == 0 {
		return
	}

	// Build a list of gaps from the resolved pinned slots
	type gap struct {
		startMin int
		endMin   int
	}

	var gaps []gap
	pinnedSlots := layout.Slots

	if len(pinnedSlots) == 0 {
		// No pinned tasks — the entire day is one big gap
		gaps = append(gaps, gap{startMin: layout.DayStartMin, endMin: layout.DayEndMin})
	} else {
		// Gap before first pinned task
		if pinnedSlots[0].StartMin > layout.DayStartMin {
			gaps = append(gaps, gap{startMin: layout.DayStartMin, endMin: pinnedSlots[0].StartMin})
		}
		// Gaps between pinned tasks
		for i := 0; i < len(pinnedSlots)-1; i++ {
			gapStart := pinnedSlots[i].EndMin
			gapEnd := pinnedSlots[i+1].StartMin
			if gapEnd > gapStart {
				gaps = append(gaps, gap{startMin: gapStart, endMin: gapEnd})
			}
		}
		// Gap after last pinned task
		lastEnd := pinnedSlots[len(pinnedSlots)-1].EndMin
		gaps = append(gaps, gap{startMin: lastEnd, endMin: max(layout.DayEndMin, lastEnd+600)})
	}

	// Place each floating task in the first gap that fits
	gapUsed := make([]int, len(gaps))

	for _, item := range floating {
		estMin := int(item.Task.Estimate.Minutes())
		placed := false

		for g := 0; g < len(gaps); g++ {
			available := (gaps[g].endMin - gaps[g].startMin) - gapUsed[g]
			if estMin <= available {
				startMin := gaps[g].startMin + gapUsed[g]
				layout.Slots = append(layout.Slots, TimelineSlot{
					Item:     item,
					StartMin: startMin,
					EndMin:   startMin + estMin,
					IsPinned: false,
				})
				gapUsed[g] += estMin
				placed = true
				break
			}
		}

		if !placed {
			// Append after everything else
			lastEnd := layout.DayStartMin
			for _, s := range layout.Slots {
				if s.EndMin > lastEnd {
					lastEnd = s.EndMin
				}
			}
			layout.Slots = append(layout.Slots, TimelineSlot{
				Item:     item,
				StartMin: lastEnd,
				EndMin:   lastEnd + estMin,
				IsPinned: false,
			})
		}
	}

	// Re-sort all slots by start time for rendering order
	sort.Slice(layout.Slots, func(i, j int) bool {
		return layout.Slots[i].StartMin < layout.Slots[j].StartMin
	})
}

// renderTimeline renders the vertical timeline view.
func renderTimeline(layout *TimelineLayout, panelWidth, panelHeight int, selectedTask *task.Task, now time.Time) string {
	var sb strings.Builder

	// Header
	sb.WriteString(titleStyle.Render("Timeline"))
	sb.WriteString("\n")
	sb.WriteString(headerStyle.Render(fmt.Sprintf("Total: %s", formatDuration(layout.TotalEstimate))))
	sb.WriteString("\n")

	// Calculate available rows
	headerRows := 2
	unschedRows := 0
	if len(layout.Unscheduled) > 0 {
		unschedRows = len(layout.Unscheduled) + 3 // separator + title + items + blank
	}
	helpRows := 2

	availableRows := panelHeight - headerRows - unschedRows - helpRows
	if availableRows < 1 {
		availableRows = 1
	}

	// Calculate minutes per row
	totalMinutes := layout.DayEndMin - layout.DayStartMin
	if totalMinutes <= 0 {
		totalMinutes = 60 // fallback: 1 hour
	}

	rawMinsPerRow := totalMinutes / availableRows
	if rawMinsPerRow < 1 {
		rawMinsPerRow = 1
	}

	// Snap minsPerRow to a value that divides evenly into 60,
	// so time labels always land on clean :00 or :30 boundaries.
	// Pick the smallest clean divisor >= rawMinsPerRow (capped at 30).
	cleanIntervals := []int{1, 2, 3, 5, 6, 10, 15, 20, 30}
	minsPerRow := 30 // fallback
	for _, iv := range cleanIntervals {
		if iv >= rawMinsPerRow {
			minsPerRow = iv
			break
		}
	}
	availableRows = totalMinutes / minsPerRow

	// Pre-compute which slot occupies each row
	type rowInfo struct {
		slot     *TimelineSlot
		isFirst  bool
		isLast   bool
		isMiddle bool
	}

	rowSlots := make([]rowInfo, availableRows)
	nextFreeRow := 0 // tracks the first row not yet claimed by any block

	for i := range layout.Slots {
		slot := &layout.Slots[i]
		naturalStartRow := (slot.StartMin - layout.DayStartMin) / minsPerRow
		naturalEndRow := (slot.EndMin - layout.DayStartMin + minsPerRow - 1) / minsPerRow // ceiling

		// Clamp to available rows
		if naturalStartRow < 0 {
			naturalStartRow = 0
		}
		if naturalEndRow > availableRows {
			naturalEndRow = availableRows
		}
		if naturalEndRow <= naturalStartRow {
			naturalEndRow = naturalStartRow + 1
		}
		if naturalEndRow > availableRows {
			naturalEndRow = availableRows
		}

		// Prevent overlap: if this block's natural start row is already claimed,
		// push it down to the next free row. This handles cases where a short task
		// (e.g. 30m with minsPerRow=30 = 1 row) is followed by another task that
		// would otherwise start on the same or adjacent row.
		startRow := naturalStartRow
		if startRow < nextFreeRow {
			startRow = nextFreeRow
		}

		// Maintain block height (number of visual rows)
		blockHeight := naturalEndRow - naturalStartRow
		endRow := startRow + blockHeight
		if endRow > availableRows {
			endRow = availableRows
		}
		if endRow <= startRow {
			continue // can't fit this block
		}

		for r := startRow; r < endRow; r++ {
			ri := rowInfo{slot: slot}
			if r == startRow {
				ri.isFirst = true
			}
			if r == endRow-1 {
				ri.isLast = true
			}
			if r != startRow && r != endRow-1 {
				ri.isMiddle = true
			}
			rowSlots[r] = ri
		}

		nextFreeRow = endRow
	}

	// Calculate "now" marker row
	nowMin := now.Hour()*60 + now.Minute()
	nowRow := -1
	if nowMin >= layout.DayStartMin && nowMin < layout.DayEndMin {
		nowRow = (nowMin - layout.DayStartMin) / minsPerRow
		if nowRow >= availableRows {
			nowRow = availableRows - 1
		}
	}

	// Label column width: "HH:MM │ " — use actual display width
	labelWidth := lipgloss.Width("00:00 │ ")
	contentWidth := panelWidth - labelWidth
	if contentWidth < 10 {
		contentWidth = 10
	}

	// Render each row
	lastLabelMin := -1
	for r := 0; r < availableRows; r++ {
		rowMin := layout.DayStartMin + r*minsPerRow
		rowHour := rowMin / 60
		rowMinute := rowMin % 60

		// "Now" marker takes precedence
		if r == nowRow {
			nowStr := fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())
			// Build the now marker line
			markerLeft := "──── "
			markerRight := " "
			remaining := panelWidth - lipgloss.Width(markerLeft) - lipgloss.Width(nowStr) - lipgloss.Width(markerRight)
			if remaining < 0 {
				remaining = 0
			}
			markerLine := markerLeft + nowStr + markerRight + strings.Repeat("─", remaining)
			sb.WriteString(nowMarkerStyle.Render(markerLine))
			sb.WriteString("\n")
			continue
		}

		// Show label at each full hour (:00) and half hour (:30)
		label := "      │ "
		isCleanBoundary := rowMinute == 0 || rowMinute == 30
		if isCleanBoundary && rowMin != lastLabelMin {
			label = fmt.Sprintf("%02d:%02d │ ", rowHour, rowMinute)
			lastLabelMin = rowMin
		}

		ri := rowSlots[r]
		if ri.slot == nil {
			// Empty row
			sb.WriteString(headerStyle.Render(label))
			sb.WriteString("\n")
			continue
		}

		slot := ri.slot
		isSelected := selectedTask != nil && slot.Item.Task == selectedTask
		isCompleted := slot.Item.Task.Completed

		// Build task content for this row
		taskContent := renderTimelineSlotRow(slot, ri.isFirst, ri.isLast, ri.isMiddle, contentWidth)

		// Pad content to full width for consistent background fill
		contentDisplayWidth := lipgloss.Width(taskContent)
		if contentDisplayWidth < contentWidth {
			taskContent += strings.Repeat(" ", contentWidth-contentDisplayWidth)
		}

		// Apply styles based on block type and selection state
		var style lipgloss.Style
		if isCompleted {
			style = blockCompletedStyle
		} else if isSelected {
			if slot.IsPinned {
				style = pinnedBlockSelectedStyle
			} else {
				style = floatingBlockSelectedStyle
			}
		} else {
			if slot.IsPinned {
				style = pinnedBlockStyle
			} else {
				style = floatingBlockStyle
			}
		}

		line := headerStyle.Render(label) + style.Render(taskContent)

		sb.WriteString(line)
		sb.WriteString("\n")
	}

	// Unscheduled section
	if len(layout.Unscheduled) > 0 {
		sb.WriteString(separatorStyle.Render(strings.Repeat("─", panelWidth)))
		sb.WriteString("\n")
		sb.WriteString(headerStyle.Render("Unscheduled:"))
		sb.WriteString("\n")

		for _, item := range layout.Unscheduled {
			check := "☐"
			if item.Task.Completed {
				check = "✓"
			}
			name := item.Task.DisplayContent()
			if item.IsSubtask {
				name = "↳ " + name
			}
			maxName := panelWidth - lipgloss.Width("  ☐  (no estimate)")
			if maxName < 10 {
				maxName = 10
			}
			nameRunes := []rune(name)
			if len(nameRunes) > maxName {
				name = string(nameRunes[:maxName-1]) + "…"
			}

			line := fmt.Sprintf("  %s %s (no estimate)", check, name)

			isSelected := selectedTask != nil && item.Task == selectedTask
			if isSelected {
				sb.WriteString(selectedStyle.Render(line))
			} else if item.Task.Completed {
				sb.WriteString(completedStyle.Render(line))
			} else {
				sb.WriteString(normalStyle.Render(line))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// renderTimelineSlotRow renders a single row of a task block in the timeline.
func renderTimelineSlotRow(slot *TimelineSlot, isFirst, isLast, isMiddle bool, width int) string {
	t := slot.Item.Task

	check := "☐"
	if t.Completed {
		check = "✓"
	}

	dur := ""
	if t.Estimate != nil {
		dur = fmt.Sprintf("(%s)", formatDuration(*t.Estimate))
	}

	timeStr := ""
	if slot.IsPinned && t.StartTime != nil {
		timeStr = fmt.Sprintf("  %s ", t.StartTime.String())
	}

	name := t.DisplayContent()
	if slot.Item.IsSubtask {
		name = "↳ " + name
	}

	if slot.IsPinned {
		return renderPinnedRow(check, name, timeStr, dur, isFirst, isLast, isMiddle, width)
	}
	return renderFloatingRow(check, name, dur, isFirst, isLast, isMiddle, width)
}

// renderPinnedRow renders a row for a pinned task using box-drawing characters.
func renderPinnedRow(check, name, timeStr, dur string, isFirst, isLast, isMiddle bool, width int) string {
	// Account for border characters: "┌ " (left) + " ┐" (right) = 4 display cells
	borderWidth := lipgloss.Width("┌ ") + lipgloss.Width(" ┐")
	innerWidth := width - borderWidth
	if innerWidth < 10 {
		innerWidth = 10
	}

	if isFirst || (isFirst && isLast) {
		content := fmt.Sprintf("%s %s%s %s", check, name, timeStr, dur)
		contentWidth := lipgloss.Width(content)
		if contentWidth > innerWidth {
			// Truncate name to fit
			otherWidth := lipgloss.Width(check) + lipgloss.Width(timeStr) + lipgloss.Width(dur) + 4
			maxName := innerWidth - otherWidth
			if maxName < 1 {
				maxName = 1
			}
			nameRunes := []rune(name)
			if len(nameRunes) > maxName {
				name = string(nameRunes[:maxName-1]) + "…"
			}
			content = fmt.Sprintf("%s %s%s %s", check, name, timeStr, dur)
			contentWidth = lipgloss.Width(content)
		}
		padding := innerWidth - contentWidth
		if padding < 0 {
			padding = 0
		}
		return fmt.Sprintf("┌ %s%s ┐", content, strings.Repeat(" ", padding))
	}

	if isLast {
		return fmt.Sprintf("└─%s─┘", strings.Repeat("─", innerWidth))
	}

	// Middle row
	return fmt.Sprintf("│ %s │", strings.Repeat(" ", innerWidth))
}

// renderFloatingRow renders a row for a floating task using ░ shading.
func renderFloatingRow(check, name, dur string, isFirst, isLast, isMiddle bool, width int) string {
	// Account for border characters: "░ " (left) + " ░" (right)
	borderWidth := lipgloss.Width("░ ") + lipgloss.Width(" ░")
	innerWidth := width - borderWidth
	if innerWidth < 10 {
		innerWidth = 10
	}

	if isFirst {
		content := fmt.Sprintf("%s %s %s", check, name, dur)
		contentWidth := lipgloss.Width(content)
		if contentWidth > innerWidth {
			otherWidth := lipgloss.Width(check) + lipgloss.Width(dur) + 4
			maxName := innerWidth - otherWidth
			if maxName < 1 {
				maxName = 1
			}
			nameRunes := []rune(name)
			if len(nameRunes) > maxName {
				name = string(nameRunes[:maxName-1]) + "…"
			}
			content = fmt.Sprintf("%s %s %s", check, name, dur)
			contentWidth = lipgloss.Width(content)
		}
		padding := innerWidth - contentWidth
		if padding < 0 {
			padding = 0
		}
		return fmt.Sprintf("░ %s%s ░", content, strings.Repeat(" ", padding))
	}

	if isLast && !isFirst {
		return fmt.Sprintf("░─%s─░", strings.Repeat("─", innerWidth))
	}

	// Middle row
	return fmt.Sprintf("░ %s ░", strings.Repeat(" ", innerWidth))
}

// formatDuration returns a compact duration format: "2h", "30m", "1h30m"
func formatDuration(d time.Duration) string {
	totalMinutes := int(d.Minutes())
	if totalMinutes <= 0 {
		return "0m"
	}

	hours := totalMinutes / 60
	minutes := totalMinutes % 60

	if hours > 0 && minutes > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dm", minutes)
}
