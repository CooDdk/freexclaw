# P2 Inline Rendering Migration Implementation Plan

**Goal:** 把 FreeX Claw 从 alt-screen 全屏 TUI 迁移到 inline scrollback 渲染，解决 Windows Terminal 状态条消失问题，同时保留 LOGO/品牌视觉。

**Architecture:** 移除 `tea.WithAltScreen()` 与 `tea.WithMouseCellMotion()`。Bubble Tea 的 View() 帧只渲染输入区 + spinner + 状态条，历史通过 `tea.Println()` 打进 scrollback。品牌视觉靠大 LOGO banner + 品牌符号（❯ ✻ ▸ ⚠）+ 定制 spinner 帧维持。

**Tech Stack:** Go 1.25 · Bubble Tea v1.3.10 · Bubbles (textarea, spinner) · Lipgloss · atotto/clipboard

**Reference spec:** `docs/design/2026-07-06-p2-inline-rendering-design.md`

**Working branch:** `feat/inline-rendering`（在开始 Task 1 前创建：`git checkout -b feat/inline-rendering`）

**关键前置说明（读第一段就够）：**

- `internal/tui/banner.go` 已经存在，含 `renderBanner(width int) string`（静态、可复用）和 `renderSplash()`（要删）。
- `internal/tui/model.go` ~1840 行，含 `Model` 结构、focus 系统、viewport、splash 状态、mouse handler、renderChat/renderHelpBar 等。迁移涉及删除约一半的代码。
- 现有测试：`internal/tui/model_test.go`（399 行，多为鼠标/focus/copy 相关，多数要删/改）。
- 现有 Model 字段 `showSplash` `splashStage` `splashProgress` `focus` `viewport` `showHelp` `dotsAnim` `flashMessage` 都要清掉。
- `cmd/main.go`（56 行）里的 `tea.WithAltScreen(), tea.WithMouseCellMotion()` 是问题根源。

---

## File Structure

**新增（3 个）：**
- `internal/tui/theme.go` — 品牌符号常量 + 品牌 spinner 帧 + marker 渲染 helper
- `internal/tui/render.go` — `renderUserMessage` / `renderAssistantMessage` / `renderToolCall` 纯函数
- `internal/tui/session_picker.go` — 内嵌会话选择器 sub-Model

**修改（4 个）：**
- `cmd/main.go` — 去 alt-screen/mouse，加 `--splash` flag，启动前 `fmt.Println(renderBanner(width))`
- `internal/tui/model.go` — Model 字段/View()/Update()/handleKeyMsg 重写，删鼠标/focus/splash
- `internal/tui/model_test.go` — 重写测试
- `internal/tui/styles.go` — 保留 status/input，删 ChatViewStyle

**删除（1 个函数）：**
- `internal/tui/banner.go` 里 `renderSplash()` 函数删除，`renderBanner()` 保留

---

## Task 1: theme.go — 品牌符号与 spinner 帧

**Files:**
- Create: `internal/tui/theme.go`
- Create: `internal/tui/theme_test.go`

- [ ] **Step 1: 写 theme_test.go**

```go
package tui

import (
	"strings"
	"testing"
)

func TestBrandSpinnerFrames_HasFourFrames(t *testing.T) {
	if got := len(BrandSpinnerFrames); got != 4 {
		t.Fatalf("expected 4 brand spinner frames, got %d", got)
	}
}

func TestBrandSpinnerFrames_AllNonEmpty(t *testing.T) {
	for i, f := range BrandSpinnerFrames {
		if strings.TrimSpace(f) == "" {
			t.Fatalf("frame %d is empty", i)
		}
	}
}

func TestMarkerUser_ContainsSymbol(t *testing.T) {
	if !strings.Contains(MarkerUser(), "❯") {
		t.Fatalf("expected MarkerUser to contain ❯, got %q", MarkerUser())
	}
}

func TestMarkerAssistant_ContainsSymbol(t *testing.T) {
	if !strings.Contains(MarkerAssistant(), "✻") {
		t.Fatalf("expected MarkerAssistant to contain ✻, got %q", MarkerAssistant())
	}
}

func TestMarkerToolStart_ContainsSymbol(t *testing.T) {
	if !strings.Contains(MarkerToolStart(), "▸") {
		t.Fatalf("expected MarkerToolStart to contain ▸, got %q", MarkerToolStart())
	}
}

func TestMarkerToolOK_ContainsCheck(t *testing.T) {
	if !strings.Contains(MarkerToolOK(), "✓") {
		t.Fatalf("expected MarkerToolOK to contain ✓, got %q", MarkerToolOK())
	}
}

func TestMarkerToolFail_ContainsCross(t *testing.T) {
	if !strings.Contains(MarkerToolFail(), "✗") {
		t.Fatalf("expected MarkerToolFail to contain ✗, got %q", MarkerToolFail())
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```
go test ./internal/tui/ -run TestBrandSpinner -v
```
Expected: FAIL — `undefined: BrandSpinnerFrames`

- [ ] **Step 3: 写 theme.go**

```go
package tui

import "github.com/charmbracelet/lipgloss"

// BrandSpinnerFrames 是 FreeX Claw 品牌 spinner 的四帧循环。
var BrandSpinnerFrames = []string{"✦", "✧", "✩", "✪"}

