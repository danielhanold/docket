package document

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestErrorStringCarriesKindNameAndPosition(t *testing.T) {
	e := &Error{Kind: KindMalformedMarker, Name: "artifacts", Offset: 120, Line: 9, Column: 1,
		Msg: "start marker has no matching end"}
	got := e.Error()
	for _, want := range []string{"malformed-marker", "artifacts", "line 9"} {
		if !strings.Contains(got, want) {
			t.Fatalf("Error() = %q, want it to contain %q", got, want)
		}
	}
}

func TestIsKindMatchesWrappedErrors(t *testing.T) {
	e := &Error{Kind: KindInvalidYAML, Offset: -1, Msg: "boom"}
	wrapped := fmt.Errorf("outer: %w", e)
	if !IsKind(wrapped, KindInvalidYAML) {
		t.Fatal("IsKind should see through wrapping")
	}
	if IsKind(wrapped, KindInvalidValue) {
		t.Fatal("IsKind must not match a different kind")
	}
	if IsKind(errors.New("plain"), KindInvalidYAML) {
		t.Fatal("IsKind must not match a non-document error")
	}
}
