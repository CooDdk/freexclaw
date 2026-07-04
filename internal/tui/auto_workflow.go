package tui

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/CooDdk/freexclaw/internal/agent"
	"github.com/CooDdk/freexclaw/internal/tools"
)

type projectSnapshot struct {
	root            string
	hasGoFiles      bool
	hasGoMod        bool
	hasPackageJSON  bool
	hasJSFiles      bool
	hasTSFiles      bool
	hasPythonFiles  bool
	hasRequirements bool
	hasPyProject    bool
	packageScripts  map[string]string
}

type commandSpec struct {
	cwd     string
	command string
}

func buildForcedPostProcessToolCalls(userPrompt string, touchedFiles []string) []*agent.ToolCall {
	if !shouldForceProjectWorkflow(userPrompt, touchedFiles) {
		return nil
	}

	workDir := tools.GetWorkDir()
	projectDir := detectProjectDir(workDir, touchedFiles)
	snapshot := inspectProjectSnapshot(projectDir)

	specs := buildCommandSpecs(snapshot)
	if len(specs) == 0 {
		return nil
	}

	toolCalls := make([]*agent.ToolCall, 0, len(specs))
	for _, spec := range specs {
		toolCalls = append(toolCalls, &agent.ToolCall{
			Name: "run_command",
			Arguments: map[string]interface{}{
				"cwd":     spec.cwd,
				"command": spec.command,
			},
		})
	}

	return toolCalls
}

func shouldForceProjectWorkflow(userPrompt string, touchedFiles []string) bool {
	if strings.TrimSpace(userPrompt) == "" || len(touchedFiles) == 0 {
		return false
	}

	lower := strings.ToLower(userPrompt)
	createHints := []string{
		"创建", "生成", "写一个", "做一个", "实现", "搭建", "使用",
		"create", "generate", "build", "make", "implement", "scaffold",
	}
	projectHints := []string{
		"项目", "服务", "接口", "api", "程序", "脚本", "应用", "网站", "框架", "server", "service", "project", "app",
	}

	hasCreateHint := containsAny(lower, createHints...)
	hasProjectHint := containsAny(lower, projectHints...)
	hasCodeFile := hasSubstantiveProjectFiles(touchedFiles)

	return hasCreateHint && hasProjectHint && hasCodeFile
}

func hasSubstantiveProjectFiles(paths []string) bool {
	for _, path := range paths {
		if isSubstantiveProjectFile(path) {
			return true
		}
	}
	return false
}

func isSubstantiveProjectFile(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}

	base := strings.ToLower(filepath.Base(path))
	ext := strings.ToLower(filepath.Ext(base))

	manifestFiles := map[string]struct{}{
		"package.json":        {},
		"package-lock.json":   {},
		"tsconfig.json":       {},
		"tsconfig.build.json": {},
		"nest-cli.json":       {},
		".gitignore":          {},
		"readme.md":           {},
		"requirements.txt":    {},
		"pyproject.toml":      {},
		"go.mod":              {},
		"go.sum":              {},
	}
	if _, ok := manifestFiles[base]; ok {
		return false
	}

	switch ext {
	case ".go", ".py":
		return true
	case ".js", ".jsx", ".ts", ".tsx":
		return base != "vite.config.ts" && base != "vitest.config.ts" && base != "eslint.config.js"
	case ".html", ".css", ".scss", ".vue", ".svelte":
		return true
	}

	return false
}

func detectProjectDir(workDir string, touchedFiles []string) string {
	if len(touchedFiles) == 0 {
		return workDir
	}

	common := filepath.Dir(normalizeTouchedPath(workDir, touchedFiles[0]))
	for _, raw := range touchedFiles[1:] {
		current := filepath.Dir(normalizeTouchedPath(workDir, raw))
		for !sameOrParent(common, current) {
			parent := filepath.Dir(common)
			if parent == common {
				return workDir
			}
			common = parent
		}
	}

	rel, err := filepath.Rel(workDir, common)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return workDir
	}
	return common
}

func normalizeTouchedPath(workDir string, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(workDir, path))
}

func sameOrParent(parent string, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")
}

