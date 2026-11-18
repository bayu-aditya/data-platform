// Package domainerr defines the errors usecases return. They are transport
// agnostic: handlers translate them to Connect codes in one place
// (handler.ConnectError), so business code never imports connect.
//
// An Error carries two things: a Kind that decides the status code, and a
// message that is safe to show the client. An optional cause is kept for
// logging and errors.Is/As, but is never sent to the client.
package domainerr

import (
	"errors"
	"fmt"
)

// Kind classifies a domain error. Each kind maps to exactly one Connect code.
type Kind int

// The zero Kind is deliberately not exported: an error without a known kind
// is treated as Internal.
const (
	_ Kind = iota
	// NotFound: the addressed entity does not exist.
	NotFound
	// AlreadyExists: creating the entity would violate a uniqueness rule.
	AlreadyExists
	// InvalidArgument: the request itself is malformed, regardless of state.
	InvalidArgument
	// FailedPrecondition: the request is well formed but the current state
	// forbids it (e.g. deleting a row that is still referenced).
	FailedPrecondition
	// Unavailable: a dependency (database, data source) cannot be reached;
	// retrying later may succeed.
	Unavailable
	// DeadlineExceeded: the operation ran out of time.
	DeadlineExceeded
	// ResourceExhausted: a configured limit (e.g. max rows) was exceeded.
	ResourceExhausted
	// Internal: a bug or unexpected failure. The message is never shown to
	// the client.
	Internal
)

var kindNames = map[Kind]string{
	NotFound:           "not found",
	AlreadyExists:      "already exists",
	InvalidArgument:    "invalid argument",
	FailedPrecondition: "failed precondition",
	Unavailable:        "unavailable",
	DeadlineExceeded:   "deadline exceeded",
	ResourceExhausted:  "resource exhausted",
	Internal:           "internal",
}

func (k Kind) String() string {
	if name, ok := kindNames[k]; ok {
		return name
	}
	return fmt.Sprintf("Kind(%d)", int(k))
}

// Error is a classified, client-presentable error.
type Error struct {
	kind  Kind
	msg   string
	cause error
}

// New returns an error of the given kind. The formatted message is shown to
// the client, so it must not contain secrets or internal details.
func New(kind Kind, format string, args ...any) *Error {
	return &Error{kind: kind, msg: fmt.Sprintf(format, args...)}
}

// Wrap is New with an underlying cause. The cause is available to
// errors.Is/As and to logs, but is not part of the client-facing message.
func Wrap(cause error, kind Kind, format string, args ...any) *Error {
	return &Error{kind: kind, msg: fmt.Sprintf(format, args...), cause: cause}
}

// Kind reports the error's classification.
func (e *Error) Kind() Kind { return e.kind }

// Message is the client-safe message, without the cause.
func (e *Error) Message() string { return e.msg }

// Error includes the cause, for logs.
func (e *Error) Error() string {
	if e.cause == nil {
		return e.msg
	}
	return e.msg + ": " + e.cause.Error()
}

// Unwrap exposes the cause to errors.Is/As.
func (e *Error) Unwrap() error { return e.cause }

// KindOf returns the kind of the first *Error in err's chain, or Internal if
// there is none.
func KindOf(err error) Kind {
	if de, ok := errors.AsType[*Error](err); ok {
		return de.kind
	}
	return Internal
}

// Is reports whether err's chain contains an *Error of the given kind.
func Is(err error, kind Kind) bool {
	return err != nil && KindOf(err) == kind
}
