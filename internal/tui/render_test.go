package tui

import (
	"strings"
	"testing"
)

func TestRenderUserMessage_ContainsMarkerAndText(t *testing.T) {
	got := renderUserMessage("你好")
	if !strings.Contains(got, "❯") {
		t.Fatalf("expected ❯ marker, got %q", got)
	}
	if !strings.Contains(got, "你好") {
		t.Fatalf("expected content, got %q", got)
	}
}

func TestRenderUserMessage_MultilineIndent(t *testing.T) {
	got := renderUserMessage("line1\nline2\nline3")
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), got)
	}
	if !strings.Contains(lines[0], "❯") {
		t.Fatalf("first line missing marker: %q", lines[0])
	}
	if strings.Contains(lines[1], "❯") {
		t.Fatalf("continuation line should not have marker: %q", lines[1])
	}
}

func TestRenderUserMessage_EmptyReturnsEmpty(t *testing.T) {
	if got := renderUserMessage(""); got != "" {
		t.Fatalf("expected empty for empty input, got %q", got)
	}
}
