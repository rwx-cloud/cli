package telemetry

import (
	"net/http"
	"sync"
	"time"
)

// CallStats records a single API round-trip.
type CallStats struct {
	Path       string `json:"path"`
	Method     string `json:"method"`
	StatusCode int    `json:"status_code"`
	DurationMS int64  `json:"duration_ms"`
}

// StatsRoundTripper wraps an http.RoundTripper and accumulates per-call stats.
// At flush time, call RecordSummary to emit an aggregated api.summary event.
type StatsRoundTripper struct {
	http.RoundTripper
	mu    sync.Mutex
	calls []CallStats
}

// NewStatsRoundTripper wraps rt with call-level stats accumulation.
func NewStatsRoundTripper(rt http.RoundTripper) *StatsRoundTripper {
	return &StatsRoundTripper{RoundTripper: rt}
}

func (s *StatsRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	path := req.URL.Path

	// Don't track telemetry endpoint calls to avoid self-referential stats
	if path == "/api/telemetry" {
		return s.RoundTripper.RoundTrip(req)
	}

	start := time.Now()
	resp, err := s.RoundTripper.RoundTrip(req)
	durationMS := time.Since(start).Milliseconds()

	if err != nil {
		cs := CallStats{
			Path:       path,
			Method:     req.Method,
			StatusCode: 0,
			DurationMS: durationMS,
		}
		s.mu.Lock()
		s.calls = append(s.calls, cs)
		s.mu.Unlock()
		return resp, err
	}

	cs := CallStats{
		Path:       path,
		Method:     req.Method,
		StatusCode: resp.StatusCode,
		DurationMS: durationMS,
	}

	s.mu.Lock()
	s.calls = append(s.calls, cs)
	s.mu.Unlock()

	return resp, nil
}

// AggregatedCall is a summary of calls to a single path+method combination.
type AggregatedCall struct {
	Path        string `json:"path"`
	Method      string `json:"method"`
	Count       int    `json:"count"`
	StatusCodes []int  `json:"status_codes"`
	TotalMS     int64  `json:"total_ms"`
}

// RecordSummary drains accumulated stats and records an api.summary event
// into the given collector. If no calls were recorded, it does nothing.
func (s *StatsRoundTripper) RecordSummary(collector *Collector) {
	s.mu.Lock()
	calls := s.calls
	s.calls = nil
	s.mu.Unlock()

	if len(calls) == 0 {
		return
	}

	type key struct {
		Path   string
		Method string
	}

	// Preserve insertion order
	var order []key
	agg := make(map[key]*AggregatedCall)

	for _, c := range calls {
		k := key{c.Path, c.Method}
		entry, exists := agg[k]
		if !exists {
			entry = &AggregatedCall{
				Path:   c.Path,
				Method: c.Method,
			}
			agg[k] = entry
			order = append(order, k)
		}
		entry.Count++
		entry.TotalMS += c.DurationMS
		entry.StatusCodes = append(entry.StatusCodes, c.StatusCode)
	}

	result := make([]AggregatedCall, 0, len(order))
	for _, k := range order {
		result = append(result, *agg[k])
	}

	collector.Record("api.summary", map[string]any{
		"calls": result,
	})
}
