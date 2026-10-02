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

// Run-tracker verdict test helpers shared with default-build (untagged) test files.
// The verdict tests themselves live behind the integration tag in
// runtracker_verdict_integration_test.go (change 0465); these fixtures, fakes and
// probes stay untagged because other untagged test files still reference them.

const runTrackerDefaultClaimedAt = "2026-08-02T00:00:00Z"

// runTrackerInProgressBlob builds an in-progress change blob whose claimed_at is the
// default stamp ("keep"), removed (""), or replaced with the given raw value.
func runTrackerInProgressBlob(id int, slug, claimedAt string) StatusBlob {
	src := lifecycleChange(id, slug, "in-progress")
	const def = "claimed_at: " + runTrackerDefaultClaimedAt
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
		Revision: miRevision,
		Data:     []byte(src),
	}
}

// runTrackerIncompleteRecord renders an in-progress change 3 carrying a valid pr: and
// linkage, so the ONLY unmet postcondition RunVerify reports is not-implemented
// (the run is claimed but not yet marked implemented). It lets the retry-mapping
// tests assert an exact single-condition report line.
func runTrackerIncompleteRecord() []byte {
	src := string(rvInProgressRecord(rvPlanPath, rvResultsPath, "feat/"+rvSlug))
	src = strings.Replace(src, "blocked_by:\n", "pr: '"+rvRecordedPR()+"'\nblocked_by:\n", 1)
	return []byte(src)
}

// runTrackerMintStarted mints a started record (Retry unused, no attribution yet) with the
// given before-set, dispatch time, and child-context hash, as run start would.
// Since change 0407 the before-set and dispatch time are diagnostics only (they
// no longer create attribution); hash is the record's ChildContextHash, the seam
// the verdict path's proof filter keys on.
func runTrackerMintStarted(t *testing.T, repoDir string, beforeIDs []int, dispatchedAt int64, hash string) string {
	t.Helper()
	key, err := MintRunTrackerRecord(repoDir, RunTrackerRecord{
		Target:           "docket-implement-next",
		CreatedAt:        1,
		DispatchedAt:     dispatchedAt,
		BeforeIDs:        beforeIDs,
		ChildContextHash: hash,
		Retry:            RetryUnused,
		Disposition:      "run-started",
		AttemptLimit:     2,
	})
	if err != nil {
		t.Fatalf("MintRunTrackerRecord: %v", err)
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

// runTrackerRetryMarkerExists reports whether the O_EXCL retry marker for key exists on
// disk. It reads the FILESYSTEM (never a mock), so a continuation that must never
// reach the retry CAS is a real, provable property.
func runTrackerRetryMarkerExists(t *testing.T, repoDir, key string) bool {
	t.Helper()
	common, err := runTrackerGitCommonDir(repoDir)
	if err != nil {
		t.Fatalf("runTrackerGitCommonDir: %v", err)
	}
	_, serr := os.Stat(filepath.Join(common, "docket", runTrackerDirName, key, runTrackerRetryMarkerName))
	if serr != nil && !os.IsNotExist(serr) {
		t.Fatalf("stat retry marker: %v", serr)
	}
	return serr == nil
}

// verdictCompletionFixture is one prepared run whose keyed verdict verifies
// run-complete AND whose run is ready to close out: a terminal coordinator and an
// accounted launch census.
type verdictCompletionFixture struct {
	repo, key, runID, worktree string
	store                      *gatedrive.Store
	deps                       PlanningDeps
	wdeps                      WorkspaceDeps
	gdeps                      GitHubDeps
	observer                   *fakeProcessObserver
	launchObserver             *fakeLaunchObserver
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
	common, err := runTrackerGitCommonDir(repo)
	if err != nil {
		t.Fatalf("runTrackerGitCommonDir: %v", err)
	}
	key := runTrackerMintStarted(t, repo, nil, 1, "ha")
	if err := ReserveRunTrackerClaim(repo, key, 3, "claim-3-v"); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := ConfirmRunTrackerClaim(repo, key, 3, "claim-3-v", "r1", ""); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	wdeps.ClaimProofs = &fakeProofScanner{proofs: []ClaimProof{
		{RequestID: "claim-3-v", ChangeID: 3, RunContextHash: "ha", Revision: "r1"},
	}}

	ep, err := MintRunRecord(repo, key, "3")
	if err != nil {
		t.Fatalf("MintRunRecord: %v", err)
	}
	worktree := filepath.Join(repo, "feature-wt")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}
	if err := runRecordCAS(repo, key, func(r *RunRecord) error {
		r.Worktree = worktree
		return nil
	}); err != nil {
		t.Fatalf("runRecordCAS set worktree: %v", err)
	}
	// The run's drives are done: the closeout's observe-only census (faked here by
	// launchObserver, accounted) finds nothing live, and no worktree record exists to
	// retire (change 0490).
	store := gatedrive.OpenStore(common)
	must(t, RegisterRunParticipant(repo, key, ep.RunID,
		RunParticipant{Kind: "coordinator", NativeHandle: "turn-1"}))
	must(t, RecordRunParticipantTerminal(repo, key, ep.RunID,
		"turn-1", "t1", ParticipantTerminalCompleted))

	observer := &fakeProcessObserver{defaultProven: true}
	launchObserver := &fakeLaunchObserver{report: gatedrive.RunLaunchReport{Accounted: true}}
	wdeps.CancelSeams = func(string) cancelSeams {
		return cancelSeams{store: store, observer: observer, launchObserver: launchObserver}
	}
	return verdictCompletionFixture{
		repo: repo, key: key, runID: ep.RunID, worktree: worktree, store: store,
		deps: deps, wdeps: wdeps, gdeps: gdeps, observer: observer, launchObserver: launchObserver,
	}
}
