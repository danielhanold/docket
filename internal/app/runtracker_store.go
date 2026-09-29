// The durable per-dispatch gate-record store (change 0334, Task 1). It is the
// Go generalization of scripts/lib/docket-dispatch-dir.sh's durable-dir
// conventions, holding the implement-next run tracker's attribution / retry state
// so a dispatch's gate outcome survives the launching process, a
// `git worktree remove`, and a restart.
//
// WHERE: <git-common-dir>/docket/run-tracker/<key>/record.json — the same family as
// the dispatch dir and the gate-drive store. Rooting under the git COMMON dir
// (not a worktree's .git) means the record sits outside every worktree yet stays
// reachable from any linked worktree of the same repository, is never tracked,
// and never leaks into a commit. An empty `git rev-parse --git-common-dir`
// answer is refused rather than passed on: `cd ""` succeeds and would silently
// resolve the root inside the worktree, the one property this location exists to
// prevent.
//
// KEY: implement-next-<UTC yyyymmddThhmmssZ, lowercased>-<pid>-<4 hex random>. It
// is a lookup token, never encoded state (spec: "Durable state"). It is
// lowercased so it satisfies the ^[a-z0-9-]+$ path-safety validator that every
// load applies BEFORE constructing a path — the safe charset excludes '/', '\\',
// and '.', so every traversal or absolute-path form is rejected as a pure string
// check, before any filesystem or git touch (the dispatch dir's safe-key rule).
//
// DURABILITY: writes go through os.CreateTemp in the record's OWN directory
// (same filesystem) followed by os.Rename, so a concurrent reader sees a whole
// old or new document, never a partial one — the atomic-adjacent-replacement
// rule.
//
// RETRY CAS: ConsumeRunTrackerRetry's exclusivity rests on os.OpenFile with
// O_CREATE|O_EXCL: the filesystem exclusive-create is the compare-and-swap, so of
// any number of concurrent callers exactly one creates the marker and returns
// true. Since change 0421 the permit is per-attempt: the marker for attempt n is
// retry-consumed-<n> (runTrackerRetryMarkerFor), and the counted budget grants at most
// AttemptLimit-1 markers over a record's life. The bare legacy retry-consumed name
// (schema v3's single permit) is read as the attempt-1 marker so an already-spent
// legacy permit can never be re-granted. Marker files remain the authority; the
// record's Retry field is a readable mirror flipped afterward, and LoadRunTrackerRecord
// reports consumed when ANY marker exists, so a crash between the two writes stays
// safe.
package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// runTrackerSchemaVersion is the on-disk record schema this store understands. A record
// carrying any other version fails closed as a corrupt record — never a
// best-effort migration. Bumped to 2 for change 0359: the record grows the outer
// recovery-scope binding (ScopeID/ParentCap/ChildContextHash) and the
// continuation triple. Bumped to 3 for change 0407: the record grows the
// claim-binding proof MIRROR fields (BoundRequestID/BoundRevision) and the store
// grows a per-key claim-binding file. Bumped to 4 for change 0421: the record
// grows the AttemptLimit snapshot (the run.max_attempts value captured at mint),
// and the single retry permit generalizes to a counted per-attempt marker budget
// (retry-consumed-<n>). A v3 (or older) record fails CLOSED here with the
// schema-mismatch diagnostic — a silent v3->v4 migration is deliberately rejected
// because an older run's consumed retry marker must NEVER be reinterpreted as
// unused configurable budget (a re-grant of an already-spent retry). The supported
// recovery is a newly started `run start --resume` for a still-valid in-progress
// change, exactly the 0407 precedent, never a migration that blesses old state.
const runTrackerSchemaVersion = 4

// Retry permit states recorded in a RunTrackerRecord. The one-retry permit is unused
// until ConsumeRunTrackerRetry spends it; the marker file, not this field, is the
// authority (see the package comment).
const (
	RetryUnused   = "unused"
	RetryConsumed = "consumed"
)

// recordFileName is the atomic record within a key directory; runTrackerRetryMarkerName
// is the bare legacy (schema v3) single-permit marker name — since change 0421 the
// per-attempt marker is runTrackerRetryMarkerFor(n) == "retry-consumed-<n>", and the
// bare name is read as the attempt-1 marker for legacy compatibility;
// runTrackerClaimBindingName is the bind-once claim-binding file whose os.Link
// hard-link create is the compare-and-swap serializing competing claim bindings
// (change 0407).
const (
	runTrackerRecordFileName   = "record.json"
	runTrackerRetryMarkerName  = "retry-consumed"
	runTrackerClaimBindingName = "claim-binding.json"
)

// The run tracker's local storage roots under <git-common-dir>/docket/
// (ADR-0129 row 38, change 0471). They were renamed by RESET, not migrated: the
// binary never reads the retired rungate/ and rungate-resume/ roots, which stay
// inert on disk and may be deleted by hand.
const (
	runTrackerDirName       = "run-tracker"
	runTrackerResumeDirName = "run-tracker-resume"
)

// runTrackerRetryMarkerPattern matches the retry-marker file shape RunTrackerRetryUsage counts:
// the bare legacy name and the per-attempt "retry-consumed-<n>" form (change 0421).
// It is anchored so no record.json / claim-binding.json / temp file can match.
var runTrackerRetryMarkerPattern = regexp.MustCompile(`^retry-consumed(-[0-9]+)?$`)

// runTrackerRetryMarkerFor names the per-attempt CAS marker: creating retry-consumed-<n>
// with O_CREATE|O_EXCL grants the single retry that moves attempt n to n+1. The
// bare legacy name (runTrackerRetryMarkerName, schema v3's single permit) is read as the
// attempt-1 marker so an already-consumed legacy permit can never be re-granted
// (change 0421).
func runTrackerRetryMarkerFor(attempt int) string {
	return fmt.Sprintf("%s-%d", runTrackerRetryMarkerName, attempt)
}

