package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/krisitan/tasks-go/internal/gist"
	"github.com/krisitan/tasks-go/internal/storage"
	"github.com/krisitan/tasks-go/internal/ui"
)

type gistSyncClient interface {
	IsConfigured() bool
	Sync(tasksDir string) (*gist.Gist, error)
}

var (
	store *storage.Storage

	dueOffsetRe   = regexp.MustCompile(`\s+\+(\d+)(r)?$`)
	validListName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	newGistClient = func() gistSyncClient {
		return gist.NewClient()
	}
	pruneCompletedTasks = func() (int, error) {
		return store.PruneCompletedTasks()
	}
)

func main() {
	store = storage.DefaultStorage()
	store.EnsureDir()
	if runMachineCommand(os.Args[1:]) {
		return
	}

	// Perform daily maintenance (today list refresh, gist sync)
	dailyRefresh()

	if len(os.Args) < 2 {
		showHelp()
		return
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	// Interactive view
	case "l", "view":
		runInteractive()

	// List all tasks from all lists (interactive)
	case "la", "all":
		runAllInteractive()

	// Plain list (no TUI)
	case "lp", "list-plain":
		listPlain()

	// Add task
	case "a", "add":
		if len(args) == 0 {
			fmt.Println("Usage: tasks add 'task content'")
			os.Exit(1)
		}
		// Special case: "add today" populates today's list with due tasks
		if args[0] == "today" && len(args) == 1 {
			addToday()
		} else {
			addTask(strings.Join(args, " "))
		}

	// Delete task
	case "d", "delete":
		if len(args) == 0 {
			fmt.Println("Usage: tasks delete <index>")
			os.Exit(1)
		}
		deleteTask(args[0])

	// Toggle task
	case "t", "toggle":
		if len(args) == 0 {
			fmt.Println("Usage: tasks toggle <index>")
			os.Exit(1)
		}
		toggleTask(args[0])

	// Calendar timeline view
	case "cal":
		store.SetCurrentList("today")
		runInteractive()

	// List management
	case "list", "ls":
		handleListCommand(args)

	// Switch list (shorthand)
	case "switch", "sw":
		if len(args) == 0 {
			fmt.Println("Usage: tasks switch <list-name>")
			os.Exit(1)
		}
		switchList(args[0])

	// Starship output
	case "starship":
		starshipOutput()

	// Debug: show raw completion status
	case "debug":
		debugAllTasks()

	// Sync to GitHub Gist
	case "sync":
		handleSyncCommand(args)

	// Help
	case "help", "-h", "--help":
		showHelp()

	default:
		fmt.Printf("Unknown command: %s\n\n", cmd)
		showHelp()
		os.Exit(1)
	}
}

func runInteractive() {
	for {
		result, err := ui.RunTaskList(store)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if result == ui.ResultSwitchList {
			// User wants to switch list - run list switcher then continue
			selected, err := ui.RunListSwitcher(store)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			if selected == "" {
				// User cancelled list selection, exit
				return
			}
			// Continue to run task list for selected list
			continue
		}
		// Normal quit
		return
	}
}

func runAllInteractive() {
	if err := ui.RunAllTasks(store); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func listPlain() {
	currentList := store.GetCurrentList()
	list, err := store.LoadList(currentList)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading list: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("📝 Tasks in list '%s':\n\n", currentList)

	if list.Len() == 0 {
		fmt.Println("  No tasks yet. Use 'tasks add \"your task\"' to add some!")
		return
	}

	total := list.Len()
	tasks := list.SortedTasks()

	for i, t := range tasks {
		check := "[]"
		if t.Completed {
			check = "[x]"
		}

		due := ""
		if t.DueDate != nil {
			due = fmt.Sprintf(" (due: %s)", t.DueDate.Format("2006-01-02"))
		}

		recur := ""
		if t.RecurDays > 0 {
			recur = fmt.Sprintf(" 🔁%dd", t.RecurDays)
		}

		fmt.Printf("  %d/%d. %s %s%s%s\n", i+1, total, check, t.Content, due, recur)
	}
}

func addTask(content string) {
	currentList := store.GetCurrentList()
	list, err := store.LoadList(currentList)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading list: %v\n", err)
		os.Exit(1)
	}

	// Parse due date syntax: +N or +Nr
	dueOffset := 0
	recurDays := 0
	displayContent := content

	// Check for +Nr (recurring) or +N (due date)
	if match := dueOffsetRe.FindStringSubmatch(content); len(match) > 1 {
		dueOffset, _ = strconv.Atoi(match[1])
		if match[2] == "r" {
			recurDays = dueOffset
		}
		content = strings.TrimSpace(dueOffsetRe.ReplaceAllString(content, ""))
		displayContent = content
		if recurDays > 0 {
			displayContent += fmt.Sprintf(" (every %dd)", recurDays)
		} else if dueOffset > 0 {
			displayContent += fmt.Sprintf(" (due in %d days)", dueOffset)
		}
	}

	t := list.AddContent(content, "", dueOffset, recurDays)

	// Auto-assign today's date when adding to the today list with no due date
	if currentList == "today" {
		t.EnsureDueToday()
	}

	if err := store.SaveList(list); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving list: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Added to '%s': %s\n", currentList, displayContent)
}

