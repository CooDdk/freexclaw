package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/CooDdk/freexclaw/internal/tools"
)

const (
	specStatusAwaitingApproval = "awaiting_approval"
	specStatusApproved         = "approved"

	taskReview        = "需求确认与方案审阅"
	taskInspect       = "检查当前工作区结构"
	taskPlanStructure = "确认目标项目结构与文件清单"
	taskScaffold      = "创建或调整项目目录与源码文件"
	taskInit          = "初始化依赖与配置"
	taskValidate      = "执行基础验证命令"
	taskHandoff       = "汇总交付结果与剩余风险"
)

type taskSpecState struct {
	Status    string `json:"status"`
	Profile   string `json:"profile"`
	Request   string `json:"request"`
	UpdatedAt string `json:"updated_at"`
}

type taskSpecArtifacts struct {
	SpecDir         string
	CurrentTaskPath string
	TasksPath       string
	ChangesPath     string
	ChecklistPath   string
	StatePath       string
}

type specTask struct {
	Label   string
	Section string
	Kind    string
	Ref     string
}

const (
	changeSectionRequirement = "Requirement Changes"
	changeSectionFailure     = "Execution Failures"
)

func specArtifacts() taskSpecArtifacts {
	specDir := filepath.Join(tools.GetWorkDir(), ".freexclaw", "spec")
	return taskSpecArtifacts{
		SpecDir:         specDir,
		CurrentTaskPath: filepath.Join(specDir, "current-task.md"),
		TasksPath:       filepath.Join(specDir, "tasks.md"),
		ChangesPath:     filepath.Join(specDir, "changes.md"),
		ChecklistPath:   filepath.Join(specDir, "checklist.md"),
		StatePath:       filepath.Join(specDir, "state.json"),
	}
}

func shouldUseEngineeringPlanning(profile string, prompt string) bool {
	return strings.TrimSpace(profile) == runtimeProfileEngineering && strings.TrimSpace(prompt) != ""
}

func isExecutionApproval(content string) bool {
	content = strings.ToLower(strings.TrimSpace(content))
	if content == "" {
		return false
	}

	approvals := []string{
		"可以", "好", "好的", "开始", "执行", "开始吧", "继续", "继续执行",
		"按这个来", "按这个做", "照这个做", "确认", "确认执行", "可以开始",
		"开始开发", "开始实现", "开始写吧", "就这样",
	}
	for _, item := range approvals {
		if content == item {
			return true
		}
	}
	return false
}

func ensureTaskSpec(prompt string, profile string) (taskSpecArtifacts, error) {
	arts := specArtifacts()
	if err := os.MkdirAll(arts.SpecDir, 0755); err != nil {
		return arts, err
	}

	prev, _ := loadTaskSpecState()
	stack := detectEngineeringStack(prompt)
	existingStatuses, _ := readTaskStatuses()
	tasks := buildSpecTasks(stack)
	statuses := initialTaskStatuses(tasks, existingStatuses, prev, prompt)

	if err := os.WriteFile(arts.CurrentTaskPath, []byte(renderCurrentTaskDoc(prompt, stack)), 0644); err != nil {
		return arts, err
	}
	if err := os.WriteFile(arts.TasksPath, []byte(renderTasksDoc(stack, statuses)), 0644); err != nil {
		return arts, err
	}
	if err := os.WriteFile(arts.ChecklistPath, []byte(renderChecklistDoc(stack)), 0644); err != nil {
		return arts, err
	}

	changeEntry := buildChangeEntry(prev.Request, prompt)
	if strings.TrimSpace(changeEntry) != "" {
		if err := appendChangeEntryToSection(arts.ChangesPath, changeSectionRequirement, changeEntry); err != nil {
			return arts, err
		}
	} else if _, err := os.Stat(arts.ChangesPath); os.IsNotExist(err) {
		if err := ensureChangesDoc(arts.ChangesPath); err != nil {
			return arts, err
		}
	}

	state := taskSpecState{
		Status:    specStatusAwaitingApproval,
		Profile:   profile,
		Request:   prompt,
		UpdatedAt: time.Now().Format("2006-01-02 15:04:05"),
	}
	if err := saveTaskSpecState(state); err != nil {
		return arts, err
	}
	return arts, nil
}

