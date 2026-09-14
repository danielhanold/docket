package gatedrive

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
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

// HistoryCleanupRequest selects the manual assessment's scope. An empty DriveID
// scans the whole registry in ascending id order; a non-empty DriveID must
// validate (a traversal or malformed id is refused before anything is scanned).
// DryRun previews without writing any abandoned marker.
type HistoryCleanupRequest struct {
	DriveID string
	DryRun  bool
}

// HistoryCleanupOutcome reports every candidate with its class and reason. It is
// a report, never a refusal — a mixed outcome still returns, and Retained > 0
// means blockers remain visible rather than complete recovery. Checked counts
// every real (readable or unreadable) record assessed; a record-less directory
// and an unrecognised registry entry are NOT counted, mirroring the
// first-admission inventory. The four class counters partition Findings by class.
type HistoryCleanupOutcome struct {
	Findings    []LegacyFinding
	Checked     int
	Recovered   int
	Recoverable int
	Retained    int
	Nonblocking int
}

// cleanupHistory is the shared MANUAL recovery assessment behind
// Driver.CleanupHistory. It mirrors inventoryLegacyDrives' enumeration — a sorted
// os.ReadDir(s.root), an absent root treated as empty, a record-less directory
// skipped, an unrecognised entry recorded as a retained finding with an empty
// DriveID and the bounded reason "unrecognized entry in the drive registry" — but
// with requestedWorktree=="" (worktree resolution is never required) and
// apply=!DryRun, and it NEVER refuses: every candidate lands in Findings and the
// counts summarise them. A non-empty DriveID validates and assesses exactly that
// one record. It takes NO admission/scope/drive lock: it mutates no gate state; the
// only write is the process layer's own lock-guarded abandoned marker, through the
// seam under apply.
func (s *Store) cleanupHistory(req HistoryCleanupRequest, proc recoverySeam) (HistoryCleanupOutcome, error) {
	apply := !req.DryRun
	var out HistoryCleanupOutcome

	if req.DriveID != "" {
		// A malformed or traversal id is rejected before any path is constructed or
		// any directory is scanned.
		if err := validateID(req.DriveID); err != nil {
			return HistoryCleanupOutcome{}, err
		}
		if f, checked, present := s.assessLegacyRecord(req.DriveID, proc, apply); present {
			out.Findings = append(out.Findings, f)
			if checked {
				out.Checked++
			}
		}
		tallyCleanupClasses(&out)
		return out, nil
	}

	entries, err := os.ReadDir(s.root)
	if err != nil {
		// An absent registry root is not an error — there is simply no history to
		// assess. Any other read fault is surfaced (never masked as a clean absence).
		if errors.Is(err, fs.ErrNotExist) {
			return out, nil
		}
		return HistoryCleanupOutcome{}, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		id := entry.Name()
		if !entry.IsDir() || validateID(id) != nil {
			// A non-directory or invalid-name entry is not a readable drive. Its
			// arbitrary name never enters a diagnostic: record a retained finding that
			// carries no drive id and the bounded reason.
			out.Findings = append(out.Findings, LegacyFinding{DriveID: "", Class: LegacyRetained, Reason: "unrecognized entry in the drive registry"})
			continue
		}
		f, checked, present := s.assessLegacyRecord(id, proc, apply)
		if !present {
			continue
		}
		out.Findings = append(out.Findings, f)
		if checked {
			out.Checked++
		}
	}
	tallyCleanupClasses(&out)
	return out, nil
}

// assessLegacyRecord classifies one drive id for the manual cleanup pass. present
// is false for a record-less directory (a concurrent first-admission creation
// window, or a crashed-mid-creation directory) — skipped and never counted, exactly
// as the first-admission inventory does. checked reports whether the id resolved to
// a real (readable or unreadable) record that counts toward Checked; an unreadable
// record is a retained finding, never a free slot. The classifier is always run with
// requestedWorktree=="" so no worktree is ever resolved.
func (s *Store) assessLegacyRecord(id string, proc recoverySeam, apply bool) (f LegacyFinding, checked, present bool) {
	h, lerr := s.loadHistoricalDrive(id)
	if lerr != nil {
		if storeErrIs(lerr, ErrNotFound) {
			return LegacyFinding{}, false, false
		}
		reason := "unreadable record"
		if storeErrIs(lerr, ErrUnknownSchema) {
			reason = "unknown schema"
		}
		return LegacyFinding{DriveID: id, Class: LegacyRetained, Reason: reason}, true, true
	}
	return s.classifyLegacyDrive(h, "", proc, apply), true, true
}

// tallyCleanupClasses derives the per-class counters from the gathered findings, so
// the summary always partitions Findings exactly.
func tallyCleanupClasses(out *HistoryCleanupOutcome) {
	for _, f := range out.Findings {
		switch f.Class {
		case LegacyRecovered:
			out.Recovered++
		case LegacyRecoverable:
			out.Recoverable++
		case LegacyRetained:
			out.Retained++
		case LegacyNonblocking:
			out.Nonblocking++
		}
	}
}
