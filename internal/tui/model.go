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
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cloudwego/eino/schema"
	"github.com/atotto/clipboard"

	"github.com/CooDdk/freexclaw/internal/agent"
	"github.com/CooDdk/freexclaw/internal/config"
	"github.com/CooDdk/freexclaw/internal/conversation"
	"github.com/CooDdk/freexclaw/internal/llm"
	"github.com/CooDdk/freexclaw/internal/tools"
)

type focusState int

const (
	focusInput focusState = iota
	focusChat
)

var copyToClipboard = clipboard.WriteAll

// commandItem 定义一个斜杠命令
type commandItem struct {
	Name string
	Desc string
}

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

type splashTickMsg time.Time

func splashTickCmd() tea.Cmd {
	return tea.Tick(60*time.Millisecond, func(t time.Time) tea.Msg {
		return splashTickMsg(t)
	})
}

type splashEndMsg struct{}

const ctrlCExitConfirmWindow = 2 * time.Second

// ModelOptions carries runtime options for NewModel.
type ModelOptions struct {
	Splash bool
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
	viewport     viewport.Model
	textarea     textarea.Model
	focus        focusState
	width        int
	height       int
	isStreaming  bool
	streamCtx    context.Context
	streamCancel context.CancelFunc
	chunkCh      <-chan llm.StreamChunk
	toolCh       <-chan tea.Msg
	showHelp     bool
	err          error
	dotsAnim     int
	isToolRunning bool
	toolStatusText string
	// 命令提示面板
	commandHintVisible bool
	commandHintIndex   int
	// 输入历史记录
	inputHistory     []string
	inputHistoryIdx  int
	inputHistoryTemp string
	// 强制滚动到底部标志
	forceScrollBottom bool
	// 启动页
	showSplash     bool
	splashStage    int
	splashProgress float64
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
	flashMessage       string
	// P2 inline migration new fields (Task 7)
	isThinking     bool
	thinkingLabel  string
	tokenCount     int
	streamBuf      strings.Builder
	spinnerTickN   int
	activeToolCall *pendingTool
	pickerActive   bool
	picker         *sessionPicker
	splashOpt      bool
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

	vp := viewport.New(0, 0)
	vp.MouseWheelEnabled = true

	m := &Model{
		cfg:         cfg,
		llmClient:   llmClient,
		convMgr:     conversation.NewManager(tools.GetWorkDir()),
		viewport:    vp,
		textarea:    ta,
		focus:       focusInput,
		showHelp:    true,
		splashStage: 0,
	}

	m.splashOpt = opts.Splash
	// Keep the legacy showSplash aligned with opt for now; Task 11 removes it entirely.
	m.showSplash = opts.Splash

	m.textarea.Focus()
	m.updateChatView()

	return m, nil
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, splashTickCmd())
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		return m, nil

	case splashTickMsg:
		if m.showSplash {
			m.splashStage++
			m.splashProgress = float64(m.splashStage) / 40.0
			if m.splashProgress > 1.0 {
				m.splashProgress = 1.0
			}
			if m.splashStage >= 50 {
				m.showSplash = false
				m.forceScrollBottom = true
				m.updateChatView()
				return m, nil
			}
			return m, splashTickCmd()
		}
		return m, nil

	case splashEndMsg:
		m.showSplash = false
		m.forceScrollBottom = true
		m.updateChatView()
		return m, nil

	case tickMsg:
		if m.isStreaming {
			m.dotsAnim++
			m.updateChatView()
			return m, tickCmd()
		}
		return m, nil

	case streamMsg:
		return m.handleStreamMsg(msg)

	case toolProgressMsg:
		m.toolStatusText = msg.text
		m.forceScrollBottom = true
		m.updateChatView()
		return m, m.waitForToolEvent()

	case toolExecutedMsg:
		return m.handleToolExecuted(msg)

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case tea.MouseMsg:
		return m.handleMouseMsg(msg)
	}

	if m.focus == focusInput {
		m.textarea, cmd = m.textarea.Update(msg)
		cmds = append(cmds, cmd)
		// textarea 内容可能变化，更新命令提示面板
		if m.updateCommandHint() {
			m.resize()
		}
	}
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *Model) handleMouseMsg(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.showSplash {
		m.showSplash = false
		m.forceScrollBottom = true
		m.updateChatView()
		return m, nil
	}

	m.clearCtrlCExitState()

	switch {
	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft:
		if m.isChatAreaY(msg.Y) {
			m.setFocusChat()
			return m, nil
		}
		if m.isInputZoneY(msg.Y) {
			m.setFocusInput()
			return m, nil
		}
	case msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown:
		if m.isInputZoneY(msg.Y) && !m.isStreaming && !m.commandHintVisible {
			m.setFocusInput()
			if msg.Button == tea.MouseButtonWheelUp {
				m.navigateHistory(-1)
			} else {
				m.navigateHistory(1)
			}
			return m, nil
		}
		if m.isChatAreaY(msg.Y) {
			m.setFocusChat()
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m *Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.showSplash {
		m.showSplash = false
		m.forceScrollBottom = true
		m.updateChatView()
		return m, nil
	}

	if msg.String() != "ctrl+c" {
		m.clearCtrlCExitState()
	}
	m.flashMessage = ""

	switch msg.String() {
	case "ctrl+c":
		if m.isStreaming {
			m.stopStreaming()
			m.clearCtrlCExitState()
			m.updateChatView()
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
		// 命令提示面板开启时，esc 优先关闭提示
		if m.commandHintVisible {
			m.commandHintVisible = false
			m.commandHintIndex = 0
			m.resize()
			return m, nil
		}
		m.toggleFocus()
		return m, nil
	}

	if m.focus == focusChat {
		return m.handleChatKey(msg)
	}

	// 输入框焦点下，命令提示面板关闭时，上下键切换输入历史
	if m.focus == focusInput && !m.isStreaming && !m.commandHintVisible {
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
				m.resize()
			}
			return m, nil
		}
	}

	if !m.isStreaming {
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		if m.updateCommandHint() {
			m.resize()
		}
		return m, cmd
	}

	return m, nil
}

func (m *Model) setFocusInput() {
	m.focus = focusInput
	m.textarea.Focus()
}

func (m *Model) setFocusChat() {
	m.focus = focusChat
	m.textarea.Blur()
}

func (m *Model) shouldQuitOnCtrlC() bool {
	return !m.ctrlCPrimedAt.IsZero() && time.Since(m.ctrlCPrimedAt) <= ctrlCExitConfirmWindow
}

func (m *Model) prepareCtrlCToClearInput() {
	m.ctrlCPrimedAt = time.Now()
	m.focus = focusInput
	m.textarea.Focus()
	m.textarea.SetValue("")
	m.commandHintVisible = false
	m.commandHintIndex = 0
	m.inputHistoryTemp = ""
	m.resize()
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

func (m *Model) handleChatKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		m.viewport.LineDown(1)
		return m, nil
	case "k", "up":
		m.viewport.LineUp(1)
		return m, nil
	case "ctrl+d":
		m.viewport.HalfPageDown()
		return m, nil
	case "ctrl+u":
		m.viewport.HalfPageUp()
		return m, nil
	case "g":
		m.viewport.GotoTop()
		return m, nil
	case "G":
		m.viewport.GotoBottom()
		return m, nil
	case "y":
		m.copyLastAssistantMessage()
		return m, nil
	default:
		m.focus = focusInput
		m.textarea.Focus()
		return m, nil
	}
}

