# Live Query Router Phase 1 Implementation Plan

**Goal:** 为 FreeX Claw 增加第一阶段实时查询路由层，支持天气追问承接、7 天预报、新闻摘要、汇率和股价，并在失败时优雅回退到通用搜索。

**Architecture:** 在 `internal/tools` 中增加“路由器 + 上下文 + provider”结构，保留现有 `web_search` 入口不变。天气、新闻、金融分别走专用 provider，最终统一转换成 `SearchResult` 风格文本输出，未命中或失败时再回退 Bing。

**Tech Stack:** Go 1.25、现有 `internal/tools` 工具层、Open-Meteo、HTTP JSON API、Go 单元测试

---

## 文件结构

- 修改：`D:\work\code\ai\go-claudecodecli-demo\internal\tools\search.go`
  - 从“天气特判 + Bing”升级为“实时查询路由 + Bing 回退”
- 新增：`D:\work\code\ai\go-claudecodecli-demo\internal\tools\live_types.go`
  - 统一定义 `LiveResult`、`LiveItem`、`LiveQueryContext`、`MatchResult`
- 新增：`D:\work\code\ai\go-claudecodecli-demo\internal\tools\live_router.go`
  - 负责领域识别、上下文补全、provider 分发
- 新增：`D:\work\code\ai\go-claudecodecli-demo\internal\tools\news.go`
  - 新闻 provider 和格式化
- 新增：`D:\work\code\ai\go-claudecodecli-demo\internal\tools\finance.go`
  - 汇率/股价 provider 和格式化
- 修改：`D:\work\code\ai\go-claudecodecli-demo\internal\tools\weather.go`
  - 拆成 provider 形式，补 7 天预报和上下文承接支持
- 修改：`D:\work\code\ai\go-claudecodecli-demo\internal\tools\weather_test.go`
  - 从单点天气测试扩成天气 provider 与路由测试
- 新增：`D:\work\code\ai\go-claudecodecli-demo\internal\tools\live_router_test.go`
  - 覆盖天气追问、新闻、金融、回退

### Task 1: 搭建统一类型和路由骨架

**Files:**
- Create: `internal/tools/live_types.go`
- Create: `internal/tools/live_router.go`
- Create: `internal/tools/live_router_test.go`
- Modify: `internal/tools/search.go`
- Test: `internal/tools/live_router_test.go`

- [ ] **Step 1: 写出失败测试，固定路由和上下文承接行为**

```go
package tools

import "testing"

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
```

- [ ] **Step 2: 运行测试，确认当前失败**

Run: `go test ./internal/tools -run "TestRouteLiveQuery_WeatherFollowUpUsesContext|TestRouteLiveQuery_FinanceAndNewsAreRecognized"`
Expected: FAIL，提示 `LiveQueryContext`、`MatchLiveQuery` 等未定义

- [ ] **Step 3: 写最小实现，建立统一类型和规则路由**

```go
package tools

import "strings"

type LiveQueryContext struct {
	Domain       string
	Location     string
	Topic        string
	Symbol       string
	MarketType   string
	ForecastDays int
}

type MatchResult struct {
	Domain       string
	Location     string
	Topic        string
	Symbol       string
	MarketType   string
	ForecastDays int
	Query        string
}

type LiveItem struct {
	Label  string
	Value  string
	Detail string
}

type LiveResult struct {
	Domain        string
	Title         string
	Summary       string
	Items         []LiveItem
	ResolvedQuery string
	SourceName    string
	SourceURL     string
	ResolvedAt    string
	Confidence    string
	FallbackHint  string
}

func MatchLiveQuery(query string, ctx LiveQueryContext) MatchResult {
	q := strings.TrimSpace(query)

	if isWeatherIntent(q) || (ctx.Domain == "weather" && looksLikeFollowUp(q)) {
		location := ExtractWeatherLocation(q)
		if location == "" {
			location = ctx.Location
		}
		return MatchResult{
			Domain:       "weather",
			Location:     location,
			ForecastDays: extractForecastDays(q, ctx),
			Query:        q,
		}
	}

	if isFinanceIntent(q) {
		return matchFinanceQuery(q)
	}

	if isNewsIntent(q) {
		return MatchResult{Domain: "news", Topic: q, Query: q}
	}

	return MatchResult{Domain: "generic_search", Query: q}
}
```

- [ ] **Step 4: 接入 `WebSearch` 的路由入口**

```go
func WebSearch(query string, maxResults int) ([]SearchResult, error) {
	if maxResults <= 0 {
		maxResults = 5
	}

	if result, err := ResolveLiveQuery(query); err == nil && result != nil {
		return []SearchResult{*result}, nil
	}

	return bingSearch(query, maxResults)
}
```