// 品牌色（与 styles.go 中的 UserColor / AssistantColor 保持一致）
var (
	MarkerUserStyle    = lipgloss.NewStyle().Foreground(UserColor).Bold(true)
	MarkerAssistStyle  = lipgloss.NewStyle().Foreground(AssistantColor).Bold(true)
	MarkerToolStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#F97316")).Bold(true)
	MarkerWarnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24")).Bold(true)
	MarkerOKStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Bold(true)
	MarkerFailStyle    = lipgloss.NewStyle().Foreground(ErrorColor).Bold(true)
)

func MarkerUser() string      { return MarkerUserStyle.Render("❯") }
func MarkerAssistant() string { return MarkerAssistStyle.Render("✻") }
func MarkerToolStart() string { return MarkerToolStyle.Render("▸") }
func MarkerToolOK() string    { return MarkerOKStyle.Render("✓") }
func MarkerToolFail() string  { return MarkerFailStyle.Render("✗") }
func MarkerWarn() string      { return MarkerWarnStyle.Render("⚠") }
```

- [ ] **Step 4: 运行测试确认通过**

```
go test ./internal/tui/ -run "TestBrandSpinner|TestMarker" -v
```
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/tui/theme.go internal/tui/theme_test.go
git commit -m "feat(tui): add brand theme (spinner frames + markers)"
```

---

## Task 2: render.go — renderUserMessage

**Files:**
- Create: `internal/tui/render.go`
- Create: `internal/tui/render_test.go`

- [ ] **Step 1: 写测试**

```go
package tui

import (
	"strings"
	"testing"
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
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), got)
	}
	// 首行带 marker；后续行缩进对齐（不带 marker）
	if !strings.Contains(lines[0], "❯") {
		t.Fatalf("first line missing marker: %q", lines[0])
	}
	if strings.Contains(lines[1], "❯") {
		t.Fatalf("continuation line should not have marker: %q", lines[1])
	}
}

func TestRenderUserMessage_EmptyReturnsEmpty(t *testing.T) {
	if got := renderUserMessage(""); got != "" {
		t.Fatalf("expected empty for empty input, got %q", got)
	}
}
```

- [ ] **Step 2: 运行确认失败**

```
go test ./internal/tui/ -run TestRenderUserMessage -v
```
Expected: FAIL — undefined `renderUserMessage`

- [ ] **Step 3: 写实现**

```go
package tui

import "strings"

// renderUserMessage 把用户输入渲染成一段可直接 tea.Println 到 scrollback 的字符串。
// 首行前缀 "❯ "，后续行缩进 2 个空格对齐。
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
```

- [ ] **Step 4: 运行确认通过**