func (m *Model) copyLastAssistantMessage() {
	current := m.convMgr.GetCurrent()
	if current == nil {
		m.flashMessage = "没有可复制的消息"
		return
	}
	for i := len(current.Messages) - 1; i >= 0; i-- {
		msg := current.Messages[i]
		if msg.Role != conversation.RoleAssistant {
			continue
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		if err := copyToClipboard(content); err != nil {
			m.flashMessage = fmt.Sprintf("复制失败: %v", err)
			return
		}
		m.flashMessage = fmt.Sprintf("已复制 %d 字符", len([]rune(content)))
		return
	}
	m.flashMessage = "没有可复制的消息"
}

func (m *Model) handleStreamMsg(msg streamMsg) (tea.Model, tea.Cmd) {
	current := m.convMgr.GetCurrent()

	if msg.err != nil {
		m.err = msg.err
		m.isStreaming = false
		// 流式失败时，删除之前添加的空 assistant 消息，避免下次请求发送空消息给 API
		if len(current.Messages) > 0 {
			last := &current.Messages[len(current.Messages)-1]
			if last.Role == conversation.RoleAssistant && last.Content == "" {
				current.Messages = current.Messages[:len(current.Messages)-1]
			}
		}
		m.updateChatView()
		m.convMgr.Save()
		return m, nil
	}

	if msg.done {
		m.isStreaming = false
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
			m.updateChatView()
			m.convMgr.Save()
			if m.shouldSkipDuplicateToolCall(tc) {
				return m, m.skipDuplicateToolCall(tc)
			}
			return m, m.startToolExecution(tc)
		}

		m.updateChatView()
		m.convMgr.Save()

		// 如果没有工具调用，尝试自动提取文件内容
		if m.shouldAutoExtractSingleFile() {
			if extracted, filePath, err := agent.AutoExtractFileContent(lastContent); extracted {
				if err != nil {
					current.AddMessage(conversation.RoleSystem, fmt.Sprintf("❌ 自动保存文件失败: %v", err))
				} else {
					m.noteTouchedFile(filePath)
					current.AddMessage(conversation.RoleSystem, fmt.Sprintf("✅ 已自动保存到文件: %s", filePath))
				}
				m.forceScrollBottom = true
				m.updateChatView()
				m.convMgr.Save()
			}
		}

		if cmd := m.maybeContinueEngineeringToolFlow(lastContent); cmd != nil {
			return m, cmd
		}

		if cmd := m.maybeRunForcedPostProcess(); cmd != nil {
			return m, cmd
		}

		_ = completeTaskSpecHandoffIfNeeded(m.runtimePromptProfile, m.turnTouchedFiles, m.turnExecutedCommands, lastContent)

		return m, nil
	}

	if len(current.Messages) > 0 {
		last := &current.Messages[len(current.Messages)-1]
		last.Content += msg.content
		current.UpdatedAt = last.CreatedAt
	}
	m.updateChatView()
	return m, m.waitForStream()
}

