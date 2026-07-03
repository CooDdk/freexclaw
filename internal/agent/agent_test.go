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
	) {
		t.Fatalf("system prompt should contain readable weather/tool instructions, got: %q", prompt)
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