func loadTaskSpecState() (taskSpecState, error) {
	data, err := os.ReadFile(specArtifacts().StatePath)
	if err != nil {
		return taskSpecState{}, err
	}
	var state taskSpecState
	if err := json.Unmarshal(data, &state); err != nil {
		return taskSpecState{}, err
	}
	return state, nil
}

func saveTaskSpecState(state taskSpecState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(specArtifacts().StatePath, data, 0644)
}

func markTaskSpecApproved() error {
	state, err := loadTaskSpecState()
	if err != nil {
		return err
	}
	state.Status = specStatusApproved
	state.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
	if err := saveTaskSpecState(state); err != nil {
		return err
	}
	statuses, _ := readTaskStatuses()
	statuses[taskReview] = "completed"
	statuses[taskInspect] = "in_progress"
	statuses[taskPlanStructure] = "pending"
	statuses[taskHandoff] = "pending"
	return saveTaskStatusesForStack(detectEngineeringStack(state.Request), statuses)
}

func updateTaskSpecProgressForTool(toolName string, success bool, touchedFiles []string, command string) error {
	if !success {
		return nil
	}
	state, err := loadTaskSpecState()
	if err != nil || state.Status != specStatusApproved {
		return err
	}
	stack := detectEngineeringStack(state.Request)
	statuses, _ := readTaskStatuses()
	statuses[taskReview] = "completed"

	switch toolName {
	case "list_dir", "read_file":
		statuses[taskInspect] = "completed"
		if statuses[taskPlanStructure] != "completed" {
			statuses[taskPlanStructure] = "in_progress"
		}
	case "write_file", "append_file":
		statuses[taskInspect] = "completed"
		statuses[taskPlanStructure] = "completed"
		for _, label := range matchedFileTaskLabels(stack, touchedFiles) {
			statuses[label] = "completed"
		}
		if hasSubstantiveProjectFiles(touchedFiles) {
			statuses[taskScaffold] = "completed"
			statuses[taskInit] = "in_progress"
		} else {
			statuses[taskScaffold] = "in_progress"
		}
	case "run_command":
		statuses[taskInspect] = "completed"
		statuses[taskPlanStructure] = "completed"
		class := classifyEngineeringCommand(command)
		for _, label := range matchedCommandTaskLabels(stack, command, class) {
			statuses[label] = "completed"
		}
		switch class {
		case "init":
			statuses[taskScaffold] = taskStatusForTouchedFiles(touchedFiles)
			statuses[taskInit] = "completed"
			statuses[taskValidate] = "in_progress"
		case "validate":
			statuses[taskScaffold] = taskStatusForTouchedFiles(touchedFiles)
			statuses[taskInit] = "completed"
			statuses[taskValidate] = "completed"
			statuses[taskHandoff] = "in_progress"
		}
	}
	return saveTaskStatusesForStack(stack, statuses)
}

func completeTaskSpecHandoffIfNeeded(profile string, touchedFiles []string, commands []string, assistantContent string) error {
	if strings.TrimSpace(profile) != runtimeProfileEngineering {
		return nil
	}
	if len(touchedFiles) == 0 && len(commands) == 0 {
		return nil
	}
	if strings.TrimSpace(assistantContent) == "" {
		return nil
	}
	state, err := loadTaskSpecState()
	if err != nil || state.Status != specStatusApproved {
		return err
	}
	stack := detectEngineeringStack(state.Request)
	statuses, _ := readTaskStatuses()
	statuses[taskReview] = "completed"
	statuses[taskInspect] = "completed"
	statuses[taskPlanStructure] = "completed"
	statuses[taskScaffold] = taskStatusForTouchedFiles(touchedFiles)
	statuses[taskInit] = taskStatusForCommands(commands, false)
	statuses[taskValidate] = taskStatusForCommands(commands, true)
	statuses[taskHandoff] = "completed"
	return saveTaskStatusesForStack(stack, statuses)
}

