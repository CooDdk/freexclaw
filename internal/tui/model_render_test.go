package tui

import "testing"

func TestShouldHideToolResult_HidesSuccessfulWebSearchResults(t *testing.T) {
	content := "<|tool_result|>\n搜索关键词: 武汉 实时天气\n\n1. **武汉天气**\n</|tool_result|>"
	if !shouldHideToolResult(content) {
		t.Fatal("expected successful web_search tool result to be hidden")
	}
}

func TestShouldHideToolResult_ShowsToolErrors(t *testing.T) {
	content := "<|tool_result|>\n错误: 实时查询失败: 天气服务返回状态码 502\n</|tool_result|>"
	if shouldHideToolResult(content) {
		t.Fatal("expected tool error result to remain visible")
	}
}
