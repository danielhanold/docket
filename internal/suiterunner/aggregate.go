// This file owns the pure aggregation core: it joins the COMPLETE scheduled set
// against the durable stat directory, classifies every target, renders the
// deterministic report byte-for-byte against the Bash oracle (scripts/run-tests.sh
// "report" block), and maps the aggregate to the oracle's exit contract
// (precedence 1 > 3 > 4 > 0). Nothing here runs a child process — validation and
// rendering are deterministic functions of the durable records plus the
// runner-observed truth, which is exactly what makes this the package's
// mutation-evidence anchor.
package suiterunner

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// OutcomeKind classifies one scheduled target after the run.
type OutcomeKind int

const (
	OutcomePassed        OutcomeKind = iota
	OutcomeFailed                    // rc != 0
	OutcomeNoResult                  // no durable result file
	OutcomeInvalidResult             // malformed / wrong-target / duplicate / observation conflict
	OutcomeInterrupted               // scheduled or launched, run was interrupted
)

// TargetOutcome is one scheduled target's classified result after the run.
type TargetOutcome struct {
	Target     Target
	Kind       OutcomeKind
	Result     Result // valid only for Passed/Failed
	Detail     string // human diagnostic for the invalid kinds
	OverDirect bool   // authoritative direct crossing (solo/serial-mode lane)
	Screened   bool   // contended parallel screening crossing (advisory; Task 5 owns it)
}

// ValidateResults joins the COMPLETE scheduled set against the stat dir. It
// validates EVERY scheduled target and never stops at the first failure
// (learning validate-the-whole-input-set-first), returning one outcome per
// scheduled target in scheduled order, plus the basenames of any stat file that
// matched no scheduled target.
//
// Failure taxonomy (the spec's "Durable result protocol" bullets):
//   - a scheduled target with no file            -> OutcomeNoResult
//   - a file whose target field != its filename  -> OutcomeInvalidResult (wrong-target)
//   - an unparseable/unsupported file            -> OutcomeInvalidResult (malformed)
//   - a stat-dir file matching NO scheduled base -> returned in unknown
//   - a leftover temp file (".stat-*")           -> its owning target OutcomeInvalidResult
//     (an atomic publish did not complete, so no valid record can be shown durable)
//   - a file disagreeing with runner-observed rc -> OutcomeInvalidResult (conflict)
//
// observed is keyed by target Base (Result.Target as ExecuteTarget publishes it);
// a missing key means the target was never observed (e.g. never launched) and the
// conflict cross-check is skipped. interrupted[Base] forces OutcomeInterrupted.
func ValidateResults(scheduled []Target, observed map[string]Result, statDir string, interrupted map[string]bool) (outcomes []TargetOutcome, unknown []string) {
	// Set of the scheduled targets' stat stems, for the unknown/unscheduled scan.
	scheduledStems := make(map[string]bool, len(scheduled))
	for _, t := range scheduled {
		scheduledStems[statStem(t.Base)] = true
	}

	// One pass over the stat dir: split real records from orphaned atomic-publish
	// temps, and surface any record that no scheduled target owns.
	leftoverTemp := false
	if entries, err := os.ReadDir(statDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			// A ".stat-*" temp is WriteResult's pre-rename scratch. Its survival
			// means an atomic publish crashed between create and rename, so the
			// stat dir's durability can no longer be certified (change 0318,
			// "publication not shown durable").
			if strings.HasPrefix(name, ".stat-") {
				leftoverTemp = true
				continue
			}
			if !strings.HasSuffix(name, ".json") {
				continue
			}
			stem := strings.TrimSuffix(name, ".json")
			if !scheduledStems[stem] {
				unknown = append(unknown, name)
			}
		}
	}
	sort.Strings(unknown)

	for _, t := range scheduled {
		stem := statStem(t.Base)
		out := TargetOutcome{Target: t}

		switch {
		case interrupted[t.Base]:
			out.Kind = OutcomeInterrupted
			out.Detail = "interrupted"
		default:
			path := filepath.Join(statDir, statFileName(t.Base))
			if _, err := os.Stat(path); err != nil {
				out.Kind = OutcomeNoResult
				break
			}
			r, err := ReadResult(path)
			if err != nil {
				out.Kind = OutcomeInvalidResult
				out.Detail = "malformed result: " + err.Error()
				break
			}
			// The durable file is named by the .sh-stripped stem while the record
			// keeps the full "test_x.sh" identity, so strip .sh from the record's
			// target before comparing it to the filename stem.
			if statStem(r.Target) != stem {
				out.Kind = OutcomeInvalidResult
				out.Detail = fmt.Sprintf("wrong-target: record claims %q but file is %s.json", r.Target, stem)
				break
			}
			if obs, ok := observed[t.Base]; ok && obs.RC != r.RC {
				out.Kind = OutcomeInvalidResult
				out.Detail = fmt.Sprintf("durable rc=%d conflicts with runner-observed execution rc=%d", r.RC, obs.RC)
				break
			}
			out.Result = r
			if r.RC == 0 {
				out.Kind = OutcomePassed
			} else {
				out.Kind = OutcomeFailed
			}
		}
		outcomes = append(outcomes, out)
	}

	// A leftover atomic-publish temp compromises the durability of the whole stat
	// dir: because the temp carries no recoverable target identity, no otherwise
	// valid record can be shown to be the durable, un-raced value. Fail closed —
	// convert every present-record outcome to invalid rather than certify a
	// possibly-stale pass (spec: "Every internal uncertainty fails closed").
	if leftoverTemp {
		for i := range outcomes {
			if outcomes[i].Kind == OutcomePassed || outcomes[i].Kind == OutcomeFailed {
				outcomes[i].Kind = OutcomeInvalidResult
				outcomes[i].Result = Result{}
				outcomes[i].Detail = "publication not shown durable (leftover .stat-* temp in stat dir)"
			}
		}
	}
	return outcomes, unknown
}

