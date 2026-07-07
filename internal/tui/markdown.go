package tui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

var (
	tableRowRegex        = regexp.MustCompile(`^\s*\|(.+)\|\s*$`)
	tableSeparatorRegex  = regexp.MustCompile(`^\s*\|?\s*:?-+:?\s*(\|\s*:?-+:?\s*)+\|?\s*$`)
	codeBlockStartRegex  = regexp.MustCompile("^```(\\w*)\\s*$")
	codeBlockEndRegex    = regexp.MustCompile("^```\\s*$")
	headingRegex         = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)
	unorderedListRegex   = regexp.MustCompile(`^(\s*)[-*+]\s+(.+)$`)
	orderedListRegex     = regexp.MustCompile(`^(\s*)(\d+)\.\s+(.+)$`)
	blockquoteRegex      = regexp.MustCompile(`^>\s?(.*)$`)
	hrRegex              = regexp.MustCompile(`^(\*{3,}|-{3,}|_{3,})\s*$`)
)

var (
	h1Style = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FBBF24")).
		BorderBottom(true).
		BorderForeground(lipgloss.Color("#FBBF24"))

	h2Style = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#A78BFA")).
		BorderBottom(true).
		BorderForeground(lipgloss.Color("#6D28D9"))

	h3Style = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#34D399"))

	h4Style = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#60A5FA"))

	h5Style = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#F472B6"))

	h6Style = h5Style

	codeBlockBg = lipgloss.Color("#0F172A")
	codeBlockBorder = lipgloss.Color("#334155")
	codeBlockLangBg = lipgloss.Color("#1E293B")
	codeBlockLangFg = lipgloss.Color("#94A3B8")

	blockquoteStyle = lipgloss.NewStyle().
		BorderLeft(true).
		BorderForeground(lipgloss.Color("#8B5CF6")).
		BorderStyle(lipgloss.ThickBorder()).
		PaddingLeft(2).
		Foreground(lipgloss.Color("#9CA3AF")).
		Italic(true)

	hrStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#374151"))

	boldStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FBBF24"))

	italicStyle = lipgloss.NewStyle().
			Italic(true).
			Foreground(lipgloss.Color("#A78BFA"))

	inlineCodeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F472B6")).
			Background(lipgloss.Color("#1F2937")).
			Padding(0, 1)

	linkStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#60A5FA")).
			Underline(true)

	listBulletStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8B5CF6")).
			Bold(true)
)

func renderMarkdown(content string, width int) string {
	if width <= 0 {
		width = 80
	}

	var result []string
	lines := strings.Split(content, "\n")
	contentWidth := width

	i := 0
	for i < len(lines) {
		line := lines[i]

		if codeBlockStartRegex.MatchString(line) {
			lang, codeLines, skip := extractCodeBlock(lines, i)
			rendered := renderCodeBlock(lang, codeLines, contentWidth)
			result = append(result, rendered)
			result = append(result, "")
			i += skip
			continue
		}

		if isTableStart(lines, i) {
			tableLines, skip := extractTable(lines, i)
			if len(tableLines) >= 2 {
				rendered := renderTable(tableLines, contentWidth)
				result = append(result, rendered)
				result = append(result, "")
				i += skip
				continue
			}
		}

		if matches := headingRegex.FindStringSubmatch(line); matches != nil {
			level := len(matches[1])
			text := matches[2]
			rendered := renderHeading(text, level, contentWidth)
			result = append(result, rendered)
			i++
			continue
		}

		if hrRegex.MatchString(line) {
			result = append(result, renderHorizontalRule(contentWidth))
			result = append(result, "")
			i++
			continue
		}

		if matches := unorderedListRegex.FindStringSubmatch(line); matches != nil {
			listLines, skip := extractListBlock(lines, i, false)
			rendered := renderList(listLines, false, contentWidth)
			result = append(result, rendered)
			result = append(result, "")
			i += skip
			continue
		}

		if matches := orderedListRegex.FindStringSubmatch(line); matches != nil {
			listLines, skip := extractListBlock(lines, i, true)
			rendered := renderList(listLines, true, contentWidth)
			result = append(result, rendered)
			result = append(result, "")
			i += skip
			continue
		}

		if blockquoteRegex.MatchString(line) {
			quoteLines, skip := extractBlockquote(lines, i)
			rendered := renderBlockquote(quoteLines, contentWidth)
			result = append(result, rendered)
			result = append(result, "")
			i += skip
			continue
		}

		if line == "" {
			result = append(result, "")
		} else {
			rendered := renderInlineMarkdown(line)
			result = append(result, rendered)
		}
		i++
	}

	return strings.Join(collapseBlankLines(result), "\n")
}