// bindingSchemaVersion is the on-disk schema of a RunTrackerClaimBinding. A binding
// carrying any other value fails closed as a corrupt record on load — never a
// best-effort migration (change 0407).
const bindingSchemaVersion = 1

// runTrackerRetentionEnv is the retention-window knob, shared with the dispatch dir's
// semantics (default 7 days; a non-numeric value disables age-pruning entirely).
const runTrackerRetentionEnv = "DOCKET_DISPATCH_RETENTION_DAYS"

// runKeyPattern is the path-safety validator every load applies before touching
// the filesystem. The charset excludes '/', '\\', and '.', so a traversal key
// cannot escape the root.
var runKeyPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

// runKeyMaxLen bounds a key length so a pathologically long token is refused
// before any path is built.
const runKeyMaxLen = 128

// RunTrackerRecord is the durable per-dispatch gate state. The key is a lookup token,
// never encoded state (spec: "Durable state"). Schema and Repo are store-owned
// identity fields: MintRunTrackerRecord and SaveRunTrackerRecord stamp them authoritatively,
// and a load refuses a record whose Repo does not name the current repository's
// canonical git common dir.
type RunTrackerRecord struct {
	Schema       int    `json:"schema"`
	Repo         string `json:"repo"`          // canonical git-common-dir path
	Target       string `json:"target"`        // "docket-implement-next"
	CreatedAt    int64  `json:"created_at"`    // epoch seconds
	DispatchedAt int64  `json:"dispatched_at"` // Unix seconds, captured AFTER the before-read
	BeforeIDs    []int  `json:"before_ids"`    // fresh-origin in-progress set
	AttributedID int    `json:"attributed_id"` // 0 = not yet attributed
	Retry        string `json:"retry"`         // RetryUnused | RetryConsumed
	Disposition  string `json:"disposition"`   // latest run-* report line
	Terminal     bool   `json:"terminal"`

	// AttemptLimit is the snapshotted run.max_attempts value (change 0421, schema
	// v4): the total number of attributed implementation attempts this gate starting
	// permits, counting the original dispatch. It is stamped at mint and is
	// IMMUTABLE thereafter — a config edit after mint never rewrites an already-owned
	// budget (the snapshot rule). The counted retry budget grants at most
	// AttemptLimit-1 retries. SaveRunTrackerRecord refuses to persist a v4 record whose
	// AttemptLimit is below the floor, so a corrupt/unstamped budget fails closed.
	AttemptLimit int `json:"attempt_limit"`

	// Outer recovery-scope binding (change 0359, schema v2). ScopeID names the
	// recovery scope run start prepared for this dispatch boundary; ParentCap is
	// the RAW parent capability the takeover path presents — persisted only in this
	// 0600-private record and NEVER printed in HumanText, a report line, or the
	// result JSON; ChildContextHash is the sha256 of the printed run context
	// (the outer scope's ChildCapability), matched against a nested drive's
	// RunContextHash when the verdict path locates the outer drive.
	ScopeID          string `json:"scope_id,omitempty"`
	ParentCap        string `json:"parent_cap,omitempty"`
	ChildContextHash string `json:"child_context_hash,omitempty"`

	// Continuation triple (change 0359, schema v2). The three fields are ALL-EMPTY
	// or ALL-SET: a partial triple is a corrupt record on read AND on write
	// (runTrackerContinuationTripleOK). ContinuationID is the single-use redemption token
	// the verdict path mints; ContinuationDrive + ContinuationHandoff name the
	// tracked drive and its unclaimed handoff a resumed controller claims.
	ContinuationID      string `json:"continuation_id,omitempty"`
	ContinuationDrive   string `json:"continuation_drive,omitempty"`
	ContinuationHandoff string `json:"continuation_handoff,omitempty"`

	// Claim-binding proof MIRROR (change 0407, schema v3). BoundRequestID and
	// BoundRevision are the local reflection of the confirmed claim binding — the
	// committed claim receipt (a Docket-Result trailer on the metadata branch) is
	// the authority; these two are a readable convenience mirrored by
	// ConfirmRunTrackerClaim. They are ALL-EMPTY or ALL-SET (runTrackerBoundPairOK): a partial
	// pair is a corrupt record on read AND on write, so a half-written mirror can
	// never be loaded or persisted. BoundRequestID is the claim's request id;
	// BoundRevision is the commit that carries the confirmed claim receipt.
	BoundRequestID string `json:"bound_request_id,omitempty"`
	BoundRevision  string `json:"bound_revision,omitempty"`
}

// resumeAttributed reports whether rec has the resume-verified shape: `run start
// --resume` pre-bound AttributedID through WorkspaceInspect identity, and no claim
// ever confirmed under it (BoundRequestID is still empty). A fresh start gains
// AttributedID only at confirm time, together with BoundRequestID, so it never
// matches. resolveRunTrackerOwnership accepts this shape as ownership, runCancel accepts
// it as cancel authority, and ChangeClaim refuses to reserve a claim under it
// (change 0463).
func (rec RunTrackerRecord) resumeAttributed() bool {
	return rec.AttributedID != 0 && rec.BoundRequestID == ""
}

