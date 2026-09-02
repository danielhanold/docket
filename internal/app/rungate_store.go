// The durable per-dispatch gate-record store (change 0334, Task 1). It is the
// Go generalization of scripts/lib/docket-dispatch-dir.sh's durable-dir
// conventions, holding the implement-next run gate's attribution / retry state
// so a dispatch's gate outcome survives the launching process, a
// `git worktree remove`, and a restart.
//
// WHERE: <git-common-dir>/docket/rungate/<key>/record.json — the same family as
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
// RETRY CAS: ConsumeGateRetry's exclusivity rests on os.OpenFile with
// O_CREATE|O_EXCL: the filesystem exclusive-create is the compare-and-swap, so of
// any number of concurrent callers exactly one creates the marker and returns
// true. The marker file is authority; the record's Retry field is a readable
// mirror flipped afterward, and LoadGateRecord reports consumed when EITHER says
// so, so a crash between the two writes stays safe.
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

// gateSchemaVersion is the on-disk record schema this store understands. A record
// carrying any other version fails closed as a corrupt record — never a
// best-effort migration. Bumped to 2 for change 0359: the record grows the outer
// recovery-scope binding (ScopeID/ParentCap/ChildContextHash) and the
// continuation triple. A schema-1 record therefore fails closed here — an
// in-flight pre-upgrade record halting after merge is correct fail-closed
// behavior, never a silent migration.
const gateSchemaVersion = 2

// Retry permit states recorded in a GateRecord. The one-retry permit is unused
// until ConsumeGateRetry spends it; the marker file, not this field, is the
// authority (see the package comment).
const (
	RetryUnused   = "unused"
	RetryConsumed = "consumed"
)

// recordFileName is the atomic record within a key directory; retryMarkerName is
// the O_EXCL compare-and-swap marker whose creation grants the single retry.
const (
	gateRecordFileName  = "record.json"
	gateRetryMarkerName = "retry-consumed"
)

// gateRetentionEnv is the retention-window knob, shared with the dispatch dir's
// semantics (default 7 days; a non-numeric value disables age-pruning entirely).
const gateRetentionEnv = "DOCKET_DISPATCH_RETENTION_DAYS"

// gateKeyPattern is the path-safety validator every load applies before touching
// the filesystem. The charset excludes '/', '\\', and '.', so a traversal key
// cannot escape the root.
var gateKeyPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

// gateKeyMaxLen bounds a key length so a pathologically long token is refused
// before any path is built.
const gateKeyMaxLen = 128

// GateRecord is the durable per-dispatch gate state. The key is a lookup token,
// never encoded state (spec: "Durable state"). Schema and Repo are store-owned
// identity fields: MintGateRecord and SaveGateRecord stamp them authoritatively,
// and a load refuses a record whose Repo does not name the current repository's
// canonical git common dir.
type GateRecord struct {
	Schema        int    `json:"schema"`
	Repo          string `json:"repo"`           // canonical git-common-dir path
	Target        string `json:"target"`         // "docket-implement-next"
	CreatedAt     int64  `json:"created_at"`     // epoch seconds
	DispatchEpoch int64  `json:"dispatch_epoch"` // captured AFTER the before-read
	BeforeIDs     []int  `json:"before_ids"`     // fresh-origin in-progress set
	AttributedID  int    `json:"attributed_id"`  // 0 = not yet attributed
	Retry         string `json:"retry"`          // RetryUnused | RetryConsumed
	Disposition   string `json:"disposition"`    // latest gate-* report line
	Terminal      bool   `json:"terminal"`

	// Outer recovery-scope binding (change 0359, schema v2). ScopeID names the
	// recovery scope gate-before prepared for this dispatch boundary; ParentCap is
	// the RAW parent capability the takeover path presents — persisted only in this
	// 0600-private record and NEVER printed in HumanText, a report line, or the
	// result JSON; ChildContextHash is the sha256 of the printed dispatch context
	// (the outer scope's ChildCapability), matched against a nested drive's
	// GateContextHash when the verdict path locates the outer drive.
	ScopeID          string `json:"scope_id,omitempty"`
	ParentCap        string `json:"parent_cap,omitempty"`
	ChildContextHash string `json:"child_context_hash,omitempty"`

	// Continuation triple (change 0359, schema v2). The three fields are ALL-EMPTY
	// or ALL-SET: a partial triple is a corrupt record on read AND on write
	// (gateContinuationTripleOK). ContinuationID is the single-use redemption token
	// the verdict path mints; ContinuationDrive + ContinuationHandoff name the
	// tracked drive and its unclaimed handoff a resumed controller claims.
	ContinuationID      string `json:"continuation_id,omitempty"`
	ContinuationDrive   string `json:"continuation_drive,omitempty"`
	ContinuationHandoff string `json:"continuation_handoff,omitempty"`
}

