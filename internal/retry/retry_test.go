package retry

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestIsTransient(t *testing.T) {
	t.Run("nil error is not transient", func(t *testing.T) {
		require.False(t, IsTransient(nil))
	})

	t.Run("generic error is not transient", func(t *testing.T) {
		require.False(t, IsTransient(errors.New("something went wrong")))
	})

	t.Run("EOF is transient", func(t *testing.T) {
		require.True(t, IsTransient(io.EOF))
	})

	t.Run("unexpected EOF is transient", func(t *testing.T) {
		require.True(t, IsTransient(io.ErrUnexpectedEOF))
	})

	t.Run("wrapped EOF is transient", func(t *testing.T) {
		require.True(t, IsTransient(fmt.Errorf("HTTP request failed: %w", io.EOF)))
	})

	t.Run("connection reset string is transient", func(t *testing.T) {
		require.True(t, IsTransient(errors.New("read tcp: connection reset by peer")))
	})

	t.Run("connection refused string is transient", func(t *testing.T) {
		require.True(t, IsTransient(errors.New("dial tcp: connection refused")))
	})

	t.Run("broken pipe string is transient", func(t *testing.T) {
		require.True(t, IsTransient(errors.New("write: broken pipe")))
	})

	t.Run("net.OpError with connection reset is transient", func(t *testing.T) {
		err := &net.OpError{
			Op:  "read",
			Net: "tcp",
			Err: &os.SyscallError{Syscall: "read", Err: syscall.ECONNRESET},
		}
		require.True(t, IsTransient(err))
	})

	t.Run("net.OpError with connection refused is transient", func(t *testing.T) {
		err := &net.OpError{
			Op:  "dial",
			Net: "tcp",
			Err: &os.SyscallError{Syscall: "connect", Err: syscall.ECONNREFUSED},
		}
		require.True(t, IsTransient(err))
	})

	t.Run("timeout error is transient", func(t *testing.T) {
		err := &net.OpError{
			Op:  "read",
			Net: "tcp",
			Err: &timeoutError{},
		}
		require.True(t, IsTransient(err))
	})

	t.Run("wrapped net error is transient", func(t *testing.T) {
		inner := &net.OpError{
			Op:  "read",
			Net: "tcp",
			Err: &os.SyscallError{Syscall: "read", Err: syscall.ECONNRESET},
		}
		require.True(t, IsTransient(fmt.Errorf("unable to get run status: %w", inner)))
	})

	t.Run("NXDOMAIN DNS error is not transient", func(t *testing.T) {
		err := &net.DNSError{
			Err:        "no such host",
			Name:       "nonexistent.example.com",
			IsNotFound: true,
		}
		require.False(t, IsTransient(err))
	})

	t.Run("temporary DNS error is transient", func(t *testing.T) {
		err := &net.DNSError{
			Err:         "server misbehaving",
			Name:        "example.com",
			IsTemporary: true,
		}
		require.True(t, IsTransient(err))
	})
}

type timeoutError struct{}

func (e *timeoutError) Error() string   { return "i/o timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return true }

func TestBackoff(t *testing.T) {
	t.Run("exponential backoff with cap", func(t *testing.T) {
		b := &Backoff{
			MaxFailures:     5,
			InitialInterval: 100 * time.Millisecond,
			MaxInterval:     500 * time.Millisecond,
		}

		d1, err := b.Record()
		require.NoError(t, err)
		require.Equal(t, 100*time.Millisecond, d1)

		d2, err := b.Record()
		require.NoError(t, err)
		require.Equal(t, 200*time.Millisecond, d2)

		d3, err := b.Record()
		require.NoError(t, err)
		require.Equal(t, 400*time.Millisecond, d3)

		d4, err := b.Record()
		require.NoError(t, err)
		require.Equal(t, 500*time.Millisecond, d4) // capped
	})

	t.Run("exhaustion after max failures", func(t *testing.T) {
		b := &Backoff{
			MaxFailures:     3,
			InitialInterval: time.Millisecond,
			MaxInterval:     time.Millisecond,
		}

		_, err := b.Record()
		require.NoError(t, err)
		_, err = b.Record()
		require.NoError(t, err)
		_, err = b.Record()
		require.Error(t, err)
		require.Contains(t, err.Error(), "3 consecutive transient failures")
	})

	t.Run("reset clears consecutive failures", func(t *testing.T) {
		b := &Backoff{
			MaxFailures:     2,
			InitialInterval: time.Millisecond,
			MaxInterval:     time.Millisecond,
		}

		_, err := b.Record()
		require.NoError(t, err)

		b.Reset()
		require.Equal(t, 0, b.ConsecutiveFailures)

		// Should be able to record again without exhaustion
		_, err = b.Record()
		require.NoError(t, err)
	})

	t.Run("NewBackoff returns sensible defaults", func(t *testing.T) {
		b := NewBackoff()
		require.Equal(t, 5, b.MaxFailures)
		require.Equal(t, 1*time.Second, b.InitialInterval)
		require.Equal(t, 5*time.Second, b.MaxInterval)
		require.Equal(t, 0, b.ConsecutiveFailures)
	})
}
