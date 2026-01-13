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
	CurrentListFile string
	LastResetFile   string
}

func New(tasksDir, currentListFile, lastResetFile string) *Storage {
	return &Storage{
		TasksDir:        tasksDir,
		CurrentListFile: currentListFile,
		LastResetFile:   lastResetFile,
	}
}

func DefaultStorage() *Storage {
	home, _ := os.UserHomeDir()
	return &Storage{
		TasksDir:        filepath.Join(home, "tasks-lists"),
		CurrentListFile: filepath.Join(home, ".current-tasks-list"),
		LastResetFile:   filepath.Join(home, ".tasks-today-last-reset"),
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

// LoadList loads a task list from disk
func (s *Storage) LoadList(name string) (*task.TaskList, error) {
	list := task.NewTaskList(name)
	path := s.ListPath(name)

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return list, nil // Return empty list if file doesn't exist
		}
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if t, err := task.Parse(line); err == nil {
			list.Add(t)
		}
	}

	return list, scanner.Err()
}

// SaveList saves a task list to disk
func (s *Storage) SaveList(list *task.TaskList) error {
	if err := s.EnsureDir(); err != nil {
		return err
	}

	path := s.ListPath(list.Name)
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, t := range list.Tasks {
		fmt.Fprintln(file, t.String())
	}

	return nil
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

// DeleteList deletes a list file
func (s *Storage) DeleteList(name string) error {
	path := s.ListPath(name)
	return os.Remove(path)
}

// RenameList renames a list
func (s *Storage) RenameList(oldName, newName string) error {
	oldPath := s.ListPath(oldName)
	newPath := s.ListPath(newName)

	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("list '%s' already exists", newName)
	}

	return os.Rename(oldPath, newPath)
}

// ListExists checks if a list exists
func (s *Storage) ListExists(name string) bool {
	path := s.ListPath(name)
	_, err := os.Stat(path)
	return err == nil
}

// ShouldResetToday checks if today's list should be reset
func (s *Storage) ShouldResetToday() bool {
	data, err := os.ReadFile(s.LastResetFile)
	if err != nil {
		return true // If file doesn't exist, reset is needed
	}

	lastReset := strings.TrimSpace(string(data))
	today := time.Now().Format("2006-01-02")

	return lastReset != today
}

// MarkTodayReset marks that today's list has been reset
func (s *Storage) MarkTodayReset() error {
	today := time.Now().Format("2006-01-02")
	return os.WriteFile(s.LastResetFile, []byte(today), 0644)
}

// ResetTodayList clears today's list and populates with due tasks
func (s *Storage) ResetTodayList() (int, error) {
	if !s.ShouldResetToday() {
		return 0, nil
	}

	// Clear today's list
	todayList := task.NewTaskList("today")

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
			// Create a copy with source reference
			todayTask := &task.Task{
				Content:   t.Content,
				Completed: false,
				DueDate:   t.DueDate,
				RecurDays: t.RecurDays,
				Source:    listName,
			}
			todayList.Add(todayTask)
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
		Total:     list.Len(),
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
		line := scanner.Text()
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
