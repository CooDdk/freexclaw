package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cloudwego/eino/schema"
	"github.com/mattn/go-runewidth"

	"github.com/CooDdk/freexclaw/internal/agent"
	"github.com/CooDdk/freexclaw/internal/config"
	"github.com/CooDdk/freexclaw/internal/conversation"
	"github.com/CooDdk/freexclaw/internal/llm"
	"github.com/CooDdk/freexclaw/internal/tools"
)

// commandItem 定义一个斜杠命令
type commandItem struct {
	Name string
	Desc string
}

// sessionHistoryReplayLimit 会话切换时最多重放到 scrollback 的可见消息条数
const sessionHistoryReplayLimit = 20

// commandList 可用的斜杠命令列表
var commandList = []commandItem{
	{"/help", "显示帮助"},
	{"/clear", "清空当前对话"},
	{"/new", "新建对话"},
	{"/save", "保存对话"},
	{"/sessions", "查看所有会话"},
	{"/open", "进入指定会话 (如 /open 1)"},
	{"/rename", "重命名当前会话 (如 /rename 新名称)"},
	{"/read", "读取文件内容 (如 /read README.md)"},
	{"/write", "写入文件 (如 /write main.go code...)"},
	{"/ls", "列出目录文件"},
	{"/quit", "退出"},
}

type streamMsg struct {
	content string
	done    bool
	err     error
}

type toolProgressMsg struct {
	text string
}

type toolExecutedMsg struct {
	tc     *agent.ToolCall
	result agent.ToolResult
}

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

const ctrlCExitConfirmWindow = 2 * time.Second

// ModelOptions carries runtime options for NewModel.
type ModelOptions struct {
	Splash bool
	// ResumeID, if non-empty, tells NewModel to resume the given session ID
	// instead of starting a fresh empty conversation.
	ResumeID string
	// Yolo disables per-invocation confirm on run_command. Off by default.
	Yolo bool
}

// pendingTool tracks a tool call currently being executed for spinner display.
type pendingTool struct {
	Name      string
	Arguments map[string]any
	StartedAt time.Time
}

type Model struct {
	cfg          *config.Config
	llmClient    *llm.Client
	convMgr      *conversation.Manager
	textarea     textarea.Model
	width        int
	height       int
	isStreaming  bool
	streamCtx    context.Context
	streamCancel context.CancelFunc
	chunkCh      <-chan llm.StreamChunk
	toolCh       <-chan tea.Msg
	err          error
	isToolRunning bool
	toolStatusText string
	// 命令提示面板
	commandHintVisible bool
	commandHintIndex   int
	// 输入历史记录
	inputHistory     []string
	inputHistoryIdx  int
	inputHistoryTemp string
	runtimePromptProfile string
	runtimePromptSummary string
	turnPrompt         string
	turnTouchedFiles   []string
	turnExecutedCommands []string
	turnSawRunCommand  bool
	turnEngineeringNudged bool
	turnAutoPlanned    bool
	turnAutoToolQueue  []*agent.ToolCall
	ctrlCPrimedAt      time.Time
	// P2 inline migration new fields (Task 7)
	isThinking     bool
	thinkingLabel  string
	tokenCount     int
	streamBuf      strings.Builder
	spinnerTickN   int
	activeToolCall *pendingTool
	pickerActive   bool
	picker         *sessionPicker
	// run_command confirm 门槛：非 Yolo 模式下，run_command 先被这里挡住，
	// 等用户按 y 才真正执行。
	pendingConfirm *pendingCommandConfirm
	yolo           bool
}

func NewModel(cfg *config.Config, opts ModelOptions) (*Model, error) {
	llmClient, err := llm.NewClient(&llm.ClientConfig{
		APIKey:  cfg.APIKey,
		BaseURL: cfg.BaseURL,
		Model:   cfg.Model,
	})
	if err != nil {
		return nil, err
	}

	ta := textarea.New()
	ta.Placeholder = "输入消息... (Enter 发送, Shift+Enter 换行, /help 帮助)"
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	// 换行键：Ctrl+J 和 Shift+Enter
	ta.KeyMap.InsertNewline = key.NewBinding(
		key.WithKeys("ctrl+j", "shift+enter"),
	)
	// 禁用默认上下键行导航，改为历史记录导航
	ta.KeyMap.LinePrevious = key.NewBinding(key.WithKeys(""))
	ta.KeyMap.LineNext = key.NewBinding(key.WithKeys(""))

	m := &Model{
		cfg:       cfg,
		llmClient: llmClient,
		convMgr:   conversation.NewManager(tools.GetWorkDir()),
		textarea:  ta,
		yolo:      opts.Yolo,
	}

	// Session policy: default to a fresh empty conversation on each launch,
	// even when the on-disk manager restored a `current_conversation_id` from
	// the previous run. Historical sessions remain browsable via /sessions,
	// /open, and the --resume CLI flag.
	if opts.ResumeID != "" {
		m.convMgr.SetCurrent(opts.ResumeID)
		if cur := m.convMgr.GetCurrent(); cur == nil || cur.ID != opts.ResumeID {
			return nil, fmt.Errorf("resume: 未找到会话 %s", opts.ResumeID)
		}
	} else {
		m.convMgr.NewConversation()
	}

	m.textarea.Focus()

	return m, nil
}

func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{textarea.Blink}
	if cur := m.convMgr.GetCurrent(); cur != nil && len(cur.Messages) > 0 {
		if replayCmds := m.replaySessionHistoryCmds(cur, sessionHistoryReplayLimit); len(replayCmds) > 0 {
			header := tea.Println(SystemMessageStyle.Render(
				fmt.Sprintf("恢复会话 %q 的历史消息：", cur.Title)))
			cmds = append(cmds, tea.Sequence(append([]tea.Cmd{header}, replayCmds...)...))
		}
	}
	return tea.Batch(cmds...)
}

// CurrentSessionID returns the ID of the active conversation, or "" if none.
// Used by main to print the resume hint after graceful exit.
func (m *Model) CurrentSessionID() string {
	if cur := m.convMgr.GetCurrent(); cur != nil {
		return cur.ID
	}
	return ""
}

