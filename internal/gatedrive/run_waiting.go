// Read-only receipt projection for the local run-waiting verdict.
//
// The run-verification reporter (internal/app/run_verify.go) derives a
// `run-waiting` verdict EXCLUSIVELY from agreeing local receipts. This file
// exposes the two gate-drive reads that reporter needs without leaking the
// private drive record: FindWaitingReceipt locates the single drive for a change
// that carries an explicit unclaimed handoff and projects it to a redaction-safe
// DriveReceipt, and ComputeLiveFingerprint recomputes the current worktree
// execution identity so the reporter can prove it has not drifted from the
// drive-start receipt.
//
// Neither surface exposes an owner credential, the launch argv, environment
// values, or worktree content — only per-dimension identity hashes, structural
// booleans, and the opaque drive locator — so a read-only reporter can compose
// them safely.
package gatedrive

import (
	"errors"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"time"
)

// DriveReceipt is the redaction-safe projection of one drive record for the
// run-waiting reporter. It carries the drive-start identity, the current
// raw-run/ownership references, the fixed deadline, and the last recorded
// terminal outcome — never an owner generation, the handoff generation, the
// resolved command, or any worktree content.
type DriveReceipt struct {
	// DriveID is the opaque drive directory id — the handoff locator the verdict
	// exposes.
	DriveID string
	// HasUnclaimedHandoff is true only when the record carries an explicit
	// unclaimed handoff (owner invalidated, handoff generation present).
	HasUnclaimedHandoff bool

	// Work identity — the change/task/phase chain the drive certifies.
	ChangeID string
	TaskID   string
	Phase    string

	// Branch and worktree the drive is bound to.
	Branch       string
	WorktreePath string

	// Drive-start HEAD and full execution-identity fingerprint.
	Head        string
	Fingerprint Fingerprint

	// The fixed-once deadline and the last recorded terminal outcome (empty or
	// WAITING when nonterminal). A PASSED/FAILED outcome is a durable terminal
	// result waiting to be consumed.
	Deadline    time.Time
	LastOutcome Outcome

	// Current raw run directory and its native ownership id for the active attempt.
	RawRunDir    string
	RawOwnership string
	Attempt      int
}

// FindWaitingReceipt returns the single drive for changeID that carries an
// explicit unclaimed handoff, projected to a redaction-safe DriveReceipt. It
// returns found=false — never an invented waiting — when the change id does not
// parse, the drive root does not exist, no drive matches, or MORE THAN ONE
// matching drive carries an unclaimed handoff (an ambiguous chain). A record that
// will not load (unknown schema, corrupt, invalid id) is skipped rather than
// failing the scan: an unreadable record is not evidence of a safe continuation.
// Only a real filesystem fault reading the root is returned as an error.
func (s *Store) FindWaitingReceipt(changeID string) (DriveReceipt, bool, error) {
	want, err := strconv.Atoi(strings.TrimSpace(changeID))
	if err != nil {
		return DriveReceipt{}, false, nil
	}

	entries, err := os.ReadDir(s.root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// No drive root at all: missing local state, never waiting.
			return DriveReceipt{}, false, nil
		}
		return DriveReceipt{}, false, storeErr(ErrIO, "find-waiting", err)
	}

	var match *DriveReceipt
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		rec, lerr := s.Load(e.Name())
		if lerr != nil {
			// Unreadable/unknown/invalid record: cannot certify a continuation from
			// it, and it is not this scan's job to HALT — skip it.
			continue
		}
		n, cerr := strconv.Atoi(strings.TrimSpace(rec.ChangeID))
		if cerr != nil || n != want {
			continue
		}
		// An explicit UNCLAIMED handoff: owner invalidated, handoff generation set.
		if rec.OwnerGeneration != "" || rec.HandoffGeneration == "" {
			continue
		}
		if match != nil {
			// More than one candidate for the same change: the chain is ambiguous.
			return DriveReceipt{}, false, nil
		}
		dr := receiptOf(e.Name(), rec)
		match = &dr
	}
	if match == nil {
		return DriveReceipt{}, false, nil
	}
	return *match, true, nil
}

