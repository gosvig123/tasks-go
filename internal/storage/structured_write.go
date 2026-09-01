package storage

import (
	"os"
	"strings"

	"github.com/krisitan/tasks-go/internal/task"
)

type serializedLine struct {
	key  string
	text string
	used bool
}

func (s *Storage) saveStructuredList(list *task.TaskList, original string) error {
	entries := serializedEntries(list)
	lines := strings.Split(original, "\n")
	result := make([]string, 0, len(lines)+len(entries))
	for _, line := range lines {
		parsed, err := task.Parse(strings.TrimSpace(line))
		if err != nil {
			result = append(result, line)
			continue
		}
		if replacement := consumeEntry(entries, parsed); replacement != "" {
			result = append(result, replacement)
		}
	}
	result = appendRemaining(result, entries)
	return writeAtomicText(s.ListPath(list.Name), strings.Join(result, "\n"))
}

func serializedEntries(list *task.TaskList) []*serializedLine {
	entries := make([]*serializedLine, 0, len(list.Tasks))
	for _, item := range list.Tasks {
		entries = append(entries, &serializedLine{key: documentKey(item), text: item.String()})
	}
	return entries
}

func consumeEntry(entries []*serializedLine, original *task.Task) string {
	key := documentKey(original)
	for _, entry := range entries {
		if !entry.used && entry.key == key {
			entry.used = true
			return entry.text
		}
	}
	return ""
}

func appendRemaining(lines []string, entries []*serializedLine) []string {
	for _, entry := range entries {
		if entry.used {
			continue
		}
		lines = append(lines, entry.text)
		entry.used = true
	}
	return lines
}

func documentKey(item *task.Task) string {
	if item.ID != "" {
		return "id:" + item.ID
	}
	return "content:" + strings.TrimSpace(item.Content)
}

func readStructuredFile(path string) (string, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	content := string(data)
	return content, hasNonTaskLines(content), nil
}

func hasNonTaskLines(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if _, err := task.Parse(strings.TrimSpace(line)); err != nil {
			return true
		}
	}
	return false
}
