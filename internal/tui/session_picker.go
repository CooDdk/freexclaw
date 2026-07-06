package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/CooDdk/freexclaw/internal/conversation"
)

type pickerItem struct {
	ID       string
	Title    string
	MsgCount int
	Current  bool
}

func pickerItemFromConversation(c *conversation.Conversation, current bool) pickerItem {
	return pickerItem{
		ID:       c.ID,
		Title:    c.Title,
		MsgCount: len(c.Messages),
		Current:  current,
	}
}

type sessionPickerSelectedMsg struct{ ID string }
type sessionPickerCancelledMsg struct{}

type sessionPicker struct {
	items  []pickerItem
	cursor int
}

func newSessionPicker(items []pickerItem) *sessionPicker {
	return &sessionPicker{items: items}
}

func (p *sessionPicker) Update(msg tea.Msg) (*sessionPicker, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}
	switch km.Type {
	case tea.KeyUp:
		p.cursor--
		if p.cursor < 0 {
			p.cursor = len(p.items) - 1
		}
	case tea.KeyDown:
		p.cursor++
		if p.cursor >= len(p.items) {
			p.cursor = 0
		}
	case tea.KeyEnter:
		if len(p.items) == 0 {
			return p, nil
		}
		id := p.items[p.cursor].ID
		return p, func() tea.Msg { return sessionPickerSelectedMsg{ID: id} }
	case tea.KeyEsc:
		return p, func() tea.Msg { return sessionPickerCancelledMsg{} }
	}
	return p, nil
}

func (p *sessionPicker) View() string {
	if len(p.items) == 0 {
		return "─ 选择会话 ──────────\n  (无会话)\n──────────────"
	}
	sep := "──────────────────────────────"
	title := lipgloss.NewStyle().Foreground(PrimaryColor).Bold(true).Render("─ 选择会话 ")
	var out []string
	out = append(out, title+sep)
	for i, it := range p.items {
		cursor := "  "
		if i == p.cursor {
			cursor = lipgloss.NewStyle().Foreground(AccentColor).Bold(true).Render("› ")
		}
		curMark := ""
		if it.Current {
			curMark = " ○"
		}
		line := fmt.Sprintf("%s%d  %s  (%d 条)%s", cursor, i+1, it.Title, it.MsgCount, curMark)
		out = append(out, line)
	}
	out = append(out, sep+"────")
	hint := lipgloss.NewStyle().Foreground(MutedColor).Faint(true).Render("(↑↓ 选择 · enter 打开 · esc 取消)")
	out = append(out, hint)
	return strings.Join(out, "\n")
}
