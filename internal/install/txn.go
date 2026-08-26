package install

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/danielhanold/docket/internal/document"
)

// An installation spans independent harness directories, so no single rename
// can publish the whole set at once. The engine below buys the missing atomicity
// with a journal instead: every pre-image is captured and the plan is written
// down BEFORE the first target changes, so any later stop — a failed step, or
// the process dying outright — has a complete, deterministic way back to the
// world as it was found.
//
// Two properties carry that guarantee and are worth stating where the code is:
//
//   - Write-ahead. The journal is complete at BeginTxn and never appended to
//     during Apply, so no crash can leave a step applied but unrecorded. The
//     price is that rollback restores every recorded step rather than only the
//     ones that ran, which is safe because every restore is idempotent: putting
//     back bytes that were never disturbed rewrites them unchanged.
//   - Per-file atomicity. Every content write lands in a temp file beside its
//     destination and arrives by rename, so a destination is only ever the old
//     complete bytes or the new complete bytes — never a torn mixture.

const (
	journalFormatVersion = 1
	journalPlanFile      = "plan.json"
	journalPlanTemp      = "plan.json.tmp"
	journalBackupDir     = "backup"

	// journalDirMode keeps rollback material private; a pre-image can hold
	// whatever the user's own file held.
	journalDirMode = 0o700
	// targetDirMode and targetFileMode are the modes for directories and files
	// the installer itself creates. An updated file keeps the mode it had.
	targetDirMode  = 0o755
	targetFileMode = 0o644
)

var (
	// ErrApplyFailed wraps the step error of an apply that could not finish. It
	// is returned only after the rollback has run, so a caller seeing it knows
	// the world is back as it was found.
	ErrApplyFailed = errors.New("install: transaction apply failed")
	// ErrPlanConflict refuses a plan carrying an inspection the installer has no
	// licence to write. Deciding what to tell the user about a conflict belongs
	// to the operation layer; refusing to touch it belongs here.
	ErrPlanConflict = errors.New("install: transaction plan contains a conflict")
	// ErrPlanStale refuses a plan whose destination no longer looks like the one
	// that was inspected: a create whose destination has since appeared, or an
	// update whose destination has since vanished. It is a race, not a
	// misjudgement, but it is the same dead end for the user — docket cannot
	// prove it may write here — so it wraps ErrPlanConflict and reaches the same
	// report, while staying separately matchable for callers that care which of
	// the two happened.
	ErrPlanStale = fmt.Errorf("%w: the plan no longer describes what is on disk", ErrPlanConflict)
	// ErrJournalInvalid is every unusable journal: absent, unparseable, of an
	// unknown format, or describing paths a rollback must not act on.
	ErrJournalInvalid = errors.New("install: transaction journal invalid")
)

// stepAction records what the step did to the destination, derived from what
// was actually there when the pre-image was captured rather than from the
// earlier inspection — the journal must describe what it can restore.
type stepAction string

const (
	actionCreate stepAction = "create"
	actionUpdate stepAction = "update"
	actionRemove stepAction = "remove"
)

// preImageState is what the destination held before the transaction touched it,
// and therefore what a rollback has to reinstate.
type preImageState string

const (
	preAbsent  preImageState = "absent"
	preFile    preImageState = "file"
	preSymlink preImageState = "symlink"
)

// preImage is the recorded pre-state of one destination.
type preImage struct {
	State      preImageState `json:"state"`
	Backup     string        `json:"backup,omitempty"`      // slash-relative path inside the journal
	Mode       uint32        `json:"mode,omitempty"`        // file only: the permissions to restore
	LinkTarget string        `json:"link_target,omitempty"` // symlink only: the raw link text
}

// journalStep is one ordered apply step as the journal records it. It carries
// what rollback needs and nothing else: the desired content stays in memory,
// because recovery only ever undoes a transaction, never resumes one.
type journalStep struct {
	Seq       int        `json:"seq"`
	Path      string     `json:"path"`
	Kind      TargetKind `json:"kind"`
	BlockName string     `json:"block_name,omitempty"`
	Remove    bool       `json:"remove,omitempty"` // the step deletes the destination rather than writing it
	// Disposition is what the inspection decided this destination was, carried
	// through so the capture can check its own reading of the disk against the
	// one the decision was made on. Empty on a removal, which carries no
	// inspection at all.
	Disposition Disposition `json:"disposition,omitempty"`
	Action      stepAction  `json:"action"`
	Staging     string      `json:"staging"`                // same-directory temp file this step writes through
	CreatedDirs []string    `json:"created_dirs,omitempty"` // ancestors absent when the plan was made
	PreImage    preImage    `json:"pre_image"`
}

// journal is the on-disk plan.json.
type journal struct {
	FormatVersion int           `json:"format_version"`
	TxnID         string        `json:"txn_id"`
	Steps         []journalStep `json:"steps"`
}

type txnPhase int

const (
	phaseOpen txnPhase = iota
	phaseApplied
	phaseFinished
)

// Txn is one journaled installation transaction.
type Txn struct {
	fs      FSOps
	dir     string   // the journal directory
	journal journal  // exactly what plan.json holds
	targets []Target // desired state, parallel to journal.Steps; never persisted
	phase   txnPhase
}