func taskStatusForTouchedFiles(touchedFiles []string) string {
	if hasSubstantiveProjectFiles(touchedFiles) {
		return "completed"
	}
	if len(touchedFiles) > 0 {
		return "in_progress"
	}
	return "pending"
}

func taskStatusForCommands(commands []string, validation bool) string {
	hasInit := false
	hasValidate := false
	for _, command := range commands {
		switch classifyEngineeringCommand(command) {
		case "init":
			hasInit = true
		case "validate":
			hasValidate = true
		}
	}
	if validation {
		if hasValidate {
			return "completed"
		}
		return "pending"
	}
	if hasInit {
		return "completed"
	}
	return "pending"
}

func classifyEngineeringCommand(command string) string {
	lower := strings.ToLower(strings.TrimSpace(command))
	switch {
	case lower == "":
		return ""
	case strings.Contains(lower, "npm install"),
		strings.Contains(lower, "npm init"),
		strings.Contains(lower, "pnpm install"),
		strings.Contains(lower, "yarn install"),
		strings.Contains(lower, "go mod init"),
		strings.Contains(lower, "go mod tidy"),
		strings.Contains(lower, "pip install"),
		strings.Contains(lower, "poetry install"):
		return "init"
	case strings.Contains(lower, "npm test"),
		strings.Contains(lower, "npm run build"),
		strings.Contains(lower, "go test"),
		strings.Contains(lower, "go vet"),
		strings.Contains(lower, "go build"),
		strings.Contains(lower, "compileall"),
		strings.Contains(lower, "pytest"),
		strings.Contains(lower, "py_compile"):
		return "validate"
	default:
		return ""
	}
}

func saveTaskStatusesForStack(stack string, statuses map[string]string) error {
	normalized := normalizeTaskStatuses(buildSpecTasks(stack), statuses)
	return os.WriteFile(specArtifacts().TasksPath, []byte(renderTasksDoc(stack, normalized)), 0644)
}

func normalizeTaskStatuses(tasks []specTask, statuses map[string]string) map[string]string {
	normalized := make(map[string]string, len(tasks))
	for _, task := range tasks {
		status := strings.TrimSpace(statuses[task.Label])
		if status == "" {
			status = "pending"
		}
		normalized[task.Label] = status
	}

	fileLabels := taskLabelsByKind(tasks, "file")
	initLabels := taskLabelsByKind(tasks, "init_command")
	validateLabels := taskLabelsByKind(tasks, "validate_command")

	normalized[taskScaffold] = mergeTaskStatuses(normalized[taskScaffold], aggregateChildTaskStatus(fileLabels, normalized))
	normalized[taskInit] = mergeTaskStatuses(normalized[taskInit], aggregateChildTaskStatus(initLabels, normalized))
	normalized[taskValidate] = mergeTaskStatuses(normalized[taskValidate], aggregateChildTaskStatus(validateLabels, normalized))

	if normalized[taskInspect] == "completed" && normalized[taskPlanStructure] == "pending" {
		if normalized[taskScaffold] != "pending" || normalized[taskInit] != "pending" || normalized[taskValidate] != "pending" {
			normalized[taskPlanStructure] = "completed"
		} else {
			normalized[taskPlanStructure] = "in_progress"
		}
	}

	return normalized
}

func taskLabelsByKind(tasks []specTask, kind string) []string {
	labels := make([]string, 0, len(tasks))
	for _, task := range tasks {
		if task.Kind == kind {
			labels = append(labels, task.Label)
		}
	}
	return labels
}

