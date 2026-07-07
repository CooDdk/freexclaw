package tui

import (
	"fmt"
	"strings"

	"github.com/CooDdk/freexclaw/internal/agent"
	"github.com/charmbracelet/lipgloss"
)

// pendingCommandConfirm 保存一个等待用户 y/n 确认的 run_command 调用。
type pendingCommandConfirm struct {
	tc *agent.ToolCall
}

// needsCommandConfirm 判断某个工具调用在给定 yolo 模式下是否需要 confirm。
// 唯一需要 confirm 的是 run_command；yolo=true 则一律放行。
func needsCommandConfirm(tc *agent.ToolCall, yolo bool) bool {
	if tc == nil || yolo {
		return false
	}
	return tc.Name == "run_command"
}

var (
	confirmHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F59E0B")).
				Bold(true)
	confirmBodyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E5E7EB"))
	confirmHintStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#9CA3AF")).
				Italic(true)
)

// renderCommandConfirmPrompt 生成展示给用户的确认提示行。
func renderCommandConfirmPrompt(tc *agent.ToolCall) string {
	if tc == nil {
		return ""
	}
	command, _ := tc.Arguments["command"].(string)
	cwd, _ := tc.Arguments["cwd"].(string)

	var sb strings.Builder
	sb.WriteString(confirmHeaderStyle.Render("⚠  需要授权：run_command"))
	sb.WriteByte('\n')
	if cwd != "" {
		sb.WriteString(confirmBodyStyle.Render(fmt.Sprintf("  cwd: %s", cwd)))
		sb.WriteByte('\n')
	}
	sb.WriteString(confirmBodyStyle.Render(fmt.Sprintf("  cmd: %s", command)))
	sb.WriteByte('\n')
	sb.WriteString(confirmHintStyle.Render("  按 y 允许 / n 拒绝 / Esc 取消"))
	return sb.String()
}

// commandRejectedResult 生成用户拒绝执行时返回给模型的 ToolResult。
func commandRejectedResult() agent.ToolResult {
	return agent.ToolResult{
		Success: false,
		Output:  "用户拒绝执行该命令",
		Error:   "user_rejected",
	}
}
