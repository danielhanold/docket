package suiterunner

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// writeStat drops a raw stat file body into dir under name; the tests control
// the exact bytes so malformed / wrong-target / temp-leftover records can be
// synthesized directly.
func writeStat(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("seed %s: %v", name, err)
	}
}

func validStat(target string, rc, secs, ok, notok int) string {
	return fmt.Sprintf(`{"schema":1,"target":%q,"rc":%d,"seconds":%d,"ok":%d,"notok":%d}`, target, rc, secs, ok, notok)
}

func tgt(base string, ceil int, mode Mode) Target {
	return Target{Path: "tests/" + base, Base: base, Ceiling: ceil, Mode: mode}
}

func findOutcome(t *testing.T, outcomes []TargetOutcome, base string) TargetOutcome {
	t.Helper()
	for _, o := range outcomes {
		if o.Target.Base == base {
			return o
		}
	}
	t.Fatalf("no outcome for %q in %d outcomes", base, len(outcomes))
	return TargetOutcome{}
}

// TestValidateCompletenessOverWholeSet — a present, a malformed, and an absent
// result all classify in a SINGLE call. Mutating ValidateResults to stop after
// the first failure, or to skip a scheduled target, drops one of the three.
func TestValidateCompletenessOverWholeSet(t *testing.T) {
	statDir := t.TempDir()
	writeStat(t, statDir, "test_pass.json", validStat("test_pass.sh", 0, 3, 2, 0))
	writeStat(t, statDir, "test_bad.json", `{"schema":1,"target":"test_bad.sh"`) // truncated
	// test_gone: no file at all.

	scheduled := []Target{
		tgt("test_pass.sh", 60, ModeParallel),
		tgt("test_bad.sh", 60, ModeParallel),
		tgt("test_gone.sh", 60, ModeParallel),
	}
	outcomes, unknown := ValidateResults(scheduled, nil, statDir, nil)
	if len(unknown) != 0 {
		t.Fatalf("unknown = %v, want none", unknown)
	}
	if len(outcomes) != 3 {
		t.Fatalf("got %d outcomes, want 3 (whole-set validation)", len(outcomes))
	}
	if k := findOutcome(t, outcomes, "test_pass.sh").Kind; k != OutcomePassed {
		t.Fatalf("test_pass kind = %v, want Passed", k)
	}
	if k := findOutcome(t, outcomes, "test_bad.sh").Kind; k != OutcomeInvalidResult {
		t.Fatalf("test_bad kind = %v, want InvalidResult", k)
	}
	if k := findOutcome(t, outcomes, "test_gone.sh").Kind; k != OutcomeNoResult {
		t.Fatalf("test_gone kind = %v, want NoResult", k)
	}
}

// TestValidateWrongTargetIdentity — a file whose target field names a different
// test than its filename is a mis-published record, not a pass.
func TestValidateWrongTargetIdentity(t *testing.T) {
	statDir := t.TempDir()
	writeStat(t, statDir, "test_a.json", validStat("test_b.sh", 0, 1, 1, 0))

	outcomes, _ := ValidateResults([]Target{tgt("test_a.sh", 60, ModeParallel)}, nil, statDir, nil)
	o := findOutcome(t, outcomes, "test_a.sh")
	if o.Kind != OutcomeInvalidResult {
		t.Fatalf("kind = %v, want InvalidResult", o.Kind)
	}
	if !contains(o.Detail, "wrong-target") {
		t.Fatalf("detail = %q, want wrong-target substring", o.Detail)
	}
}

// TestValidateUnknownAndUnscheduled — a stat file for a target that was never
// scheduled is surfaced (never silently ignored), and it certifies nothing:
// ExitCode with unknown>0 is 3.
func TestValidateUnknownAndUnscheduled(t *testing.T) {
	statDir := t.TempDir()
	writeStat(t, statDir, "test_a.json", validStat("test_a.sh", 0, 1, 1, 0))
	writeStat(t, statDir, "test_ghost.json", validStat("test_ghost.sh", 0, 1, 1, 0))

	outcomes, unknown := ValidateResults([]Target{tgt("test_a.sh", 60, ModeParallel)}, nil, statDir, nil)
	if findOutcome(t, outcomes, "test_a.sh").Kind != OutcomePassed {
		t.Fatalf("test_a should pass")
	}
	if len(unknown) != 1 || !contains(unknown[0], "test_ghost") {
		t.Fatalf("unknown = %v, want [test_ghost...]", unknown)
	}
	tally := RenderReport(&bytes.Buffer{}, outcomes, unknown, 1, false, false, "")
	if got := ExitCode(tally, len(unknown), false); got != 3 {
		t.Fatalf("ExitCode with unknown>0 = %d, want 3", got)
	}
}

