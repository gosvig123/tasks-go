package task

import (
	"strings"
	"testing"
)

func TestDescriptionStorageEscapes(t *testing.T) {
	parsed, err := Parse(`[] Brief @desc:"Morning\n- Outcome\r\n- Review"`)
	if err != nil {
		t.Fatalf("parse description: %v", err)
	}
	want := "Morning\n- Outcome\n- Review"
	if parsed.Description != want {
		t.Fatalf("description = %q, want %q", parsed.Description, want)
	}
}

func TestDescriptionRoundTrip(t *testing.T) {
	want := "First line\nSecond line with \\\\ and \"quotes\""
	serialized := (&Task{Content: "Brief", Description: want}).String()
	if strings.Contains(serialized, "First line\nSecond line") {
		t.Fatalf("serialized description contains a physical newline: %q", serialized)
	}
	parsed, err := Parse(serialized)
	if err != nil {
		t.Fatalf("parse serialized description: %v", err)
	}
	if parsed.Description != want {
		t.Fatalf("round-trip description = %q, want %q", parsed.Description, want)
	}
}

func TestDescriptionPreservesDoubleEscapedNewline(t *testing.T) {
	parsed, err := Parse(`[] Example @desc:"Use \\n in source"`)
	if err != nil {
		t.Fatalf("parse description: %v", err)
	}
	if parsed.Description != `Use \n in source` {
		t.Fatalf("description = %q, want a literal \\n", parsed.Description)
	}
}