// collapseBlankLines returns lines with runs of consecutive empty lines
// collapsed to a single empty line. Also strips a leading blank line so the
// assistant message doesn't start with vertical whitespace.
func collapseBlankLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	prevBlank := true // treat pre-start as blank so leading blanks are dropped
	for _, l := range lines {
		blank := strings.TrimSpace(stripStyle(l)) == ""
		if blank && prevBlank {
			continue
		}
		out = append(out, l)
		prevBlank = blank
	}
	// Trim trailing blank as well.
	for len(out) > 0 && strings.TrimSpace(stripStyle(out[len(out)-1])) == "" {
		out = out[:len(out)-1]
	}
	return out
}

func renderHeading(text string, level int, width int) string {
	text = renderInlineMarkdown(text)
	switch level {
	case 1:
		return h1Style.Width(width).Render(text)
	case 2:
		return h2Style.Width(width).Render(text)
	case 3:
		return h3Style.Render(text)
	case 4:
		return h4Style.Render(text)
	case 5:
		return h5Style.Render(text)
	default:
		return h6Style.Render(text)
	}
}

func renderHorizontalRule(width int) string {
	if width < 1 {
		width = 1
	}
	return hrStyle.Render(strings.Repeat("─", width))
}

func extractCodeBlock(lines []string, startIdx int) (string, []string, int) {
	lang := ""
	if matches := codeBlockStartRegex.FindStringSubmatch(lines[startIdx]); matches != nil {
		lang = matches[1]
	}

	var codeLines []string
	i := startIdx + 1
	for i < len(lines) {
		if codeBlockEndRegex.MatchString(lines[i]) {
			return lang, codeLines, i - startIdx + 1
		}
		codeLines = append(codeLines, lines[i])
		i++
	}
	return lang, codeLines, i - startIdx
}

func renderCodeBlock(lang string, codeLines []string, width int) string {
	var result []string

	if width < 6 {
		// 宽度过小，直接返回原始代码
		return strings.Join(codeLines, "\n")
	}

	borderColor := codeBlockBorder
	if lang != "" {
		borderColor = lipgloss.Color("#8B5CF6")
	}

	dashCount := width - 2
	if dashCount < 0 {
		dashCount = 0
	}
	topBorder := lipgloss.NewStyle().Foreground(borderColor).Render("╭" + strings.Repeat("─", dashCount) + "╮")
	result = append(result, topBorder)

	if lang != "" {
		langStyle := lipgloss.NewStyle().
			Foreground(codeBlockLangFg).
			Background(codeBlockLangBg).
			Bold(true).
			Padding(0, 2)

		leftBorder := lipgloss.NewStyle().Foreground(borderColor).Render("│")
		langLabel := langStyle.Render(strings.ToUpper(lang))
		langPad := width - 2 - runewidth.StringWidth(stripStyle(langLabel))
		if langPad < 0 {
			langPad = 0
		}
		padding := strings.Repeat(" ", langPad)
		langLine := leftBorder + langLabel + padding + leftBorder
		result = append(result, langLine)

		sepLine := lipgloss.NewStyle().Foreground(borderColor).Render("├" + strings.Repeat("─", dashCount) + "┤")
		result = append(result, sepLine)
	}

	codeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E2E8F0")).
		Background(codeBlockBg)

	leftBorder := lipgloss.NewStyle().Foreground(borderColor).Render("│")
	rightBorder := lipgloss.NewStyle().Foreground(borderColor).Render("│")

	// 内部内容宽度 = width - 2（去掉左右边框）
	// codePart = " " + wl，占 1 + len(wl) 宽度
	// padding 填充剩余空间，使总宽度 = width - 2
	innerWidth := width - 2
	for _, line := range codeLines {
		wrapped := wrapCode(line, innerWidth-1) // 留 1 给前缀空格
		for _, wl := range wrapped {
			codePart := codeStyle.Render(" " + wl)
			padCount := innerWidth - runewidth.StringWidth(" "+wl)
			if padCount < 0 {
				padCount = 0
			}
			padding := strings.Repeat(" ", padCount)
			bgPadding := lipgloss.NewStyle().Background(codeBlockBg).Render(padding)
			result = append(result, leftBorder+codePart+bgPadding+rightBorder)
		}
	}

	bottomBorder := lipgloss.NewStyle().Foreground(borderColor).Render("╰" + strings.Repeat("─", dashCount) + "╯")
	result = append(result, bottomBorder)

	return strings.Join(result, "\n")
}

