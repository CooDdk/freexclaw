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
		"go test ./...",
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

func containsAll(s string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(s, part) {
			return false
		}
	}
	return true
}
