# Calendar Timeline View Design

## Overview

Add a timeline/calendar view to the today list, showing tasks laid out across the day with time estimates and optional pinned start times. The view is a split panel — task list on the left, timeline on the right — shown by default for the today list.

## Data Model

### New fields on Task

```go
type Task struct {
    // ... existing fields ...
    Estimate  *time.Duration  // how long the task takes
    StartTime *TimeOfDay      // optional pinned start time
}

type TimeOfDay struct {
    Hour   int  // 0-23
    Minute int  // 0-59
}
```

### New file tags

| Tag | Format | Example | Purpose |
|-----|--------|---------|---------|
| `@est` | `@est:<duration>` | `@est:2h`, `@est:30m`, `@est:1h30m` | Time estimate (duration) |
| `@at` | `@at:<HH:MM>` | `@at:09:00`, `@at:14:30` | Pinned start time (24h) |

### File format example

```
[] Write proposal @due:2026-02-08 @est:2h @at:09:00 @source:work || 2026-02-01
[] Read chapter 5 @due:2026-02-08 @est:2h || 2026-02-05
[] Buy groceries @due:2026-02-08 || 2026-02-06
```

### Three task tiers

- **Pinned**: has `@at` + `@est` — placed at exact time in timeline
- **Floating**: has `@est` only — stacked in gaps after pinned tasks, in list order
- **Unscheduled**: no `@est` — shown below the timeline

## Layout

Default today view is a split panel. Task list on left (50%), timeline on right (50%). Toggle with `c` to collapse timeline and show classic full-width list.

```
+--------- Task List (left, 50%) ----------+ +---------- Timeline (right, 50%) ---------+
| today (4 pending)                         | | Sun Feb 08          4.5h / 10h           |
|                                           | |--------------------------------------------|
|    Task                   Due             | | 08:00 |                                    |
| ---------------------------------         | | 09:00 | +----------------------------+    |
| > [ ] Write proposal    2026-02-08       | |       | | [ ] Write proposal   (2h)  |    |
|   [ ] Review PRs        2026-02-08       | | 10:00 | |                            |    |
|   [ ] Read chapter 5    2026-02-08       | |       | |                            |    |
|   [ ] Buy groceries                      | | 11:00 | +----------------------------+    |
|                                           | |       | +----------------------------+    |
|                                           | | 11:00 | | [ ] Review PRs      (30m)  |    |
|                                           | | 11:30 | +----------------------------+    |
|                                           | | 12:00 |                                    |
|                                           | | 13:00 | ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~   |
|                                           | |       | ~ [ ] Read ch. 5      (2h)  ~    |
|                                           | | 15:00 | ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~   |
|                                           | | 16:00 |                                    |
|                                           | | 17:00 |                                    |
|                                           | |--------------------------------------------|
|                                           | | Unscheduled:                               |
|                                           | |   [ ] Buy groceries (no estimate)          |
|                                           | |--------------------------------------------|
| up/down: nav  a: add  e: edit            | | J/K: reorder  s: set time  e: estimate     |
+-------------------------------------------+ +--------------------------------------------+
```

### Rendering rules

- Block height proportional to duration (1 hour ~ 2 terminal rows, adjustable to terminal height)
- Pinned blocks use solid border, floating blocks use dotted/shaded style
- Currently selected task highlighted in both panels simultaneously
- "Now" marker — horizontal line at current time (e.g. `-- 10:34 --`)
- Time range defaults to 08:00-18:00, auto-expands if a task is pinned outside that range
- Completed tasks shown as faint/dimmed in both panels

## Interactions

### Focus switching

| Key | Action |
|-----|--------|
| `h` or left arrow | Focus left panel (list) |
| `l` or right arrow | Focus right panel (timeline) |
| `c` | Toggle timeline panel on/off |

### Left panel (task list) — existing keybindings preserved

| Key | Action |
|-----|--------|
| `j/k`, up/down | Navigate tasks |
| `a` | Add task (form now has 6 fields) |
| `e` | Edit task |
| `enter` | View task details |
| `tab/space` | Toggle completion |
| `d` | Delete task |
| `/` | Search |

### Right panel (timeline)

| Key | Action |
|-----|--------|
| `j/k`, up/down | Move cursor between tasks in timeline order |
| `J/K` | Reorder: floating tasks change position, pinned tasks nudge +/-30m |
| `s` | Set start time (inline input, pins the task) |
| `S` | Unpin (remove start time, make floating) |
| `e` | Set/change estimate (inline input) |
| `tab/space` | Toggle completion |
| `d` | Delete task |

### Cursor sync

- Selecting a task in either panel highlights it in both
- Navigating in the list scrolls the timeline to that task
- Navigating in the timeline moves the list cursor to match

### Add/Edit form

Existing 4-field form gains 2 new fields (6 total, Tab to cycle):

```
> Task:  [Write proposal          ]
  Desc:  [Draft the Q1 proposal   ]
  Due:   [2026-02-08              ]
  Recur: [                        ]
  Est:   [2h                      ]
  Start: [09:00                   ]
```

## Timeline Layout Algorithm

```
1. Collect today's tasks
2. Separate into: pinned (@at + @est), floating (@est only), unscheduled (no @est)
3. Place pinned tasks at their @at times
4. Place floating tasks in gaps between pinned tasks, in list order
5. If floating tasks don't fit in gaps, stack them after the last pinned task
6. Unscheduled tasks go in a section below the timeline
```

### Overlap handling

Overlaps are allowed and shown visually (nested/offset blocks). The user can fix with `J/K` or `s`. No automatic conflict resolution.

## Config

New defaults in `internal/config/config.go`:

```go
DayStartHour:        8   // 08:00
DayEndHour:          18  // 18:00
TimelineSlotMinutes: 30  // granularity for J/K nudging pinned tasks
```

Auto-expand: if any task is pinned outside the default range, the timeline extends to include it.

## CLI

- `tasks cal` or `tasks t` — open today list in timeline view directly
- Existing `tasks l` on the today list shows the split view by default

## Implementation Scope

### Task model (`internal/task/task.go`)
- Add `Estimate *time.Duration` and `StartTime *TimeOfDay` fields
- Add `TimeOfDay` struct
- Add regex patterns for `@est:` and `@at:` tags
- Update `Parse()` and `String()` to handle new tags

### Task list (`internal/task/list.go`)
- No changes needed (operates on Task pointers)

### Config (`internal/config/config.go`)
- Add timeline config defaults

### UI (`internal/ui/tasklist.go`)
- Add timeline panel rendering
- Add focus management (left/right panels)
- Add timeline-specific keybindings (`J/K`, `s`, `S`, `e` in timeline context)
- Add cursor sync logic
- Update add/edit forms with 2 new fields
- Add `c` toggle for timeline visibility
- Add "now" marker rendering
- Add timeline layout algorithm (placing pinned, floating, unscheduled)

### CLI (`main.go`)
- Add `cal`/`t` command aliases
