package agent

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/CooDdk/freexclaw/internal/tools"
)

type ToolCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type ToolResult struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
	Error   string `json:"error,omitempty"`
}

var (
	writeFileRegex  = regexp.MustCompile(`<write_file>([\s\S]*?)</write_file>`)
	appendFileRegex = regexp.MustCompile(`<append_file>([\s\S]*?)</append_file>`)
	readFileRegex   = regexp.MustCompile(`<read_file>(.*?)</read_file>`)
	listDirRegex    = regexp.MustCompile(`<list_dir>(.*?)</list_dir>`)
	webSearchRegex  = regexp.MustCompile(`<web_search>\s*(.*?)\s*</web_search>`)
	runCommandRegex = regexp.MustCompile(`<run_command>([\s\S]*?)</run_command>`)
	editFileRegex   = regexp.MustCompile(`<edit_file>([\s\S]*?)</edit_file>`)
)

// parseEditFileBody 解析 <edit_file> 内部：首行是路径，之后 <<<OLD ... OLD 段与
// <<<NEW ... NEW 段各出现一次。任何格式偏差都返回 error。
func parseEditFileBody(body string) (path, oldStr, newStr string, err error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return "", "", "", fmt.Errorf("edit_file body 为空")
	}

	lines := strings.SplitN(body, "\n", 2)
	path = strings.TrimSpace(lines[0])
	if path == "" || len(lines) < 2 {
		return "", "", "", fmt.Errorf("edit_file 缺少路径或替换块")
	}
	rest := lines[1]

	oldStr, afterOld, ok := extractBlock(rest, "OLD")
	if !ok {
		return "", "", "", fmt.Errorf("edit_file 缺少 <<<OLD ... OLD 段")
	}
	newStr, _, ok = extractBlock(afterOld, "NEW")
	if !ok {
		return "", "", "", fmt.Errorf("edit_file 缺少 <<<NEW ... NEW 段")
	}
	return path, oldStr, newStr, nil
}

// extractBlock 找到 "<<<TAG" 起始行到只包含 "TAG" 的结束行之间的内容。
func extractBlock(s, tag string) (content, remaining string, ok bool) {
	openMarker := "<<<" + tag
	closeMarker := tag
	openIdx := strings.Index(s, openMarker)
	if openIdx < 0 {
		return "", "", false
	}
	// 定位 openMarker 所在行的下一行开始
	afterOpen := s[openIdx+len(openMarker):]
	if nl := strings.IndexByte(afterOpen, '\n'); nl >= 0 {
		afterOpen = afterOpen[nl+1:]
	} else {
		return "", "", false
	}
	// 找到"独占一行"的 closeMarker
	closeIdx := -1
	scan := afterOpen
	base := 0
	for {
		idx := strings.Index(scan, closeMarker)
		if idx < 0 {
			return "", "", false
		}
		absIdx := base + idx
		// 判断是不是整行
		startsLine := absIdx == 0 || afterOpen[absIdx-1] == '\n'
		endPos := absIdx + len(closeMarker)
		endsLine := endPos == len(afterOpen) || afterOpen[endPos] == '\n' || afterOpen[endPos] == '\r'
		if startsLine && endsLine {
			closeIdx = absIdx
			break
		}
		scan = scan[idx+len(closeMarker):]
		base = absIdx + len(closeMarker)
	}
	content = strings.TrimRight(afterOpen[:closeIdx], "\r\n")
	remaining = afterOpen[closeIdx+len(closeMarker):]
	return content, remaining, true
}

func SystemPrompt() string {
	return `你是一个智能助手。当用户的问题需要实时信息（如天气、新闻、事件等），请使用 <web_search> 工具搜索网络获取最新信息。

## 工具列表

1. 搜索网络：<web_search>搜索关键词</web_search>

2. 创建/覆盖文件：<write_file>文件名
文件内容</write_file>

3. 读取文件：<read_file>文件名</read_file>

4. 追加内容：<append_file>文件名
追加内容</append_file>

5. 列出目录：<list_dir>目录路径</list_dir>

6. 执行命令：<run_command>
可选第一行写 cwd: 相对目录
后续写要执行的单次命令
</run_command>

7. 精确编辑文件：<edit_file>相对路径
<<<OLD
待替换的原文（必须在文件中唯一命中）
OLD
<<<NEW
替换后的内容
NEW
</edit_file>

## 严格规则

**当用户要求搜索、查询实时信息时，你必须使用 <web_search> 工具！**

**当用户要求创建、修改、保存文件时，你必须使用 <write_file> 工具，绝对不要直接输出文件内容！**

**当用户要求读取文件时，你必须使用 <read_file> 工具！**

**当任务是“创建一个可运行的项目 / 服务 / 脚本”时，你不能只写单个源码文件就结束。你应该优先检查当前目录结构，补齐必要文件，并在需要时用 <run_command> 做初始化和验证。**

**当你已经写完代码后，应继续使用 <run_command> 执行短时命令做验证，而不是停留在“请用户自行运行”。**

**禁止使用 <run_command> 启动长期不退出的进程，例如 go run、npm run dev、vite、python app.py、uvicorn、flask run 等。你应该优先执行初始化、依赖安装、测试、静态检查、构建这类会结束的命令。**

**如果命令失败，你应该根据错误继续修复文件或依赖，然后再次验证，直到得到一个尽量可运行、可通过基础检查的结果。**

**工具调用必须单独占一行，前后要有空行。**

## 正确示例

用户：今天北京的天气怎么样
你：

<web_search>北京 今天 天气</web_search>

用户：帮我生成一个 test.md 文件
你：

<write_file>test.md
# 测试文件

这是生成的内容。
</write_file>

用户：读取 main.go 的内容
你：

<read_file>main.go</read_file>

用户：使用 gin 写一个简单 API 服务，并尽量帮我初始化和检查
你：

<list_dir>.</list_dir>

（读取必要文件后继续）

<write_file>main.go
package main
...
</write_file>

<run_command>
go mod init gin-demo
</run_command>

<run_command>
go mod tidy
</run_command>

<run_command>
go build ./...
</run_command>`
}

