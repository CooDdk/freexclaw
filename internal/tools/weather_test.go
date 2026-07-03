package tools

import (
	"strings"
	"testing"
)

func TestIsWeatherQuery(t *testing.T) {
	if !IsWeatherQuery("看一下今天武汉的天气情况") {
		t.Fatal("expected weather query to be detected")
	}

	if IsWeatherQuery("帮我读取 main.go 文件") {
		t.Fatal("did not expect file read request to be treated as weather query")
	}
}

func TestExtractWeatherLocation(t *testing.T) {
	location := ExtractWeatherLocation("看一下今天武汉的天气情况")
	if location != "武汉" {
		t.Fatalf("expected location 武汉, got %q", location)
	}
}

func TestExtractWeatherLocation_StripsProviderNoise(t *testing.T) {
	location := ExtractWeatherLocation("武汉 实时天气 2026-07-03 Open-Meteo")
	if location != "武汉" {
		t.Fatalf("expected location 武汉, got %q", location)
	}
}

func TestExtractWeatherLocation_StripsTimeNoise(t *testing.T) {
	location := ExtractWeatherLocation("武汉 16:15 实时天气")
	if location != "武汉" {
		t.Fatalf("expected location 武汉, got %q", location)
	}
}

func TestFormatWeatherSearchResult(t *testing.T) {
	report := WeatherReport{
		Location:                  "武汉",
		ResolvedAt:                "2026-07-03 15:15",
		Condition:                 "中雨",
		TemperatureC:              27.2,
		ApparentTemperatureC:      32.4,
		HumidityPercent:           86,
		WindSpeedKmh:              9.5,
		TodayMinC:                 24.3,
		TodayMaxC:                 27.2,
		TodayPrecipitationMM:      22.7,
		TodayPrecipitationProbMax: 100,
		SourceURL:                 "https://api.open-meteo.com/",
	}

	result := FormatWeatherSearchResult(report)
	if result.Title == "" || result.URL == "" || result.Description == "" {
		t.Fatalf("expected formatted weather search result to be fully populated, got %#v", result)
	}

	if result.Title != "武汉天气" {
		t.Fatalf("expected weather title, got %q", result.Title)
	}
}

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
