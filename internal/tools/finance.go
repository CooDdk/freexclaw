package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type exchangeRateResponse struct {
	Result            string             `json:"result"`
	BaseCode          string             `json:"base_code"`
	TimeLastUpdateUTC string             `json:"time_last_update_utc"`
	Rates             map[string]float64 `json:"rates"`
}

var financeSymbolAliases = map[string]string{
	"英伟达": "NVDA",
	"nvidia": "NVDA",
	"特斯拉": "TSLA",
	"tesla":  "TSLA",
	"苹果":   "AAPL",
	"apple":  "AAPL",
}

var currencyAliases = map[string]string{
	"人民币": "CNY",
	"美元":  "USD",
	"欧元":  "EUR",
	"日元":  "JPY",
	"港币":  "HKD",
	"英镑":  "GBP",
}

func ResolveFinanceQuery(match MatchResult) (*SearchResult, error) {
	if match.MarketType == "fx" {
		return resolveFXQuery(match)
	}

	return resolveEquityQuery(match)
}

func resolveFXQuery(match MatchResult) (*SearchResult, error) {
	base, target := parseCurrencyPair(match.Query)
	if base == "" || target == "" {
		return nil, fmt.Errorf("无法识别汇率货币对: %s", match.Query)
	}

	requestURL := fmt.Sprintf("https://open.er-api.com/v6/latest/%s", base)
	var payload exchangeRateResponse
	if err := getFinanceJSON(requestURL, &payload); err != nil {
		return nil, err
	}

	rate, ok := payload.Rates[target]
	if !ok {
		return nil, fmt.Errorf("汇率结果缺少目标币种: %s", target)
	}

	live := LiveResult{
		Domain:        "finance",
		Title:         fmt.Sprintf("%s兑%s", currencyCodeToChinese(base), currencyCodeToChinese(target)),
		ResolvedQuery: match.Query,
		SourceName:    "ExchangeRate API",
		SourceURL:     "https://www.exchangerate-api.com/",
		ResolvedAt:    payload.TimeLastUpdateUTC,
		Confidence:    "high",
		Items: []LiveItem{
			{Label: "当前汇率", Value: fmt.Sprintf("%.4f", rate)},
			{Label: "货币对", Value: fmt.Sprintf("%s/%s", base, target)},
			{Label: "更新时间", Value: payload.TimeLastUpdateUTC},
		},
	}

	formatted := FormatFinanceSearchResult(live)
	return &formatted, nil
}

func resolveEquityQuery(match MatchResult) (*SearchResult, error) {
	query := match.Query
	results, err := bingSearch(query+" 实时 股价", 3)
	if err != nil {
		return nil, err
	}

	live := LiveResult{
		Domain:        "finance",
		Title:         query,
		ResolvedQuery: query,
		SourceName:    "Bing Finance Search",
		SourceURL:     "https://www.bing.com/search",
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

	formatted := FormatFinanceSearchResult(live)
	return &formatted, nil
}

func FormatFinanceSearchResult(live LiveResult) SearchResult {
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
		description = "暂无可用金融数据。"
	}

	return SearchResult{
		Title:       live.Title,
		URL:         live.SourceURL,
		Description: description,
	}
}

func getFinanceJSON(requestURL string, target interface{}) error {
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
		return fmt.Errorf("金融服务返回状态码 %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return err
	}

	return json.Unmarshal(body, target)
}

func parseCurrencyPair(query string) (string, string) {
	parts := strings.Split(query, "兑")
	if len(parts) != 2 {
		return "", ""
	}

	base := normalizeCurrencyCode(parts[0])
	target := normalizeCurrencyCode(parts[1])
	return base, target
}

func normalizeCurrencyCode(value string) string {
	value = strings.TrimSpace(strings.TrimSuffix(value, "汇率"))
	if code, ok := currencyAliases[value]; ok {
		return code
	}
	return strings.ToUpper(value)
}

func currencyCodeToChinese(code string) string {
	for name, alias := range currencyAliases {
		if alias == code {
			return name
		}
	}
	return code
}
