package tui

import "strings"

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

// renderAssistantMessage renders an AI reply as marker + markdown content.
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
