package app

// This file is the run refusal vocabulary (change 0463; key-only since change 0491).
// agent.enter's optional --run-key names a run; when the presented key cannot be
// resolved, the caller learns WHICH mistake it made through a stable token — the
// run registry error's own kind. Tokens are a fixed vocabulary; nothing here echoes
// the presented value, a path, or record content.

// ClassifyRunRecordError maps a run registry failure (a *RunError anywhere in
// err's chain) to a protocol result and a bounded reason token, the error's own
// kind: a corrupt or unreadable record is internal-error; every other
// readable-but-unusable state (run-not-found, run-record-conflict, run-not-active,
// …) is invalid-input. ok is false when err carries no *RunError, so callers fall
// through to their own classification.
func ClassifyRunRecordError(err error) (Result, string, bool) {
	ee, ok := AsRunError(err)
	if !ok {
		return "", "", false
	}
	switch ee.Kind {
	case ErrRunRecordCorrupt, ErrRunRecordIO:
		return ResultInternalError, string(ee.Kind), true
	default:
		return ResultInvalidInput, string(ee.Kind), true
	}
}

// RunRecordNextAction maps a run refusal reason to a one-line, credential-free next
// action (the ownershipNextAction pattern). It never echoes the presented value. A
// reason with no specific remedy yields "", and callers then omit the message.
func RunRecordNextAction(reason string) string {
	switch reason {
	case string(ErrRunNotFound):
		return "the --run-key value names no run in this repository; pass the run key run.start printed, " +
			"never the run context (that goes to --run-context on change claim and the gate drive)"
	default:
		return ""
	}
}

// CheckRunKey verifies, before agent.enter spawns anything, that --run-key names a
// run in repoDir's repository (change 0491; it replaces change 0463's
// linkage check, which matched a separate locator the run no longer carries). It returns nil when the key's run record loads and otherwise
// ALWAYS a *RunError: ErrRunNotFound for a key with no directory, a malformed key,
// or no run record; ErrRunRecordCorrupt for a corrupt record; ErrRunRecordIO for
// any other fault. It only reads and never checks liveness: participant
// registration still refuses a non-active run.
func CheckRunKey(repoDir, runKey string) error {
	_, _, err := LoadRunRecord(repoDir, runKey)
	if err == nil {
		return nil
	}
	if _, ok := AsRunError(err); ok {
		return err
	}
	if ge, ok := AsRunTrackerStoreError(err); ok && (ge.Kind == ErrRunTrackerNotFound || ge.Kind == ErrRunTrackerMalformedKey) {
		return runErr(ErrRunNotFound, "check-run-key", nil)
	}
	return runErr(ErrRunRecordIO, "check-run-key", err)
}