```
go test ./internal/tui/ -run TestRenderUserMessage -v
```
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/tui/render.go internal/tui/render_test.go
git commit -m "feat(tui): add renderUserMessage"
```

---

## Task 3: render.go — renderAssistantMessage

**Files:**
- Modify: `internal/tui/render.go`
- Modify: `internal/tui/render_test.go`

- [ ] **Step 1: 追加测试**

在 `render_test.go` 末尾追加：

```go
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
```

- [ ] **Step 2: 运行确认失败**

```
go test ./internal/tui/ -run TestRenderAssistantMessage -v
```

- [ ] **Step 3: 追加实现到 render.go**

```go
// renderAssistantMessage 把 AI 回复渲染为 marker + markdown 结果。
// width 用于 markdown 换行（当前项目已有 markdown.go 的渲染器）。
func renderAssistantMessage(content string, width int) string {
	content = strings.TrimRight(content, "\n")
	if content == "" {
		return ""
	}
	rendered := renderMarkdown(content, width) // 已有函数，见 internal/tui/markdown.go
	rendered = strings.TrimRight(rendered, "\n")
	lines := strings.Split(rendered, "\n")
	var out []string
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
```

**注意：** `renderMarkdown` 是 `internal/tui/markdown.go` 已存在的函数（`Read` 一下确认签名，若签名不同则调整调用；本 plan 假设为 `func renderMarkdown(content string, width int) string`）。若签名不同，在此 Step 内调整以匹配现有 API。

- [ ] **Step 4: 运行确认通过**

```
go test ./internal/tui/ -run TestRenderAssistantMessage -v
```

- [ ] **Step 5: 提交**

```bash
git add internal/tui/render.go internal/tui/render_test.go
git commit -m "feat(tui): add renderAssistantMessage with markdown"
```

---

## Task 4: render.go — renderToolCall

**Files:**
- Modify: `internal/tui/render.go`
- Modify: `internal/tui/render_test.go`

- [ ] **Step 1: 追加测试**

```go
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
```

- [ ] **Step 2: 运行确认失败**

```
go test ./internal/tui/ -run TestRenderToolCall -v
```

- [ ] **Step 3: 追加实现**

```go
import (
	"fmt"
	"sort"
	"time"
)

// renderToolCall 把一次完成的工具调用渲染为固定格式的两三行文本。
// name: 工具名称（web_search / read_file / write_file / list_dir 等）
// args: 参数 map
// resultSummary: 结果摘要（单行或多行）
// ok: 是否成功
// durationMS: 耗时毫秒
func renderToolCall(name string, args map[string]interface{}, resultSummary string, ok bool, durationMS int) string {
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

func formatToolArgs(args map[string]interface{}) string {
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
		parts = append(parts, fmt.Sprintf("%s=%q", k, fmt.Sprint(v)))
	}
	return strings.Join(parts, ", ")
}

func formatToolDuration(ms int) string {
	if ms >= 1000 {
		return fmt.Sprintf("%.1fs", time.Duration(ms)*time.Millisecond.Seconds())
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
```

**注意：** `time.Duration(ms)*time.Millisecond.Seconds()` 写法有误，正确写法是 `float64(ms)/1000.0`。修正：

```go
func formatToolDuration(ms int) string {
	if ms >= 1000 {
		return fmt.Sprintf("%.1fs", float64(ms)/1000.0)
	}
	return fmt.Sprintf("%dms", ms)
}
```

- [ ] **Step 4: 运行确认通过**

```
go test ./internal/tui/ -run TestRenderToolCall -v
```

- [ ] **Step 5: 提交**

```bash
git add internal/tui/render.go internal/tui/render_test.go
git commit -m "feat(tui): add renderToolCall with success/failure markers"
```

---

## Task 5: session_picker.go — 内嵌选择器

**Files:**
- Create: `internal/tui/session_picker.go`
- Create: `internal/tui/session_picker_test.go`

- [ ] **Step 1: 写测试**

```go
package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/CooDdk/freexclaw/internal/conversation"
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

// 保证 pickerItemFromConversation 能从 Conversation 构造
func TestPickerItemFromConversation(t *testing.T) {
	c := &conversation.Conversation{ID: "abc", Title: "测试", Messages: make([]conversation.Message, 4)}
	it := pickerItemFromConversation(c, true)
	if it.ID != "abc" || it.Title != "测试" || it.MsgCount != 4 || !it.Current {
		t.Fatalf("unexpected item: %+v", it)
	}
}
```

- [ ] **Step 2: 运行确认失败**

```
go test ./internal/tui/ -run TestSessionPicker -v
```

- [ ] **Step 3: 写实现**

```go
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
```

- [ ] **Step 4: 运行确认通过**

```
go test ./internal/tui/ -run TestSessionPicker -v
go test ./internal/tui/ -run TestPickerItem -v
```

- [ ] **Step 5: 提交**

```bash
git add internal/tui/session_picker.go internal/tui/session_picker_test.go
git commit -m "feat(tui): add inline session picker component"
```

---

## Task 6: cmd/main.go — 移除 alt-screen 与 mouse，加 --splash flag

**Files:**
- Modify: `cmd/main.go`

- [ ] **Step 1: 完整重写 cmd/main.go**

```go
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"github.com/CooDdk/freexclaw/internal/config"
	"github.com/CooDdk/freexclaw/internal/tools"
	"github.com/CooDdk/freexclaw/internal/tui"
)

func main() {
	splashEnabled := flag.Bool("splash", false, "启用启动动画（默认关闭）")
	flag.Parse()

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[FreeX Claw] 获取当前目录失败: %v\n", err)
		os.Exit(1)
	}
	tools.SetWorkDir(cwd)

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[FreeX Claw] 加载配置失败: %v\n", err)
		os.Exit(1)
	}

	if cfg.APIKey == "" {
		configPath, _ := config.GetConfigPath()
		fmt.Fprintln(os.Stderr, "请先配置 API Key。")
		fmt.Fprintf(os.Stderr, "配置文件路径: %s\n", configPath)
		os.Exit(1)
	}

	// 启动前打印品牌 banner（一次性，进 scrollback）
	width := 80
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		width = w
	}
	fmt.Println(tui.RenderBannerPublic(width))
	fmt.Println()

	model, err := tui.NewModel(cfg, tui.ModelOptions{Splash: *splashEnabled})
	if err != nil {
		fmt.Fprintf(os.Stderr, "[FreeX Claw] 初始化界面失败: %v\n", err)
		os.Exit(1)
	}

	// 关键：不使用 WithAltScreen，不使用 WithMouseCellMotion
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "[FreeX Claw] 运行失败: %v\n", err)
		os.Exit(1)
	}
}
```

**说明：**
- `tui.RenderBannerPublic(width)` 是导出封装（Task 9 会在 banner.go 里加），当前 `renderBanner` 是小写包私有。
- `tui.NewModel` 签名要改（加 `ModelOptions`），会在 Task 7 里改。**Task 6 的这一步会让 build 暂时红**，Task 7 完成后恢复。这是可接受的红：连续两个 commit 内。

- [ ] **Step 2: 添加 golang.org/x/term 依赖**

```
go get golang.org/x/term@latest
go mod tidy
```

- [ ] **Step 3: 暂不构建**

因签名变更还没完成，跳过 `go build`。Task 7 结束再验证。

- [ ] **Step 4: 提交（build 暂红）**

```bash
git add cmd/main.go go.mod go.sum
git commit -m "feat(cmd): remove alt-screen and mouse capture, add --splash flag

Build will be red until Task 7 aligns Model signature."
```

---

## Task 7: model.go — 添加新状态字段与 View() 骨架

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/banner.go`（导出 `RenderBannerPublic`）

- [ ] **Step 1: 在 banner.go 顶部添加导出封装**

在 `banner.go` 最下方追加：

```go
// RenderBannerPublic 是 renderBanner 的对外导出封装，供 cmd/main.go 使用。
func RenderBannerPublic(width int) string {
	return renderBanner(width)
}
```

