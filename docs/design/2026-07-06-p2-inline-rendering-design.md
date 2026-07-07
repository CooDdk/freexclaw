# P2：从全屏 TUI 迁移到 inline scrollback 渲染

日期：2026-07-06
项目：FreeX Claw
范围：TUI 架构级迁移，解决 Windows Terminal 鼠标捕获问题，同时保留品牌视觉

## 目标

把 FreeX Claw 的终端界面从 **alt-screen 全屏 TUI** 迁移到 **inline scrollback 渲染**，解决三个核心问题：

1. Windows Terminal + PowerShell 下，鼠标点击/滚动消息内容时状态条消失（有时整块底部区域消失）。
2. alt-screen + 鼠标捕获使得系统级文本选中失效，用户无法用原生方式复制消息。
3. 当前 F2 切换鼠标模式的 workaround 只能掩盖问题，长期不可持续。

同时保留 FreeX Claw 的视觉辨识度，不因架构简化而丢失品牌感。

## 问题背景

### 观察到的失败模式

- **状态条消失**：Windows 11 + Windows Terminal + PowerShell 环境下，鼠标点击消息内容或滚轮滚动时，底部 `FreeX Claw │ ark-code-latest` 状态条会消失，有时下方一整块区域丢失，需要重绘才恢复。
- **原生选中失效**：`tea.WithMouseCellMotion()` 捕获所有鼠标事件，包括系统原本用来做文本选中的左键拖拽。用户想复制一段回答，必须借助 Shift+drag 或额外快捷键。
- **F2 workaround 不是解**：目前通过 F2 切换鼠标模式让用户可以临时禁用捕获来复制。属于对症不对因，且用户需要理解"什么时候按 F2"，学习成本高。
- **上游同问题**：Gemini CLI 的 issue #13875 与本问题结构一致（Windows Terminal + 鼠标捕获 + 全屏），17 位以上用户投诉。表明这是 alt-screen + mouse capture 的架构性问题，不是本项目独有 bug。

### 根本原因

`tea.WithAltScreen()` 让程序独占终端备用缓冲区，配合 `tea.WithMouseCellMotion()` 捕获所有鼠标事件。这个组合在 Windows Terminal 上会与终端自身的重绘/选中逻辑产生竞态，导致状态条区域被终端错误清除。

无论怎么调整键位、增加 workaround，只要保留 "alt-screen + 鼠标捕获" 这个组合，Windows 用户就会遇到问题。

## 推荐方案

改为 **inline scrollback 渲染**：

- 不使用 alt-screen，不占独立缓冲区，程序输出直接进入终端 scrollback。
- 完全移除鼠标捕获，把鼠标事件交给终端原生处理。
- Bubble Tea 的 `View()` 帧只负责渲染 "当前活跃 UI"（输入框、spinner、状态条），完成的历史内容通过 `tea.Println()` 打到 scrollback 上方。
- 用户通过终端自身的滚动条查看历史，通过原生鼠标拖拽选中复制。

这是 Claude Code、Codex CLI、aider 等工具的通用架构。它彻底规避 Windows Terminal 的重绘竞态，同时得到"和 shell 无缝融合"的体验。

### 交互模型选择：B 型（inline + 底部常驻状态条）

参考 Codex CLI 的做法：

- 历史内容作为 scrollback 记录累积在上方
- 屏幕底部保留一行状态条（模型名、会话名等）
- 输入框在状态条上方

这样保留了"永远能看到当前上下文"的信息密度，又不牺牲 inline 架构的核心优势。

### 品牌视觉方案

为避免迁移后视觉过于素净，保留 FreeX Claw 的品牌辨识度：

1. **大 LOGO Banner**：启动时一次性打印 10+ 行 ASCII 艺术 LOGO + 版本信息面板 + Tip，进 scrollback 固化。
2. **短启动动画（可选）**：LOGO 逐字/渐显动画 1.5 秒，播完落定；默认关闭，通过 `--splash` 显式开启。
3. **品牌符号系统**：
   - 用户输入：`❯`（紫色）
   - AI 回复：`✻`（青色）
   - 工具调用：`▸`（橙色）
   - 警告：`⚠`（黄色）