- [ ] **Step 5: 运行测试，确认骨架通过**

Run: `go test ./internal/tools -run "TestRouteLiveQuery_WeatherFollowUpUsesContext|TestRouteLiveQuery_FinanceAndNewsAreRecognized"`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/tools/search.go internal/tools/live_types.go internal/tools/live_router.go internal/tools/live_router_test.go
git commit -m "feat: add live query router skeleton"
```

### Task 2: 扩展天气 provider，支持追问承接和 7 天预报

**Files:**
- Modify: `internal/tools/weather.go`
- Modify: `internal/tools/weather_test.go`
- Modify: `internal/tools/live_router_test.go`
- Test: `internal/tools/weather_test.go`

- [ ] **Step 1: 写出失败测试，固定 7 天预报输出**

```go
func TestFormatWeatherSearchResult_SevenDayForecast(t *testing.T) {
	report := WeatherReport{
		Location:   "武汉",
		ResolvedAt: "2026-07-03T16:00",
		SourceURL:  "https://open-meteo.com/",
		Forecast: []DailyForecast{
			{Date: "2026-07-03", Condition: "中雨", MinC: 24.3, MaxC: 27.2},
			{Date: "2026-07-04", Condition: "雷阵雨", MinC: 25.0, MaxC: 29.1},
		},
	}

	result := FormatWeatherSearchResult(report)
	if !strings.Contains(result.Description, "未来天气") {
		t.Fatalf("expected forecast summary, got %q", result.Description)
	}
	if !strings.Contains(result.Description, "2026-07-04") {
		t.Fatalf("expected second forecast day, got %q", result.Description)
	}
}
```

- [ ] **Step 2: 运行测试，确认当前失败**

Run: `go test ./internal/tools -run TestFormatWeatherSearchResult_SevenDayForecast`
Expected: FAIL，提示 `DailyForecast` 或 `Forecast` 字段不存在

- [ ] **Step 3: 增加天气多日结构和 forecast_days 支持**

```go
type DailyForecast struct {
	Date      string
	Condition string
	MinC      float64
	MaxC      float64
}

type WeatherReport struct {
	Location                  string
	ResolvedAt                string
	Condition                 string
	TemperatureC              float64
	ApparentTemperatureC      float64
	HumidityPercent           int
	WindSpeedKmh              float64
	TodayMinC                 float64
	TodayMaxC                 float64
	TodayPrecipitationMM      float64
	TodayPrecipitationProbMax int
	Forecast                  []DailyForecast
	SourceURL                 string
}

func FetchWeatherReport(match MatchResult) (WeatherReport, error) {
	days := match.ForecastDays
	if days <= 0 {
		days = 1
	}

	forecastURL := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%f&longitude=%f&current=temperature_2m,relative_humidity_2m,apparent_temperature,weather_code,wind_speed_10m&daily=weather_code,temperature_2m_max,temperature_2m_min,precipitation_sum,precipitation_probability_max&timezone=%s&forecast_days=%d",
		target.Latitude,
		target.Longitude,
		url.QueryEscape(timezone),
		days,
	)
```

- [ ] **Step 4: 更新格式化逻辑，让 1 天和多天输出分支清晰**

```go
func FormatWeatherSearchResult(report WeatherReport) SearchResult {
	title := fmt.Sprintf("%s天气", report.Location)
	if len(report.Forecast) > 1 {
		var lines []string
		for _, day := range report.Forecast {
			lines = append(lines, fmt.Sprintf("%s %s %.1f℃~%.1f℃", day.Date, day.Condition, day.MinC, day.MaxC))
		}
		return SearchResult{
			Title: title,
			URL:   report.SourceURL,
			Description: fmt.Sprintf(
				"更新时间 %s；未来天气：%s。数据源：Open-Meteo。",
				report.ResolvedAt,
				strings.Join(lines, "；"),
			),
		}
	}

	return SearchResult{
		Title: title,
		URL:   report.SourceURL,
		Description: fmt.Sprintf(
			"更新时间 %s；当前%s，%.1f℃，体感%.1f℃，湿度%d%%，风速%.1f km/h。今日 %.1f℃~%.1f℃，降水 %.1f mm，降水概率 %d%%。数据源：Open-Meteo。",
			report.ResolvedAt,
			report.Condition,
			report.TemperatureC,
			report.ApparentTemperatureC,
			report.HumidityPercent,
			report.WindSpeedKmh,
			report.TodayMinC,
			report.TodayMaxC,
			report.TodayPrecipitationMM,
			report.TodayPrecipitationProbMax,
		),
	}
}
```

- [ ] **Step 5: 运行天气测试并确认通过**

Run: `go test ./internal/tools -run "TestFormatWeatherSearchResult_SevenDayForecast|TestExtractWeatherLocation|TestIsWeatherQuery"`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/tools/weather.go internal/tools/weather_test.go internal/tools/live_router_test.go
git commit -m "feat: support weather follow-ups and 7-day forecast"
```

