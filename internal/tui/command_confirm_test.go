package tui

import (
	"strings"
	"testing"

	"github.com/CooDdk/freexclaw/internal/agent"
	"github.com/CooDdk/freexclaw/internal/conversation"
)

func TestNeedsCommandConfirm_OnlyRunCommandWhenNotYolo(t *testing.T) {
	cases := []struct {
		name string
		tc   *agent.ToolCall
		yolo bool
		want bool
	}{
		{"nil", nil, false, false},
		{"read_file", &agent.ToolCall{Name: "read_file"}, false, false},
		{"write_file", &agent.ToolCall{Name: "write_file"}, false, false},
		{"edit_file", &agent.ToolCall{Name: "edit_file"}, false, false},
		{"grep", &agent.ToolCall{Name: "grep"}, false, false},
		{"glob", &agent.ToolCall{Name: "glob"}, false, false},
		{"run_command_needs_confirm", &agent.ToolCall{Name: "run_command"}, false, true},
		{"run_command_yolo_off", &agent.ToolCall{Name: "run_command"}, true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := needsCommandConfirm(c.tc, c.yolo); got != c.want {
				t.Fatalf("want %v, got %v", c.want, got)
			}
		})
	}
}

func TestRenderCommandConfirmPrompt_ShowsCwdAndCmd(t *testing.T) {
	tc := &agent.ToolCall{
		Name: "run_command",
		Arguments: map[string]interface{}{
			"cwd":     "internal/tools",
			"command": "go test ./...",
		},
	}
	out := renderCommandConfirmPrompt(tc)
	if !strings.Contains(out, "cwd: internal/tools") {
		t.Fatalf("expected cwd line, got %q", out)
	}
	if !strings.Contains(out, "cmd: go test ./...") {
		t.Fatalf("expected cmd line, got %q", out)
	}
	if !strings.Contains(out, "按 y 允许") {
		t.Fatalf("expected y/n hint, got %q", out)
	}
}

func TestRenderCommandConfirmPrompt_OmitsEmptyCwd(t *testing.T) {
	tc := &agent.ToolCall{
		Name: "run_command",
		Arguments: map[string]interface{}{
			"command": "ls",
		},
	}
	out := renderCommandConfirmPrompt(tc)
	if strings.Contains(out, "cwd:") {
		t.Fatalf("empty cwd should be omitted, got %q", out)
	}
	if !strings.Contains(out, "cmd: ls") {
		t.Fatalf("expected cmd line, got %q", out)
	}
}

func TestCommandRejectedResult_MarksFailure(t *testing.T) {
	r := commandRejectedResult()
	if r.Success {
		t.Fatalf("expected Success=false")
	}
	if !strings.Contains(r.Output, "拒绝") {
		t.Fatalf("expected 拒绝 in Output, got %q", r.Output)
	}
	if r.Error == "" {
		t.Fatalf("expected non-empty Error tag")
	}
}

func TestStartToolExecution_RunCommandSetsPendingConfirmWhenNotYolo(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	m := &Model{
		convMgr: conversation.NewManager(root),
		yolo:    false,
	}
	defer m.convMgr.Close()

	tc := &agent.ToolCall{
		Name:      "run_command",
		Arguments: map[string]interface{}{"command": "ls"},
	}
	cmd := m.startToolExecution(tc)

	if m.pendingConfirm == nil {
		t.Fatalf("expected pendingConfirm to be set")
	}
	if m.pendingConfirm.tc != tc {
		t.Fatalf("pendingConfirm should hold the original tc")
	}
	if m.isToolRunning {
		t.Fatalf("isToolRunning must stay false until user confirms")
	}
	if cmd == nil {
		t.Fatalf("expected a Println cmd for the prompt")
	}
}

func TestStartToolExecution_RunCommandBypassesConfirmWhenYolo(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	m := &Model{
		convMgr: conversation.NewManager(root),
		yolo:    true,
	}
	defer m.convMgr.Close()

	tc := &agent.ToolCall{
		Name:      "run_command",
		Arguments: map[string]interface{}{"command": "ls"},
	}
	_ = m.startToolExecution(tc)

	if m.pendingConfirm != nil {
		t.Fatalf("yolo mode should skip pendingConfirm")
	}
	if !m.isToolRunning {
		t.Fatalf("yolo mode should start execution immediately")
	}
}

func TestStartToolExecution_NonRunCommandNeverAsks(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	m := &Model{
		convMgr: conversation.NewManager(root),
		yolo:    false,
	}
	defer m.convMgr.Close()

	tc := &agent.ToolCall{
		Name:      "read_file",
		Arguments: map[string]interface{}{"path": "go.mod"},
	}
	_ = m.startToolExecution(tc)

	if m.pendingConfirm != nil {
		t.Fatalf("non-run_command should never trigger confirm")
	}
	if !m.isToolRunning {
		t.Fatalf("non-run_command should start immediately")
	}
}