func (m *Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "初始化中..."
	}

	if m.showSplash {
		return m.renderSplash()
	}

	chat := m.renderChat()
	input := m.renderInput()
	status := m.renderStatusBar()
	help := m.renderHelpBar()

	parts := []string{chat}
	if m.commandHintVisible {
		parts = append(parts, m.renderCommandHint())
	}
	parts = append(parts, input, status, help)

	return lipgloss.JoinVertical(lipgloss.Top, parts...)
}

func (m *Model) resize() {
	const (
		statusH        = 1
		helpH          = 1
		inputBorderTop = 1
		inputPaddingV  = 1
		inputPaddingH  = 2
		chatPaddingV   = 1
		chatPaddingH   = 2
		textareaLines  = 1
	)

	inputH := inputBorderTop + inputPaddingV*2 + textareaLines

	// 命令提示面板占用的高度
	hintH := 0
	if m.commandHintVisible {
		hintH = m.commandHintHeight()
	}

	chatContentH := m.height - inputH - hintH - statusH - helpH - chatPaddingV*2
	if chatContentH < 5 {
		chatContentH = 5
	}

	chatContentW := m.width - chatPaddingH*2
	if chatContentW < 20 {
		chatContentW = 20
	}

	m.viewport.Width = chatContentW
	m.viewport.Height = chatContentH

	inputContentW := m.width - inputPaddingH*2
	if inputContentW < 20 {
		inputContentW = 20
	}

	m.textarea.SetWidth(inputContentW)
	m.textarea.SetHeight(textareaLines)

	m.updateChatView()
}

func (m *Model) inputAreaHeight() int {
	const (
		inputBorderTop = 1
		inputPaddingV  = 1
		textareaLines  = 1
	)
	return inputBorderTop + inputPaddingV*2 + textareaLines
}

