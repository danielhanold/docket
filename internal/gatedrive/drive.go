// Package gatedrive holds the native gate driver: a versioned drive state
// machine that composes internal/process to advance a workflow suite run in
// short slice-bounded synchronous calls behind one persisted deadline and
// execution identity. This file establishes the two foundational contracts the
// rest of the package builds on: the typed Outcome vocabulary and the
// protocol-v1 DriveDoc that every caller (CLI, app seam, tests) reads, plus the
// private driveRecord persisted schema.
//
// Canonical JSON follows the repo's protocol-v1 envelope convention (see
// internal/app.Envelope and canonicalDigest): a closed struct with explicit
// json tags, so json.Marshal serializes in declaration order with no
// insignificant whitespace. No custom MarshalJSON is needed or wanted.
package gatedrive

import "time"

// ProtocolVersion is the gate-drive protocol generation. It matches
// internal/app.ProtocolVersion (1) for this generation; a later generation that
// removes, renames, or retypes a DriveDoc field bumps it.
const ProtocolVersion = 1

// driveSchemaVersion is the persisted driveRecord schema generation. The store
// (Task 4) refuses an unknown schema version with a typed error rather than
// best-effort migrating it, so this is bumped only on a real schema change.
const driveSchemaVersion = 1

// Outcome is the four-way typed result of a single slice-bounded driver call.
// It is the sole vocabulary a workflow caller keys on; the raw process state is
// never surfaced to a workflow.
type Outcome string

const (
	// WAITING: the owned run is still live after one observation slice. No
	// shell monitor, sleep, or notification is created — the caller carries the
	// opaque continuation and re-enters.
	WAITING Outcome = "WAITING"
	// PASSED: the suite itself completed green AND the repository fingerprint
	// still matches the drive-start identity. Only PASSED exposes the raw run
	// dir so trusted evidence can be minted from it.
	PASSED Outcome = "PASSED"
	// FAILED: the suite itself completed red. Process death, malformed state,
	// deadline expiry, identity uncertainty, and handoff mismatch are NEVER
	// converted into FAILED.
	FAILED Outcome = "FAILED"
	// HALTED: a fail-closed terminal — identity drift, uncertain ownership,
	// deadline expiry, malformed state, or an unadmitted death. Never red.
	HALTED Outcome = "HALTED"
)

// DriveDoc is the protocol-v1 outcome document emitted by every driver
// operation, shared verbatim by the CLI, the app service seam, and tests. It is
// a diagnostic surface: it carries bounded execution identity, the typed
// outcome, and an optional typed cause — never the launch argv, environment
// values, worktree diff, file contents, or any ownership credential. RawRunDir
// is populated on PASSED only (omitempty), so a non-PASSED doc cannot expose it.
type DriveDoc struct {
	ProtocolVersion int       `json:"protocol_version"`
	DriveID         string    `json:"drive_id,omitempty"`
	Generation      string    `json:"generation,omitempty"`
	Attempt         int       `json:"attempt,omitempty"`
	Deadline        time.Time `json:"deadline"`
	Outcome         Outcome   `json:"outcome"`
	Cause           string    `json:"cause,omitempty"`
	RawRunDir       string    `json:"raw_run_dir,omitempty"`
}

// driveRecord is the durable, owner-private persisted schema of one drive. It is
// never emitted to a workflow — unlike DriveDoc it retains the resolved command,
// config provenance, and identity hashes the state machine needs across process
// restarts and ownership boundaries. Every field carries an explicit snake_case
// json tag so the store round-trips it canonically (Task 4). SchemaVersion is
// stamped so an unknown schema is refused, never migrated.
//
// The field groups below transcribe the spec's "Persisted execution identity":
// repo identity, worktree path, change/task/phase identity, branch/ref + full
// HEAD OID, fingerprint, resolved command + cwd, config provenance + budget, env
// hash, timestamps + fixed deadline + last-accepted clock + protocol version,
// current raw run dir + raw ownership identity + attempt + relaunch count +
// terminal receipt, and current owner generation or single-use handoff
// generation. Later tasks (clock, fingerprint, ownership, state machine) refine
// the concrete field types they own; this is the foundational schema.
type driveRecord struct {
	SchemaVersion int `json:"schema_version"`

	// Repository identity.
	RepoIdentity string `json:"repo_identity"`
	WorktreePath string `json:"worktree_path"`

	// Change/task/phase identity.
	ChangeID string `json:"change_id"`
	TaskID   string `json:"task_id"`
	Phase    string `json:"phase"`

	// Branch/ref + full HEAD object id.
	Branch  string `json:"branch"`
	Ref     string `json:"ref"`
	HeadOID string `json:"head_oid"`

	// Repository execution-identity fingerprint. Stored as the canonical
	// per-dimension hash digest (never file/diff content); the Fingerprint type
	// and its equality land in Task 3, which refines this field's concrete
	// shape.
	Fingerprint string `json:"fingerprint"`

	// Resolved command + working directory. Authoritative config resolves these
	// — never agent input.
	Command []string `json:"command"`
	Cwd     string   `json:"cwd"`

	// Config provenance + resolved observation budget.
	ConfigProvenance string        `json:"config_provenance"`
	Budget           time.Duration `json:"budget"`

	// Environment hash — a digest of the launch environment, never its values.
	EnvHash string `json:"env_hash"`

	// Timestamps + fixed-once deadline + last-accepted clock + protocol version.
	// The deadline is computed once at Start and never extended (Task 2).
	StartedAt       time.Time `json:"started_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	Deadline        time.Time `json:"deadline"`
	LastClock       time.Time `json:"last_clock"`
	ProtocolVersion int       `json:"protocol_version"`

	// Current raw run dir + raw ownership identity + attempt + relaunch count +
	// terminal receipt. At most one owned raw tree is live per drive.
	RawRunDir       string `json:"raw_run_dir"`
	RawOwnership    string `json:"raw_ownership"`
	Attempt         int    `json:"attempt"`
	RelaunchCount   int    `json:"relaunch_count"`
	TerminalReceipt string `json:"terminal_receipt"`

	// Current owner generation, or the single-use handoff generation when the
	// drive is offered for claim (Task 5). Exactly one owner at a time.
	OwnerGeneration   string `json:"owner_generation"`
	HandoffGeneration string `json:"handoff_generation,omitempty"`
}
