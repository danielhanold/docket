package gitcli

import (
	"errors"
	"fmt"
)

// Operation names the adapter operation a Failure arose from.
type Operation string

// FailureKind is the typed category of a Failure.
type FailureKind string

const (
	KindInvalidRequest        FailureKind = "invalid-request"
	KindExecutableUnavailable FailureKind = "executable-unavailable"
	KindInvalidRepository     FailureKind = "invalid-repository"
	KindRemoteUnavailable     FailureKind = "remote-unavailable"
	KindRefUnavailable        FailureKind = "ref-unavailable"
	KindCommandFailed         FailureKind = "command-failed"
	KindUnexpectedObject      FailureKind = "unexpected-object"
	KindInvalidOutput         FailureKind = "invalid-output"
	KindCancelled             FailureKind = "cancelled"
	KindTimedOut              FailureKind = "timed-out"
)

// Failure is the adapter's typed error. Detail carries bounded, safe prose
// only — never environment values, remote URLs, credentials, or blob bytes.
type Failure struct {
	Operation Operation
	Kind      FailureKind
	ExitCode  int    // 0 when no process exit is involved
	Detail    string // bounded safe prose; never env/URL/credential/bytes
	Err       error  // wrapped cause, may be nil
}

// Error renders the operation, kind, and bounded detail.
func (f *Failure) Error() string {
	if f.Detail == "" {
		return fmt.Sprintf("gitcli %s: %s", f.Operation, f.Kind)
	}
	return fmt.Sprintf("gitcli %s: %s: %s", f.Operation, f.Kind, f.Detail)
}

// Unwrap exposes the wrapped cause for errors.Is/As.
func (f *Failure) Unwrap() error { return f.Err }

// newFailure constructs a *Failure with the given operation, kind, bounded
// detail, and optional wrapped cause.
func newFailure(op Operation, kind FailureKind, detail string, err error) *Failure {
	return &Failure{Operation: op, Kind: kind, Detail: detail, Err: err}
}

// AsFailure is an errors.As convenience recovering a *Failure from an error.
func AsFailure(err error) (*Failure, bool) {
	var f *Failure
	if errors.As(err, &f) {
		return f, true
	}
	return nil, false
}
