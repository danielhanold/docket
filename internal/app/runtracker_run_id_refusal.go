package app

import "path/filepath"

// This file is the run-epoch refusal vocabulary (change 0463). The run epoch is a
// public locator (ADR-0111) that a caller threads into --run-id flags (gate drive
// start, gate drive prepare-scope, agent.enter). When the presented value cannot be
// resolved, the caller must learn WHICH mistake it made through a stable token. A
// catch-all invalid-request makes a misrouted token (0382: the dispatch context
// passed as the epoch) indistinguishable from a malformed request. Tokens are a
// fixed vocabulary; nothing here echoes the presented value, a path, or record
// content.

// ReasonUnknownRunID is the stable refusal token for a --run-id that names no
// run epoch in this repository.
const ReasonUnknownRunID = "unknown-run-id"

// ClassifyRunIDError maps a run-epoch registry failure (an *RunError anywhere
// in err's chain) to a protocol result and a bounded reason token:
//   - not-found: unknown-run-id.
//   - mismatch: the existing stale-linkage token, stale-run-id.
//   - corrupt or unreadable: internal-error carrying the kind.
//   - any other readable-but-unusable registry state: invalid-input carrying the kind.
//
// ok is false when err carries no *RunError, so callers fall through to their
// own classification.
func ClassifyRunIDError(err error) (Result, string, bool) {
	ee, ok := AsRunError(err)
	if !ok {
		return "", "", false
	}
	switch ee.Kind {
	case ErrRunNotFound:
		return ResultInvalidInput, ReasonUnknownRunID, true
	case ErrRunIDMismatch:
		return ResultInvalidInput, ErrStaleRunID.Reason, true
	case ErrRunRecordCorrupt, ErrRunRecordIO:
		return ResultInternalError, string(ee.Kind), true
	default:
		return ResultInvalidInput, string(ee.Kind), true
	}
}

// RunIDNextAction maps a run-epoch refusal reason to a one-line, credential-free
// next action (the ownershipNextAction / fenceNextAction pattern). It never echoes
// the presented value. A reason with no specific remedy yields "", and callers then
// omit the message.
func RunIDNextAction(reason string) string {
	switch reason {
	case ReasonUnknownRunID:
		return "the --run-id value names no run epoch in this repository; pass the <epoch> field of the arm's " +
			"`run-started <key> <epoch> <dispatch-context>` line (the <dispatch-context> goes to --gate-context) — " +
			"never drop --run-id and retry"
	case ErrStaleRunID.Reason:
		return "the --run-id value is not the run epoch this gate key carries; pass the <epoch> printed on the same run-started line as the key"
	default:
		return ""
	}
}

// runIDLocator builds the existence check prepare-scope runs on a presented
// --run-id. It resolves the id to exactly one epoch record under gitCommonDir's
// run-epoch registry through findRunDirByID (the same locator the epoch launch
// gate uses) and returns its typed RunError (not-found, ambiguous, IO) unchanged.
// It checks RESOLVABILITY only, never liveness: a scope may legitimately carry a
// cancelled epoch (the takeover revocation gate reads it later), and the launch gate
// still enforces liveness and worktree ownership at start.
func runIDLocator(gitCommonDir string) func(string) error {
	runTrackerRoot := filepath.Join(gitCommonDir, "docket", runTrackerDirName)
	return func(runID string) error {
		_, _, err := findRunDirByID(runTrackerRoot, runID)
		return err
	}
}

// CheckRunIDExists verifies, before agent.enter spawns anything, that a lone
// --run-id (presented without --run-key) resolves to exactly one run epoch in
// repoDir's repository (change 0463). It is the same resolvability check prepare-scope
// runs (runIDLocator / findRunDirByID): it never checks liveness, and it returns
// the locator's typed *RunError (not-found, ambiguous, IO) unchanged. A repository
// whose git common dir cannot be resolved yields ErrRunRecordIO.
func CheckRunIDExists(repoDir, runID string) error {
	common, err := runTrackerGitCommonDir(repoDir)
	if err != nil {
		return runErr(ErrRunRecordIO, "check-exists", err)
	}
	return runIDLocator(common)(runID)
}

// CheckRunIDLinkage verifies, before agent.enter spawns anything, that the
// presented (--run-key, --run-id) pair names a real run epoch (change 0463).
// It returns nil when the gate key's epoch record carries exactly runID, and
// otherwise ALWAYS an *RunError:
//   - a gate key with no directory, a malformed key, or no epoch record:
//     ErrRunNotFound (the pair names no epoch);
//   - a different recorded id: ErrRunIDMismatch;
//   - a corrupt record: keeps ErrRunRecordCorrupt;
//   - any other resolution fault: ErrRunRecordIO.
//
// It only reads. It never checks liveness, because participant registration still
// refuses a non-active epoch.
func CheckRunIDLinkage(repoDir, runKey, runID string) error {
	rec, _, err := LoadRunRecord(repoDir, runKey)
	if err != nil {
		if _, ok := AsRunError(err); ok {
			return err
		}
		if ge, ok := AsRunTrackerStoreError(err); ok && (ge.Kind == ErrRunTrackerNotFound || ge.Kind == ErrRunTrackerMalformedKey) {
			return runErr(ErrRunNotFound, "check-linkage", nil)
		}
		return runErr(ErrRunRecordIO, "check-linkage", err)
	}
	if rec.RunID != runID {
		return runErr(ErrRunIDMismatch, "check-linkage", nil)
	}
	return nil
}