- [ ] **Step 2: 修改 Model 结构（在 model.go 里）**

在 `type Model struct {` 内部**追加**（保留旧字段，Task 10 才删）：

```go
	// P2 inline 迁移新增字段
	isThinking     bool
	thinkingLabel  string
	tokenCount     int
	streamBuf      strings.Builder
	spinnerTickN   int // 0-3 循环
	activeToolCall *pendingTool
	pickerActive   bool
	picker         *sessionPicker
	splashOpt      bool // 是否启用启动动画（--splash flag 传入）
```

在 model.go 靠近顶部（type 段附近）追加：

```go
type pendingTool struct {
	Name      string
	Arguments map[string]interface{}
	StartedAt time.Time
}

// ModelOptions 是 NewModel 的运行时选项。
type ModelOptions struct {
	Splash bool
}
```

- [ ] **Step 3: 修改 NewModel 签名**

```go
func NewModel(cfg *config.Config, opts ModelOptions) (*Model, error) {
	// ... 保持原有内容 ...
	// 在返回 m 前添加：
	m.splashOpt = opts.Splash
	m.showSplash = opts.Splash // 若不启用 splash，直接跳过动画
	return m, nil
}
```

- [ ] **Step 4: 验证 build 恢复**

```
go build ./...
```
Expected: 成功。若失败，检查其他调用 NewModel 的地方（可能测试文件里也调用）。

- [ ] **Step 5: 提交**

```bash
git add internal/tui/model.go internal/tui/banner.go
git commit -m "feat(tui): add inline Model state and ModelOptions"
```

---

## Task 8: model.go — 新版 View() 与 renderInline()（未激活）

**Files:**
- Modify: `internal/tui/model.go`

**目标：** 添加 `renderInline()` 但**不切换 View()**。允许后续任务先接线各种消息。

- [ ] **Step 1: 在 model.go 里添加 renderInline / renderSpinnerLine / renderToolCallLine**

```go
func (m *Model) renderInline() string {
	if m.pickerActive && m.picker != nil {
		return m.picker.View() + "\n" + m.renderStatusBarInline()
	}

	var parts []string
	parts = append(parts, m.textarea.View())

	if m.isThinking {
		parts = append(parts, m.renderSpinnerLine())
	}
	if m.activeToolCall != nil {
		parts = append(parts, m.renderToolCallLine())
	}
	parts = append(parts, m.renderStatusBarInline())
	return strings.Join(parts, "\n")
}

func (m *Model) renderSpinnerLine() string {
	frame := BrandSpinnerFrames[m.spinnerTickN%len(BrandSpinnerFrames)]
	label := m.thinkingLabel
	if label == "" {
		label = "思考中"
	}
	return lipgloss.NewStyle().Foreground(AssistantColor).Bold(true).Render(frame) +
		" " + label + "... " +
		lipgloss.NewStyle().Foreground(MutedColor).Faint(true).
			Render(fmt.Sprintf("(%d tokens)", m.tokenCount))
}

func (m *Model) renderToolCallLine() string {
	frame := BrandSpinnerFrames[m.spinnerTickN%len(BrandSpinnerFrames)]
	args := formatToolArgs(m.activeToolCall.Arguments)
	head := fmt.Sprintf("%s %s(%s)", MarkerToolStart(), m.activeToolCall.Name, args)
	sub := fmt.Sprintf("  %s 执行中...",
		lipgloss.NewStyle().Foreground(AssistantColor).Render(frame))
	return head + "\n" + sub
}

// renderStatusBarInline 品牌化状态条：上方一根渐变分隔线 + 下方 icon 分隔的信息行。
func (m *Model) renderStatusBarInline() string {
	sep := " │ "
	sess := m.convMgr.GetCurrent()
	title := "会话1"
	msgCount := 0
	if sess != nil {
		title = sess.Title
		msgCount = len(sess.Messages)
	}
	parts := []string{
		MarkerAssistant() + " FreeX Claw",
		m.cfg.Model,
		fmt.Sprintf("📁 %s (%d条)", title, msgCount),
		"/help",
	}
	info := StatusBarStyle.Render(strings.Join(parts, sep))

	// 顶部渐变分隔线（占宽度或退化为固定长度）
	lineW := m.width
	if lineW <= 0 {
		lineW = 80
	}
	gradient := renderGradientLine(lineW)
	return gradient + "\n" + info
}

// renderGradientLine 用 UserColor → AssistantColor 的渐变绘制一行 ▔。
func renderGradientLine(width int) string {
	if width <= 0 {
		return ""
	}
	chars := make([]string, width)
	// 两色简化渐变：前半 UserColor，后半 AssistantColor
	mid := width / 2
	userStyle := lipgloss.NewStyle().Foreground(UserColor)
	assistStyle := lipgloss.NewStyle().Foreground(AssistantColor)
	for i := 0; i < width; i++ {
		if i < mid {
			chars[i] = userStyle.Render("▔")
		} else {
			chars[i] = assistStyle.Render("▔")
		}
	}
	return strings.Join(chars, "")
}
```

- [ ] **Step 2: 验证 build**

```
go build ./...
```
Expected: 成功。

- [ ] **Step 3: 单元测试 renderStatusBarInline**

在 `internal/tui/render_test.go` 追加：

