package retry

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"syscall"
	"time"
)

// IsTransient returns true if the error is a transient network error that
// should be retried. It checks for connection resets, timeouts, EOF, and
// similar transport-level failures.
func IsTransient(err error) bool {
	if err == nil {
		return false
	}

	// DNS errors: only retry temporary failures (not NXDOMAIN)
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return dnsErr.IsTemporary
	}

	// Timeout errors (i/o timeout, deadline exceeded at transport level)
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	// Syscall-level connection errors (reset, refused)
	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}

	// Check for EOF (server closed connection mid-response)
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	// Fallback: some wrapped errors lose the typed information
	msg := err.Error()
	if strings.Contains(msg, "connection reset by peer") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "broken pipe") {
		return true
	}

	return false
}

// Backoff tracks consecutive transient failures and computes exponential
// backoff delays for polling loops.
type Backoff struct {
	ConsecutiveFailures int
	MaxFailures         int
	InitialInterval     time.Duration
	MaxInterval         time.Duration
}

// NewBackoff creates a Backoff with defaults suitable for polling loops:
// 5 max consecutive failures, 1s initial interval, 5s max interval.
func NewBackoff() *Backoff {
	return &Backoff{
		MaxFailures:     5,
		InitialInterval: 1 * time.Second,
		MaxInterval:     5 * time.Second,
	}
}

// Record records a transient failure and returns the duration to sleep before
// retrying. Returns an error if MaxFailures consecutive failures have been
// reached.
func (b *Backoff) Record() (time.Duration, error) {
	b.ConsecutiveFailures++
	if b.ConsecutiveFailures >= b.MaxFailures {
		return 0, fmt.Errorf("exceeded %d consecutive transient failures", b.MaxFailures)
	}

	interval := b.InitialInterval
	for i := 1; i < b.ConsecutiveFailures; i++ {
		interval *= 2
		if interval > b.MaxInterval {
			interval = b.MaxInterval
			break
		}
	}
	return interval, nil
}

// Reset resets the consecutive failure counter after a successful poll.
func (b *Backoff) Reset() {
	b.ConsecutiveFailures = 0
}