// TestValidateDuplicatePublication — a leftover .stat-* temp beside a valid
// record means an atomic publish did not complete; the record's durability
// cannot be shown, so the target is invalid, not passed.
func TestValidateDuplicatePublication(t *testing.T) {
	statDir := t.TempDir()
	writeStat(t, statDir, "test_p.json", validStat("test_p.sh", 0, 1, 1, 0))
	writeStat(t, statDir, ".stat-xyz", `{"schema":1,"target":"test_p.sh"`) // orphaned temp

	outcomes, _ := ValidateResults([]Target{tgt("test_p.sh", 60, ModeParallel)}, nil, statDir, nil)
	o := findOutcome(t, outcomes, "test_p.sh")
	if o.Kind != OutcomeInvalidResult {
		t.Fatalf("kind = %v, want InvalidResult", o.Kind)
	}
	if !contains(o.Detail, "publication not shown durable") {
		t.Fatalf("detail = %q, want publication-not-durable substring", o.Detail)
	}
}

// TestValidateObservationConflict — a nominal (rc=0) durable file cannot conceal
// a runner-observed execution failure (rc=1); the disagreement is invalid.
func TestValidateObservationConflict(t *testing.T) {
	statDir := t.TempDir()
	writeStat(t, statDir, "test_c.json", validStat("test_c.sh", 0, 1, 1, 0))
	observed := map[string]Result{"test_c.sh": {Schema: 1, Target: "test_c.sh", RC: 1}}

	outcomes, _ := ValidateResults([]Target{tgt("test_c.sh", 60, ModeParallel)}, observed, statDir, nil)
	o := findOutcome(t, outcomes, "test_c.sh")
	if o.Kind != OutcomeInvalidResult {
		t.Fatalf("kind = %v, want InvalidResult", o.Kind)
	}
	if !contains(o.Detail, "conflicts with runner-observed execution") {
		t.Fatalf("detail = %q, want observation-conflict substring", o.Detail)
	}
}

// TestReportOrderIsCompletionIndependent — the same outcomes, fed in two shuffled
// orders, render byte-identical. Mutating RenderReport to emit in arrival order
// reddens this.
func TestReportOrderIsCompletionIndependent(t *testing.T) {
	mk := func(base string, kind OutcomeKind, rc, secs int) TargetOutcome {
		return TargetOutcome{
			Target: tgt(base, 60, ModeParallel),
			Kind:   kind,
			Result: Result{Schema: 1, Target: base, RC: rc, Seconds: secs, OK: 1},
		}
	}
	a := mk("test_a.sh", OutcomePassed, 0, 2)
	b := mk("test_b.sh", OutcomeFailed, 1, 4)
	c := TargetOutcome{Target: tgt("test_c.sh", 60, ModeParallel), Kind: OutcomeNoResult}

	var buf1, buf2 bytes.Buffer
	RenderReport(&buf1, []TargetOutcome{a, b, c}, nil, 9, false, false, "")
	RenderReport(&buf2, []TargetOutcome{c, b, a}, nil, 9, false, false, "")
	if buf1.String() != buf2.String() {
		t.Fatalf("report differs by input order:\n--- order1 ---\n%s\n--- order2 ---\n%s", buf1.String(), buf2.String())
	}
}

// TestReportPreservesAllFailures — every failing category survives into the
// report and the tallies; no failure is collapsed or dropped.
func TestReportPreservesAllFailures(t *testing.T) {
	mkPassFail := func(base string, rc int) TargetOutcome {
		k := OutcomePassed
		if rc != 0 {
			k = OutcomeFailed
		}
		return TargetOutcome{Target: tgt(base, 60, ModeParallel), Kind: k,
			Result: Result{Schema: 1, Target: base, RC: rc, Seconds: 1, OK: 2, NotOK: rc}}
	}
	outcomes := []TargetOutcome{
		mkPassFail("test_f1.sh", 1),
		mkPassFail("test_f2.sh", 1),
		{Target: tgt("test_nr.sh", 60, ModeParallel), Kind: OutcomeNoResult},
		{Target: tgt("test_iv.sh", 60, ModeParallel), Kind: OutcomeInvalidResult, Detail: "malformed"},
	}
	var buf bytes.Buffer
	tally := RenderReport(&buf, outcomes, nil, 5, false, false, "")
	if tally.Failed != 2 || tally.NoResult != 1 || tally.Invalid != 1 {
		t.Fatalf("tally = %+v, want Failed=2 NoResult=1 Invalid=1", tally)
	}
	if tally.Files != 2 || tally.Passed != 0 {
		t.Fatalf("tally = %+v, want Files=2 Passed=0", tally)
	}
	out := buf.String()
	for _, want := range []string{
		"SUITE files=2 passed=0 failed=2",
		"FAILED: test_f1 test_f2",
		"NO RESULT: test_nr",
	} {
		if !contains(out, want) {
			t.Fatalf("report missing %q:\n%s", want, out)
		}
	}
}