```go
func TestRenderStatusBarInline_ContainsBrand(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	m := &Model{
		cfg:     &config.Config{Model: "test-model"},
		convMgr: conversation.NewManager(root),
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
```

需要在 test import 里加 `"github.com/CooDdk/freexclaw/internal/config"` `"github.com/CooDdk/freexclaw/internal/conversation"`（现有 model_test.go 里已有类似样板）。

```
go test ./internal/tui/ -run TestRenderStatusBarInline -v
```
Expected: PASS

- [ ] **Step 4: 提交**

```bash
git add internal/tui/model.go internal/tui/render_test.go
git commit -m "feat(tui): add renderInline (not yet wired to View)"
```

---

## Task 9: model.go — 切换 View() 到 renderInline，并接线消息路由

**Files:**
- Modify: `internal/tui/model.go`

**关键：** 这是原子切换的一步。View() 从旧版换到新版；Update() 里的消息路由改为使用 `tea.Println` 派发历史。

- [ ] **Step 1: 替换 View() 主体**

找到 `func (m *Model) View() string { ... }`（约 589 行开始），整个函数体替换为：

```go
func (m *Model) View() string {
	return m.renderInline()
}
```

- [ ] **Step 2: 替换 sendMessage（约 702 行）—— 发送前先把用户消息打到 scrollback**

在 `sendMessage()` 开头（把当前 textarea 内容取出、清空之前），先构造要打到 scrollback 的行；然后返回一个 `tea.Sequence(tea.Println(userLine), <existing stream cmd>)`。示例（**要根据现有函数结构合并**）：

```go
func (m *Model) sendMessage() tea.Cmd {
	content := strings.TrimSpace(m.textarea.Value())
	if content == "" {
		return nil
	}
	m.textarea.Reset()
	m.addToInputHistory(content)

	// 打用户消息到 scrollback
	userLine := renderUserMessage(content)

	// ... 保留原有 preflight / conversation state 更新逻辑 ...
	// 在末尾把 stream cmd 前面加 tea.Println：
	streamCmd := m.waitForStream() // 或原函数返回的 cmd
	// spinner 状态
	m.isThinking = true
	m.thinkingLabel = "思考中"
	m.tokenCount = 0
	m.streamBuf.Reset()

	return tea.Sequence(
		tea.Println(userLine),
		streamCmd,
	)
}
```

**注意：** 现有 `sendMessage` 有约 70 行的复杂逻辑（preflight, 消息组装, 会话保存等），本 Step **不改这些**，只在函数返回处包一层 `tea.Sequence`。如需修改结构，先 `Read` 现有函数完整实现再改。

- [ ] **Step 3: 改写 handleStreamMsg 里的完成分支（约 525 行）**

原逻辑：把 chunk 追加到 `current.Messages` 最后一条 assistant 消息、`updateChatView()`。

改为：
- chunk 累加到 `m.streamBuf`，同时累加 `m.tokenCount++`
- 完成时：把 `m.streamBuf.String()` 作为最终 content 存进 `current.Messages`（保持既有持久化不变），用 `tea.Println(renderAssistantMessage(...))` 打进 scrollback，清空 `isThinking` / `streamBuf`

示例（**注意与现有 tool call 分支合并**）：

```go
func (m *Model) handleStreamMsg(msg streamMsg) (tea.Model, tea.Cmd) {
	current := m.convMgr.GetCurrent()

	if msg.err != nil {
		m.isThinking = false
		m.err = msg.err
		errLine := MarkerWarn() + " 出错: " + msg.err.Error()
		return m, tea.Println(errLine)
	}

	if msg.done {
		m.isThinking = false
		content := m.streamBuf.String()
		m.streamBuf.Reset()

		// 持久化：现有代码依赖 current.Messages 的最后一条 assistant 消息含完整内容
		if len(current.Messages) > 0 {
			last := &current.Messages[len(current.Messages)-1]
			if last.Role == conversation.RoleAssistant {
				last.Content = content
			}
		}

		// 工具调用检测 —— 保留现有 pruneTrailingToolCallMessage / startToolExecution 逻辑
		if tc := agent.ParseToolCall(content); tc != nil {
			m.pruneTrailingToolCallMessage()
			m.convMgr.Save()
			if m.shouldSkipDuplicateToolCall(tc) {
				return m, m.skipDuplicateToolCall(tc)
			}
			return m, m.startToolExecution(tc)
		}

		m.convMgr.Save()

		// 打到 scrollback
		var cmds []tea.Cmd
		if trimmed := strings.TrimSpace(content); trimmed != "" {
			cmds = append(cmds, tea.Println(renderAssistantMessage(trimmed, m.width)))
		}
		// 后续 auto-extract / engineering flow / post-process 保持原有逻辑
		if m.shouldAutoExtractSingleFile() {
			// ... 保留 ...
		}
		if cmd := m.maybeContinueEngineeringToolFlow(content); cmd != nil {
			cmds = append(cmds, cmd)
		}
		if cmd := m.maybeRunForcedPostProcess(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)
	}

	// 中途 chunk：只累加到 buffer，不刷 UI
	m.streamBuf.WriteString(msg.content)
	m.tokenCount++
	// 同步到 conversation.Messages 供持久化
	if len(current.Messages) > 0 {
		last := &current.Messages[len(current.Messages)-1]
		last.Content = m.streamBuf.String()
	}
	return m, m.waitForStream()
}
```