// receiptOf projects a loaded drive record to the redaction-safe DriveReceipt.
// It is only reached for a record already filtered to an unclaimed handoff, so
// HasUnclaimedHandoff is true by construction.
func receiptOf(id string, rec driveRecord) DriveReceipt {
	return DriveReceipt{
		DriveID:             id,
		HasUnclaimedHandoff: true,
		ChangeID:            rec.ChangeID,
		TaskID:              rec.TaskID,
		Phase:               rec.Phase,
		Branch:              rec.Branch,
		WorktreePath:        rec.WorktreePath,
		Head:                rec.HeadOID,
		Fingerprint:         rec.Fingerprint,
		Deadline:            rec.Deadline,
		LastOutcome:         rec.LastOutcome,
		RawRunDir:           rec.RawRunDir,
		RawOwnership:        rec.RawOwnership,
		Attempt:             rec.Attempt,
	}
}

// FindScopeDriveIDs lists the drive ids for changeID whose GateContextHash equals
// gateContextHash and whose LastOutcome is nonterminal OR terminal-unconsumed
// (terminal with a still-set OwnerGeneration — a child that wrote a verdict then
// died before the parent consumed it). A terminal-AND-consumed drive (owner
// cleared) is excluded: there is nothing left to recover. A record that will not
// load (unknown schema, corrupt, invalid id) is SKIPPED rather than failing the
// scan — an unreadable record is not evidence of a recoverable drive. Only a real
// filesystem fault reading the root is returned as an error. It is the outer
// gate's candidate resolver: exactly one match authorizes an outer takeover
// (takeover.go); zero or many fail closed upstream.
func (s *Store) FindScopeDriveIDs(changeID, gateContextHash string) ([]string, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// No drive root at all: no candidates, never an error.
			return nil, nil
		}
		return nil, storeErr(ErrIO, "find-scope-drives", err)
	}
	var ids []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		rec, lerr := s.Load(e.Name())
		if lerr != nil {
			// Unreadable/unknown/invalid record: skip it, never HALT the scan.
			continue
		}
		if !changeIDsEqual(rec.ChangeID, changeID) {
			continue
		}
		if rec.GateContextHash != gateContextHash {
			continue
		}
		// Include a nonterminal (still live) drive, and a terminal drive whose owner
		// generation is still set (a verdict written but not yet consumed). Exclude a
		// terminal drive whose owner was cleared: it was already consumed.
		if isTerminalOutcome(rec.LastOutcome) && rec.OwnerGeneration == "" {
			continue
		}
		ids = append(ids, e.Name())
	}
	return ids, nil
}

// changeIDsEqual compares two change ids tolerantly: trimmed string equality, or
// numeric equality when both parse as integers (so "0342" and "342" match, the
// same normalization FindWaitingReceipt applies to a change id).
func changeIDsEqual(a, b string) bool {
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	if a == b {
		return true
	}
	na, ea := strconv.Atoi(a)
	nb, eb := strconv.Atoi(b)
	return ea == nil && eb == nil && na == nb
}

// ContinuationHandle returns the CURRENT unclaimed handoff token of driveID, for
// in-process facade use only (the outer gate synthesizes a normal handoff and
// then reads it back to hand to the resumed controller). The token is NEVER
// emitted in a document or human text. It fails with a typed ErrNoHandoffOffered
// when the drive carries no unclaimed handoff (owner still set, or handoff
// already consumed).
func (s *Store) ContinuationHandle(driveID string) (string, error) {
	rec, err := s.Load(driveID)
	if err != nil {
		return "", err
	}
	if rec.OwnerGeneration != "" || rec.HandoffGeneration == "" {
		return "", ownershipErr(ErrNoHandoffOffered, "continuation-handle")
	}
	return rec.HandoffGeneration, nil
}

// ComputeLiveFingerprint recomputes the execution-identity fingerprint of the
// worktree at path using the production git seam, so a read-only caller can prove
// the live bytes still match a drive-start receipt. It follows no symlink and
// retains no content (see ComputeFingerprint).
func ComputeLiveFingerprint(worktree string) (Fingerprint, error) {
	return ComputeFingerprint(worktree, realGit{})
}