// runTrackerBoundPairOK reports whether rec's claim-binding mirror pair is well formed:
// BoundRequestID and BoundRevision must be ALL-EMPTY or ALL-SET (0396's pair rule
// applied to the change-0407 mirror). A partial pair is a corrupt record — the
// store refuses it on both the read and the write boundary so a half-written
// mirror can never be loaded or persisted.
func runTrackerBoundPairOK(rec RunTrackerRecord) bool {
	set := 0
	if rec.BoundRequestID != "" {
		set++
	}
	if rec.BoundRevision != "" {
		set++
	}
	return set == 0 || set == 2
}

// RunTrackerClaimBinding is the durable, bind-once record of which (change, claim
// request) a run key's run context is bound to (change 0407). It is written
// by ReserveRunTrackerClaim through an os.Link hard-link create (the compare-and-swap
// that serializes competing binding attempts with whole-file atomicity) and
// finalized by ConfirmRunTrackerClaim. Schema is stamped authoritatively; a load fails
// closed on any other value.
type RunTrackerClaimBinding struct {
	Schema    int    `json:"schema"`
	ChangeID  int    `json:"change_id"`
	RequestID string `json:"request_id"`
	Confirmed bool   `json:"confirmed"`
	Revision  string `json:"revision,omitempty"`
}

// runTrackerContinuationTripleOK reports whether rec's continuation triple is well
// formed: the three Continuation* fields must be ALL-EMPTY or ALL-SET (0396's
// pair rule extended to a triple). A partial triple is a corrupt record — the
// store refuses it on both the read and the write boundary so a half-written
// continuation can never be loaded or persisted.
func runTrackerContinuationTripleOK(rec RunTrackerRecord) bool {
	set := 0
	if rec.ContinuationID != "" {
		set++
	}
	if rec.ContinuationDrive != "" {
		set++
	}
	if rec.ContinuationHandoff != "" {
		set++
	}
	return set == 0 || set == 3
}

// RunTrackerStoreErrorKind is the typed category of a RunTrackerStoreError. The caller (the
// run verdict verb) maps it to a run-tracker-unavailable reason token: ErrRunTrackerWrongRepo
// -> wrong-repo, ErrRunTrackerMalformedKey -> malformed-key, ErrRunTrackerCorruptRecord ->
// corrupt-record; the remaining kinds are ordinary not-found / IO faults.
type RunTrackerStoreErrorKind string

const (
	// ErrRunTrackerMalformedKey: a key failed the path-safety validator (bad charset,
	// empty, or over-long) before any path was constructed.
	ErrRunTrackerMalformedKey RunTrackerStoreErrorKind = "malformed-key"
	// ErrRunTrackerWrongRepo: the record's embedded Repo does not name the current
	// repository's canonical git common dir — a stale copy or a moved .git.
	ErrRunTrackerWrongRepo RunTrackerStoreErrorKind = "wrong-repo"
	// ErrRunTrackerCorruptRecord: the record could not be decoded, or its schema version
	// is not the one this store understands. Fail closed.
	ErrRunTrackerCorruptRecord RunTrackerStoreErrorKind = "corrupt-record"
	// ErrRunTrackerNotFound: no record exists for a well-formed key.
	ErrRunTrackerNotFound RunTrackerStoreErrorKind = "not-found"
	// ErrRunTrackerUnavailable: the git common dir could not be resolved (an empty
	// answer, or git itself failed).
	ErrRunTrackerUnavailable RunTrackerStoreErrorKind = "run-tracker-unavailable"
	// ErrRunTrackerIO: an underlying filesystem or randomness operation failed.
	ErrRunTrackerIO RunTrackerStoreErrorKind = "io"
	// ErrRunTrackerBindingConflict: a competing binding attempt for a different claim
	// under one run context — the claim-binding file already binds a different
	// (change, request), or a confirm named fields that disagree with the reserved
	// binding. Bind-once: a later attempt can never overwrite the first (change 0407).
	ErrRunTrackerBindingConflict RunTrackerStoreErrorKind = "binding-conflict"
	// ErrRunContextAmbiguous: more than one live (non-terminal) gate record claims
	// one run-context hash, so ownership cannot be resolved to a single record.
	// Fail closed (change 0407).
	ErrRunContextAmbiguous RunTrackerStoreErrorKind = "context-ambiguous"
)

// RunTrackerStoreError is the store's typed failure carrying a stable kind and stage.
// It never embeds record content.
type RunTrackerStoreError struct {
	Kind RunTrackerStoreErrorKind
	Op   string
	err  error
}

func (e *RunTrackerStoreError) Error() string {
	if e.err != nil {
		return fmt.Sprintf("run-tracker store %s: %s: %v", e.Op, e.Kind, e.err)
	}
	return fmt.Sprintf("run-tracker store %s: %s", e.Op, e.Kind)
}

func (e *RunTrackerStoreError) Unwrap() error { return e.err }

func runTrackerErr(kind RunTrackerStoreErrorKind, op string, err error) *RunTrackerStoreError {
	return &RunTrackerStoreError{Kind: kind, Op: op, err: err}
}

