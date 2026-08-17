package process

import (
	"errors"
	"fmt"
	"testing"
)

func TestFailureError(t *testing.T) {
	f := failf(FailInvalidInput, "validate-root", "root %q is not absolute", "x")
	if f.Class != FailInvalidInput || f.Stage != "validate-root" {
		t.Fatalf("class/stage: %+v", f)
	}
	want := `validate-root: root "x" is not absolute`
	if f.Error() != want {
		t.Fatalf("Error() = %q, want %q", f.Error(), want)
	}
}

func TestAsFailure(t *testing.T) {
	f := failf(FailBlocked, "stop-reprove", "ownership unprovable")
	wrapped := fmt.Errorf("outer: %w", f)
	got, ok := AsFailure(wrapped)
	if !ok || got.Class != FailBlocked {
		t.Fatalf("AsFailure(wrapped) = %v, %v", got, ok)
	}
	if _, ok := AsFailure(errors.New("plain")); ok {
		t.Fatalf("plain error must not classify")
	}
}

func TestNewServiceRejectsRelativeExecutable(t *testing.T) {
	if _, err := NewService("docket"); err == nil {
		t.Fatalf("relative executable path must be rejected")
	}
	svc, err := NewService("/bin/true")
	if err != nil || svc == nil {
		t.Fatalf("absolute path rejected: %v", err)
	}
	if svc.establishTimeout.Seconds() != 10 || svc.stopTermWait.Seconds() != 10 || svc.stopKillWait.Seconds() != 5 {
		t.Fatalf("production bounds wrong: %+v", svc)
	}
}
