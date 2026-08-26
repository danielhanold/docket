package reposeed

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/danielhanold/docket/internal/install"
)

// RecordFormatVersion is the version of the per-worktree ownership document
// shape stored at RecordPath.
const RecordFormatVersion = 1

// ErrRecordInvalid is the sentinel every unreadable-but-present record wraps —
// the reposeed analogue of install.ErrStateInvalid.
var ErrRecordInvalid = errors.New("reposeed: ownership record invalid")

// SurfaceRecord is the ownership proof for one repository dispatch surface,
// stored worktree-RELATIVE so a moved worktree keeps its history. Its identity
// fields (SHA256, LinkTarget) are computed by the same machine-installer logic
// (install.RecordFor) that InspectTarget checks against, so a published record
// and a later inspection can never disagree about what "unchanged" means.
type SurfaceRecord struct {
	Path       string             `json:"path"`                  // worktree-relative, slash-separated
	Kind       install.TargetKind `json:"kind"`                  //
	BlockName  string             `json:"block_name,omitempty"`  // managed-block only
	LinkTarget string             `json:"link_target,omitempty"` // worktree-relative, slash-separated; symlink kind
	SHA256     string             `json:"sha256,omitempty"`      // file: whole file; block: normalized interior
	Harnesses  []string           `json:"harnesses"`             // sorted owners
}

// Record is the published per-worktree ownership document at
// <git-dir>/docket/install.json.
type Record struct {
	FormatVersion int             `json:"format_version"`
	Surfaces      []SurfaceRecord `json:"surfaces"` // sorted by Path
}

// RecordPath is the per-working-tree ownership document location under a git
// dir: <git-dir>/docket/install.json.
func RecordPath(gitDir string) string {
	return filepath.Join(gitDir, "docket", "install.json")
}

// LoadRecord reads the per-worktree ownership record. An absent file is "not
// installed" (nil, nil), not an error; a present but unreadable one is always an
// error — silently treating corruption as "not installed" would let a later
// publish overwrite surfaces it cannot prove it owns, exactly as
// install.LoadState refuses to adopt a corrupt machine state.
func LoadRecord(path string) (*Record, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reposeed: reading %s: %w", path, err)
	}
	var r Record
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("%w: parsing %s: %s", ErrRecordInvalid, path, err)
	}
	if r.FormatVersion != RecordFormatVersion {
		return nil, fmt.Errorf("%w: %s has format_version %d (want %d)", ErrRecordInvalid, path, r.FormatVersion, RecordFormatVersion)
	}
	return &r, nil
}

// ToState projects the record into a synthetic install.State whose TargetRecords
// carry ABSOLUTE cleaned paths joined under worktreeRoot, so install.InspectTarget
// can consume it as the prior state and prove ownership against the current disk.
// Joining under the current root (rather than a stored absolute) is what lets a
// relocated worktree keep its history. Only the fields recordMatchesDisk reads
// are populated; Harness/Role attribution is not needed to answer "is this still
// ours".
func (r *Record) ToState(worktreeRoot string) *install.State {
	root := filepath.Clean(worktreeRoot)
	targets := make([]install.TargetRecord, 0, len(r.Surfaces))
	for _, s := range r.Surfaces {
		tr := install.TargetRecord{
			Path:      filepath.Clean(filepath.Join(root, filepath.FromSlash(s.Path))),
			Kind:      s.Kind,
			BlockName: s.BlockName,
			SHA256:    s.SHA256,
		}
		if s.Kind == install.KindSymlink {
			tr.LinkTarget = filepath.Clean(filepath.Join(root, filepath.FromSlash(s.LinkTarget)))
		}
		targets = append(targets, tr)
	}
	return &install.State{FormatVersion: install.StateFormatVersion, Targets: targets}
}

// DesiredRecord renders the ownership record for a planned set of repository
// targets and their harness owners (owners keyed by cleaned absolute path, as
// reposeed.Plan emits). Every path is stored worktree-relative; a target whose
// path — or a symlink whose destination — escapes worktreeRoot is refused rather
// than stored as an escape. Surfaces are sorted by Path and each surface's owners
// are sorted. Identity (SHA256, block name) is computed by install.RecordFor, the
// same half of the proof InspectTarget checks.
func DesiredRecord(targets []install.Target, owners map[string][]string, worktreeRoot string) (*Record, error) {
	root := filepath.Clean(worktreeRoot)
	surfaces := make([]SurfaceRecord, 0, len(targets))
	for _, t := range targets {
		rec, err := install.RecordFor(t)
		if err != nil {
			return nil, err
		}
		rel, err := relWithin(root, filepath.Clean(t.Path))
		if err != nil {
			return nil, err
		}
		sr := SurfaceRecord{
			Path:      rel,
			Kind:      t.Kind,
			BlockName: t.BlockName,
			SHA256:    rec.SHA256,
		}
		if t.Kind == install.KindSymlink {
			linkRel, err := relWithin(root, filepath.Clean(t.LinkTarget))
			if err != nil {
				return nil, err
			}
			sr.LinkTarget = linkRel
		}
		if o := owners[filepath.Clean(t.Path)]; len(o) > 0 {
			owned := append([]string(nil), o...)
			sort.Strings(owned)
			sr.Harnesses = owned
		}
		surfaces = append(surfaces, sr)
	}
	sort.Slice(surfaces, func(i, j int) bool { return surfaces[i].Path < surfaces[j].Path })
	return &Record{FormatVersion: RecordFormatVersion, Surfaces: surfaces}, nil
}

// EncodeRecord renders the canonical document bytes for a journaled publish:
// surfaces sorted by Path, each surface's owners sorted, two-space indent, and a
// trailing newline. The caller's slices are never reordered.
func EncodeRecord(r *Record) ([]byte, error) {
	if r == nil {
		return nil, errors.New("reposeed: EncodeRecord requires a record")
	}
	out := Record{FormatVersion: RecordFormatVersion}
	out.Surfaces = append([]SurfaceRecord(nil), r.Surfaces...)
	sort.Slice(out.Surfaces, func(i, j int) bool { return out.Surfaces[i].Path < out.Surfaces[j].Path })
	for i := range out.Surfaces {
		if h := out.Surfaces[i].Harnesses; h != nil {
			owned := append([]string(nil), h...)
			sort.Strings(owned)
			out.Surfaces[i].Harnesses = owned
		}
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("reposeed: encoding ownership record: %w", err)
	}
	return append(data, '\n'), nil
}

// relWithin returns the slash-separated path of p relative to root, refusing any
// path that escapes root. Both arguments are already filepath.Clean'd. This is
// the single containment guard both a surface path and a symlink destination pass
// through.
func relWithin(root, p string) (string, error) {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return "", fmt.Errorf("reposeed: %q is not relative to worktree root %q: %w", p, root, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("reposeed: path %q escapes worktree root %q", p, root)
	}
	return filepath.ToSlash(rel), nil
}