- [ ] **Step 4: spinner tick**

在 model.go 顶部保留 `tickCmd()` / `tickMsg`。修改 `tickMsg` case：

```go
case tickMsg:
	if m.isThinking || m.activeToolCall != nil {
		m.spinnerTickN++
		return m, tickCmd()
	}
	return m, nil
```

在 `sendMessage` 里加 tickCmd 触发（如果没有的话）：`return tea.Batch(tea.Println(userLine), streamCmd, tickCmd())`。

- [ ] **Step 5: WindowSizeMsg 精简**

现有 `resize()` 计算了 chatArea 高度等。替换为：

```go
case tea.WindowSizeMsg:
	m.width = msg.Width
	m.height = msg.Height
	m.textarea.SetWidth(msg.Width)
	return m, nil
```

（chatAreaHeight / hintAreaHeight / inputAreaHeight 等辅助函数暂时保留，Task 10 一并删。）

- [ ] **Step 6: 编译并运行**

```
go build ./...
```

手工运行一次：`.\freexclaw.exe`（Windows）或 `./freexclaw`（Unix）。期望：
- banner 一次性打印
- 输入 "hi" 能得到 spinner 转、然后 assistant 回复打进 scrollback
- 状态条常驻底部

Windows Terminal 里点击/滚动应**不再消失**状态条。

- [ ] **Step 7: 提交**

```bash
git add internal/tui/model.go
git commit -m "feat(tui): switch View() to inline renderer and route stream to tea.Println"
```

---

## Task 10: model.go — 接线 tool call 与 slash command

**Files:**
- Modify: `internal/tui/model.go`

- [ ] **Step 1: 改写 startToolExecution / handleToolExecuted**

`startToolExecution` 里设置 `m.activeToolCall = &pendingTool{Name: tc.Name, Arguments: tc.Arguments, StartedAt: time.Now()}`。

`handleToolExecuted` 里：
- 计算 `durationMS := int(time.Since(m.activeToolCall.StartedAt) / time.Millisecond)`
- 拿 `msg.result` 摘要（截断到 200 字符做展示，完整存 conversation）
- 构造 `line := renderToolCall(m.activeToolCall.Name, m.activeToolCall.Arguments, summary, msg.err == nil, durationMS)`
- `m.activeToolCall = nil`
- 返回 `tea.Sequence(tea.Println(line), <continue LLM turn>)`

**示例合并到现有 handleToolExecuted**（约 1569 行）：

**执行前先 `Read` 现有 `handleToolExecuted` 完整实现**，识别现有代码里"工具执行结束后如何继续 LLM 下一轮"的路径（通常是构造下一个 stream request 或调用 `sendMessage` 变体）。保留这段逻辑不动，只在开头添加 `tea.Println(line)` 通过 `tea.Sequence` 串在最终 return 前。

```go
func (m *Model) handleToolExecuted(msg toolExecutedMsg) (tea.Model, tea.Cmd) {
	// ... 保留原有 conversation 写入 tool_result 消息的逻辑 ...

	// 计算展示
	var durationMS int
	var toolName string
	var toolArgs map[string]interface{}
	if m.activeToolCall != nil {
		durationMS = int(time.Since(m.activeToolCall.StartedAt) / time.Millisecond)
		toolName = m.activeToolCall.Name
		toolArgs = m.activeToolCall.Arguments
	}
	m.activeToolCall = nil

	summary := msg.result
	if len(summary) > 200 {
		summary = summary[:200] + "..."
	}
	line := renderToolCall(toolName, toolArgs, summary, msg.err == nil, durationMS)

	// 保留原有"继续 LLM 下一轮"逻辑；把结果 cmd 放进 continueCmd
	var continueCmd tea.Cmd
	// ... 从原函数复制现有的继续逻辑（可能是 m.waitForStream / m.sendFollowup / 类似）...

	if continueCmd == nil {
		return m, tea.Println(line)
	}
	return m, tea.Sequence(tea.Println(line), continueCmd)
}
```

- [ ] **Step 2: 改写 handleCommand（slash 命令）**

`handleCommand` 是现有约 903 行的长 switch。目标：**保持所有命令语义不变**，但输出方式改为 `tea.Println`。

`/sessions` 分支改为：

```go
case "sessions":
	items := make([]pickerItem, 0)
	for _, c := range m.convMgr.List() {
		items = append(items, pickerItemFromConversation(c, m.convMgr.CurrentID() == c.ID))
	}
	m.picker = newSessionPicker(items)
	m.pickerActive = true
	return nil
```

其他分支（`/help` `/new` `/clear` 等）里凡是 `m.convMgr.AddSystemMessage(...)` 或写入 chatView 的地方，改为返回 `tea.Println(marker + " " + text)`。

- [ ] **Step 3: 处理 sessionPickerSelectedMsg / sessionPickerCancelledMsg**

在 `Update()` 主 switch 追加：

```go
case sessionPickerSelectedMsg:
	m.pickerActive = false
	m.picker = nil
	m.convMgr.Switch(msg.ID)
	current := m.convMgr.GetCurrent()
	title := "未命名"
	if current != nil {
		title = current.Title
	}
	return m, tea.Println(fmt.Sprintf("%s → 已切换到 %q", MarkerAssistant(), title))

case sessionPickerCancelledMsg:
	m.pickerActive = false
	m.picker = nil
	return m, nil
```

