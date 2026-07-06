package tui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/CooDdk/freexclaw/internal/agent"
	"github.com/CooDdk/freexclaw/internal/config"
	"github.com/CooDdk/freexclaw/internal/conversation"
	"github.com/CooDdk/freexclaw/internal/tools"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

func TestBuildPreflightToolCall_SkipsWeatherWithoutLocation(t *testing.T) {
	tools.SetCurrentLiveQueryContext(tools.LiveQueryContext{})
	defer tools.SetCurrentLiveQueryContext(tools.LiveQueryContext{})

	if tc := buildPreflightToolCall("未来15天呢"); tc != nil {
		t.Fatalf("expected no preflight tool call without location, got %#v", tc)
	}
}

func TestBuildPreflightToolCall_ProjectScaffoldUsesListDir(t *testing.T) {
	tc := buildPreflightToolCall("使用 nestjs 写一个简单的 api 服务")
	if tc == nil {
		t.Fatal("expected project scaffold preflight tool call")
	}
	if tc.Name != "list_dir" {
		t.Fatalf("expected list_dir tool, got %q", tc.Name)
	}
	if got := tc.Arguments["path"]; got != "." {
		t.Fatalf("expected list_dir path '.', got %#v", got)
	}
}

func TestBuildPreflightToolCall_UsesRawFollowUpQuery(t *testing.T) {
	tools.SetCurrentLiveQueryContext(tools.LiveQueryContext{
		Domain:   "weather",
		Location: "武汉",
	})
	defer tools.SetCurrentLiveQueryContext(tools.LiveQueryContext{})

	tc := buildPreflightToolCall("未来7天的天气呢")
	if tc == nil {
		t.Fatal("expected preflight tool call")
	}
	if tc.Name != "web_search" {
		t.Fatalf("expected web_search tool, got %q", tc.Name)
	}

	query, _ := tc.Arguments["query"].(string)
	if query != "武汉 7天 天气预报" {
		t.Fatalf("expected canonical weather query, got %q", query)
	}
}

func TestShouldSkipDuplicateToolCall_WebSearch(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	m := &Model{
		convMgr: conversation.NewManager(root),
	}
	defer m.convMgr.Close()
	tools.SetCurrentLiveQueryContext(tools.LiveQueryContext{
		Domain:       "weather",
		Location:     "武汉",
		ForecastDays: 7,
	})
	defer tools.SetCurrentLiveQueryContext(tools.LiveQueryContext{})
	current := m.convMgr.GetCurrent()
	current.AddMessage(conversation.RoleUser, "未来7天的天气呢")
	current.AddMessage(conversation.RoleUser, "<|tool_result|>\n搜索关键词: 武汉 7天 天气预报\n\n1. **武汉天气**\n</|tool_result|>")
	current.AddMessage(conversation.RoleAssistant, "<web_search>武汉 7天 天气预报</web_search>")

	tc := buildPreflightToolCall("未来7天的天气呢")
	if tc == nil {
		t.Fatal("expected tool call")
	}
	if !m.shouldSkipDuplicateToolCall(tc) {
		t.Fatal("expected duplicate web_search to be skipped")
	}
}

func TestPruneTrailingToolCallMessage_RemovesAssistantToolCall(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	m := &Model{
		convMgr: conversation.NewManager(root),
	}
	defer m.convMgr.Close()

	current := m.convMgr.GetCurrent()
	current.AddMessage(conversation.RoleUser, "看一下武汉今天的天气")
	current.AddMessage(conversation.RoleAssistant, "<web_search>武汉 实时天气</web_search>")

	m.pruneTrailingToolCallMessage()

	if got := len(current.Messages); got != 1 {
		t.Fatalf("expected trailing tool call message to be removed, got %d messages", got)
	}
	if current.Messages[0].Role != conversation.RoleUser {
		t.Fatalf("expected remaining message to be user message, got %q", current.Messages[0].Role)
	}
}

func TestShouldSkipDuplicateToolCall_DoesNotSkipFailedWebSearch(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	m := &Model{
		convMgr: conversation.NewManager(root),
	}
	defer m.convMgr.Close()

	current := m.convMgr.GetCurrent()
	current.AddMessage(conversation.RoleUser, "北京最新的天气")
	current.AddMessage(conversation.RoleUser, "<|tool_result|>\n错误: 实时查询失败: 未找到地点: 北京最新\n</|tool_result|>")
	current.AddMessage(conversation.RoleAssistant, "<web_search>北京 实时天气</web_search>")

	tc := &agent.ToolCall{
		Name: "web_search",
		Arguments: map[string]interface{}{
			"query": "北京 实时天气",
		},
	}
	if m.shouldSkipDuplicateToolCall(tc) {
		t.Fatal("expected failed web_search result not to be treated as duplicate success")
	}
}

func TestShouldAutoExtractSingleFile_ProjectScaffoldDisabled(t *testing.T) {
	m := &Model{
		runtimePromptProfile: runtimeProfileEngineering,
		turnPrompt:           "使用 nestjs 写一个简单的 api 服务",
	}
	if m.shouldAutoExtractSingleFile() {
		t.Fatal("expected single-file auto extract to be disabled for project scaffold tasks")
	}
}

func TestHasSubstantiveProjectFiles_ManifestOnlyIsNotEnough(t *testing.T) {
	root := t.TempDir()
	paths := []string{
		filepath.Join(root, "package.json"),
		filepath.Join(root, "tsconfig.json"),
	}
	if hasSubstantiveProjectFiles(paths) {
		t.Fatal("expected manifest-only files not to count as substantive project files")
	}
}

func TestHasSubstantiveProjectFiles_SourceFileCounts(t *testing.T) {
	root := t.TempDir()
	paths := []string{
		filepath.Join(root, "package.json"),
		filepath.Join(root, "src", "main.ts"),
	}
	if !hasSubstantiveProjectFiles(paths) {
		t.Fatal("expected source files to count as substantive project files")
	}
}

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

	model, cmd := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyCtrlC})
	got := model.(*Model)
	if cmd != nil {
		t.Fatal("first ctrl+c should not quit")
	}
	if got.textarea.Value() != "" {
		t.Fatalf("expected input cleared, got %q", got.textarea.Value())
	}
	if got.ctrlCPrimedAt.IsZero() {
		t.Fatal("expected ctrl+c exit to be armed")
	}

	_, cmd = got.handleKeyMsg(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("second ctrl+c should return quit cmd")
	}
}

func TestSessionsCommand_ActivatesPicker(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	ta := textarea.New()
	m := &Model{
		cfg:      &config.Config{Model: "test"},
		convMgr:  conversation.NewManager(root),
		textarea: ta,
	}
	defer m.convMgr.Close()

	m.handleCommand("/sessions")
	if !m.pickerActive {
		t.Fatal("expected pickerActive after /sessions")
	}
	if m.picker == nil {
		t.Fatal("expected picker instance to be constructed")
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
		t.Fatalf("expected status bar brand in View, got: %q", v)
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
	if !strings.Contains(v, "思考中") {
		t.Fatalf("expected 思考中 label in View when isThinking, got: %q", v)
	}
}