// CurrentSessionHasMessages reports whether the active session recorded any
// user/assistant messages during this run — used to decide whether the resume
// hint is worth showing.
func (m *Model) CurrentSessionHasMessages() bool {
	cur := m.convMgr.GetCurrent()
	if cur == nil {
		return false
	}
	for _, msg := range cur.Messages {
		if msg.Role == conversation.RoleUser || msg.Role == conversation.RoleAssistant {
			if strings.TrimSpace(msg.Content) != "" {
				return true
			}
		}
	}
	return false
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textarea.SetWidth(msg.Width)
		return m, nil

	case tickMsg:
		if m.isThinking || m.activeToolCall != nil {
			m.spinnerTickN++
			return m, tickCmd()
		}
		return m, nil

	case streamMsg:
		return m.handleStreamMsg(msg)

	case toolProgressMsg:
		m.toolStatusText = msg.text
		return m, m.waitForToolEvent()

	case toolExecutedMsg:
		return m.handleToolExecuted(msg)

	case sessionPickerSelectedMsg:
		m.pickerActive = false
		m.picker = nil
		m.convMgr.SetCurrent(msg.ID)
		m.convMgr.Save()
		title := "未命名"
		var replayCmds []tea.Cmd
		if cur := m.convMgr.GetCurrent(); cur != nil {
			title = cur.Title
			replayCmds = m.replaySessionHistoryCmds(cur, sessionHistoryReplayLimit)
		}
		switchLine := tea.Println(fmt.Sprintf("%s → 已切换到 %q", MarkerAssistant(), title))
		if len(replayCmds) == 0 {
			return m, switchLine
		}
		return m, tea.Sequence(append([]tea.Cmd{switchLine}, replayCmds...)...)

	case sessionPickerCancelledMsg:
		m.pickerActive = false
		m.picker = nil
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	}

	m.textarea, cmd = m.textarea.Update(msg)
	cmds = append(cmds, cmd)
	// textarea 内容可能变化，更新命令提示面板
	m.updateCommandHint()

	return m, tea.Batch(cmds...)
}

func (m *Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.pendingConfirm != nil {
		return m.handleCommandConfirmKey(msg)
	}
	if m.pickerActive && m.picker != nil {
		var cmd tea.Cmd
		m.picker, cmd = m.picker.Update(msg)
		return m, cmd
	}

	if msg.String() != "ctrl+c" {
		m.clearCtrlCExitState()
	}

	switch msg.String() {
	case "ctrl+c":
		if m.isStreaming {
			m.stopStreaming()
			m.clearCtrlCExitState()
			return m, nil
		}
		if m.shouldQuitOnCtrlC() {
			m.convMgr.Save()
			return m, tea.Quit
		}
		m.prepareCtrlCToClearInput()
		return m, nil

	case "esc":
		m.clearCtrlCExitState()
		// 命令提示面板开启时，esc 关闭提示
		if m.commandHintVisible {
			m.commandHintVisible = false
			m.commandHintIndex = 0
		}
		return m, nil
	}

	// 命令提示面板关闭时，上下键切换输入历史
	if !m.isStreaming && !m.commandHintVisible {
		if msg.String() == "up" {
			m.navigateHistory(-1)
			return m, nil
		}
		if msg.String() == "down" {
			m.navigateHistory(1)
			return m, nil
		}
	}

	switch msg.String() {
	case "enter":
		if !m.isStreaming && !m.isToolRunning {
			return m, m.sendMessage()
		}
		return m, nil

	case "ctrl+j", "shift+enter":
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		return m, cmd
	}

	// 命令提示面板开启时，拦截导航键
	if m.commandHintVisible && !m.isStreaming {
		switch msg.String() {
		case "up":
			matched := m.matchedCommands()
			if len(matched) > 0 {
				m.commandHintIndex--
				if m.commandHintIndex < 0 {
					m.commandHintIndex = len(matched) - 1
				}
			}
			return m, nil
		case "down":
			matched := m.matchedCommands()
			if len(matched) > 0 {
				m.commandHintIndex++
				if m.commandHintIndex >= len(matched) {
					m.commandHintIndex = 0
				}
			}
			return m, nil
		case "tab":
			matched := m.matchedCommands()
			if len(matched) > 0 {
				if m.commandHintIndex >= len(matched) {
					m.commandHintIndex = 0
				}
				m.textarea.SetValue(matched[m.commandHintIndex].Name + " ")
				m.commandHintVisible = false
			}
			return m, nil
		}
	}

	if !m.isStreaming {
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		m.updateCommandHint()
		return m, cmd
	}

	return m, nil
}

func (m *Model) shouldQuitOnCtrlC() bool {
	return !m.ctrlCPrimedAt.IsZero() && time.Since(m.ctrlCPrimedAt) <= ctrlCExitConfirmWindow
}

func (m *Model) prepareCtrlCToClearInput() {
	m.ctrlCPrimedAt = time.Now()
	m.textarea.Focus()
	m.textarea.SetValue("")
	m.commandHintVisible = false
	m.commandHintIndex = 0
	m.inputHistoryTemp = ""
}

func (m *Model) clearCtrlCExitState() {
	if m.ctrlCPrimedAt.IsZero() {
		return
	}
	if time.Since(m.ctrlCPrimedAt) > ctrlCExitConfirmWindow {
		m.ctrlCPrimedAt = time.Time{}
		return
	}
	m.ctrlCPrimedAt = time.Time{}
}

func (m *Model) handleStreamMsg(msg streamMsg) (tea.Model, tea.Cmd) {
	current := m.convMgr.GetCurrent()

	if msg.err != nil {
		m.err = msg.err
		m.isThinking = false
		m.isStreaming = false
		// 流式失败时，删除之前添加的空 assistant 消息，避免下次请求发送空消息给 API
		if len(current.Messages) > 0 {
			last := &current.Messages[len(current.Messages)-1]
			if last.Role == conversation.RoleAssistant && last.Content == "" {
				current.Messages = current.Messages[:len(current.Messages)-1]
			}
		}
		m.streamBuf.Reset()
		m.convMgr.Save()
		return m, tea.Println(MarkerWarn() + " 出错: " + msg.err.Error())
	}

	if msg.done {
		m.isThinking = false
		m.isStreaming = false
		// 最终内容写回会话，供持久化使用
		content := m.streamBuf.String()
		m.streamBuf.Reset()
		if len(current.Messages) > 0 {
			last := &current.Messages[len(current.Messages)-1]
			if last.Role == conversation.RoleAssistant {
				last.Content = content
			}
		}
		// 如果最终内容为空（模型返回空），删除这条空消息
		if len(current.Messages) > 0 {
			last := &current.Messages[len(current.Messages)-1]
			if last.Role == conversation.RoleAssistant && last.Content == "" {
				current.Messages = current.Messages[:len(current.Messages)-1]
			}
		}
		// 检测是否有工具调用
		lastContent := ""
		if len(current.Messages) > 0 {
			lastContent = current.Messages[len(current.Messages)-1].Content
		}
		if tc := agent.ParseToolCall(lastContent); tc != nil {
			m.pruneTrailingToolCallMessage()
			m.convMgr.Save()
			if m.shouldSkipDuplicateToolCall(tc) {
				return m, m.skipDuplicateToolCall(tc)
			}
			return m, m.startToolExecution(tc)
		}

		m.convMgr.Save()

		var cmds []tea.Cmd
		if trimmed := strings.TrimSpace(lastContent); trimmed != "" {
			cmds = append(cmds, tea.Println(renderAssistantMessage(trimmed, m.width)))
		}

		// 如果没有工具调用，尝试自动提取文件内容
		if m.shouldAutoExtractSingleFile() {
			if extracted, filePath, err := agent.AutoExtractFileContent(lastContent); extracted {
				if err != nil {
					current.AddMessage(conversation.RoleSystem, fmt.Sprintf("❌ 自动保存文件失败: %v", err))
				} else {
					m.noteTouchedFile(filePath)
					current.AddMessage(conversation.RoleSystem, fmt.Sprintf("✅ 已自动保存到文件: %s", filePath))
				}
				m.convMgr.Save()
			}
		}

		if cmd := m.maybeContinueEngineeringToolFlow(lastContent); cmd != nil {
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		}

		if cmd := m.maybeRunForcedPostProcess(); cmd != nil {
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		}

		_ = completeTaskSpecHandoffIfNeeded(m.runtimePromptProfile, m.turnTouchedFiles, m.turnExecutedCommands, lastContent)

		return m, tea.Batch(cmds...)
	}

	m.streamBuf.WriteString(msg.content)
	m.tokenCount++
	if len(current.Messages) > 0 {
		last := &current.Messages[len(current.Messages)-1]
		last.Content = m.streamBuf.String()
		current.UpdatedAt = last.CreatedAt
	}
	return m, m.waitForStream()
}

