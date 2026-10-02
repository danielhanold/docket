package app

import "path/filepath"

// This file is the run refusal vocabulary (change 0463). The run id is a
// public locator (ADR-0111) that a caller threads into --run-id flags (gate drive
// start, agent.enter). When the presented value cannot be
// resolved, the caller must learn WHICH mistake it made through a stable token. A
// catch-all invalid-request makes a misrouted token (0382: the run context
// passed as the run id) indistinguishable from a malformed request. Tokens are a
// fixed vocabulary; nothing here echoes the presented value, a path, or record
// content.

// ReasonUnknownRunID is the stable refusal token for a --run-id that names no
// run in this repository.
const ReasonUnknownRunID = "unknown-run-id"

// ClassifyRunIDError maps a run registry failure (a *RunError anywhere
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

// RunIDNextAction maps a run refusal reason to a one-line, credential-free
// next action (the ownershipNextAction pattern). It never echoes
// the presented value. A reason with no specific remedy yields "", and callers then
// omit the message.
func RunIDNextAction(reason string) string {
	switch reason {
	case ReasonUnknownRunID:
		return "the --run-id value names no run in this repository; pass the <run-id> field of the start's " +
			"`run-started <key> <run-id> <run-context>` line (the <run-context> goes to --run-context on change claim and the gate drive) — " +
			"never drop --run-id and retry"
	case ErrStaleRunID.Reason:
		return "the --run-id value is not the run this run key carries; pass the <run-id> printed on the same run-started line as the key"
	default:
		return ""
	}
}

// runIDLocator builds the existence check CheckRunIDExists runs on a presented
// --run-id for agent.enter. It resolves the id to exactly one run record under
// gitCommonDir's run registry through findRunDirByID and returns its typed
// RunError (not-found, ambiguous, IO) unchanged. It checks RESOLVABILITY only,
// never liveness.
func runIDLocator(gitCommonDir string) func(string) error {
	runTrackerRoot := filepath.Join(gitCommonDir, "docket", runTrackerDirName)
	return func(runID string) error {
		_, _, err := findRunDirByID(runTrackerRoot, runID)
		return err
	}
}

// CheckRunIDExists verifies, before agent.enter spawns anything, that a lone
// --run-id (presented without --run-key) resolves to exactly one run in
// repoDir's repository (change 0463). It is a resolvability check
// (runIDLocator / findRunDirByID): it never checks liveness, and it returns
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
// presented (--run-key, --run-id) pair names a real run (change 0463).
// It returns nil when the run key's run record carries exactly runID, and
// otherwise ALWAYS a *RunError:
//   - a run key with no directory, a malformed key, or no run record:
//     ErrRunNotFound (the pair names no run);
//   - a different recorded id: ErrRunIDMismatch;
//   - a corrupt record: keeps ErrRunRecordCorrupt;
//   - any other resolution fault: ErrRunRecordIO.
//
// It only reads. It never checks liveness, because participant registration still
// refuses a non-active run.
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
