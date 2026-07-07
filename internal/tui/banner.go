package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

const (
	bannerBig = `
███████╗██████╗ ███████╗███████╗██╗  ██╗ ██████╗██╗      █████╗ ██╗    ██╗
██╔════╝██╔══██╗██╔════╝██╔════╝╚██╗██╔╝██╔════╝██║     ██╔══██╗██║    ██║
█████╗  ██████╔╝█████╗  █████╗   ╚███╔╝ ██║     ██║     ███████║██║ █╗ ██║
██╔══╝  ██╔══██╗██╔══╝  ██╔══╝   ██╔██╗ ██║     ██║     ██╔══██║██║███╗██║
██║     ██║  ██║███████╗███████╗██╔╝ ██╗╚██████╗███████╗██║  ██║╚███╔███╔╝
╚═╝     ╚═╝  ╚═╝╚══════╝╚══════╝╚═╝  ╚═╝ ╚═════╝╚══════╝╚═╝  ╚═╝ ╚══╝╚══╝`

	subTitleBig = "Terminal AI Programming Assistant"
	version     = "v0.1.3"
)

func renderBanner(width int) string {
	var lines []string

	colors := []lipgloss.Color{
		lipgloss.Color("#A78BFA"),
		lipgloss.Color("#8B5CF6"),
		lipgloss.Color("#7C3AED"),
		lipgloss.Color("#6D28D9"),
		lipgloss.Color("#7C3AED"),
		lipgloss.Color("#8B5CF6"),
		lipgloss.Color("#A78BFA"),
	}

	bannerLines := strings.Split(strings.TrimLeft(bannerBig, "\n"), "\n")

	// Frame width tracks the widest banner line so the ╭─╮ / ╰─╯ rules
	// visually enclose the ASCII art rather than floating above/below it.
	bannerW := 0
	for _, l := range bannerLines {
		if w := runewidth.StringWidth(l); w > bannerW {
			bannerW = w
		}
	}
	frameW := bannerW
	if frameW > width-2 && width > 12 {
		frameW = width - 2
	}
	if frameW < 12 {
		frameW = 12
	}

	decorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6D28D9"))
	centerPad := func(s string) string {
		w := runewidth.StringWidth(s)
		pad := (width - w) / 2
		if pad > 0 {
			return strings.Repeat(" ", pad) + s
		}
		return s
	}

	topDecor := "╭" + strings.Repeat("─", frameW-2) + "╮"
	lines = append(lines, decorStyle.Render(centerPad(topDecor)))
	lines = append(lines, "")

	for i, line := range bannerLines {
		colorIdx := i * len(colors) / len(bannerLines)
		if colorIdx >= len(colors) {
			colorIdx = len(colors) - 1
		}
		style := lipgloss.NewStyle().Foreground(colors[colorIdx]).Bold(true)
		lines = append(lines, style.Render(centerPad(line)))
	}

	lines = append(lines, "")

	accentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F472B6")).
		Bold(true)
	verLine := "── freex claw · " + version + " ──"
	lines = append(lines, accentStyle.Render(centerPad(verLine)))

	lines = append(lines, "")

	subStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#C4B5FD"))
	lines = append(lines, subStyle.Render(centerPad(subTitleBig)))

	lines = append(lines, "")

	bottomDecor := "╰" + strings.Repeat("─", frameW-2) + "╯"
	lines = append(lines, decorStyle.Render(centerPad(bottomDecor)))

	return strings.Join(lines, "\n")
}

// RenderBannerPublic is the exported entry point for renderBanner, used by cmd/main.go
// to print the brand banner once before entering the Bubble Tea loop.
func RenderBannerPublic(width int) string {
	return renderBanner(width)
}