// gateContinuationTripleOK reports whether rec's continuation triple is well
// formed: the three Continuation* fields must be ALL-EMPTY or ALL-SET (0396's
// pair rule extended to a triple). A partial triple is a corrupt record — the
// store refuses it on both the read and the write boundary so a half-written
// continuation can never be loaded or persisted.
func gateContinuationTripleOK(rec GateRecord) bool {
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

// GateStoreErrorKind is the typed category of a GateStoreError. The caller (the
// gate-verdict verb) maps it to a gate-unavailable reason token: ErrGateWrongRepo
// -> wrong-repo, ErrGateMalformedKey -> malformed-key, ErrGateCorruptRecord ->
// corrupt-record; the remaining kinds are ordinary not-found / IO faults.
type GateStoreErrorKind string

const (
	// ErrGateMalformedKey: a key failed the path-safety validator (bad charset,
	// empty, or over-long) before any path was constructed.
	ErrGateMalformedKey GateStoreErrorKind = "malformed-key"
	// ErrGateWrongRepo: the record's embedded Repo does not name the current
	// repository's canonical git common dir — a stale copy or a moved .git.
	ErrGateWrongRepo GateStoreErrorKind = "wrong-repo"
	// ErrGateCorruptRecord: the record could not be decoded, or its schema version
	// is not the one this store understands. Fail closed.
	ErrGateCorruptRecord GateStoreErrorKind = "corrupt-record"
	// ErrGateNotFound: no record exists for a well-formed key.
	ErrGateNotFound GateStoreErrorKind = "not-found"
	// ErrGateUnavailable: the git common dir could not be resolved (an empty
	// answer, or git itself failed).
	ErrGateUnavailable GateStoreErrorKind = "gate-unavailable"
	// ErrGateIO: an underlying filesystem or randomness operation failed.
	ErrGateIO GateStoreErrorKind = "io"
)

// GateStoreError is the store's typed failure carrying a stable kind and stage.
// It never embeds record content.
type GateStoreError struct {
	Kind GateStoreErrorKind
	Op   string
	err  error
}

func (e *GateStoreError) Error() string {
	if e.err != nil {
		return fmt.Sprintf("rungate store %s: %s: %v", e.Op, e.Kind, e.err)
	}
	return fmt.Sprintf("rungate store %s: %s", e.Op, e.Kind)
}

func (e *GateStoreError) Unwrap() error { return e.err }

func gateErr(kind GateStoreErrorKind, op string, err error) *GateStoreError {
	return &GateStoreError{Kind: kind, Op: op, err: err}
}

// AsGateStoreError unwraps err to a *GateStoreError when one is in the chain so a
// caller can branch on its Kind.
func AsGateStoreError(err error) (*GateStoreError, bool) {
	var e *GateStoreError
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// gateGitCommonDir resolves the canonical (symlink-evaluated, absolute) git
// common dir for repoDir. It refuses an empty answer — the guard the dispatch
// dir documents: `cd ""` succeeds, so a fall-through would resolve the root
// inside the worktree instead of under .git/. It creates nothing.
func gateGitCommonDir(repoDir string) (string, error) {
	wt := strings.TrimSpace(repoDir)
	if wt == "" {
		wt = "."
	}
	cmd := exec.Command("git", "-C", wt, "rev-parse", "--git-common-dir")
	out, err := cmd.Output()
	if err != nil {
		return "", gateErr(ErrGateUnavailable, "common-dir", err)
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return "", gateErr(ErrGateUnavailable, "common-dir", errors.New("empty git-common-dir answer"))
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
		return "", gateErr(ErrGateUnavailable, "common-dir", err)
	}
	common, err = filepath.Abs(common)
	if err != nil {
		return "", gateErr(ErrGateUnavailable, "common-dir", err)
	}
	return common, nil
}

// gateRoot resolves <git-common-dir>/docket/rungate for repoDir. It resolves,
// never creates — an observer must be able to ask where the root is without
// minting one as a side effect of looking.
func gateRoot(repoDir string) (string, error) {
	common, err := gateGitCommonDir(repoDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(common, "docket", "rungate"), nil
}

// validateGateKey enforces the path-safety contract before any path is built: a
// non-empty, bounded, lowercase [a-z0-9-] token. Every load, save, and consume
// calls it first, so a malformed key never reaches the filesystem or git.
func validateGateKey(key string) error {
	if key == "" || len(key) > gateKeyMaxLen {
		return gateErr(ErrGateMalformedKey, "validate-key", nil)
	}
	if !gateKeyPattern.MatchString(key) {
		return gateErr(ErrGateMalformedKey, "validate-key", nil)
	}
	return nil
}

// MintGateRecord mints a fresh key, creates its directory under the repository's
// rungate root, and atomically writes rec. Schema and Repo are stamped
// authoritatively (any caller-supplied values are overwritten), so wrong-repo
// detection cannot be defeated by a bad input value.
func MintGateRecord(repoDir string, rec GateRecord) (string, error) {
	common, err := gateGitCommonDir(repoDir)
	if err != nil {
		return "", err
	}
	root := filepath.Join(common, "docket", "rungate")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", gateErr(ErrGateIO, "mint", err)
	}

	rec.Schema = gateSchemaVersion
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
			if werr := writeGateRecordAtomic(dir, rec); werr != nil {
				return "", werr
			}
			return key, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return "", gateErr(ErrGateIO, "mint", err)
		}
	}
	return "", gateErr(ErrGateIO, "mint", errors.New("could not mint a unique key"))
}

