package tui

import (
	"testing"

	"github.com/CooDdk/freexclaw/internal/conversation"
)

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

func TestShouldHideToolResult_HidesDuplicateSkipControlMessage(t *testing.T) {
	content := "<|tool_result|>\n已跳过重复的 web_search 调用，请直接基于已有查询结果继续回答，不要再次调用相同查询。\n</|tool_result|>"
	if !shouldHideToolResult(content) {
		t.Fatal("expected duplicate-skip control message to be hidden")
	}
}

func TestShouldHideToolResultInContext_HidesTransientToolErrorWithLaterAnswer(t *testing.T) {
	messages := []conversation.Message{
		{Role: conversation.RoleUser, Content: "未来15天呢"},
		{Role: conversation.RoleUser, Content: "<|tool_result|>\n错误: 实时查询失败: 未找到地点: 呢\n</|tool_result|>"},
		{Role: conversation.RoleAssistant, Content: "南京未来15天（7月4日-7月18日）天气预报如下。"},
	}

	if !shouldHideToolResultInContext(messages, 1) {
		t.Fatal("expected transient tool error to be hidden when later assistant answer exists")
	}
}

func TestShouldHideToolResultInContext_ShowsFinalToolErrorWithoutAnswer(t *testing.T) {
	messages := []conversation.Message{
		{Role: conversation.RoleUser, Content: "查一下天气"},
		{Role: conversation.RoleUser, Content: "<|tool_result|>\n错误: 实时查询失败: 未找到地点: 呢\n</|tool_result|>"},
		{Role: conversation.RoleAssistant, Content: ""},
	}

	if shouldHideToolResultInContext(messages, 1) {
		t.Fatal("expected final tool error to remain visible without later assistant answer")
	}
}