// ID is the transaction identifier, which is also the journal directory name a
// later recovery names.
func (t *Txn) ID() string { return t.journal.TxnID }

// BeginTxn writes the journal for inspections and returns the transaction ready
// to apply. Nothing outside the journal directory is touched: when BeginTxn
// returns an error, no destination has been changed and no journal is left for
// a later run to find.
//
// Only create and update inspections become steps. A no-op is dropped — an
// installation that rewrites unchanged files churns mtimes for nothing — and a
// conflict is refused outright.
//
// Each step's disposition travels with it into the journal, and the pre-image
// capture refuses the transaction when the disk no longer agrees with it: the
// inspection and the write have to be about the same world, or the ownership
// judgement licensing the write was made about a file that is no longer there.
// See agreesWithDisposition.
func BeginTxn(fsops FSOps, roots UserRoots, inspections []Inspection) (*Txn, error) {
	return BeginTxnWithRemovals(fsops, roots, inspections, nil)
}

// BeginTxnWithRemovals is BeginTxn plus deletions: removals name targets a
// previous installation recorded owning that the new plan no longer contains.
// They travel inside the same transaction as the writes because they carry the
// same risk — a removal whose pre-image is not journaled is a deletion nothing
// can undo — so each one's bytes (or link) are captured before anything runs.
//
// KindFile and KindSymlink removals delete the destination outright. A
// KindManagedBlock removal is different: the block lives inside a file the user
// also owns, so retiring it rewrites the file with only that block's own lines
// gone — through the same staged rename a write uses — and requires the record
// to name the block. Either way the pre-image is captured before anything runs,
// so no removal happens whose undo is not already journaled.
func BeginTxnWithRemovals(fsops FSOps, roots UserRoots, inspections []Inspection, removals []TargetRecord) (*Txn, error) {
	if fsops == nil {
		return nil, errors.New("install: BeginTxn requires a filesystem")
	}

	id, err := newTxnID()
	if err != nil {
		return nil, err
	}
	steps, targets, err := planSteps(id, inspections, removals)
	if err != nil {
		return nil, err
	}

	dir := filepath.Join(roots.TransactionsDir(), id)
	if err := fsops.MkdirAll(filepath.Join(dir, journalBackupDir), journalDirMode); err != nil {
		return nil, fmt.Errorf("install: creating transaction directory %s: %w", dir, err)
	}
	// A journal that was never finished is worse than no journal: it would
	// invite a later run to recover a transaction that never began. Every exit
	// before plan.json is renamed into place takes the whole directory with it.
	published := false
	defer func() {
		if !published {
			_ = removeTree(fsops, dir)
		}
	}()

	for i := range steps {
		if err := capturePreImage(fsops, dir, &steps[i]); err != nil {
			return nil, err
		}
	}

	j := journal{FormatVersion: journalFormatVersion, TxnID: id, Steps: steps}
	if err := writeJournal(fsops, dir, j); err != nil {
		return nil, err
	}
	published = true

	return &Txn{fs: fsops, dir: dir, journal: j, targets: targets, phase: phaseOpen}, nil
}

// Apply verifies every journaled pre-image, then executes every step in plan
// order. A destination that has changed since the journal was written refuses
// the apply outright, with ErrPlanStale and nothing mutated — see
// verifyPreImages. Once the first step has run, the failure mode is the other
// one: on the first error it rolls the whole transaction back and returns the
// step error wrapped in ErrApplyFailed; if the rollback itself fails, both
// errors are returned and the journal is left behind so a later Recover can
// finish the job.
func (t *Txn) Apply() error {
	if t.phase != phaseOpen {
		return fmt.Errorf("install: transaction %s is not open for apply", t.journal.TxnID)
	}
	if err := t.verifyPreImages(); err != nil {
		// Nothing has been applied — the check runs before the first step — so
		// there is nothing to undo, and undoing anyway would be destructive: a
		// rollback restores what the journal recorded, and for a step recorded
		// absent that means DELETING whatever is at the path, which is precisely
		// the file the check just refused to overwrite. Discarding the journal is
		// the only exit that touches nothing; leaving it would hand a later
		// Recover the same deletion to perform.
		t.phase = phaseFinished
		if rmErr := removeTree(t.fs, t.dir); rmErr != nil {
			return errors.Join(err, fmt.Errorf("install: removing transaction %s: %w", t.journal.TxnID, rmErr))
		}
		return err
	}
	if err := t.applySteps(); err != nil {
		applyErr := fmt.Errorf("%w: %w", ErrApplyFailed, err)
		if rbErr := t.Rollback(); rbErr != nil {
			return errors.Join(applyErr, rbErr)
		}
		return applyErr
	}
	t.phase = phaseApplied
	return nil
}