func (m *Model) hintAreaHeight() int {
	if m.commandHintVisible {
		return m.commandHintHeight()
	}
	return 0
}

func (m *Model) chatAreaHeight() int {
	const (
		statusH = 1
		helpH   = 1
	)
	chatH := m.height - m.inputAreaHeight() - m.hintAreaHeight() - statusH - helpH
	if chatH < 1 {
		return 1
	}
	return chatH
}

func (m *Model) isChatAreaY(y int) bool {
	return y >= 0 && y < m.chatAreaHeight()
}

func (m *Model) isInputZoneY(y int) bool {
	start := m.chatAreaHeight()
	end := start + m.hintAreaHeight() + m.inputAreaHeight()
	return y >= start && y < end
}

func (m *Model) toggleFocus() {
	if m.focus == focusInput {
		m.setFocusChat()
	} else {
		m.setFocusInput()
	}
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

	if shouldUseEngineeringPlanning(m.runtimePromptProfile, content) && !isExecutionApproval(content) {
		if _, err := ensureTaskSpec(content, m.runtimePromptProfile); err != nil {
			current.AddMessage(conversation.RoleSystem, fmt.Sprintf("❌ 生成内部开发文档失败: %v", err))
		} else {
			current.AddMessage(conversation.RoleSystem, buildPlanningNotice(content))
		}
		m.textarea.SetValue("")
		m.isStreaming = false
		m.err = nil
		m.dotsAnim = 0
		m.inputHistoryIdx = len(m.inputHistory)
		m.inputHistoryTemp = ""
		m.forceScrollBottom = true
		m.updateChatView()
		m.convMgr.Save()
		return nil
	}

	if awaitingApproval && isExecutionApproval(content) {
		current.AddMessage(conversation.RoleSystem, "已确认 .freexclaw/spec 内部开发文档，开始按步骤执行。")
	}
	current.AddMessage(conversation.RoleAssistant, "")

	m.textarea.SetValue("")
	m.isStreaming = true
	m.err = nil
	m.dotsAnim = 0
	m.inputHistoryIdx = len(m.inputHistory)
	m.inputHistoryTemp = ""
	m.forceScrollBottom = true
	m.updateChatView()

	if tc := buildPreflightToolCall(content); tc != nil {
		current.UpdateLastMessage(tcToTag(tc))
		return m.startToolExecution(tc)
	}

	m.streamCtx, m.streamCancel = context.WithCancel(context.Background())

	messages := m.buildMessages()
	m.chunkCh = m.llmClient.StreamChat(m.streamCtx, messages)

	return tea.Batch(m.waitForStream(), tickCmd())
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

func canonicalizePreflightQuery(content string, match tools.MatchResult) string {
	if match.Domain == "weather" {
		location := strings.TrimSpace(match.Location)
		timeOfDay := strings.TrimSpace(match.TimeOfDay)
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
		current.AddMessage(conversation.RoleSystem, helpText)
		m.forceScrollBottom = true
		m.updateChatView()

	case "/clear":
		current.Messages = nil
		m.err = nil
		m.forceScrollBottom = true
		m.updateChatView()

	case "/new":
		m.convMgr.NewConversation()
		m.convMgr.Save()
		m.forceScrollBottom = true
		m.updateChatView()

	case "/save":
		if err := m.convMgr.Save(); err != nil {
			m.err = err
		} else {
			current.AddMessage(conversation.RoleSystem, "✓ 对话已保存")
		}
		m.forceScrollBottom = true
		m.updateChatView()

	case "/sessions":
		convs := m.convMgr.List()
		if len(convs) == 0 {
			current.AddMessage(conversation.RoleSystem, "暂无会话")
		} else {
			var sb strings.Builder
			curID := m.convMgr.GetCurrent().ID
			sb.WriteString(fmt.Sprintf("会话列表 (共 %d 个):\n", len(convs)))
			for i, c := range convs {
				marker := "  "
				if c.ID == curID {
					marker = "▶ "
				}
				sb.WriteString(fmt.Sprintf("%s[%d] %s (%d 条消息) %s\n",
					marker, i+1, c.Title, len(c.Messages),
					c.UpdatedAt.Format("2006-01-02 15:04")))
			}
			sb.WriteString("\n输入 /open <序号> 进入指定会话")
			current.AddMessage(conversation.RoleSystem, sb.String())
		}
		m.forceScrollBottom = true
		m.updateChatView()

	case "/open":
		if len(parts) < 2 {
			current.AddMessage(conversation.RoleSystem, "用法: /open <序号>\n输入 /sessions 查看会话列表")
			m.updateChatView()
			break
		}
		idx, err := strconv.Atoi(parts[1])
		if err != nil || idx < 1 {
			current.AddMessage(conversation.RoleSystem, "序号必须是正整数")
			m.updateChatView()
			break
		}
		convs := m.convMgr.List()
		if idx > len(convs) {
			current.AddMessage(conversation.RoleSystem, fmt.Sprintf("序号超出范围，共 %d 个会话", len(convs)))
			m.updateChatView()
			break
		}
		m.convMgr.SetCurrent(convs[idx-1].ID)
		m.forceScrollBottom = true
		m.updateChatView()

	case "/rename":
		if len(parts) < 2 {
			current.AddMessage(conversation.RoleSystem, "用法: /rename <新名称>\n示例: /rename Python 脚本开发")
			m.forceScrollBottom = true
			m.updateChatView()
			break
		}
		newTitle := strings.Join(parts[1:], " ")
		current := m.convMgr.GetCurrent()
		oldTitle := current.Title
		current.Title = newTitle
		current.UpdatedAt = time.Now()
		if err := m.convMgr.Save(); err != nil {
			current.AddMessage(conversation.RoleSystem, fmt.Sprintf("❌ 重命名失败: %v", err))
		} else {
			current.AddMessage(conversation.RoleSystem, fmt.Sprintf("✓ 会话已重命名: %q → %q", oldTitle, newTitle))
		}
		m.forceScrollBottom = true
		m.updateChatView()

	case "/read":
		if len(parts) < 2 {
			current.AddMessage(conversation.RoleSystem, "用法: /read <文件路径>\n示例: /read main.go")
			m.forceScrollBottom = true
			m.updateChatView()
			break
		}
		path := strings.Join(parts[1:], " ")
		fc, err := tools.ReadFile(path)
		if err != nil {
			current.AddMessage(conversation.RoleSystem, fmt.Sprintf("❌ 读取失败: %v", err))
		} else {
			current.AddMessage(conversation.RoleUser, fmt.Sprintf("请分析这个文件的内容:\n\n%s", tools.FormatFileContent(fc)))
		}
		m.forceScrollBottom = true
		m.updateChatView()

	case "/write":
		if len(parts) < 3 {
			current.AddMessage(conversation.RoleSystem, "用法: /write <文件路径> <内容>\n示例: /write hello.txt Hello World\n使用 -a 参数追加: /write -a log.txt 新内容")
			m.forceScrollBottom = true
			m.updateChatView()
			break
		}
		args := strings.Join(parts[1:], " ")
		path, content, appendMode, err := tools.ParseWriteArgs(args)
		if err != nil {
			current.AddMessage(conversation.RoleSystem, fmt.Sprintf("❌ 参数错误: %v", err))
			m.forceScrollBottom = true
			m.updateChatView()
			break
		}
		if err := tools.WriteFile(path, content+"\n", appendMode); err != nil {
			current.AddMessage(conversation.RoleSystem, fmt.Sprintf("❌ 写入失败: %v", err))
		} else {
			if appendMode {
				current.AddMessage(conversation.RoleSystem, fmt.Sprintf("✅ 已追加到 %s", path))
			} else {
				current.AddMessage(conversation.RoleSystem, fmt.Sprintf("✅ 已写入 %s", path))
			}
			// 读取刚写入的文件，方便继续编辑
			if fc, readErr := tools.ReadFile(path); readErr == nil {
				current.AddMessage(conversation.RoleUser, fmt.Sprintf("文件内容已更新，继续编辑:\n\n%s", tools.FormatFileContent(fc)))
			}
		}
		m.forceScrollBottom = true
		m.updateChatView()

	case "/ls":
		path := "."
		if len(parts) >= 2 {
			path = strings.Join(parts[1:], " ")
		}
		files, err := tools.ListDir(path)
		if err != nil {
			current.AddMessage(conversation.RoleSystem, fmt.Sprintf("❌ 读取目录失败: %v", err))
		} else {
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("📂 目录: %s\n", path))
			for _, f := range files {
				sb.WriteString("  " + f + "\n")
			}
			if len(files) == 0 {
				sb.WriteString("  (空目录)\n")
			}
			current.AddMessage(conversation.RoleSystem, sb.String())
		}
		m.forceScrollBottom = true
		m.updateChatView()

	case "/quit":
		m.convMgr.Save()
		return tea.Quit

	default:
		current.AddMessage(conversation.RoleSystem, fmt.Sprintf("未知命令: %s\n输入 /help 查看可用命令", parts[0]))
		m.updateChatView()
	}

	m.textarea.SetValue("")
	m.commandHintVisible = false
	m.commandHintIndex = 0
	m.resize()
	return nil
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

func (m *Model) updateChatView() {
	current := m.convMgr.GetCurrent()
	var messages []string

	if len(current.Messages) == 0 {
		bannerText := renderBanner(m.viewport.Width)
		messages = append(messages, bannerText)
		messages = append(messages, "")

		welcome := WelcomeStyle.Render("  欢迎使用 FreeX Claw 终端 AI 助手")
		messages = append(messages, welcome)
		messages = append(messages, "")

		tips := []string{
			"  💡 快捷操作:",
			"     Enter       发送消息",
			"     Ctrl+J      换行",
			"     Esc         切换焦点（输入框 ↔ 聊天区）",
			"     j/k         聊天区上下滚动",
			"     Ctrl+u/d    聊天区上下翻页",
			"     g/G         聊天区跳到顶部/底部",
			"     /help       显示帮助",
			"     Ctrl+C      退出或停止生成",
			"",
			"  🚀 在下方输入消息，开始你的 AI 之旅！",
		}
		for _, tip := range tips {
			messages = append(messages, PlaceholderStyle.Render(tip))
		}
		messages = append(messages, "")
	}

	for i, msg := range current.Messages {
		content := current.Messages[i].Content

		if msg.Role == conversation.RoleSystem {
			messages = append(messages, SystemMessageStyle.Render(content))
			messages = append(messages, "")
			continue
		}

		// 工具结果消息（来自用户角色的 tool_result），美化显示
		if msg.Role == conversation.RoleUser && strings.HasPrefix(content, "<|tool_result|>") && strings.HasSuffix(content, "</|tool_result|>") {
			if shouldHideToolResultInContext(current.Messages, i) {
				continue
			}
			resultContent := strings.TrimPrefix(content, "<|tool_result|>")
			resultContent = strings.TrimSuffix(resultContent, "</|tool_result|>")
			resultContent = strings.TrimSpace(resultContent)
			display := "🔧 工具执行结果\n" + resultContent
			messages = append(messages, ToolResultStyle.Render(display))
			messages = append(messages, "")
			continue
		}

		// AI 消息中包含工具调用标记，美化显示
		if msg.Role == conversation.RoleAssistant && (strings.Contains(content, "<write_file>") || strings.Contains(content, "<read_file>") || strings.Contains(content, "<list_dir>") || strings.Contains(content, "<append_file>") || strings.Contains(content, "<web_search>") || strings.Contains(content, "<run_command>")) {
			tc := agent.ParseToolCall(content)
			if tc != nil {
				// 显示思考过程（工具调用之前的文本）和工具调用
				display := fmt.Sprintf("🔧 正在调用工具: %s", tc.Name)
				if tc.Name == "write_file" || tc.Name == "append_file" {
					if path, ok := tc.Arguments["path"].(string); ok {
						display += fmt.Sprintf(" → %s", path)
					}
				} else if tc.Name == "read_file" || tc.Name == "list_dir" {
					if path, ok := tc.Arguments["path"].(string); ok {
						display += fmt.Sprintf(" → %s", path)
					}
				} else if tc.Name == "run_command" {
					if command, ok := tc.Arguments["command"].(string); ok {
						display += fmt.Sprintf(" → %s", command)
					}
				}
				messages = append(messages, AIPrefixStyle.Render("● ")+MessageContentStyle.Render(display))
				messages = append(messages, "")
				continue
			}
		}

		var prefix string
		var prefixStyle lipgloss.Style

		switch msg.Role {
		case conversation.RoleUser:
			prefix = "┃ "
			prefixStyle = UserPrefixStyle
		case conversation.RoleAssistant:
			prefix = "● "
			prefixStyle = AIPrefixStyle
		}

		if msg.Role == conversation.RoleAssistant && m.isStreaming && content == "" {
			dots := ""
			switch m.dotsAnim % 4 {
			case 0:
				dots = "   "
			case 1:
				dots = ".  "
			case 2:
				dots = ".. "
			case 3:
				dots = "..."
			}
			thinking := ThinkingStyle.Render("● thinking" + dots)
			messages = append(messages, thinking)
			messages = append(messages, "")
			continue
		}

		var rendered string
		isLastAssistant := msg.Role == conversation.RoleAssistant && i == len(current.Messages)-1
		isStreamingThisMsg := m.isStreaming && isLastAssistant
		if msg.Role == conversation.RoleAssistant && content != "" && !isStreamingThisMsg {
			rendered = renderMarkdown(content, m.viewport.Width)
		} else {
			rendered = MessageContentStyle.Render(content)
		}

		lines := strings.SplitN(rendered, "\n", 2)
		if len(lines) > 0 {
			lines[0] = prefixStyle.Render(prefix) + lines[0]
		}
		messages = append(messages, strings.Join(lines, "\n"))
		messages = append(messages, "")
	}

	if m.isToolRunning {
		toolLine := "● 工具执行中"
		if strings.TrimSpace(m.toolStatusText) != "" {
			toolLine = "● " + m.toolStatusText
		}
		messages = append(messages, ThinkingStyle.Render(toolLine))
		messages = append(messages, "")
	}

	fullContent := lipgloss.JoinVertical(lipgloss.Left, messages...)

	// SetContent 之前记录用户是否在底部（内容变化后 AtBottom 会不准）
	wasAtBottom := m.viewport.AtBottom()

	m.viewport.SetContent(fullContent)

	// 强制滚动 或 用户之前在底部 或 视口还没初始化（0高度），都滚到底部
	if m.forceScrollBottom || wasAtBottom || m.viewport.Height == 0 {
		m.viewport.GotoBottom()
		m.forceScrollBottom = false
	}
}

func (m *Model) renderChat() string {
	return ChatViewStyle.Width(m.width).Render(m.viewport.View())
}

func (m *Model) renderInput() string {
	var inputView string
	if m.isStreaming {
		inputView = PlaceholderStyle.Render("⏳ 正在生成回复... 按 Ctrl+C 停止")
	} else if m.isToolRunning {
		inputView = PlaceholderStyle.Render("⏳ 正在处理... 详见上方进度")
	} else {
		inputView = m.textarea.View()
	}
	return InputAreaStyle.Width(m.width).Render(inputView)
}

func (m *Model) renderStatusBar() string {
	var parts []string

	brand := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A78BFA")).
		Bold(true).
		Render("FreeX Claw")

	modelInfo := m.cfg.Model

	current := m.convMgr.GetCurrent()
	convInfo := current.Title

	parts = append(parts, brand, modelInfo, convInfo)

	cwd := tools.GetWorkDir()
	cwdStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#86EFAC")).
		Render(cwd)
	parts = append(parts, cwdStyle)

	if m.isStreaming {
		status := lipgloss.NewStyle().Foreground(lipgloss.Color("#22D3EE")).Render("● streaming")
		parts = append(parts, status)
	}

	if m.err != nil {
		errInfo := lipgloss.NewStyle().Foreground(ErrorColor).Render(fmt.Sprintf("错误: %v", m.err))
		parts = append(parts, errInfo)
	}

	if m.flashMessage != "" {
		flash := lipgloss.NewStyle().Foreground(lipgloss.Color("#86EFAC")).Render(m.flashMessage)
		parts = append(parts, flash)
	}

	status := strings.Join(parts, " │ ")
	return StatusBarStyle.Width(m.width).MaxWidth(m.width).MaxHeight(1).Render(status)
}

func (m *Model) renderHelpBar() string {
	var help string
	if m.focus == focusInput {
		if m.shouldQuitOnCtrlC() {
			help = "enter 发送 | shift+enter 换行 | ↑↓/滚轮 历史 | esc 聊天区 | shift+拖拽 复制 | ctrl+c 再按退出"
		} else {
			help = "enter 发送 | shift+enter 换行 | ↑↓/滚轮 历史 | esc 聊天区 | shift+拖拽 复制 | ctrl+c 清空/退出"
		}
	} else {
		help = "j/k/滚轮 滚动 | ctrl+u/d 翻页 | g/G 顶/底 | y 复制最后回复 | shift+拖拽 复制 | 点击/esc 返回输入"
	}
	return HelpStyle.Width(m.width).MaxWidth(m.width).MaxHeight(1).Render(help)
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

// commandHintHeight 返回命令提示面板占用的行数（含边框）
func (m *Model) commandHintHeight() int {
	matched := m.matchedCommands()
	if len(matched) == 0 {
		return 0
	}
	// 命令行数 + 上下边框(2)
	return len(matched) + 2
}

// renderCommandHint 渲染命令提示面板
func (m *Model) renderCommandHint() string {
	matched := m.matchedCommands()
	if len(matched) == 0 {
		return ""
	}
	var lines []string
	for i, cmd := range matched {
		marker := "  "
		nameStyle := CommandHintItemStyle
		if i == m.commandHintIndex {
			marker = "▶ "
			nameStyle = CommandHintSelectedItemStyle
		}
		line := marker + nameStyle.Render(cmd.Name) + "  " + CommandHintDescStyle.Render(cmd.Desc)
		lines = append(lines, line)
	}
	content := strings.Join(lines, "\n")
	// border 2 + padding 2 = 4，使总宽度 = m.width
	return CommandHintStyle.Width(m.width - 4).Render(content)
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
	m.isToolRunning = true
	m.toolStatusText = describeToolExecution(tc)
	m.forceScrollBottom = true
	m.updateChatView()
	m.convMgr.Save()
	m.toolCh = runToolAsync(tc)
	return m.waitForToolEvent()
}

func (m *Model) handleToolExecuted(msg toolExecutedMsg) (tea.Model, tea.Cmd) {
	current := m.convMgr.GetCurrent()
	tc := msg.tc
	result := msg.result
	m.isToolRunning = false
	m.toolStatusText = ""

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
	m.err = nil
	m.dotsAnim = 0
	m.forceScrollBottom = true
	m.updateChatView()
	m.convMgr.Save()

	// 启动下一轮流式
	m.streamCtx, m.streamCancel = context.WithCancel(context.Background())
	messages := m.buildMessages()
	m.chunkCh = m.llmClient.StreamChat(m.streamCtx, messages)

	return m, tea.Batch(m.waitForStream(), tickCmd())
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
	m.err = nil
	m.dotsAnim = 0
	m.forceScrollBottom = true
	m.updateChatView()
	m.convMgr.Save()

	m.streamCtx, m.streamCancel = context.WithCancel(context.Background())
	messages := m.buildMessages()
	m.chunkCh = m.llmClient.StreamChat(m.streamCtx, messages)
	return tea.Batch(m.waitForStream(), tickCmd())
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
			m.forceScrollBottom = true
			m.updateChatView()
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
	m.err = nil
	m.dotsAnim = 0
	m.forceScrollBottom = true
	m.updateChatView()
	m.convMgr.Save()

	m.streamCtx, m.streamCancel = context.WithCancel(context.Background())
	messages := m.buildMessages()
	m.chunkCh = m.llmClient.StreamChat(m.streamCtx, messages)
	return tea.Batch(m.waitForStream(), tickCmd())
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
