package retry

import (
	"fmt"
	"net/http"
	"time"

	"github.com/rwx-cloud/rwx/internal/errors"
)

// RoundTripper wraps an http.RoundTripper and automatically retries requests
// that fail with transient network errors, using exponential backoff.
type RoundTripper struct {
	Inner http.RoundTripper
	Sleep func(time.Duration)
}

// NewRoundTripper creates a RoundTripper that retries transient errors.
func NewRoundTripper(inner http.RoundTripper) *RoundTripper {
	return &RoundTripper{
		Inner: inner,
		Sleep: time.Sleep,
	}
}

func (rt *RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	backoff := NewBackoff()

	for {
		resp, err := rt.Inner.RoundTrip(req)
		if err == nil {
			return resp, nil
		}

		if req.Method != http.MethodGet || !IsTransient(err) {
			return nil, err
		}

		sleepDur, retryErr := backoff.Record()
		if retryErr != nil {
			return nil, errors.WrapSentinel(
				fmt.Errorf("request failed after %d consecutive network errors: %w", backoff.MaxFailures, err),
				errors.ErrNetworkTransient,
			)
		}

		rt.Sleep(sleepDur)
	}
}