func (m *Model) View() string {
	return m.renderInline()
}

func (m *Model) sendMessage() tea.Cmd {
	content := strings.TrimSpace(m.textarea.Value())
	if content == "" {
		return nil
	}

	if strings.HasPrefix(content, "/") {
		return m.handleCommand(content)
	}

	// 添加到输入历史记录（去重）
	m.addToInputHistory(content)
	m.selectRuntimePrompt(content)
	state, stateErr := loadTaskSpecState()
	awaitingApproval := stateErr == nil && state.Status == specStatusAwaitingApproval && state.Profile == runtimeProfileEngineering
	if awaitingApproval && isExecutionApproval(content) {
		m.runtimePromptProfile = runtimeProfileEngineering
		m.runtimePromptSummary = loadRuntimePrompt(runtimeProfileEngineering)
		_ = markTaskSpecApproved()
	}
	m.resetTurnAutomation(content)

	current := m.convMgr.GetCurrent()
	// 发送新消息前，清理之前的系统消息（命令输出等），保持界面整洁
	m.clearSystemMessages()
	current.AddMessage(conversation.RoleUser, content)
	userLine := renderUserMessage(content)

	if shouldUseEngineeringPlanning(m.runtimePromptProfile, content) && !isExecutionApproval(content) {
		if _, err := ensureTaskSpec(content, m.runtimePromptProfile); err != nil {
			current.AddMessage(conversation.RoleSystem, fmt.Sprintf("❌ 生成内部开发文档失败: %v", err))
		} else {
			current.AddMessage(conversation.RoleSystem, buildPlanningNotice(content))
		}
		m.textarea.SetValue("")
		m.isStreaming = false
		m.err = nil
		m.inputHistoryIdx = len(m.inputHistory)
		m.inputHistoryTemp = ""
		m.convMgr.Save()
		return tea.Println(userLine)
	}

	if awaitingApproval && isExecutionApproval(content) {
		current.AddMessage(conversation.RoleSystem, "已确认 .freexclaw/spec 内部开发文档，开始按步骤执行。")
	}
	current.AddMessage(conversation.RoleAssistant, "")

	m.textarea.SetValue("")
	m.isStreaming = true
	m.err = nil
	m.inputHistoryIdx = len(m.inputHistory)
	m.inputHistoryTemp = ""

	if tc := buildPreflightToolCall(content); tc != nil {
		current.UpdateLastMessage(tcToTag(tc))
		return tea.Sequence(
			tea.Println(userLine),
			tea.Batch(m.startToolExecution(tc), tickCmd()),
		)
	}

	m.streamCtx, m.streamCancel = context.WithCancel(context.Background())

	messages := m.buildMessages()
	m.chunkCh = m.llmClient.StreamChat(m.streamCtx, messages)

	m.isThinking = true
	m.thinkingLabel = "思考中"
	m.tokenCount = 0
	m.streamBuf.Reset()

	return tea.Sequence(tea.Println(userLine), tea.Batch(m.waitForStream(), tickCmd()))
}

func buildPreflightToolCall(content string) *agent.ToolCall {
	if isProjectScaffoldRequest(content) {
		return &agent.ToolCall{
			Name: "list_dir",
			Arguments: map[string]interface{}{
				"path": ".",
			},
		}
	}

	match := tools.MatchLiveQuery(content, tools.GetCurrentLiveQueryContext())
	if match.Domain == "generic_search" {
		return nil
	}

	if match.Domain == "weather" && strings.TrimSpace(match.Location) == "" {
		return nil
	}

	query := canonicalizePreflightQuery(content, match)

	return &agent.ToolCall{
		Name: "web_search",
		Arguments: map[string]interface{}{
			"query": query,
		},
	}
}

func tcToTag(tc *agent.ToolCall) string {
	if tc == nil {
		return ""
	}

	if tc.Name == "web_search" {
		if query, ok := tc.Arguments["query"].(string); ok {
			return fmt.Sprintf("<web_search>%s</web_search>", query)
		}
	}
	if tc.Name == "list_dir" {
		if path, ok := tc.Arguments["path"].(string); ok {
			return fmt.Sprintf("<list_dir>%s</list_dir>", path)
		}
	}

	return ""
}

// parseToolCallTag decodes an assistant message that stores a preflight tool call
// as `<toolname>arg</toolname>`. Returns ok=false for anything else.
func parseToolCallTag(content string) (name string, arg string, ok bool) {
	content = strings.TrimSpace(content)
	if len(content) < 3 || content[0] != '<' {
		return "", "", false
	}
	end := strings.IndexByte(content, '>')
	if end < 2 {
		return "", "", false
	}
	tag := content[1:end]
	// Reject non-tag chars
	for _, r := range tag {
		if r == '/' || r == ' ' || r == '<' || r == '>' {
			return "", "", false
		}
	}
	closeTag := "</" + tag + ">"
	if !strings.HasSuffix(content, closeTag) {
		return "", "", false
	}
	return tag, content[end+1 : len(content)-len(closeTag)], true
}

// replaySessionHistoryCmds returns tea.Println commands that reprint the last
// `limit` visible messages of sess into scrollback. System messages and hidden
// tool-result messages are skipped. Prepends a "(省略 N 条更早的消息)" line
// when older messages are truncated.
func (m *Model) replaySessionHistoryCmds(sess *conversation.Conversation, limit int) []tea.Cmd {
	if sess == nil || len(sess.Messages) == 0 {
		return nil
	}
	kept := make([]conversation.Message, 0, len(sess.Messages))
	for i, msg := range sess.Messages {
		if msg.Role == conversation.RoleSystem {
			continue
		}
		if msg.Role == conversation.RoleUser && shouldHideToolResultInContext(sess.Messages, i) {
			continue
		}
		if strings.TrimSpace(msg.Content) == "" {
			continue
		}
		kept = append(kept, msg)
	}
	if len(kept) == 0 {
		return nil
	}
	omitted := 0
	if limit > 0 && len(kept) > limit {
		omitted = len(kept) - limit
		kept = kept[len(kept)-limit:]
	}
	var cmds []tea.Cmd
	if omitted > 0 {
		cmds = append(cmds, tea.Println(SystemMessageStyle.Render(
			fmt.Sprintf("(省略 %d 条更早的消息)", omitted))))
	}
	for _, msg := range kept {
		line := m.renderHistoryMessage(msg)
		if line == "" {
			continue
		}
		cmds = append(cmds, tea.Println(line))
	}
	return cmds
}

