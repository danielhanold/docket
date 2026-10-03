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
// change 0359, which adds scope_id + RunContextHash; a v1 record read by a v2
// store fails closed as ErrUnknownSchema (never migrated). Bumped to 3 by change
// 0375, which adds AdmissionToken (the launch token threaded into the raw
// launch; minted by the driver itself since change 0490). Bumped to 4 by change
// 0375 Task 5, which added a relaunch-reservation journal. Change 0493 retired
// the relaunch and stopped writing its fields (idempotent_suite_gate,
// relaunch_count, relaunch_reserved, relaunch_token, prior_raw_run_dir) without
// a version bump: a record that still carries them loads with them ignored, and
// its next write drops them. A v3 record still LOADS (see
// driveSchemaVersionLegacy in readStored) and the next write stamps it forward
// to v4; older records still fail closed.
//
// Upgrade boundary (change 0428): a retired schema below the executable range is
// still readable by the HISTORICAL reader (loadHistoricalDrive) — but ONLY for
// the launch census (reconcile.go). Schema 2 (the immediately-pre-0375
// generation) is the sole historical schema recognised today
// (historicalSchemaV2, history.go). A
// historical record is never loaded into the executable state machine, never
// migrated, and never re-written: the execution reader (readStored) keeps
// accepting exactly v4 + v3 and fails a v2 record closed as ErrUnknownSchema, so
// no execution path gains schema-2 compatibility. The historical range grows only
// by an explicit decision at each future schema bump (whether the retired version
// joins it); there is no automatic retirement lifecycle. No claim is made that an
// OLD binary launched concurrently over the same store honors the new admission
// rules — the boundary governs how THIS binary reads retired history, not how a
// prior binary behaves.
const driveSchemaVersion = 4

// driveSchemaVersionLegacy is the immediately-prior schema generation a v4 store
// still reads (never writes). A v3 record carries every field a v4 reader needs.
// Only the immediately-prior generation is tolerated; v2 and any other version
// still fail closed as ErrUnknownSchema.
const driveSchemaVersionLegacy = 3

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
	// HALTED: a fail-closed terminal — a changed worktree, uncertain ownership,
	// deadline expiry, malformed state, or a supervisor death. Never red.
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
	// CauseTakeoverAmbiguous: the run tracker's outer scan found more than one
	// live candidate drive for one outer recovery scope — the recovery target is
	// ambiguous, so the outer continuation fails closed rather than guessing which
	// live run to supersede. (change 0359)
	CauseTakeoverAmbiguous = "takeover-ambiguous"
	// CauseSupervisorDied: the drive's run died without a verdict (signaled or
	// vanished) and no owned tree survives. A gate drive never relaunches (change
	// 0493): it HALTs, and a human re-runs the workflow, which re-runs the suite.
	CauseSupervisorDied = "supervisor-died"
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
	// parent of the raw run dir(s)). It is always exposed on a TERMINAL document
	// (PASSED/FAILED/HALTED, omitempty) and never on WAITING — a live drive's run
	// still writes under it, so a WAITING consumer must retain it. It is exposed
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
// current raw run dir + raw ownership identity + attempt + terminal receipt,
// and current owner generation or single-use handoff generation. Later tasks (clock, fingerprint, ownership, state machine) refine
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
	// LaunchRequest.Root). It is part of the deterministic launch input, so it is
	// persisted with the rest of the launch identity.
	RunRoot string `json:"run_root"`

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

	// Current raw run dir + raw ownership identity + attempt (always 1: a drive
	// never relaunches, change 0493) + terminal receipt. At most one owned raw
	// tree is live per drive.
	RawRunDir       string `json:"raw_run_dir"`
	RawOwnership    string `json:"raw_ownership"`
	Attempt         int    `json:"attempt"`
	TerminalReceipt string `json:"terminal_receipt"`

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

	// RunContextHash links a drive started inside a dispatched run to that run
	// (sha256 of the outer scope's child capability, the run context run.start
	// prints); the run tracker's outer takeover scan matches on it. Empty for a
	// drive started without a run context (e.g. finalize's local gate). (schema
	// v2, change 0359; the scope_id field it was added beside was dropped by change
	// 0489, and a pre-0489 record still carrying it decodes with it ignored.)
	RunContextHash string `json:"run_context_hash,omitempty"`

	// AdmissionToken is the launch token the driver mints at admission for the
	// drive's first launch (change 0490; the JSON key is unchanged from the slot
	// era, change 0375, when the worktree slot minted it). It is threaded into the
	// raw launch as LaunchRequest.ReservationToken so a lost launch response is
	// resolvable to this exact run (ResolveReservation), which the launch census
	// uses to settle a first launch whose run was never attached. It is never a
	// child capability. Empty on a legacy record that launched without one.
	// (schema v3)
	AdmissionToken string `json:"admission_token,omitempty"`
}
