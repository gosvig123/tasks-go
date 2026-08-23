package storage

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/krisitan/tasks-go/internal/task"
)

type Storage struct {
	TasksDir        string
	LockFile        string
	CurrentListFile string
	LastResetFile   string
	LastSyncFile    string
	LocalStateFile  string
	resetDoneToday  bool
	syncDoneToday   bool
}

const completedTaskRetentionDays = 14

func DefaultStorage() *Storage {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not determine home directory: %v\n", err)
		os.Exit(1)
	}
	tasksDir := filepath.Join(home, "tasks-lists")
	return &Storage{
		TasksDir:        tasksDir,
		LockFile:        filepath.Join(tasksDir, ".tasks.lock"),
		CurrentListFile: filepath.Join(home, ".current-tasks-list"),
		LastResetFile:   filepath.Join(home, ".tasks-today-last-reset"),
		LastSyncFile:    filepath.Join(home, ".tasks-last-sync"),
		LocalStateFile:  filepath.Join(home, ".tasks-local-state.json"),
	}
}

// EnsureDir creates the tasks directory if it doesn't exist
func (s *Storage) EnsureDir() error {
	return os.MkdirAll(s.TasksDir, 0755)
}

// GetCurrentList returns the name of the current list
func (s *Storage) GetCurrentList() string {
	data, err := os.ReadFile(s.CurrentListFile)
	if err != nil {
		return "main"
	}
	return strings.TrimSpace(string(data))
}

// SetCurrentList sets the current list name
func (s *Storage) SetCurrentList(name string) error {
	return os.WriteFile(s.CurrentListFile, []byte(name), 0644)
}

// ListPath returns the full path to a list file
func (s *Storage) ListPath(name string) string {
	return filepath.Join(s.TasksDir, name+".md")
}

// LoadList loads a task list from disk.
// For the "today" list, references are resolved against source lists.
func (s *Storage) LoadList(name string) (*task.TaskList, error) {
	list, err := s.loadListRaw(name)
	if err != nil {
		return nil, err
	}

	if name == "today" {
		s.ResolveReferences(list)
	}

	return list, nil
}

// loadListRaw loads a task list without resolving references.
func (s *Storage) loadListRaw(name string) (*task.TaskList, error) {
	list := task.NewTaskList(name)
	data, err := os.ReadFile(s.ListPath(name))
	if err != nil {
		if os.IsNotExist(err) {
			return list, nil
		}
		return nil, err
	}
	list.LoadedRevision = revisionBytes(data)
	for _, item := range task.ParseLines(strings.Split(string(data), "\n")) {
		list.Add(item)
	}
	return list, nil
}

// ResolveReferences enriches reference tasks in today's list
// with metadata from their source lists.
func (s *Storage) ResolveReferences(todayList *task.TaskList) {
	// Group references by source list to avoid loading the same list multiple times
	sourceCache := make(map[string]*task.TaskList)

	for _, t := range todayList.Tasks {
		if !t.IsReference() {
			continue
		}

		sourceList, ok := sourceCache[t.Source]
		if !ok {
			loaded, err := s.loadListRaw(t.Source)
			if err != nil {
				continue
			}
			sourceCache[t.Source] = loaded
			sourceList = loaded
		}

		if sourceTask := findTaskByIdentity(sourceList.Tasks, t); sourceTask != nil {
			t.ResolveFrom(sourceTask)
		}
	}
}

// SyncCompletionToSource propagates completion status from a today reference
// task back to its source list. Syncs Completed, CompletedAt, and subtask
// completion. This is the single function for today→source completion sync;
// the reverse (source→today) is handled by ResolveFrom on next load.
func (s *Storage) SyncCompletionToSource(todayTask *task.Task) {
	if todayTask.Source == "" {
		return
	}

	sourceList, err := s.loadListRaw(todayTask.Source)
	if err != nil {
		return
	}

	sourceTask := findTaskByIdentity(sourceList.Tasks, todayTask)
	if sourceTask == nil {
		return
	}
	sourceTask.Completed = todayTask.Completed
	sourceTask.CompletedAt = todayTask.CompletedAt
	for _, todaySub := range todayTask.Subtasks {
		if sourceSub := findTaskByIdentity(sourceTask.Subtasks, todaySub); sourceSub != nil {
			sourceSub.Completed = todaySub.Completed
			sourceSub.CompletedAt = todaySub.CompletedAt
		}
	}
	if err := s.SaveList(sourceList); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving list %s: %v\n", sourceList.Name, err)
	}
}

