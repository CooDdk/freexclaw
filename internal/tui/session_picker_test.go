package tui

import (
	"strings"
	"testing"

	"github.com/CooDdk/freexclaw/internal/conversation"
	tea "github.com/charmbracelet/bubbletea"
)

func newTestPicker() *sessionPicker {
	items := []pickerItem{
		{ID: "1", Title: "项目分析", MsgCount: 12},
		{ID: "2", Title: "天气查询", MsgCount: 3},
		{ID: "3", Title: "当前会话", MsgCount: 5, Current: true},
	}
	return newSessionPicker(items)
}

func TestSessionPicker_View_ShowsAllItems(t *testing.T) {
	p := newTestPicker()
	v := p.View()
	for _, name := range []string{"项目分析", "天气查询", "当前会话"} {
		if !strings.Contains(v, name) {
			t.Fatalf("expected %q in view, got %q", name, v)
		}
	}
}

func TestSessionPicker_Down_MovesHighlight(t *testing.T) {
	p := newTestPicker()
	if got := p.cursor; got != 0 {
		t.Fatalf("initial cursor should be 0, got %d", got)
	}
	p.Update(tea.KeyMsg{Type: tea.KeyDown})
	if got := p.cursor; got != 1 {
		t.Fatalf("after down cursor should be 1, got %d", got)
	}
}

func TestSessionPicker_Up_AtTop_Wraps(t *testing.T) {
	p := newTestPicker()
	p.Update(tea.KeyMsg{Type: tea.KeyUp})
	if got := p.cursor; got != 2 {
		t.Fatalf("expected wrap to last (2), got %d", got)
	}
}

func TestSessionPicker_Enter_EmitsSelectMsg(t *testing.T) {
	p := newTestPicker()
	p.cursor = 1
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected select cmd")
	}
	msg := cmd()
	sel, ok := msg.(sessionPickerSelectedMsg)
	if !ok {
		t.Fatalf("expected sessionPickerSelectedMsg, got %T", msg)
	}
	if sel.ID != "2" {
		t.Fatalf("expected ID=2, got %s", sel.ID)
	}
}

func TestSessionPicker_Esc_EmitsCancelMsg(t *testing.T) {
	p := newTestPicker()
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected cancel cmd")
	}
	if _, ok := cmd().(sessionPickerCancelledMsg); !ok {
		t.Fatalf("expected sessionPickerCancelledMsg, got %T", cmd())
	}
}

func TestPickerItemFromConversation(t *testing.T) {
	c := &conversation.Conversation{ID: "abc", Title: "测试", Messages: make([]conversation.Message, 4)}
	it := pickerItemFromConversation(c, true)
	if it.ID != "abc" || it.Title != "测试" || it.MsgCount != 4 || !it.Current {
		t.Fatalf("unexpected item: %+v", it)
	}
}
