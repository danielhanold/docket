package workspace

// This file owns the on-disk workspace manifest: the hashed per-workspace
// directory under the repository's common directory, the three-outcome load, and
// the crash-safe atomic write. The manifest is PROVENANCE, not an oracle — every
// operation still checks live Git registration, branch identity, and object
// reachability. A directory name derived from the caller's branch string would
// leak that string into the filesystem, so the directory is named by the hex
// sha256 of the fully qualified feature ref instead.
//
// Atomic write and mode discipline follow internal/repository/transaction's
// candidate.go exactly (reference only — this package must not import that one):
// a same-directory temp file, file sync, Chmod, atomic rename, and directory
// sync, with the private 0700/0600 modes forced by explicit Chmod so a permissive
// umask cannot loosen them (learnings: promised-file-mode-needs-explicit-chmod).

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
)

const (
	// manifestSchemaVersion is the on-disk manifest format version. A load of any
	// other schema is an error, never clean absence.
	manifestSchemaVersion = 1

	// manifestFileName is the published manifest inside a workspace directory.
	manifestFileName = "manifest.json"

	// workspaceDirMode and workspaceFileMode are the documented private modes,
	// forced with an explicit Chmod after creation because O_CREATE / Mkdir perms
	// are masked by the process umask and this tree must stay private regardless.
	workspaceDirMode  os.FileMode = 0o700
	workspaceFileMode os.FileMode = 0o600
)

// Phase is the coarse lifecycle stage recorded in a manifest. It is provenance
// for a resuming or cleaning run (paired with live Git checks), never authority
// on its own.
type Phase string

const (
	PhaseAllocating Phase = "allocating"
	PhaseReady      Phase = "ready"
	PhaseCleaned    Phase = "cleaned" // tombstone: retained after cleanup
)

// valid reports whether p is one of the known phases.
func (p Phase) valid() bool {
	switch p {
	case PhaseAllocating, PhaseReady, PhaseCleaned:
		return true
	default:
		return false
	}
}

// canAdvanceTo reports whether p may advance to next. The lifecycle is a strict
// monotonic chain allocating→ready→cleaned: every other transition, including
// any backward move or a skip, is refused.
func (p Phase) canAdvanceTo(next Phase) bool {
	switch p {
	case PhaseAllocating:
		return next == PhaseReady
	case PhaseReady:
		return next == PhaseCleaned
	default:
		return false
	}
}

// Manifest is a workspace's published description. It carries enough repository
// identity (CommonDir) for a resuming or cleaning run to prove ownership before
// it touches anything, plus the exact refs and base commit an operation
// re-verifies against live Git. Timestamps are diagnostics only — never an
// oracle for liveness.
type Manifest struct {
	Schema     int             `json:"schema"`
	ID         string          `json:"id"`         // hex sha256 of feature ref (also the dir name)
	CommonDir  string          `json:"common_dir"` // canonical git common dir (ownership identity)
	ChangeID   domain.ChangeID `json:"change_id"`
	Slug       string          `json:"slug"`
	FeatureRef gitcli.RefName  `json:"feature_ref"`
	BaseRef    gitcli.RefName  `json:"base_ref"`
	BaseCommit gitcli.ObjectID `json:"base_commit"` // exact fetched base at first preparation
	Path       string          `json:"path"`        // canonical workspace path
	Phase      Phase           `json:"phase"`
	CreatedUTC string          `json:"created_utc"` // RFC3339 seconds, diagnostics only
	UpdatedUTC string          `json:"updated_utc"`
}

// workspaceID is the stable per-workspace id: the lowercase-hex sha256 of the
// fully qualified feature ref. It is directory-safe and reveals no branch string.
func workspaceID(ref gitcli.RefName) string {
	sum := sha256.Sum256([]byte(ref))
	return hex.EncodeToString(sum[:])
}

// workspacesRoot is the root of all workspace state for a repository:
// <commonDir>/docket/workspaces. It sits under the shared common directory so it
// is invisible to any working-tree status and shared across every linked
// worktree of the repository.
func workspacesRoot(commonDir string) string {
	return filepath.Join(commonDir, "docket", "workspaces")
}

// workspaceDir is the per-workspace directory root/<hex sha256(featureRef)>.
func workspaceDir(commonDir string, ref gitcli.RefName) string {
	return filepath.Join(workspacesRoot(commonDir), workspaceID(ref))
}