// SyncSubtasksToSource propagates subtask additions from a today reference task
// back to its source task so both views share the same subtask structure.
func (s *Storage) SyncSubtasksToSource(todayTask *task.Task) {
	if todayTask.Source == "" {
		return
	}

	sourceList, err := s.loadListRaw(todayTask.Source)
	if err != nil {
		return
	}

	for _, t := range sourceList.Tasks {
		if !sameTask(t, todayTask) {
			continue
		}

		existing := make(map[string]*task.Task)
		for _, srcSub := range t.Subtasks {
			existing[identityKey(srcSub)] = srcSub
		}

		changed := false
		for _, todaySub := range todayTask.Subtasks {
			key := identityKey(todaySub)
			if _, ok := existing[key]; ok {
				continue
			}

			newSub := &task.Task{
				ID:          todaySub.ID,
				Content:     todaySub.Content,
				Description: todaySub.Description,
				Completed:   todaySub.Completed,
				DueDate:     todaySub.DueDate,
				CompletedAt: todaySub.CompletedAt,
				RecurDays:   todaySub.RecurDays,
				CreatedAt:   todaySub.CreatedAt,
				Estimate:    todaySub.Estimate,
				StartTime:   todaySub.StartTime,
				Parent:      t,
			}
			t.Subtasks = append(t.Subtasks, newSub)
			changed = true
		}

		if changed {
			if err := s.SaveList(sourceList); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving list %s: %v\n", sourceList.Name, err)
			}
		}
		break
	}
}

// SyncCompletionToToday propagates task state from a source list task
// to its matching reference on the today list. Besides completion, it also
// refreshes subtask structure and metadata via ResolveFrom.
func (s *Storage) SyncCompletionToToday(sourceListName string, sourceTask *task.Task) {
	todayList, err := s.loadListRaw("today")
	if err != nil {
		return
	}

	for _, t := range todayList.Tasks {
		if t.Source != sourceListName || !sameTask(t, sourceTask) {
			continue
		}
		t.ResolveFrom(sourceTask)
		if err := s.SaveList(todayList); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving list %s: %v\n", todayList.Name, err)
		}
		break
	}
}

// HandleRecurrence creates the next occurrence of a recurring task after completion.
// Returns the name of the list the new task was added to, or "" if no recurrence was needed.
// When the task has a Source, the new occurrence is added to the source list (and saved).
// Otherwise it is added to currentList (caller is responsible for saving).
func (s *Storage) HandleRecurrence(toggled *task.Task, currentList *task.TaskList) string {
	if toggled == nil || !toggled.Completed || toggled.RecurDays <= 0 {
		return ""
	}

	nextTask := toggled.CreateNextRecurrence()
	if nextTask == nil {
		return ""
	}

	if toggled.Source != "" {
		sourceList, err := s.LoadList(toggled.Source)
		if err != nil {
			return ""
		}

		if recurrenceExists(sourceList.Tasks, nextTask) {
			return ""
		}

		sourceList.Add(nextTask)
		if err := s.SaveList(sourceList); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving list %s: %v\n", sourceList.Name, err)
		}
		return toggled.Source
	}

	currentList.Add(nextTask)
	return currentList.Name
}

// SaveList saves a list atomically while preserving non-task Markdown.
func (s *Storage) SaveList(list *task.TaskList) error {
	if err := s.EnsureDir(); err != nil {
		return err
	}
	path := s.ListPath(list.Name)
	original, structured, err := readStructuredFile(path)
	if err != nil {
		return err
	}
	if structured {
		err = s.saveStructuredList(list, original)
	} else {
		content := strings.Join(migratedLines(list), "\n")
		if content != "" {
			content += "\n"
		}
		err = writeAtomicText(path, content)
	}
	if err != nil {
		return err
	}
	return s.refreshLoadedRevision(list)
}

// GetAllLists returns names of all available lists
func (s *Storage) GetAllLists() ([]string, error) {
	if err := s.EnsureDir(); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(s.TasksDir)
	if err != nil {
		return nil, err
	}

	var lists []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			name := strings.TrimSuffix(entry.Name(), ".md")
			lists = append(lists, name)
		}
	}

	sort.Strings(lists)
	return lists, nil
}

// CreateList creates a new empty list
func (s *Storage) CreateList(name string) error {
	if err := s.EnsureDir(); err != nil {
		return err
	}

	path := s.ListPath(name)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("list '%s' already exists", name)
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	return file.Close()
}

