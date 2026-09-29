package app

import "path/filepath"

// This file is the run-epoch refusal vocabulary (change 0463). The run epoch is a
// public locator (ADR-0111) that a caller threads into --run-epoch flags (gate drive
// start, gate drive prepare-scope, agent.enter). When the presented value cannot be
// resolved, the caller must learn WHICH mistake it made through a stable token. A
// catch-all invalid-request makes a misrouted token (0382: the dispatch context
// passed as the epoch) indistinguishable from a malformed request. Tokens are a
// fixed vocabulary; nothing here echoes the presented value, a path, or record
// content.

// ReasonUnknownRunEpoch is the stable refusal token for a --run-epoch that names no
// run epoch in this repository.
const ReasonUnknownRunEpoch = "unknown-run-epoch"

// ClassifyRunEpochError maps a run-epoch registry failure (an *EpochError anywhere
// in err's chain) to a protocol result and a bounded reason token:
//   - not-found: unknown-run-epoch.
//   - mismatch: the existing stale-linkage token, stale-run-epoch.
//   - corrupt or unreadable: internal-error carrying the kind.
//   - any other readable-but-unusable registry state: invalid-input carrying the kind.
//
// ok is false when err carries no *EpochError, so callers fall through to their
// own classification.
func ClassifyRunEpochError(err error) (Result, string, bool) {
	ee, ok := AsEpochError(err)
	if !ok {
		return "", "", false
	}
	switch ee.Kind {
	case ErrEpochNotFound:
		return ResultInvalidInput, ReasonUnknownRunEpoch, true
	case ErrEpochMismatch:
		return ResultInvalidInput, ErrStaleRunEpoch.Reason, true
	case ErrEpochCorrupt, ErrEpochIO:
		return ResultInternalError, string(ee.Kind), true
	default:
		return ResultInvalidInput, string(ee.Kind), true
	}
}

// RunEpochNextAction maps a run-epoch refusal reason to a one-line, credential-free
// next action (the ownershipNextAction / fenceNextAction pattern). It never echoes
// the presented value. A reason with no specific remedy yields "", and callers then
// omit the message.
func RunEpochNextAction(reason string) string {
	switch reason {
	case ReasonUnknownRunEpoch:
		return "the --run-epoch value names no run epoch in this repository; pass the <epoch> field of the arm's " +
			"`gate-armed <key> <epoch> <dispatch-context>` line (the <dispatch-context> goes to --gate-context) — " +
			"never drop --run-epoch and retry"
	case ErrStaleRunEpoch.Reason:
		return "the --run-epoch value is not the run epoch this gate key carries; pass the <epoch> printed on the same gate-armed line as the key"
	default:
		return ""
	}
}

// runEpochLocator builds the existence check prepare-scope runs on a presented
// --run-epoch. It resolves the id to exactly one epoch record under gitCommonDir's
// run-epoch registry through findEpochDirByID (the same locator the epoch launch
// gate uses) and returns its typed EpochError (not-found, ambiguous, IO) unchanged.
// It checks RESOLVABILITY only, never liveness: a scope may legitimately carry a
// cancelled epoch (the takeover revocation gate reads it later), and the launch gate
// still enforces liveness and worktree ownership at start.
func runEpochLocator(gitCommonDir string) func(string) error {
	rungateRoot := filepath.Join(gitCommonDir, "docket", runTrackerDirName)
	return func(epochID string) error {
		_, _, err := findEpochDirByID(rungateRoot, epochID)
		return err
	}
}

// CheckRunEpochExists verifies, before agent.enter spawns anything, that a lone
// --run-epoch (presented without --run-gate-key) resolves to exactly one run epoch in
// repoDir's repository (change 0463). It is the same resolvability check prepare-scope
// runs (runEpochLocator / findEpochDirByID): it never checks liveness, and it returns
// the locator's typed *EpochError (not-found, ambiguous, IO) unchanged. A repository
// whose git common dir cannot be resolved yields ErrEpochIO.
func CheckRunEpochExists(repoDir, epochID string) error {
	common, err := gateGitCommonDir(repoDir)
	if err != nil {
		return epochErr(ErrEpochIO, "check-exists", err)
	}
	return runEpochLocator(common)(epochID)
}

// CheckRunEpochLinkage verifies, before agent.enter spawns anything, that the
// presented (--run-gate-key, --run-epoch) pair names a real run epoch (change 0463).
// It returns nil when the gate key's epoch record carries exactly epochID, and
// otherwise ALWAYS an *EpochError:
//   - a gate key with no directory, a malformed key, or no epoch record:
//     ErrEpochNotFound (the pair names no epoch);
//   - a different recorded id: ErrEpochMismatch;
//   - a corrupt record: keeps ErrEpochCorrupt;
//   - any other resolution fault: ErrEpochIO.
//
// It only reads. It never checks liveness, because participant registration still
// refuses a non-active epoch.
func CheckRunEpochLinkage(repoDir, gateKey, epochID string) error {
	rec, _, err := LoadEpochRecord(repoDir, gateKey)
	if err != nil {
		if _, ok := AsEpochError(err); ok {
			return err
		}
		if ge, ok := AsGateStoreError(err); ok && (ge.Kind == ErrGateNotFound || ge.Kind == ErrGateMalformedKey) {
			return epochErr(ErrEpochNotFound, "check-linkage", nil)
		}
		return epochErr(ErrEpochIO, "check-linkage", err)
	}
	if rec.EpochID != epochID {
		return epochErr(ErrEpochMismatch, "check-linkage", nil)
	}
	return nil
}