func aggregateChildTaskStatus(labels []string, statuses map[string]string) string {
	if len(labels) == 0 {
		return "pending"
	}
	completed := 0
	inProgress := false
	for _, label := range labels {
		switch statuses[label] {
		case "completed":
			completed++
		case "in_progress":
			inProgress = true
		}
	}
	switch {
	case completed == len(labels):
		return "completed"
	case completed > 0 || inProgress:
		return "in_progress"
	default:
		return "pending"
	}
}

func mergeTaskStatuses(current string, derived string) string {
	rank := func(status string) int {
		switch status {
		case "completed":
			return 3
		case "in_progress":
			return 2
		default:
			return 1
		}
	}
	if rank(derived) > rank(current) {
		return derived
	}
	if strings.TrimSpace(current) == "" {
		return derived
	}
	return current
}

func applyTaskStatuses(statuses map[string]string) error {
	arts := specArtifacts()
	data, err := os.ReadFile(arts.TasksPath)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		for task, status := range statuses {
			targets := []string{
				"- [ ] " + task,
				"- [x] " + task,
				"- [>] " + task,
			}
			matched := false
			for _, target := range targets {
				if trimmed == target {
					lines[i] = strings.Replace(line, target, "- ["+taskMarker(status)+"] "+task, 1)
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
	}
	return os.WriteFile(arts.TasksPath, []byte(strings.Join(lines, "\n")), 0644)
}

func taskMarker(status string) string {
	switch status {
	case "completed":
		return "x"
	case "in_progress":
		return ">"
	default:
		return " "
	}
}

func buildPlanningNotice(prompt string) string {
	stack := detectEngineeringStack(prompt)
	arts := specArtifacts()
	structure := renderStructureTree(stack)
	steps := strings.Join(suggestedSteps(stack), " / ")
	return "已生成内部开发文档，请先确认后再执行。\n\n" +
		"文档位置：\n" +
		"- " + arts.CurrentTaskPath + "\n" +
		"- " + arts.TasksPath + "\n" +
		"- " + arts.ChecklistPath + "\n" +
		"- " + arts.ChangesPath + "\n\n" +
		"建议结构：\n```text\n" + structure + "\n```\n\n" +
		"开发步骤：\n- " + steps + "\n\n" +
		"如需调整，请直接说你的修改意见；确认后我再开始真正写代码和执行命令。"
}

func buildTaskSpecExecutionMessage(profile string) string {
	if strings.TrimSpace(profile) != runtimeProfileEngineering {
		return ""
	}
	state, err := loadTaskSpecState()
	if err != nil || state.Status != specStatusApproved {
		return ""
	}

	arts := specArtifacts()
	currentTask, err1 := os.ReadFile(arts.CurrentTaskPath)
	tasks, err2 := os.ReadFile(arts.TasksPath)
	if err1 != nil || err2 != nil {
		return ""
	}

	return "执行前请严格遵循 .freexclaw/spec 内部开发文档。先按任务步骤推进，若用户中途变更需求，先更新这些文档再继续执行。\n\n[current-task.md]\n" +
		strings.TrimSpace(string(currentTask)) +
		"\n\n[tasks.md]\n" +
		strings.TrimSpace(string(tasks))
}

func detectEngineeringStack(prompt string) string {
	lower := strings.ToLower(strings.TrimSpace(prompt))
	switch {
	case strings.Contains(lower, "nestjs") || strings.Contains(lower, "nest "):
		return "nestjs"
	case strings.Contains(lower, "go ") || strings.Contains(lower, "gin") || strings.Contains(lower, "golang"):
		return "go"
	case strings.Contains(lower, "python") || strings.Contains(lower, "fastapi") || strings.Contains(lower, "flask") || strings.Contains(lower, "django"):
		return "python"
	case strings.Contains(lower, "react") || strings.Contains(lower, "vue") || strings.Contains(lower, "next.js") || strings.Contains(lower, "nextjs"):
		return "frontend"
	case strings.Contains(lower, "node") || strings.Contains(lower, "typescript") || strings.Contains(lower, "javascript") || strings.Contains(lower, "express"):
		return "node"
	default:
		return "generic"
	}
}

func renderCurrentTaskDoc(prompt string, stack string) string {
	return "# Current Task\n\n" +
		"- Status: awaiting approval\n" +
		"- Mode: engineering-delivery\n" +
		"- Stack Hint: " + stack + "\n" +
		"- Request: " + strings.TrimSpace(prompt) + "\n\n" +
		"## Suggested Structure\n\n```text\n" + renderStructureTree(stack) + "\n```\n\n" +
		"## Development Steps\n\n1. " + strings.Join(suggestedSteps(stack), "\n1. ") + "\n"
}

func renderTasksDoc(stack string, statuses map[string]string) string {
	tasks := buildSpecTasks(stack)
	statuses = normalizeTaskStatuses(tasks, statuses)

	var sections []string
	for _, section := range []string{"Planning", "Implementation", "Initialization", "Validation", "Handoff"} {
		var lines []string
		for _, task := range tasks {
			if task.Section != section {
				continue
			}
			lines = append(lines, "- ["+taskMarker(statuses[task.Label])+"] "+task.Label)
		}
		if len(lines) == 0 {
			continue
		}
		sections = append(sections, "## "+section+"\n\n"+strings.Join(lines, "\n"))
	}
	return "# Tasks\n\n" + strings.Join(sections, "\n\n")
}

func renderChecklistDoc(stack string) string {
	return "# Checklist\n\n- [ ] 目录结构合理\n- [ ] 依赖文件齐全\n- [ ] 至少一个实际源码入口文件存在\n- [ ] 已执行安全的初始化/安装/校验命令\n- [ ] 结果已汇总到最终交付说明\n\n## Validation Hints\n\n- " + strings.Join(validationHints(stack), "\n- ") + "\n"
}

func suggestedStructure(stack string) []string {
	switch stack {
	case "nestjs":
		return []string{
			"package.json",
			"tsconfig.json",
			"nest-cli.json",
			"src/main.ts",
			"src/app.module.ts",
			"src/app.controller.ts",
			"src/app.service.ts",
			"src/health/health.controller.ts",
			"src/health/health.service.ts",
		}
	case "go":
		return []string{"go.mod", "main.go", "internal/... 或 pkg/...", "README 或示例配置"}
	case "python":
		return []string{"requirements.txt 或 pyproject.toml", "app.py / main.py", "package/module 目录", "tests/"}
	case "frontend":
		return []string{"package.json", "tsconfig.json", "src/main.ts(x)", "src/App.tsx 或页面模块", "必要配置文件"}
	case "node":
		return []string{"package.json", "tsconfig.json", "src/index.ts 或 src/main.ts", "路由/服务模块", "必要配置文件"}
	default:
		return []string{"依赖清单文件", "项目入口文件", "核心源码目录", "必要配置与验证文件"}
	}
}

func renderStructureTree(stack string) string {
	return strings.Join(suggestedStructureTreeLines(stack), "\n")
}

func suggestedStructureTreeLines(stack string) []string {
	switch stack {
	case "nestjs":
		return []string{
			".",
			"├── package.json",
			"├── tsconfig.json",
			"├── nest-cli.json",
			"└── src",
			"    ├── main.ts",
			"    ├── app.module.ts",
			"    ├── app.controller.ts",
			"    ├── app.service.ts",
			"    └── health",
			"        ├── health.controller.ts",
			"        └── health.service.ts",
		}
	case "go":
		return []string{
			".",
			"├── go.mod",
			"├── main.go",
			"└── internal",
			"    ├── handler",
			"    │   └── handler.go",
			"    └── service",
			"        └── service.go",
		}
	case "python":
		return []string{
			".",
			"├── requirements.txt",
			"├── main.py",
			"└── app",
			"    ├── __init__.py",
			"    └── api.py",
		}
	case "frontend":
		return []string{
			".",
			"├── package.json",
			"├── tsconfig.json",
			"└── src",
			"    ├── main.tsx",
			"    └── App.tsx",
		}
	case "node":
		return []string{
			".",
			"├── package.json",
			"├── tsconfig.json",
			"└── src",
			"    ├── main.ts",
			"    └── routes",
			"        └── index.ts",
		}
	default:
		return []string{
			".",
			"├── 依赖清单文件",
			"├── 项目入口文件",
			"├── 核心源码目录",
			"└── 必要配置与验证文件",
		}
	}
}

func suggestedSteps(stack string) []string {
	steps := []string{
		"检查工作区已有结构、读取关键配置文件",
		"明确最小可运行项目骨架和需要创建/修改的文件",
		"按多文件方式逐步创建源码、配置和依赖清单",
		"执行安全的初始化、依赖安装和基础校验命令",
		"如中途需求变更，先更新 .freexclaw/spec 文档再继续执行",
	}
	if stack == "nestjs" {
		steps[1] = "明确 NestJS 最小骨架，包括 src/main.ts、app.module、controller、service 等文件"
	}
	return steps
}

func validationHints(stack string) []string {
	switch stack {
	case "nestjs", "node", "frontend":
		return []string{"npm install", "npm run build", "npm test（若存在）"}
	case "go":
		return []string{"go mod tidy", "go test ./...", "go vet ./...", "go build ./..."}
	case "python":
		return []string{"python -m pip install -r requirements.txt 或 python -m pip install -e .", "python -m compileall .", "pytest（若存在）"}
	default:
		return []string{"初始化依赖", "执行可结束的构建或测试命令", "汇总验证结果"}
	}
}

func buildSpecTasks(stack string) []specTask {
	tasks := []specTask{
		{Label: taskReview, Section: "Planning", Kind: "phase"},
		{Label: taskInspect, Section: "Planning", Kind: "phase"},
		{Label: taskPlanStructure, Section: "Planning", Kind: "phase"},
		{Label: taskScaffold, Section: "Implementation", Kind: "phase"},
	}
	for _, path := range stackFileRefs(stack) {
		tasks = append(tasks, specTask{
			Label:   fileTaskLabel(path),
			Section: "Implementation",
			Kind:    "file",
			Ref:     normalizeTaskRef(path),
		})
	}
	tasks = append(tasks, specTask{Label: taskInit, Section: "Initialization", Kind: "phase"})
	for _, task := range stackInitCommandTasks(stack) {
		task.Section = "Initialization"
		task.Kind = "init_command"
		tasks = append(tasks, task)
	}
	tasks = append(tasks, specTask{Label: taskValidate, Section: "Validation", Kind: "phase"})
	for _, task := range stackValidateCommandTasks(stack) {
		task.Section = "Validation"
		task.Kind = "validate_command"
		tasks = append(tasks, task)
	}
	tasks = append(tasks, specTask{Label: taskHandoff, Section: "Handoff", Kind: "phase"})
	return tasks
}

func stackFileRefs(stack string) []string {
	switch stack {
	case "nestjs":
		return []string{
			"package.json",
			"tsconfig.json",
			"nest-cli.json",
			"src/main.ts",
			"src/app.module.ts",
			"src/app.controller.ts",
			"src/app.service.ts",
			"src/health/health.controller.ts",
			"src/health/health.service.ts",
		}
	case "go":
		return []string{"go.mod", "main.go", "internal/service/service.go", "internal/handler/handler.go"}
	case "python":
		return []string{"requirements.txt", "main.py", "app/__init__.py", "app/api.py"}
	case "frontend":
		return []string{"package.json", "tsconfig.json", "src/main.tsx", "src/App.tsx"}
	case "node":
		return []string{"package.json", "tsconfig.json", "src/main.ts", "src/routes/index.ts"}
	default:
		return []string{"依赖清单文件", "项目入口文件", "核心源码文件"}
	}
}

func stackInitCommandTasks(stack string) []specTask {
	switch stack {
	case "nestjs", "frontend", "node":
		return []specTask{{Label: "执行 npm install", Ref: "npm install|pnpm install|yarn install"}}
	case "go":
		return []specTask{{Label: "执行 go mod tidy", Ref: "go mod tidy"}}
	case "python":
		return []specTask{{Label: "安装 Python 依赖", Ref: "pip install|poetry install"}}
	default:
		return []specTask{{Label: "执行依赖初始化命令", Ref: "install|init"}}
	}
}

func stackValidateCommandTasks(stack string) []specTask {
	switch stack {
	case "nestjs", "frontend", "node":
		return []specTask{{Label: "执行 npm run build", Ref: "npm run build|pnpm build|yarn build"}}
	case "go":
		return []specTask{
			{Label: "执行 go test ./...", Ref: "go test"},
			{Label: "执行 go build ./...", Ref: "go build"},
		}
	case "python":
		return []specTask{{Label: "执行 python -m compileall .", Ref: "compileall|py_compile"}}
	default:
		return []specTask{{Label: "执行基础校验命令", Ref: "test|build|compile"}}
	}
}

func fileTaskLabel(path string) string {
	return "创建/更新 " + path
}

func normalizeTaskRef(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\\", "/")
	return strings.ToLower(strings.TrimPrefix(value, "./"))
}

func matchedFileTaskLabels(stack string, touchedFiles []string) []string {
	tasks := buildSpecTasks(stack)
	seen := map[string]struct{}{}
	var labels []string
	for _, path := range touchedFiles {
		normalizedPath := normalizeTouchedFilePath(path)
		for _, task := range tasks {
			if task.Kind != "file" {
				continue
			}
			if normalizedPath == task.Ref || strings.HasSuffix(normalizedPath, "/"+task.Ref) {
				if _, ok := seen[task.Label]; ok {
					continue
				}
				seen[task.Label] = struct{}{}
				labels = append(labels, task.Label)
			}
		}
	}
	return labels
}

func normalizeTouchedFilePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		if rel, err := filepath.Rel(tools.GetWorkDir(), path); err == nil {
			path = rel
		}
	}
	return normalizeTaskRef(path)
}