func (m *Model) renderHistoryMessage(msg conversation.Message) string {
	switch msg.Role {
	case conversation.RoleUser:
		if strings.HasPrefix(msg.Content, "<|tool_result|>") {
			return ""
		}
		return renderUserMessage(msg.Content)
	case conversation.RoleAssistant:
		if name, arg, ok := parseToolCallTag(msg.Content); ok {
			return renderHistoryToolCall(name, arg)
		}
		return renderAssistantMessage(msg.Content, m.width)
	}
	return ""
}

func renderHistoryToolCall(name, arg string) string {
	argStr := arg
	if argStr != "" {
		argStr = fmt.Sprintf("%q", arg)
	}
	return fmt.Sprintf("%s %s(%s)  %s",
		MarkerToolStart(),
		name,
		argStr,
		CommandHintDescStyle.Render("(历史)"),
	)
}

// previewFileContent returns the first `maxLines` lines (or `maxBytes` bytes,
// whichever comes first) of content. truncated is true when content was cut.
// totalLines is the total line count of the original content.
func previewFileContent(content string, maxLines, maxBytes int) (preview string, truncated bool, totalLines int) {
	if content == "" {
		return "", false, 0
	}
	totalLines = strings.Count(content, "\n")
	if !strings.HasSuffix(content, "\n") {
		totalLines++
	}
	lines := strings.SplitAfter(content, "\n")
	var b strings.Builder
	for i, line := range lines {
		if i >= maxLines {
			truncated = true
			break
		}
		if b.Len()+len(line) > maxBytes && b.Len() > 0 {
			truncated = true
			break
		}
		b.WriteString(line)
	}
	preview = strings.TrimRight(b.String(), "\n")
	return preview, truncated, totalLines
}

func previewLineCount(preview string) int {
	if preview == "" {
		return 0
	}
	return strings.Count(preview, "\n") + 1
}

func canonicalizePreflightQuery(content string, match tools.MatchResult) string {
	if match.Domain == "weather" {
		location := strings.TrimSpace(match.Location)
		timeOfDay := strings.TrimSpace(match.TimeOfDay)
		if location != "" {
			// 优先保留相对日期这个用户原话里的强信号（"明天/后天/今天"），比 N 天更贴意图
			for _, day := range []string{"大后天", "后天", "明天", "今天"} {
				if strings.Contains(content, day) {
					return fmt.Sprintf("%s %s 天气预报", location, day)
				}
			}
		}
		if location != "" && match.ForecastDays > 1 {
			return fmt.Sprintf("%s %d天 天气预报", location, match.ForecastDays)
		}
		if location != "" && timeOfDay != "" {
			return fmt.Sprintf("%s %s 天气", location, timeOfDay)
		}
		if location != "" {
			return fmt.Sprintf("%s 实时天气", location)
		}
	}

	return content
}

func shouldHideToolResult(content string) bool {
	if !strings.HasPrefix(content, "<|tool_result|>") || !strings.HasSuffix(content, "</|tool_result|>") {
		return false
	}

	resultContent := strings.TrimPrefix(content, "<|tool_result|>")
	resultContent = strings.TrimSuffix(resultContent, "</|tool_result|>")
	resultContent = strings.TrimSpace(resultContent)

	if resultContent == "" {
		return false
	}
	if strings.Contains(resultContent, "错误:") {
		return false
	}
	if strings.Contains(resultContent, "已跳过重复的 web_search 调用") {
		return true
	}
	if strings.Contains(resultContent, "搜索关键词:") {
		return true
	}

	return false
}

func shouldHideToolResultInContext(messages []conversation.Message, index int) bool {
	if index < 0 || index >= len(messages) {
		return false
	}

	content := messages[index].Content
	if shouldHideToolResult(content) {
		return true
	}

	resultContent := strings.TrimPrefix(content, "<|tool_result|>")
	resultContent = strings.TrimSuffix(resultContent, "</|tool_result|>")
	resultContent = strings.TrimSpace(resultContent)
	if !strings.Contains(resultContent, "错误:") {
		return false
	}

	for i := index + 1; i < len(messages); i++ {
		msg := messages[i]
		if msg.Role == conversation.RoleUser {
			break
		}
		if msg.Role != conversation.RoleAssistant {
			continue
		}
		assistantContent := strings.TrimSpace(msg.Content)
		if assistantContent == "" {
			continue
		}
		if agent.ParseToolCall(assistantContent) != nil {
			continue
		}
		return true
	}

	return false
}