// DeleteList deletes a list file and removes any @source: references
// to it from today's list to prevent orphaned stubs.
func (s *Storage) DeleteList(name string) error {
	path := s.ListPath(name)
	if err := os.Remove(path); err != nil {
		return err
	}
	// Remove references to the deleted list from today's list
	todayList, err := s.loadListRaw("today")
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var kept []*task.Task
	for _, t := range todayList.Tasks {
		if t.Source != name {
			kept = append(kept, t)
		}
	}
	if len(kept) == len(todayList.Tasks) {
		return nil
	}
	todayList.Tasks = kept
	return s.SaveList(todayList)
}

// RenameList renames a list and updates any @source: references to it
// in today's list so existing stubs continue to resolve correctly.
func (s *Storage) RenameList(oldName, newName string) error {
	oldPath := s.ListPath(oldName)
	newPath := s.ListPath(newName)

	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("list '%s' already exists", newName)
	}

	if err := os.Rename(oldPath, newPath); err != nil {
		return err
	}

	// Update @source: references in today's list
	todayList, err := s.loadListRaw("today")
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	changed := false
	for _, t := range todayList.Tasks {
		if t.Source == oldName {
			t.Source = newName
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return s.SaveList(todayList)
}

// ListExists checks if a list exists
func (s *Storage) ListExists(name string) bool {
	path := s.ListPath(name)
	_, err := os.Stat(path)
	return err == nil
}

// ShouldResetToday checks if today's list should be reset
func (s *Storage) ShouldResetToday() bool {
	if s.resetDoneToday {
		return false
	}

	data, err := os.ReadFile(s.LastResetFile)
	if err != nil {
		return true
	}

	lastReset := strings.TrimSpace(string(data))
	today := time.Now().Format("2006-01-02")

	return lastReset != today
}

// MarkTodayReset marks that today's list has been reset
func (s *Storage) MarkTodayReset() error {
	today := time.Now().Format("2006-01-02")
	if err := os.WriteFile(s.LastResetFile, []byte(today), 0644); err != nil {
		return err
	}
	s.resetDoneToday = true
	return nil
}

// ShouldSyncToday checks if lists should sync to gist today
func (s *Storage) ShouldSyncToday() bool {
	if s.syncDoneToday {
		return false
	}

	data, err := os.ReadFile(s.LastSyncFile)
	if err != nil {
		return true
	}

	lastSync := strings.TrimSpace(string(data))
	today := time.Now().Format("2006-01-02")

	return lastSync != today
}

// MarkSyncDone marks that sync has been done today
func (s *Storage) MarkSyncDone() error {
	today := time.Now().Format("2006-01-02")
	if err := os.WriteFile(s.LastSyncFile, []byte(today), 0644); err != nil {
		return err
	}
	s.syncDoneToday = true
	return nil
}

func pruneCompletedTasksCutoff(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -completedTaskRetentionDays)
}

func shouldPruneCompletedTask(t *task.Task, cutoff time.Time) bool {
	if !t.Completed || t.CompletedAt == nil {
		return false
	}

	completedAt := time.Date(t.CompletedAt.Year(), t.CompletedAt.Month(), t.CompletedAt.Day(), 0, 0, 0, 0, cutoff.Location())
	return !completedAt.After(cutoff)
}

func pruneCompletedTasks(tasks []*task.Task, cutoff time.Time) ([]*task.Task, int) {
	kept := make([]*task.Task, 0, len(tasks))
	pruned := 0

	for _, t := range tasks {
		if shouldPruneCompletedTask(t, cutoff) {
			pruned++
			continue
		}
		kept = append(kept, t)
	}

	return kept, pruned
}

func (s *Storage) PruneCompletedTasks() (int, error) {
	return s.pruneCompletedTasksBefore(time.Now())
}

func (s *Storage) pruneCompletedTasksBefore(now time.Time) (int, error) {
	lists, err := s.GetAllLists()
	if err != nil {
		return 0, err
	}

	cutoff := pruneCompletedTasksCutoff(now)
	totalPruned := 0
	for _, listName := range lists {
		if listName == "today" {
			continue
		}

		list, err := s.loadListRaw(listName)
		if err != nil {
			continue
		}

		kept, pruned := pruneCompletedTasks(list.Tasks, cutoff)
		if pruned == 0 {
			continue
		}

		list.Tasks = kept
		if err := s.SaveList(list); err != nil {
			return totalPruned, fmt.Errorf("saving pruned list %s: %w", list.Name, err)
		}
		totalPruned += pruned
	}

	return totalPruned, nil
}

