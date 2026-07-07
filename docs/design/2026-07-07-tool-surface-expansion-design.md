# Tool Surface Expansion (Phase 1) — Design

**Date:** 2026-07-07
**Branch:** `feat/tool-surface-expansion`
**Scope:** Add `edit_file`, `grep`, `glob`, and per-invocation confirm on `run_command`. No skill loader, no task system, no subagents (those are Phase 2+).

## Motivation

现有工具集 (`read_file` / `write_file` / `append_file` / `list_dir` / `web_search` / `run_command`) 缺三个 harness 都会用到的能力：

- **精确编辑**：`write_file` 全量覆盖，模型只想改一行也要重写整个文件，容易丢内容。
- **代码检索**：没有 grep，模型只能靠 `list_dir` + `read_file` 逐个翻。
- **模式匹配**：没有 glob，无法一次拿到 `**/*.go` 之类的候选集。

`run_command` 已存在，但当前无任何确认门槛，模型可以静默跑 `rm -rf`。目标是补上逐次 confirm 使其达到主流 harness 的安全默认值。

## Tool Semantics

### `edit_file`

- XML 语法：
  ```
  <edit_file>path
  <<<OLD
  原文（必须在文件中唯一命中）
  OLD
  <<<NEW
  替换后的内容
  NEW
  </edit_file>
  ```
- 强制 `OLD` 在目标文件中**恰好出现一次**，多次或零次都返回错误。
- 保留原文件的换行风格（LF/CRLF），不主动改动。
- 失败模式：文件不存在、匹配零次、匹配多次、`OLD` 为空——全部返回 error，不做部分写入。
- 不支持 `replace_all`（Phase 1 简化；Phase 2 视需要再补）。

### `grep`

- XML 语法：
  ```
  <grep>pattern
  path: .
  glob: *.go
  case: i
  </grep>
  ```
- 首行是 regex pattern；后续 `key: value` 行为可选参数。
- 底层：优先调用系统 `rg`（ripgrep）；不可用时回退到 Go `regexp` 递归实现。
- 输出格式：`file:line:content`，最多 200 行。

### `glob`

- XML 语法：`<glob>**/*.go</glob>` 或多行首行是 pattern，第二行 `path: subdir`。
- 底层用 `doublestar` 库（`**` 支持）；标准库 `filepath.Glob` 不支持递归。
- 按 mtime 降序返回，最多 200 条。

### `run_command` confirm

- 保持现有 XML 语法不变。
- 新增：Model 层收到 `run_command` 工具调用后，**不直接执行**，而是抛出一个 `pendingCommandConfirm` 状态，等用户按 `y/n` 决定。
- 拒绝时返回 `ToolResult{Success:false, Output:"用户拒绝执行"}`，模型继续。
- 命令预览显示 `cwd + command`。
- 可通过启动参数 `--yolo` 关闭 confirm（给自动化脚本用）。

## File Layout

- **New**: `internal/tools/edit.go` + `edit_test.go`
- **New**: `internal/tools/grep.go` + `grep_test.go`
- **New**: `internal/tools/glob.go` + `glob_test.go`
- **Modify**: `internal/agent/agent.go` — regex + dispatch + system prompt
- **Modify**: `internal/tui/model.go` — pending command confirm state + `y/n` key handler
- **Modify**: `cmd/main.go` — `--yolo` flag

## Testing

TDD per tool：先写失败的单测，再最小实现，再重构。confirm 部分需要 TUI 层测试（沿用现有 `model_test.go` 的风格）。

## Out of Scope

- Skill loader（`~/.freexclaw/skills/` 加载，Phase 2）
- Task 系统（TaskCreate/List/Update，Phase 2）
- Subagent 调度（Phase 3）
- Worktree 隔离（Phase 4）
- MCP（Phase 4）
