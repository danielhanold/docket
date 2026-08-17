package process

import (
	"regexp"
	"testing"
)

var hex32 = regexp.MustCompile(`^[0-9a-f]{32}$`)

func TestNewRunIdentityShape(t *testing.T) {
	id, tok, err := NewRunIdentity()
	if err != nil {
		t.Fatalf("NewRunIdentity: %v", err)
	}
	if !hex32.MatchString(id) || !hex32.MatchString(tok) {
		t.Fatalf("want 32 lowercase hex chars each, got id=%q token=%q", id, tok)
	}
	if id == tok {
		t.Fatalf("run id and token must be independent randomness")
	}
}

func TestNewRunIdentityUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 64; i++ {
		id, tok, err := NewRunIdentity()
		if err != nil {
			t.Fatalf("NewRunIdentity: %v", err)
		}
		if seen[id] || seen[tok] {
			t.Fatalf("collision at iteration %d", i)
		}
		seen[id], seen[tok] = true, true
	}
}
