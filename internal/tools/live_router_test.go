package tools

import (
	"strings"
	"testing"
)

func TestRouteLiveQuery_WeatherFollowUpUsesContext(t *testing.T) {
	ctx := LiveQueryContext{
		Domain:   "weather",
		Location: "武汉",
	}

	match := MatchLiveQuery("未来7天的天气呢", ctx)
	if match.Domain != "weather" {
		t.Fatalf("expected weather domain, got %q", match.Domain)
	}
	if match.Location != "武汉" {
		t.Fatalf("expected inherited location 武汉, got %q", match.Location)
	}
	if match.ForecastDays != 7 {
		t.Fatalf("expected 7-day forecast, got %d", match.ForecastDays)
	}
}

func TestRouteLiveQuery_FinanceAndNewsAreRecognized(t *testing.T) {
	finance := MatchLiveQuery("美元兑人民币", LiveQueryContext{})
	if finance.Domain != "finance" || finance.MarketType != "fx" {
		t.Fatalf("expected fx finance match, got %#v", finance)
	}

	news := MatchLiveQuery("今天有什么大新闻", LiveQueryContext{})
	if news.Domain != "news" {
		t.Fatalf("expected news match, got %#v", news)
	}
}

func TestFormatFinanceSearchResult(t *testing.T) {
	result := FormatFinanceSearchResult(LiveResult{
		Domain:     "finance",
		Title:      "美元兑人民币",
		SourceName: "Finance API",
		Items: []LiveItem{
			{Label: "当前汇率", Value: "7.1200"},
			{Label: "更新时间", Value: "2026-07-03 16:00"},
		},
	})

	if !strings.Contains(result.Description, "7.1200") {
		t.Fatalf("expected exchange rate in description, got %q", result.Description)
	}
}

func TestFormatNewsSearchResult(t *testing.T) {
	result := FormatNewsSearchResult(LiveResult{
		Domain:     "news",
		Title:      "今日要闻",
		SourceName: "News API",
		Items: []LiveItem{
			{Label: "头条", Value: "示例新闻", Detail: "2026-07-03 16:00"},
		},
	})

	if !strings.Contains(result.Description, "示例新闻") {
		t.Fatalf("expected headline in description, got %q", result.Description)
	}
}

func TestResolveLiveQuery_UnknownQueryFallsBack(t *testing.T) {
	match := MatchLiveQuery("帮我解释 main.go", LiveQueryContext{})
	if match.Domain != "generic_search" {
		t.Fatalf("expected generic search fallback, got %#v", match)
	}
}

func TestMatchLiveQuery_TreatsProviderTaggedWeatherQueryAsWeather(t *testing.T) {
	match := MatchLiveQuery("武汉 实时天气 2026-07-03 Open-Meteo", LiveQueryContext{})
	if match.Domain != "weather" {
		t.Fatalf("expected weather domain, got %#v", match)
	}
	if match.Location != "武汉" {
		t.Fatalf("expected location 武汉, got %#v", match)
	}
}