func matchedCommandTaskLabels(stack string, command string, class string) []string {
	command = strings.ToLower(strings.TrimSpace(command))
	if command == "" {
		return nil
	}
	tasks := buildSpecTasks(stack)
	var labels []string
	for _, task := range tasks {
		if class == "init" && task.Kind != "init_command" {
			continue
		}
		if class == "validate" && task.Kind != "validate_command" {
			continue
		}
		for _, pattern := range strings.Split(task.Ref, "|") {
			pattern = strings.ToLower(strings.TrimSpace(pattern))
			if pattern != "" && strings.Contains(command, pattern) {
				labels = append(labels, task.Label)
				break
			}
		}
	}
	return labels
}

func readTaskStatuses() (map[string]string, error) {
	data, err := os.ReadFile(specArtifacts().TasksPath)
	if err != nil {
		return nil, err
	}
	statuses := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "- [") || len(trimmed) < 6 {
			continue
		}
		marker := trimmed[3:4]
		label := strings.TrimSpace(trimmed[6:])
		if label == "" {
			continue
		}
		statuses[label] = taskStatusFromMarker(marker)
	}
	return statuses, nil
}

func taskStatusFromMarker(marker string) string {
	switch marker {
	case "x":
		return "completed"
	case ">":
		return "in_progress"
	default:
		return "pending"
	}
}

