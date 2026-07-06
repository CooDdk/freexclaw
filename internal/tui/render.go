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