4. **双行渐变状态条**：上方一根渐变分隔线 + 下方 icon 分隔的信息行。
5. **定制 spinner 帧**：`✦ ✧ ✩ ✪` 循环，替代默认 Braille 转圈。
6. **消息卡片化**：*第二阶段* 做（AI 消息用轻量边框包起来）。

### 流式输出策略

- LLM 回复采用 **Spinner + 一次性 finalize**：期间只显示 `✦ 思考中... (N tokens)` 单行 spinner，不逐字渲染
- LLM 完成后，一次性把渲染好的完整 markdown 用 `tea.Println` 打到 scrollback
- 避免逐字流式渲染在 Windows Terminal 上引发的高频重绘/闪烁

### 工具调用展示策略

**详细展开**（不是隐藏）：
- 开始时 View() 帧显示：`▸ web_search("武汉 实时天气")` + `  ⣾ 查询中...`
- 结束时把包含参数、耗时、结果摘要的完整块 `tea.Println` 到 scrollback，帧中的临时行消失

### 会话切换策略

**内嵌选择器**：
- `/sessions` 命令让 View() 切到 picker 模式
- ↑↓ 移动高亮，enter 选中，esc 取消
- 选中后 `tea.Println("→ 已切换到 xxx")`，回到普通输入模式

### 输入历史策略

保留 `↑↓` 回退到之前发送过的输入（bash 风格）。在 spinner 活跃期间禁用，避免歧义。

### 启动画面

一次性 banner，无重复动画。若 `--splash` 明确开启，播 1.5 秒 fade-in 后固化。默认关闭动画。

### 迁移策略

**一步到位**。不保留 `--tui` 回退旗。一个 feat 分支、一个 PR、一次完成，新旧代码不并存。

## 架构对比

| 维度 | 现状（alt-screen） | 迁移后（inline） |
|---|---|---|
| 终端模式 | `tea.WithAltScreen()` 独占备用缓冲区 | 不占，进 scrollback |
| 鼠标 | `tea.WithMouseCellMotion()` 捕获所有事件 | 完全移除，交给终端原生 |
| View() 输出 | 满屏 = splash / 聊天区 / 输入框 / 状态条 / 帮助 | 仅：spinner + 输入框 + 状态条 |
| 历史内容 | 存内存中，viewport 组件滚动 | `tea.Println()` 打到 scrollback |
| 选择/复制 | F2 workaround / shift+drag | 鼠标原生选中，Ctrl+Shift+C 复制 |
| Splash | 3D 动画 loop | 一次性 banner（可选 1.5s 动画） |
| 焦点系统 | input ↔ chat 双焦点 | 移除，只有输入 |
| Viewport 组件 | bubbles/viewport | 删除 |

## 组件设计

### 文件改动清单

| 文件 | 动作 | 说明 |
|---|---|---|
| `cmd/main.go` | 修改 | 去掉 `WithAltScreen`/`WithMouseCellMotion`；启动前调用 `PrintBanner()`；新增 `--splash` flag（默认关闭动画） |
| `internal/tui/model.go` | 大幅简化 | ~1800 行 → 预计 ~800 行 |
| `internal/tui/model_test.go` | 重写 | 删鼠标/焦点相关测试，加新架构测试 |
| `internal/tui/styles.go` | 简化 | 删 ChatViewStyle，保留 Status/Input |
| `internal/tui/banner.go` | 新增 | LOGO ASCII + `PrintBanner()` 函数 |
| `internal/tui/render.go` | 新增 | user/assistant/tool 消息渲染函数 |
| `internal/tui/render_test.go` | 新增 | 渲染函数单元测试 |
| `internal/tui/session_picker.go` | 新增 | 内嵌 picker 组件 |
| `internal/tui/session_picker_test.go` | 新增 | picker 状态转移测试 |
| `internal/tui/theme.go` | 新增 | 品牌色板 + 品牌 spinner 帧 |
| `internal/tui/splash.go`（若存在） | 删除 | 不再需要 |

### Model 状态精简

