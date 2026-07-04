package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CooDdk/freexclaw/internal/tools"
)

func TestBuildForcedPostProcessToolCalls_GoProject(t *testing.T) {
	root := t.TempDir()
	tools.SetWorkDir(root)

	mainFile := filepath.Join(root, "main.go")
	if err := os.WriteFile(mainFile, []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	toolCalls := buildForcedPostProcessToolCalls("使用 gin 框架写一个简单的 api 服务", []string{mainFile})
	if len(toolCalls) != 5 {
		t.Fatalf("expected 5 tool calls, got %d", len(toolCalls))
	}

	want := []string{
		"go mod init " + filepath.Base(root),
		"go mod tidy",
		"go test ./...",
		"go vet ./...",
		"go build ./...",
	}
	for i, command := range want {
		if got := toolCalls[i].Arguments["command"]; got != command {
			t.Fatalf("tool call %d expected %q, got %#v", i, command, got)
		}
	}
}

func TestBuildForcedPostProcessToolCalls_SkipsNonProjectRequest(t *testing.T) {
	root := t.TempDir()
	tools.SetWorkDir(root)

	file := filepath.Join(root, "main.go")
	if err := os.WriteFile(file, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	toolCalls := buildForcedPostProcessToolCalls("帮我分析一下这个文件", []string{file})
	if len(toolCalls) != 0 {
		t.Fatalf("expected no tool calls, got %d", len(toolCalls))
	}
}

func TestBuildForcedPostProcessToolCalls_ManifestOnlyDoesNotTrigger(t *testing.T) {
	root := t.TempDir()
	tools.SetWorkDir(root)

	packageJSON := filepath.Join(root, "package.json")
	tsconfig := filepath.Join(root, "tsconfig.json")
	if err := os.WriteFile(packageJSON, []byte("{\"name\":\"demo\"}\n"), 0644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	if err := os.WriteFile(tsconfig, []byte("{\"compilerOptions\":{}}\n"), 0644); err != nil {
		t.Fatalf("write tsconfig.json: %v", err)
	}

	toolCalls := buildForcedPostProcessToolCalls("使用 nestjs 写一个简单的 api 服务", []string{packageJSON, tsconfig})
	if len(toolCalls) != 0 {
		t.Fatalf("expected no tool calls for manifest-only scaffold state, got %d", len(toolCalls))
	}
}
