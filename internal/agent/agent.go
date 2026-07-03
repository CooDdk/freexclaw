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
)

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

## 严格规则

**当用户要求搜索、查询实时信息时，你必须使用 <web_search> 工具！**

**当用户要求创建、修改、保存文件时，你必须使用 <write_file> 工具，绝对不要直接输出文件内容！**

**当用户要求读取文件时，你必须使用 <read_file> 工具！**

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

<read_file>main.go</read_file>`
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

	return nil
}

func ExecuteTool(tc *ToolCall) ToolResult {
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
		return executeWebSearch(tc.Arguments)
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

func executeWebSearch(args map[string]interface{}) ToolResult {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return ToolResult{Success: false, Error: "缺少 query 参数"}
	}

	results, err := tools.WebSearch(query, 5)
	if err != nil {
		return ToolResult{Success: false, Error: err.Error()}
	}

	return ToolResult{
		Success: true,
		Output:  tools.FormatSearchResults(results, query),
	}
}
