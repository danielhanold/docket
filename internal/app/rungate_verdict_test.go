package app

import (
	"github.com/danielhanold/docket/internal/repository"
	"strings"
	"testing"
	"time"
)

// These are the `docket run gate-verdict <key>` (attributed mode) tests (change
// 0334, Task 3). gate-verdict loads the durable gate record, attributes exactly
// one new in-progress claim through the three filters (id not in the before-set;
// claimed_at parses; claimed_at >= dispatch epoch), then delegates the run
// predicate to RunVerify and maps its verdict onto the attributed vocabulary —
// consuming the single retry permit BEFORE emitting so a lost retry is the safe
// failure, and failing closed to gate-unavailable on any load error or
// unrecognized verdict. Every outcome is a report line that exits 0.
//
// The store is rooted at a real temp git repo (newGateRepo / the run-verify
// fixture's invocation clone); attribution reads the fakeReader corpus; the
// RunVerify delegation is driven by the run_verify_test.go fixtures (rvFixture,
// rvRecord, rvInProgressRecord, rvPR, rvAgreeingReceipt).

const gateDefaultClaimedAt = "2026-08-02T00:00:00Z"

// gateClaimEpoch is the Unix epoch of gateDefaultClaimedAt — the claim instant
// lifecycleChange stamps on an in-progress record. Attribution filter (c)
// compares a candidate's claimed_at against the record's DispatchEpoch, so tests
// straddle this value.
func gateClaimEpoch(t *testing.T) int64 {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, gateDefaultClaimedAt)
	if err != nil {
		t.Fatalf("parse %q: %v", gateDefaultClaimedAt, err)
	}
	return tm.Unix()
}

// gateInProgressBlob builds an in-progress change blob whose claimed_at is the
// default stamp ("keep"), removed (""), or replaced with the given raw value.
func gateInProgressBlob(id int, slug, claimedAt string) StatusBlob {
	src := lifecycleChange(id, slug, "in-progress")
	const def = "claimed_at: " + gateDefaultClaimedAt
	switch claimedAt {
	case "keep":
		// leave the default stamp in place
	case "":
		src = strings.Replace(src, def+"\n", "", 1)
	default:
		src = strings.Replace(src, def, "claimed_at: "+claimedAt, 1)
	}
	return StatusBlob{
		Kind:     repository.KindChange,
		Location: repository.LocationActive,
		Path:     groomPath(id, slug),
		Version:  miVersion,
		Data:     []byte(src),
	}
}

// gateIncompleteRecord renders an in-progress change 3 carrying a valid pr: and
// linkage, so the ONLY unmet postcondition RunVerify reports is not-implemented
// (the run is claimed but not yet marked implemented). It lets the retry-mapping
// tests assert an exact single-conjunct report line.
func gateIncompleteRecord() []byte {
	src := string(rvInProgressRecord(rvPlanPath, rvResultsPath, "feat/"+rvSlug))
	src = strings.Replace(src, "blocked_by:\n", "pr: '"+rvRecordedPR()+"'\nblocked_by:\n", 1)
	return []byte(src)
}

// gateLightDeps wires a planning-only deps set over the given corpus (no git
// client, no workspace/github seams) for attribution paths that never reach
// RunVerify.
func gateLightDeps(t *testing.T, corpus []StatusBlob) PlanningDeps {
	t.Helper()
	return PlanningDeps{Reader: &fakeReader{pin: mainPin(t), corpus: corpus}, Clock: testClock()}
}

// gateMintArmed mints an armed record (Retry unused, no attribution yet) with the
// given before-set and dispatch epoch, as gate-before would.
func gateMintArmed(t *testing.T, repoDir string, beforeIDs []int, dispatchEpoch int64) string {
	t.Helper()
	key, err := MintGateRecord(repoDir, GateRecord{
		Target:        "docket-implement-next",
		CreatedAt:     1,
		DispatchEpoch: dispatchEpoch,
		BeforeIDs:     beforeIDs,
		Retry:         RetryUnused,
		Disposition:   "gate-armed",
	})
	if err != nil {
		t.Fatalf("MintGateRecord: %v", err)
	}
	return key
}

// gateMintAttributed mints a record already attributed to id — the state a second
// gate-verdict call reads after a first call attributed the claim.
func gateMintAttributed(t *testing.T, repoDir string, id int) string {
	t.Helper()
	key := gateMintArmed(t, repoDir, nil, 1)
	rec, err := LoadGateRecord(repoDir, key)
	if err != nil {
		t.Fatalf("LoadGateRecord: %v", err)
	}
	rec.AttributedID = id
	if err := SaveGateRecord(repoDir, key, rec); err != nil {
		t.Fatalf("SaveGateRecord: %v", err)
	}
	return key
}

// --- attribution filters (never reach RunVerify) ---------------------------

// --- RunVerify delegation (mapping table) ----------------------------------

// --- fail-closed: load errors and unknown verdicts -------------------------

// --- concurrency, isolation, restart durability ----------------------------

// --- unattributed (observe-only) mode --------------------------------------
//
// RunGateVerdictObserve is the `--unattributed` mode (change 0334, Task 4): NO
// key, NO record, NO writes. It re-syncs, verifies each supplied hint id (a hint
// is an id to verify, never attribution evidence) — or every current in-progress
// id when none are supplied — and renders one `gate-observe <verdict> <id>` line
// per id using RunVerify's verdict verbatim, through a SEPARATE render function
// that only knows the `gate-observe` prefix and is structurally unable to emit
// gate-retry-once.

// gateHaltedInProgressBlob builds an in-progress change carrying a durable
// "## Run halted" body section — status stays in-progress (so observeInProgress
// counts it) while RunVerify short-circuits it to run-halted with reader-only
// deps, letting the observe tests assert exact lines without the full git path.
func gateHaltedInProgressBlob(id int, slug string) StatusBlob {
	src := strings.TrimRight(lifecycleChange(id, slug, "in-progress"), "\n") +
		"\n\n## Run halted\n\n### 2026-08-14\n\nPaused.\n"
	return StatusBlob{
		Kind:     repository.KindChange,
		Location: repository.LocationActive,
		Path:     groomPath(id, slug),
		Version:  miVersion,
		Data:     []byte(src),
	}
}

// gateProposedBlob builds a proposed (never-claimed) change — RunVerify maps it
// to run-unclaimed with reader-only deps.
func gateProposedBlob(id int, slug string) StatusBlob {
	return StatusBlob{
		Kind:     repository.KindChange,
		Location: repository.LocationActive,
		Path:     groomPath(id, slug),
		Version:  miVersion,
		Data:     []byte(lifecycleChange(id, slug, "proposed")),
	}
}
