package tui

import (
	"math"
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

func (m *Model) renderSplash() string {
	var lines []string

	progress := m.splashProgress
	if progress > 1.0 {
		progress = 1.0
	}

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
	if m.width < 80 {
		topDecor = "╭──────────────────────────────────────╮"
	}
	decorW := runewidth.StringWidth(topDecor)
	decorPad := (m.width - decorW) / 2
	if decorPad > 0 {
		topDecor = strings.Repeat(" ", decorPad) + topDecor
	}

	decorColor := colors[int(math.Mod(float64(m.splashStage), float64(len(colors))))]
	decorStyle := lipgloss.NewStyle().Foreground(decorColor)

	emptyLine := strings.Repeat(" ", m.width)
	vertPad := (m.height - 16) / 2
	if vertPad > 0 {
		for i := 0; i < vertPad; i++ {
			lines = append(lines, emptyLine)
		}
	}

	lines = append(lines, decorStyle.Render(topDecor))
	lines = append(lines, "")

	logoLines := int(float64(len(bannerLines)) * progress)
	if logoLines < 0 {
		logoLines = 0
	}
	if logoLines > len(bannerLines) {
		logoLines = len(bannerLines)
	}

	for i := 0; i < len(bannerLines); i++ {
		line := bannerLines[i]
		colorIdx := i * len(colors) / len(bannerLines)
		if colorIdx >= len(colors) {
			colorIdx = len(colors) - 1
		}

		var styledLine string
		if i < logoLines {
			style := lipgloss.NewStyle().Foreground(colors[colorIdx]).Bold(true)
			styledLine = style.Render(line)
		} else {
			dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#2D2D3A"))
			styledLine = dimStyle.Render(line)
		}

		lineW := runewidth.StringWidth(line)
		pad := (m.width - lineW) / 2
		if pad > 0 {
			styledLine = strings.Repeat(" ", pad) + styledLine
		}
		lines = append(lines, styledLine)
	}

	lines = append(lines, "")

	if progress >= 0.6 {
		verStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F472B6")).
			Bold(true)
		verW := runewidth.StringWidth(version)
		verPad := (m.width - verW) / 2
		verLine := version
		if verPad > 0 {
			verLine = strings.Repeat(" ", verPad) + version
		}
		lines = append(lines, verStyle.Render(verLine))
	} else {
		lines = append(lines, "")
	}

	lines = append(lines, "")

	if progress >= 0.75 {
		subStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#C4B5FD"))
		subW := runewidth.StringWidth(subTitleBig)
		subPad := (m.width - subW) / 2
		subLine := subTitleBig
		if subPad > 0 {
			subLine = strings.Repeat(" ", subPad) + subTitleBig
		}
		lines = append(lines, subStyle.Render(subLine))
	} else {
		lines = append(lines, "")
	}

	if progress >= 0.85 {
		tagStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#818CF8")).
			Italic(true)
		tagW := runewidth.StringWidth(tagline)
		tagPad := (m.width - tagW) / 2
		tagLine := tagline
		if tagPad > 0 {
			tagLine = strings.Repeat(" ", tagPad) + tagline
		}
		lines = append(lines, tagStyle.Render(tagLine))
	} else {
		lines = append(lines, "")
	}

	lines = append(lines, "")

	bottomDecor := "╰──────────────────────────────────────────────────────────╯"
	if m.width < 80 {
		bottomDecor = "╰──────────────────────────────────────╯"
	}
	bottomW := runewidth.StringWidth(bottomDecor)
	bottomPad := (m.width - bottomW) / 2
	if bottomPad > 0 {
		bottomDecor = strings.Repeat(" ", bottomPad) + bottomDecor
	}
	lines = append(lines, decorStyle.Render(bottomDecor))

	if progress >= 0.95 {
		pressKey := "按任意键开始..."
		keyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#60A5FA")).
			Italic(true)
		keyW := runewidth.StringWidth(pressKey)
		keyPad := (m.width - keyW) / 2
		if keyPad > 0 {
			pressKey = strings.Repeat(" ", keyPad) + pressKey
		}
		lines = append(lines, keyStyle.Render(pressKey))
	}

	return strings.Join(lines, "\n")
}

// RenderBannerPublic is the exported entry point for renderBanner, used by cmd/main.go
// to print the brand banner once before entering the Bubble Tea loop.
func RenderBannerPublic(width int) string {
	return renderBanner(width)
}