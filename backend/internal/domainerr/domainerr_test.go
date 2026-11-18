package domainerr_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/bayu-aditya/data-platform/server/internal/domainerr"
)

func TestNewFormatsClientMessage(t *testing.T) {
	t.Parallel()

	err := domainerr.New(domainerr.NotFound, "environment %q not found", "staging")

	if got, want := err.Message(), `environment "staging" not found`; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
	if got, want := err.Error(), err.Message(); got != want {
		t.Errorf("error = %q, want %q", got, want)
	}
}

func TestWrapKeepsCauseOutOfMessage(t *testing.T) {
	t.Parallel()

	cause := errors.New("dial tcp 10.0.0.5:5432: connection refused")
	err := domainerr.Wrap(cause, domainerr.Unavailable, "metadata database unavailable")

	if got, want := err.Message(), "metadata database unavailable"; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
	if got, want := err.Error(), "metadata database unavailable: "+cause.Error(); got != want {
		t.Errorf("error = %q, want %q", got, want)
	}
	if !errors.Is(err, cause) {
		t.Error("errors.Is(err, cause) = false, want true")
	}
}

func TestKindOfFindsWrappedError(t *testing.T) {
	t.Parallel()

	inner := domainerr.New(domainerr.FailedPrecondition, "still referenced")
	wrapped := fmt.Errorf("delete environment: %w", inner)

	if got := domainerr.KindOf(wrapped); got != domainerr.FailedPrecondition {
		t.Errorf("KindOf = %v, want %v", got, domainerr.FailedPrecondition)
	}
	if !domainerr.Is(wrapped, domainerr.FailedPrecondition) {
		t.Error("Is(wrapped, FailedPrecondition) = false, want true")
	}
	if domainerr.Is(wrapped, domainerr.NotFound) {
		t.Error("Is(wrapped, NotFound) = true, want false")
	}
}

func TestKindOfUnknownErrorIsInternal(t *testing.T) {
	t.Parallel()

	if got := domainerr.KindOf(errors.New("boom")); got != domainerr.Internal {
		t.Errorf("KindOf = %v, want %v", got, domainerr.Internal)
	}
	if domainerr.Is(nil, domainerr.Internal) {
		t.Error("Is(nil, Internal) = true, want false")
	}
}
