package tools

type LiveQueryContext struct {
	Domain       string
	Location     string
	Topic        string
	Symbol       string
	MarketType   string
	ForecastDays int
	TimeOfDay    string
}

type MatchResult struct {
	Domain       string
	Location     string
	Topic        string
	Symbol       string
	MarketType   string
	ForecastDays int
	TimeOfDay    string
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