func addToday() {
	// Show interactive picker for manual selection
	// (today's list is already refreshed by dailyRefresh on startup)
	pickerAdded, err := ui.RunAddToday(store)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// If tasks were added, switch to today list and open task view
	if pickerAdded > 0 {
		store.SetCurrentList("today")
		runInteractive()
	}
}

func deleteTask(indexStr string) {
	indices := parseIndices(indexStr)
	if len(indices) == 0 {
		fmt.Println("Invalid index")
		os.Exit(1)
	}

	currentList := store.GetCurrentList()
	list, err := store.LoadList(currentList)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading list: %v\n", err)
		os.Exit(1)
	}

	sorted := list.SortedTasks()
	targets := make([]storage.TaskTarget, 0, len(indices))
	for _, userIdx := range indices {
		si := userIdx - 1
		if si < 0 || si >= len(sorted) {
			fmt.Printf("Index %d out of range\n", userIdx)
			continue
		}
		targets = append(targets, storage.TaskTarget{
			ListName: currentList,
			Index:    list.OriginalIndex(sorted[si]),
			SubIndex: -1,
			Content:  sorted[si].Content,
		})
	}

	// Delete highest original index first to avoid shift.
	for i := 0; i < len(targets); i++ {
		for j := i + 1; j < len(targets); j++ {
			if targets[j].Index > targets[i].Index {
				targets[i], targets[j] = targets[j], targets[i]
			}
		}
	}
	for _, target := range targets {
		result, err := store.DeleteTask(target)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error deleting task: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Deleted: %s\n", result.Task.Content)
	}
}

func toggleTask(indexStr string) {
	indices := parseIndices(indexStr)
	if len(indices) == 0 {
		fmt.Println("Invalid index")
		os.Exit(1)
	}

	currentList := store.GetCurrentList()
	list, err := store.LoadList(currentList)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading list: %v\n", err)
		os.Exit(1)
	}

	sorted := list.SortedTasks()
	for _, userIdx := range indices {
		si := userIdx - 1
		if si < 0 || si >= len(sorted) {
			fmt.Printf("Index %d out of range\n", userIdx)
			continue
		}

		result, err := store.ToggleTask(storage.TaskTarget{
			ListName: currentList,
			Index:    list.OriginalIndex(sorted[si]),
			SubIndex: -1,
			Content:  sorted[si].Content,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error toggling task: %v\n", err)
			os.Exit(1)
		}

		status := "🔄 Reopened"
		if result.Task.Completed {
			status = "✅ Completed"
			printRecurrence(result.RecurrenceList, currentList)
		}
		if result.SyncedToSource != "" {
			fmt.Printf("   ↳ Synced to '%s'\n", result.SyncedToSource)
		}
		fmt.Printf("%s: %s\n", status, result.Task.Content)
	}
}

func printRecurrence(addedTo, currentList string) {
	if addedTo == "" {
		return
	}
	if addedTo == currentList {
		fmt.Println("   ↳ Next occurrence created")
		return
	}
	fmt.Printf("   ↳ Next occurrence added to '%s'\n", addedTo)
}

