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

	subTitleBig = " Terminal AI Programming Assistant"
	tagline     = "  智能终端 AI 助手 · 聊天 · 编程 · 搜索 · 写文档"
	version     = "v0.1.0"
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

	topDecor := "╭──────────────────────────────────────────────────────────╮"
	if width < 80 {
		topDecor = "╭──────────────────────────────────────╮"
	}
	decorW := runewidth.StringWidth(topDecor)
	decorPad := (width - decorW) / 2
	if decorPad > 0 {
		topDecor = strings.Repeat(" ", decorPad) + topDecor
	}

	decorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6D28D9"))
	lines = append(lines, decorStyle.Render(topDecor))
	lines = append(lines, "")

	for i, line := range bannerLines {
		colorIdx := i * len(colors) / len(bannerLines)
		if colorIdx >= len(colors) {
			colorIdx = len(colors) - 1
		}
		style := lipgloss.NewStyle().Foreground(colors[colorIdx]).Bold(true)
		lineW := runewidth.StringWidth(line)
		pad := (width - lineW) / 2
		if pad > 0 {
			line = strings.Repeat(" ", pad) + line
		}
		lines = append(lines, style.Render(line))
	}

	lines = append(lines, "")

	accentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F472B6")).
		Bold(true)
	verW := runewidth.StringWidth(version)
	verPad := (width - verW) / 2
	verLine := version
	if verPad > 0 {
		verLine = strings.Repeat(" ", verPad) + version
	}
	lines = append(lines, accentStyle.Render(verLine))

	lines = append(lines, "")

	subStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#C4B5FD"))
	subW := runewidth.StringWidth(subTitleBig)
	subPad := (width - subW) / 2
	subLine := subTitleBig
	if subPad > 0 {
		subLine = strings.Repeat(" ", subPad) + subTitleBig
	}
	lines = append(lines, subStyle.Render(subLine))

	tagStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#818CF8")).
		Italic(true)
	tagW := runewidth.StringWidth(tagline)
	tagPad := (width - tagW) / 2
	tagLine := tagline
	if tagPad > 0 {
		tagLine = strings.Repeat(" ", tagPad) + tagline
	}
	lines = append(lines, tagStyle.Render(tagLine))

	lines = append(lines, "")

	bottomDecor := "╰──────────────────────────────────────────────────────────╯"
	if width < 80 {
		bottomDecor = "╰──────────────────────────────────────╯"
	}
	bottomW := runewidth.StringWidth(bottomDecor)
	bottomPad := (width - bottomW) / 2
	if bottomPad > 0 {
		bottomDecor = strings.Repeat(" ", bottomPad) + bottomDecor
	}
	lines = append(lines, decorStyle.Render(bottomDecor))

	return strings.Join(lines, "\n")
}

// RenderBannerPublic is the exported entry point for renderBanner, used by cmd/main.go
// to print the brand banner once before entering the Bubble Tea loop.
func RenderBannerPublic(width int) string {
	return renderBanner(width)
}