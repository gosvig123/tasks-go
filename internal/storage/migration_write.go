package storage

import (
	"fmt"
	"os"
	"strings"

	"github.com/krisitan/tasks-go/internal/task"
)

func (s *Storage) saveMigratedList(list *task.TaskList) error {
	data, err := os.ReadFile(s.ListPath(list.Name))
	if err != nil {
		return err
	}
	merged, err := mergeMigrationLines(strings.Split(string(data), "\n"), migratedLines(list))
	if err != nil {
		return err
	}
	return writeAtomicText(s.ListPath(list.Name), strings.Join(merged, "\n"))
}

func migratedLines(list *task.TaskList) []string {
	lines := make([]string, 0)
	for _, item := range list.Tasks {
		lines = append(lines, strings.Split(item.String(), "\n")...)
	}
	return lines
}

func mergeMigrationLines(original, migrated []string) ([]string, error) {
	result := append([]string(nil), original...)
	index := 0
	for line, value := range original {
		if _, err := task.Parse(strings.TrimSpace(value)); err != nil {
			continue
		}
		if index >= len(migrated) {
			return nil, fmt.Errorf("migration task count changed")
		}
		result[line] = migrated[index]
		index++
	}
	if index != len(migrated) {
		return nil, fmt.Errorf("migration expected %d task lines, replaced %d", len(migrated), index)
	}
	return result, nil
}

func writeAtomicText(path, content string) error {
	temporary := path + ".tmp"
	file, err := os.Create(temporary)
	if err != nil {
		return err
	}
	if _, err := file.WriteString(content); err != nil {
		file.Close()
		os.Remove(temporary)
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		os.Remove(temporary)
		return err
	}
	if err := file.Close(); err != nil {
		os.Remove(temporary)
		return err
	}
	return os.Rename(temporary, path)
}
