package gatedrive

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

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
