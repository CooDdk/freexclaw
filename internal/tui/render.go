package tui

import (
	"fmt"
	"sort"
	"strings"
)

// renderUserMessage renders user input as a single string suitable for tea.Println.
// The first line is prefixed with "❯ "; continuation lines are indented by 2 spaces.
func renderUserMessage(text string) string {
	text = strings.TrimRight(text, "\n")
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	var out []string
	prefix := MarkerUser() + " "
	for i, l := range lines {
		if i == 0 {
			out = append(out, prefix+l)
		} else {
			out = append(out, "  "+l)
		}
	}
	return strings.Join(out, "\n")
}

// renderToolCall renders one completed tool call as a fixed-format block.
// name: tool name (web_search / read_file / write_file / list_dir, etc.)
// args: argument map
// resultSummary: single- or multi-line result summary
// ok: success flag
// durationMS: elapsed time in milliseconds
func renderToolCall(name string, args map[string]any, resultSummary string, ok bool, durationMS int) string {
	argStr := formatToolArgs(args)
	var status string
	if ok {
		status = MarkerToolOK()
	} else {
		status = MarkerToolFail()
	}
	dur := formatToolDuration(durationMS)
	head := fmt.Sprintf("%s %s(%s)", MarkerToolStart(), name, argStr)
	tail := fmt.Sprintf("  %s %s", status, dur)
	if resultSummary != "" {
		tail = fmt.Sprintf("  %s %s\n%s", status, dur,
			indentLines(strings.TrimRight(resultSummary, "\n"), "    "))
	}
	return head + "\n" + tail
}

func formatToolArgs(args map[string]any) string {
	if len(args) == 0 {
		return ""
	}
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		v := args[k]
		switch vv := v.(type) {
		case string:
			parts = append(parts, fmt.Sprintf("%s=%q", k, vv))
		default:
			parts = append(parts, fmt.Sprintf("%s=%v", k, vv))
		}
	}
	return strings.Join(parts, ", ")
}

func formatToolDuration(ms int) string {
	if ms >= 1000 {
		return fmt.Sprintf("%.1fs", float64(ms)/1000.0)
	}
	return fmt.Sprintf("%dms", ms)
}

func indentLines(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}
// width is used for markdown wrapping.
func renderAssistantMessage(content string, width int) string {
	content = strings.TrimRight(content, "\n")
	if content == "" {
		return ""
	}
	rendered := renderMarkdown(content, width)
	rendered = strings.TrimRight(rendered, "\n")
	lines := strings.Split(rendered, "\n")
	out := make([]string, 0, len(lines))
	prefix := MarkerAssistant() + " "
	for i, l := range lines {
		if i == 0 {
			out = append(out, prefix+l)
		} else {
			out = append(out, "  "+l)
		}
	}
	return strings.Join(out, "\n")
}