在 KeyMsg 处理开头判定 picker 模式：

```go
case tea.KeyMsg:
	if m.pickerActive && m.picker != nil {
		p, cmd := m.picker.Update(msg)
		m.picker = p
		return m, cmd
	}
	// ... 后续原有 handleKeyMsg 逻辑 ...
```

- [ ] **Step 4: 编译并手工验证**

```
go build ./...
./freexclaw
```

手工验证清单：
1. 输入 `/sessions` → picker 出现在输入框上方 → ↑↓ 移动 → enter 切换
2. 输入 `/help` → 帮助文本进 scrollback
3. 输入 `/new` → 新会话，`tea.Println("→ 已新建会话")` 类提示
4. 输入天气类问题 → 触发 web_search → 工具行进 scrollback → 最终回复进 scrollback

- [ ] **Step 5: 提交**

```bash
git add internal/tui/model.go
git commit -m "feat(tui): route tool calls and slash commands through tea.Println"
```

---

## Task 11: 删除死代码（focus/mouse/viewport/splash/chat renderer）

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/banner.go`
- Modify: `internal/tui/styles.go`

- [ ] **Step 1: 从 model.go 删除以下函数**

- `handleMouseMsg` (约 258-297 行)
- `setFocusInput` / `setFocusChat` / `toggleFocus`
- `handleChatKey` (约 452-480 行)
- `copyLastAssistantMessage` (可能被 `y` 键使用；spec 已确认可弃)
- `renderChat` (约 1363 行)
- `renderHelpBar` (约 1419 行)
- `chatAreaHeight` / `isChatAreaY` / `isInputZoneY` / `hintAreaHeight` / `inputAreaHeight`
- `updateChatView` (约 1213 行 —— 现在不再需要 viewport)

- [ ] **Step 2: 从 Model 结构删除以下字段**

```go
	viewport     viewport.Model
	focus        focusState
	showHelp     bool
	dotsAnim     int
	forceScrollBottom bool
	showSplash     bool
	splashStage    int
	splashProgress float64
	flashMessage       string
```

以及移除对应 import：`github.com/charmbracelet/bubbles/viewport`、`github.com/atotto/clipboard`（若确认不再使用）。

- [ ] **Step 3: 从 Update() 主 switch 里删除 splash 相关分支**

- `splashTickMsg` case
- `splashEndMsg` case
- 顶部 `type splashTickMsg` / `type splashEndMsg` / `splashTickCmd` 函数

- [ ] **Step 4: 从 banner.go 删除 renderSplash 函数**

删除 `func (m *Model) renderSplash() string { ... }` 整个（约 120-271 行）。保留 `renderBanner` 和 `RenderBannerPublic`。

- [ ] **Step 5: 从 styles.go 删除 ChatViewStyle**

删除：
```go
	ChatViewStyle = lipgloss.NewStyle().
			Padding(1, 2)
```

`InputAreaStyle` 里的 `Border(top)` 建议改为 `Border(false, false, false, false)`（或整个删除 border，改成仅在 View() 里加分隔线）。若测试不依赖，可保留原样。

- [ ] **Step 6: 编译**

```
go build ./...
```

Expected: 成功。若出现"unused import"或"undefined variable"报错，逐个清理。

- [ ] **Step 7: 提交**

```bash
git add internal/tui/model.go internal/tui/banner.go internal/tui/styles.go
git commit -m "refactor(tui): delete legacy alt-screen renderer and focus/mouse/splash code"
```

---

## Task 12: 重写 model_test.go

**Files:**
- Modify: `internal/tui/model_test.go`
- Modify: `internal/tui/model_render_test.go`

- [ ] **Step 1: 删除失效的测试**

删除以下测试函数：
- `TestHandleKeyMsg_CtrlCClearsInputBeforeQuit`（保留但改写：现在不需要 `focus` 字段）
- `TestHandleKeyMsg_CtrlCTwiceQuits`（同上）
- `TestHandleKeyMsg_OtherKeyClearsCtrlCArmState`（同上）
- `TestHandleMouseMsg_*` 全部三个删除（无鼠标处理）
- `TestRenderHelpBar_ChatFocusShowsCopyHints`（无 help bar）
- `TestCopyLastAssistantMessage_*` 全部（无 y 复制）

保留并修改：
- `TestBuildPreflightToolCall_*`（业务逻辑不变）
- `TestShouldSkipDuplicateToolCall_*`（业务逻辑不变）
- `TestPruneTrailingToolCallMessage_*`（业务逻辑不变）
- `TestShouldAutoExtractSingleFile_*` / `TestHasSubstantiveProjectFiles_*`（不涉及 UI）

- [ ] **Step 2: 修改保留测试里的 Model 构造**

原：
```go
m := &Model{
    convMgr:   conversation.NewManager(root),
    textarea:  ta,
    focus:     focusInput,
    showHelp:  true,
    ...
}
```

改为：
```go
m := &Model{
    cfg:      &config.Config{Model: "test-model"},
    convMgr:  conversation.NewManager(root),
    textarea: ta,
}
```

- [ ] **Step 3: 新增测试**

```go
func TestHandleKeyMsg_CtrlCArmsAndQuits(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	ta := textarea.New()
	ta.SetValue("hello")
	m := &Model{
		cfg:      &config.Config{Model: "test"},
		convMgr:  conversation.NewManager(root),
		textarea: ta,
	}
	defer m.convMgr.Close()

	// first ctrl+c clears input, arms exit
	model, cmd := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyCtrlC})
	got := model.(*Model)
	if cmd != nil {
		t.Fatal("first ctrl+c should not quit")
	}
	if got.textarea.Value() != "" {
		t.Fatalf("expected input cleared, got %q", got.textarea.Value())
	}
	// second ctrl+c quits
	_, cmd = got.handleKeyMsg(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("second ctrl+c should return quit cmd")
	}
}

