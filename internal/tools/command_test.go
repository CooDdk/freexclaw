package tools

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCommand_SucceedsInWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	SetWorkDir(tmpDir)

	result, err := RunCommand("go env GOOS", "")
	if err != nil {
		t.Fatalf("expected command to succeed, got %v", err)
	}
	if result.Cwd != tmpDir {
		t.Fatalf("expected cwd %q, got %q", tmpDir, result.Cwd)
	}
	if strings.TrimSpace(result.Stdout) == "" {
		t.Fatal("expected stdout to be non-empty")
	}
}

func TestRunCommand_RejectsDangerousCommand(t *testing.T) {
	SetWorkDir(t.TempDir())

	_, err := RunCommand("git reset --hard", "")
	if err == nil {
		t.Fatal("expected dangerous command to be rejected")
	}
	if !strings.Contains(err.Error(), "高风险命令") {
		t.Fatalf("expected dangerous command error, got %v", err)
	}
}

func TestRunCommand_RejectsOutsideWorkspaceCwd(t *testing.T) {
	root := t.TempDir()
	SetWorkDir(root)

	outside := filepath.Dir(root)
	_, err := RunCommand("go env GOARCH", outside)
	if err == nil {
		t.Fatal("expected cwd outside workspace to fail")
	}
	if !strings.Contains(err.Error(), "超出当前工作区") {
		t.Fatalf("expected workspace boundary error, got %v", err)
	}
}
