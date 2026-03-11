package errors_test

import (
	stderrors "errors"
	"fmt"
	"strings"
	"testing"

	"github.com/rwx-cloud/rwx/internal/errors"
)

func TestWrapSentinel(t *testing.T) {
	t.Run("wraps error with sentinel", func(t *testing.T) {
		inner := fmt.Errorf("connection refused")
		wrapped := errors.WrapSentinel(inner, errors.ErrSSH)

		if !stderrors.Is(wrapped, errors.ErrSSH) {
			t.Fatal("expected errors.Is(wrapped, ErrSSH) to be true")
		}
		if !stderrors.Is(wrapped, inner) {
			t.Fatal("expected errors.Is(wrapped, inner) to be true")
		}
	})

	t.Run("returns nil for nil error", func(t *testing.T) {
		result := errors.WrapSentinel(nil, errors.ErrSSH)
		if result != nil {
			t.Fatalf("expected nil, got %v", result)
		}
	})

	t.Run("returns error unchanged if already matches sentinel", func(t *testing.T) {
		wrapped := errors.WrapSentinel(errors.ErrSSH, errors.ErrSSH)
		if wrapped != errors.ErrSSH {
			t.Fatal("expected same error returned when already matching sentinel")
		}
	})

	t.Run("error message shows only inner error", func(t *testing.T) {
		inner := fmt.Errorf("timeout after 30s")
		wrapped := errors.WrapSentinel(inner, errors.ErrSSH)

		msg := wrapped.Error()
		if msg != "timeout after 30s" {
			t.Fatalf("unexpected error message: %s", msg)
		}
	})

	t.Run("works with ErrPatch", func(t *testing.T) {
		inner := fmt.Errorf("git apply failed")
		wrapped := errors.WrapSentinel(inner, errors.ErrPatch)

		if !stderrors.Is(wrapped, errors.ErrPatch) {
			t.Fatal("expected errors.Is(wrapped, ErrPatch) to be true")
		}
	})

	t.Run("verbose formatting includes stack trace", func(t *testing.T) {
		inner := fmt.Errorf("connection refused")
		wrapped := errors.WrapSentinel(inner, errors.ErrSSH)

		verbose := fmt.Sprintf("%+v", wrapped)
		if !strings.Contains(verbose, "connection refused") {
			t.Fatalf("expected error message in verbose output, got: %s", verbose)
		}
		if !strings.Contains(verbose, "errors_test.go") {
			t.Fatalf("expected stack trace with test file name, got: %s", verbose)
		}
	})

	t.Run("non-verbose formatting has no stack trace", func(t *testing.T) {
		inner := fmt.Errorf("connection refused")
		wrapped := errors.WrapSentinel(inner, errors.ErrSSH)

		simple := fmt.Sprintf("%v", wrapped)
		if simple != "connection refused" {
			t.Fatalf("expected plain error message, got: %s", simple)
		}
		if strings.Contains(simple, ".go") {
			t.Fatalf("unexpected stack trace in non-verbose output: %s", simple)
		}
	})

	t.Run("chaining with fmt.Errorf preserves sentinel", func(t *testing.T) {
		inner := fmt.Errorf("connection refused")
		wrapped := errors.WrapSentinel(inner, errors.ErrSSH)
		outer := fmt.Errorf("sandbox connect failed: %w", wrapped)

		if !stderrors.Is(outer, errors.ErrSSH) {
			t.Fatal("expected sentinel to be preserved through fmt.Errorf wrapping")
		}
		if !stderrors.Is(outer, inner) {
			t.Fatal("expected inner error to be preserved through fmt.Errorf wrapping")
		}
	})
}
