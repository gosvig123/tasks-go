package ui

import (
	"testing"
	"time"

	"github.com/krisitan/tasks-go/internal/task"
)

func TestTimelineSplitViewNoCrash(t *testing.T) {
	est := 2 * time.Hour
	testTask := &task.Task{
		Content:   "buy supplements",
		Completed: false,
		Estimate:  &est,
	}

	items := []TaskItem{
		{Task: testTask, ListName: "personal", Index: 0},
	}

	m := &TaskViewModel{
		viewMode:     ViewSingleList,
		listName:     "today",
		showLists:    true,
		showTimeline: true,
		inputMode:    InputNormal,
		items:        items,
		allItems:     items,
		width:        120,
		height:       40,
		cursor:       0,
	}

	m.timelineLayout = computeTimelineLayout(m.items, 8, 18)

	t.Logf("Timeline layout: slots=%d, unsched=%d, dayStart=%d, dayEnd=%d",
		len(m.timelineLayout.Slots), len(m.timelineLayout.Unscheduled),
		m.timelineLayout.DayStartMin, m.timelineLayout.DayEndMin)

	for i, slot := range m.timelineLayout.Slots {
		t.Logf("  Slot %d: %s start=%d end=%d pinned=%v",
			i, slot.Item.Task.Content, slot.StartMin, slot.EndMin, slot.IsPinned)
	}

	result := m.renderTimelineSplitView()
	t.Logf("renderTimelineSplitView result length: %d", len(result))
	if len(result) == 0 {
		t.Error("Expected non-empty result")
	}
}

func TestTimelineSplitViewSmallTerminal(t *testing.T) {
	est := 2 * time.Hour
	testTask := &task.Task{
		Content:   "buy supplements",
		Completed: false,
		Estimate:  &est,
	}

	items := []TaskItem{
		{Task: testTask, ListName: "personal", Index: 0},
	}

	testCases := []struct {
		name   string
		width  int
		height int
	}{
		{"zero_dimensions", 0, 0},
		{"tiny_width", 20, 10},
		{"tiny_height", 80, 5},
		{"normal", 120, 40},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := &TaskViewModel{
				viewMode:     ViewSingleList,
				listName:     "today",
				showLists:    true,
				showTimeline: true,
				inputMode:    InputNormal,
				items:        items,
				allItems:     items,
				width:        tc.width,
				height:       tc.height,
				cursor:       0,
			}

			m.timelineLayout = computeTimelineLayout(m.items, 8, 18)
			result := m.renderTimelineSplitView()
			t.Logf("[%s] width=%d height=%d result_len=%d", tc.name, tc.width, tc.height, len(result))
		})
	}
}

func TestTimelineViewNoCrash(t *testing.T) {
	est := 2 * time.Hour
	testTask := &task.Task{
		Content:   "buy supplements",
		Completed: false,
		Estimate:  &est,
	}

	items := []TaskItem{
		{Task: testTask, ListName: "personal", Index: 0},
	}

	m := &TaskViewModel{
		viewMode:     ViewSingleList,
		listName:     "today",
		showLists:    true,
		showTimeline: true,
		inputMode:    InputNormal,
		items:        items,
		allItems:     items,
		width:        120,
		height:       40,
		cursor:       0,
	}

	m.timelineLayout = computeTimelineLayout(m.items, 8, 18)

	result := m.View()
	t.Logf("View() result length: %d", len(result))
	if len(result) == 0 {
		t.Error("Expected non-empty result")
	}
}

