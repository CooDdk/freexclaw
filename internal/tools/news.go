package tools

import (
	"fmt"
	"strings"
	"time"
)

func ResolveNewsQuery(match MatchResult) (*SearchResult, error) {
	query := match.Topic
	if query == "" {
		query = match.Query
	}

	results, err := bingSearch(query, 3)
	if err != nil {
		return nil, err
	}

	live := LiveResult{
		Domain:        "news",
		Title:         "新闻摘要",
		ResolvedQuery: query,
		SourceName:    "Bing News Search",
		SourceURL:     "https://www.bing.com/news",
		ResolvedAt:    time.Now().Format("2006-01-02 15:04"),
		Confidence:    "medium",
		Items:         make([]LiveItem, 0, len(results)),
	}

	for _, result := range results {
		live.Items = append(live.Items, LiveItem{
			Label:  result.Title,
			Value:  result.Description,
			Detail: result.URL,
		})
	}

	formatted := FormatNewsSearchResult(live)
	return &formatted, nil
}

func FormatNewsSearchResult(live LiveResult) SearchResult {
	parts := make([]string, 0, len(live.Items))
	for _, item := range live.Items {
		if item.Detail != "" {
			parts = append(parts, fmt.Sprintf("%s：%s（%s）", item.Label, item.Value, item.Detail))
			continue
		}
		parts = append(parts, fmt.Sprintf("%s：%s", item.Label, item.Value))
	}

	description := strings.Join(parts, "；")
	if description == "" {
		description = "暂无可用新闻摘要。"
	}

	if live.ResolvedAt != "" {
		description = fmt.Sprintf("整理时间 %s；%s", live.ResolvedAt, description)
	}

	return SearchResult{
		Title:       live.Title,
		URL:         live.SourceURL,
		Description: description,
	}
}
