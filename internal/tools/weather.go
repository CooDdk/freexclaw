package tools

import (
	"encoding/json"
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
	"今天", " ",
	"明天", " ",
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
	if match.Domain != "weather" && !IsWeatherQuery(match.Query) {
		return nil, nil
	}

	report, err := FetchWeatherReport(match)
	if err != nil {
		return nil, err
	}

	result := FormatWeatherSearchResult(report)
	return &result, nil
}

func FetchWeatherReport(match MatchResult) (WeatherReport, error) {
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
	if err := getJSON(geoURL, &geo); err != nil {
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

	forecastURL := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%f&longitude=%f&current=temperature_2m,relative_humidity_2m,apparent_temperature,weather_code,wind_speed_10m&daily=weather_code,temperature_2m_max,temperature_2m_min,precipitation_sum,precipitation_probability_max&timezone=%s&forecast_days=%d",
		target.Latitude,
		target.Longitude,
		url.QueryEscape(timezone),
		forecastDays,
	)

	var forecast forecastResponse
	if err := getJSON(forecastURL, &forecast); err != nil {
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
	}

	return report, nil
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

func getJSON(requestURL string, target interface{}) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		err := getJSONOnce(requestURL, target)
		if err == nil {
			return nil
		}
		lastErr = err
		time.Sleep(time.Duration(attempt+1) * 250 * time.Millisecond)
	}
	return lastErr
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