func TestExitPrecedence(t *testing.T) {
	cases := []struct {
		name        string
		tally       Tally
		unknown     int
		strictArmed bool
		want        int
	}{
		{"failed beats noresult", Tally{Failed: 1, NoResult: 1}, 0, false, 1},
		{"failed beats strict", Tally{Failed: 1}, 0, true, 1},
		{"noresult is 3", Tally{NoResult: 1}, 0, false, 3},
		{"invalid is 3", Tally{Invalid: 1}, 0, false, 3},
		{"unknown is 3", Tally{}, 1, false, 3},
		{"noresult beats strict", Tally{NoResult: 1}, 0, true, 3},
		{"strict only when clean", Tally{}, 0, true, 4},
		{"advisory breach clean", Tally{OverDirect: 1}, 0, false, 0},
		{"all clean", Tally{Passed: 3, Files: 3}, 0, false, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ExitCode(c.tally, c.unknown, c.strictArmed); got != c.want {
				t.Fatalf("ExitCode(%+v, %d, %v) = %d, want %d", c.tally, c.unknown, c.strictArmed, got, c.want)
			}
		})
	}
}

// TestAdvisoryBreachExitsZeroWithLoudReport — an authoritative direct breach on
// an otherwise-green run is LOUD in the report but exits 0 (ADR-0074
// completed-with-observation). The remedy leads with sharding and never teaches
// raising the ceiling. Mutating either half — silencing the report, or exiting
// non-zero — reddens.
func TestAdvisoryBreachExitsZeroWithLoudReport(t *testing.T) {
	o := TargetOutcome{
		Target:     tgt("test_slow.sh", 60, ModeSerial),
		Kind:       OutcomePassed,
		Result:     Result{Schema: 1, Target: "test_slow.sh", RC: 0, Seconds: 100, OK: 1},
		OverDirect: true,
	}
	var buf bytes.Buffer
	tally := RenderReport(&buf, []TargetOutcome{o}, nil, 100, false, false, "")
	out := buf.String()
	for _, want := range []string{
		"OVER BUDGET:",
		"OVER BUDGET (ceiling 60s)",
		"Advisory: the tests all passed, so this run does not fail on the breach (exit 0).",
		"Remedy: shard this file or extend an existing shard so each part stays under its ceiling.",
	} {
		if !contains(out, want) {
			t.Fatalf("report missing %q:\n%s", want, out)
		}
	}
	if got := ExitCode(tally, 0, false); got != 0 {
		t.Fatalf("advisory breach ExitCode = %d, want 0", got)
	}
}

// TestStrictBreachRendersStrictNote — under DOCKET_RUNTESTS_STRICT=1 (strict=true) an
// authoritative direct breach on an otherwise-green run GATES the run (exit 4). The
// OVER BUDGET note must then render the oracle's strict arm byte-for-byte, and must NOT
// hand the reader the advisory "the tests all passed … (exit 0)" line — the exact
// ADR-0074 harm the note-selection guards. This pins the fourth arm the earlier
// three-arm switch could not reach; the non-strict case is covered by
// TestAdvisoryBreachExitsZeroWithLoudReport.
func TestStrictBreachRendersStrictNote(t *testing.T) {
	o := TargetOutcome{
		Target:     tgt("test_slow.sh", 60, ModeSerial),
		Kind:       OutcomePassed,
		Result:     Result{Schema: 1, Target: "test_slow.sh", RC: 0, Seconds: 100, OK: 1},
		OverDirect: true,
	}
	var buf bytes.Buffer
	tally := RenderReport(&buf, []TargetOutcome{o}, nil, 100, false, true, "")
	out := buf.String()
	if !contains(out, "Strict: --strict-budget was given, so this breach fails the run (exit 4). The tests themselves passed.") {
		t.Fatalf("strict breach missing the oracle's strict note:\n%s", out)
	}
	if contains(out, "Advisory: the tests all passed") {
		t.Fatalf("strict breach still printed the advisory exit-0 line (ADR-0074 harm):\n%s", out)
	}
	// A strict-armed direct crossing exits 4 (run.go sets strictArmed on this path).
	if got := ExitCode(tally, 0, true); got != 4 {
		t.Fatalf("strict breach ExitCode = %d, want 4", got)
	}
}

func contains(haystack, needle string) bool {
	return bytes.Contains([]byte(haystack), []byte(needle))
}