func wrapCode(text string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{text}
	}

	var lines []string
	current := ""
	currentWidth := 0

	for _, r := range text {
		w := runewidth.RuneWidth(r)
		if currentWidth+w > maxWidth {
			lines = append(lines, current)
			current = string(r)
			currentWidth = w
		} else {
			current += string(r)
			currentWidth += w
		}
	}
	lines = append(lines, current)
	return lines
}

func extractListBlock(lines []string, startIdx int, ordered bool) ([][]string, int) {
	var groups [][]string
	var currentGroup []string

	i := startIdx
	for i < len(lines) {
		line := lines[i]
		if line == "" {
			if i+1 < len(lines) && (unorderedListRegex.MatchString(lines[i+1]) || orderedListRegex.MatchString(lines[i+1])) {
				i++
				continue
			}
			break
		}

		if ordered {
			if orderedListRegex.MatchString(line) {
				if len(currentGroup) > 0 {
					groups = append(groups, currentGroup)
					currentGroup = nil
				}
				currentGroup = append(currentGroup, line)
			} else if len(currentGroup) > 0 && strings.HasPrefix(line, " ") {
				currentGroup = append(currentGroup, line)
			} else if len(currentGroup) > 0 {
				break
			} else {
				break
			}
		} else {
			if unorderedListRegex.MatchString(line) {
				if len(currentGroup) > 0 {
					groups = append(groups, currentGroup)
					currentGroup = nil
				}
				currentGroup = append(currentGroup, line)
			} else if len(currentGroup) > 0 && strings.HasPrefix(line, " ") {
				currentGroup = append(currentGroup, line)
			} else if len(currentGroup) > 0 {
				break
			} else {
				break
			}
		}
		i++
	}

	if len(currentGroup) > 0 {
		groups = append(groups, currentGroup)
	}

	return groups, i - startIdx
}

func renderList(groups [][]string, ordered bool, width int) string {
	var result []string

	for idx, group := range groups {
		if len(group) == 0 {
			continue
		}

		var content string
		var indent int

		if ordered {
			matches := orderedListRegex.FindStringSubmatch(group[0])
			if matches != nil {
				num := matches[2]
				content = matches[3]
				bullet := listBulletStyle.Render(num + ". ")
				bulletWidth := runewidth.StringWidth(stripStyle(bullet))
				textWidth := width - bulletWidth - 2
				wrapped := wrapText(content, textWidth)
				for i, wl := range wrapped {
					if i == 0 {
						result = append(result, "  "+bullet+renderInlineMarkdown(wl))
					} else {
						pad := strings.Repeat(" ", bulletWidth+2)
						result = append(result, pad+renderInlineMarkdown(wl))
					}
				}
				indent = len(matches[1])
			}
		} else {
			matches := unorderedListRegex.FindStringSubmatch(group[0])
			if matches != nil {
				content = matches[2]
				bullet := listBulletStyle.Render("• ")
				bulletWidth := runewidth.StringWidth(stripStyle(bullet))
				textWidth := width - bulletWidth - 2
				wrapped := wrapText(content, textWidth)
				for i, wl := range wrapped {
					if i == 0 {
						result = append(result, "  "+bullet+renderInlineMarkdown(wl))
					} else {
						pad := strings.Repeat(" ", bulletWidth+2)
						result = append(result, pad+renderInlineMarkdown(wl))
					}
				}
				indent = len(matches[1])
			}
		}

		for j := 1; j < len(group); j++ {
			contLine := strings.TrimLeft(group[j], " ")
			pad := strings.Repeat(" ", indent+4)
			wrapped := wrapText(contLine, width-len(pad))
			for _, wl := range wrapped {
				result = append(result, pad+renderInlineMarkdown(wl))
			}
		}

		_ = idx
	}

	return strings.Join(result, "\n")
}