func (m *Model) handleCommand(cmd string) tea.Cmd {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return nil
	}

	var cmds []tea.Cmd
	emit := func(s string) {
		cmds = append(cmds, tea.Println(s))
	}

	// Always clear the input textarea at the end of a command
	defer func() {
		m.textarea.SetValue("")
		m.commandHintVisible = false
		m.commandHintIndex = 0
	}()

	current := m.convMgr.GetCurrent()

	switch parts[0] {
	case "/help":
		helpText := `可用命令:
  /help          - 显示帮助
  /clear         - 清空当前对话
  /new           - 新建对话
  /save          - 保存对话
  /sessions      - 查看所有会话
  /open <序号>   - 进入指定会话
  /read <文件>   - 读取文件内容（会加入对话上下文）
  /write <文件> <内容> - 写入文件（-a 参数追加）
  /ls [目录]     - 列出目录文件
  /quit          - 退出

快捷键:
  Enter          - 发送消息
  Shift+Enter    - 换行（也可用 Ctrl+J）
  Esc            - 切换焦点（输入框 ↔ 聊天区）
  j/k            - 聊天区上下滚动
  Ctrl+u/d       - 聊天区上下翻页
  g/G            - 聊天区跳到顶部/底部
  Ctrl+C         - 退出或停止生成

文件操作示例:
  /read main.go          - 读取并显示 main.go
  /write index.html <html>...</html> - 写入 HTML 文件
  /write -a log.txt 追加内容 - 追加到文件
  /ls                    - 列出当前目录
  /ls src/               - 列出 src 目录`
		emit(helpText)

	case "/clear":
		current.Messages = nil
		m.err = nil
		emit(MarkerAssistant() + " 已清空当前会话")

	case "/new":
		m.convMgr.NewConversation()
		m.convMgr.Save()
		emit(MarkerAssistant() + " 已新建会话")

	case "/save":
		if err := m.convMgr.Save(); err != nil {
			m.err = err
			emit(MarkerToolFail() + " 保存失败: " + err.Error())
		} else {
			emit(MarkerToolOK() + " 对话已保存")
		}

	case "/sessions":
		items := make([]pickerItem, 0)
		curID := ""
		if cur := m.convMgr.GetCurrent(); cur != nil {
			curID = cur.ID
		}
		for _, c := range m.convMgr.List() {
			items = append(items, pickerItemFromConversation(c, c.ID == curID))
		}
		m.picker = newSessionPicker(items)
		m.pickerActive = true

	case "/open":
		if len(parts) < 2 {
			emit(MarkerToolFail() + " 用法: /open <序号>（输入 /sessions 查看会话列表）")
			break
		}
		idx, err := strconv.Atoi(parts[1])
		if err != nil || idx < 1 {
			emit(MarkerToolFail() + " 序号必须是正整数")
			break
		}
		convs := m.convMgr.List()
		if idx > len(convs) {
			emit(MarkerToolFail() + fmt.Sprintf(" 序号超出范围，共 %d 个会话", len(convs)))
			break
		}
		target := convs[idx-1]
		m.convMgr.SetCurrent(target.ID)
		emit(MarkerAssistant() + fmt.Sprintf(" → 已切换到 %q", target.Title))
		if cur := m.convMgr.GetCurrent(); cur != nil {
			for _, c := range m.replaySessionHistoryCmds(cur, sessionHistoryReplayLimit) {
				cmds = append(cmds, c)
			}
		}

	case "/rename":
		if len(parts) < 2 {
			emit(MarkerToolFail() + " 用法: /rename <新名称>（示例: /rename Python 脚本开发）")
			break
		}
		newTitle := strings.Join(parts[1:], " ")
		current := m.convMgr.GetCurrent()
		oldTitle := current.Title
		current.Title = newTitle
		current.UpdatedAt = time.Now()
		if err := m.convMgr.Save(); err != nil {
			emit(MarkerToolFail() + fmt.Sprintf(" 重命名失败: %v", err))
		} else {
			emit(MarkerAssistant() + fmt.Sprintf(" 会话已重命名: %q → %q", oldTitle, newTitle))
		}

	case "/read":
		if len(parts) < 2 {
			emit(MarkerToolFail() + " 用法: /read <文件路径>（示例: /read main.go）")
			break
		}
		path := strings.Join(parts[1:], " ")
		fc, err := tools.ReadFile(path)
		if err != nil {
			emit(MarkerToolFail() + fmt.Sprintf(" 读取失败: %v", err))
		} else {
			current.AddMessage(conversation.RoleUser, fmt.Sprintf("请分析这个文件的内容:\n\n%s", tools.FormatFileContent(fc)))
			preview, truncated, totalLines := previewFileContent(fc.Content, 60, 6000)
			emit(fmt.Sprintf("%s %s (%d 行)", MarkerAssistant(), path, totalLines))
			emit(preview)
			if truncated {
				emit(SystemMessageStyle.Render(
					fmt.Sprintf("(仅显示前 %d 行预览，完整内容已加入下一轮上下文)", previewLineCount(preview))))
			}
			emit(MarkerToolOK() + " 已读取 " + path + "（内容已加入下一轮上下文）")
		}

	case "/write":
		if len(parts) < 3 {
			emit(MarkerToolFail() + " 用法: /write <文件路径> <内容>（示例: /write hello.txt Hello World；使用 -a 追加）")
			break
		}
		args := strings.Join(parts[1:], " ")
		path, content, appendMode, err := tools.ParseWriteArgs(args)
		if err != nil {
			emit(MarkerToolFail() + fmt.Sprintf(" 参数错误: %v", err))
			break
		}
		if err := tools.WriteFile(path, content+"\n", appendMode); err != nil {
			emit(MarkerToolFail() + fmt.Sprintf(" 写入失败: %v", err))
		} else {
			if appendMode {
				emit(MarkerToolOK() + fmt.Sprintf(" 已追加到 %s", path))
			} else {
				emit(MarkerToolOK() + fmt.Sprintf(" 已写入 %s", path))
			}
			// 读取刚写入的文件，方便继续编辑
			if fc, readErr := tools.ReadFile(path); readErr == nil {
				current.AddMessage(conversation.RoleUser, fmt.Sprintf("文件内容已更新，继续编辑:\n\n%s", tools.FormatFileContent(fc)))
			}
		}

	case "/ls":
		path := "."
		if len(parts) >= 2 {
			path = strings.Join(parts[1:], " ")
		}
		files, err := tools.ListDir(path)
		if err != nil {
			emit(MarkerToolFail() + fmt.Sprintf(" 读取目录失败: %v", err))
		} else {
			var sb strings.Builder
			sb.WriteString(MarkerAssistant() + fmt.Sprintf(" 目录: %s\n", path))
			for _, f := range files {
				sb.WriteString("  " + f + "\n")
			}
			if len(files) == 0 {
				sb.WriteString("  (空目录)\n")
			}
			emit(strings.TrimRight(sb.String(), "\n"))
		}

	case "/quit":
		m.convMgr.Save()
		return tea.Quit

	default:
		emit(MarkerToolFail() + fmt.Sprintf(" 未知命令: %s（输入 /help 查看可用命令）", parts[0]))
	}

	if len(cmds) == 0 {
		return nil
	}
	if len(cmds) == 1 {
		return cmds[0]
	}
	return tea.Sequence(cmds...)
}

func (m *Model) waitForStream() tea.Cmd {
	return func() tea.Msg {
		chunk, ok := <-m.chunkCh
		if !ok {
			return streamMsg{done: true}
		}
		if chunk.Err != nil {
			return streamMsg{err: chunk.Err}
		}
		if chunk.Done {
			return streamMsg{done: true}
		}
		return streamMsg{content: chunk.Content}
	}
}

func (m *Model) stopStreaming() {
	if m.streamCancel != nil {
		m.streamCancel()
	}
	m.isStreaming = false
	m.isThinking = false
	m.streamBuf.Reset()
	m.tokenCount = 0
	m.activeToolCall = nil
	m.isToolRunning = false
}

func (m *Model) buildMessages() []*schema.Message {
	current := m.convMgr.GetCurrent()
	messages := make([]*schema.Message, 0)

	now := time.Now()
	dateInfo := "当前时间：" + now.Format("2006-01-02 15:04:05") +
		"，星期" + []string{"日", "一", "二", "三", "四", "五", "六"}[now.Weekday()] +
		"，时区：" + now.Location().String() + "。"

	agentPrompt := agent.SystemPrompt()
	if m.cfg.SystemPrompt != "" {
		agentPrompt = dateInfo + "\n\n" + m.cfg.SystemPrompt + "\n\n" + agentPrompt
	} else {
		agentPrompt = dateInfo + "\n\n" + agentPrompt
	}
	messages = append(messages, &schema.Message{
		Role:    schema.System,
		Content: agentPrompt,
	})
	if runtimeMsg := buildRuntimeSystemMessage(m.runtimePromptProfile, m.runtimePromptSummary); runtimeMsg != "" {
		messages = append(messages, &schema.Message{
			Role:    schema.System,
			Content: runtimeMsg,
		})
	}
	if deliveryMsg := buildDeliverySystemMessage(m.runtimePromptProfile, m.turnTouchedFiles, m.turnExecutedCommands); deliveryMsg != "" {
		messages = append(messages, &schema.Message{
			Role:    schema.System,
			Content: deliveryMsg,
		})
	}
	if specMsg := buildTaskSpecExecutionMessage(m.runtimePromptProfile); specMsg != "" {
		messages = append(messages, &schema.Message{
			Role:    schema.System,
			Content: specMsg,
		})
	}

	for i, msg := range current.Messages {
		if i == len(current.Messages)-1 && msg.Role == conversation.RoleAssistant && msg.Content == "" {
			continue
		}

		role := schema.User
		switch msg.Role {
		case conversation.RoleUser:
			role = schema.User
		case conversation.RoleAssistant:
			role = schema.Assistant
		case conversation.RoleSystem:
			role = schema.System
		}

		messages = append(messages, &schema.Message{
			Role:    role,
			Content: msg.Content,
		})
	}

	return messages
}

