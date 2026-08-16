package workspace

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestFailureImplementsError(t *testing.T) {
	var err error = &Failure{Op: "prepare", Stage: "validate", Kind: KindInvalidInput, Detail: "bad"}
	if err == nil {
		t.Fatal("Failure should satisfy error")
	}
	msg := err.Error()
	for _, want := range []string{"prepare", "validate", string(KindInvalidInput), "bad"} {
		if !contains(msg, want) {
			t.Errorf("Error() = %q; want it to contain %q", msg, want)
		}
	}
}

func TestFailureErrorWithoutDetail(t *testing.T) {
	f := &Failure{Op: "cleanup", Stage: "remove", Kind: KindExternal}
	msg := f.Error()
	for _, want := range []string{"cleanup", "remove", string(KindExternal)} {
		if !contains(msg, want) {
			t.Errorf("Error() = %q; want it to contain %q", msg, want)
		}
	}
}

func TestFailureUnwrapRoundTrips(t *testing.T) {
	sentinel := errors.New("root cause")
	f := &Failure{Op: "publish-head", Stage: "push", Kind: KindTimedOut, Err: sentinel}

	if got := f.Unwrap(); got != sentinel {
		t.Errorf("Unwrap() = %v; want %v", got, sentinel)
	}
	if !errors.Is(f, sentinel) {
		t.Errorf("errors.Is(f, sentinel) = false; want true")
	}
}

func TestFailureUnwrapNil(t *testing.T) {
	f := &Failure{Op: "inspect", Stage: "validate", Kind: KindInvalidState}
	if got := f.Unwrap(); got != nil {
		t.Errorf("Unwrap() with no cause = %v; want nil", got)
	}
}

func TestAsFailureMatchesWrappedMissesPlain(t *testing.T) {
	f := &Failure{Op: "prepare", Stage: "lock", Kind: KindInvalidState, Detail: "held"}
	wrapped := fmt.Errorf("context: %w", f)

	got, ok := AsFailure(wrapped)
	if !ok {
		t.Fatal("AsFailure(wrapped) ok = false; want true")
	}
	if got != f {
		t.Errorf("AsFailure(wrapped) = %v; want %v", got, f)
	}

	if _, ok := AsFailure(errors.New("plain")); ok {
		t.Error("AsFailure(plain) ok = true; want false")
	}
	if _, ok := AsFailure(nil); ok {
		t.Error("AsFailure(nil) ok = true; want false")
	}
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}
