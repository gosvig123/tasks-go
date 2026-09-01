package task

import (
	"io"
	"os"
	"testing"
)

func TestParseLinesIgnoresNonTaskMarkdown(t *testing.T) {
	lines := []string{
		"# Goals",
		"Source: gmail:personal:thread:123",
		"Related source: gmail:work:thread:456",
		"2026-08-22.",
		"[] Valid task",
	}
	var tasks []*Task
	stderr := captureStderr(t, func() { tasks = ParseLines(lines) })
	if len(tasks) != 1 || tasks[0].Content != "Valid task" {
		t.Fatalf("expected one valid task, got %#v", tasks)
	}
	if stderr != "" {
		t.Fatalf("expected Markdown to be ignored quietly, got %q", stderr)
	}
}

func TestParseLinesWarnsForMalformedTaskMarker(t *testing.T) {
	var tasks []*Task
	stderr := captureStderr(t, func() {
		tasks = ParseLines([]string{"[]Missing task separator"})
	})
	if len(tasks) != 0 {
		t.Fatalf("expected malformed task to be skipped, got %#v", tasks)
	}
	if stderr == "" {
		t.Fatal("expected malformed task marker warning")
	}
}

func captureStderr(t *testing.T, run func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	original := os.Stderr
	os.Stderr = writer
	defer func() { os.Stderr = original }()
	run()
	os.Stderr = original
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
