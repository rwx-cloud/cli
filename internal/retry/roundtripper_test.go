package retry

import (
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	internalerrors "github.com/rwx-cloud/rwx/internal/errors"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newTestRoundTripper(inner roundTripFunc) *RoundTripper {
	return &RoundTripper{
		Inner: inner,
		Sleep: func(time.Duration) {},
	}
}

func TestRoundTripper(t *testing.T) {
	t.Run("succeeds on first try", func(t *testing.T) {
		callCount := 0
		rt := newTestRoundTripper(func(req *http.Request) (*http.Response, error) {
			callCount++
			return &http.Response{StatusCode: 200}, nil
		})

		req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
		resp, err := rt.RoundTrip(req)

		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)
		require.Equal(t, 1, callCount)
	})

	t.Run("retries transient errors and succeeds", func(t *testing.T) {
		callCount := 0
		rt := newTestRoundTripper(func(req *http.Request) (*http.Response, error) {
			callCount++
			if callCount <= 2 {
				return nil, io.EOF
			}
			return &http.Response{StatusCode: 200}, nil
		})

		req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
		resp, err := rt.RoundTrip(req)

		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)
		require.Equal(t, 3, callCount)
	})

	t.Run("gives up after max consecutive failures", func(t *testing.T) {
		callCount := 0
		rt := newTestRoundTripper(func(req *http.Request) (*http.Response, error) {
			callCount++
			return nil, io.EOF
		})

		req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
		_, err := rt.RoundTrip(req)

		require.Error(t, err)
		require.ErrorIs(t, err, internalerrors.ErrNetworkTransient)
		require.Equal(t, 5, callCount)
	})

	t.Run("does not retry non-transient errors", func(t *testing.T) {
		callCount := 0
		rt := newTestRoundTripper(func(req *http.Request) (*http.Response, error) {
			callCount++
			return nil, errors.New("authentication failed")
		})

		req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
		_, err := rt.RoundTrip(req)

		require.Error(t, err)
		require.Equal(t, 1, callCount)
	})

	t.Run("does not retry POST requests", func(t *testing.T) {
		callCount := 0
		rt := newTestRoundTripper(func(req *http.Request) (*http.Response, error) {
			callCount++
			return nil, io.EOF
		})

		req, _ := http.NewRequest(http.MethodPost, "http://example.com", nil)
		_, err := rt.RoundTrip(req)

		require.Error(t, err)
		require.Equal(t, 1, callCount)
	})

	t.Run("records sleep durations with exponential backoff", func(t *testing.T) {
		var sleepDurations []time.Duration
		callCount := 0
		rt := &RoundTripper{
			Inner: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				callCount++
				if callCount <= 3 {
					return nil, io.EOF
				}
				return &http.Response{StatusCode: 200}, nil
			}),
			Sleep: func(d time.Duration) {
				sleepDurations = append(sleepDurations, d)
			},
		}

		req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
		_, err := rt.RoundTrip(req)

		require.NoError(t, err)
		require.Equal(t, 4, callCount)
		require.Len(t, sleepDurations, 3)
		require.Equal(t, 1*time.Second, sleepDurations[0])
		require.Equal(t, 2*time.Second, sleepDurations[1])
		require.Equal(t, 4*time.Second, sleepDurations[2])
	})
}