// LoadGateRecord reads the record for key from the repository's rungate root. It
// validates the key before any filesystem or git touch, refuses a record whose
// Repo does not match the current canonical common dir (wrong-repo), fails closed
// on an unknown schema or unparseable JSON (corrupt-record), and reports Retry as
// consumed when EITHER the marker file or the JSON field says so.
func LoadGateRecord(repoDir, key string) (GateRecord, error) {
	if err := validateGateKey(key); err != nil {
		return GateRecord{}, err
	}
	common, err := gateGitCommonDir(repoDir)
	if err != nil {
		return GateRecord{}, err
	}
	dir := filepath.Join(common, "docket", "rungate", key)
	buf, err := os.ReadFile(filepath.Join(dir, gateRecordFileName))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return GateRecord{}, gateErr(ErrGateNotFound, "load", err)
		}
		return GateRecord{}, gateErr(ErrGateIO, "load", err)
	}
	var rec GateRecord
	if err := json.Unmarshal(buf, &rec); err != nil {
		return GateRecord{}, gateErr(ErrGateCorruptRecord, "load", err)
	}
	if rec.Schema != gateSchemaVersion {
		return GateRecord{}, gateErr(ErrGateCorruptRecord, "load",
			fmt.Errorf("schema version %d, want %d", rec.Schema, gateSchemaVersion))
	}
	if rec.Repo != common {
		return GateRecord{}, gateErr(ErrGateWrongRepo, "load", nil)
	}
	// A partial continuation triple is a corrupt record: fail closed on read so a
	// half-written continuation is never handed to the verdict path.
	if !gateContinuationTripleOK(rec) {
		return GateRecord{}, gateErr(ErrGateCorruptRecord, "load",
			errors.New("partial continuation triple"))
	}
	// The marker is authority; reflect it into the readable mirror on read so a
	// crash between the O_EXCL create and the JSON flip still reads as consumed.
	if _, serr := os.Stat(filepath.Join(dir, gateRetryMarkerName)); serr == nil {
		rec.Retry = RetryConsumed
	}
	return rec, nil
}

// SaveGateRecord persists rec for key via a same-directory temp file and an
// atomic rename. Schema and Repo are re-stamped authoritatively so a save cannot
// drift the identity fields. The key directory must already exist (minted).
func SaveGateRecord(repoDir, key string, rec GateRecord) error {
	if err := validateGateKey(key); err != nil {
		return err
	}
	common, err := gateGitCommonDir(repoDir)
	if err != nil {
		return err
	}
	dir := filepath.Join(common, "docket", "rungate", key)
	if fi, serr := os.Stat(dir); serr != nil || !fi.IsDir() {
		return gateErr(ErrGateNotFound, "save", serr)
	}
	rec.Schema = gateSchemaVersion
	rec.Repo = common
	return writeGateRecordAtomic(dir, rec)
}