// AsRunTrackerStoreError unwraps err to a *RunTrackerStoreError when one is in the chain so a
// caller can branch on its Kind.
func AsRunTrackerStoreError(err error) (*RunTrackerStoreError, bool) {
	var e *RunTrackerStoreError
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// runTrackerGitCommonDir resolves the canonical (symlink-evaluated, absolute) git
// common dir for repoDir. It refuses an empty answer — the guard the dispatch
// dir documents: `cd ""` succeeds, so a fall-through would resolve the root
// inside the worktree instead of under .git/. It creates nothing.
func runTrackerGitCommonDir(repoDir string) (string, error) {
	wt := strings.TrimSpace(repoDir)
	if wt == "" {
		wt = "."
	}
	cmd := exec.Command("git", "-C", wt, "rev-parse", "--git-common-dir")
	out, err := cmd.Output()
	if err != nil {
		return "", runTrackerErr(ErrRunTrackerUnavailable, "common-dir", err)
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return "", runTrackerErr(ErrRunTrackerUnavailable, "common-dir", errors.New("empty git-common-dir answer"))
	}
	// `git -C <wt> rev-parse --git-common-dir` reports the common dir relative to
	// <wt> for the main worktree and absolute for a linked one; resolve the
	// relative form against <wt>, then canonicalize (the `pwd -P` equivalent) so a
	// linked worktree and its main repo compare equal.
	if !filepath.IsAbs(raw) {
		raw = filepath.Join(wt, raw)
	}
	common, err := filepath.EvalSymlinks(raw)
	if err != nil {
		return "", runTrackerErr(ErrRunTrackerUnavailable, "common-dir", err)
	}
	common, err = filepath.Abs(common)
	if err != nil {
		return "", runTrackerErr(ErrRunTrackerUnavailable, "common-dir", err)
	}
	return common, nil
}

// runTrackerRoot resolves <git-common-dir>/docket/run-tracker for repoDir. It resolves,
// never creates — an observer must be able to ask where the root is without
// minting one as a side effect of looking.
func runTrackerRoot(repoDir string) (string, error) {
	common, err := runTrackerGitCommonDir(repoDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(common, "docket", runTrackerDirName), nil
}

// validateRunKey enforces the path-safety contract before any path is built: a
// non-empty, bounded, lowercase [a-z0-9-] token. Every load, save, and consume
// calls it first, so a malformed key never reaches the filesystem or git.
func validateRunKey(key string) error {
	if key == "" || len(key) > runKeyMaxLen {
		return runTrackerErr(ErrRunTrackerMalformedKey, "validate-key", nil)
	}
	if !runKeyPattern.MatchString(key) {
		return runTrackerErr(ErrRunTrackerMalformedKey, "validate-key", nil)
	}
	return nil
}

// MintRunTrackerRecord mints a fresh key, creates its directory under the repository's
// run-tracker root, and atomically writes rec. Schema and Repo are stamped
// authoritatively (any caller-supplied values are overwritten), so wrong-repo
// detection cannot be defeated by a bad input value.
func MintRunTrackerRecord(repoDir string, rec RunTrackerRecord) (string, error) {
	common, err := runTrackerGitCommonDir(repoDir)
	if err != nil {
		return "", err
	}
	root := filepath.Join(common, "docket", runTrackerDirName)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", runTrackerErr(ErrRunTrackerIO, "mint", err)
	}

	rec.Schema = runTrackerSchemaVersion
	rec.Repo = common

	// A key collision would clobber a live dispatch's record, so refuse rather
	// than overwrite: os.Mkdir (not MkdirAll) fails on an existing leaf. The 4 hex
	// random suffix makes a same-second, same-pid collision astronomically
	// unlikely; the guard covers the residual.
	stamp := strings.ToLower(time.Now().UTC().Format("20060102T150405Z"))
	for attempt := 0; attempt < 8; attempt++ {
		key := fmt.Sprintf("implement-next-%s-%d-%04x", stamp, os.Getpid(), rand.Intn(0x10000))
		dir := filepath.Join(root, key)
		err := os.Mkdir(dir, 0o755)
		if err == nil {
			if werr := writeRunTrackerRecordAtomic(dir, rec); werr != nil {
				return "", werr
			}
			return key, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return "", runTrackerErr(ErrRunTrackerIO, "mint", err)
		}
	}
	return "", runTrackerErr(ErrRunTrackerIO, "mint", errors.New("could not mint a unique key"))
}

// LoadRunTrackerRecord reads the record for key from the repository's run-tracker root. It
// validates the key before any filesystem or git touch, refuses a record whose
// Repo does not match the current canonical common dir (wrong-repo), fails closed
// on an unknown schema or unparseable JSON (corrupt-record), and reports Retry as
// consumed when EITHER the marker file or the JSON field says so.
func LoadRunTrackerRecord(repoDir, key string) (RunTrackerRecord, error) {
	if err := validateRunKey(key); err != nil {
		return RunTrackerRecord{}, err
	}
	common, err := runTrackerGitCommonDir(repoDir)
	if err != nil {
		return RunTrackerRecord{}, err
	}
	dir := filepath.Join(common, "docket", runTrackerDirName, key)
	buf, err := os.ReadFile(filepath.Join(dir, runTrackerRecordFileName))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return RunTrackerRecord{}, runTrackerErr(ErrRunTrackerNotFound, "load", err)
		}
		return RunTrackerRecord{}, runTrackerErr(ErrRunTrackerIO, "load", err)
	}
	var rec RunTrackerRecord
	if err := json.Unmarshal(buf, &rec); err != nil {
		return RunTrackerRecord{}, runTrackerErr(ErrRunTrackerCorruptRecord, "load", err)
	}
	if rec.Schema != runTrackerSchemaVersion {
		return RunTrackerRecord{}, runTrackerErr(ErrRunTrackerCorruptRecord, "load",
			fmt.Errorf("schema version %d, want %d", rec.Schema, runTrackerSchemaVersion))
	}
	if rec.Repo != common {
		return RunTrackerRecord{}, runTrackerErr(ErrRunTrackerWrongRepo, "load", nil)
	}
	// A partial continuation triple is a corrupt record: fail closed on read so a
	// half-written continuation is never handed to the verdict path.
	if !runTrackerContinuationTripleOK(rec) {
		return RunTrackerRecord{}, runTrackerErr(ErrRunTrackerCorruptRecord, "load",
			errors.New("partial continuation triple"))
	}
	// A partial claim-binding mirror pair is a corrupt record: fail closed on read
	// so a half-written mirror is never handed to the verdict path.
	if !runTrackerBoundPairOK(rec) {
		return RunTrackerRecord{}, runTrackerErr(ErrRunTrackerCorruptRecord, "load",
			errors.New("partial claim-binding mirror pair"))
	}
	// The markers are authority; reflect them into the readable mirror on read so a
	// crash between an O_EXCL create and the JSON flip still reads as consumed.
	// Consumed == at least one retry marker exists (bare legacy name or any
	// per-attempt retry-consumed-<n>), so no JSON consumer of Retry breaks (0421).
	if n, cerr := countRunTrackerRetryMarkers(dir); cerr == nil && n > 0 {
		rec.Retry = RetryConsumed
	}
	return rec, nil
}