func TestOverlappingPinnedTasks(t *testing.T) {
	est1 := 2 * time.Hour
	est2 := time.Hour
	est3 := 90 * time.Minute

	// Three pinned tasks that overlap: 09:00-11:00, 10:00-11:00, 10:30-12:00
	items := []TaskItem{
		{Task: &task.Task{
			Content:   "task A",
			Estimate:  &est1,
			StartTime: &task.TimeOfDay{Hour: 9, Minute: 0},
		}, ListName: "today", Index: 0},
		{Task: &task.Task{
			Content:   "task B",
			Estimate:  &est2,
			StartTime: &task.TimeOfDay{Hour: 10, Minute: 0},
		}, ListName: "today", Index: 1},
		{Task: &task.Task{
			Content:   "task C",
			Estimate:  &est3,
			StartTime: &task.TimeOfDay{Hour: 10, Minute: 30},
		}, ListName: "today", Index: 2},
	}

	layout := computeTimelineLayout(items, 8, 18)

	// Verify no overlaps
	for i := 1; i < len(layout.Slots); i++ {
		prev := layout.Slots[i-1]
		curr := layout.Slots[i]
		if curr.StartMin < prev.EndMin {
			t.Errorf("Overlap between slot %d (%s, %d-%d) and slot %d (%s, %d-%d)",
				i-1, prev.Item.Task.Content, prev.StartMin, prev.EndMin,
				i, curr.Item.Task.Content, curr.StartMin, curr.EndMin)
		}
	}

	// Task A: 09:00-11:00 (540-660) — unchanged
	if layout.Slots[0].StartMin != 540 || layout.Slots[0].EndMin != 660 {
		t.Errorf("Task A: expected 540-660, got %d-%d", layout.Slots[0].StartMin, layout.Slots[0].EndMin)
	}

	// Task B: pushed to 11:00-12:00 (660-720)
	if layout.Slots[1].StartMin != 660 || layout.Slots[1].EndMin != 720 {
		t.Errorf("Task B: expected 660-720 (pushed), got %d-%d", layout.Slots[1].StartMin, layout.Slots[1].EndMin)
	}

	// Task C: pushed to 12:00-13:30 (720-810)
	if layout.Slots[2].StartMin != 720 || layout.Slots[2].EndMin != 810 {
		t.Errorf("Task C: expected 720-810 (pushed), got %d-%d", layout.Slots[2].StartMin, layout.Slots[2].EndMin)
	}

	t.Logf("Overlap resolution: A=%d-%d, B=%d-%d, C=%d-%d",
		layout.Slots[0].StartMin, layout.Slots[0].EndMin,
		layout.Slots[1].StartMin, layout.Slots[1].EndMin,
		layout.Slots[2].StartMin, layout.Slots[2].EndMin)
}

func TestMixedTasksInTimeline(t *testing.T) {
	est1 := 2 * time.Hour
	est2 := time.Hour

	items := []TaskItem{
		// Task with estimate (goes to timeline as floating)
		{Task: &task.Task{
			Content:  "has estimate",
			Estimate: &est1,
		}, ListName: "today", Index: 0},
		// Task without estimate (goes to unscheduled)
		{Task: &task.Task{
			Content: "no estimate",
		}, ListName: "today", Index: 1},
		// Completed task with estimate (goes to timeline as completed block)
		{Task: &task.Task{
			Content:   "done with est",
			Completed: true,
			Estimate:  &est2,
		}, ListName: "today", Index: 2},
		// Completed task without estimate (goes to unscheduled)
		{Task: &task.Task{
			Content:   "done no est",
			Completed: true,
		}, ListName: "today", Index: 3},
	}

	layout := computeTimelineLayout(items, 8, 18)

	// Should have 2 slots: "has estimate" (floating) + "done with est" (floating/completed)
	if len(layout.Slots) != 2 {
		t.Errorf("Expected 2 timeline slots, got %d", len(layout.Slots))
		for i, s := range layout.Slots {
			t.Logf("  Slot %d: %s completed=%v", i, s.Item.Task.Content, s.Item.Task.Completed)
		}
	}

	// Should have 2 unscheduled: "no estimate" + "done no est"
	if len(layout.Unscheduled) != 2 {
		t.Errorf("Expected 2 unscheduled, got %d", len(layout.Unscheduled))
		for i, u := range layout.Unscheduled {
			t.Logf("  Unsched %d: %s completed=%v", i, u.Task.Content, u.Task.Completed)
		}
	}

	// TotalEstimate should only count pending tasks (2h, not the completed 1h)
	expectedTotal := 2 * time.Hour
	if layout.TotalEstimate != expectedTotal {
		t.Errorf("TotalEstimate: expected %v, got %v", expectedTotal, layout.TotalEstimate)
	}
}

func TestFloatingTasksNoOverlap(t *testing.T) {
	est1 := 2 * time.Hour
	est2 := time.Hour
	est3 := 3 * time.Hour

	// Pinned task in the middle, floating tasks should fill gaps without overlap
	items := []TaskItem{
		{Task: &task.Task{
			Content:   "pinned",
			Estimate:  &est2,
			StartTime: &task.TimeOfDay{Hour: 12, Minute: 0},
		}, ListName: "today", Index: 0},
		{Task: &task.Task{
			Content:  "float A",
			Estimate: &est1,
		}, ListName: "today", Index: 1},
		{Task: &task.Task{
			Content:  "float B",
			Estimate: &est3,
		}, ListName: "today", Index: 2},
	}

	layout := computeTimelineLayout(items, 8, 18)

	// Verify no overlaps among all slots
	for i := 1; i < len(layout.Slots); i++ {
		prev := layout.Slots[i-1]
		curr := layout.Slots[i]
		if curr.StartMin < prev.EndMin {
			t.Errorf("Overlap between slot %d (%s, %d-%d) and slot %d (%s, %d-%d)",
				i-1, prev.Item.Task.Content, prev.StartMin, prev.EndMin,
				i, curr.Item.Task.Content, curr.StartMin, curr.EndMin)
		}
	}

	for i, s := range layout.Slots {
		t.Logf("  Slot %d: %s start=%d end=%d pinned=%v",
			i, s.Item.Task.Content, s.StartMin, s.EndMin, s.IsPinned)
	}
}