func handleListCommand(args []string) {
	if len(args) == 0 {
		// Interactive list selector, then open task view
		selected, err := ui.RunListSwitcher(store)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		// If a list was selected, open the task view
		if selected != "" {
			runInteractive()
		}
		return
	}

	subcmd := args[0]
	subargs := args[1:]

	switch subcmd {
	case "show", "p", "plain":
		showLists()

	case "a", "add", "create", "c":
		if len(subargs) == 0 {
			fmt.Println("Usage: tasks list add <name>")
			os.Exit(1)
		}
		createList(subargs[0])

	case "d", "delete", "del", "remove":
		if len(subargs) == 0 {
			fmt.Println("Usage: tasks list delete <name>")
			os.Exit(1)
		}
		deleteList(subargs[0])

	case "s", "switch", "sw":
		if len(subargs) == 0 {
			fmt.Println("Usage: tasks list switch <name>")
			os.Exit(1)
		}
		switchList(subargs[0])

	case "r", "rename", "mv":
		if len(subargs) < 2 {
			fmt.Println("Usage: tasks list rename <old-name> <new-name>")
			os.Exit(1)
		}
		renameList(subargs[0], subargs[1])

	default:
		// Check if it's a number for quick switch
		if _, err := strconv.Atoi(subcmd); err == nil {
			switchList(subcmd)
		} else {
			fmt.Printf("Unknown list command: %s\n", subcmd)
			os.Exit(1)
		}
	}
}

func showLists() {
	fmt.Println("📋 Available task lists:")
	fmt.Println()

	currentList := store.GetCurrentList()
	infos, err := store.GetAllListInfo()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(infos) == 0 {
		fmt.Println("  No task lists found. Create one with 'tasks list add <name>'")
		return
	}

	for i, info := range infos {
		marker := "  "
		current := ""
		if info.Name == currentList {
			marker = "→ "
			current = " (current)"
		}
		fmt.Printf("[%d] %s%s%s - %d pending, %d completed\n",
			i, marker, info.Name, current, info.Pending, info.Completed)
	}
}

func createList(name string) {
	// Validate name
	if !validListName.MatchString(name) {
		fmt.Println("❌ List name can only contain letters, numbers, hyphens, and underscores")
		os.Exit(1)
	}

	if err := store.CreateList(name); err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Created new task list '%s'\n", name)
}

func deleteList(name string) {
	// Check if it's an index
	if idx, err := strconv.Atoi(name); err == nil {
		lists, _ := store.GetAllLists()
		if idx >= 0 && idx < len(lists) {
			name = lists[idx]
		}
	}

	currentList := store.GetCurrentList()
	if name == currentList {
		fmt.Println("❌ Cannot delete the currently active list")
		fmt.Println("Switch to another list first")
		os.Exit(1)
	}

	if err := store.DeleteList(name); err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Deleted list '%s'\n", name)
}

func switchList(name string) {
	// Check if it's an index
	if idx, err := strconv.Atoi(name); err == nil {
		lists, _ := store.GetAllLists()
		if idx >= 0 && idx < len(lists) {
			name = lists[idx]
		} else {
			fmt.Printf("❌ Index %d out of range\n", idx)
			os.Exit(1)
		}
	}

	if !store.ListExists(name) {
		fmt.Printf("❌ List '%s' does not exist\n", name)
		os.Exit(1)
	}

	if err := store.SetCurrentList(name); err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Switched to list '%s'\n", name)
}

func renameList(oldName, newName string) {
	// Check if oldName is an index
	if idx, err := strconv.Atoi(oldName); err == nil {
		lists, _ := store.GetAllLists()
		if idx >= 0 && idx < len(lists) {
			oldName = lists[idx]
		}
	}

	// Validate new name
	if !validListName.MatchString(newName) {
		fmt.Println("❌ List name can only contain letters, numbers, hyphens, and underscores")
		os.Exit(1)
	}

	if err := store.RenameList(oldName, newName); err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		os.Exit(1)
	}

	// Update current list if renamed
	if store.GetCurrentList() == oldName {
		store.SetCurrentList(newName)
	}

	fmt.Printf("✅ Renamed list '%s' to '%s'\n", oldName, newName)
}

func starshipOutput() {
	currentList := store.GetCurrentList()
	completed, total := store.GetListInfoFast(currentList)
	fmt.Printf("%s(%d/%d)", currentList, completed, total)
}

func debugAllTasks() {
	lists, _ := store.GetAllLists()

	uncompleted := 0
	completed := 0

	for _, listName := range lists {
		list, err := store.LoadList(listName)
		if err != nil {
			continue
		}

		for _, t := range list.Tasks {
			if t.Completed {
				completed++
			} else {
				uncompleted++
			}
		}
	}

	fmt.Printf("=== DEBUG ===\n")
	fmt.Printf("Uncompleted: %d\n", uncompleted)
	fmt.Printf("Completed: %d\n", completed)
	fmt.Printf("Total: %d\n", uncompleted+completed)
	fmt.Println()

	// Show first 10 uncompleted
	fmt.Println("First 10 uncompleted:")
	count := 0
	for _, listName := range lists {
		list, _ := store.LoadList(listName)
		for _, t := range list.Tasks {
			if !t.Completed && count < 10 {
				fmt.Printf("  [%s] %s\n", listName, t.Content[:min(40, len(t.Content))])
				count++
			}
		}
	}
}