func TestSessionsCommand_ActivatesPicker(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	m := &Model{
		cfg:     &config.Config{Model: "test"},
		convMgr: conversation.NewManager(root),
	}
	defer m.convMgr.Close()

	m.handleCommand("sessions")
	if !m.pickerActive {
		t.Fatal("expected pickerActive after /sessions")
	}
	if m.picker == nil {
		t.Fatal("expected picker instance")
	}
}

func TestViewInline_ContainsStatusBar(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	ta := textarea.New()
	m := &Model{
		cfg:      &config.Config{Model: "test-model"},
		convMgr:  conversation.NewManager(root),
		textarea: ta,
		width:    120,
	}
	defer m.convMgr.Close()

	v := m.View()
	if !strings.Contains(v, "FreeX Claw") {
		t.Fatalf("expected status bar brand in View: %q", v)
	}
}

func TestViewInline_ThinkingRendersSpinner(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	ta := textarea.New()
	m := &Model{
		cfg:           &config.Config{Model: "test"},
		convMgr:       conversation.NewManager(root),
		textarea:      ta,
		width:         120,
		isThinking:    true,
		thinkingLabel: "思考中",
	}
	defer m.convMgr.Close()

	v := m.View()
	// spinner 帧之一或"思考中"应出现
	if !strings.Contains(v, "思考中") {
		t.Fatalf("expected 思考中 label in View: %q", v)
	}
}
```

- [ ] **Step 4: 运行完整测试套件**

```
go test ./... -v
```
Expected: 全绿。如有失败，逐个修复。

- [ ] **Step 5: 提交**

```bash
git add internal/tui/model_test.go internal/tui/model_render_test.go
git commit -m "test(tui): rewrite model tests for inline architecture"
```

---

## Task 13: 手工验证矩阵

**不是 code task，但必须做，做完打 tag。**

- [ ] **Step 1: Windows Terminal + PowerShell（关键场景）**

在 Windows Terminal + PowerShell 里跑：
```
.\freexclaw.exe
```

验证：
- [ ] banner 完整打印，颜色渐变正常
- [ ] 输入"你好"→ spinner 转 → 回复 push 到 scrollback
- [ ] 底部状态条**在鼠标点击/滚动消息内容时不消失**（核心目标）
- [ ] 鼠标左键拖拽消息内容可以**原生选中**
- [ ] Ctrl+Shift+C 复制成功
- [ ] `/sessions` 切换正常
- [ ] Ctrl+C 一次清空输入，二次退出

- [ ] **Step 2: Windows cmd.exe**

同 Step 1 关键项验证。

- [ ] **Step 3: VS Code 集成终端**

同 Step 1 关键项验证。

- [ ] **Step 4: 若有条件，macOS Terminal / iTerm2 或 Linux xterm**

- [ ] **Step 5: 修 bug（若有）**

若发现问题，创建修复 commit（"fix(tui): ..."）。

- [ ] **Step 6: 打 tag & 合并**

```bash
# 项目实际走 0.x semver（v0.1.3 是 pre-inline 的 latest release）。
# 本分支合并后打下一位 patch 或 minor，例如 v0.1.4。
git tag v0.1.3-preinline master   # 可选：给 pre-inline 顶点起个别名，便于回滚
git tag v0.1.4 HEAD               # 合并后打在 master 上，触发 release workflow
```

（如果这是 PR 流程，改为在 PR 描述里贴对比截图/录屏。）

---

## Self-Review Checklist（由 executor 完成整个 plan 后自审）

- [ ] Banner 一次性打印，进 scrollback，无重复
- [ ] 状态条常驻底部，不随鼠标操作消失
- [ ] Spinner 用品牌帧 `✦✧✩✪`，非默认 Braille
- [ ] 用户消息前缀 `❯`，AI 消息前缀 `✻`，工具调用 `▸ ✓/✗`
- [ ] 会话切换用内嵌 picker，非全屏列表
- [ ] `/help` `/new` `/clear` 输出通过 `tea.Println` 进 scrollback
- [ ] `--splash` flag 默认关闭；若指定则播放动画（Task 8 未实现此动画路径，若最终需要，在 Task 8/9 之间补 splashTick 逻辑；MVP 可先只做 `--splash` flag 存在但不启用动画，接受 `--splash` 无效果）
- [ ] 所有单元测试通过
- [ ] Windows Terminal 手工验证矩阵完成

**关于 `--splash` 的说明：** MVP 阶段可以先实现"接受 flag 但不启用动画"（因为动画本身与 inline 架构存在潜在闪烁风险）。如果第一版验证一切平稳，第二阶段单独做动画 (Task 14 in a follow-up plan)。