func initialTaskStatuses(tasks []specTask, existing map[string]string, prev taskSpecState, prompt string) map[string]string {
	statuses := map[string]string{}
	for _, task := range tasks {
		if status := strings.TrimSpace(existing[task.Label]); status != "" {
			statuses[task.Label] = status
			continue
		}
		statuses[task.Label] = "pending"
	}

	if len(existing) == 0 || strings.TrimSpace(prev.Request) == "" {
		statuses[taskReview] = "in_progress"
		return normalizeTaskStatuses(tasks, statuses)
	}

	if strings.TrimSpace(prev.Request) == strings.TrimSpace(prompt) {
		return normalizeTaskStatuses(tasks, statuses)
	}

	if prev.Status == specStatusApproved {
		statuses[taskReview] = "in_progress"
		statuses[taskInspect] = "in_progress"
		statuses[taskPlanStructure] = "pending"
		statuses[taskHandoff] = "pending"
		return normalizeTaskStatuses(tasks, statuses)
	}

	for _, task := range tasks {
		statuses[task.Label] = "pending"
	}
	statuses[taskReview] = "in_progress"
	return normalizeTaskStatuses(tasks, statuses)
}

func buildChangeEntry(prev string, current string) string {
	prev = strings.TrimSpace(prev)
	current = strings.TrimSpace(current)
	if current == "" || prev == current {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("### ")
	sb.WriteString(time.Now().Format("2006-01-02 15:04:05"))
	sb.WriteString("\n\n")
	if prev != "" {
		sb.WriteString("- Previous Request: ")
		sb.WriteString(prev)
		sb.WriteString("\n")
	}
	sb.WriteString("- Current Request: ")
	sb.WriteString(current)
	sb.WriteString("\n\n")
	return sb.String()
}

func buildFailureChangeEntry(toolName string, target string, failure string) string {
	toolName = strings.TrimSpace(toolName)
	target = strings.TrimSpace(target)
	failure = strings.TrimSpace(failure)
	if toolName == "" || failure == "" {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("### ")
	sb.WriteString(time.Now().Format("2006-01-02 15:04:05"))
	sb.WriteString("\n\n")
	sb.WriteString("- Failure Tool: ")
	sb.WriteString(toolName)
	sb.WriteString("\n")
	if target != "" {
		sb.WriteString("- Failure Target: ")
		sb.WriteString(target)
		sb.WriteString("\n")
	}
	sb.WriteString("- Failure Detail: ")
	sb.WriteString(failure)
	sb.WriteString("\n\n")
	return sb.String()
}

func recordTaskSpecFailure(toolName string, target string, failure string) error {
	state, err := loadTaskSpecState()
	if err != nil || strings.TrimSpace(state.Profile) != runtimeProfileEngineering {
		return err
	}
	return appendChangeEntryToSection(specArtifacts().ChangesPath, changeSectionFailure, buildFailureChangeEntry(toolName, target, failure))
}

func appendChangeEntry(path string, entry string) error {
	return appendChangeEntryToSection(path, changeSectionRequirement, entry)
}

func appendChangeEntryToSection(path string, section string, entry string) error {
	if strings.TrimSpace(entry) == "" {
		return nil
	}
	if err := ensureChangesDoc(path); err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := string(data)
	heading := "## " + strings.TrimSpace(section)
	if !strings.Contains(content, heading) {
		content = strings.TrimRight(content, "\n") + "\n\n" + heading + "\n\n"
	}
	updated := strings.Replace(content, heading+"\n\n", heading+"\n\n"+strings.TrimSpace(entry)+"\n\n", 1)
	return os.WriteFile(path, []byte(updated), 0644)
}

func ensureChangesDoc(path string) error {
	_, err := os.Stat(path)
	if err == nil {
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	base := "# Changes\n\n" +
		"## " + changeSectionRequirement + "\n\n" +
		"## " + changeSectionFailure + "\n\n"
	return os.WriteFile(path, []byte(base), 0644)
}