// ResetTodayList resets today's list: reference stubs are replaced with
// freshly-computed due tasks, but manually-added tasks (no @source:) that
// are still pending are carried over so they are not lost on daily reset.
func (s *Storage) ResetTodayList() (int, error) {
	if !s.ShouldResetToday() {
		return 0, nil
	}

	// Preserve pending manually-added tasks from the previous day
	todayList := task.NewTaskList("today")
	if existing, err := s.loadListRaw("today"); err == nil {
		for _, t := range existing.Tasks {
			if !t.IsReference() && !t.Completed {
				todayList.Add(t)
			}
		}
	}

	// Get all lists and find due tasks
	lists, err := s.GetAllLists()
	if err != nil {
		return 0, err
	}

	addedCount := 0
	for _, listName := range lists {
		if listName == "today" {
			continue
		}

		list, err := s.LoadList(listName)
		if err != nil {
			continue
		}

		for _, t := range list.GetDueTasks() {
			todayList.Add(task.NewReferenceStub(t, listName))
			addedCount++
		}
	}

	if err := s.SaveList(todayList); err != nil {
		return 0, err
	}

	if err := s.MarkTodayReset(); err != nil {
		return 0, err
	}

	return addedCount, nil
}

// ListInfo contains summary info about a list
type ListInfo struct {
	Name      string
	Total     int
	Completed int
	Pending   int
}

// GetListInfo returns info about a specific list
func (s *Storage) GetListInfo(name string) (*ListInfo, error) {
	list, err := s.LoadList(name)
	if err != nil {
		return nil, err
	}

	return &ListInfo{
		Name:      name,
		Total:     list.FlattenedCount(),
		Completed: list.CompletedCount(),
		Pending:   list.PendingCount(),
	}, nil
}

// GetListInfoFast returns basic counts without full parsing (for starship)
func (s *Storage) GetListInfoFast(name string) (completed, total int) {
	path := s.ListPath(name)
	file, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[x] ") || strings.HasPrefix(line, "[X] ") {
			completed++
			total++
		} else if strings.HasPrefix(line, "[ ] ") || strings.HasPrefix(line, "[] ") {
			total++
		}
	}
	return completed, total
}

// GetAllListInfo returns info about all lists
func (s *Storage) GetAllListInfo() ([]*ListInfo, error) {
	lists, err := s.GetAllLists()
	if err != nil {
		return nil, err
	}

	var infos []*ListInfo
	for _, name := range lists {
		info, err := s.GetListInfo(name)
		if err != nil {
			continue
		}
		infos = append(infos, info)
	}

	return infos, nil
}

// SaveTaskTracked saves tracked time to a task identified by list name and content.
// If source is non-empty, the tracked time is saved to the source list instead
// (for today-list references that should persist across daily resets).
func (s *Storage) SaveTaskTracked(listName, content, source string, tracked time.Duration) error {
	saveList := listName
	if source != "" {
		saveList = source
	}

	list, err := s.loadListRaw(saveList)
	if err != nil {
		return err
	}

	for _, t := range list.Tasks {
		if strings.TrimSpace(t.Content) == content {
			t.Tracked = &tracked
			break
		}
	}

	return s.SaveList(list)
}

// UpcomingTask holds a task and its source list name for display in the upcoming section
type UpcomingTask struct {
	Task     *task.Task
	ListName string
}

// LoadUpcomingTasks loads all tasks due within the given number of days from all lists (excluding today).
// Returns tasks sorted by due date, deduplicated against today's list.
func (s *Storage) LoadUpcomingTasks(days int) ([]UpcomingTask, error) {
	lists, err := s.GetAllLists()
	if err != nil {
		return nil, fmt.Errorf("loading lists: %w", err)
	}

	// Load today's list to deduplicate (upcoming should not include tasks already in today)
	todayList, _ := s.loadListRaw("today")
	todayContents := make(map[string]bool)
	if todayList != nil {
		for _, t := range todayList.Tasks {
			todayContents[t.Content] = true
		}
	}

	var upcoming []UpcomingTask
	for _, listName := range lists {
		if listName == "today" {
			continue
		}
		list, err := s.loadListRaw(listName)
		if err != nil {
			continue // skip lists that fail to load
		}
		for _, t := range list.GetUpcomingTasks(days) {
			// Skip if already in today's list (by content match)
			if todayContents[t.Content] {
				continue
			}
			upcoming = append(upcoming, UpcomingTask{Task: t, ListName: listName})
		}
	}

	// Sort all upcoming tasks by due date
	sort.Slice(upcoming, func(i, j int) bool {
		return upcoming[i].Task.DueDate.Before(*upcoming[j].Task.DueDate)
	})

	return upcoming, nil
}