// Tally is the aggregate the exit mapping needs. Files counts only targets with a
// valid, cross-checked result (Passed+Failed); NoResult and Invalid are separate
// certification-failure axes, mirroring the oracle's `files`/`noresult` split.
type Tally struct {
	Files, Passed, Failed, Asserts, NoResult, Invalid, OverDirect int
}

// RenderReport writes the deterministic report to w and returns the tallies the
// exit mapping needs. Rows are sorted by basename then path (byte order), so the
// output is independent of completion order. The per-row format, the SUITE line,
// and the FAILED:/NO RESULT:/OVER BUDGET: blocks match scripts/run-tests.sh
// byte-for-byte (the parity contract); the INVALID/UNSCHEDULED blocks are the
// Go runner's own additions for failure modes the oracle cannot represent.
//
// strict is the run's confirmed exit disposition for a direct crossing: when it is
// set, a strict-armed OverDirect breach on an otherwise-green run gates the run
// (exit 4), so the OVER BUDGET note must render the oracle's strict arm instead of
// the advisory exit-0 line — the exact ADR-0074 harm the note-selection guards.
func RenderReport(w io.Writer, outcomes []TargetOutcome, unknown []string, wall int, verbose, strict bool, logsDir string) Tally {
	ordered := make([]TargetOutcome, len(outcomes))
	copy(ordered, outcomes)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Target.Base != ordered[j].Target.Base {
			return ordered[i].Target.Base < ordered[j].Target.Base
		}
		return ordered[i].Target.Path < ordered[j].Target.Path
	})

	var t Tally
	var failedNames, noresultNames, invalidNames, overNames string

	for _, o := range ordered {
		stem := statStem(o.Target.Base)
		switch o.Kind {
		case OutcomeNoResult:
			t.NoResult++
			noresultNames += " " + stem
			// The oracle's NO RESULT row: rc/ok/notok/secs are all "?".
			fmt.Fprintf(w, "%-52s %4ss  rc=%-3s ok=%-5s notok=%-4s  NO RESULT (the job died before writing one)\n",
				stem, "?", "?", "?", "?")
			continue
		case OutcomeInvalidResult:
			t.Invalid++
			invalidNames += " " + stem
			fmt.Fprintf(w, "%-52s %4ss  rc=%-3s ok=%-5s notok=%-4s  INVALID RESULT (%s)\n",
				stem, "?", "?", "?", "?", o.Detail)
			continue
		case OutcomeInterrupted:
			note := "INTERRUPTED"
			if o.Detail != "" {
				note = "INTERRUPTED (" + o.Detail + ")"
			}
			fmt.Fprintf(w, "%-52s %4ss  rc=%-3s ok=%-5s notok=%-4s  %s\n",
				stem, "?", "?", "?", "?", note)
			continue
		}

		// Passed / Failed: a valid, cross-checked record.
		r := o.Result
		t.Files++
		t.Asserts += r.OK + r.NotOK
		if r.RC == 0 {
			t.Passed++
		} else {
			t.Failed++
			failedNames += " " + stem
		}
		suffix := ""
		if o.OverDirect {
			t.OverDirect++
			overNames += " " + stem
			suffix = fmt.Sprintf("  OVER BUDGET (ceiling %ds)", o.Target.Ceiling)
		}
		fmt.Fprintf(w, "%-52s %4ss  rc=%s  ok=%-5s notok=%-4s%s\n",
			stem, itoa(r.Seconds), itoa(r.RC), itoa(r.OK), itoa(r.NotOK), suffix)

		if verbose || o.Kind == OutcomeFailed {
			dumpLog(w, logsDir, stem)
		}
	}

	fmt.Fprintf(w, "SUITE files=%s passed=%s failed=%s asserts=%s wall=%ss\n",
		itoa(t.Files), itoa(t.Passed), itoa(t.Failed), itoa(t.Asserts), itoa(wall))

	if failedNames != "" {
		fmt.Fprintf(w, "FAILED:%s\n", failedNames)
	}
	if t.NoResult > 0 {
		fmt.Fprintf(w, "NO RESULT:%s\n", noresultNames)
		fmt.Fprintf(w, "%d of %d test files produced no result — those jobs died before recording one (OOM kill under -j, a full disk, an external signal).\n",
			t.NoResult, len(outcomes))
		fmt.Fprintf(w, "This run certified nothing about them: re-run, and if it recurs lower -j.\n")
	}
	if t.Invalid > 0 {
		fmt.Fprintf(w, "INVALID RESULT:%s\n", invalidNames)
		fmt.Fprintf(w, "%d result file(s) were present but could not be trusted (mis-published target, malformed record, or a conflict with the observed execution): this run certified nothing about them.\n", t.Invalid)
	}
	if len(unknown) > 0 {
		fmt.Fprintf(w, "UNSCHEDULED RESULT:%s\n", " "+strings.Join(unknown, " "))
		fmt.Fprintf(w, "%d stat record(s) matched no scheduled target — the run cannot account for them.\n", len(unknown))
	}
	if overNames != "" {
		fmt.Fprintf(w, "OVER BUDGET:%s\n", overNames)
		// The remedy leads with the substantive fix and must NOT teach raising the
		// ceiling — a budget guard whose remedy is "raise the number" teaches the
		// evasion it exists to catch (learning guard-remedy-must-not-teach-the-evasion).
		fmt.Fprintf(w, "Remedy: shard this file or extend an existing shard so each part stays under its ceiling.\n")
		// State the posture out loud, red branch first: a reader of a failing or
		// incomplete run must never be handed "the tests all passed" (ADR-0074).
		switch {
		case t.Failed > 0:
			fmt.Fprintf(w, "Note: this run already fails on test failures (exit 1). The breach above is a separate finding.\n")
		case t.NoResult > 0 || t.Invalid > 0 || len(unknown) > 0:
			fmt.Fprintf(w, "Note: this run already fails on missing results (exit 3). The breach above is a separate finding.\n")
		case strict:
			// The strict arm: a strict-armed direct crossing on an otherwise-green
			// run gates the run (exit 4), so a reader must NOT be handed "the tests
			// all passed … (exit 0)" (ADR-0074, same reason the red branch leads).
			// Byte-for-byte with the oracle's `BUDGET_STRICT=1` arm in
			// scripts/run-tests.sh ("Strict: --strict-budget was given …").
			fmt.Fprintf(w, "Strict: --strict-budget was given, so this breach fails the run (exit 4). The tests themselves passed.\n")
		default:
			fmt.Fprintf(w, "Advisory: the tests all passed, so this run does not fail on the breach (exit 0).\n")
			// The Go entry `docket development test` takes cobra.NoArgs and exposes
			// no --strict-budget flag; strict is reachable ONLY through the
			// DOCKET_RUNTESTS_STRICT=1 env seam, so name that seam rather than the
			// oracle's flag (which would be a usage error here). This human-facing
			// sentence is a documented Go/oracle deviation; the differential harness
			// normalizes the knob spelling (tests/test_devtest_differential.sh).
			fmt.Fprintf(w, "Set DOCKET_RUNTESTS_STRICT=1 to gate on it — but see scripts/run-tests.md first: the screening factor is calibrated to one machine (change 0251).\n")
		}
	}
	return t
}

