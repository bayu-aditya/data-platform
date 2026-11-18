package handler_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"connectrpc.com/connect"

	"github.com/bayu-aditya/data-platform/server/internal/domainerr"
	"github.com/bayu-aditya/data-platform/server/internal/handler"
)

func TestConnectErrorMapsEveryKind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		kind domainerr.Kind
		code connect.Code
	}{
		{domainerr.NotFound, connect.CodeNotFound},
		{domainerr.AlreadyExists, connect.CodeAlreadyExists},
		{domainerr.InvalidArgument, connect.CodeInvalidArgument},
		{domainerr.FailedPrecondition, connect.CodeFailedPrecondition},
		{domainerr.Unavailable, connect.CodeUnavailable},
		{domainerr.DeadlineExceeded, connect.CodeDeadlineExceeded},
		{domainerr.ResourceExhausted, connect.CodeResourceExhausted},
	}

	for _, tt := range tests {
		t.Run(tt.kind.String(), func(t *testing.T) {
			t.Parallel()

			err := handler.ConnectError(context.Background(), domainerr.New(tt.kind, "client-safe detail"))

			assertConnectError(t, err, tt.code, "client-safe detail")
		})
	}
}

func TestConnectErrorKeepsCauseOutOfMessage(t *testing.T) {
	t.Parallel()

	cause := errors.New("dial tcp 10.0.0.5:5432: connection refused")
	err := handler.ConnectError(context.Background(),
		fmt.Errorf("get environment: %w",
			domainerr.Wrap(cause, domainerr.Unavailable, "database unavailable")))

	assertConnectError(t, err, connect.CodeUnavailable, "database unavailable")
}

func TestConnectErrorHidesInternalDetails(t *testing.T) {
	t.Parallel()

	tests := map[string]error{
		"unknown error": errors.New(`pq: relation "secret_table" does not exist`),
		"internal kind": domainerr.New(domainerr.Internal, "invariant broken for row 42"),
	}

	for name, in := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := handler.ConnectError(context.Background(), in)

			assertConnectError(t, err, connect.CodeInternal, "internal error")
		})
	}
}

func TestConnectErrorNil(t *testing.T) {
	t.Parallel()

	if err := handler.ConnectError(context.Background(), nil); err != nil {
		t.Errorf("ConnectError(nil) = %v, want nil", err)
	}
}

func assertConnectError(t *testing.T, err error, code connect.Code, msg string) {
	t.Helper()

	ce, ok := errors.AsType[*connect.Error](err)
	if !ok {
		t.Fatalf("err = %#v, want *connect.Error", err)
	}
	if ce.Code() != code {
		t.Errorf("code = %v, want %v", ce.Code(), code)
	}
	if ce.Message() != msg {
		t.Errorf("message = %q, want %q", ce.Message(), msg)
	}
}