// verifyPreImages re-reads every journaled destination and refuses the apply
// when one no longer holds what the journal recorded. At apply time the
// pre-image is the authority: it is what the rollback would put back, so a
// destination that has diverged from it is one this transaction can no longer
// change and then undo.
//
// It runs as a pass of its own, before the first step, because the fail-closed
// answer here cannot be a rollback (see Apply). Whole-plan verification also
// beats a per-kind guard inside applyStep: KindSymlink's own EEXIST refusal
// stops the overwrite but only after earlier steps have run, and the rollback
// that follows is what deletes the intruding file.
//
// Kinds are compared, not contents. Byte-comparing every pre-image would put a
// full read of the installation on the apply path to close a window measured in
// microseconds, and it would still be a window — the residual race is a
// destination rewritten in place, with its kind unchanged, between this pass and
// its step.
func (t *Txn) verifyPreImages() error {
	for _, step := range t.journal.Steps {
		if step.Remove && step.Kind != KindManagedBlock {
			// A whole-file removal deletes whatever it finds and restores what it
			// captured; a target that vanished on its own has simply arrived early.
			// A managed-block removal is exempt from that exemption: it rewrites the
			// file in place, so it is verified like an update — the destination must
			// still hold what the journal recorded, or the rewrite has nothing it
			// can safely undo.
			continue
		}
		info, err := os.Lstat(step.Path)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			if step.PreImage.State != preAbsent {
				return fmt.Errorf("%w: %s held %s when the transaction began, and holds nothing now",
					ErrPlanStale, step.Path, describePreImageState(step.PreImage.State))
			}
		case err != nil:
			return fmt.Errorf("install: inspecting %s: %w", step.Path, err)
		default:
			if have := observedPreImageState(info); have != step.PreImage.State {
				return fmt.Errorf("%w: %s held %s when the transaction began, and holds %s now",
					ErrPlanStale, step.Path,
					describePreImageState(step.PreImage.State), describePreImageState(have))
			}
		}
	}
	return nil
}

// observedPreImageState classifies what is on disk in the journal's own terms.
// Anything the journal cannot record a pre-image for reports as an empty state,
// which matches nothing a capture ever wrote.
func observedPreImageState(info fs.FileInfo) preImageState {
	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		return preSymlink
	case info.Mode().IsRegular():
		return preFile
	default:
		return ""
	}
}

// describePreImageState names a pre-image state the way the refusal message
// needs it: as what the user would see at the path.
func describePreImageState(s preImageState) string {
	switch s {
	case preAbsent:
		return "nothing"
	case preFile:
		return "a regular file"
	case preSymlink:
		return "a symlink"
	default:
		return "neither a regular file nor a symlink"
	}
}

// applySteps is Apply without its rollback. It is separate so a test can stop a
// transaction the way a killed process does — mid-plan, with the journal still
// on disk and nobody left to undo it.
func (t *Txn) applySteps() error {
	for i := range t.journal.Steps {
		if err := t.applyStep(i); err != nil {
			return fmt.Errorf("step %d (%s): %w", t.journal.Steps[i].Seq, t.journal.Steps[i].Path, err)
		}
	}
	return nil
}

func (t *Txn) applyStep(i int) error {
	step := &t.journal.Steps[i]
	target := t.targets[i]

	if step.Remove {
		if step.Kind == KindManagedBlock {
			// A managed-block removal rewrites the file with only that block's
			// lines gone, keeping every surrounding byte; it is not a whole-file
			// deletion.
			return t.removeManagedBlock(step)
		}
		// Nothing is created on the way to a deletion, so the destination's
		// directory is left exactly as it was found — including absent.
		return removeIfPresent(t.fs, step.Path)
	}

	if err := t.fs.MkdirAll(filepath.Dir(step.Path), targetDirMode); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(step.Path), err)
	}

	switch step.Kind {
	case KindFile:
		return t.writeThroughStaging(step, target.Content, target.Mode)

	case KindManagedBlock:
		// A block rewrite reads through a symlink but publishes by rename, which
		// would replace the link itself with a regular file. Ownership never
		// authorizes that — a symlinked destination cannot carry a recorded block
		// interior — so a link found here appeared after the inspection, and the
		// transaction refuses rather than quietly eating the user's link.
		if step.PreImage.State == preSymlink {
			return fmt.Errorf("%w: %s is a symlink, which cannot carry a managed block",
				ErrInvalidTarget, step.Path)
		}
		data, err := renderManagedBlock(step.Path, target)
		if err != nil {
			return err
		}
		return t.writeThroughStaging(step, data, target.Mode)

	case KindSymlink:
		// Symlink refuses an occupied path, so an update clears the old one
		// first. A destination that has appeared since the plan was made is left
		// alone here as the last line of defence — verifyPreImages has already
		// refused the whole apply for it, and did so before any step ran, which
		// is what keeps the rollback from deleting the file this branch declines
		// to overwrite.
		if step.PreImage.State != preAbsent {
			if err := removeIfPresent(t.fs, step.Path); err != nil {
				return err
			}
		}
		if err := t.fs.Symlink(target.LinkTarget, step.Path); err != nil {
			return fmt.Errorf("linking %s: %w", step.Path, err)
		}
		return nil

	default:
		return fmt.Errorf("%w: %s has unknown kind %q", ErrInvalidTarget, step.Path, step.Kind)
	}
}

