package gitcli

import (
	"errors"
	"strings"
	"testing"
)

func TestFailureErrorAndUnwrap(t *testing.T) {
	cause := errors.New("boom")
	f := newFailure("fetch-branch", KindCommandFailed, "git exited 128", cause)
	if !errors.Is(f, cause) {
		t.Fatal("Unwrap chain broken")
	}
	var got *Failure
	if !errors.As(f, &got) || got.Kind != KindCommandFailed {
		t.Fatal("errors.As failed")
	}
	for _, s := range []string{"fetch-branch", "command-failed", "git exited 128"} {
		if !strings.Contains(f.Error(), s) {
			t.Errorf("Error() missing %q", s)
		}
	}
}

func TestAsFailure(t *testing.T) {
	f := newFailure("discover", KindInvalidRepository, "not a repo", nil)
	got, ok := AsFailure(f)
	if !ok || got.Kind != KindInvalidRepository {
		t.Fatalf("AsFailure did not recover the Failure: %v %v", got, ok)
	}
	if _, ok := AsFailure(errors.New("plain")); ok {
		t.Error("AsFailure returned ok for a non-Failure error")
	}
	if _, ok := AsFailure(nil); ok {
		t.Error("AsFailure returned ok for nil")
	}
}