**保留**：
```go
type Model struct {
    cfg          *config.Config
    convMgr      *conversation.Manager
    agent        *agent.Agent
    textarea     textarea.Model
    spinner      spinner.Model
    width, height int

    inputHistory    []string
    inputHistoryIdx int

    isThinking    bool
    thinkingLabel string
    tokenCount    int

    activeToolCall *pendingTool

    pickerActive bool
    picker       *sessionPicker

    streamBuf strings.Builder

    turnPrompt           string
    runtimePromptProfile string
    ctrlCPrimedAt        time.Time
}
```

**删除**：`focus`、`viewport`、`chatContent`、`showHelp`、`mouseEnabled`、`flashMessage`、所有 splash/animation 状态。

### View() 极简实现

```go
func (m *Model) View() string {
    if m.pickerActive {
        return m.picker.View() + "\n" + m.renderStatusBar()
    }

    var parts []string
    parts = append(parts, m.textarea.View())

    if m.isThinking {
        parts = append(parts, m.renderSpinnerLine())
    }
    if m.activeToolCall != nil {
        parts = append(parts, m.renderToolCallLine())
    }

    parts = append(parts, m.renderStatusBar())
    return strings.Join(parts, "\n")
}
```

View() 返回的行数会随 spinner/toolCall 出现或消失自然变化，Bubble Tea 通过 ANSI 清屏正确处理。

### 关键渲染函数

```go
// render.go
func renderUserMessage(text string) string          // "❯ text"
func renderAssistantMessage(md string) string       // "✻ ..." + markdown 渲染
func renderToolCall(name string, args map[string]any, result string, ok bool, dur time.Duration) string
```

三个函数返回可以直接 `tea.Println` 的字符串（含品牌 marker + 颜色）。

## 交互流程

### 场景 1：普通对话

```
[启动]
  <banner>
❯ ▊
▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔
 ✻ FreeX Claw │ ark-code-latest │ 📁 会话1 (0条) │ /help

[用户输入 "hi" + enter]
  <banner>
  ❯ hi                                        ← tea.Println 立即入
❯ ▊
✦ 思考中... (0 tokens)                        ← View() 帧
▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔
 ✻ FreeX Claw │ ...

[LLM 流式返回，tokens 累加，spinner 转]
✦ 思考中... (312 tokens)
✧ 思考中... (312 tokens)
✩ 思考中... (312 tokens)

[LLM 完成，一次性 finalize]
  <banner>
  ❯ hi
  ✻ 你好！有什么可以帮你的吗？                ← tea.Println
❯ ▊
▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔
 ✻ FreeX Claw │ ark-code-latest │ 📁 会话1 (2条) │ /help
```

### 场景 2：带工具调用

```
[用户输入 "武汉今天天气"]
❯ ▊
✦ 分析中... (45 tokens)

[LLM 决定调用 web_search]
❯ ▊
▸ web_search("武汉 实时天气")
  ⣾ 查询中...

[工具返回]
  ❯ 武汉今天天气
  ▸ web_search("武汉 实时天气")
    ✓ 3 个结果，0.8s
    • 武汉天气 - 中国天气网
    • 湖北气象台预报
    • 高德天气实时
❯ ▊
✦ 思考中... (128 tokens)

[最终回复]
  ▸ web_search(...) ✓ 3 个结果
  ✻ 武汉今天多云，气温 18-24°C ...
    数据来源：中国天气网
❯ ▊
```

### 场景 3：/sessions 切换

```
[用户输入 /sessions]
─ 选择会话 ──────────────────────
› 1  项目分析  (12 条)
  2  天气查询  (3 条)
  3  当前会话  (5 条) ○
──────────────────────────────
(↑↓ 选择 · enter 打开 · esc 取消)

[enter 选中 会话 1]
  ❯ /sessions
  → 已切换到 "项目分析" (12 条消息)          ← tea.Println
❯ ▊
▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔
 ✻ FreeX Claw │ ark-code-latest │ 📁 项目分析 (12条) │ /help
```

## 测试策略

### 单元测试