// writeThroughStaging is the only way content reaches a destination: a
// same-directory temp file, then a rename. Same-directory keeps the rename on
// one filesystem, which is what makes it atomic.
func (t *Txn) writeThroughStaging(step *journalStep, data []byte, want os.FileMode) error {
	mode := os.FileMode(targetFileMode)
	if step.PreImage.State == preFile && step.PreImage.Mode != 0 {
		// An update keeps the permissions the file already had: the mode is the
		// user's, even when the content is docket's.
		mode = os.FileMode(step.PreImage.Mode).Perm()
	}
	if want != 0 {
		// A target that names its own mode overrides both: the one target that
		// does is the development binary, whose executability is not the user's
		// to have set.
		mode = want.Perm()
	}
	if err := t.fs.WriteFile(step.Staging, data, mode); err != nil {
		return fmt.Errorf("staging %s: %w", step.Path, err)
	}
	// WriteFile's mode only applies at creation, where the process umask filters
	// it — under a restrictive umask a 0o755 binary would land 0o700. The mode
	// here is policy, not a ceiling, so it is enforced with an explicit chmod on
	// the staging file, before the rename, so the destination never holds the
	// right bytes under the wrong permissions.
	if err := t.fs.Chmod(step.Staging, mode); err != nil {
		return fmt.Errorf("setting the mode of %s: %w", step.Path, err)
	}
	if err := t.fs.Rename(step.Staging, step.Path); err != nil {
		return fmt.Errorf("publishing %s: %w", step.Path, err)
	}
	return nil
}

// renderManagedBlock produces the complete new bytes of a file carrying one
// managed block. Everything outside the block is copied through untouched: the
// surrounding bytes are the user's, and an install that reflows them has eaten
// content it was never given.
func renderManagedBlock(path string, target Target) ([]byte, error) {
	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	doc, err := document.Parse(existing)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	var patch document.PatchSet
	if _, ok := doc.Block(target.BlockName); ok {
		patch.ReplaceBlock(target.BlockName, string(target.Content))
	} else {
		// A block cannot precede frontmatter without demoting it to prose, so a
		// document that has frontmatter takes the block immediately after it.
		at := document.AtDocumentStart
		if doc.HasFrontmatter() {
			at = document.AfterFrontmatter
		}
		patch.InsertBlock(target.BlockName, target.Annotation, string(target.Content), at)
	}
	out, err := doc.Apply(patch)
	if err != nil {
		return nil, fmt.Errorf("rewriting the %s block in %s: %w", target.BlockName, path, err)
	}
	return out, nil
}

// removeManagedBlock retires one managed block from a file the user also owns,
// rewriting it with exactly that block's marker-to-marker lines gone and every
// surrounding byte untouched. It is the deletion counterpart of
// renderManagedBlock and publishes through the same staged rename, so the
// journal's pre-image covers the whole rewrite for rollback.
func (t *Txn) removeManagedBlock(step *journalStep) error {
	switch step.PreImage.State {
	case preAbsent:
		// The file that carried the block is already gone: there is nothing to
		// retire and no bytes to rewrite. verifyPreImages has confirmed the disk
		// still agrees, so this is the "already retired" no-op, not a lost file.
		return nil
	case preSymlink:
		// A rewrite publishes by rename, which would replace the link itself with
		// a regular file. A managed block is never installed through a symlink
		// (see the KindManagedBlock write branch), so a link here appeared after
		// the record was written and the transaction refuses rather than quietly
		// eating the user's link.
		return fmt.Errorf("%w: %s is a symlink, which cannot carry a managed block",
			ErrInvalidTarget, step.Path)
	}
	existing, err := os.ReadFile(step.Path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", step.Path, err)
	}
	doc, err := document.Parse(existing)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", step.Path, err)
	}
	var patch document.PatchSet
	patch.RemoveBlock(step.BlockName)
	out, err := doc.Apply(patch)
	if err != nil {
		return fmt.Errorf("retiring the %s block in %s: %w", step.BlockName, step.Path, err)
	}
	// want == 0 keeps the file's recorded pre-image mode: the surrounding bytes
	// are the user's, and so is the mode.
	return t.writeThroughStaging(step, out, 0)
}

// Rollback restores every journaled destination, newest first, and removes the
// journal once the world is back as it was found. A rollback that cannot finish
// leaves the journal in place on purpose: it is the only remaining record of
// what still has to be undone.
func (t *Txn) Rollback() error {
	if t.phase == phaseFinished {
		return fmt.Errorf("install: transaction %s is already finished", t.journal.TxnID)
	}
	if err := rollbackJournal(t.fs, t.dir, t.journal); err != nil {
		return err
	}
	if err := removeTree(t.fs, t.dir); err != nil {
		return fmt.Errorf("install: removing transaction %s: %w", t.journal.TxnID, err)
	}
	t.phase = phaseFinished
	return nil
}

// Commit publishes the installation record and only then removes the journal.
// The order is the recovery contract: until state/install.json is on disk the
// transaction is unpublished, and an unpublished transaction is one a later run
// must be able to find and undo.
func (t *Txn) Commit(statePath string, s *State) error {
	if t.phase != phaseApplied {
		return fmt.Errorf("install: transaction %s cannot commit before a successful apply", t.journal.TxnID)
	}
	if err := WriteStateAtomic(statePath, s); err != nil {
		return err
	}
	if err := removeTree(t.fs, t.dir); err != nil {
		return fmt.Errorf("install: removing transaction %s: %w", t.journal.TxnID, err)
	}
	t.phase = phaseFinished
	return nil
}