// dumpLog reproduces the oracle's inline per-failure log dump. A missing or
// unreadable log yields an empty body between the markers rather than an error,
// keeping the report deterministic when no log exists (e.g. a hand-built stat dir).
func dumpLog(w io.Writer, logsDir, stem string) {
	fmt.Fprintf(w, "---- %s ----\n", stem)
	if logsDir != "" {
		if data, err := os.ReadFile(filepath.Join(logsDir, stem+".log")); err == nil {
			w.Write(data)
		}
	}
	fmt.Fprintf(w, "---- end %s ----\n", stem)
}

// ExitCode applies the oracle's precedence 1 > 3 > 4 > 0. A failed test wins
// (exit 1); a scheduled target that produced no valid result — no file, an
// invalid file, or an unscheduled/unknown file — certifies nothing about the set
// (exit 3); strictArmed is Task 5's confirmed/failed budget breach (exit 4); a
// clean run, advisory breaches included, exits 0.
func ExitCode(t Tally, unknownCount int, strictArmed bool) int {
	switch {
	case t.Failed > 0:
		return 1
	case t.NoResult > 0 || t.Invalid > 0 || unknownCount > 0:
		return 3
	case strictArmed:
		return 4
	default:
		return 0
	}
}

func itoa(n int) string { return strconv.Itoa(n) }