// dailyRefresh performs all daily maintenance tasks:
// - Resets the today list (clears and repopulates with due tasks)
// - Syncs all lists to gist if configured
func dailyRefresh() {
	// Always reset today's list on a new day
	if added, _ := store.ResetTodayList(); added > 0 {
		fmt.Printf("Auto-refreshed today's list with %d due task(s)\n", added)
	}

	// Sync to gist if configured and not synced today
	if store.ShouldSyncToday() {
		client := newGistClient()
		if client.IsConfigured() {
			if _, err := client.Sync(store.TasksDir); err == nil {
				store.MarkSyncDone()
				fmt.Println("Auto-synced lists to gist")
				if pruned, err := pruneCompletedTasks(); err == nil && pruned > 0 {
					fmt.Printf("Pruned %d completed task(s) from source lists\n", pruned)
				} else if err != nil {
					fmt.Fprintf(os.Stderr, "Error pruning completed tasks: %v\n", err)
				}
			}
		}
	}
}

func parseIndices(s string) []int {
	var indices []int

	// Handle ranges and comma-separated values
	parts := strings.Split(s, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)

		if strings.Contains(part, "-") {
			// Range: "1-5"
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) == 2 {
				start, err1 := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
				end, err2 := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
				if err1 == nil && err2 == nil {
					if start <= end {
						for i := start; i <= end; i++ {
							indices = append(indices, i)
						}
					} else {
						for i := start; i >= end; i-- {
							indices = append(indices, i)
						}
					}
				}
			}
		} else {
			// Single index
			if idx, err := strconv.Atoi(part); err == nil {
				indices = append(indices, idx)
			}
		}
	}

	return indices
}