func ParseToolCall(content string) *ToolCall {
	if matches := writeFileRegex.FindStringSubmatch(content); len(matches) >= 2 {
		body := strings.TrimSpace(matches[1])
		parts := strings.SplitN(body, "\n", 2)
		path := strings.TrimSpace(parts[0])
		fileContent := ""
		if len(parts) > 1 {
			fileContent = parts[1]
		}
		return &ToolCall{
			Name: "write_file",
			Arguments: map[string]interface{}{
				"path":    path,
				"content": fileContent,
			},
		}
	}

	if matches := appendFileRegex.FindStringSubmatch(content); len(matches) >= 2 {
		body := strings.TrimSpace(matches[1])
		parts := strings.SplitN(body, "\n", 2)
		path := strings.TrimSpace(parts[0])
		fileContent := ""
		if len(parts) > 1 {
			fileContent = parts[1]
		}
		return &ToolCall{
			Name: "append_file",
			Arguments: map[string]interface{}{
				"path":    path,
				"content": fileContent,
			},
		}
	}

	if matches := readFileRegex.FindStringSubmatch(content); len(matches) >= 2 {
		return &ToolCall{
			Name: "read_file",
			Arguments: map[string]interface{}{
				"path": strings.TrimSpace(matches[1]),
			},
		}
	}

	if matches := listDirRegex.FindStringSubmatch(content); len(matches) >= 2 {
		path := strings.TrimSpace(matches[1])
		if path == "" {
			path = "."
		}
		return &ToolCall{
			Name: "list_dir",
			Arguments: map[string]interface{}{
				"path": path,
			},
		}
	}

	if matches := webSearchRegex.FindStringSubmatch(content); len(matches) >= 2 {
		return &ToolCall{
			Name: "web_search",
			Arguments: map[string]interface{}{
				"query": strings.TrimSpace(matches[1]),
			},
		}
	}

	if matches := editFileRegex.FindStringSubmatch(content); len(matches) >= 2 {
		path, oldStr, newStr, err := parseEditFileBody(matches[1])
		if err != nil {
			return &ToolCall{
				Name: "edit_file",
				Arguments: map[string]interface{}{
					"parse_error": err.Error(),
				},
			}
		}
		return &ToolCall{
			Name: "edit_file",
			Arguments: map[string]interface{}{
				"path": path,
				"old":  oldStr,
				"new":  newStr,
			},
		}
	}

	if matches := runCommandRegex.FindStringSubmatch(content); len(matches) >= 2 {
		body := strings.TrimSpace(matches[1])
		if body == "" {
			return nil
		}
		cwd := ""
		command := body
		lines := strings.Split(body, "\n")
		if len(lines) > 1 {
			firstLine := strings.TrimSpace(lines[0])
			switch {
			case strings.HasPrefix(strings.ToLower(firstLine), "cwd:"):
				cwd = strings.TrimSpace(firstLine[4:])
				command = strings.TrimSpace(strings.Join(lines[1:], "\n"))
			case strings.HasPrefix(firstLine, "目录:"):
				cwd = strings.TrimSpace(strings.TrimPrefix(firstLine, "目录:"))
				command = strings.TrimSpace(strings.Join(lines[1:], "\n"))
			}
		}
		return &ToolCall{
			Name: "run_command",
			Arguments: map[string]interface{}{
				"cwd":     cwd,
				"command": command,
			},
		}
	}

	return nil
}

func ExecuteTool(tc *ToolCall) ToolResult {
	return ExecuteToolWithProgress(tc, nil)
}

func ExecuteToolWithProgress(tc *ToolCall, progress func(string)) ToolResult {
	switch tc.Name {
	case "read_file":
		return executeReadFile(tc.Arguments)
	case "write_file":
		return executeWriteFile(tc.Arguments, false)
	case "append_file":
		return executeWriteFile(tc.Arguments, true)
	case "list_dir":
		return executeListDir(tc.Arguments)
	case "web_search":
		return executeWebSearch(tc.Arguments, progress)
	case "run_command":
		return executeRunCommand(tc.Arguments)
	case "edit_file":
		return executeEditFile(tc.Arguments)
	default:
		return ToolResult{
			Success: false,
			Error:   fmt.Sprintf("未知工具: %s", tc.Name),
		}
	}
}

