package app

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/danielhanold/docket/internal/repository"
)

// These are the `docket run gate-before` (arm the gate) tests (change 0334,
// Task 2). gate-before re-syncs the metadata worktree to fresh origin, reads the
// in-progress claim set, captures a dispatch epoch AFTER that read, and mints a
// durable gate record — printing `gate-armed <key>` on success and
// `gate-unarmed <reason-token>` on any failure, exiting 0 either way (the report
// line is the contract). Only `implement-next` is an accepted target; anything
// else is a usage error that exits non-zero.
//
// The in-progress read reuses the same PinContext/ReadCorpus plumbing the claim
// path uses (a fresh-origin fetch inside PinContext, one change-file parse), so
// these tests inject the scriptable fakeReader for the corpus and a real temp
// git repo (newGateRepo) for the store's git-common-dir rooting.

// gateBeforeCorpus is a scriptable in-progress claim set: ids 3 and 7 are
// in-progress (the before-set), id 5 is proposed and id 8 is blocked (neither is
// an in-progress claim), so a correct before-read collects exactly {3, 7}.
func gateBeforeCorpus() []StatusBlob {
	blob := func(id int, slug, status string) StatusBlob {
		return StatusBlob{
			Kind:     repository.KindChange,
			Location: repository.LocationActive,
			Path:     groomPath(id, slug),
			Version:  miVersion,
			Data:     []byte(lifecycleChange(id, slug, status)),
		}
	}
	return []StatusBlob{
		blob(3, "alpha", "in-progress"),
		blob(7, "bravo", "in-progress"),
		blob(5, "charlie", "proposed"),
		blob(8, "delta", "blocked"),
	}
}

// gateBeforeReader is the fakeReader wired with a mainPin and the given corpus /
// injected errors.
func gateBeforeReader(t *testing.T, corpus []StatusBlob, pinErr, corpusErr error) *fakeReader {
	t.Helper()
	return &fakeReader{pin: mainPin(t), corpus: corpus, pinErr: pinErr, corpusErr: corpusErr}
}

// TestRunGateBeforeArmsWithLoadableKey: a successful arm prints `gate-armed
// <key>`, the record loads, and its BeforeIDs are exactly the fixture's
// in-progress ids with the store-owned target and an unused retry permit.
func TestRunGateBeforeArmsWithLoadableKey(t *testing.T) {
	repo := newGateRepo(t)
	deps := PlanningDeps{Reader: gateBeforeReader(t, gateBeforeCorpus(), nil, nil), Clock: testClock()}

	res := RunGateBefore(context.Background(), deps, repo, "implement-next")
	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want %q (report lines exit 0)", res.Result, ResultApplied)
	}
	if code := ExitCode(res.Env().Result); code != 0 {
		t.Errorf("gate-armed exit code = %d, want 0", code)
	}
	if !res.Armed || res.Key == "" {
		t.Fatalf("Armed=%v Key=%q, want armed with a non-empty key", res.Armed, res.Key)
	}
	if got, want := res.HumanText(), "gate-armed "+res.Key; got != want {
		t.Errorf("HumanText = %q, want %q", got, want)
	}

	got, err := LoadGateRecord(repo, res.Key)
	if err != nil {
		t.Fatalf("LoadGateRecord(%q): %v", res.Key, err)
	}
	if want := []int{3, 7}; !slices.Equal(got.BeforeIDs, want) {
		t.Errorf("BeforeIDs = %v, want %v", got.BeforeIDs, want)
	}
	if got.Target != "docket-implement-next" {
		t.Errorf("Target = %q, want %q", got.Target, "docket-implement-next")
	}
	if got.Retry != RetryUnused {
		t.Errorf("Retry = %q, want %q", got.Retry, RetryUnused)
	}
	if got.AttributedID != 0 {
		t.Errorf("AttributedID = %d, want 0 (not yet attributed)", got.AttributedID)
	}
	if got.Terminal {
		t.Errorf("Terminal = true, want false on a fresh arm")
	}
}