// TestShortPinnedWithFloatingNoVisualOverlap tests the exact scenario:
// a 30m pinned task at 08:00 and a 2h floating task. The floating task must
// start after the pinned task ends, and in the row grid they must not share rows.
func TestShortPinnedWithFloatingNoVisualOverlap(t *testing.T) {
	est30m := 30 * time.Minute
	est2h := 2 * time.Hour

	items := []TaskItem{
		{Task: &task.Task{
			Content:   "read paper",
			Estimate:  &est30m,
			StartTime: &task.TimeOfDay{Hour: 8, Minute: 0},
		}, ListName: "today", Index: 0},
		{Task: &task.Task{
			Content:  "buy supplements",
			Estimate: &est2h,
		}, ListName: "today", Index: 1},
	}

	layout := computeTimelineLayout(items, 8, 18)

	// Layout should place floating task after pinned
	if len(layout.Slots) != 2 {
		t.Fatalf("expected 2 slots, got %d", len(layout.Slots))
	}

	pinned := layout.Slots[0]
	floating := layout.Slots[1]

	t.Logf("Pinned: %s start=%d end=%d", pinned.Item.Task.Content, pinned.StartMin, pinned.EndMin)
	t.Logf("Floating: %s start=%d end=%d", floating.Item.Task.Content, floating.StartMin, floating.EndMin)

	if floating.StartMin < pinned.EndMin {
		t.Errorf("Floating task starts at %d but pinned ends at %d — time overlap!", floating.StartMin, pinned.EndMin)
	}

	// Now test the row mapping: simulate renderTimeline's row assignment logic
	minsPerRow := 30
	availableRows := (layout.DayEndMin - layout.DayStartMin) / minsPerRow

	type rowInfo struct {
		slotIdx int
		isFirst bool
	}

	rowSlots := make([]rowInfo, availableRows)
	for i := range rowSlots {
		rowSlots[i].slotIdx = -1
	}
	nextFreeRow := 0

	for i := range layout.Slots {
		slot := &layout.Slots[i]
		startRow := (slot.StartMin - layout.DayStartMin) / minsPerRow
		endRow := (slot.EndMin - layout.DayStartMin + minsPerRow - 1) / minsPerRow

		if startRow < 0 {
			startRow = 0
		}
		if endRow > availableRows {
			endRow = availableRows
		}
		if endRow <= startRow {
			endRow = startRow + 1
		}
		if endRow > availableRows {
			endRow = availableRows
		}

		if startRow < nextFreeRow {
			blockHeight := endRow - startRow
			startRow = nextFreeRow
			endRow = startRow + blockHeight
			if endRow > availableRows {
				endRow = availableRows
			}
		}

		for r := startRow; r < endRow; r++ {
			if rowSlots[r].slotIdx != -1 && rowSlots[r].slotIdx != i {
				t.Errorf("Row %d claimed by slot %d, but slot %d also wants it — visual overlap!",
					r, rowSlots[r].slotIdx, i)
			}
			rowSlots[r] = rowInfo{slotIdx: i, isFirst: r == startRow}
		}
		nextFreeRow = endRow
	}

	t.Logf("Row assignment (minsPerRow=%d, availableRows=%d):", minsPerRow, availableRows)
	for r := 0; r < availableRows; r++ {
		rowMin := layout.DayStartMin + r*minsPerRow
		if rowSlots[r].slotIdx >= 0 {
			slot := layout.Slots[rowSlots[r].slotIdx]
			t.Logf("  Row %2d (%02d:%02d): slot %d (%s) first=%v",
				r, rowMin/60, rowMin%60, rowSlots[r].slotIdx, slot.Item.Task.Content, rowSlots[r].isFirst)
		} else {
			t.Logf("  Row %2d (%02d:%02d): empty", r, rowMin/60, rowMin%60)
		}
	}
}
