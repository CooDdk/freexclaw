package tools

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var weatherKeywords = []string{
	"天气", "气温", "温度", "降雨", "降水", "湿度", "风力", "预报",
	"weather", "forecast", "temperature", "rain",
}

var weatherNoiseReplacer = strings.NewReplacer(
	"看一下", " ",
	"看下", " ",
	"看看", " ",
	"查一下", " ",
	"查下", " ",
	"帮我", " ",
	"请", " ",
	"实时", " ",
	"open-meteo", " ",
	"open meteo", " ",
	"weather", " ",
	"forecast", " ",
	"api", " ",
	"现在", " ",
	"目前", " ",
	"当前", " ",
	"此刻", " ",
	"这会儿", " ",
	"刚刚", " ",
	"刚才", " ",
	"最新", " ",
	"实况", " ",
	"今天", " ",
	"明天", " ",
	"大后天", " ",
	"后天", " ",
	"未来7天", " ",
	"未来七天", " ",
	"天气情况", " ",
	"天气", " ",
	"气温", " ",
	"温度", " ",
	"湿度", " ",
	"风力", " ",
	"降雨", " ",
	"降水", " ",
	"预报", " ",
	"情况", " ",
	"如何", " ",
	"怎么样", " ",
	"是什么", " ",
	"多少度", " ",
	"的", " ",
	"呢", " ",
	"？", " ",
	"?", " ",
	"，", " ",
	",", " ",
)

var dateNoisePattern = regexp.MustCompile(`\d{4}[-/年]\d{1,2}[-/月]\d{1,2}(?:日)?`)
var timeNoisePattern = regexp.MustCompile(`\b\d{1,2}:\d{2}\b|\d{1,2}点(?:\d{1,2}分)?`)
var forecastPhrasePattern = regexp.MustCompile(`(?:未来|接下来|后续)?\s*[0-9一二两三四五六七八九十]+\s*天`)
var timeOfDayPhrasePattern = regexp.MustCompile(`凌晨|清晨|早晨|早上|上午|中午|下午|傍晚|晚上|夜晚|半夜|白天|夜间`)

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
	TimeOfDay                 string
	TimeSlot                  *TimeSlotForecast
}

type TimeSlotForecast struct {
	Label             string
	Time              string
	Condition         string
	TemperatureC      float64
	ApparentC         float64
	HumidityPercent   int
	WindSpeedKmh      float64
	PrecipitationProb int
}

