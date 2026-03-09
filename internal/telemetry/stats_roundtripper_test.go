package telemetry

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestServer(statusCode int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
	}))
}

func TestStatsRoundTripper_TracksAPICalls(t *testing.T) {
	server := newTestServer(http.StatusOK)
	defer server.Close()

	inner := &testRoundTripper{server: server}
	srt := NewStatsRoundTripper(inner)

	req, _ := http.NewRequest(http.MethodGet, "/mint/api/runs", nil)
	resp, err := srt.RoundTrip(req)
	require.NoError(t, err)
	resp.Body.Close()

	collector := NewCollector()
	srt.RecordSummary(collector)

	events := collector.Drain()
	require.Len(t, events, 1)
	require.Equal(t, "api.summary", events[0].Event)

	calls, ok := events[0].Props["calls"].([]AggregatedCall)
	require.True(t, ok)
	require.Len(t, calls, 1)
	require.Equal(t, "/mint/api/runs", calls[0].Path)
	require.Equal(t, http.MethodGet, calls[0].Method)
	require.Equal(t, 1, calls[0].Count)
	require.Equal(t, []int{200}, calls[0].StatusCodes)
}

func TestStatsRoundTripper_AggregatesDuplicatePaths(t *testing.T) {
	server := newTestServer(http.StatusOK)
	defer server.Close()

	inner := &testRoundTripper{server: server}
	srt := NewStatsRoundTripper(inner)

	for i := 0; i < 3; i++ {
		req, _ := http.NewRequest(http.MethodGet, "/mint/api/runs", nil)
		resp, err := srt.RoundTrip(req)
		require.NoError(t, err)
		resp.Body.Close()
	}

	collector := NewCollector()
	srt.RecordSummary(collector)

	events := collector.Drain()
	require.Len(t, events, 1)

	calls := events[0].Props["calls"].([]AggregatedCall)
	require.Len(t, calls, 1)
	require.Equal(t, 3, calls[0].Count)
	require.Len(t, calls[0].StatusCodes, 3)
}

func TestStatsRoundTripper_ExcludesTelemetryEndpoint(t *testing.T) {
	server := newTestServer(http.StatusOK)
	defer server.Close()

	inner := &testRoundTripper{server: server}
	srt := NewStatsRoundTripper(inner)

	req, _ := http.NewRequest(http.MethodPost, "/api/telemetry", nil)
	resp, err := srt.RoundTrip(req)
	require.NoError(t, err)
	resp.Body.Close()

	collector := NewCollector()
	srt.RecordSummary(collector)

	events := collector.Drain()
	require.Empty(t, events)
}

func TestStatsRoundTripper_NoCallsNoEvent(t *testing.T) {
	srt := NewStatsRoundTripper(http.DefaultTransport)

	collector := NewCollector()
	srt.RecordSummary(collector)

	events := collector.Drain()
	require.Empty(t, events)
}

func TestStatsRoundTripper_DrainsAfterRecordSummary(t *testing.T) {
	server := newTestServer(http.StatusOK)
	defer server.Close()

	inner := &testRoundTripper{server: server}
	srt := NewStatsRoundTripper(inner)

	req, _ := http.NewRequest(http.MethodGet, "/mint/api/runs", nil)
	resp, err := srt.RoundTrip(req)
	require.NoError(t, err)
	resp.Body.Close()

	collector := NewCollector()
	srt.RecordSummary(collector)

	// Second call should produce nothing
	collector2 := NewCollector()
	srt.RecordSummary(collector2)
	events := collector2.Drain()
	require.Empty(t, events)
}

func TestStatsRoundTripper_SkipsOnTransportError(t *testing.T) {
	// Use a server that's immediately closed to trigger transport error
	server := newTestServer(http.StatusOK)
	server.Close()

	inner := &testRoundTripper{server: server}
	srt := NewStatsRoundTripper(inner)

	req, _ := http.NewRequest(http.MethodGet, "/mint/api/runs", nil)
	_, err := srt.RoundTrip(req)
	require.Error(t, err)

	collector := NewCollector()
	srt.RecordSummary(collector)

	events := collector.Drain()
	require.Empty(t, events)
}

func TestStatsRoundTripper_PreservesPathOrder(t *testing.T) {
	server := newTestServer(http.StatusOK)
	defer server.Close()

	inner := &testRoundTripper{server: server}
	srt := NewStatsRoundTripper(inner)

	paths := []string{"/mint/api/runs", "/api/auth/whoami", "/mint/api/runs"}
	for _, p := range paths {
		req, _ := http.NewRequest(http.MethodGet, p, nil)
		resp, err := srt.RoundTrip(req)
		require.NoError(t, err)
		resp.Body.Close()
	}

	collector := NewCollector()
	srt.RecordSummary(collector)

	events := collector.Drain()
	calls := events[0].Props["calls"].([]AggregatedCall)
	require.Len(t, calls, 2)
	require.Equal(t, "/mint/api/runs", calls[0].Path)
	require.Equal(t, 2, calls[0].Count)
	require.Equal(t, "/api/auth/whoami", calls[1].Path)
	require.Equal(t, 1, calls[1].Count)
}