func executeReadFile(args map[string]interface{}) ToolResult {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return ToolResult{Success: false, Error: "缺少 path 参数"}
	}

	fc, err := tools.ReadFile(path)
	if err != nil {
		return ToolResult{Success: false, Error: err.Error()}
	}
	return ToolResult{
		Success: true,
		Output:  fmt.Sprintf("文件: %s\n\n%s", fc.Path, fc.Content),
	}
}

func executeWriteFile(args map[string]interface{}, appendMode bool) ToolResult {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return ToolResult{Success: false, Error: "缺少 path 参数"}
	}

	content, _ := args["content"].(string)

	if err := tools.WriteFile(path, content, appendMode); err != nil {
		return ToolResult{Success: false, Error: err.Error()}
	}

	mode := "写入"
	if appendMode {
		mode = "追加"
	}
	return ToolResult{
		Success: true,
		Output:  fmt.Sprintf("✓ %s成功: %s", mode, path),
	}
}

func executeListDir(args map[string]interface{}) ToolResult {
	path := "."
	if p, ok := args["path"].(string); ok && p != "" {
		path = p
	}

	files, err := tools.ListDir(path)
	if err != nil {
		return ToolResult{Success: false, Error: err.Error()}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("目录: %s\n\n", path))
	for _, f := range files {
		sb.WriteString(f + "\n")
	}
	return ToolResult{
		Success: true,
		Output:  sb.String(),
	}
}

func executeRunCommand(args map[string]interface{}) ToolResult {
	command, ok := args["command"].(string)
	if !ok || strings.TrimSpace(command) == "" {
		return ToolResult{Success: false, Error: "缺少 command 参数"}
	}

	cwd, _ := args["cwd"].(string)
	result, err := tools.RunCommand(command, cwd)
	output := tools.FormatCommandResult(result)
	if err != nil {
		return ToolResult{
			Success: false,
			Output:  output,
			Error:   err.Error(),
		}
	}

	return ToolResult{
		Success: true,
		Output:  output,
	}
}

func executeEditFile(args map[string]interface{}) ToolResult {
	if msg, ok := args["parse_error"].(string); ok {
		return ToolResult{Success: false, Error: msg}
	}
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return ToolResult{Success: false, Error: "缺少 path 参数"}
	}
	oldStr, _ := args["old"].(string)
	newStr, _ := args["new"].(string)

	if err := tools.EditFile(path, oldStr, newStr); err != nil {
		return ToolResult{Success: false, Error: err.Error()}
	}
	return ToolResult{
		Success: true,
		Output:  fmt.Sprintf("✓ 编辑成功: %s", path),
	}
}

func FormatToolResult(tc *ToolCall, result ToolResult) string {
	status := "✅"
	if !result.Success {
		status = "❌"
	}
	return fmt.Sprintf("%s 工具调用: %s\n%s", status, tc.Name, result.Output)
}

func BuildToolResultMessage(tc *ToolCall, result ToolResult) string {
	content := result.Output
	if strings.TrimSpace(content) == "" && result.Error != "" {
		content = "错误: " + result.Error
	}
	return fmt.Sprintf("<|tool_result|>\n%s\n</|tool_result|>", content)
}

var (
	codeBlockRegex = regexp.MustCompile("(?:(?:save as|保存为|保存|生成|创建)\\s*[\"']?([^\"' \\n]+)[\"']?[^\\n]*(?:\\n|$))?```(?:\\w+)?\\n([\\s\\S]*?)```")
	fileNameRegex  = regexp.MustCompile("[\"']([^\"' ]+\\.(?:md|txt|go|py|js|ts|html|css|json|yaml|yml))[\"']")
)

// AutoExtractFileContent 自动从 AI 回复中提取文件内容并写入
// 返回: 是否自动执行了写入, 文件路径, 错误信息
func AutoExtractFileContent(content string) (bool, string, error) {
	matches := codeBlockRegex.FindStringSubmatch(content)
	if len(matches) < 3 {
		return false, "", nil
	}

	explicitFileName := matches[1]
	codeContent := matches[2]

	if codeContent == "" {
		return false, "", nil
	}

	var fileName string
	if explicitFileName != "" {
		fileName = explicitFileName
	} else {
		altMatches := fileNameRegex.FindStringSubmatch(content)
		if len(altMatches) >= 2 {
			fileName = altMatches[1]
		}
	}

	if fileName == "" {
		return false, "", nil
	}

	if err := tools.WriteFile(fileName, codeContent, false); err != nil {
		return false, fileName, err
	}

	return true, fileName, nil
}

func executeWebSearch(args map[string]interface{}, progress func(string)) ToolResult {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return ToolResult{Success: false, Error: "缺少 query 参数"}
	}

	results, err := tools.WebSearchWithProgress(query, 5, progress)
	if err != nil {
		return ToolResult{Success: false, Error: err.Error()}
	}

	return ToolResult{
		Success: true,
		Output:  tools.FormatSearchResults(results, query),
	}
}
