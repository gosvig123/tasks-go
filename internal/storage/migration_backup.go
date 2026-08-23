package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/krisitan/tasks-go/internal/task"
)

func (s *Storage) persistMigration(lists map[string]*task.TaskList, report *MigrationReport) error {
	backup, err := s.backupTaskLists()
	if err != nil {
		return err
	}
	report.Backup = backup
	for _, name := range sortedMigrationNames(lists) {
		if err := s.saveMigratedList(lists[name]); err != nil {
			return fmt.Errorf("saving migrated list %s: %w", name, err)
		}
	}
	return nil
}

func (s *Storage) backupTaskLists() (string, error) {
	backup := fmt.Sprintf("%s-backup-%s", s.TasksDir, time.Now().Format("20060102-150405"))
	if err := os.MkdirAll(backup, 0700); err != nil {
		return "", fmt.Errorf("creating migration backup: %w", err)
	}
	entries, err := os.ReadDir(s.TasksDir)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		if err := copyFile(filepath.Join(s.TasksDir, entry.Name()), filepath.Join(backup, entry.Name())); err != nil {
			return "", err
		}
	}
	return backup, nil
}

func copyFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		output.Close()
		return err
	}
	return output.Close()
}