func (m *Model) selectRuntimePrompt(content string) {
	profile := detectRuntimePromptProfile(content, m.convMgr.GetCurrent().Messages)
	if profile == "" {
		m.runtimePromptProfile = ""
		m.runtimePromptSummary = ""
		return
	}

	m.runtimePromptProfile = profile
	m.runtimePromptSummary = loadRuntimePrompt(profile)
}

// updateCommandHint 根据输入框当前值更新命令提示面板状态，返回可见性是否有变化
func (m *Model) updateCommandHint() bool {
	oldVisible := m.commandHintVisible
	val := m.textarea.Value()
	// 只在单行且以 "/" 开头时显示命令提示
	if strings.HasPrefix(val, "/") && !strings.Contains(val, "\n") {
		matched := m.matchedCommands()
		if len(matched) > 0 {
			m.commandHintVisible = true
			if m.commandHintIndex >= len(matched) {
				m.commandHintIndex = 0
			}
		} else {
			m.commandHintVisible = false
			m.commandHintIndex = 0
		}
	} else {
		m.commandHintVisible = false
		m.commandHintIndex = 0
	}
	return oldVisible != m.commandHintVisible
}

// matchedCommands 根据输入框当前值过滤匹配的命令
func (m *Model) matchedCommands() []commandItem {
	val := m.textarea.Value()
	if val == "/" {
		return commandList
	}
	var matched []commandItem
	for _, cmd := range commandList {
		if strings.HasPrefix(cmd.Name, val) {
			matched = append(matched, cmd)
		}
	}
	return matched
}

// clearSystemMessages 清除对话中所有的系统消息（命令输出、提示信息等）
func (m *Model) clearSystemMessages() {
	current := m.convMgr.GetCurrent()
	filtered := current.Messages[:0]
	for _, msg := range current.Messages {
		if msg.Role != conversation.RoleSystem {
			filtered = append(filtered, msg)
		}
	}
	current.Messages = filtered
}

// addToInputHistory 添加消息到输入历史记录（去重）
func (m *Model) addToInputHistory(content string) {
	// 去重：如果最后一条相同则不添加
	if len(m.inputHistory) > 0 && m.inputHistory[len(m.inputHistory)-1] == content {
		return
	}
	m.inputHistory = append(m.inputHistory, content)
	m.inputHistoryIdx = len(m.inputHistory)
}

// navigateHistory 导航输入历史记录（-1: 向上，1: 向下）
func (m *Model) navigateHistory(direction int) {
	if len(m.inputHistory) == 0 {
		return
	}

	// 首次向上时，保存当前输入作为临时值
	if m.inputHistoryIdx == len(m.inputHistory) && direction == -1 {
		m.inputHistoryTemp = m.textarea.Value()
	}

	// 导航
	m.inputHistoryIdx += direction

	// 边界检查
	if m.inputHistoryIdx < 0 {
		m.inputHistoryIdx = 0
	} else if m.inputHistoryIdx > len(m.inputHistory) {
		m.inputHistoryIdx = len(m.inputHistory)
	}

	// 设置输入框内容并将光标移到末尾
	var newVal string
	if m.inputHistoryIdx == len(m.inputHistory) {
		newVal = m.inputHistoryTemp
	} else {
		newVal = m.inputHistory[m.inputHistoryIdx]
	}
	m.textarea.SetValue(newVal)
	// 将光标移动到末尾
	m.textarea.CursorEnd()
}

// executeToolAndContinue 执行工具调用并启动下一轮流式生成
func (m *Model) startToolExecution(tc *agent.ToolCall) tea.Cmd {
	// 非 Yolo 模式下，run_command 先挂起等待用户按 y/n 确认。
	if needsCommandConfirm(tc, m.yolo) {
		m.pendingConfirm = &pendingCommandConfirm{tc: tc}
		return tea.Println(renderCommandConfirmPrompt(tc))
	}
	return m.doStartToolExecution(tc)
}

// doStartToolExecution 是绕过 confirm 的真正执行入口。
func (m *Model) doStartToolExecution(tc *agent.ToolCall) tea.Cmd {
	m.isToolRunning = true
	m.toolStatusText = describeToolExecution(tc)
	m.activeToolCall = &pendingTool{
		Name:      tc.Name,
		Arguments: tc.Arguments,
		StartedAt: time.Now(),
	}
	m.convMgr.Save()
	m.toolCh = runToolAsync(tc)
	return m.waitForToolEvent()
}

// handleCommandConfirmKey 在 pendingConfirm 激活时接管键盘：
// y/Y 放行、n/N/esc 拒绝，其余键忽略保持提示。
func (m *Model) handleCommandConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	pc := m.pendingConfirm
	if pc == nil {
		return m, nil
	}
	switch msg.String() {
	case "y", "Y":
		m.pendingConfirm = nil
		return m, m.doStartToolExecution(pc.tc)
	case "n", "N", "esc":
		m.pendingConfirm = nil
		rejected := commandRejectedResult()
		tc := pc.tc
		return m, tea.Batch(
			tea.Println(SystemMessageStyle.Render("✗ 已拒绝执行该命令")),
			func() tea.Msg { return toolExecutedMsg{tc: tc, result: rejected} },
		)
	}
	return m, nil
}

func (m *Model) handleToolExecuted(msg toolExecutedMsg) (tea.Model, tea.Cmd) {
	current := m.convMgr.GetCurrent()
	tc := msg.tc
	result := msg.result
	m.isToolRunning = false
	m.toolStatusText = ""

	// Capture display info from activeToolCall BEFORE clearing
	var toolLine string
	if m.activeToolCall != nil {
		durationMS := int(time.Since(m.activeToolCall.StartedAt) / time.Millisecond)
		summary := result.Output
		if !result.Success && result.Error != "" {
			summary = result.Error
		}
		if len(summary) > 200 {
			summary = summary[:200] + "..."
		}
		toolLine = renderToolCall(m.activeToolCall.Name, m.activeToolCall.Arguments, summary, result.Success, durationMS)
	}
	m.activeToolCall = nil

	if tc.Name == "run_command" {
		m.turnSawRunCommand = true
	}
	if result.Success && (tc.Name == "write_file" || tc.Name == "append_file") {
		if path, ok := tc.Arguments["path"].(string); ok {
			m.noteTouchedFile(path)
		}
	}
	if tc.Name == "run_command" {
		if command, ok := tc.Arguments["command"].(string); ok {
			m.noteExecutedCommand(command)
		}
	}
	commandText, _ := tc.Arguments["command"].(string)
	if !result.Success {
		failureTarget := commandText
		if failureTarget == "" && (tc.Name == "write_file" || tc.Name == "append_file") {
			failureTarget, _ = tc.Arguments["path"].(string)
		}
		_ = recordTaskSpecFailure(tc.Name, failureTarget, result.Error)
	}
	_ = updateTaskSpecProgressForTool(tc.Name, result.Success, m.turnTouchedFiles, commandText)

	// 将工具结果作为用户消息添加到对话中（以 tool_result 格式）
	toolResultMsg := agent.BuildToolResultMessage(tc, result)
	current.AddMessage(conversation.RoleUser, toolResultMsg)

	// 添加新的空 assistant 消息
	current.AddMessage(conversation.RoleAssistant, "")

	m.isStreaming = true
	m.isThinking = true
	m.thinkingLabel = "思考中"
	m.tokenCount = 0
	m.streamBuf.Reset()
	m.err = nil
	m.convMgr.Save()

	// 启动下一轮流式
	m.streamCtx, m.streamCancel = context.WithCancel(context.Background())
	messages := m.buildMessages()
	m.chunkCh = m.llmClient.StreamChat(m.streamCtx, messages)

	continueCmd := m.waitForStream()
	if toolLine == "" {
		return m, continueCmd
	}
	return m, tea.Sequence(tea.Println(toolLine), continueCmd)
}

