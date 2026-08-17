package process

import (
	"errors"
	"fmt"
)

// FailureClass mirrors the app-result classes an operation failure maps to.
type FailureClass string

const (
	FailInvalidInput FailureClass = "invalid-input"
	FailInvalidState FailureClass = "invalid-state"
	FailBlocked      FailureClass = "blocked"
	FailExternal     FailureClass = "external-failed"
)

// Failure is a typed operation failure with a stable stage and bounded safe
// reason. It never carries argv, environment values, or child output.
type Failure struct {
	Class  FailureClass
	Stage  string
	Reason string
}

func (f *Failure) Error() string { return f.Stage + ": " + f.Reason }

func failf(class FailureClass, stage, format string, a ...any) *Failure {
	return &Failure{Class: class, Stage: stage, Reason: fmt.Sprintf(format, a...)}
}

// AsFailure unwraps err to a *Failure when one is in the chain.
func AsFailure(err error) (*Failure, bool) {
	var f *Failure
	if errors.As(err, &f) {
		return f, true
	}
	return nil, false
}
