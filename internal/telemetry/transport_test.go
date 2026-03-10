package telemetry

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

type testRoundTripper struct {
	server *httptest.Server
}

func (rt *testRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Rewrite the request URL to point at the test server while preserving the path
	req.URL.Scheme = "http"
	req.URL.Host = rt.server.Listener.Addr().String()
	return http.DefaultTransport.RoundTrip(req)
}

func TestSender_Send(t *testing.T) {
	var received []Event
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, telemetryEndpoint, r.URL.Path)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &received))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	rt := &testRoundTripper{server: server}

	collector := NewCollector()
	collector.Record("test_event", map[string]any{"foo": "bar"})

	sender := NewSender(collector, rt)
	sender.send(collector.Drain())

	require.Len(t, received, 1)
	require.Equal(t, "test_event", received[0].Event)
	require.Equal(t, "bar", received[0].Props["foo"])
}

func TestSender_FlushNil(t *testing.T) {
	var s *Sender
	s.Flush() // should not panic
}

func TestSender_FlushNoEvents(t *testing.T) {
	collector := NewCollector()
	s := NewSender(collector, http.DefaultTransport)
	s.Flush()
}