func (m *Model) pruneTrailingToolCallMessage() {
	current := m.convMgr.GetCurrent()
	if len(current.Messages) == 0 {
		return
	}

	last := current.Messages[len(current.Messages)-1]
	if last.Role != conversation.RoleAssistant {
		return
	}
	if agent.ParseToolCall(last.Content) == nil {
		return
	}

	current.Messages = current.Messages[:len(current.Messages)-1]
}

func (m *Model) waitForToolEvent() tea.Cmd {
	return func() tea.Msg {
		if m.toolCh == nil {
			return nil
		}
		msg, ok := <-m.toolCh
		if !ok {
			return nil
		}
		return msg
	}
}

func (m *Model) resetTurnAutomation(prompt string) {
	m.turnPrompt = prompt
	m.turnTouchedFiles = nil
	m.turnExecutedCommands = nil
	m.turnSawRunCommand = false
	m.turnEngineeringNudged = false
	m.turnAutoPlanned = false
	m.turnAutoToolQueue = nil
}

func (m *Model) shouldAutoExtractSingleFile() bool {
	if strings.TrimSpace(m.runtimePromptProfile) == runtimeProfileEngineering && isProjectScaffoldRequest(m.turnPrompt) {
		return false
	}
	return true
}

func (m *Model) maybeContinueEngineeringToolFlow(lastContent string) tea.Cmd {
	if strings.TrimSpace(m.runtimePromptProfile) != runtimeProfileEngineering {
		return nil
	}
	if !isProjectScaffoldRequest(m.turnPrompt) {
		return nil
	}
	if m.turnEngineeringNudged || len(m.turnExecutedCommands) > 0 {
		return nil
	}
	if strings.TrimSpace(lastContent) == "" {
		return nil
	}

	hasSubstantiveFiles := hasSubstantiveProjectFiles(m.turnTouchedFiles)
	if hasSubstantiveFiles {
		return nil
	}

	current := m.convMgr.GetCurrent()
	current.AddMessage(conversation.RoleSystem, "你还没有真正创建项目源码文件。当前如果只写了 package.json、tsconfig.json、nest-cli.json 这类清单/配置文件，仍然不算完成脚手架。请继续，仅使用 <list_dir>/<read_file>/<write_file>/<append_file>/<run_command> 工具，创建实际源码文件（例如 src/main.ts、src/app.module.ts、controller、service 等）和必要目录，不要重复改写同一个 package.json 或 tsconfig.json。")
	current.AddMessage(conversation.RoleAssistant, "")
	m.turnEngineeringNudged = true
	m.isStreaming = true
	m.isThinking = true
	m.thinkingLabel = "思考中"
	m.tokenCount = 0
	m.streamBuf.Reset()
	m.err = nil
	m.convMgr.Save()

	m.streamCtx, m.streamCancel = context.WithCancel(context.Background())
	messages := m.buildMessages()
	m.chunkCh = m.llmClient.StreamChat(m.streamCtx, messages)
	return m.waitForStream()
}

func (m *Model) noteTouchedFile(path string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(tools.GetWorkDir(), path)
	}
	path = filepath.Clean(path)
	for _, existing := range m.turnTouchedFiles {
		if strings.EqualFold(existing, path) {
			return
		}
	}
	m.turnTouchedFiles = append(m.turnTouchedFiles, path)
}

func (m *Model) noteExecutedCommand(command string) {
	command = strings.TrimSpace(command)
	if command == "" {
		return
	}
	for _, existing := range m.turnExecutedCommands {
		if existing == command {
			return
		}
	}
	m.turnExecutedCommands = append(m.turnExecutedCommands, command)
}

func (m *Model) maybeRunForcedPostProcess() tea.Cmd {
	if m.turnSawRunCommand {
		return nil
	}

	if !m.turnAutoPlanned {
		m.turnAutoPlanned = true
		m.turnAutoToolQueue = buildForcedPostProcessToolCalls(m.turnPrompt, m.turnTouchedFiles)
		if len(m.turnAutoToolQueue) > 0 {
			current := m.convMgr.GetCurrent()
			current.AddMessage(conversation.RoleSystem, "🛠️ 检测到项目生成任务，开始自动执行初始化与基础校验...")
			m.convMgr.Save()
		}
	}

	if len(m.turnAutoToolQueue) == 0 {
		return nil
	}

	next := m.turnAutoToolQueue[0]
	m.turnAutoToolQueue = m.turnAutoToolQueue[1:]
	return m.startToolExecution(next)
}

func (m *Model) shouldSkipDuplicateToolCall(tc *agent.ToolCall) bool {
	if tc == nil || tc.Name != "web_search" {
		return false
	}
	query, _ := tc.Arguments["query"].(string)
	query = strings.TrimSpace(query)
	if query == "" {
		return false
	}

	current := m.convMgr.GetCurrent()
	for i := len(current.Messages) - 2; i >= 0; i-- {
		msg := current.Messages[i]
		if msg.Role == conversation.RoleUser &&
			strings.HasPrefix(msg.Content, "<|tool_result|>") &&
			!strings.Contains(msg.Content, "错误:") &&
			strings.Contains(msg.Content, "搜索关键词: "+query) {
			return true
		}
	}
	return false
}

func (m *Model) skipDuplicateToolCall(tc *agent.ToolCall) tea.Cmd {
	current := m.convMgr.GetCurrent()
	if len(current.Messages) > 0 {
		last := current.Messages[len(current.Messages)-1]
		if last.Role == conversation.RoleAssistant {
			current.Messages = current.Messages[:len(current.Messages)-1]
		}
	}

	current.AddMessage(conversation.RoleUser, "<|tool_result|>\n已跳过重复的 web_search 调用，请直接基于已有查询结果继续回答，不要再次调用相同查询。\n</|tool_result|>")
	current.AddMessage(conversation.RoleAssistant, "")
	m.isStreaming = true
	m.isThinking = true
	m.thinkingLabel = "思考中"
	m.tokenCount = 0
	m.streamBuf.Reset()
	m.err = nil
	m.convMgr.Save()

	m.streamCtx, m.streamCancel = context.WithCancel(context.Background())
	messages := m.buildMessages()
	m.chunkCh = m.llmClient.StreamChat(m.streamCtx, messages)
	return m.waitForStream()
}