// validObjectID mirrors gitcli.validateObjectID's shape rules (that helper is
// package-private): a non-empty, all-lowercase-hex id of length 40 or 64.
func validObjectID(id gitcli.ObjectID) bool {
	s := string(id)
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// validManifestID reports whether id is a 64-char lowercase-hex sha256 digest.
func validManifestID(id string) bool {
	if len(id) != 64 {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// validateManifest rejects every field or identity violation. It is the single
// gate both writeManifest (before publishing) and loadManifest (after decoding)
// pass through, so a manifest that reaches disk and one that reads back valid
// obey identical rules.
func validateManifest(m Manifest) error {
	if m.Schema != manifestSchemaVersion {
		return fmt.Errorf("unknown schema %d", m.Schema)
	}
	if !validManifestID(m.ID) {
		return fmt.Errorf("invalid workspace id")
	}
	if m.CommonDir == "" || !filepath.IsAbs(m.CommonDir) {
		return fmt.Errorf("common dir is not an absolute path")
	}
	if m.ChangeID <= 0 {
		return fmt.Errorf("non-positive change id")
	}
	if !domain.ValidSlugToken(m.Slug) {
		return fmt.Errorf("invalid slug")
	}
	if err := validBranchRef(m.FeatureRef); err != nil {
		return fmt.Errorf("invalid feature ref: %w", err)
	}
	if err := validBranchRef(m.BaseRef); err != nil {
		return fmt.Errorf("invalid base ref: %w", err)
	}
	if !validObjectID(m.BaseCommit) {
		return fmt.Errorf("invalid base commit")
	}
	if m.Path == "" || !filepath.IsAbs(m.Path) {
		return fmt.Errorf("workspace path is not absolute")
	}
	if !m.Phase.valid() {
		return fmt.Errorf("invalid phase %q", m.Phase)
	}
	if _, err := time.Parse(time.RFC3339, m.CreatedUTC); err != nil {
		return fmt.Errorf("invalid created_utc")
	}
	if _, err := time.Parse(time.RFC3339, m.UpdatedUTC); err != nil {
		return fmt.Errorf("invalid updated_utc")
	}
	return nil
}

// loadManifest reads the manifest in dir with a strict three-outcome contract:
//   - (m, true, nil)   present and valid;
//   - (zero, false, nil) cleanly absent — os.IsNotExist on the exact manifest path;
//   - (zero, false, err) anything else: unreadable, truncated, unknown schema, or
//     any field/identity violation.
//
// Unknown NEVER reads as absent (learnings: probe-error-is-not-clean-absence):
// only a not-exist error on the manifest path is clean absence; a permission
// error, a decode error, or a validation failure is an error.
func loadManifest(dir string) (Manifest, bool, error) {
	data, err := os.ReadFile(filepath.Join(dir, manifestFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return Manifest{}, false, nil
		}
		return Manifest{}, false, fmt.Errorf("workspace: reading manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, false, fmt.Errorf("workspace: decoding manifest: %w", err)
	}
	if err := validateManifest(m); err != nil {
		return Manifest{}, false, fmt.Errorf("workspace: invalid manifest: %w", err)
	}
	return m, true, nil
}

// ensureDir creates dir (and any missing parents) and forces its mode to
// workspaceDirMode with an explicit Chmod so a permissive umask cannot loosen
// the leaf directory.
func ensureDir(dir string) error {
	if err := os.MkdirAll(dir, workspaceDirMode); err != nil {
		return fmt.Errorf("workspace: creating %s: %w", dir, err)
	}
	if err := os.Chmod(dir, workspaceDirMode); err != nil {
		return fmt.Errorf("workspace: setting mode on %s: %w", dir, err)
	}
	return nil
}

// writeManifest publishes m into dir crash-safely: it validates first, ensures
// the directory exists at 0700, writes a same-directory temp file, chmods it to
// 0600, fsyncs it, renames it over the manifest, and fsyncs the directory so the
// rename is durable. Any exit before the rename removes the temp file, so a
// failed publish leaves no stray sibling. The written bytes are reloaded and must
// round-trip equal, so a manifest that survives is one loadManifest accepts.
func writeManifest(dir string, m Manifest) error {
	if err := validateManifest(m); err != nil {
		return fmt.Errorf("workspace: refusing to write invalid manifest: %w", err)
	}
	if err := ensureDir(dir); err != nil {
		return err
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("workspace: encoding manifest: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, ".manifest.json.*.tmp")
	if err != nil {
		return fmt.Errorf("workspace: staging manifest: %w", err)
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("workspace: writing manifest: %w", err)
	}
	if err := tmp.Chmod(workspaceFileMode); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("workspace: setting mode on manifest temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("workspace: syncing manifest: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("workspace: closing manifest temp: %w", err)
	}
	if err := os.Rename(tmpName, filepath.Join(dir, manifestFileName)); err != nil {
		return fmt.Errorf("workspace: publishing manifest: %w", err)
	}
	if err := syncDir(dir); err != nil {
		return err
	}
	committed = true

	// Round-trip guard: the bytes just written must read back valid and equal.
	reloaded, present, err := loadManifest(dir)
	if err != nil {
		return fmt.Errorf("workspace: verifying written manifest: %w", err)
	}
	if !present || reloaded != m {
		return fmt.Errorf("workspace: written manifest did not round-trip equal")
	}
	return nil
}

// syncDir fsyncs a directory so a rename into it is durable across a crash.
func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("workspace: opening %s for sync: %w", dir, err)
	}
	defer func() { _ = d.Close() }()
	if err := d.Sync(); err != nil {
		return fmt.Errorf("workspace: syncing %s: %w", dir, err)
	}
	return nil
}