func handleSyncCommand(args []string) {
	client := gist.NewClient()

	if len(args) == 0 {
		// Default: sync to gist
		if !client.IsConfigured() {
			fmt.Println("Gist sync not configured.")
			fmt.Println("Run 'tasks sync init <github-token>' to set up.")
			fmt.Println()
			fmt.Println("To create a token:")
			fmt.Println("  1. Go to https://github.com/settings/tokens")
			fmt.Println("  2. Generate new token (classic)")
			fmt.Println("  3. Select 'gist' scope only")
			os.Exit(1)
		}

		fmt.Println("Syncing tasks to GitHub Gist...")
		g, err := client.Sync(store.TasksDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Sync failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Synced to: %s\n", g.HTMLURL)
		return
	}

	subcmd := args[0]
	subargs := args[1:]

	switch subcmd {
	case "init":
		if len(subargs) == 0 {
			fmt.Println("Usage: tasks sync init <github-token>")
			fmt.Println()
			fmt.Println("To create a token:")
			fmt.Println("  1. Go to https://github.com/settings/tokens")
			fmt.Println("  2. Generate new token (classic)")
			fmt.Println("  3. Select 'gist' scope only")
			os.Exit(1)
		}

		token := subargs[0]
		fmt.Println("Initializing gist backup...")

		g, err := client.Init(token, store.TasksDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to initialize: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Gist backup initialized!")
		fmt.Printf("Gist URL: %s\n", g.HTMLURL)
		fmt.Println()
		fmt.Println("To set up daily sync at noon, run:")
		fmt.Println("  tasks sync schedule")

	case "status":
		if !client.IsConfigured() {
			fmt.Println("Gist sync not configured.")
			fmt.Println("Run 'tasks sync init <github-token>' to set up.")
			os.Exit(1)
		}

		g, err := client.Status()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get status: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Gist Sync Status")
		fmt.Println()
		fmt.Printf("  URL:     %s\n", g.HTMLURL)
		fmt.Printf("  Updated: %s\n", g.UpdatedAt)
		fmt.Printf("  Files:   %d\n", len(g.Files))

	case "schedule":
		setupSchedule()

	default:
		fmt.Printf("Unknown sync command: %s\n", subcmd)
		os.Exit(1)
	}
}

func setupSchedule() {
	home, _ := os.UserHomeDir()
	plistPath := home + "/Library/LaunchAgents/com.tasks-go.sync.plist"

	// Find the tasks binary
	tasksPath, err := os.Executable()
	if err != nil {
		// Fallback to common locations
		if _, err := os.Stat("/usr/local/bin/tasks"); err == nil {
			tasksPath = "/usr/local/bin/tasks"
		} else {
			tasksPath = home + "/go/bin/tasks"
		}
	}

	plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.tasks-go.sync</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>sync</string>
    </array>
    <key>StartCalendarInterval</key>
    <dict>
        <key>Hour</key>
        <integer>12</integer>
        <key>Minute</key>
        <integer>0</integer>
    </dict>
    <key>StandardOutPath</key>
    <string>%s/.tasks-sync.log</string>
    <key>StandardErrorPath</key>
    <string>%s/.tasks-sync.log</string>
</dict>
</plist>
`, tasksPath, home, home)

	// Ensure LaunchAgents directory exists
	os.MkdirAll(home+"/Library/LaunchAgents", 0755)

	if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create launch agent: %v\n", err)
		os.Exit(1)
	}

	// Unload if already loaded, then load
	os.Setenv("HOME", home)
	fmt.Println("Installing daily sync schedule (12:00 PM)...")

	// Try to unload first (ignore errors if not loaded)
	unloadCmd := fmt.Sprintf("launchctl unload '%s' 2>/dev/null; launchctl load '%s'", plistPath, plistPath)
	if err := runShellCommand(unloadCmd); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load launch agent: %v\n", err)
		fmt.Println()
		fmt.Println("Manual installation:")
		fmt.Printf("  launchctl load '%s'\n", plistPath)
		os.Exit(1)
	}

	fmt.Println("Daily sync scheduled!")
	fmt.Println()
	fmt.Printf("  Schedule: Every day at 12:00 PM (noon)\n")
	fmt.Printf("  Log file: %s/.tasks-sync.log\n", home)
	fmt.Println()
	fmt.Println("To uninstall:")
	fmt.Printf("  launchctl unload '%s'\n", plistPath)
	fmt.Printf("  rm '%s'\n", plistPath)
}

func runShellCommand(cmd string) error {
	c := exec.Command("sh", "-c", cmd)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func showHelp() {
	currentList := store.GetCurrentList()
	fmt.Printf("Tasks Manager (current list: %s)\n\n", currentList)

	fmt.Println("Task Commands:")
	fmt.Println("  tasks l|view              - Interactive task list (TUI)")
	fmt.Println("  tasks la|all              - Show all tasks from all lists")
	fmt.Println("  tasks lp|list-plain       - Plain text task list")
	fmt.Println("  tasks a|add 'task'        - Add task")
	fmt.Println("  tasks a|add 'task +N'     - Add task with due date N days from now")
	fmt.Println("  tasks a|add 'task +Nr'    - Add recurring task every N days")
	fmt.Println("  tasks add today           - Populate today's list with due tasks")
	fmt.Println("  tasks cal                 - Open today's timeline view")
	fmt.Println("  tasks d|delete <#>        - Delete task by index")
	fmt.Println("  tasks t|toggle <#>        - Toggle task completion")
	fmt.Println()
	fmt.Println("List Commands:")
	fmt.Println("  tasks list                - Show all lists")
	fmt.Println("  tasks list a|add <name>   - Create a new list")
	fmt.Println("  tasks list d|delete <#>   - Delete list")
	fmt.Println("  tasks list s|switch <#>   - Switch to list")
	fmt.Println("  tasks list r|rename <#> <new>  - Rename a list")
	fmt.Println()
	fmt.Println("Sync Commands:")
	fmt.Println("  tasks sync                - Sync tasks to GitHub Gist")
	fmt.Println("  tasks sync init <token>   - Initialize gist backup with GitHub token")
	fmt.Println("  tasks sync status         - Show sync status")
	fmt.Println("  tasks sync schedule       - Set up daily sync at 12:00 PM")
	fmt.Println()
	fmt.Println("Other:")
	fmt.Println("  tasks api lists          - List task lists as versioned JSON")
	fmt.Println("  tasks api snapshot --list today - Output versioned JSON")
	fmt.Println("  tasks api exec            - Apply one JSON mutation from stdin")
	fmt.Println("  tasks migrate ids [--dry-run] - Add stable task IDs")
	fmt.Println("  tasks starship            - Output for Starship prompt")
	fmt.Println("  tasks help                - Show this help")
	fmt.Println()
	fmt.Println("Interactive Controls:")
	fmt.Println("  ↑/↓ or j/k  - Navigate")
	fmt.Println("  Tab/Enter   - Toggle task")
	fmt.Println("  a           - Add task")
	fmt.Println("  d           - Delete task")
	fmt.Println("  /           - Search tasks")
	fmt.Println("  L           - Switch list")
	fmt.Println("  q/Esc       - Quit")
}
