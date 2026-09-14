package gatedrive

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/danielhanold/docket/internal/process"
)

// recoverySeam is the single process-recovery predicate the classifier
// consults for HALTED history. *process.Service satisfies it (change 0428
// Task 1's ClassifyRun). Keeping it an interface here lets the classifier be
// exercised with a scripted seam without launching real processes.
type recoverySeam interface {
	ClassifyRun(runDir string, mark bool) (process.RecoveryEntry, error)
}

// Legacy classification classes — the four verdicts classifyLegacyDrive can
// return for one historical record.
const (
	LegacyNonblocking = "nonblocking"
	LegacyRecovered   = "recovered"
	LegacyRecoverable = "recoverable" // dry-run only: an applied run would recover it
	LegacyRetained    = "retained"
)

// LegacyFinding is one drive's assessment. Reason is a bounded token/phrase;
// DriveID is always a validated id (never an arbitrary directory name).
type LegacyFinding struct {
	DriveID string `json:"drive_id"`
	Class   string `json:"class"`
	Reason  string `json:"reason"`
}

// LegacyHistorySummary is the compact recovery summary carried on successful
// AND refused starts when legacy history was relevant.
type LegacyHistorySummary struct {
	Checked   int             `json:"checked"`
	Recovered []string        `json:"recovered,omitempty"`
	Retained  []LegacyFinding `json:"retained,omitempty"`
}

// historicalSchemaV2 is the pre-0375 drive schema this repository's history
// can contain. It is readable for HISTORICAL ASSESSMENT ONLY — never loaded
// into the executable state machine, never migrated, never re-written.
const historicalSchemaV2 = 2

// historicalDrive is the bounded, assessment-only view of one persisted drive,
// readable across the historical schema range {2} plus the executable range
// {3,4}. It exposes exactly what classification needs — never command, env,
// credentials, or generations.
type historicalDrive struct {
	ID             string
	SchemaVersion  int
	RepoIdentity   string
	WorktreePath   string
	LastOutcome    Outcome
	RawRunDir      string
	PriorRawRunDir string
	RunRoot        string
}

// loadHistoricalDrive reads drive id for HISTORICAL ASSESSMENT ONLY. It first
// consults the executable reader (readStored) so a live-range v4/v3 record is
// viewed through the exact schema policy execution enforces; only an
// ErrUnknownSchema refusal falls through to the historical schema-2 decoder,
// which validates schema, outcome vocabulary, and required identity fields
// explicitly — a document is never trusted because Go zero-filled it. It
// grants NO new executable compatibility: readStored still refuses v2.
func (s *Store) loadHistoricalDrive(id string) (historicalDrive, error) {
	const op = "load-historical-drive"
	dir, err := s.driveDir(id)
	if err != nil {
		return historicalDrive{}, err
	}
	// Executable range first: v4/v3 load through the existing reader unchanged.
	stored, rerr := s.readStored(dir)
	if rerr == nil {
		return historicalView(id, stored.Record), nil
	}
	if !storeErrIs(rerr, ErrUnknownSchema) {
		return historicalDrive{}, rerr // corrupt / IO / not-found: unchanged
	}
	// Historical range: decode the raw envelope and validate schema 2
	// explicitly. Field presence is validated — a document is never trusted
	// because Go zero-filled it.
	buf, ferr := os.ReadFile(filepath.Join(dir, recordFileName))
	if ferr != nil {
		return historicalDrive{}, storeErr(ErrIO, op, ferr)
	}
	var raw struct {
		Record struct {
			SchemaVersion  int       `json:"schema_version"`
			RepoIdentity   string    `json:"repo_identity"`
			WorktreePath   string    `json:"worktree_path"`
			LastOutcome    Outcome   `json:"last_outcome"`
			RawRunDir      string    `json:"raw_run_dir"`
			PriorRawRunDir string    `json:"prior_raw_run_dir"`
			RunRoot        string    `json:"run_root"`
			StartedAt      time.Time `json:"started_at"`
		} `json:"record"`
	}
	if err := json.Unmarshal(buf, &raw); err != nil {
		return historicalDrive{}, storeErr(ErrCorruptRecord, op, err)
	}
	r := raw.Record
	if r.SchemaVersion != historicalSchemaV2 {
		return historicalDrive{}, storeErr(ErrUnknownSchema, op,
			fmt.Errorf("schema version %d is neither executable nor a supported historical schema", r.SchemaVersion))
	}
	switch r.LastOutcome {
	case "", WAITING, PASSED, FAILED, HALTED:
	default:
		return historicalDrive{}, storeErr(ErrCorruptRecord, op, fmt.Errorf("unknown outcome vocabulary"))
	}
	if r.RepoIdentity == "" || r.WorktreePath == "" || r.StartedAt.IsZero() {
		return historicalDrive{}, storeErr(ErrCorruptRecord, op, fmt.Errorf("required identity fields missing"))
	}
	return historicalDrive{ID: id, SchemaVersion: r.SchemaVersion, RepoIdentity: r.RepoIdentity,
		WorktreePath: r.WorktreePath, LastOutcome: r.LastOutcome, RawRunDir: r.RawRunDir,
		PriorRawRunDir: r.PriorRawRunDir, RunRoot: r.RunRoot}, nil
}