// countRunTrackerRetryMarkers counts the retry-marker files in a key directory: the bare
// legacy name plus every per-attempt retry-consumed-<n> (runTrackerRetryMarkerPattern).
// It is the shared counter behind RunTrackerRetryUsage (the diagnostics surface) and
// LoadRunTrackerRecord's readable-mirror reflection (change 0421). A missing directory
// counts as zero.
func countRunTrackerRetryMarkers(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if runTrackerRetryMarkerPattern.MatchString(e.Name()) {
			n++
		}
	}
	return n, nil
}

// SaveRunTrackerRecord persists rec for key via a same-directory temp file and an
// atomic rename. Schema and Repo are re-stamped authoritatively so a save cannot
// drift the identity fields. The key directory must already exist (minted).
func SaveRunTrackerRecord(repoDir, key string, rec RunTrackerRecord) error {
	if err := validateRunKey(key); err != nil {
		return err
	}
	common, err := runTrackerGitCommonDir(repoDir)
	if err != nil {
		return err
	}
	dir := filepath.Join(common, "docket", runTrackerDirName, key)
	if fi, serr := os.Stat(dir); serr != nil || !fi.IsDir() {
		return runTrackerErr(ErrRunTrackerNotFound, "save", serr)
	}
	rec.Schema = runTrackerSchemaVersion
	rec.Repo = common
	return writeRunTrackerRecordAtomic(dir, rec)
}