func extractBlockquote(lines []string, startIdx int) ([]string, int) {
	var quoteLines []string
	i := startIdx
	for i < len(lines) {
		if lines[i] == "" {
			break
		}
		if matches := blockquoteRegex.FindStringSubmatch(lines[i]); matches != nil {
			quoteLines = append(quoteLines, matches[1])
		} else {
			break
		}
		i++
	}
	return quoteLines, i - startIdx
}

func renderBlockquote(lines []string, width int) string {
	text := strings.Join(lines, " ")
	rendered := renderInlineMarkdown(text)
	wrapped := wrapText(stripStyle(rendered), width-6)
	var result []string
	for _, line := range wrapped {
		result = append(result, blockquoteStyle.Render(line))
	}
	return strings.Join(result, "\n")
}

func isTableStart(lines []string, idx int) bool {
	if idx+1 >= len(lines) {
		return false
	}
	if !tableRowRegex.MatchString(lines[idx]) {
		return false
	}
	return tableSeparatorRegex.MatchString(lines[idx+1])
}

func extractTable(lines []string, startIdx int) ([]string, int) {
	var tableLines []string
	i := startIdx
	for i < len(lines) {
		line := lines[i]
		if tableRowRegex.MatchString(line) {
			tableLines = append(tableLines, line)
			i++
		} else if tableSeparatorRegex.MatchString(line) {
			tableLines = append(tableLines, line)
			i++
		} else {
			break
		}
	}
	return tableLines, i - startIdx
}

func parseTableRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	cells := strings.Split(line, "|")
	for i, cell := range cells {
		cells[i] = strings.TrimSpace(cell)
	}
	return cells
}

func renderTable(tableLines []string, maxWidth int) string {
	var rows [][]string
	var separatorIdx = -1

	for i, line := range tableLines {
		if tableSeparatorRegex.MatchString(line) {
			separatorIdx = i
			continue
		}
		rows = append(rows, parseTableRow(line))
	}

	if len(rows) < 1 {
		return strings.Join(tableLines, "\n")
	}

	numCols := len(rows[0])
	for _, row := range rows {
		if len(row) > numCols {
			numCols = len(row)
		}
	}

	for i := range rows {
		for len(rows[i]) < numCols {
			rows[i] = append(rows[i], "")
		}
	}

	colWidths := make([]int, numCols)
	for _, row := range rows {
		for j, cell := range row {
			cellPlain := stripInlineMarkdown(cell)
			w := runewidth.StringWidth(cellPlain)
			if w > colWidths[j] {
				colWidths[j] = w
			}
		}
	}

	totalWidth := 1
	for _, w := range colWidths {
		totalWidth += w + 3
	}

	if totalWidth > maxWidth && numCols > 1 {
		excess := totalWidth - maxWidth
		for excess > 0 {
			widestIdx := 0
			for i := 1; i < numCols; i++ {
				if colWidths[i] > colWidths[widestIdx] {
					widestIdx = i
				}
			}
			if colWidths[widestIdx] <= 10 {
				break
			}
			colWidths[widestIdx]--
			excess--
		}
	}

	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FBBF24")).
		Bold(true)
	borderStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#4B5563"))
	cellStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E5E7EB"))

	var result []string

	topBorder := borderStyle.Render("┌")
	for j := 0; j < numCols; j++ {
		topBorder += borderStyle.Render(strings.Repeat("─", colWidths[j]+2))
		if j < numCols-1 {
			topBorder += borderStyle.Render("┬")
		}
	}
	topBorder += borderStyle.Render("┐")
	result = append(result, topBorder)

	for rowIdx := range rows {
		wrappedCells := make([][]string, numCols)
		maxLines := 1
		for j, cell := range rows[rowIdx] {
			cellPlain := stripInlineMarkdown(cell)
			wrapped := wrapText(cellPlain, colWidths[j])
			wrappedCells[j] = wrapped
			if len(wrapped) > maxLines {
				maxLines = len(wrapped)
			}
		}

		for lineIdx := 0; lineIdx < maxLines; lineIdx++ {
			lineStr := borderStyle.Render("│")
			for j := 0; j < numCols; j++ {
				cellText := ""
				if lineIdx < len(wrappedCells[j]) {
					cellText = wrappedCells[j][lineIdx]
				}
				cellText = renderInlineMarkdown(cellText)

				padding := colWidths[j] - runewidth.StringWidth(stripInlineMarkdown(cellText))
				if padding < 0 {
					padding = 0
				}
				padded := " " + cellText + strings.Repeat(" ", padding+1)

				if rowIdx == 0 && separatorIdx >= 0 {
					padded = headerStyle.Render(padded)
				} else {
					padded = cellStyle.Render(padded)
				}

				lineStr += padded + borderStyle.Render("│")
			}
			result = append(result, lineStr)
		}

		if rowIdx == 0 && separatorIdx >= 0 {
			sepLine := borderStyle.Render("├")
			for j := 0; j < numCols; j++ {
				sepLine += borderStyle.Render(strings.Repeat("─", colWidths[j]+2))
				if j < numCols-1 {
					sepLine += borderStyle.Render("┼")
				}
			}
			sepLine += borderStyle.Render("┤")
			result = append(result, sepLine)
		}
	}

	bottomBorder := borderStyle.Render("└")
	for j := 0; j < numCols; j++ {
		bottomBorder += borderStyle.Render(strings.Repeat("─", colWidths[j]+2))
		if j < numCols-1 {
			bottomBorder += borderStyle.Render("┴")
		}
	}
	bottomBorder += borderStyle.Render("┘")
	result = append(result, bottomBorder)

	return strings.Join(result, "\n")
}

