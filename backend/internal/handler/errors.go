// Package handler holds the Connect RPC handlers. Handlers are thin: they
// translate protobuf messages to usecase calls and back, depend only on
// usecase interfaces, and turn every error into a Connect error with
// ConnectError.
package handler

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"

	"github.com/bayu-aditya/data-platform/server/internal/domainerr"
)

// internalMessage replaces the message of every Internal or unclassified
// error, so database messages, stack details or secrets never reach clients.
const internalMessage = "internal error"

// kindCodes maps every client-presentable kind. domainerr.Internal is
// deliberately absent: it takes the generic-message path below.
var kindCodes = map[domainerr.Kind]connect.Code{
	domainerr.NotFound:           connect.CodeNotFound,
	domainerr.AlreadyExists:      connect.CodeAlreadyExists,
	domainerr.InvalidArgument:    connect.CodeInvalidArgument,
	domainerr.FailedPrecondition: connect.CodeFailedPrecondition,
	domainerr.Unavailable:        connect.CodeUnavailable,
	domainerr.DeadlineExceeded:   connect.CodeDeadlineExceeded,
	domainerr.ResourceExhausted:  connect.CodeResourceExhausted,
}

// ConnectError converts an error returned by a usecase into the error a
// handler returns to the client. It is the only place domain errors become
// Connect codes.
//
// Classified errors keep their client-safe message. Internal and
// unclassified errors become CodeInternal with a generic message; the
// original error is logged with the default slog logger instead. A nil err
// returns nil.
func ConnectError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}

	if de, ok := errors.AsType[*domainerr.Error](err); ok {
		if code, ok := kindCodes[de.Kind()]; ok {
			return connect.NewError(code, errors.New(de.Message()))
		}
	}

	slog.ErrorContext(ctx, "internal error", slog.Any("error", err))
	return connect.NewError(connect.CodeInternal, errors.New(internalMessage))
}
