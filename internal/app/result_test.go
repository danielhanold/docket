package app

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestResultTaxonomySpellings(t *testing.T) {
	want := []string{
		"applied", "no-op", "contended", "invalid-input", "invalid-state",
		"blocked", "unsupported-config", "gate-failed", "external-failed",
		"interrupted", "internal-error",
	}
	if len(AllResults) != len(want) {
		t.Fatalf("taxonomy has %d results, want %d", len(AllResults), len(want))
	}
	for i, w := range want {
		if string(AllResults[i]) != w {
			t.Fatalf("AllResults[%d] = %q, want %q", i, AllResults[i], w)
		}
	}
}

func TestExitCodeMapping(t *testing.T) {
	for _, r := range AllResults {
		got := ExitCode(r)
		var want int
		switch r {
		case ResultApplied, ResultNoOp:
			want = 0
		case ResultInvalidInput:
			want = 2
		default:
			want = 1
		}
		if got != want {
			t.Fatalf("ExitCode(%q) = %d, want %d", r, got, want)
		}
	}
}

func TestEnvelopeFieldNamesAndOrder(t *testing.T) {
	b, err := json.Marshal(NewEnvelope("op.name", ResultApplied))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"protocol_version":1,"operation":"op.name","result":"applied"}`
	if string(b) != want {
		t.Fatalf("envelope encoding = %s, want %s", b, want)
	}
}

func TestEnvelopeFailureMarshalsOnlyWhenPresent(t *testing.T) {
	env := NewEnvelope("change.claim", ResultApplied)
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte(`"failure"`)) {
		t.Errorf("failure must be absent on a non-failed envelope: %s", b)
	}

	env.Failure = &FailureStatus{Stage: "verify-delta", Kind: "invalid-state", Detail: "x"}
	b, err = json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte(`"failure":{"stage":"verify-delta","kind":"invalid-state","detail":"x"}`)) {
		t.Errorf("failure field missing or misshapen: %s", b)
	}
}
