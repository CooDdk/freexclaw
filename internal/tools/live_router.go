package tools

import (
	"fmt"
	"regexp"
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
	return ResolveLiveQueryWithProgress(query, nil)
}

func ResolveLiveQueryWithProgress(query string, progress func(string)) (*SearchResult, error) {
	match := MatchLiveQuery(query, currentLiveQueryContext)

	switch match.Domain {
	case "weather":
		result, err := GetWeatherSearchResultWithProgress(match, progress)
		if err != nil {
			return nil, err
		}
		currentLiveQueryContext = LiveQueryContext{
			Domain:       "weather",
			Location:     match.Location,
			ForecastDays: match.ForecastDays,
			TimeOfDay:    match.TimeOfDay,
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
			TimeOfDay:    extractTimeOfDay(query, ctx),
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

	invalidParts := []string{"未来", "7天", "七天", "今天", "明天", "大后天", "后天", "这周", "最新", "当前", "目前", "此刻", "现在", "上午", "下午", "中午", "晚上", "凌晨", "傍晚", "早上", "夜间", "呢"}
	for _, part := range invalidParts {
		if strings.Contains(location, part) {
			return ""
		}
	}
	if regexp.MustCompile(`[0-9一二两三四五六七八九十]+\s*天`).MatchString(location) {
		return ""
	}

	return location
}

func looksLikeFollowUp(query string) bool {
	followUpSignals := []string{"呢", "那", "今天的", "未来", "明天", "后天", "这周", "最新", "当前", "目前", "此刻", "上午", "下午", "中午", "晚上", "凌晨", "傍晚", "早上", "夜间"}
	for _, signal := range followUpSignals {
		if strings.Contains(query, signal) {
			return true
		}
	}
	return false
}

func extractForecastDays(query string, ctx LiveQueryContext) int {
	if explicit := extractExplicitForecastDays(query); explicit > 0 {
		return explicit
	}
	if strings.Contains(query, "7天") || strings.Contains(query, "七天") {
		return 7
	}
	// 相对日期需要拉多日预报数据才能覆盖到目标日期
	if strings.Contains(query, "大后天") {
		return 4
	}
	if strings.Contains(query, "后天") {
		return 3
	}
	if strings.Contains(query, "明天") {
		return 2
	}
	if strings.Contains(query, "未来") && ctx.ForecastDays > 0 {
		return ctx.ForecastDays
	}
	if ctx.Domain == "weather" && strings.Contains(query, "未来") {
		return 7
	}
	return 1
}

func extractTimeOfDay(query string, ctx LiveQueryContext) string {
	timeKeywords := []string{"上午", "下午", "中午", "晚上", "凌晨", "傍晚", "早上", "夜间"}
	for _, keyword := range timeKeywords {
		if strings.Contains(query, keyword) {
			return keyword
		}
	}
	return ctx.TimeOfDay
}

var forecastDaysPattern = regexp.MustCompile(`(?i)(?:未来|接下来|后续)?\s*([0-9一二两三四五六七八九十]+)\s*天`)

func extractExplicitForecastDays(query string) int {
	matches := forecastDaysPattern.FindStringSubmatch(query)
	if len(matches) < 2 {
		return 0
	}

	raw := strings.TrimSpace(matches[1])
	if raw == "" {
		return 0
	}
	if raw == "十" {
		return 10
	}

	chineseNums := map[string]int{
		"一": 1,
		"二": 2,
		"两": 2,
		"三": 3,
		"四": 4,
		"五": 5,
		"六": 6,
		"七": 7,
		"八": 8,
		"九": 9,
	}
	if v, ok := chineseNums[raw]; ok {
		return v
	}
	if strings.HasPrefix(raw, "十") && len(raw) == len("十一") {
		if v, ok := chineseNums[strings.TrimPrefix(raw, "十")]; ok {
			return 10 + v
		}
	}
	for i := 1; i <= 15; i++ {
		if raw == fmt.Sprintf("%d", i) {
			return i
		}
	}
	return 0
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