// writeRunTrackerRecordAtomic marshals rec and writes it at dir/record.json through a
// same-directory temp file followed by os.Rename — the atomic-adjacent
// replacement rule. os.CreateTemp is templated into the destination's own
// directory so the rename is same-filesystem.
func writeRunTrackerRecordAtomic(dir string, rec RunTrackerRecord) error {
	// A partial continuation triple is a corrupt record: refuse to persist one so a
	// half-written continuation never reaches disk.
	if !runTrackerContinuationTripleOK(rec) {
		return runTrackerErr(ErrRunTrackerCorruptRecord, "write", errors.New("partial continuation triple"))
	}
	// A partial claim-binding mirror pair is a corrupt record: refuse to persist one
	// so a half-written mirror never reaches disk.
	if !runTrackerBoundPairOK(rec) {
		return runTrackerErr(ErrRunTrackerCorruptRecord, "write", errors.New("partial claim-binding mirror pair"))
	}
	// A v4 record whose AttemptLimit is below the floor is corrupt/unstamped: refuse
	// to persist it so an unstamped budget can never be loaded and mistaken for a
	// valid snapshot (change 0421). Schema is re-stamped to runTrackerSchemaVersion by both
	// MintRunTrackerRecord and SaveRunTrackerRecord before this write, so this fires on every
	// current-schema write with an unset or sub-floor limit.
	if rec.Schema == runTrackerSchemaVersion && rec.AttemptLimit < 1 {
		return runTrackerErr(ErrRunTrackerCorruptRecord, "write", errors.New("attempt_limit below floor"))
	}
	buf, err := json.Marshal(rec)
	if err != nil {
		return runTrackerErr(ErrRunTrackerIO, "write", err)
	}
	tmp, err := os.CreateTemp(dir, "."+runTrackerRecordFileName+".tmp-*")
	if err != nil {
		return runTrackerErr(ErrRunTrackerIO, "write", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename
	if _, err := tmp.Write(buf); err != nil {
		tmp.Close()
		return runTrackerErr(ErrRunTrackerIO, "write", err)
	}
	if err := tmp.Close(); err != nil {
		return runTrackerErr(ErrRunTrackerIO, "write", err)
	}
	if err := os.Rename(tmpName, filepath.Join(dir, runTrackerRecordFileName)); err != nil {
		return runTrackerErr(ErrRunTrackerIO, "write", err)
	}
	return nil
}

// ConsumeRunTrackerRetry grants the per-attempt retry permit that moves attempt to
// attempt+1, from a counted budget of at most limit-1 retries (change 0421). It
// refuses (false, nil) BEFORE any filesystem write when attempt >= limit (the
// budget is spent, or limit 1 disables retries) or attempt < 1 (a nonsensical
// attempt number). Otherwise the os.OpenFile O_CREATE|O_EXCL create of
// runTrackerRetryMarkerFor(attempt) is the compare-and-swap: of any number of concurrent
// callers observing the same attempt transition exactly one creates the marker and
// returns true; every other observes fs.ErrExist and returns false without
// granting. For attempt 1 the bare legacy retry-consumed marker (schema v3's single
// permit) is treated as already-spent so an older consumed permit can never be
// re-granted. After winning it flips the record's Retry mirror to consumed
// (best-effort; the marker is authority).
func ConsumeRunTrackerRetry(repoDir, key string, attempt, limit int) (bool, error) {
	if err := validateRunKey(key); err != nil {
		return false, err
	}
	// Budget refusal happens BEFORE any filesystem write: an attempt at or above the
	// limit is a spent budget (a lost grant is the safe failure), and a
	// sub-one attempt number is nonsensical. Neither creates a marker.
	if attempt >= limit || attempt < 1 {
		return false, nil
	}
	common, err := runTrackerGitCommonDir(repoDir)
	if err != nil {
		return false, err
	}
	dir := filepath.Join(common, "docket", runTrackerDirName, key)
	if fi, serr := os.Stat(dir); serr != nil || !fi.IsDir() {
		return false, runTrackerErr(ErrRunTrackerNotFound, "consume", serr)
	}
	// Attempt 1 is the transition a legacy bare marker recorded: if it exists, the
	// attempt-1 retry was already spent under schema v3 and must never be re-granted.
	if attempt == 1 {
		if _, serr := os.Stat(filepath.Join(dir, runTrackerRetryMarkerName)); serr == nil {
			return false, nil
		}
	}
	marker := filepath.Join(dir, runTrackerRetryMarkerFor(attempt))
	f, err := os.OpenFile(marker, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return false, nil // lost the compare-and-swap; this attempt's permit already spent
		}
		return false, runTrackerErr(ErrRunTrackerIO, "consume", err)
	}
	_ = f.Close()

	// Flip the readable mirror. The marker already made the grant durable, so a
	// failure here does not un-grant — LoadRunTrackerRecord reports consumed from the
	// markers regardless.
	if rec, lerr := LoadRunTrackerRecord(repoDir, key); lerr == nil {
		rec.Retry = RetryConsumed
		_ = SaveRunTrackerRecord(repoDir, key, rec)
	}
	return true, nil
}

// RunTrackerRetryUsage reports how many retry markers key has consumed: the bare legacy
// marker plus every per-attempt retry-consumed-<n> (change 0421). It is a
// diagnostics surface, NOT load-bearing for grants — the per-attempt O_CREATE|O_EXCL
// CAS in ConsumeRunTrackerRetry is the sole grant authority. A well-formed key with no
// record directory yet reports (0, nil).
func RunTrackerRetryUsage(repoDir, key string) (int, error) {
	if err := validateRunKey(key); err != nil {
		return 0, err
	}
	common, err := runTrackerGitCommonDir(repoDir)
	if err != nil {
		return 0, err
	}
	dir := filepath.Join(common, "docket", runTrackerDirName, key)
	n, cerr := countRunTrackerRetryMarkers(dir)
	if cerr != nil {
		return 0, runTrackerErr(ErrRunTrackerIO, "retry-usage", cerr)
	}
	return n, nil
}

// runKeyDir validates key and resolves its record directory under the
// repository's run-tracker root, requiring the directory to already exist (minted).
// It is the shared preamble of the claim-binding primitives.
func runKeyDir(repoDir, key, op string) (string, error) {
	if err := validateRunKey(key); err != nil {
		return "", err
	}
	common, err := runTrackerGitCommonDir(repoDir)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(common, "docket", runTrackerDirName, key)
	if fi, serr := os.Stat(dir); serr != nil || !fi.IsDir() {
		return "", runTrackerErr(ErrRunTrackerNotFound, op, serr)
	}
	return dir, nil
}

// readRunTrackerClaimBinding reads the claim-binding file at dir. A missing file is
// (RunTrackerClaimBinding{}, false, nil); unparseable bytes or a bad schema fail closed
// as corrupt-record; a well-formed binding is (b, true, nil).
func readRunTrackerClaimBinding(dir, op string) (RunTrackerClaimBinding, bool, error) {
	buf, err := os.ReadFile(filepath.Join(dir, runTrackerClaimBindingName))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return RunTrackerClaimBinding{}, false, nil
		}
		return RunTrackerClaimBinding{}, false, runTrackerErr(ErrRunTrackerIO, op, err)
	}
	var b RunTrackerClaimBinding
	if err := json.Unmarshal(buf, &b); err != nil {
		return RunTrackerClaimBinding{}, false, runTrackerErr(ErrRunTrackerCorruptRecord, op, err)
	}
	if b.Schema != bindingSchemaVersion {
		return RunTrackerClaimBinding{}, false, runTrackerErr(ErrRunTrackerCorruptRecord, op,
			fmt.Errorf("binding schema version %d, want %d", b.Schema, bindingSchemaVersion))
	}
	return b, true, nil
}