// DetectRecovery reports the oldest unpublished journal an interrupted run left
// behind. It only reads: reporting the need for recovery and performing it are
// separate so `install check` can say so without mutating anything.
//
// A directory without a plan.json is not a recovery: the journal is renamed into
// place before any destination is touched, so an interruption before that point
// changed nothing.
//
// What it CANNOT report is whose journal it found. A live transaction and an
// abandoned one are the same bytes on disk; only the exclusive installation
// lock separates them (see lock.go), which is why the operation layer detects
// under that lock and `install check` reports the ambiguity rather than
// resolving it.
func DetectRecovery(roots UserRoots) (string, bool, error) {
	dir := roots.TransactionsDir()
	entries, err := os.ReadDir(dir) // sorted by name, so the oldest id comes first
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("install: reading %s: %w", dir, err)
	}
	for _, e := range entries {
		if !e.IsDir() || !safeTxnID(e.Name()) {
			continue
		}
		_, err := os.Stat(filepath.Join(dir, e.Name(), journalPlanFile))
		switch {
		case err == nil:
			return e.Name(), true, nil
		case errors.Is(err, fs.ErrNotExist):
			continue
		default:
			return "", false, fmt.Errorf("install: reading transaction %s: %w", e.Name(), err)
		}
	}
	return "", false, nil
}

// Recover rolls the named journal back and removes it. It is deterministic: the
// journal alone decides what is restored and in what order, so recovering the
// same interruption twice produces the same world.
//
// It is only ever safe to call while holding the exclusive installation lock:
// the journal is evidence that a transaction BEGAN, never that it ended, so
// rolling one back without the lock can undo an installation that is still
// running. recoverPending is the one caller, and it refuses without the lock.
func Recover(fsops FSOps, roots UserRoots, txnID string) error {
	if fsops == nil {
		return errors.New("install: Recover requires a filesystem")
	}
	if !safeTxnID(txnID) {
		return fmt.Errorf("%w: %q is not a transaction identifier", ErrJournalInvalid, txnID)
	}
	dir := filepath.Join(roots.TransactionsDir(), txnID)
	j, err := readJournal(dir)
	if err != nil {
		return err
	}
	if err := rollbackJournal(fsops, dir, j); err != nil {
		return err
	}
	if err := removeTree(fsops, dir); err != nil {
		return fmt.Errorf("install: removing transaction %s: %w", txnID, err)
	}
	return nil
}

// rollbackJournal restores every step newest first, then prunes the directories
// the transaction created. Newest first is what makes repeated destinations —
// two managed blocks in one file — end at the original bytes.
func rollbackJournal(fsops FSOps, dir string, j journal) error {
	for i := len(j.Steps) - 1; i >= 0; i-- {
		if err := restoreStep(fsops, dir, j.Steps[i]); err != nil {
			return fmt.Errorf("install: rolling back transaction %s: %w", j.TxnID, err)
		}
	}
	pruneCreatedDirs(fsops, j)
	return nil
}

// restoreStep puts one destination back. Every branch is idempotent, which is
// what lets a write-ahead journal restore steps that may never have run.
func restoreStep(fsops FSOps, dir string, step journalStep) error {
	switch step.PreImage.State {
	case preAbsent:
		if err := removeIfPresent(fsops, step.Path); err != nil {
			return err
		}

	case preFile:
		backup := filepath.Join(dir, filepath.FromSlash(step.PreImage.Backup))
		data, err := os.ReadFile(backup)
		if err != nil {
			return fmt.Errorf("reading rollback material for %s: %w", step.Path, err)
		}
		mode := os.FileMode(step.PreImage.Mode).Perm()
		if mode == 0 {
			mode = targetFileMode
		}
		// The restore is published the same way the apply was, so an interrupted
		// rollback cannot leave a torn file either.
		if err := fsops.WriteFile(step.Staging, data, mode); err != nil {
			return fmt.Errorf("staging the restore of %s: %w", step.Path, err)
		}
		// The chmod mirrors writeThroughStaging, and for the same reason: the
		// recorded pre-image mode is what the user had, and a restore that lets
		// the umask narrow it has not put the world back.
		if err := fsops.Chmod(step.Staging, mode); err != nil {
			return fmt.Errorf("setting the mode of the restore of %s: %w", step.Path, err)
		}
		if err := fsops.Rename(step.Staging, step.Path); err != nil {
			return fmt.Errorf("restoring %s: %w", step.Path, err)
		}
		return nil

	case preSymlink:
		if err := removeIfPresent(fsops, step.Path); err != nil {
			return err
		}
		if err := fsops.Symlink(step.PreImage.LinkTarget, step.Path); err != nil {
			return fmt.Errorf("restoring the link %s: %w", step.Path, err)
		}

	default:
		return fmt.Errorf("%w: step %d records unknown pre-image state %q",
			ErrJournalInvalid, step.Seq, step.PreImage.State)
	}
	// A staging file only survives when its rename never happened; leaving one
	// beside a user's file would be litter this transaction owns.
	return removeIfPresent(fsops, step.Staging)
}

