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
// refuses an unknown schema version with a typed error rather than best-effort
// migrating it, so this is bumped only on a real schema change. Bumped to 2 by
// change 0359, which adds ScopeID + GateContextHash; a v1 record read by a v2
// store fails closed as ErrUnknownSchema (never migrated).
const driveSchemaVersion = 2

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

// The HALTED cause tokens a workflow consumer distinguishes when it maps a
// driver document onto its own vocabulary. A consumer (e.g. the finalize local
// gate's mapDriveHaltCause) MUST switch on these exported constants, never on
// the literal spellings, so a rename of a token here is a COMPILE error at every
// consumer rather than a silent reclassification (repo rule: key a guard on a
// typed identity, never an enumerated list of spellings). These are the single
// source for the tokens; the driver's emission sites reference them too. Other
// emitted causes (owner-superseded, fingerprint-error, uncertain-ownership, …)
// are not distinguished by any consumer — they fall through a consumer's default
// — so they need no exported identity here.
const (
	// CauseSchemaMismatch: the persisted record's schema version is unknown or the
	// record is corrupt — a fail-closed halt on an unusable record.
	CauseSchemaMismatch = "schema-mismatch"
	// CauseObservationUnreadable: the native run observation could not be read.
	CauseObservationUnreadable = "observation-unreadable"
	// CauseUnknownObservation: the native run reported an unrecognized state.
	CauseUnknownObservation = "unknown-observation"
	// CauseDeadlineExpired: the observation budget expired with a live run. It is
	// the running-at-budget analog; a consumer matches it as a PREFIX because a
	// variant (deadline-expired-stop-unproven) extends it.
	CauseDeadlineExpired = "deadline-expired"
	// CauseTakeoverAmbiguous: an event-authorized takeover resolved MORE THAN ONE
	// candidate drive for one scope — the recovery target is ambiguous, so it
	// fails closed rather than guessing which live run to supersede. (change 0359)
	CauseTakeoverAmbiguous = "takeover-ambiguous"
	// CauseTakeoverNoCandidate: a takeover resolved ZERO candidate drives for a
	// scope with no bound drive — there is no live or unconsumed work to recover,
	// so it fails closed rather than transferring nothing. (change 0359)
	CauseTakeoverNoCandidate = "takeover-no-candidate"
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
	// RunRoot is the drive's private process-supervisor allocation root (the
	// parent of the raw run dir(s)). It is populated on a TERMINAL document only
	// (PASSED/FAILED/HALTED, omitempty) and never on WAITING — a live drive may
	// still relaunch under it, so a WAITING consumer must retain it. It is exposed
	// for exactly one purpose: the owning caller that minted the root removes it at
	// the terminal to avoid leaking one temp dir per drive across retries. Like
	// RawRunDir it is a host path, not a secret; it carries no argv/env/credential.
	RunRoot string `json:"run_root,omitempty"`
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

	// Repository execution-identity fingerprint: per-dimension hashes and
	// structural counts only, never file/diff content (see fingerprint.go). The
	// driver recomputes it at every ownership boundary and before accepting a
	// terminal pass; any drift HALTs rather than going red.
	Fingerprint Fingerprint `json:"fingerprint"`

	// Resolved command + working directory. Authoritative config resolves these
	// — never agent input.
	Command []string `json:"command"`
	Cwd     string   `json:"cwd"`

	// RunRoot is the native process-supervisor allocation root (the raw
	// LaunchRequest.Root). It is the deterministic launch input the one admitted
	// relaunch replays, so it is persisted with the rest of the launch identity.
	RunRoot string `json:"run_root"`

	// IdempotentSuiteGate marks a workflow suite gate the application contract
	// designates idempotent. It is condition 1 of the single relaunch: only such
	// a gate may earn a second raw run after a proven death.
	IdempotentSuiteGate bool `json:"idempotent_suite_gate"`

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

	// PriorRawRunDir links the dead first attempt after the one admitted
	// relaunch, so both attempts' diagnostics are preserved (spec: "A relaunch
	// preserves both attempts").
	PriorRawRunDir string `json:"prior_raw_run_dir,omitempty"`

	// LastOutcome + LastCause record the last transition the driver persisted.
	// A terminal LastOutcome (PASSED/FAILED/HALTED) makes the drive idempotent:
	// re-advancing returns the recorded verdict rather than re-driving the run.
	// WAITING is nonterminal and drives again. These are private runtime state,
	// never change frontmatter.
	LastOutcome Outcome `json:"last_outcome,omitempty"`
	LastCause   string  `json:"last_cause,omitempty"`

	// Current owner generation, or the single-use handoff generation when the
	// drive is offered for claim (Task 5). Exactly one owner at a time.
	OwnerGeneration   string `json:"owner_generation"`
	HandoffGeneration string `json:"handoff_generation,omitempty"`

	// ScopeID links the drive to the recovery scope its owner was dispatched
	// under; GateContextHash links every nested drive to the outer gate
	// (sha256 of the outer child-context token). Both empty for scopeless
	// drives (e.g. finalize's local gate). (schema v2, change 0359)
	ScopeID         string `json:"scope_id,omitempty"`
	GateContextHash string `json:"gate_context_hash,omitempty"`
}