func wrapText(text string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{text}
	}

	var lines []string
	runes := []rune(text)
	current := ""
	currentWidth := 0

	for _, r := range runes {
		w := runewidth.RuneWidth(r)
		if r == '\n' {
			lines = append(lines, current)
			current = ""
			currentWidth = 0
			continue
		}
		if currentWidth+w > maxWidth && current != "" {
			lines = append(lines, current)
			current = string(r)
			currentWidth = w
		} else {
			current += string(r)
			currentWidth += w
		}
	}
	if current != "" {
		lines = append(lines, current)
	}

	if len(lines) == 0 {
		lines = append(lines, "")
	}

	return lines
}

func stripInlineMarkdown(text string) string {
	text = regexp.MustCompile(`\*\*(.+?)\*\*`).ReplaceAllString(text, "$1")
	text = regexp.MustCompile(`\*(.+?)\*`).ReplaceAllString(text, "$1")
	text = regexp.MustCompile("`(.+?)`").ReplaceAllString(text, "$1")
	text = regexp.MustCompile(`\[(.+?)\]\(.+?\)`).ReplaceAllString(text, "$1")
	return text
}

func stripStyle(text string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(text, "")
}

func renderInlineMarkdown(text string) string {
	text = regexp.MustCompile(`\[(.+?)\]\((.+?)\)`).ReplaceAllStringFunc(text, func(m string) string {
		matches := regexp.MustCompile(`\[(.+?)\]\((.+?)\)`).FindStringSubmatch(m)
		if len(matches) >= 3 {
			return linkStyle.Render(matches[1]) + fmt.Sprintf(" (%s)", matches[2])
		}
		return m
	})

	text = regexp.MustCompile(`\*\*(.+?)\*\*`).ReplaceAllStringFunc(text, func(m string) string {
		content := m[2 : len(m)-2]
		return boldStyle.Render(content)
	})

	text = regexp.MustCompile(`\*(.+?)\*`).ReplaceAllStringFunc(text, func(m string) string {
		content := m[1 : len(m)-1]
		return italicStyle.Render(content)
	})

	text = regexp.MustCompile("`([^`]+)`").ReplaceAllStringFunc(text, func(m string) string {
		content := m[1 : len(m)-1]
		return inlineCodeStyle.Render(content)
	})

	return text
}