// historicalView projects an executable-range driveRecord onto the bounded
// assessment view, exposing only the fields classification consults.
func historicalView(id string, r driveRecord) historicalDrive {
	return historicalDrive{ID: id, SchemaVersion: r.SchemaVersion, RepoIdentity: r.RepoIdentity,
		WorktreePath: r.WorktreePath, LastOutcome: r.LastOutcome, RawRunDir: r.RawRunDir,
		PriorRawRunDir: r.PriorRawRunDir, RunRoot: r.RunRoot}
}

// classifyLegacyDrive assesses one historical record. requestedWorktree is the
// canonical root being admitted, or "" for a repository-wide (cleanup) pass.
// apply=true may write the existing process abandoned marker via the seam;
// apply=false previews (LegacyRecoverable instead of LegacyRecovered).
//
// The numbered step order is LOAD-BEARING and must not be reordered:
//
//  1. A supervisor-committed PASSED/FAILED outcome is durable evidence
//     recognised BEFORE any path resolution — a removed worktree or temp dir
//     cannot undo it, so it is nonblocking regardless of where its worktree now
//     resolves (or whether it resolves at all).
//  2. Only a not-conclusively-completed record establishes the worktree
//     binding: a valid binding to a DIFFERENT worktree is irrelevant to this
//     admission, but an unresolvable path is NOT proof of unrelatedness.
//  3. A terminal HALTED record is assessed through the process predicate, with
//     BOTH the current and prior recorded run dirs required to prove teardown; a
//     probe error or missing run evidence retains — a probe error is not clean
//     absence.
//  4. Any other nonterminal state (WAITING or empty) is never guessed dead.
func (s *Store) classifyLegacyDrive(h historicalDrive, requestedWorktree string, proc recoverySeam, apply bool) LegacyFinding {
	f := LegacyFinding{DriveID: h.ID}
	// 1. Trustworthy completed history is nonblocking BEFORE any path
	//    resolution: a supervisor-committed PASSED/FAILED outcome is durable
	//    evidence a removed worktree or temp dir cannot undo.
	if h.LastOutcome == PASSED || h.LastOutcome == FAILED {
		f.Class, f.Reason = LegacyNonblocking, "completed terminal outcome ("+string(h.LastOutcome)+")"
		return f
	}
	// 2. Not conclusively completed: establish the worktree binding. A valid
	//    binding to a DIFFERENT worktree is irrelevant to this admission; an
	//    unresolvable path is NOT proof of unrelatedness or teardown.
	if requestedWorktree != "" {
		if legacyRoot, _, err := s.admissionKeyFor(h.WorktreePath, "inventory-legacy-drive"); err == nil && legacyRoot != requestedWorktree {
			f.Class, f.Reason = LegacyNonblocking, "bound to a different worktree"
			return f
		}
	}
	// 3. Terminal HALTED: one exact-run recovery assessment through the existing
	//    process predicate — both recorded attempts must prove teardown. A probe
	//    error or missing evidence retains.
	if h.LastOutcome == HALTED {
		if proc == nil || h.RawRunDir == "" {
			f.Class, f.Reason = LegacyRetained, "halted with no assessable run evidence"
			return f
		}
		runs := []string{h.RawRunDir}
		if h.PriorRawRunDir != "" {
			runs = append(runs, h.PriorRawRunDir)
		}
		recovered := false
		for _, runDir := range runs {
			entry, err := proc.ClassifyRun(runDir, apply)
			if err != nil {
				f.Class, f.Reason = LegacyRetained, "process assessment failed; evidence unprovable"
				return f
			}
			switch entry.Disposition {
			case "terminal", "stopped", "already-abandoned":
				// durable teardown evidence already present
			case "abandoned-marked":
				recovered = true
			case "abandonable": // apply=false preview
				recovered = true
			default: // live, needs-inspection, invalid, foreign, unresolved-establishment
				f.Class, f.Reason = LegacyRetained, "halted run not provably torn down ("+entry.Disposition+")"
				return f
			}
		}
		if recovered {
			if apply {
				f.Class, f.Reason = LegacyRecovered, "abandoned marker recorded from provable group absence"
			} else {
				f.Class, f.Reason = LegacyRecoverable, "provable group absence; an applied run would record the marker"
			}
		} else {
			f.Class, f.Reason = LegacyNonblocking, "halted with durable teardown evidence"
		}
		return f
	}
	// 4. Live/nonterminal (WAITING or empty outcome): never guessed dead.
	f.Class, f.Reason = LegacyRetained, "nonterminal execution state"
	return f
}