func inspectProjectSnapshot(projectDir string) projectSnapshot {
	snapshot := projectSnapshot{
		root:           projectDir,
		packageScripts: map[string]string{},
	}

	_ = filepath.WalkDir(projectDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "dist", "build", "vendor", ".venv", "venv":
				if path != projectDir {
					return filepath.SkipDir
				}
			}
			return nil
		}

		base := strings.ToLower(filepath.Base(path))
		switch base {
		case "go.mod":
			snapshot.hasGoMod = true
		case "package.json":
			snapshot.hasPackageJSON = true
			loadPackageScripts(path, snapshot.packageScripts)
		case "requirements.txt":
			snapshot.hasRequirements = true
		case "pyproject.toml":
			snapshot.hasPyProject = true
		}

		switch strings.ToLower(filepath.Ext(path)) {
		case ".go":
			snapshot.hasGoFiles = true
		case ".js", ".mjs", ".cjs":
			snapshot.hasJSFiles = true
		case ".ts", ".tsx":
			snapshot.hasTSFiles = true
		case ".py":
			snapshot.hasPythonFiles = true
		}
		return nil
	})

	return snapshot
}

func loadPackageScripts(path string, target map[string]string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return
	}
	for k, v := range pkg.Scripts {
		target[k] = v
	}
}

func buildCommandSpecs(snapshot projectSnapshot) []commandSpec {
	relCwd := "."
	if rel, err := filepath.Rel(tools.GetWorkDir(), snapshot.root); err == nil && rel != "." {
		relCwd = rel
	}

	switch {
	case snapshot.hasGoMod || snapshot.hasGoFiles:
		return buildGoCommandSpecs(snapshot, relCwd)
	case snapshot.hasPackageJSON || snapshot.hasJSFiles || snapshot.hasTSFiles:
		return buildNodeCommandSpecs(snapshot, relCwd)
	case snapshot.hasRequirements || snapshot.hasPyProject || snapshot.hasPythonFiles:
		return buildPythonCommandSpecs(snapshot, relCwd)
	default:
		return nil
	}
}

func buildGoCommandSpecs(snapshot projectSnapshot, cwd string) []commandSpec {
	var specs []commandSpec
	if !snapshot.hasGoMod {
		specs = append(specs, commandSpec{
			cwd:     cwd,
			command: "go mod init " + guessGoModuleName(snapshot.root),
		})
	}
	specs = append(specs,
		commandSpec{cwd: cwd, command: "go mod tidy"},
		commandSpec{cwd: cwd, command: "go test ./..."},
		commandSpec{cwd: cwd, command: "go vet ./..."},
		commandSpec{cwd: cwd, command: "go build ./..."},
	)
	return specs
}

func buildNodeCommandSpecs(snapshot projectSnapshot, cwd string) []commandSpec {
	var specs []commandSpec
	if !snapshot.hasPackageJSON {
		specs = append(specs, commandSpec{cwd: cwd, command: "npm init -y"})
	}
	specs = append(specs, commandSpec{cwd: cwd, command: "npm install"})
	if script := snapshot.packageScripts["test"]; script != "" && !strings.Contains(script, "no test specified") {
		specs = append(specs, commandSpec{cwd: cwd, command: "npm test"})
	}
	if script := snapshot.packageScripts["build"]; script != "" {
		specs = append(specs, commandSpec{cwd: cwd, command: "npm run build"})
	}
	return specs
}

func buildPythonCommandSpecs(snapshot projectSnapshot, cwd string) []commandSpec {
	var specs []commandSpec
	if snapshot.hasRequirements {
		specs = append(specs, commandSpec{cwd: cwd, command: "python -m pip install -r requirements.txt"})
	}
	if snapshot.hasPyProject {
		specs = append(specs, commandSpec{cwd: cwd, command: "python -m pip install -e ."})
	}
	specs = append(specs, commandSpec{cwd: cwd, command: "python -m compileall ."})
	return specs
}

func guessGoModuleName(projectDir string) string {
	name := strings.ToLower(filepath.Base(projectDir))
	if name == "." || name == "" {
		return "app"
	}
	re := regexp.MustCompile(`[^a-z0-9._-]+`)
	name = re.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-.")
	if name == "" {
		return "app"
	}
	return name
}

func containsAny(s string, parts ...string) bool {
	for _, part := range parts {
		if strings.Contains(s, part) {
			return true
		}
	}
	return false
}
