package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/repository"
)

// Run-gate verdict test helpers shared with default-build (untagged) test files.
// The verdict tests themselves live behind the integration tag in
// rungate_verdict_integration_test.go (change 0465); these fixtures, fakes and
// probes stay untagged because other untagged test files still reference them.

const gateDefaultClaimedAt = "2026-08-02T00:00:00Z"

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

// gateMintArmed mints an armed record (Retry unused, no attribution yet) with the
// given before-set, dispatch epoch, and child-context hash, as run start would.
// Since change 0407 the before-set and dispatch epoch are diagnostics only (they
// no longer create attribution); hash is the record's ChildContextHash, the seam
// the verdict path's proof filter keys on.
func gateMintArmed(t *testing.T, repoDir string, beforeIDs []int, dispatchEpoch int64, hash string) string {
	t.Helper()
	key, err := MintGateRecord(repoDir, GateRecord{
		Target:           "docket-implement-next",
		CreatedAt:        1,
		DispatchEpoch:    dispatchEpoch,
		BeforeIDs:        beforeIDs,
		ChildContextHash: hash,
		Retry:            RetryUnused,
		Disposition:      "gate-armed",
		AttemptLimit:     2,
	})
	if err != nil {
		t.Fatalf("MintGateRecord: %v", err)
	}
	return key
}

// fakeProofScanner is the injected ClaimProofScanner for the verdict path's
// ownership resolution (change 0407): it returns canned proofs newest-first, or an
// error. A nil scanner (not this fake) is the fail-closed proof-unavailable case.
type fakeProofScanner struct {
	proofs []ClaimProof
	err    error
}

func (f *fakeProofScanner) ScanClaimProofs(context.Context, string) ([]ClaimProof, error) {
	return f.proofs, f.err
}

// gateRetryMarkerExists reports whether the O_EXCL retry marker for key exists on
// disk. It reads the FILESYSTEM (never a mock), so a continuation that must never
// reach the retry CAS is a real, provable property.
func gateRetryMarkerExists(t *testing.T, repoDir, key string) bool {
	t.Helper()
	common, err := gateGitCommonDir(repoDir)
	if err != nil {
		t.Fatalf("gateGitCommonDir: %v", err)
	}
	_, serr := os.Stat(filepath.Join(common, "docket", runTrackerDirName, key, gateRetryMarkerName))
	if serr != nil && !os.IsNotExist(serr) {
		t.Fatalf("stat retry marker: %v", serr)
	}
	return serr == nil
}

// verdictCompletionFixture is one prepared run whose keyed verdict verifies
// run-complete AND whose epoch ownership is ready to close out.
type verdictCompletionFixture struct {
	repo, key, epochID, worktree string
	store                        *gatedrive.Store
	deps                         PlanningDeps
	wdeps                        WorkspaceDeps
	gdeps                        GitHubDeps
	observer                     *fakeProcessObserver
	launchObserver               *fakeLaunchObserver
}

// seams returns the injected completion seam bundle for a direct completeSuccessfulRun
// call (used to pre-drive the closeout before a persistence-fault replay test).
func (fx verdictCompletionFixture) seams() cancelSeams {
	return cancelSeams{store: fx.store, observer: fx.observer, launchObserver: fx.launchObserver}
}

func newVerdictCompletionFixture(t *testing.T) verdictCompletionFixture {
	t.Helper()
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		rvRecord(rvPlanPath, rvResultsPath, rvRecordedPR(), "feat/"+rvSlug),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	repo := f.repo.invocation
	common, err := gateGitCommonDir(repo)
	if err != nil {
		t.Fatalf("gateGitCommonDir: %v", err)
	}
	key := gateMintArmed(t, repo, nil, 1, "ha")
	if err := ReserveGateClaim(repo, key, 3, "claim-3-v"); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := ConfirmGateClaim(repo, key, 3, "claim-3-v", "r1", ""); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	wdeps.ClaimProofs = &fakeProofScanner{proofs: []ClaimProof{
		{RequestID: "claim-3-v", ChangeID: 3, GateContextHash: "ha", Revision: "r1"},
	}}

	ep, err := MintEpochRecord(repo, key, "3")
	if err != nil {
		t.Fatalf("MintEpochRecord: %v", err)
	}
	worktree := filepath.Join(repo, "feature-wt")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}
	if err := epochCAS(repo, key, func(r *EpochRecord) error {
		r.Worktree = worktree
		return nil
	}); err != nil {
		t.Fatalf("epochCAS set worktree: %v", err)
	}
	store := gatedrive.OpenStore(common)
	// A released epoch-owned slot (the run's drives are done) is exactly what the
	// closeout retires — reserve+confirm+release, mirroring the cancel/completion
	// fixtures. Release retains RunEpochID (between-drive ownership), so the slot is
	// slotOwned+released until the closeout detaches it.
	runDir := filepath.Join(worktree, "run-1")
	token, terr := store.ReserveWorktreeExecutionForEpoch(common, worktree, ep.EpochID, nil)
	if terr != nil {
		t.Fatalf("ReserveWorktreeExecutionForEpoch: %v", terr)
	}
	if cerr := store.ConfirmWorktreeExecution(worktree, token, "run-1", runDir); cerr != nil {
		t.Fatalf("ConfirmWorktreeExecution: %v", cerr)
	}
	slot, _, lerr := store.LoadWorktreeExecution(worktree)
	if lerr != nil {
		t.Fatalf("LoadWorktreeExecution: %v", lerr)
	}
	if rerr := store.ReleaseWorktreeExecution(worktree, slot.ReservationToken); rerr != nil {
		t.Fatalf("ReleaseWorktreeExecution: %v", rerr)
	}
	must(t, RegisterEpochParticipant(repo, key, ep.EpochID,
		EpochParticipant{Kind: "coordinator", NativeHandle: "turn-1"}))
	must(t, RecordEpochParticipantTerminal(repo, key, ep.EpochID,
		"turn-1", "t1", ParticipantTerminalCompleted))

	observer := &fakeProcessObserver{defaultProven: true}
	launchObserver := &fakeLaunchObserver{report: gatedrive.EpochLaunchReport{Accounted: true}}
	wdeps.CancelSeams = func(string) cancelSeams {
		return cancelSeams{store: store, observer: observer, launchObserver: launchObserver}
	}
	return verdictCompletionFixture{
		repo: repo, key: key, epochID: ep.EpochID, worktree: worktree, store: store,
		deps: deps, wdeps: wdeps, gdeps: gdeps, observer: observer, launchObserver: launchObserver,
	}
}
