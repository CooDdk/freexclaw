package tools

import (
	"fmt"
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

func TestExtractWeatherLocation_StripsForecastPhrase(t *testing.T) {
	location := ExtractWeatherLocation("北京未来3天的呢")
	if location != "北京" {
		t.Fatalf("expected location 北京, got %q", location)
	}
}

func TestExtractWeatherLocation_StripsLatestPhrase(t *testing.T) {
	location := ExtractWeatherLocation("北京最新的天气")
	if location != "北京" {
		t.Fatalf("expected location 北京, got %q", location)
	}
}

func TestExtractWeatherLocation_StripsCurrentPhrase(t *testing.T) {
	location := ExtractWeatherLocation("北京当前的天气")
	if location != "北京" {
		t.Fatalf("expected location 北京, got %q", location)
	}
}

func TestExtractWeatherLocation_StripsTimeOfDayPhrase(t *testing.T) {
	for _, q := range []string{"北京下午的天气", "北京晚上天气怎么样", "上海明天上午的天气"} {
		location := ExtractWeatherLocation(q)
		if location != "北京" && location != "上海" {
			t.Fatalf("expected city location for %q, got %q", q, location)
		}
	}
}

func TestPickTimeSlotForecast_SelectsAfternoonHour(t *testing.T) {
	var forecast forecastResponse
	forecast.Daily.Time = []string{"2026-07-04"}
	forecast.Hourly.Time = []string{
		"2026-07-04T09:00", "2026-07-04T12:00", "2026-07-04T15:00", "2026-07-04T21:00",
	}
	forecast.Hourly.Temperature2M = []float64{30, 34, 37, 29}
	forecast.Hourly.ApparentTemperature = []float64{32, 36, 39, 30}
	forecast.Hourly.RelativeHumidity2M = []float64{40, 35, 30, 45}
	forecast.Hourly.WeatherCode = []int{1, 2, 3, 0}
	forecast.Hourly.WindSpeed10M = []float64{8, 9, 10, 7}
	forecast.Hourly.PrecipitationProbability = []int{10, 20, 30, 15}

	slot := pickTimeSlotForecast(forecast, "下午")
	if slot == nil {
		t.Fatal("expected a time slot forecast")
	}
	if slot.Time != "2026-07-04T15:00" {
		t.Fatalf("expected 15:00 slot for 下午, got %q", slot.Time)
	}
	if slot.TemperatureC != 37 {
		t.Fatalf("expected 37℃ for afternoon, got %.1f", slot.TemperatureC)
	}
}

func TestFormatWeatherSearchResult_TimeSlot(t *testing.T) {
	report := WeatherReport{
		Location:   "北京",
		TimeOfDay:  "下午",
		TodayMinC:  27.6,
		TodayMaxC:  39.2,
		SourceURL:  "https://open-meteo.com/",
		TimeSlot: &TimeSlotForecast{
			Label:             "下午",
			Time:              "2026-07-04T15:00",
			Condition:         "多云",
			TemperatureC:      37.0,
			ApparentC:         39.0,
			HumidityPercent:   30,
			WindSpeedKmh:      10.0,
			PrecipitationProb: 30,
		},
	}

	result := FormatWeatherSearchResult(report)
	if !strings.Contains(result.Description, "下午时段") {
		t.Fatalf("expected time slot description, got %q", result.Description)
	}
	if !strings.Contains(result.Description, "37.0℃") {
		t.Fatalf("expected slot temperature, got %q", result.Description)
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

func TestSummarizeWeatherRequestError_Timeout(t *testing.T) {
	err := summarizeWeatherRequestError("天气数据请求", fmt.Errorf(`Get "https://api.open-meteo.com/...": context deadline exceeded (Client.Timeout exceeded while awaiting headers)`))
	if !strings.Contains(err.Error(), "超时") {
		t.Fatalf("expected timeout summary, got %q", err.Error())
	}
	if strings.Contains(err.Error(), "https://api.open-meteo.com") {
		t.Fatalf("expected verbose URL to be trimmed, got %q", err.Error())
	}
}
