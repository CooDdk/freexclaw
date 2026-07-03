package tools

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var searchHTTPClient = &http.Client{Timeout: 15 * time.Second}

type SearchResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

func WebSearch(query string, maxResults int) ([]SearchResult, error) {
	if maxResults <= 0 {
		maxResults = 5
	}

	match := MatchLiveQuery(query, currentLiveQueryContext)
	if match.Domain != "generic_search" {
		liveResult, err := ResolveLiveQuery(query)
		if err != nil {
			return nil, fmt.Errorf("实时查询失败: %w", err)
		}
		if liveResult != nil {
			return []SearchResult{*liveResult}, nil
		}
	}
	return bingSearch(query, maxResults)
}

var bingEndpoints = []string{
	"https://cn.bing.com/search?q=%s&count=%d",
	"https://www.bing.com/search?q=%s&count=%d",
}

func bingSearch(query string, maxResults int) ([]SearchResult, error) {
	var lastErr error
	for _, endpoint := range bingEndpoints {
		results, err := tryBing(endpoint, query, maxResults)
		if err == nil {
			return results, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("搜索失败: %w", lastErr)
}

func tryBing(endpoint, query string, maxResults int) ([]SearchResult, error) {
	searchURL := fmt.Sprintf(endpoint, url.QueryEscape(query), maxResults)

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")

	resp, err := searchHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("状态码 %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return nil, err
	}

	raw := string(body)
	if !strings.Contains(raw, "b_algo") {
		return nil, fmt.Errorf("搜索结果页异常")
	}

	return parseBing(raw, maxResults), nil
}

func parseBing(html string, maxResults int) []SearchResult {
	var results []SearchResult
	pos := 0

	for len(results) < maxResults {
		liIdx := strings.Index(html[pos:], `<li class="b_algo"`)
		if liIdx < 0 {
			liIdx = strings.Index(html[pos:], `class="b_algo"`)
		}
		if liIdx < 0 {
			break
		}
		start := pos + liIdx

		endLi := strings.Index(html[start:], `</li>`)
		if endLi < 0 {
			break
		}
		end := start + endLi + 5
		block := html[start:end]
		pos = end

		r := extractBing(block)
		if r.Title != "" && r.URL != "" {
			results = append(results, r)
		}
	}

	return results
}

func extractBing(block string) SearchResult {
	var r SearchResult

	h2Start := strings.Index(block, `<h2`)
	if h2Start >= 0 {
		h2End := strings.Index(block[h2Start:], `</h2>`)
		if h2End >= 0 {
			h2Block := block[h2Start : h2Start+h2End]

			aStart := strings.Index(h2Block, `<a `)
			if aStart >= 0 {
				aBlock := h2Block[aStart:]

				hrefStart := strings.Index(aBlock, `href="`)
				if hrefStart >= 0 {
					hrefStart += 6
					hrefEnd := strings.Index(aBlock[hrefStart:], `"`)
					if hrefEnd >= 0 {
						r.URL = aBlock[hrefStart : hrefStart+hrefEnd]
					}
				}

				aClose := strings.Index(aBlock, `</a>`)
				if aClose >= 0 {
					r.Title = stripAllTags(aBlock[:aClose])
				}
			}
		}
	}

	captionStart := strings.Index(block, `<p class="b_lineclamp2"`)
	if captionStart < 0 {
		captionStart = strings.Index(block, `<p class="b_lineclamp`)
	}
	if captionStart < 0 {
		captionStart = strings.Index(block, `<div class="b_caption"`)
		if captionStart >= 0 {
			pStart := strings.Index(block[captionStart:], `<p`)
			if pStart >= 0 {
				captionStart = captionStart + pStart
			}
		}
	}
	if captionStart >= 0 {
		pClose := strings.Index(block[captionStart:], `</p>`)
		if pClose >= 0 {
			pBlock := block[captionStart : captionStart+pClose]
			gtIdx := strings.Index(pBlock, ">")
			if gtIdx >= 0 {
				r.Description = stripAllTags(pBlock[gtIdx+1:])
			}
		}
	}

	if r.Description == "" {
		captionStart = strings.Index(block, `<div class="b_caption"`)
		if captionStart >= 0 {
			divClose := strings.Index(block[captionStart:], `</div>`)
			if divClose >= 0 {
				divBlock := block[captionStart : captionStart+divClose]
				gtIdx := strings.Index(divBlock, ">")
				if gtIdx >= 0 {
					r.Description = stripAllTags(divBlock[gtIdx+1:])
				}
			}
		}
	}

	if len(r.Description) > 500 {
		r.Description = r.Description[:500]
	}

	r.Title = cleanEntity(r.Title)
	r.Description = cleanEntity(r.Description)

	return r
}

func stripAllTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, c := range s {
		if c == '<' {
			inTag = true
			continue
		}
		if c == '>' {
			inTag = false
			continue
		}
		if !inTag {
			b.WriteRune(c)
		}
	}
	return strings.TrimSpace(b.String())
}

func cleanEntity(s string) string {
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&ensp;", " ")
	s = strings.ReplaceAll(s, "&ndash;", "–")
	s = strings.ReplaceAll(s, "&mdash;", "—")
	s = strings.TrimSpace(s)
	return s
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