// pruneCreatedDirs removes the directories the transaction created, deepest
// first and only while empty. Failure is deliberately ignored: a directory
// somebody else has since put a file in is theirs, and rollback is complete
// without it.
func pruneCreatedDirs(fsops FSOps, j journal) {
	seen := map[string]bool{}
	var dirs []string
	for _, step := range j.Steps {
		for _, d := range step.CreatedDirs {
			if !seen[d] {
				seen[d] = true
				dirs = append(dirs, d)
			}
		}
	}
	// A child path is always longer than its parent, so longest-first is
	// deepest-first.
	sort.Slice(dirs, func(i, k int) bool {
		if len(dirs[i]) != len(dirs[k]) {
			return len(dirs[i]) > len(dirs[k])
		}
		return dirs[i] > dirs[k]
	})
	for _, d := range dirs {
		_ = fsops.Remove(d)
	}
}

// planSteps turns inspections into the ordered, validated step list. Ordering is
// by destination path so the same plan always applies in the same sequence
// whatever order the harness adapters produced it in; the sort is stable, so two
// blocks in one file keep the order their planner chose.
func planSteps(txnID string, inspections []Inspection, removals []TargetRecord) ([]journalStep, []Target, error) {
	ordered := make([]plannedStep, 0, len(inspections)+len(removals))
	for _, insp := range inspections {
		switch insp.Disposition {
		case DispositionNoop:
			continue
		case DispositionConflict:
			return nil, nil, fmt.Errorf("%w: %s (%s)", ErrPlanConflict, insp.Target.Path, insp.ConflictDetail())
		case DispositionCreate, DispositionUpdate:
		default:
			return nil, nil, fmt.Errorf("%w: %s carries unknown disposition %q",
				ErrInvalidTarget, insp.Target.Path, insp.Disposition)
		}
		if err := insp.Target.validate(); err != nil {
			return nil, nil, err
		}
		ordered = append(ordered, plannedStep{target: insp.Target, disposition: insp.Disposition})
	}
	for _, rec := range removals {
		target, err := removalTarget(rec)
		if err != nil {
			return nil, nil, err
		}
		ordered = append(ordered, plannedStep{target: target, remove: true})
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		return filepath.Clean(ordered[i].target.Path) < filepath.Clean(ordered[j].target.Path)
	})
	if err := rejectDuplicateDestinations(ordered); err != nil {
		return nil, nil, err
	}

	steps := make([]journalStep, len(ordered))
	targets := make([]Target, len(ordered))
	for i, planned := range ordered {
		path := filepath.Clean(planned.target.Path)
		steps[i] = journalStep{
			Seq:         i,
			Path:        path,
			Kind:        planned.target.Kind,
			BlockName:   planned.target.BlockName,
			Remove:      planned.remove,
			Disposition: planned.disposition,
			Staging:     stagingPath(path, txnID, i),
		}
		if !planned.remove {
			steps[i].CreatedDirs = missingAncestors(path)
		}
		targets[i] = planned.target
	}
	return steps, targets, nil
}

// plannedStep pairs a desired target with what the transaction does to it and
// with the inspection's reading of the destination, which the capture then has
// to find still true.
type plannedStep struct {
	target      Target
	remove      bool
	disposition Disposition
}

// removalTarget turns a prior ownership record into the step that retires it.
// The record is the only description of what is being deleted, so a record that
// cannot describe a deletable thing refuses the whole transaction.
func removalTarget(rec TargetRecord) (Target, error) {
	if rec.Path == "" || !filepath.IsAbs(rec.Path) {
		return Target{}, fmt.Errorf("%w: a removal names %q, which is not an absolute path",
			ErrInvalidTarget, rec.Path)
	}
	switch rec.Kind {
	case KindFile, KindSymlink:
		return Target{
			Path:       rec.Path,
			Kind:       rec.Kind,
			LinkTarget: rec.LinkTarget,
			Role:       rec.Role,
		}, nil
	case KindManagedBlock:
		// The block is what the removal deletes, so a record that cannot name it
		// describes no deletable thing and refuses the whole transaction.
		if rec.BlockName == "" {
			return Target{}, fmt.Errorf("%w: %s is recorded as a managed block with no block name",
				ErrInvalidTarget, rec.Path)
		}
		return Target{
			Path:      rec.Path,
			Kind:      rec.Kind,
			BlockName: rec.BlockName,
			Role:      rec.Role,
		}, nil
	default:
		return Target{}, fmt.Errorf("%w: %s is recorded as %q, which a transaction never deletes",
			ErrInvalidTarget, rec.Path, rec.Kind)
	}
}

