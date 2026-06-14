package contracts

import (
	"context"
	"time"
)

// TrafficSite holds request/byte counts for one site or aggregate total.
type TrafficSite struct {
	Requests int64 `json:"requests"`
	Bytes    int64 `json:"bytes"`
}

// TrafficSummary is rollup traffic suitable for dashboard cards.
type TrafficSummary struct {
	Sites map[string]TrafficSite `json:"sites"`
	Total TrafficSite            `json:"total"`
}

// TrafficSummaryProvider serves pre-aggregated traffic (e.g. grafanalite).
type TrafficSummaryProvider interface {
	TrafficSummary(ctx context.Context, since time.Time) (TrafficSummary, error)
	Available() bool
}
