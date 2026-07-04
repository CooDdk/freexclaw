package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CooDdk/freexclaw/internal/config"
	"github.com/CooDdk/freexclaw/internal/conversation"
	"github.com/CooDdk/freexclaw/internal/tools"
)

func TestDetectRuntimePromptProfile_CodingRequest(t *testing.T) {
	profile := detectRuntimePromptProfile("使用 gin 框架写一个简单的 api 服务", nil)
	if profile != runtimeProfileEngineering {
		t.Fatalf("expected %q, got %q", runtimeProfileEngineering, profile)
	}
}

func TestIsProjectScaffoldRequest(t *testing.T) {
	if !isProjectScaffoldRequest("使用 nestjs 写一个简单的 api 服务") {
		t.Fatal("expected nestjs scaffold request to be treated as project scaffold")
	}
	if isProjectScaffoldRequest("帮我解释一下 package.json") {
		t.Fatal("did not expect plain file explanation to be treated as project scaffold")
	}
}

func TestDetectRuntimePromptProfile_FollowUpWithCodingContext(t *testing.T) {
	messages := []conversation.Message{
		{Role: conversation.RoleAssistant, Content: "<write_file>main.go\npackage main\n</write_file>"},
	}
	profile := detectRuntimePromptProfile("继续，加一个健康检查接口", messages)
	if profile != runtimeProfileEngineering {
		t.Fatalf("expected follow-up to keep coding runtime profile, got %q", profile)
	}
}

func TestBuildMessages_IncludesRuntimePromptSummary(t *testing.T) {
	root := t.TempDir()
	tools.SetWorkDir(root)
	t.Setenv("APPDATA", filepath.Join(root, "appdata"))

	promptDir := filepath.Join(root, ".freexclaw", "prompts")
	if err := os.MkdirAll(promptDir, 0755); err != nil {
		t.Fatalf("mkdir prompts: %v", err)
	}
	templateDir := filepath.Join(root, ".freexclaw", "templates")
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		t.Fatalf("mkdir templates: %v", err)
	}
	runtimeText := "# Runtime Summary\n\n- keep it short"
	if err := os.WriteFile(filepath.Join(promptDir, "coding-runtime.md"), []byte(runtimeText), 0644); err != nil {
		t.Fatalf("write runtime prompt: %v", err)
	}
	skillDir := filepath.Join(root, ".freexclaw", "skills", "engineering-delivery")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	skillText := "# Engineering Delivery\n\n- inspect workspace first\n- create multi-file project structure\n- run safe verification commands\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillText), 0644); err != nil {
		t.Fatalf("write skill summary: %v", err)
	}
	templateText := "# Delivery Report\n\n## 变更文件\n\n## 执行命令\n\n## 校验结果\n\n## 下一步\n"
	if err := os.WriteFile(filepath.Join(templateDir, "delivery-report.md"), []byte(templateText), 0644); err != nil {
		t.Fatalf("write delivery template: %v", err)
	}

	m := &Model{
		cfg:                  &config.Config{},
		convMgr:              conversation.NewManager(root),
		runtimePromptProfile: runtimeProfileEngineering,
		runtimePromptSummary: loadRuntimePrompt(runtimeProfileEngineering),
		turnTouchedFiles:     []string{filepath.Join(root, "main.go")},
		turnExecutedCommands: []string{"go test ./..."},
	}
	defer m.convMgr.Close()
	m.convMgr.GetCurrent().AddMessage(conversation.RoleUser, "帮我创建一个 Go 服务")

	messages := m.buildMessages()
	if len(messages) < 3 {
		t.Fatalf("expected runtime and delivery system messages to be included, got %d messages", len(messages))
	}
	if !strings.Contains(messages[1].Content, "当前任务模式: "+runtimeProfileEngineering) {
		t.Fatalf("expected runtime profile marker, got %q", messages[1].Content)
	}
	if !strings.Contains(messages[1].Content, "keep it short") {
		t.Fatalf("expected short runtime prompt summary, got %q", messages[1].Content)
	}
	if !strings.Contains(messages[1].Content, "inspect workspace first") {
		t.Fatalf("expected skill summary to be included, got %q", messages[1].Content)
	}
	if !strings.Contains(messages[2].Content, "变更文件 / 执行命令 / 校验结果 / 下一步") {
		t.Fatalf("expected condensed delivery template summary, got %q", messages[2].Content)
	}
}

func TestBuildDeliverySystemMessage_RequiresActualWork(t *testing.T) {
	msg := buildDeliverySystemMessage(runtimeProfileEngineering, nil, nil)
	if msg != "" {
		t.Fatalf("expected empty delivery message without touched files or commands, got %q", msg)
	}
}