| 测试项 | 位置 | 说明 |
|---|---|---|
| `PrintBanner()` 输出包含品牌 tokens | `banner_test.go` | LOGO / 版本 / tagline 出现 |
| `renderStatusBar()` 不同 width 下不崩溃 | `render_test.go` | 结构稳定 |
| `renderUserMessage("hi")` 含 `❯` | `render_test.go` | 品牌 marker |
| `renderAssistantMessage(md)` 含 `✻` | `render_test.go` | 卡片首行 |
| `renderToolCall` 含 `▸` + `✓`/`✗` | `render_test.go` | 成功/失败区分 |
| 品牌 spinner 有 4 帧 | `spinner_test.go` | 帧列表固化 |
| picker `↑/↓/enter/esc` 状态转移 | `picker_test.go` | 索引边界、emit msg |
| 输入历史 `↑/↓` 在 spinner 期间被禁用 | `model_test.go` | `isThinking=true` 时忽略 |
| `handleSlash("/sessions")` → picker 激活 | `slash_test.go` | 命令分发 |
| `handleSlash("/help")` → tea.Println 帮助 | `slash_test.go` | 命令分发 |

### 集成测试

用 `teatest`（Bubble Tea 官方测试工具）跑一个假 Agent：
- 输入 "hi" → 断言 View() 帧结构（textarea + status）
- 触发 thinking → 断言 spinner 行出现
- 触发 tool call → 断言 `▸` 行出现
- 触发 assistant done → 断言 `tea.Println` 被调用（截获输出）

### 手动测试矩阵

| 终端 | OS | 关键验证点 |
|---|---|---|
| Windows Terminal + PowerShell | Win11 | ✅ 状态条不消失 · ✅ 鼠标原生选中 · ✅ scrollback 完整 · ✅ 中文对齐 |
| Windows Terminal + cmd.exe | Win11 | 同上 |
| VS Code 集成终端 | 全平台 | banner 不错位、spinner 不闪 |
| macOS Terminal.app | macOS | 颜色渐变生效 |
| iTerm2 | macOS | OSC 52 剪贴板路径 |
| xterm / gnome-terminal | Linux | ANSI 兼容 |
| ssh 会话 | 任意 | 移除 alt-screen 后 scrollback 是否可回滚到 client 侧 |

## 风险与缓解

| 风险 | 缓解 |
|---|---|
| `tea.Println` 与 View() 帧交互不当会闪烁 | 单元测试验证顺序；Windows Terminal 手工走一遍所有流程 |
| Spinner 每 100ms 重绘 View()，部分终端微闪 | 保持 bubbles 默认 tick 频率；可通过 spinner 帧数控制视觉稳定性 |
| 长回复入 scrollback 后无法"回到当前对话"聚焦 | scrollback 是终端职责，由用户 Ctrl+End；文档中说明 |
| 会话切换后 scrollback 不重放旧对话 | 设计取舍：scrollback 反映"本次运行历史"；完整历史通过 `/history` 命令另建查看 |
| 输入历史 `↑` 与 spinner 冲突 | spinner 期间禁用 `↑↓` 历史，只允许 Ctrl+C 中断 |
| `--splash` 动画在 Windows Terminal 上闪 | 默认关闭；帧高度 <10 行且总时长 <2 秒；出问题时 fallback 静态 banner |

## 明确不做的事

- ❌ 消息卡片化（列在视觉方案里，移到第二阶段，先跑通 inline）
- ❌ 主题定制（第一版硬编码品牌色板）
- ❌ Markdown 表格/图片增强
- ❌ 会话历史"重放到 scrollback"功能（通过 `/history` 命令另建）
- ❌ 保留 `--tui` 回退旗（一步到位）

## 灰度与回滚

- 迁移分支：`feat/inline-rendering`
- 单 PR，附上迁移前后对比截图 + Windows Terminal 问题录屏
- 上线后保留 `v1.0-preinline` git tag，用户报问题可 `git checkout` 回去

## 工作量估计

- 核心 inline 迁移：2-3 天
- 品牌视觉打磨（LOGO、符号、状态条、spinner、可选动画）：1-2 天
- 测试 + 手工验证矩阵：0.5-1 天
- **合计：4-6 天**
