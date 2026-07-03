package tools

import (
	"fmt"
	"strings"
)

var currentLiveQueryContext LiveQueryContext

func GetCurrentLiveQueryContext() LiveQueryContext {
	return currentLiveQueryContext
}

func SetCurrentLiveQueryContext(ctx LiveQueryContext) {
	currentLiveQueryContext = ctx
}

func ResolveLiveQuery(query string) (*SearchResult, error) {
	match := MatchLiveQuery(query, currentLiveQueryContext)

	switch match.Domain {
	case "weather":
		result, err := GetWeatherSearchResult(match)
		if err != nil {
			return nil, err
		}
		currentLiveQueryContext = LiveQueryContext{
			Domain:       "weather",
			Location:     match.Location,
			ForecastDays: match.ForecastDays,
		}
		return result, nil
	case "news":
		result, err := ResolveNewsQuery(match)
		if err != nil {
			return nil, err
		}
		currentLiveQueryContext = LiveQueryContext{
			Domain: "news",
			Topic:  match.Topic,
		}
		return result, nil
	case "finance":
		result, err := ResolveFinanceQuery(match)
		if err != nil {
			return nil, err
		}
		currentLiveQueryContext = LiveQueryContext{
			Domain:     "finance",
			Symbol:     match.Symbol,
			MarketType: match.MarketType,
		}
		return result, nil
	default:
		return nil, fmt.Errorf("no live provider matched")
	}
}

func MatchLiveQuery(query string, ctx LiveQueryContext) MatchResult {
	query = strings.TrimSpace(query)

	if isWeatherIntent(query) || (ctx.Domain == "weather" && looksLikeFollowUp(query)) {
		location := normalizeWeatherLocationCandidate(ExtractWeatherLocation(query))
		if location == "" {
			location = ctx.Location
		}

		return MatchResult{
			Domain:       "weather",
			Location:     location,
			ForecastDays: extractForecastDays(query, ctx),
			Query:        query,
		}
	}

	if isFinanceIntent(query) {
		return matchFinanceQuery(query)
	}

	if isNewsIntent(query) {
		return MatchResult{
			Domain: "news",
			Topic:  query,
			Query:  query,
		}
	}

	return MatchResult{
		Domain: "generic_search",
		Query:  query,
	}
}

func isWeatherIntent(query string) bool {
	return IsWeatherQuery(query)
}

func normalizeWeatherLocationCandidate(location string) string {
	location = strings.TrimSpace(location)
	if location == "" {
		return ""
	}

	invalidParts := []string{"未来", "7天", "七天", "今天", "明天", "后天", "这周", "呢"}
	for _, part := range invalidParts {
		if strings.Contains(location, part) {
			return ""
		}
	}

	return location
}

func looksLikeFollowUp(query string) bool {
	followUpSignals := []string{"呢", "那", "今天的", "未来", "明天", "后天", "这周"}
	for _, signal := range followUpSignals {
		if strings.Contains(query, signal) {
			return true
		}
	}
	return false
}

func extractForecastDays(query string, ctx LiveQueryContext) int {
	if strings.Contains(query, "7天") || strings.Contains(query, "七天") {
		return 7
	}
	if strings.Contains(query, "未来") && ctx.ForecastDays > 0 {
		return ctx.ForecastDays
	}
	if ctx.Domain == "weather" && strings.Contains(query, "未来") {
		return 7
	}
	return 1
}

func isNewsIntent(query string) bool {
	keywords := []string{"新闻", "头条", "热点", "最新消息"}
	for _, keyword := range keywords {
		if strings.Contains(query, keyword) {
			return true
		}
	}
	return strings.Contains(query, "今天有什么")
}

func isFinanceIntent(query string) bool {
	keywords := []string{"汇率", "兑", "股价", "股票", "价格"}
	for _, keyword := range keywords {
		if strings.Contains(query, keyword) {
			return true
		}
	}
	return false
}

func matchFinanceQuery(query string) MatchResult {
	marketType := "equity"
	if strings.Contains(query, "汇率") || strings.Contains(query, "兑") {
		marketType = "fx"
	}

	return MatchResult{
		Domain:     "finance",
		MarketType: marketType,
		Symbol:     query,
		Query:      query,
	}
}
