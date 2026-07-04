package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CooDdk/freexclaw/internal/tools"
)

func TestEnsureTaskSpec_WritesArtifacts(t *testing.T) {
	root := t.TempDir()
	tools.SetWorkDir(root)
	defer tools.SetWorkDir("")

	arts, err := ensureTaskSpec("使用 nestjs 写一个简单的 api 服务", runtimeProfileEngineering)
	if err != nil {
		t.Fatalf("ensure task spec: %v", err)
	}

	for _, path := range []string{arts.CurrentTaskPath, arts.TasksPath, arts.ChecklistPath, arts.ChangesPath, arts.StatePath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected artifact %s to exist: %v", path, err)
		}
	}

	state, err := loadTaskSpecState()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state.Status != specStatusAwaitingApproval {
		t.Fatalf("expected awaiting approval, got %q", state.Status)
	}

	data, err := os.ReadFile(arts.TasksPath)
	if err != nil {
		t.Fatalf("read tasks: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "创建/更新 src/main.ts") {
		t.Fatalf("expected dynamic file task in tasks.md, got %q", text)
	}
	if !strings.Contains(text, "创建/更新 src/health/health.controller.ts") {
		t.Fatalf("expected nested nestjs file task in tasks.md, got %q", text)
	}
	if !strings.Contains(text, "执行 npm install") {
		t.Fatalf("expected dynamic init command task in tasks.md, got %q", text)
	}

	changeData, err := os.ReadFile(arts.ChangesPath)
	if err != nil {
		t.Fatalf("read changes: %v", err)
	}
	changeText := string(changeData)
	if !strings.Contains(changeText, "## Requirement Changes") {
		t.Fatalf("expected requirement section in changes.md, got %q", changeText)
	}
	if !strings.Contains(changeText, "## Execution Failures") {
		t.Fatalf("expected failure section in changes.md, got %q", changeText)
	}

	currentTaskData, err := os.ReadFile(arts.CurrentTaskPath)
	if err != nil {
		t.Fatalf("read current task: %v", err)
	}
	currentTaskText := string(currentTaskData)
	if !strings.Contains(currentTaskText, "```text\n.\n├── package.json") {
		t.Fatalf("expected tree structure in current-task.md, got %q", currentTaskText)
	}
}

func TestBuildTaskSpecExecutionMessage_UsesApprovedDocs(t *testing.T) {
	root := t.TempDir()
	tools.SetWorkDir(root)
	defer tools.SetWorkDir("")

	if _, err := ensureTaskSpec("使用 nestjs 写一个简单的 api 服务", runtimeProfileEngineering); err != nil {
		t.Fatalf("ensure task spec: %v", err)
	}

	state, err := loadTaskSpecState()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	state.Status = specStatusApproved
	if err := saveTaskSpecState(state); err != nil {
		t.Fatalf("save approved state: %v", err)
	}

	msg := buildTaskSpecExecutionMessage(runtimeProfileEngineering)
	if !strings.Contains(msg, "current-task.md") {
		t.Fatalf("expected current-task content in execution message, got %q", msg)
	}
	if !strings.Contains(msg, "src/main.ts") {
		t.Fatalf("expected nestjs structure hint in execution message, got %q", msg)
	}
}

func TestIsExecutionApproval(t *testing.T) {
	if !isExecutionApproval("可以") {
		t.Fatal("expected 可以 to be approval")
	}
	if !isExecutionApproval("继续执行") {
		t.Fatal("expected 继续执行 to be approval")
	}
	if isExecutionApproval("把 controller 再拆一下") {
		t.Fatal("did not expect ordinary change request to be approval")
	}
}

