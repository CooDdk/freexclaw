package agent

import (
	"strings"
	"testing"
)

func TestSystemPromptIncludesReadableWeatherInstructions(t *testing.T) {
	prompt := SystemPrompt()

	if prompt == "" {
		t.Fatal("expected non-empty system prompt")
	}

	if !containsAll(prompt,
		"当用户的问题需要实时信息",
		"<web_search>北京 今天 天气</web_search>",
		"你必须使用 <web_search> 工具",
		"<run_command>",
		"go build ./...",
	) {
		t.Fatalf("system prompt should contain readable weather/tool instructions, got: %q", prompt)
	}
}

func TestParseToolCall_RunCommand(t *testing.T) {
	tc := ParseToolCall("<run_command>\ncwd: api\nnpm test\n</run_command>")
	if tc == nil {
		t.Fatal("expected run_command tool call")
	}
	if tc.Name != "run_command" {
		t.Fatalf("expected run_command, got %q", tc.Name)
	}
	if got := tc.Arguments["cwd"]; got != "api" {
		t.Fatalf("expected cwd api, got %#v", got)
	}
	if got := tc.Arguments["command"]; got != "npm test" {
		t.Fatalf("expected command npm test, got %#v", got)
	}
}

func TestBuildToolResultMessage_UsesErrorWhenOutputEmpty(t *testing.T) {
	tc := &ToolCall{Name: "web_search", Arguments: map[string]interface{}{"query": "未来7天的天气呢"}}
	result := ToolResult{Success: false, Error: "实时查询失败: 天气服务返回状态码 502"}

	message := BuildToolResultMessage(tc, result)
	if !strings.Contains(message, "天气服务返回状态码 502") {
		t.Fatalf("expected error message to be included, got %q", message)
	}
}

func TestParseToolCall_EditFile(t *testing.T) {
	body := "<edit_file>main.go\n<<<OLD\nfmt.Println(\"a\")\nOLD\n<<<NEW\nfmt.Println(\"b\")\nNEW\n</edit_file>"
	tc := ParseToolCall(body)
	if tc == nil {
		t.Fatal("expected edit_file tool call")
	}
	if tc.Name != "edit_file" {
		t.Fatalf("expected edit_file, got %q", tc.Name)
	}
	if got := tc.Arguments["path"]; got != "main.go" {
		t.Fatalf("expected path main.go, got %#v", got)
	}
	if got := tc.Arguments["old"]; got != "fmt.Println(\"a\")" {
		t.Fatalf("unexpected old: %#v", got)
	}
	if got := tc.Arguments["new"]; got != "fmt.Println(\"b\")" {
		t.Fatalf("unexpected new: %#v", got)
	}
}

func TestParseToolCall_EditFile_MalformedReportsParseError(t *testing.T) {
	tc := ParseToolCall("<edit_file>main.go\n<<<OLD\nno close\n</edit_file>")
	if tc == nil || tc.Name != "edit_file" {
		t.Fatalf("expected edit_file call with parse_error, got %#v", tc)
	}
	if _, ok := tc.Arguments["parse_error"].(string); !ok {
		t.Fatalf("expected parse_error argument, got %#v", tc.Arguments)
	}
}

func TestSystemPrompt_MentionsEditFile(t *testing.T) {
	if !strings.Contains(SystemPrompt(), "<edit_file>") {
		t.Fatalf("system prompt should advertise <edit_file>")
	}
}

func TestParseToolCall_Grep(t *testing.T) {
	body := "<grep>func Hello\npath: internal\nglob: *.go\ncase: i\n</grep>"
	tc := ParseToolCall(body)
	if tc == nil || tc.Name != "grep" {
		t.Fatalf("expected grep tool call, got %#v", tc)
	}
	if got := tc.Arguments["pattern"]; got != "func Hello" {
		t.Fatalf("pattern: %#v", got)
	}
	if got := tc.Arguments["path"]; got != "internal" {
		t.Fatalf("path: %#v", got)
	}
	if got := tc.Arguments["glob"]; got != "*.go" {
		t.Fatalf("glob: %#v", got)
	}
	if got := tc.Arguments["ignore_case"]; got != true {
		t.Fatalf("ignore_case: %#v", got)
	}
}

func TestSystemPrompt_MentionsGrep(t *testing.T) {
	if !strings.Contains(SystemPrompt(), "<grep>") {
		t.Fatalf("system prompt should advertise <grep>")
	}
}

func TestParseToolCall_Glob(t *testing.T) {
	tc := ParseToolCall("<glob>**/*.go\npath: internal\n</glob>")
	if tc == nil || tc.Name != "glob" {
		t.Fatalf("expected glob tool call, got %#v", tc)
	}
	if got := tc.Arguments["pattern"]; got != "**/*.go" {
		t.Fatalf("pattern: %#v", got)
	}
	if got := tc.Arguments["path"]; got != "internal" {
		t.Fatalf("path: %#v", got)
	}
}

func TestSystemPrompt_MentionsGlob(t *testing.T) {
	if !strings.Contains(SystemPrompt(), "<glob>") {
		t.Fatalf("system prompt should advertise <glob>")
	}
}

func containsAll(s string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(s, part) {
			return false
		}
	}
	return true
}