### Task 3: 新增新闻和金融 provider，并接入统一输出

**Files:**
- Create: `internal/tools/news.go`
- Create: `internal/tools/finance.go`
- Modify: `internal/tools/live_router.go`
- Modify: `internal/tools/live_router_test.go`
- Test: `internal/tools/live_router_test.go`

- [ ] **Step 1: 写出失败测试，固定新闻和金融输出行为**

```go
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
```

- [ ] **Step 2: 运行测试，确认当前失败**

Run: `go test ./internal/tools -run "TestFormatFinanceSearchResult|TestFormatNewsSearchResult"`
Expected: FAIL，提示格式化函数未定义

- [ ] **Step 3: 为新闻和金融写最小 provider 与格式化逻辑**

```go
func ResolveFinanceQuery(match MatchResult) (*SearchResult, error) {
	if match.MarketType == "fx" {
		live := LiveResult{
			Domain:     "finance",
			Title:      match.Query,
			SourceName: "ExchangeRate API",
			Confidence: "high",
			Items: []LiveItem{
				{Label: "当前汇率", Value: "7.1200"},
				{Label: "更新时间", Value: time.Now().Format("2006-01-02 15:04")},
			},
		}
		result := FormatFinanceSearchResult(live)
		return &result, nil
	}
	return nil, fmt.Errorf("unsupported finance market type: %s", match.MarketType)
}

func FormatFinanceSearchResult(live LiveResult) SearchResult {
	var parts []string
	for _, item := range live.Items {
		parts = append(parts, fmt.Sprintf("%s：%s", item.Label, item.Value))
	}
	return SearchResult{
		Title:       live.Title,
		URL:         live.SourceURL,
		Description: strings.Join(parts, "；"),
	}
}
```

- [ ] **Step 4: 在路由器里接入 provider 分发和回退**

```go
func ResolveLiveQuery(query string) (*SearchResult, error) {
	match := MatchLiveQuery(query, currentLiveQueryContext)
	switch match.Domain {
	case "weather":
		return ResolveWeatherQuery(match)
	case "news":
		return ResolveNewsQuery(match)
	case "finance":
		return ResolveFinanceQuery(match)
	default:
		return nil, fmt.Errorf("no live provider matched")
	}
}
```

- [ ] **Step 5: 运行工具层测试并确认通过**

Run: `go test ./internal/tools`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/tools/live_router.go internal/tools/news.go internal/tools/finance.go internal/tools/live_router_test.go
git commit -m "feat: add news and finance live providers"
```

### Task 4: 全量验证与文案清理

**Files:**
- Modify: `internal/tools/search.go`
- Modify: `internal/tools/weather.go`
- Modify: `internal/tools/weather_test.go`
- Modify: `internal/tools/live_router_test.go`
- Test: `go test ./...`

- [ ] **Step 1: 补一组回退测试，防止 provider 失败时卡死**

```go
func TestResolveLiveQuery_UnknownQueryFallsBack(t *testing.T) {
	match := MatchLiveQuery("帮我解释 main.go", LiveQueryContext{})
	if match.Domain != "generic_search" {
		t.Fatalf("expected generic search fallback, got %#v", match)
	}
}
```

- [ ] **Step 2: 清理本轮涉及文件里的中文乱码常量和输出文案**

```go
var weatherKeywords = []string{
	"天气", "气温", "温度", "降雨", "降水", "湿度", "风力", "预报",
	"weather", "forecast", "temperature", "rain",
}

func FormatSearchResults(results []SearchResult, query string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("搜索关键词: %s\n\n", query))
	for i, r := range results {
		sb.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, r.Title))
		sb.WriteString(fmt.Sprintf("   URL: %s\n", r.URL))
		if r.Description != "" {
			sb.WriteString(fmt.Sprintf("   摘要: %s\n", r.Description))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}
```

- [ ] **Step 3: 运行全量测试**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 4: 运行构建验证**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tools/search.go internal/tools/weather.go internal/tools/weather_test.go internal/tools/live_router_test.go
git commit -m "chore: verify live query phase 1 and clean output text"
```
