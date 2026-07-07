package tui

import (
	"strings"
	"testing"

	"github.com/CooDdk/freexclaw/internal/config"
	"github.com/CooDdk/freexclaw/internal/conversation"
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
	// A leading blank line is prepended for visual separation between messages.
	got = strings.TrimPrefix(got, "\n")
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

func TestRenderUserMessage_HasLeadingBlankLineForSpacing(t *testing.T) {
	got := renderUserMessage("hi")
	if !strings.HasPrefix(got, "\n") {
		t.Fatalf("expected leading blank line for message spacing, got %q", got)
	}
}

func TestRenderUserMessage_EmptyReturnsEmpty(t *testing.T) {
	if got := renderUserMessage(""); got != "" {
		t.Fatalf("expected empty for empty input, got %q", got)
	}
}

func TestRenderAssistantMessage_ContainsMarker(t *testing.T) {
	got := renderAssistantMessage("Hello **world**", 80)
	if !strings.Contains(got, "✻") {
		t.Fatalf("expected ✻ marker, got %q", got)
	}
}

func TestRenderAssistantMessage_EmptyReturnsEmpty(t *testing.T) {
	if got := renderAssistantMessage("", 80); got != "" {
		t.Fatalf("expected empty for empty input, got %q", got)
	}
}

func TestRenderAssistantMessage_MultilineHasMarkerOnlyFirstLine(t *testing.T) {
	got := renderAssistantMessage("line1\nline2", 80)
	lines := strings.Split(got, "\n")
	markerCount := 0
	for _, l := range lines {
		if strings.Contains(l, "✻") {
			markerCount++
		}
	}
	if markerCount != 1 {
		t.Fatalf("expected marker on exactly 1 line, got %d in %q", markerCount, got)
	}
}

func TestRenderToolCall_SuccessHasStartAndOK(t *testing.T) {
	got := renderToolCall("web_search",
		map[string]interface{}{"query": "武汉 天气"},
		"3 个结果",
		true, 800)
	if !strings.Contains(got, "▸") {
		t.Fatalf("missing ▸: %q", got)
	}
	if !strings.Contains(got, "✓") {
		t.Fatalf("missing ✓: %q", got)
	}
	if !strings.Contains(got, "web_search") {
		t.Fatalf("missing tool name: %q", got)
	}
	if !strings.Contains(got, "武汉") {
		t.Fatalf("missing arg: %q", got)
	}
	if !strings.Contains(got, "0.8s") && !strings.Contains(got, "800ms") {
		t.Fatalf("missing duration: %q", got)
	}
}

func TestRenderToolCall_FailureHasCross(t *testing.T) {
	got := renderToolCall("web_search",
		map[string]interface{}{"query": "x"},
		"错误: 网络超时",
		false, 1200)
	if !strings.Contains(got, "✗") {
		t.Fatalf("expected ✗ marker on failure, got %q", got)
	}
	if !strings.Contains(got, "网络超时") {
		t.Fatalf("expected error text, got %q", got)
	}
}

func TestFormatToolArgs_NonStringNotQuoted(t *testing.T) {
	got := formatToolArgs(map[string]any{
		"recursive": true,
		"count":     42,
		"path":      ".",
	})
	// keys are sorted alphabetically
	if !strings.Contains(got, "count=42") {
		t.Fatalf("expected count=42 (unquoted), got %q", got)
	}
	if !strings.Contains(got, "recursive=true") {
		t.Fatalf("expected recursive=true (unquoted), got %q", got)
	}
	if !strings.Contains(got, `path="."`) {
		t.Fatalf("expected path=\".\" (quoted), got %q", got)
	}
}

func TestFormatToolDuration_BoundaryAt1000(t *testing.T) {
	if got := formatToolDuration(999); got != "999ms" {
		t.Fatalf("999ms expected, got %q", got)
	}
	if got := formatToolDuration(1000); got != "1.0s" {
		t.Fatalf("1.0s expected, got %q", got)
	}
}

func TestRenderStatusBarInline_ContainsBrand(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	m := &Model{
		cfg:     &config.Config{Model: "test-model"},
		convMgr: conversation.NewManager(root),
		width:   120,
	}
	defer m.convMgr.Close()

	bar := m.renderStatusBarInline()
	if !strings.Contains(bar, "FreeX Claw") {
		t.Fatalf("missing brand: %q", bar)
	}
	if !strings.Contains(bar, "test-model") {
		t.Fatalf("missing model name: %q", bar)
	}
}

func TestRenderMarkdown_CollapsesConsecutiveBlankLines(t *testing.T) {
	// Heading margins + block trailing "" + source-level blank line used to
	// stack up to 3 blank lines between sections. Verify the result never
	// contains more than one blank line in a row.
	src := "## 标题一\n\n- 条目 A\n- 条目 B\n\n## 标题二\n\n段落文本"
	rendered := renderMarkdown(src, 80)
	if strings.Contains(rendered, "\n\n\n") {
		t.Fatalf("expected no runs of 3+ newlines, got:\n%q", rendered)
	}
}

func TestRenderMarkdown_NoLeadingOrTrailingBlank(t *testing.T) {
	rendered := renderMarkdown("## 标题\n\n段落", 80)
	if strings.HasPrefix(rendered, "\n") {
		t.Fatalf("expected no leading blank, got: %q", rendered)
	}
	if strings.HasSuffix(rendered, "\n") {
		t.Fatalf("expected no trailing blank, got: %q", rendered)
	}
}

func TestRenderInputDividerInline_ContainsLabel(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	m := &Model{
		cfg:     &config.Config{Model: "test"},
		convMgr: conversation.NewManager(root),
		width:   80,
	}
	defer m.convMgr.Close()

	div := m.renderInputDividerInline()
	if !strings.Contains(div, "✎ 输入") {
		t.Fatalf("expected 输入 label in divider, got %q", div)
	}
	if !strings.Contains(div, "─") {
		t.Fatalf("expected rule glyph in divider, got %q", div)
	}
	if !strings.Contains(div, "╭") {
		t.Fatalf("expected corner glyph in divider, got %q", div)
	}
}

func TestRenderInputDividerInline_NarrowTerminalFallsBackToPlainRule(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	m := &Model{
		cfg:     &config.Config{Model: "test"},
		convMgr: conversation.NewManager(root),
		width:   6,
	}
	defer m.convMgr.Close()

	div := m.renderInputDividerInline()
	if strings.Contains(div, "✎") {
		t.Fatalf("expected label to be dropped in narrow terminal, got %q", div)
	}
	if !strings.Contains(div, "─") {
		t.Fatalf("expected plain rule glyphs, got %q", div)
	}
}