func TestMarkTaskSpecApproved_UpdatesTasks(t *testing.T) {
	root := t.TempDir()
	tools.SetWorkDir(root)
	defer tools.SetWorkDir("")

	if _, err := ensureTaskSpec("使用 nestjs 写一个简单的 api 服务", runtimeProfileEngineering); err != nil {
		t.Fatalf("ensure task spec: %v", err)
	}
	if err := markTaskSpecApproved(); err != nil {
		t.Fatalf("mark approved: %v", err)
	}

	data, err := os.ReadFile(specArtifacts().TasksPath)
	if err != nil {
		t.Fatalf("read tasks: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "- [x] "+taskReview) {
		t.Fatalf("expected review task completed, got %q", text)
	}
	if !strings.Contains(text, "- [>] "+taskInspect) {
		t.Fatalf("expected inspect task in progress, got %q", text)
	}
	if !strings.Contains(text, taskPlanStructure) {
		t.Fatalf("expected structure planning task, got %q", text)
	}
}

func TestUpdateTaskSpecProgressForTool_ListDirAdvancesTasks(t *testing.T) {
	root := t.TempDir()
	tools.SetWorkDir(root)
	defer tools.SetWorkDir("")

	if _, err := ensureTaskSpec("使用 nestjs 写一个简单的 api 服务", runtimeProfileEngineering); err != nil {
		t.Fatalf("ensure task spec: %v", err)
	}
	if err := markTaskSpecApproved(); err != nil {
		t.Fatalf("mark approved: %v", err)
	}
	if err := updateTaskSpecProgressForTool("list_dir", true, nil, ""); err != nil {
		t.Fatalf("update list_dir progress: %v", err)
	}

	data, err := os.ReadFile(specArtifacts().TasksPath)
	if err != nil {
		t.Fatalf("read tasks: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "- [x] "+taskInspect) {
		t.Fatalf("expected inspect task completed, got %q", text)
	}
	if !strings.Contains(text, "- [>] "+taskPlanStructure) {
		t.Fatalf("expected structure planning task in progress, got %q", text)
	}
}

func TestUpdateTaskSpecProgressForTool_WriteSubstantiveFileAdvancesTasks(t *testing.T) {
	root := t.TempDir()
	tools.SetWorkDir(root)
	defer tools.SetWorkDir("")

	if _, err := ensureTaskSpec("使用 nestjs 写一个简单的 api 服务", runtimeProfileEngineering); err != nil {
		t.Fatalf("ensure task spec: %v", err)
	}
	if err := markTaskSpecApproved(); err != nil {
		t.Fatalf("mark approved: %v", err)
	}
	touched := []string{filepath.Join(root, "src", "main.ts")}
	if err := updateTaskSpecProgressForTool("write_file", true, touched, ""); err != nil {
		t.Fatalf("update write_file progress: %v", err)
	}

	data, err := os.ReadFile(specArtifacts().TasksPath)
	if err != nil {
		t.Fatalf("read tasks: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "- [x] "+taskScaffold) {
		t.Fatalf("expected scaffold task completed, got %q", text)
	}
	if !strings.Contains(text, "- [>] "+taskInit) {
		t.Fatalf("expected init task in progress, got %q", text)
	}
	if !strings.Contains(text, "- [x] 创建/更新 src/main.ts") {
		t.Fatalf("expected file-level task completed, got %q", text)
	}
}

func TestEnsureTaskSpec_RequestChangeKeepsProgressAndUpdatesChanges(t *testing.T) {
	root := t.TempDir()
	tools.SetWorkDir(root)
	defer tools.SetWorkDir("")

	if _, err := ensureTaskSpec("使用 nestjs 写一个简单的 api 服务", runtimeProfileEngineering); err != nil {
		t.Fatalf("ensure task spec: %v", err)
	}
	if err := markTaskSpecApproved(); err != nil {
		t.Fatalf("mark approved: %v", err)
	}
	touched := []string{filepath.Join(root, "src", "main.ts")}
	if err := updateTaskSpecProgressForTool("write_file", true, touched, ""); err != nil {
		t.Fatalf("update write_file progress: %v", err)
	}

	if _, err := ensureTaskSpec("使用 nestjs 写一个简单的 api 服务，并增加 health 接口", runtimeProfileEngineering); err != nil {
		t.Fatalf("ensure changed task spec: %v", err)
	}

	taskData, err := os.ReadFile(specArtifacts().TasksPath)
	if err != nil {
		t.Fatalf("read tasks after change: %v", err)
	}
	taskText := string(taskData)
	if !strings.Contains(taskText, "- [>] "+taskReview) {
		t.Fatalf("expected review task reopened after request change, got %q", taskText)
	}
	if !strings.Contains(taskText, "- [x] 创建/更新 src/main.ts") {
		t.Fatalf("expected file progress preserved after request change, got %q", taskText)
	}

	currentTask, err := os.ReadFile(specArtifacts().CurrentTaskPath)
	if err != nil {
		t.Fatalf("read current task: %v", err)
	}
	if !strings.Contains(string(currentTask), "health 接口") {
		t.Fatalf("expected current-task.md updated with new request, got %q", string(currentTask))
	}

	changeData, err := os.ReadFile(specArtifacts().ChangesPath)
	if err != nil {
		t.Fatalf("read changes: %v", err)
	}
	changeText := string(changeData)
	if !strings.Contains(changeText, "## Requirement Changes") {
		t.Fatalf("expected requirement section preserved, got %q", changeText)
	}
	if !strings.Contains(changeText, "Previous Request: 使用 nestjs 写一个简单的 api 服务") {
		t.Fatalf("expected previous request recorded, got %q", changeText)
	}
	if !strings.Contains(changeText, "Current Request: 使用 nestjs 写一个简单的 api 服务，并增加 health 接口") {
		t.Fatalf("expected current request recorded, got %q", changeText)
	}
}

func TestRecordTaskSpecFailure_AppendsChangeEntry(t *testing.T) {
	root := t.TempDir()
	tools.SetWorkDir(root)
	defer tools.SetWorkDir("")

	if _, err := ensureTaskSpec("使用 nestjs 写一个简单的 api 服务", runtimeProfileEngineering); err != nil {
		t.Fatalf("ensure task spec: %v", err)
	}
	if err := recordTaskSpecFailure("run_command", "npm install", "network timeout"); err != nil {
		t.Fatalf("record failure: %v", err)
	}

	data, err := os.ReadFile(specArtifacts().ChangesPath)
	if err != nil {
		t.Fatalf("read changes: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "## Execution Failures") {
		t.Fatalf("expected failure section heading, got %q", text)
	}
	if !strings.Contains(text, "Failure Tool: run_command") {
		t.Fatalf("expected failure tool recorded, got %q", text)
	}
	if !strings.Contains(text, "Failure Target: npm install") {
		t.Fatalf("expected failure target recorded, got %q", text)
	}
	if !strings.Contains(text, "Failure Detail: network timeout") {
		t.Fatalf("expected failure detail recorded, got %q", text)
	}
}

func TestBuildPlanningNotice_UsesTreeStructure(t *testing.T) {
	notice := buildPlanningNotice("使用 nestjs 写一个简单的 api 服务")
	if !strings.Contains(notice, "```text\n.\n├── package.json") {
		t.Fatalf("expected tree structure in planning notice, got %q", notice)
	}
	if !strings.Contains(notice, "└── src") {
		t.Fatalf("expected src subtree in planning notice, got %q", notice)
	}
}