// TestRunGateBeforeDispatchEpochAfterBeforeRead: DispatchEpoch is captured after
// the before-read, so it is at or after the record's CreatedAt and is a real
// (non-zero) wall-clock stamp.
func TestRunGateBeforeDispatchEpochAfterBeforeRead(t *testing.T) {
	repo := newGateRepo(t)
	deps := PlanningDeps{Reader: gateBeforeReader(t, gateBeforeCorpus(), nil, nil), Clock: testClock()}

	res := RunGateBefore(context.Background(), deps, repo, "implement-next")
	if !res.Armed {
		t.Fatalf("gate did not arm: %q", res.HumanText())
	}
	rec, err := LoadGateRecord(repo, res.Key)
	if err != nil {
		t.Fatalf("LoadGateRecord: %v", err)
	}
	if rec.CreatedAt <= 0 || rec.DispatchEpoch <= 0 {
		t.Fatalf("CreatedAt=%d DispatchEpoch=%d, want real wall-clock stamps", rec.CreatedAt, rec.DispatchEpoch)
	}
	if rec.DispatchEpoch < rec.CreatedAt {
		t.Errorf("DispatchEpoch %d < CreatedAt %d, want captured at or after the before-read", rec.DispatchEpoch, rec.CreatedAt)
	}
}

// TestRunGateBeforeEmptyBacklogArms: no in-progress claims still arms with an
// empty before-set — an empty set is a valid observation, not a failure.
func TestRunGateBeforeEmptyBacklogArms(t *testing.T) {
	repo := newGateRepo(t)
	deps := PlanningDeps{Reader: gateBeforeReader(t, []StatusBlob{}, nil, nil), Clock: testClock()}

	res := RunGateBefore(context.Background(), deps, repo, "implement-next")
	if !res.Armed {
		t.Fatalf("empty backlog did not arm: %q", res.HumanText())
	}
	rec, err := LoadGateRecord(repo, res.Key)
	if err != nil {
		t.Fatalf("LoadGateRecord: %v", err)
	}
	if len(rec.BeforeIDs) != 0 {
		t.Errorf("BeforeIDs = %v, want empty", rec.BeforeIDs)
	}
}

// TestRunGateBeforeUnreadableChangesDir: an unreadable corpus prints
// `gate-unarmed <reason>` with a stable token and still exits 0 (the report line
// is the contract), and mints no record.
func TestRunGateBeforeUnreadableChangesDir(t *testing.T) {
	repo := newGateRepo(t)
	deps := PlanningDeps{Reader: gateBeforeReader(t, nil, nil, errors.New("changes dir unreadable")), Clock: testClock()}

	res := RunGateBefore(context.Background(), deps, repo, "implement-next")
	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want %q (gate-unarmed is a report line)", res.Result, ResultApplied)
	}
	if code := ExitCode(res.Env().Result); code != 0 {
		t.Errorf("gate-unarmed exit code = %d, want 0", code)
	}
	if res.Armed {
		t.Fatalf("armed on an unreadable corpus, want gate-unarmed")
	}
	if res.Reason != ReasonGateChangesUnreadable {
		t.Errorf("Reason = %q, want %q", res.Reason, ReasonGateChangesUnreadable)
	}
	if got, want := res.HumanText(), "gate-unarmed "+ReasonGateChangesUnreadable; got != want {
		t.Errorf("HumanText = %q, want %q", got, want)
	}
}

// TestRunGateBeforeSyncFailure: a fresh-origin re-sync failure (PinContext) is
// reported as gate-unarmed with the sync reason token, exit 0.
func TestRunGateBeforeSyncFailure(t *testing.T) {
	repo := newGateRepo(t)
	deps := PlanningDeps{Reader: gateBeforeReader(t, nil, errors.New("fetch failed"), nil), Clock: testClock()}

	res := RunGateBefore(context.Background(), deps, repo, "implement-next")
	if res.Armed {
		t.Fatalf("armed despite a sync failure, want gate-unarmed")
	}
	if res.Reason != ReasonGateSyncFailed {
		t.Errorf("Reason = %q, want %q", res.Reason, ReasonGateSyncFailed)
	}
	if code := ExitCode(res.Env().Result); code != 0 {
		t.Errorf("gate-unarmed exit code = %d, want 0", code)
	}
}

// TestRunGateBeforeInvalidTarget: any target other than `implement-next` is a
// usage error — a non-zero exit with no gate-armed / gate-unarmed report line.
func TestRunGateBeforeInvalidTarget(t *testing.T) {
	repo := newGateRepo(t)
	deps := PlanningDeps{Reader: gateBeforeReader(t, gateBeforeCorpus(), nil, nil), Clock: testClock()}

	res := RunGateBefore(context.Background(), deps, repo, "bogus-target")
	if res.Armed {
		t.Fatalf("armed for an invalid target")
	}
	if code := ExitCode(res.Env().Result); code == 0 {
		t.Errorf("invalid-target exit code = 0, want non-zero (result %q)", res.Env().Result)
	}
	if res.Key != "" {
		t.Errorf("invalid target minted a key %q, want none", res.Key)
	}
}