// writeGateRecordAtomic marshals rec and writes it at dir/record.json through a
// same-directory temp file followed by os.Rename — the atomic-adjacent
// replacement rule. os.CreateTemp is templated into the destination's own
// directory so the rename is same-filesystem.
func writeGateRecordAtomic(dir string, rec GateRecord) error {
	// A partial continuation triple is a corrupt record: refuse to persist one so a
	// half-written continuation never reaches disk.
	if !gateContinuationTripleOK(rec) {
		return gateErr(ErrGateCorruptRecord, "write", errors.New("partial continuation triple"))
	}
	buf, err := json.Marshal(rec)
	if err != nil {
		return gateErr(ErrGateIO, "write", err)
	}
	tmp, err := os.CreateTemp(dir, "."+gateRecordFileName+".tmp-*")
	if err != nil {
		return gateErr(ErrGateIO, "write", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename
	if _, err := tmp.Write(buf); err != nil {
		tmp.Close()
		return gateErr(ErrGateIO, "write", err)
	}
	if err := tmp.Close(); err != nil {
		return gateErr(ErrGateIO, "write", err)
	}
	if err := os.Rename(tmpName, filepath.Join(dir, gateRecordFileName)); err != nil {
		return gateErr(ErrGateIO, "write", err)
	}
	return nil
}

// ConsumeGateRetry grants the single retry permit for key exactly once. The
// os.OpenFile O_CREATE|O_EXCL create is the compare-and-swap: of any number of
// concurrent callers exactly one creates the marker and returns true; every other
// caller observes fs.ErrExist and returns false without granting. After winning,
// it flips the record's Retry mirror to consumed (best-effort; the marker is
// authority).
func ConsumeGateRetry(repoDir, key string) (bool, error) {
	if err := validateGateKey(key); err != nil {
		return false, err
	}
	common, err := gateGitCommonDir(repoDir)
	if err != nil {
		return false, err
	}
	dir := filepath.Join(common, "docket", "rungate", key)
	if fi, serr := os.Stat(dir); serr != nil || !fi.IsDir() {
		return false, gateErr(ErrGateNotFound, "consume", serr)
	}
	marker := filepath.Join(dir, gateRetryMarkerName)
	f, err := os.OpenFile(marker, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return false, nil // lost the compare-and-swap; permit already spent
		}
		return false, gateErr(ErrGateIO, "consume", err)
	}
	_ = f.Close()

	// Flip the readable mirror. The marker already made the grant durable, so a
	// failure here does not un-grant — LoadGateRecord reports consumed from the
	// marker regardless.
	if rec, lerr := LoadGateRecord(repoDir, key); lerr == nil {
		rec.Retry = RetryConsumed
		_ = SaveGateRecord(repoDir, key, rec)
	}
	return true, nil
}

// PruneGateRecords removes terminal records whose record file has aged past the
// retention window. It is best-effort and returns nothing: a prune never fails a
// dispatch. A nonterminal record is NEVER age-pruned merely because its
// originating process exited (spec) — only Terminal records are eligible, and the
// age is measured from the record file's mtime (written last, so the window runs
// from the end of the run). A non-numeric retention knob disables pruning
// entirely, matching the dispatch dir's conservative behavior.
func PruneGateRecords(repoDir string) {
	days, ok := gateRetentionDays()
	if !ok {
		return
	}
	root, err := gateRoot(repoDir)
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
		recPath := filepath.Join(root, e.Name(), gateRecordFileName)
		buf, err := os.ReadFile(recPath)
		if err != nil {
			continue
		}
		var rec GateRecord
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

// gateRetentionDays reads the retention window in days. An unset knob is the
// 7-day default; a non-numeric or negative value disables pruning (ok=false), the
// conservative direction — the evidence a gate record preserves is never
// destroyed on a misconfigured knob.
func gateRetentionDays() (days int, ok bool) {
	v := strings.TrimSpace(os.Getenv(gateRetentionEnv))
	if v == "" {
		return 7, true
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}