// rejectDuplicateDestinations refuses a plan that writes the same destination
// twice. Two managed blocks in one file are legitimate — they touch disjoint
// bytes — but anything else is a planner defect whose outcome would depend on
// step order.
//
// A removal and a write of one destination in the same plan is a defect of the
// same kind, and is refused for the same reason.
func rejectDuplicateDestinations(ordered []plannedStep) error {
	seen := map[string]plannedStep{}
	blocks := map[string]bool{}
	for _, planned := range ordered {
		path := filepath.Clean(planned.target.Path)
		prev, ok := seen[path]
		if !ok {
			seen[path] = planned
			if planned.target.Kind == KindManagedBlock {
				blocks[path+"\x00"+planned.target.BlockName] = true
			}
			continue
		}
		if prev.remove || planned.remove ||
			prev.target.Kind != KindManagedBlock || planned.target.Kind != KindManagedBlock {
			return fmt.Errorf("%w: %s is written by more than one step", ErrInvalidTarget, path)
		}
		key := path + "\x00" + planned.target.BlockName
		if blocks[key] {
			return fmt.Errorf("%w: the %s block in %s is written by more than one step",
				ErrInvalidTarget, planned.target.BlockName, path)
		}
		blocks[key] = true
	}
	return nil
}

// capturePreImage records what the destination holds now, writing file bytes
// into the journal. Only a regular file, a symlink, or nothing at all can be
// restored, so anything else refuses the whole transaction rather than
// promising a rollback it could not deliver.
//
// The capture is also where the plan's decision meets the disk a second time:
// what is found here has to agree with the disposition the inspection produced,
// or the two halves of the operation are reasoning about different worlds.
func capturePreImage(fsops FSOps, dir string, step *journalStep) error {
	info, err := os.Lstat(step.Path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if err := agreesWithDisposition(step, false); err != nil {
			return err
		}
		step.PreImage = preImage{State: preAbsent}
		step.Action = actionCreate
		if step.Remove {
			// A removal of something already gone still runs: every restore is
			// idempotent, and a vanished target must not block an upgrade.
			step.Action = actionRemove
		}
		return nil
	case err != nil:
		return fmt.Errorf("install: inspecting %s: %w", step.Path, err)
	}
	if err := agreesWithDisposition(step, true); err != nil {
		return err
	}

	step.Action = actionUpdate
	if step.Remove {
		step.Action = actionRemove
	}
	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		dest, err := os.Readlink(step.Path)
		if err != nil {
			return fmt.Errorf("install: reading link %s: %w", step.Path, err)
		}
		step.PreImage = preImage{State: preSymlink, LinkTarget: dest}
		return nil

	case info.Mode().IsRegular():
		data, err := os.ReadFile(step.Path)
		if err != nil {
			return fmt.Errorf("install: reading %s: %w", step.Path, err)
		}
		rel := journalBackupDir + "/" + strconv.Itoa(step.Seq)
		if err := fsops.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), data, 0o600); err != nil {
			return fmt.Errorf("install: recording rollback material for %s: %w", step.Path, err)
		}
		step.PreImage = preImage{State: preFile, Backup: rel, Mode: uint32(info.Mode().Perm())}
		return nil

	default:
		return fmt.Errorf("%w: %s is neither a regular file nor a symlink, so no rollback material exists",
			ErrInvalidTarget, step.Path)
	}
}

// agreesWithDisposition refuses a step whose destination is no longer the one
// the inspection classified. present says what the capture just found there.
//
// Both directions refuse, for one reason: an installer decides and acts on the
// same copy of the world, and neither disagreement can be resolved without
// re-deciding on a copy nobody classified.
//
//   - A create that finds something has found a file nothing has proven is
//     docket's. Proceeding would rename over it on the strength of an ownership
//     judgement made when the path was empty.
//   - An update that finds nothing has lost the thing it was licensed to
//     rewrite. Writing it anyway would be a create the inspection never
//     authorised — the path may have been freed for somebody else's file, and
//     the next run classifies it honestly.
//
// The refusal costs the user one re-run, which is the cheapest possible answer:
// a fresh inspection sees whatever is there now and decides again.
func agreesWithDisposition(step *journalStep, present bool) error {
	if step.Remove {
		// A removal carries no inspection. Its licence is the prior ownership
		// record the operation layer already checked, and a retired target that
		// has already vanished must not block the upgrade that retires it.
		return nil
	}
	switch step.Disposition {
	case DispositionCreate:
		if present {
			return fmt.Errorf("%w: %s was absent when it was inspected, and something is there now",
				ErrPlanStale, step.Path)
		}
	case DispositionUpdate:
		if !present {
			return fmt.Errorf("%w: %s was present when it was inspected, and is gone now",
				ErrPlanStale, step.Path)
		}
	default:
		// planSteps admits only creates and updates, so this is a planner defect
		// rather than a race — but a step whose decision cannot be checked is the
		// one thing this function must never wave through.
		return fmt.Errorf("%w: %s carries disposition %q where a create or an update belongs",
			ErrInvalidTarget, step.Path, step.Disposition)
	}
	return nil
}

// writeJournal publishes plan.json by rename: its presence is what marks the
// transaction live, so it must never be half-written.
func writeJournal(fsops FSOps, dir string, j journal) error {
	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return fmt.Errorf("install: encoding the transaction journal: %w", err)
	}
	data = append(data, '\n')
	tmp := filepath.Join(dir, journalPlanTemp)
	if err := fsops.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("install: writing %s: %w", tmp, err)
	}
	if err := fsops.Rename(tmp, filepath.Join(dir, journalPlanFile)); err != nil {
		return fmt.Errorf("install: publishing %s: %w", filepath.Join(dir, journalPlanFile), err)
	}
	return nil
}