func runToolAsync(tc *agent.ToolCall) <-chan tea.Msg {
	ch := make(chan tea.Msg)
	go func() {
		defer close(ch)
		result := agent.ExecuteToolWithProgress(tc, func(text string) {
			ch <- toolProgressMsg{text: text}
		})
		ch <- toolExecutedMsg{tc: tc, result: result}
	}()
	return ch
}

func describeToolExecution(tc *agent.ToolCall) string {
	if tc == nil {
		return ""
	}
	switch tc.Name {
	case "web_search":
		if query, ok := tc.Arguments["query"].(string); ok && strings.TrimSpace(query) != "" {
			return "正在查询：" + query
		}
		return "正在查询实时信息"
	case "run_command":
		if command, ok := tc.Arguments["command"].(string); ok && strings.TrimSpace(command) != "" {
			return "正在执行命令：" + command
		}
		return "正在执行命令"
	case "read_file", "write_file", "append_file", "list_dir":
		if path, ok := tc.Arguments["path"].(string); ok && strings.TrimSpace(path) != "" {
			return "正在处理：" + path
		}
	}
	return "正在处理工具请求"
}

// renderInline is the new inline scrollback View renderer. It renders only the
// currently-active UI (textarea, spinner, tool line, status bar). Historical
// messages are pushed to scrollback via tea.Println by callers.
// NOT YET WIRED to View(); Task 9 does the switch.
func (m *Model) renderInline() string {
	if m.pickerActive && m.picker != nil {
		return m.picker.View() + "\n" + m.renderStatusBarInline()
	}

	var parts []string
	parts = append(parts, m.renderInputDividerInline())
	parts = append(parts, m.textarea.View())

	if hint := m.renderCommandHintInline(); hint != "" {
		parts = append(parts, hint)
	}
	if m.isThinking {
		parts = append(parts, m.renderSpinnerLine())
	}
	if m.activeToolCall != nil {
		parts = append(parts, m.renderToolCallLine())
	}
	parts = append(parts, m.renderStatusBarInline())
	return strings.Join(parts, "\n")
}

// renderInputDividerInline draws a subtle labeled rule above the textarea so
// the input zone is visually separated from the scrollback message area.
// Format: `╭─ ✎ 输入 ──────────────────────────`
func (m *Model) renderInputDividerInline() string {
	w := m.width
	if w <= 0 {
		w = 80
	}
	label := " ✎ 输入 "
	prefix := "╭─"
	ruleStyle := lipgloss.NewStyle().Foreground(MutedColor).Faint(true)
	labelStyle := lipgloss.NewStyle().Foreground(UserColor).Bold(true)
	labelW := runewidth.StringWidth(label)
	prefixW := runewidth.StringWidth(prefix)
	remain := w - prefixW - labelW
	if remain < 0 {
		// Terminal too narrow — fall back to a plain rule.
		return ruleStyle.Render(strings.Repeat("─", w))
	}
	return ruleStyle.Render(prefix) + labelStyle.Render(label) + ruleStyle.Render(strings.Repeat("─", remain))
}

// renderCommandHintInline draws the slash-command completion dropdown just below
// the textarea when the user is typing a "/" command. Returns "" when hidden.
func (m *Model) renderCommandHintInline() string {
	if !m.commandHintVisible {
		return ""
	}
	matched := m.matchedCommands()
	if len(matched) == 0 {
		return ""
	}
	const maxRows = 8
	sel := m.commandHintIndex
	if sel < 0 {
		sel = 0
	}
	if sel >= len(matched) {
		sel = len(matched) - 1
	}
	if len(matched) > maxRows {
		start := 0
		if sel >= maxRows {
			start = sel - maxRows + 1
		}
		matched = matched[start : start+maxRows]
		sel -= start
	}
	maxName := 0
	for _, c := range matched {
		if n := len(c.Name); n > maxName {
			maxName = n
		}
	}
	var lines []string
	for i, c := range matched {
		nameCol := c.Name + strings.Repeat(" ", maxName-len(c.Name))
		var line string
		if i == sel {
			line = CommandHintSelectedItemStyle.Render(nameCol) + "  " + CommandHintDescStyle.Render(c.Desc)
		} else {
			line = CommandHintItemStyle.Render(nameCol) + "  " + CommandHintDescStyle.Render(c.Desc)
		}
		lines = append(lines, line)
	}
	footer := CommandHintDescStyle.Render("↑↓ 选择 · Tab/Enter 补全 · Esc 取消")
	body := strings.Join(lines, "\n") + "\n" + footer
	return CommandHintStyle.Render(body)
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
	if m.activeToolCall == nil {
		return ""
	}
	frame := BrandSpinnerFrames[m.spinnerTickN%len(BrandSpinnerFrames)]
	args := formatToolArgs(m.activeToolCall.Arguments)
	head := fmt.Sprintf("%s %s(%s)", MarkerToolStart(), m.activeToolCall.Name, args)
	sub := fmt.Sprintf("  %s 执行中...",
		lipgloss.NewStyle().Foreground(AssistantColor).Render(frame))
	return head + "\n" + sub
}

// renderStatusBarInline draws the branded two-line status bar:
// a gradient rule on top and an icon-separated info line below.
func (m *Model) renderStatusBarInline() string {
	sep := " │ "
	title := "会话1"
	msgCount := 0
	if m.convMgr != nil {
		if sess := m.convMgr.GetCurrent(); sess != nil {
			title = sess.Title
			if title == "" {
				title = "会话1"
			}
			msgCount = len(sess.Messages)
		}
	}
	modelName := ""
	if m.cfg != nil {
		modelName = m.cfg.Model
	}
	parts := []string{
		MarkerAssistant() + " FreeX Claw",
		modelName,
		fmt.Sprintf("📁 %s (%d条)", title, msgCount),
		"/help",
	}
	info := StatusBarStyle.Render(strings.Join(parts, sep))

	lineW := m.width
	if lineW <= 0 {
		lineW = 80
	}
	gradient := renderGradientLine(lineW)
	return gradient + "\n" + info
}

// renderGradientLine draws a horizontal rule using a two-color simple gradient
// (UserColor → AssistantColor) over the given width using the ▔ glyph.
func renderGradientLine(width int) string {
	if width <= 0 {
		return ""
	}
	mid := width / 2
	userStyle := lipgloss.NewStyle().Foreground(UserColor)
	assistStyle := lipgloss.NewStyle().Foreground(AssistantColor)
	chars := make([]string, width)
	for i := 0; i < width; i++ {
		if i < mid {
			chars[i] = userStyle.Render("▔")
		} else {
			chars[i] = assistStyle.Render("▔")
		}
	}
	return strings.Join(chars, "")
}