type geoCodingResponse struct {
	Results []struct {
		Name      string  `json:"name"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Timezone  string  `json:"timezone"`
	} `json:"results"`
}

type forecastResponse struct {
	Current struct {
		Time                string  `json:"time"`
		Temperature2M       float64 `json:"temperature_2m"`
		RelativeHumidity2M  float64 `json:"relative_humidity_2m"`
		ApparentTemperature float64 `json:"apparent_temperature"`
		WeatherCode         int     `json:"weather_code"`
		WindSpeed10M        float64 `json:"wind_speed_10m"`
	} `json:"current"`
	Daily struct {
		Time                     []string  `json:"time"`
		Temperature2MMax         []float64 `json:"temperature_2m_max"`
		Temperature2MMin         []float64 `json:"temperature_2m_min"`
		WeatherCode              []int     `json:"weather_code"`
		PrecipitationSum         []float64 `json:"precipitation_sum"`
		PrecipitationProbability []int     `json:"precipitation_probability_max"`
	} `json:"daily"`
	Hourly struct {
		Time                     []string  `json:"time"`
		Temperature2M            []float64 `json:"temperature_2m"`
		RelativeHumidity2M       []float64 `json:"relative_humidity_2m"`
		ApparentTemperature      []float64 `json:"apparent_temperature"`
		WeatherCode              []int     `json:"weather_code"`
		WindSpeed10M             []float64 `json:"wind_speed_10m"`
		PrecipitationProbability []int     `json:"precipitation_probability"`
	} `json:"hourly"`
}

func IsWeatherQuery(query string) bool {
	lower := strings.ToLower(query)
	for _, keyword := range weatherKeywords {
		if strings.Contains(lower, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func ExtractWeatherLocation(query string) string {
	cleaned := dateNoisePattern.ReplaceAllString(query, " ")
	cleaned = timeNoisePattern.ReplaceAllString(cleaned, " ")
	cleaned = forecastPhrasePattern.ReplaceAllString(cleaned, " ")
	cleaned = timeOfDayPhrasePattern.ReplaceAllString(cleaned, " ")
	cleaned = weatherNoiseReplacer.Replace(strings.ToLower(cleaned))
	cleaned = strings.Join(strings.Fields(cleaned), " ")
	cleaned = trimWeatherNoiseTokens(cleaned)
	return strings.TrimSpace(cleaned)
}

func trimWeatherNoiseTokens(location string) string {
	if location == "" {
		return ""
	}

	noiseTokens := map[string]struct{}{
		"open-meteo": {},
		"open":       {},
		"meteo":      {},
		"weather":    {},
		"forecast":   {},
		"api":        {},
	}

	parts := strings.Fields(location)
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		if _, ok := noiseTokens[part]; ok {
			continue
		}
		filtered = append(filtered, part)
	}

	return strings.Join(filtered, " ")
}

func GetWeatherSearchResult(match MatchResult) (*SearchResult, error) {
	return GetWeatherSearchResultWithProgress(match, nil)
}

func GetWeatherSearchResultWithProgress(match MatchResult, progress func(string)) (*SearchResult, error) {
	if match.Domain != "weather" && !IsWeatherQuery(match.Query) {
		return nil, nil
	}

	report, err := FetchWeatherReportWithProgress(match, progress)
	if err != nil {
		return nil, err
	}

	result := FormatWeatherSearchResult(report)
	return &result, nil
}

func FetchWeatherReport(match MatchResult) (WeatherReport, error) {
	return FetchWeatherReportWithProgress(match, nil)
}

func FetchWeatherReportWithProgress(match MatchResult, progress func(string)) (WeatherReport, error) {
	location := strings.TrimSpace(match.Location)
	if location == "" {
		location = ExtractWeatherLocation(match.Query)
	}
	if location == "" {
		location = match.Query
	}

	geoURL := fmt.Sprintf(
		"https://geocoding-api.open-meteo.com/v1/search?name=%s&count=1&language=zh&format=json",
		url.QueryEscape(location),
	)

	var geo geoCodingResponse
	if progress != nil {
		progress(fmt.Sprintf("正在解析地点：%s", location))
	}
	if err := getJSON(geoURL, &geo, "地点解析", progress); err != nil {
		return WeatherReport{}, err
	}
	if len(geo.Results) == 0 {
		return WeatherReport{}, fmt.Errorf("未找到地点: %s", location)
	}

	target := geo.Results[0]
	timezone := target.Timezone
	if timezone == "" {
		timezone = "auto"
	}

	forecastDays := match.ForecastDays
	if forecastDays <= 0 {
		forecastDays = 1
	}

	hourlyParam := ""
	if strings.TrimSpace(match.TimeOfDay) != "" {
		hourlyParam = "&hourly=temperature_2m,relative_humidity_2m,apparent_temperature,weather_code,wind_speed_10m,precipitation_probability"
	}

	forecastURL := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%f&longitude=%f&current=temperature_2m,relative_humidity_2m,apparent_temperature,weather_code,wind_speed_10m&daily=weather_code,temperature_2m_max,temperature_2m_min,precipitation_sum,precipitation_probability_max%s&timezone=%s&forecast_days=%d",
		target.Latitude,
		target.Longitude,
		hourlyParam,
		url.QueryEscape(timezone),
		forecastDays,
	)

	var forecast forecastResponse
	if progress != nil {
		progress(fmt.Sprintf("正在获取 %s 天气数据...", target.Name))
	}
	if err := getJSON(forecastURL, &forecast, "天气数据请求", progress); err != nil {
		return WeatherReport{}, err
	}

	report := WeatherReport{
		Location:                  target.Name,
		ResolvedAt:                forecast.Current.Time,
		Condition:                 weatherCodeToText(forecast.Current.WeatherCode),
		TemperatureC:              forecast.Current.Temperature2M,
		ApparentTemperatureC:      forecast.Current.ApparentTemperature,
		HumidityPercent:           int(math.Round(forecast.Current.RelativeHumidity2M)),
		WindSpeedKmh:              forecast.Current.WindSpeed10M,
		TodayMinC:                 firstFloat(forecast.Daily.Temperature2MMin),
		TodayMaxC:                 firstFloat(forecast.Daily.Temperature2MMax),
		TodayPrecipitationMM:      firstFloat(forecast.Daily.PrecipitationSum),
		TodayPrecipitationProbMax: firstInt(forecast.Daily.PrecipitationProbability, 0),
		Forecast:                  buildDailyForecasts(forecast.Daily),
		SourceURL:                 "https://open-meteo.com/",
		TimeOfDay:                 strings.TrimSpace(match.TimeOfDay),
	}

	if report.TimeOfDay != "" {
		report.TimeSlot = pickTimeSlotForecast(forecast, report.TimeOfDay)
	}

	return report, nil
}

func pickTimeSlotForecast(forecast forecastResponse, timeOfDay string) *TimeSlotForecast {
	hours := forecast.Hourly.Time
	if len(hours) == 0 {
		return nil
	}

	targetHour := timeOfDayToHour(timeOfDay)
	baseDate := ""
	if len(forecast.Daily.Time) > 0 {
		baseDate = forecast.Daily.Time[0]
	} else if len(hours) > 0 && len(hours[0]) >= 10 {
		baseDate = hours[0][:10]
	}

	bestIdx := -1
	bestScore := 1 << 30
	for i, ts := range hours {
		if len(ts) < 13 {
			continue
		}
		if baseDate != "" && !strings.HasPrefix(ts, baseDate) {
			continue
		}
		hour := parseHourFromTimestamp(ts)
		if hour < 0 {
			continue
		}
		score := hour - targetHour
		if score < 0 {
			score = -score
		}
		if score < bestScore {
			bestScore = score
			bestIdx = i
		}
	}

	if bestIdx < 0 {
		return nil
	}

	return &TimeSlotForecast{
		Label:             timeOfDay,
		Time:              hours[bestIdx],
		Condition:         weatherCodeToText(valueAtInt(forecast.Hourly.WeatherCode, bestIdx)),
		TemperatureC:      valueAtFloat(forecast.Hourly.Temperature2M, bestIdx),
		ApparentC:         valueAtFloat(forecast.Hourly.ApparentTemperature, bestIdx),
		HumidityPercent:   int(math.Round(valueAtFloat(forecast.Hourly.RelativeHumidity2M, bestIdx))),
		WindSpeedKmh:      valueAtFloat(forecast.Hourly.WindSpeed10M, bestIdx),
		PrecipitationProb: valueAtInt(forecast.Hourly.PrecipitationProbability, bestIdx),
	}
}

func timeOfDayToHour(timeOfDay string) int {
	switch timeOfDay {
	case "凌晨":
		return 3
	case "清晨", "早晨", "早上":
		return 7
	case "上午":
		return 10
	case "中午":
		return 12
	case "下午":
		return 15
	case "傍晚":
		return 18
	case "晚上", "夜晚", "夜间":
		return 21
	case "半夜":
		return 0
	}
	return 12
}

func parseHourFromTimestamp(ts string) int {
	idx := strings.IndexByte(ts, 'T')
	if idx < 0 || idx+3 > len(ts) {
		return -1
	}
	hourStr := ts[idx+1 : idx+3]
	hour := 0
	for _, r := range hourStr {
		if r < '0' || r > '9' {
			return -1
		}
		hour = hour*10 + int(r-'0')
	}
	return hour
}

func buildDailyForecasts(daily struct {
	Time                     []string  `json:"time"`
	Temperature2MMax         []float64 `json:"temperature_2m_max"`
	Temperature2MMin         []float64 `json:"temperature_2m_min"`
	WeatherCode              []int     `json:"weather_code"`
	PrecipitationSum         []float64 `json:"precipitation_sum"`
	PrecipitationProbability []int     `json:"precipitation_probability_max"`
}) []DailyForecast {
	maxLen := len(daily.Time)
	forecasts := make([]DailyForecast, 0, maxLen)
	for i := 0; i < maxLen; i++ {
		forecasts = append(forecasts, DailyForecast{
			Date:      daily.Time[i],
			Condition: weatherCodeToText(valueAtInt(daily.WeatherCode, i)),
			MinC:      valueAtFloat(daily.Temperature2MMin, i),
			MaxC:      valueAtFloat(daily.Temperature2MMax, i),
		})
	}
	return forecasts
}

func FormatWeatherSearchResult(report WeatherReport) SearchResult {
	title := fmt.Sprintf("%s天气", report.Location)

	if report.TimeSlot != nil {
		slot := report.TimeSlot
		return SearchResult{
			Title: title,
			URL:   report.SourceURL,
			Description: fmt.Sprintf(
				"%s%s时段（%s）：%s，%.1f℃，体感%.1f℃，湿度%d%%，风速%.1f km/h，降水概率 %d%%。今日 %.1f℃~%.1f℃。数据源：Open-Meteo。",
				report.Location,
				slot.Label,
				slot.Time,
				slot.Condition,
				slot.TemperatureC,
				slot.ApparentC,
				slot.HumidityPercent,
				slot.WindSpeedKmh,
				slot.PrecipitationProb,
				report.TodayMinC,
				report.TodayMaxC,
			),
		}
	}

	if len(report.Forecast) > 1 {
		lines := make([]string, 0, len(report.Forecast))
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

func getJSON(requestURL string, target interface{}, stage string, progress func(string)) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if progress != nil {
			progress(fmt.Sprintf("%s attempt %d/3", stage, attempt+1))
		}
		err := getJSONOnce(requestURL, target)
		if err == nil {
			return nil
		}
		lastErr = err
		time.Sleep(time.Duration(attempt+1) * 250 * time.Millisecond)
	}
	return summarizeWeatherRequestError(stage, lastErr)
}

func getJSONOnce(requestURL string, target interface{}) error {
	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "FREEXCLAW/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := searchHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("天气服务返回状态码 %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return err
	}

	if err := json.Unmarshal(body, target); err != nil {
		return err
	}
	return nil
}

func summarizeWeatherRequestError(stage string, err error) error {
	if err == nil {
		return nil
	}

	var netErr interface{ Timeout() bool }
	if errors.As(err, &netErr) && netErr.Timeout() {
		return fmt.Errorf("%s超时，已重试 3 次", stage)
	}

	msg := err.Error()
	if strings.Contains(strings.ToLower(msg), "timeout") {
		return fmt.Errorf("%s超时，已重试 3 次", stage)
	}
	if strings.Contains(msg, "天气服务返回状态码") {
		return fmt.Errorf("%s失败：%s", stage, msg)
	}
	return fmt.Errorf("%s失败：%s", stage, trimVerboseError(msg))
}

func trimVerboseError(msg string) string {
	msg = strings.TrimSpace(msg)
	if idx := strings.Index(msg, `"`); idx >= 0 {
		msg = strings.TrimSpace(msg[:idx])
	}
	if len(msg) > 120 {
		msg = msg[:120] + "..."
	}
	return msg
}

func firstFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	return values[0]
}

func firstInt(values []int, fallback int) int {
	if len(values) == 0 {
		return fallback
	}
	return values[0]
}

func valueAtFloat(values []float64, index int) float64 {
	if index < 0 || index >= len(values) {
		return 0
	}
	return values[index]
}

func valueAtInt(values []int, index int) int {
	if index < 0 || index >= len(values) {
		return 0
	}
	return values[index]
}

func weatherCodeToText(code int) string {
	switch code {
	case 0:
		return "晴"
	case 1, 2:
		return "多云"
	case 3:
		return "阴"
	case 45, 48:
		return "雾"
	case 51, 53, 55:
		return "毛毛雨"
	case 56, 57:
		return "冻毛毛雨"
	case 61, 63, 65:
		return "雨"
	case 66, 67:
		return "冻雨"
	case 71, 73, 75, 77:
		return "雪"
	case 80, 81, 82:
		return "阵雨"
	case 85, 86:
		return "阵雪"
	case 95:
		return "雷暴"
	case 96, 99:
		return "强对流"
	default:
		return "天气多变"
	}
}
