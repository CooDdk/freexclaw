package tui

import (
	"testing"

	"github.com/CooDdk/freexclaw/internal/tools"
)

func TestBuildPreflightToolCall_UsesRawFollowUpQuery(t *testing.T) {
	tools.SetCurrentLiveQueryContext(tools.LiveQueryContext{
		Domain:   "weather",
		Location: "武汉",
	})
	defer tools.SetCurrentLiveQueryContext(tools.LiveQueryContext{})

	tc := buildPreflightToolCall("未来7天的天气呢")
	if tc == nil {
		t.Fatal("expected preflight tool call")
	}
	if tc.Name != "web_search" {
		t.Fatalf("expected web_search tool, got %q", tc.Name)
	}

	query, _ := tc.Arguments["query"].(string)
	if query != "武汉 7天 天气预报" {
		t.Fatalf("expected canonical weather query, got %q", query)
	}
}
