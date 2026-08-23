package storage

import (
	"fmt"
	"sort"

	"github.com/krisitan/tasks-go/internal/task"
)

type MigrationReport struct {
	Assigned int    `json:"assigned"`
	Backup   string `json:"backup,omitempty"`
	DryRun   bool   `json:"dryRun"`
}

func (s *Storage) MigrateIDs(dryRun bool) (MigrationReport, error) {
	report := MigrationReport{DryRun: dryRun}
	if dryRun {
		err := s.prepareMigration(&report, false)
		return report, err
	}
	err := s.WithLock(func() error { return s.prepareMigration(&report, true) })
	return report, err
}

func (s *Storage) prepareMigration(report *MigrationReport, persist bool) error {
	lists, err := s.loadMigrationLists()
	if err != nil {
		return err
	}
	if err := migrateLists(lists, report); err != nil {
		return err
	}
	if !persist {
		return nil
	}
	return s.persistMigration(lists, report)
}

func (s *Storage) loadMigrationLists() (map[string]*task.TaskList, error) {
	names, err := s.GetAllLists()
	if err != nil {
		return nil, err
	}
	lists := make(map[string]*task.TaskList, len(names))
	for _, name := range names {
		list, loadErr := s.loadListRaw(name)
		if loadErr != nil {
			return nil, fmt.Errorf("loading %s: %w", name, loadErr)
		}
		lists[name] = list
	}
	return lists, nil
}

func migrateLists(lists map[string]*task.TaskList, report *MigrationReport) error {
	for name, list := range lists {
		if name != "today" {
			assignTaskIDs(list.Tasks, report)
		}
	}
	if today := lists["today"]; today != nil {
		if err := migrateToday(today, lists, report); err != nil {
			return err
		}
	}
	return validateCanonicalIDs(lists)
}

func assignTaskIDs(tasks []*task.Task, report *MigrationReport) {
	for _, item := range tasks {
		if item.EnsureID() {
			report.Assigned++
		}
		assignTaskIDs(item.Subtasks, report)
	}
}

func migrateToday(today *task.TaskList, lists map[string]*task.TaskList, report *MigrationReport) error {
	for _, item := range today.Tasks {
		if item.Source == "" {
			assignTaskIDs([]*task.Task{item}, report)
			continue
		}
		canonical, err := canonicalReference(lists[item.Source], item)
		if err != nil {
			return fmt.Errorf("today task %q: %w", item.Content, err)
		}
		item.ID = canonical.ID
		if err := migrateTodaySubtasks(item, canonical); err != nil {
			return err
		}
	}
	return nil
}

func migrateTodaySubtasks(reference, canonical *task.Task) error {
	for _, subtask := range reference.Subtasks {
		match, err := canonicalSubtask(canonical.Subtasks, subtask)
		if err != nil {
			return fmt.Errorf("subtask %q: %w", subtask.Content, err)
		}
		subtask.ID = match.ID
	}
	return nil
}

func validateCanonicalIDs(lists map[string]*task.TaskList) error {
	seen := make(map[string]string)
	for name, list := range lists {
		if name == "today" {
			continue
		}
		if err := validateTaskIDs(name, list.Tasks, seen); err != nil {
			return err
		}
	}
	return validateDirectToday(lists["today"], seen)
}

func validateDirectToday(today *task.TaskList, seen map[string]string) error {
	if today == nil {
		return nil
	}
	for _, item := range today.Tasks {
		if item.Source == "" {
			if err := validateTaskIDs("today", []*task.Task{item}, seen); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateTaskIDs(list string, tasks []*task.Task, seen map[string]string) error {
	for _, item := range tasks {
		if item.ID == "" {
			return fmt.Errorf("task %q in %s has no ID", item.Content, list)
		}
		if owner, exists := seen[item.ID]; exists {
			return fmt.Errorf("duplicate task ID %s in %s and %s", item.ID, owner, list)
		}
		seen[item.ID] = list
		if err := validateTaskIDs(list, item.Subtasks, seen); err != nil {
			return err
		}
	}
	return nil
}

func sortedMigrationNames(lists map[string]*task.TaskList) []string {
	names := make([]string, 0, len(lists))
	for name := range lists {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