// ReserveRunTrackerClaim binds key's run context to (changeID, requestID) exactly
// once, before the claim's metadata transaction. The os.Link hard-link create is
// the compare-and-swap: the reservation is written to a same-directory temp file
// and hard-linked into place, so of any number of concurrent callers exactly one
// creates the binding with whole-file atomicity (never a partially-written CAS
// winner). An existing binding for the same (changeID, requestID) is an idempotent
// replay (nil); an existing binding for a different claim is ErrRunTrackerBindingConflict
// — bind-once, a later attempt can never overwrite the first.
func ReserveRunTrackerClaim(repoDir, key string, changeID int, requestID string) error {
	dir, err := runKeyDir(repoDir, key, "reserve")
	if err != nil {
		return err
	}
	// The os.Link hard-link create below is the sole authoritative CAS — there is no
	// short-circuiting pre-read, so the exists-decision always flows through the
	// fs.ErrExist re-load branch and the atomicity guard stays load-bearing (a
	// pre-read that answered match-or-conflict on its own would make the CAS
	// untestable and would race a concurrent writer). This mirrors the
	// reserveScopeDrive CAS discipline: the compare-and-swap is authority.
	buf, err := json.Marshal(RunTrackerClaimBinding{Schema: bindingSchemaVersion, ChangeID: changeID, RequestID: requestID, Confirmed: false})
	if err != nil {
		return runTrackerErr(ErrRunTrackerIO, "reserve", err)
	}
	tmp, err := os.CreateTemp(dir, "."+runTrackerClaimBindingName+".tmp-*")
	if err != nil {
		return runTrackerErr(ErrRunTrackerIO, "reserve", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // removes the temp hard-link once the final link is in place (or after a link conflict)
	if _, err := tmp.Write(buf); err != nil {
		tmp.Close()
		return runTrackerErr(ErrRunTrackerIO, "reserve", err)
	}
	if err := tmp.Close(); err != nil {
		return runTrackerErr(ErrRunTrackerIO, "reserve", err)
	}
	// The hard-link create is the CAS: it fails fs.ErrExist if a concurrent
	// reservation already won. On a win the temp name is removed; on a loss re-load
	// and apply the same match-or-conflict rule as the fast path.
	final := filepath.Join(dir, runTrackerClaimBindingName)
	if err := os.Link(tmpName, final); err != nil {
		if errors.Is(err, fs.ErrExist) {
			b, ok, berr := readRunTrackerClaimBinding(dir, "reserve")
			if berr != nil {
				return berr
			}
			if !ok {
				return runTrackerErr(ErrRunTrackerIO, "reserve", errors.New("binding vanished after link conflict"))
			}
			return reserveMatchOrConflict(b, changeID, requestID)
		}
		return runTrackerErr(ErrRunTrackerIO, "reserve", err)
	}
	return nil
}

// reserveMatchOrConflict is the bind-once decision on an existing binding: the
// same (changeID, requestID) is an idempotent replay (nil); any other pairing is
// ErrRunTrackerBindingConflict.
func reserveMatchOrConflict(b RunTrackerClaimBinding, changeID int, requestID string) error {
	if b.ChangeID == changeID && b.RequestID == requestID {
		return nil
	}
	return runTrackerErr(ErrRunTrackerBindingConflict, "reserve", nil)
}

// ConfirmRunTrackerClaim finalizes key's reserved binding after the claim's metadata
// transaction applied, recording revision (the commit carrying the confirmed
// claim receipt) and mirroring AttributedID/BoundRequestID/BoundRevision onto the
// record. An absent binding is ErrRunTrackerNotFound — a failed or absent reservation
// can never become a confirmed binding. A binding whose (changeID, requestID)
// disagrees is ErrRunTrackerBindingConflict, as is a re-confirm carrying a different
// revision — a later verdict or replay can never overwrite a confirmed binding. A
// re-confirm with identical fields is an idempotent no-op. The mirror save's error
// is returned (callers may treat it as best-effort; the binding file already made
// the confirm durable).
//
// worktree is the change's canonical feature worktree (the LOGICAL path — it need not
// exist yet), bound onto the run here so a FRESH run's mutation fence and
// run.cancel teardown locate it (change 0375). It is best-effort and a NO-OP when
// empty or when no run exists (see bindRunWorktree); the claim path passes it, the
// verdict recovery paths — which have no worktree in hand — pass "".
func ConfirmRunTrackerClaim(repoDir, key string, changeID int, requestID, revision, worktree string) error {
	dir, err := runKeyDir(repoDir, key, "confirm")
	if err != nil {
		return err
	}
	b, ok, berr := readRunTrackerClaimBinding(dir, "confirm")
	if berr != nil {
		return berr
	}
	if !ok {
		return runTrackerErr(ErrRunTrackerNotFound, "confirm", nil)
	}
	if b.ChangeID != changeID || b.RequestID != requestID {
		return runTrackerErr(ErrRunTrackerBindingConflict, "confirm", nil)
	}
	if b.Confirmed {
		// Already confirmed: identical revision is idempotent; a differing revision
		// can never overwrite the confirmed binding.
		if b.Revision == revision {
			return nil
		}
		return runTrackerErr(ErrRunTrackerBindingConflict, "confirm", nil)
	}

	confirmed := RunTrackerClaimBinding{Schema: bindingSchemaVersion, ChangeID: changeID, RequestID: requestID, Confirmed: true, Revision: revision}
	if err := writeRunTrackerClaimBindingAtomic(dir, confirmed); err != nil {
		return err
	}

	// Mirror onto the record: the committed claim receipt is authority, this is the
	// readable local reflection. A mirror-save failure is returned but does not
	// un-confirm — the binding file already made the confirm durable.
	rec, lerr := LoadRunTrackerRecord(repoDir, key)
	if lerr != nil {
		return lerr
	}
	rec.AttributedID = changeID
	rec.BoundRequestID = requestID
	rec.BoundRevision = revision
	// Bind the run to this confirmed change instance (change 0375 Task 9). The
	// committed claim receipt is authority (ADR-0111); the run's ChangeID is the
	// readable locator a later resume/cancel resolves the run by. It is best-effort
	// and a NO-OP when no run exists (a standalone or keyless dispatch), so it never
	// fails an otherwise-confirmed claim; a genuine re-point over a different change is
	// refused inside bindRunChange and swallowed here (the receipt already bound the
	// change).
	_ = bindRunChange(repoDir, key, strconv.Itoa(changeID))
	// Bind the run's feature worktree (change 0375). A FRESH run's run record is minted
	// with an empty Worktree, so without this the mutation fence (findRunByWorktree)
	// and run.cancel's worktree teardown are inert for the common first-dispatch case —
	// the resume path already binds it in armResumeReplacement. Best-effort and a NO-OP
	// on an empty worktree or a keyless/no-run-record dispatch; a benign spelling difference
	// against a resume's pre-bound worktree is refused inside bindRunWorktree and
	// swallowed here.
	_ = bindRunWorktree(repoDir, key, worktree)
	return SaveRunTrackerRecord(repoDir, key, rec)
}

// writeRunTrackerClaimBindingAtomic writes b at dir/claim-binding.json through a
// same-directory temp file followed by os.Rename — the atomic-adjacent
// replacement rule, matching writeRunTrackerRecordAtomic. It is the confirm rewrite; the
// bind-once create goes through ReserveRunTrackerClaim's os.Link CAS instead.
func writeRunTrackerClaimBindingAtomic(dir string, b RunTrackerClaimBinding) error {
	buf, err := json.Marshal(b)
	if err != nil {
		return runTrackerErr(ErrRunTrackerIO, "write-binding", err)
	}
	tmp, err := os.CreateTemp(dir, "."+runTrackerClaimBindingName+".tmp-*")
	if err != nil {
		return runTrackerErr(ErrRunTrackerIO, "write-binding", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename
	if _, err := tmp.Write(buf); err != nil {
		tmp.Close()
		return runTrackerErr(ErrRunTrackerIO, "write-binding", err)
	}
	if err := tmp.Close(); err != nil {
		return runTrackerErr(ErrRunTrackerIO, "write-binding", err)
	}
	if err := os.Rename(tmpName, filepath.Join(dir, runTrackerClaimBindingName)); err != nil {
		return runTrackerErr(ErrRunTrackerIO, "write-binding", err)
	}
	return nil
}

// LoadRunTrackerClaimBinding reads the claim binding for key. A missing binding file is
// (RunTrackerClaimBinding{}, false, nil); unparseable bytes or a bad schema fail closed
// as corrupt-record; a well-formed binding is (b, true, nil).
func LoadRunTrackerClaimBinding(repoDir, key string) (RunTrackerClaimBinding, bool, error) {
	dir, err := runKeyDir(repoDir, key, "load-binding")
	if err != nil {
		return RunTrackerClaimBinding{}, false, err
	}
	return readRunTrackerClaimBinding(dir, "load-binding")
}

// FindRunTrackerRecordByContextHash resolves the single live (non-terminal) gate record
// whose ChildContextHash equals contextHash. It reads the run-tracker root and skips
// any sibling whose record fails to load — a foreign-repo or corrupt sibling never
// blocks an unrelated claim. Zero matches is ErrRunTrackerNotFound; more than one is
// ErrRunContextAmbiguous; exactly one returns (key, record, nil). An empty
// contextHash never matches.
func FindRunTrackerRecordByContextHash(repoDir, contextHash string) (string, RunTrackerRecord, error) {
	if contextHash == "" {
		return "", RunTrackerRecord{}, runTrackerErr(ErrRunTrackerNotFound, "find-context", nil)
	}
	root, err := runTrackerRoot(repoDir)
	if err != nil {
		return "", RunTrackerRecord{}, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", RunTrackerRecord{}, runTrackerErr(ErrRunTrackerNotFound, "find-context", err)
		}
		return "", RunTrackerRecord{}, runTrackerErr(ErrRunTrackerIO, "find-context", err)
	}
	var (
		matchKey string
		matchRec RunTrackerRecord
		matches  int
	)
	for _, e := range entries {
		if !e.IsDir() || validateRunKey(e.Name()) != nil {
			continue
		}
		rec, lerr := LoadRunTrackerRecord(repoDir, e.Name())
		if lerr != nil {
			continue // a foreign-repo or corrupt sibling never blocks an unrelated claim
		}
		if rec.Terminal || rec.ChildContextHash != contextHash {
			continue
		}
		matchKey = e.Name()
		matchRec = rec
		matches++
	}
	switch {
	case matches == 0:
		return "", RunTrackerRecord{}, runTrackerErr(ErrRunTrackerNotFound, "find-context", nil)
	case matches > 1:
		return "", RunTrackerRecord{}, runTrackerErr(ErrRunContextAmbiguous, "find-context", nil)
	default:
		return matchKey, matchRec, nil
	}
}

// PruneRunTrackerRecords removes terminal records whose record file has aged past the
// retention window. It is best-effort and returns nothing: a prune never fails a
// dispatch. A nonterminal record is NEVER age-pruned merely because its
// originating process exited (spec) — only Terminal records are eligible, and the
// age is measured from the record file's mtime (written last, so the window runs
// from the end of the run). A non-numeric retention knob disables pruning
// entirely, matching the dispatch dir's conservative behavior.
func PruneRunTrackerRecords(repoDir string) {
	days, ok := runTrackerRetentionDays()
	if !ok {
		return
	}
	root, err := runTrackerRoot(repoDir)
	if err != nil {
		return
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		recPath := filepath.Join(root, e.Name(), runTrackerRecordFileName)
		buf, err := os.ReadFile(recPath)
		if err != nil {
			continue
		}
		var rec RunTrackerRecord
		if err := json.Unmarshal(buf, &rec); err != nil {
			continue
		}
		if !rec.Terminal {
			continue
		}
		fi, err := os.Stat(recPath)
		if err != nil {
			continue
		}
		if fi.ModTime().Before(cutoff) {
			_ = os.RemoveAll(filepath.Join(root, e.Name()))
		}
	}
}

// runTrackerRetentionDays reads the retention window in days. An unset knob is the
// 7-day default; a non-numeric or negative value disables pruning (ok=false), the
// conservative direction — the evidence a gate record preserves is never
// destroyed on a misconfigured knob.
func runTrackerRetentionDays() (days int, ok bool) {
	v := strings.TrimSpace(os.Getenv(runTrackerRetentionEnv))
	if v == "" {
		return 7, true
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}
