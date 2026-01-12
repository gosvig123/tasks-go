package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/krisitan/tasks-go/internal/storage"
	"github.com/krisitan/tasks-go/internal/task"
	"github.com/krisitan/tasks-go/internal/ui"
)

var store *storage.Storage

func main() {
	store = storage.DefaultStorage()
	store.EnsureDir()

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

	// Plain list (no TUI)
	case "lp", "list-plain":
		listPlain()

	// Add task
	case "a", "add":
		if len(args) == 0 {
			fmt.Println("Usage: tasks add 'task content'")
			os.Exit(1)
		}
		addTask(strings.Join(args, " "))

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
	// Reset today's list if needed
	currentList := store.GetCurrentList()
	if currentList == "today" {
		if added, _ := store.ResetTodayList(); added > 0 {
			fmt.Printf("📅 Auto-added %d due task(s) to today's list\n", added)
		}
	}

	if err := ui.RunTaskList(store); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func listPlain() {
	currentList := store.GetCurrentList()

	// Reset today's list if needed
	if currentList == "today" {
		store.ResetTodayList()
	}

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

		fmt.Printf("  %d/%d. %s %s%s%s\n", i+1, total, check, t.DisplayContent(), due, recur)
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
	re := regexp.MustCompile(`\s+\+(\d+)(r)?$`)
	if match := re.FindStringSubmatch(content); len(match) > 1 {
		dueOffset, _ = strconv.Atoi(match[1])
		if match[2] == "r" {
			recurDays = dueOffset
		}
		content = strings.TrimSpace(re.ReplaceAllString(content, ""))
		displayContent = content
		if recurDays > 0 {
			displayContent += fmt.Sprintf(" (every %dd)", recurDays)
		} else if dueOffset > 0 {
			displayContent += fmt.Sprintf(" (due in %d days)", dueOffset)
		}
	}

	list.AddContent(content, dueOffset, recurDays)

	if err := store.SaveList(list); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving list: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Added to '%s': %s\n", currentList, displayContent)
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

	// Delete in reverse order to maintain correct indices
	for i := len(indices) - 1; i >= 0; i-- {
		idx := indices[i]
		if idx < 0 || idx >= list.Len() {
			fmt.Printf("Index %d out of range\n", idx)
			continue
		}
		deleted := list.Delete(idx)
		if deleted != nil {
			fmt.Printf("✅ Deleted: %s\n", deleted.DisplayContent())
		}
	}

	if err := store.SaveList(list); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving list: %v\n", err)
		os.Exit(1)
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

	for _, idx := range indices {
		if idx < 0 || idx >= list.Len() {
			fmt.Printf("Index %d out of range\n", idx)
			continue
		}

		toggled := list.Toggle(idx)
		if toggled == nil {
			continue
		}

		status := "🔄 Reopened"
		if toggled.Completed {
			status = "✅ Completed"

			// Handle recurring tasks
			if toggled.RecurDays > 0 {
				nextTask := toggled.CreateNextRecurrence()
				if nextTask != nil {
					if toggled.Source != "" {
						// Add to source list
						sourceList, err := store.LoadList(toggled.Source)
						if err == nil {
							sourceList.Add(nextTask)
							store.SaveList(sourceList)
							fmt.Printf("   ↳ Next occurrence added to '%s'\n", toggled.Source)
						}
					} else {
						list.Add(nextTask)
						fmt.Println("   ↳ Next occurrence created")
					}
				}
			}

			// Sync to source if from today's list
			if currentList == "today" && toggled.Source != "" {
				syncToSource(toggled)
				fmt.Printf("   ↳ Synced to '%s'\n", toggled.Source)
			}
		}

		fmt.Printf("%s: %s\n", status, toggled.DisplayContent())
	}

	if err := store.SaveList(list); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving list: %v\n", err)
		os.Exit(1)
	}
}

func syncToSource(todayTask *task.Task) {
	sourceList, err := store.LoadList(todayTask.Source)
	if err != nil {
		return
	}

	for _, t := range sourceList.Tasks {
		if t.Content == todayTask.Content {
			t.Completed = todayTask.Completed
			break
		}
	}

	store.SaveList(sourceList)
}

func handleListCommand(args []string) {
	if len(args) == 0 {
		showLists()
		return
	}

	subcmd := args[0]
	subargs := args[1:]

	switch subcmd {
	case "show", "":
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
	validName := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !validName.MatchString(name) {
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
	validName := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !validName.MatchString(newName) {
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
	info, err := store.GetListInfo(currentList)
	if err != nil {
		fmt.Print(currentList)
		return
	}

	fmt.Printf("%s(%d/%d)", currentList, info.Completed, info.Total)
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

func showHelp() {
	currentList := store.GetCurrentList()
	fmt.Printf("Tasks Manager (current list: %s)\n\n", currentList)

	fmt.Println("Task Commands:")
	fmt.Println("  tasks l|view              - Interactive task list (TUI)")
	fmt.Println("  tasks lp|list-plain       - Plain text task list")
	fmt.Println("  tasks a|add 'task'        - Add task")
	fmt.Println("  tasks a|add 'task +N'     - Add task with due date N days from now")
	fmt.Println("  tasks a|add 'task +Nr'    - Add recurring task every N days")
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
	fmt.Println("Other:")
	fmt.Println("  tasks starship            - Output for Starship prompt")
	fmt.Println("  tasks help                - Show this help")
	fmt.Println()
	fmt.Println("Interactive Controls:")
	fmt.Println("  ↑/↓ or j/k  - Navigate")
	fmt.Println("  Enter/Space - Toggle task")
	fmt.Println("  a           - Add task")
	fmt.Println("  d           - Delete task")
	fmt.Println("  L           - Switch list")
	fmt.Println("  q/Esc       - Quit")
}
