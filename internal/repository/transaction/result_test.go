package transaction

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestValidateReceipt(t *testing.T) {
	// A canonical compact object whose keys are sorted and whose values
	// round-trip through json.Marshal byte-for-byte.
	canonical := []byte(`{"a":1,"b":"x","c":true,"d":[1,2,3]}`)
	if err := validateReceipt(canonical); err != nil {
		t.Fatalf("validateReceipt(canonical) = %v, want nil", err)
	}

	// Boundary: an object exactly 4096 bytes is accepted.
	at4096 := []byte(`{"a":"` + strings.Repeat("x", 4088) + `"}`)
	if len(at4096) != 4096 {
		t.Fatalf("test setup: at4096 is %d bytes, want 4096", len(at4096))
	}
	if err := validateReceipt(at4096); err != nil {
		t.Fatalf("validateReceipt(4096-byte object) = %v, want nil", err)
	}

	// A compact canonical object whose string value carries literal HTML-active
	// bytes (<, >, &) is valid: the canonical-form check must be independent of
	// Go's default HTML escaping. json.Marshal would escape these to < etc.,
	// so a naive re-marshal comparison would wrongly reject this.
	htmlActive := []byte(`{"a":"<b>&</b>"}`)
	if err := validateReceipt(htmlActive); err != nil {
		t.Fatalf("validateReceipt(html-active string value) = %v, want nil", err)
	}

	cases := []struct {
		receipt []byte
		name    string
	}{
		{[]byte("not json"), "non-json"},
		{[]byte(`[1,2,3]`), "json-array"},
		{[]byte(`"a string"`), "json-string"},
		{[]byte(`{"a": 1}`), "insignificant-whitespace"},
		{[]byte(`{"b":"x","a":1}`), "unsorted-keys-remarshal-mismatch"},
		{[]byte(`{"a":"` + strings.Repeat("x", 4089) + `"}`), "4097-bytes"},
		{[]byte(""), "empty"},
		// A string value written with a \uXXXX escape for an HTML-active byte is
		// NOT canonical: the escaping-independent re-marshal emits the literal
		// byte, so this must still be rejected.
		{[]byte(`{"a":"\u003c"}`), "unicode-escaped-html-active-not-canonical"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := validateReceipt(c.receipt); err == nil {
				t.Fatalf("validateReceipt(%s) = nil, want error", c.name)
			}
		})
	}
}

func TestFailureImplementsError(t *testing.T) {
	var _ error = (*Failure)(nil)
	f := &Failure{Stage: "push", Kind: KindExternal, Detail: "boom"}
	if f.Error() == "" {
		t.Fatalf("Failure.Error() = empty, want non-empty")
	}
}

func TestFailureUnwrapRoundTripsThroughErrorsIs(t *testing.T) {
	sentinel := errors.New("underlying cause")
	f := &Failure{Stage: "commit", Kind: KindExternal, Detail: "wrap", Err: sentinel}
	if !errors.Is(f, sentinel) {
		t.Fatalf("errors.Is(f, sentinel) = false, want true")
	}
	if f.Unwrap() != sentinel {
		t.Fatalf("f.Unwrap() = %v, want sentinel", f.Unwrap())
	}
	// A failure with no cause unwraps to nil.
	g := &Failure{Stage: "plan", Kind: KindInvalidInput}
	if g.Unwrap() != nil {
		t.Fatalf("g.Unwrap() = %v, want nil", g.Unwrap())
	}
}

func TestAsFailure(t *testing.T) {
	f := &Failure{Stage: "verify-delta", Kind: KindInvalidState, Detail: "mismatch"}
	wrapped := fmt.Errorf("context: %w", f)
	got, ok := AsFailure(wrapped)
	if !ok {
		t.Fatalf("AsFailure(wrapped) ok = false, want true")
	}
	if got != f {
		t.Fatalf("AsFailure(wrapped) = %v, want the wrapped *Failure", got)
	}

	if _, ok := AsFailure(errors.New("plain")); ok {
		t.Fatalf("AsFailure(plain) ok = true, want false")
	}
	if _, ok := AsFailure(nil); ok {
		t.Fatalf("AsFailure(nil) ok = true, want false")
	}
}
