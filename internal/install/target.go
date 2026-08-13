package install

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Target is the declarative unit a harness adapter emits and the installer
// applies. It is pure data: constructing one touches nothing on disk.
type Target struct {
	Path       string // absolute destination
	Kind       TargetKind
	Content    []byte // KindFile: full bytes; KindManagedBlock: desired interior
	LinkTarget string // KindSymlink: desired destination (canonicalised by the planner)
	BlockName  string // KindManagedBlock: marker name ("dispatch")
	Annotation string // KindManagedBlock: start-marker annotation
	Role       string
	// Mode overrides the permissions an applied KindFile target is published
	// with. Zero means "the installer's policy": the mode an updated file
	// already had, else 0o644. It exists for the one target whose usefulness
	// IS its mode — the development install's own binary, which has to be
	// executable however the destination was found. Asset payloads never set
	// it: the bundle's only policy mode is 0o644.
	Mode os.FileMode
}

// Disposition is what applying a target would do to what is on disk now.
type Disposition string

const (
	DispositionCreate   Disposition = "create"   // target absent
	DispositionNoop     Disposition = "no-op"    // present, already desired
	DispositionUpdate   Disposition = "update"   // present, owned, differs
	DispositionConflict Disposition = "conflict" // present, not provably ours
)

// The conflict reasons are the spec's stable machine reasons; the operation
// layer reports them verbatim.
const (
	ReasonOwnershipConflict   = "ownership-conflict"
	ReasonManagedBlockInvalid = "managed-block-invalid"
)

// ErrInvalidTarget is the sentinel every structurally impossible target wraps.
// It is an authoring defect in the plan, never a state of the user's disk.
var ErrInvalidTarget = errors.New("install: invalid target")

// validate rejects a target no inspection could reason about. Every check is
// structural: a plan that cannot name an absolute destination, or that omits
// the field its own kind is defined by, must never reach the filesystem.
func (t Target) validate() error {
	switch {
	case t.Path == "":
		return fmt.Errorf("%w: empty path", ErrInvalidTarget)
	case !filepath.IsAbs(t.Path):
		return fmt.Errorf("%w: path %q is not absolute", ErrInvalidTarget, t.Path)
	}
	switch t.Kind {
	case KindFile:
		return nil
	case KindSymlink:
		if t.LinkTarget == "" {
			return fmt.Errorf("%w: %s is a symlink with no link target", ErrInvalidTarget, t.Path)
		}
		if !filepath.IsAbs(t.LinkTarget) {
			return fmt.Errorf("%w: %s links to %q, which is not absolute", ErrInvalidTarget, t.Path, t.LinkTarget)
		}
		return nil
	case KindManagedBlock:
		if t.BlockName == "" {
			return fmt.Errorf("%w: %s is a managed block with no block name", ErrInvalidTarget, t.Path)
		}
		return nil
	default:
		return fmt.Errorf("%w: %s has unknown kind %q", ErrInvalidTarget, t.Path, t.Kind)
	}
}

// RecordFor derives the ownership record the installer publishes once t has
// been applied. It is the other half of every proof InspectTarget checks:
// both sides compute identity here, so a published record and a later
// inspection can never disagree about what "unchanged" means.
func RecordFor(t Target) (TargetRecord, error) {
	if err := t.validate(); err != nil {
		return TargetRecord{}, err
	}
	rec := TargetRecord{Path: filepath.Clean(t.Path), Kind: t.Kind, Role: t.Role}
	switch t.Kind {
	case KindFile:
		rec.SHA256 = hashBytes(t.Content)
	case KindSymlink:
		// The record stores the canonical destination, so a later inspection
		// comparing canonical forms is comparing like with like however the
		// plan spelled the link.
		canonical, err := canonicalPath(t.LinkTarget)
		if err != nil {
			return TargetRecord{}, err
		}
		rec.LinkTarget = canonical
	case KindManagedBlock:
		rec.BlockName = t.BlockName
		rec.SHA256 = interiorDigest(t.Content)
	}
	return rec, nil
}

// hashBytes is the file-identity digest: lowercase hex sha256 over exact bytes,
// matching the per-entry digest spelling in internal/assets.
func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// interiorDigest is the managed-block identity digest. It covers the block's
// LOGICAL interior rather than its raw bytes, because rewriting a block keeps
// that block's own line ending and absorbs one trailing newline: two spellings
// a rewrite cannot tell apart must not read as drift, or an unchanged
// installation would report a conflict against its own last write.
func interiorDigest(b []byte) string { return hashBytes([]byte(normalizeInterior(b))) }

// normalizeInterior reduces block content to the form a rewrite preserves.
func normalizeInterior(b []byte) string {
	s := strings.ReplaceAll(string(b), "\r\n", "\n")
	return strings.TrimSuffix(s, "\n")
}