// readJournal loads and validates plan.json. The journal is about to drive
// writes, so every field it steers is checked before a single one happens: a
// corrupted journal must fail, never misfire.
func readJournal(dir string) (journal, error) {
	data, err := os.ReadFile(filepath.Join(dir, journalPlanFile))
	if err != nil {
		return journal{}, fmt.Errorf("%w: reading %s: %s", ErrJournalInvalid, dir, err)
	}
	var j journal
	if err := json.Unmarshal(data, &j); err != nil {
		return journal{}, fmt.Errorf("%w: parsing %s: %s", ErrJournalInvalid, dir, err)
	}
	if j.FormatVersion != journalFormatVersion {
		return journal{}, fmt.Errorf("%w: %s has format_version %d (want %d)",
			ErrJournalInvalid, dir, j.FormatVersion, journalFormatVersion)
	}
	for _, step := range j.Steps {
		switch {
		case !filepath.IsAbs(step.Path):
			return journal{}, fmt.Errorf("%w: step %d names a relative path %q", ErrJournalInvalid, step.Seq, step.Path)
		case !filepath.IsAbs(step.Staging):
			return journal{}, fmt.Errorf("%w: step %d names a relative staging path %q", ErrJournalInvalid, step.Seq, step.Staging)
		case filepath.Dir(step.Staging) != filepath.Dir(step.Path):
			return journal{}, fmt.Errorf("%w: step %d stages %q outside the directory of %q",
				ErrJournalInvalid, step.Seq, step.Staging, step.Path)
		}
		for _, d := range step.CreatedDirs {
			if !filepath.IsAbs(d) {
				return journal{}, fmt.Errorf("%w: step %d names a relative created directory %q",
					ErrJournalInvalid, step.Seq, d)
			}
		}
		if step.PreImage.State == preFile && !safeBackupRef(step.PreImage.Backup) {
			return journal{}, fmt.Errorf("%w: step %d names rollback material %q outside the journal",
				ErrJournalInvalid, step.Seq, step.PreImage.Backup)
		}
	}
	return j, nil
}

// safeBackupRef reports whether ref names a file this journal owns: a
// slash-relative path directly under backup/, with no traversal.
func safeBackupRef(ref string) bool {
	rest, ok := strings.CutPrefix(ref, journalBackupDir+"/")
	if !ok || rest == "" {
		return false
	}
	return !strings.ContainsAny(rest, "/\\") && rest != "." && rest != ".."
}

// safeTxnID reports whether id may be joined onto the transactions directory:
// exactly one path segment, naming no ancestor.
func safeTxnID(id string) bool {
	if id == "" || id == "." || id == ".." {
		return false
	}
	if strings.ContainsAny(id, `/\`) || strings.ContainsRune(id, filepath.Separator) {
		return false
	}
	return id == filepath.Base(id)
}

// newTxnID mints a sortable, unique transaction identifier. The timestamp
// prefix makes directory order recovery order; the random suffix keeps two runs
// in the same second apart.
func newTxnID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("install: minting a transaction id: %w", err)
	}
	return time.Now().UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(b[:]), nil
}

// stagingPath is the temp file a step writes through: beside its destination so
// the rename is atomic, and named after the transaction and step so two runs can
// never collide.
func stagingPath(dest, txnID string, seq int) string {
	return filepath.Join(filepath.Dir(dest), fmt.Sprintf(".docket-install-%s-%d.tmp", txnID, seq))
}

// missingAncestors lists the directories above path that do not exist yet,
// outermost first. They are computed before anything is created, so a rollback
// knows exactly which directories are the transaction's to prune.
func missingAncestors(path string) []string {
	var missing []string
	for dir := filepath.Dir(path); ; {
		if _, err := os.Lstat(dir); err == nil || !errors.Is(err, fs.ErrNotExist) {
			break
		}
		missing = append(missing, dir)
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	for i, j := 0, len(missing)-1; i < j; i, j = i+1, j-1 {
		missing[i], missing[j] = missing[j], missing[i]
	}
	return missing
}

// removeIfPresent deletes path, treating "already gone" as success: every
// rollback branch has to be safe to run on a step that never ran.
func removeIfPresent(fsops FSOps, path string) error {
	if path == "" {
		return nil
	}
	if err := fsops.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("removing %s: %w", path, err)
	}
	return nil
}

// removeTree deletes a journal directory and its contents through the seam, so
// a test can watch the cleanup fail too. It is only ever pointed at directories
// this package created.
func removeTree(fsops FSOps, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("reading %s: %w", dir, err)
	}
	for _, e := range entries {
		child := filepath.Join(dir, e.Name())
		if e.IsDir() {
			if err := removeTree(fsops, child); err != nil {
				return err
			}
			continue
		}
		if err := removeIfPresent(fsops, child); err != nil {
			return err
		}
	}
	return removeIfPresent(fsops, dir)
}
