package install

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/danielhanold/docket/internal/document"
)

// Inspection is one target classified against the filesystem and the prior
// installation record.
type Inspection struct {
	Target      Target
	Disposition Disposition
	Reason      string // conflict detail: ReasonOwnershipConflict | ReasonManagedBlockInvalid
	// Remedy is the target-specific way forward for a conflict: what docket
	// found here, and the one action that clears it. There is deliberately no
	// --force, so the only way past a conflict is for the user to change what
	// is on disk and run again — a report naming the reason alone would leave
	// them with nowhere to go. Every conflict carries one; no other
	// disposition does.
	Remedy string
}

// ConflictDetail is the human-readable half of a conflict report: the stable
// reason, then the remedy that says what to do about it. It is what the
// service puts in an Action's Detail, so the machine-readable reason survives
// verbatim as the first field and the prose follows it.
func (i Inspection) ConflictDetail() string {
	if i.Remedy == "" {
		return i.Reason
	}
	return i.Reason + ": " + i.Remedy
}

// LegacyReproducer reproduces a legacy user-level artifact's complete bytes
// from the frozen legacy renderer, reporting false when this target has no
// legacy spelling. A nil reproducer disables the third proof entirely, which
// is what change 0311 ships: the initial matrix takes over nothing by legacy
// reproduction, so ownership rests on the prior manifest and managed markers.
type LegacyReproducer func(t Target) ([]byte, bool)

// InspectTarget classifies one target against disk plus the prior state. prior
// may be nil (fresh install), and legacy may be nil (no legacy takeover).
//
// The classification is read-only and answers exactly one question: may the
// installer write here? Three proofs of ownership are accepted, in the spec's
// order — exact identity with the prior record, a valid Docket-managed block
// carrying the recorded interior, and byte-exact legacy reproduction. Anything
// else on disk is somebody else's, so it is preserved and reported as a
// conflict. A filename that merely looks like Docket's is never a proof.
//
// An error means the target itself is unusable or the filesystem failed; it
// never means "conflict" — a conflict is a successful classification.
func InspectTarget(t Target, prior *State, legacy LegacyReproducer) (Inspection, error) {
	if err := t.validate(); err != nil {
		return Inspection{}, err
	}

	info, err := os.Lstat(t.Path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return Inspection{Target: t, Disposition: DispositionCreate}, nil
	case err != nil:
		return Inspection{}, fmt.Errorf("install: inspecting %s: %w", t.Path, err)
	}

	rec, hasRec := priorRecord(prior, t.Path)

	switch t.Kind {
	case KindFile:
		return inspectFile(t, info, rec, hasRec, legacy)
	case KindSymlink:
		return inspectSymlink(t, info, rec, hasRec, legacy)
	case KindManagedBlock:
		return inspectManagedBlock(t, info, rec, hasRec)
	default:
		// validate has already refused every other kind.
		return Inspection{}, fmt.Errorf("%w: %s has unknown kind %q", ErrInvalidTarget, t.Path, t.Kind)
	}
}

func inspectFile(t Target, info fs.FileInfo, rec TargetRecord, hasRec bool, legacy LegacyReproducer) (Inspection, error) {
	if info.Mode().IsRegular() {
		data, err := os.ReadFile(t.Path)
		if err != nil {
			return Inspection{}, fmt.Errorf("install: reading %s: %w", t.Path, err)
		}
		if bytes.Equal(data, t.Content) {
			return Inspection{Target: t, Disposition: DispositionNoop}, nil
		}
		if owned, err := provenByRecord(rec, hasRec); err != nil {
			return Inspection{}, err
		} else if owned {
			return Inspection{Target: t, Disposition: DispositionUpdate}, nil
		}
		if provenByLegacy(t, data, legacy) {
			return Inspection{Target: t, Disposition: DispositionUpdate}, nil
		}
		return conflict(t, ReasonOwnershipConflict, remedyForPath(hasRec)), nil
	}

	// A directory, device, or symlink where a plain file belongs is only ours
	// when the prior record says so — an upgrade that changes a target's kind
	// still has to prove it owns what is there now.
	if owned, err := provenByRecord(rec, hasRec); err != nil {
		return Inspection{}, err
	} else if owned {
		return Inspection{Target: t, Disposition: DispositionUpdate}, nil
	}
	return conflict(t, ReasonOwnershipConflict, remedyForPath(hasRec)), nil
}

func inspectSymlink(t Target, info fs.FileInfo, rec TargetRecord, hasRec bool, legacy LegacyReproducer) (Inspection, error) {
	var repointed string // the destination the plan wants, when a link is in the way
	if info.Mode()&fs.ModeSymlink != 0 {
		have, err := linkDestination(t.Path)
		if err != nil {
			return Inspection{}, err
		}
		want, err := canonicalPath(t.LinkTarget)
		if err != nil {
			return Inspection{}, err
		}
		if have == want {
			return Inspection{Target: t, Disposition: DispositionNoop}, nil
		}
		repointed = want
	}

	if owned, err := provenByRecord(rec, hasRec); err != nil {
		return Inspection{}, err
	} else if owned {
		return Inspection{Target: t, Disposition: DispositionUpdate}, nil
	}

	if info.Mode().IsRegular() {
		data, err := os.ReadFile(t.Path)
		if err != nil {
			return Inspection{}, fmt.Errorf("install: reading %s: %w", t.Path, err)
		}
		if provenByLegacy(t, data, legacy) {
			return Inspection{Target: t, Disposition: DispositionUpdate}, nil
		}
	}
	if repointed != "" {
		return conflict(t, ReasonOwnershipConflict, remedyLinkDestination(repointed)), nil
	}
	return conflict(t, ReasonOwnershipConflict, remedyForPath(hasRec)), nil
}

// inspectManagedBlock classifies a block inside a file the user also owns. The
// file's other bytes are never ours, so every outcome here either leaves the
// file untouched or rewrites exactly one block's interior.
func inspectManagedBlock(t Target, info fs.FileInfo, rec TargetRecord, hasRec bool) (Inspection, error) {
	if !info.Mode().IsRegular() {
		if owned, err := provenByRecord(rec, hasRec); err != nil {
			return Inspection{}, err
		} else if owned {
			return Inspection{Target: t, Disposition: DispositionUpdate}, nil
		}
		return conflict(t, ReasonOwnershipConflict, remedyForPath(hasRec)), nil
	}

	data, err := os.ReadFile(t.Path)
	if err != nil {
		return Inspection{}, fmt.Errorf("install: reading %s: %w", t.Path, err)
	}
	doc, err := document.Parse(data)
	if err != nil {
		// Malformed or unbalanced markers above all: an unbounded or
		// out-of-order marker range would swallow the user's own bytes, so the
		// file is reported and left exactly as it is. Any other parse failure
		// is equally unwritable for the same reason.
		return conflict(t, ReasonManagedBlockInvalid, remedyBlockMarkers(t.BlockName)), nil
	}

	block, ok := doc.Block(t.BlockName)
	if !ok {
		// Adding a block only appends; every existing byte survives, so no
		// ownership proof is required to write one into a file we did not
		// author.
		return Inspection{Target: t, Disposition: DispositionUpdate}, nil
	}
	interior := doc.Source()[block.Interior.Start:block.Interior.End]
	if normalizeInterior(interior) == normalizeInterior(t.Content) {
		return Inspection{Target: t, Disposition: DispositionNoop}, nil
	}
	if hasRec && rec.Kind == KindManagedBlock && rec.BlockName == t.BlockName &&
		rec.SHA256 != "" && interiorDigest(interior) == rec.SHA256 {
		return Inspection{Target: t, Disposition: DispositionUpdate}, nil
	}
	if hasRec {
		return conflict(t, ReasonOwnershipConflict, remedyDriftedBlock(t.BlockName)), nil
	}
	return conflict(t, ReasonOwnershipConflict, remedyForeignBlock(t.BlockName)), nil
}

func conflict(t Target, reason, remedy string) Inspection {
	return Inspection{Target: t, Disposition: DispositionConflict, Reason: reason, Remedy: remedy}
}

// The remedies. Each names what docket found, then the one action that clears
// it — the installer has no --force, so the only way past a conflict is for the
// user to change what is on disk and run again.
const (
	remedyForeignPath = "docket did not write what is at this path; move or delete it, then re-run"
	remedyDriftedPath = "this path no longer matches the recorded install, so docket cannot prove it may overwrite it; " +
		"restore the recorded content, or move it aside, then re-run"
)

// remedyLinkDestination names the destination the plan wants, which is the one
// fact a repointed link hides: the link on disk shows where it goes, never
// where docket meant it to go.
func remedyLinkDestination(want string) string {
	return "docket expected a symlink to " + want +
		"; move or delete what is there, then re-run"
}

// remedyBlockMarkers is the only remedy docket cannot offer to perform itself:
// the markers delimit somebody's own file, and guessing where a dangling range
// was meant to end would eat their content.
func remedyBlockMarkers(block string) string {
	return "the docket:" + block + " markers in this file are malformed, unbalanced, or out of order; " +
		"repair them by hand, then re-run"
}

func remedyForeignBlock(block string) string {
	return "the docket:" + block + " block in this file holds content docket did not write; " +
		"delete the block, or move the file aside, then re-run"
}

func remedyDriftedBlock(block string) string {
	return "the docket:" + block + " block no longer matches the recorded install, " +
		"so docket cannot prove it may rewrite it; restore its recorded content, or delete the block, then re-run"
}

// remedyForPath picks between the two path-shaped remedies. A prior record for
// this path means docket did write here once and what is there now has since
// drifted, which is a different situation for the user than a file that was
// never docket's — and a different one to describe.
func remedyForPath(hasRec bool) string {
	if hasRec {
		return remedyDriftedPath
	}
	return remedyForeignPath
}

// provenByRecord is ownership proof one: what is on disk right now is exactly
// what the prior installation recorded writing.
func provenByRecord(rec TargetRecord, hasRec bool) (bool, error) {
	if !hasRec {
		return false, nil
	}
	return recordMatchesDisk(rec)
}

// provenByLegacy is ownership proof three: the bytes on disk are exactly what
// the frozen legacy renderer would have produced for this target.
func provenByLegacy(t Target, data []byte, legacy LegacyReproducer) bool {
	if legacy == nil {
		return false
	}
	want, ok := legacy(t)
	return ok && bytes.Equal(want, data)
}

// recordMatchesDisk reports whether the path named by rec still holds exactly
// what rec says docket put there. An absent path is not a match; an unreadable
// or unparseable one is not a match either, since neither proves ownership.
// Only an unexpected filesystem failure is an error.
func recordMatchesDisk(rec TargetRecord) (bool, error) {
	info, err := os.Lstat(rec.Path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("install: inspecting %s: %w", rec.Path, err)
	}

	switch rec.Kind {
	case KindFile:
		if !info.Mode().IsRegular() || rec.SHA256 == "" {
			return false, nil
		}
		data, err := os.ReadFile(rec.Path)
		if err != nil {
			return false, fmt.Errorf("install: reading %s: %w", rec.Path, err)
		}
		return hashBytes(data) == rec.SHA256, nil

	case KindSymlink:
		if info.Mode()&fs.ModeSymlink == 0 || rec.LinkTarget == "" {
			return false, nil
		}
		have, err := linkDestination(rec.Path)
		if err != nil {
			return false, err
		}
		want, err := canonicalPath(rec.LinkTarget)
		if err != nil {
			return false, err
		}
		return have == want, nil

	case KindManagedBlock:
		if !info.Mode().IsRegular() || rec.SHA256 == "" || rec.BlockName == "" {
			return false, nil
		}
		data, err := os.ReadFile(rec.Path)
		if err != nil {
			return false, fmt.Errorf("install: reading %s: %w", rec.Path, err)
		}
		doc, err := document.Parse(data)
		if err != nil {
			return false, nil
		}
		block, ok := doc.Block(rec.BlockName)
		if !ok {
			return false, nil
		}
		return interiorDigest(doc.Source()[block.Interior.Start:block.Interior.End]) == rec.SHA256, nil

	default:
		return false, nil
	}
}

// priorRecord finds the prior installation's record for path.
func priorRecord(prior *State, path string) (TargetRecord, bool) {
	if prior == nil {
		return TargetRecord{}, false
	}
	clean := filepath.Clean(path)
	for _, rec := range prior.Targets {
		if filepath.Clean(rec.Path) == clean {
			return rec, true
		}
	}
	return TargetRecord{}, false
}

// Prune is one previously owned target the new plan no longer contains.
// Removable says its identity still equals the prior record, which is the only
// licence to delete it; a drifted one is preserved and blocks the upgrade.
type Prune struct {
	Record    TargetRecord
	Removable bool
}

// PruneCandidates returns the prior-state targets absent from plan, each
// classified. A target that has already vanished is removable: there is
// nothing left to preserve, so it must not block an upgrade.
func PruneCandidates(prior *State, plan []Target) ([]Prune, error) {
	if prior == nil {
		return nil, nil
	}
	planned := make(map[string]bool, len(plan))
	for _, t := range plan {
		if t.Path != "" {
			planned[filepath.Clean(t.Path)] = true
		}
	}

	var out []Prune
	for _, rec := range prior.Targets {
		if rec.Path == "" || !filepath.IsAbs(rec.Path) {
			return nil, fmt.Errorf("%w: installed state records target path %q, which is not absolute",
				ErrStateInvalid, rec.Path)
		}
		if planned[filepath.Clean(rec.Path)] {
			continue
		}
		if _, err := os.Lstat(rec.Path); errors.Is(err, fs.ErrNotExist) {
			out = append(out, Prune{Record: rec, Removable: true})
			continue
		}
		matches, err := recordMatchesDisk(rec)
		if err != nil {
			return nil, err
		}
		out = append(out, Prune{Record: rec, Removable: matches})
	}
	return out, nil
}

// canonicalPath resolves every symlink hop of p. A path whose final components
// do not exist yet is still canonicalised as far as it does exist — an install
// plans links into a version tree it has not extracted yet, and macOS resolves
// /tmp and /var through symlinks, so comparing unresolved spellings would call
// two names for one file a conflict.
func canonicalPath(p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("%w: empty path", ErrInvalidTarget)
	}
	clean := filepath.Clean(p)
	resolved, err := filepath.EvalSymlinks(clean)
	if err == nil {
		return resolved, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("install: canonicalising %s: %w", p, err)
	}
	parent := filepath.Dir(clean)
	if parent == clean {
		return clean, nil // filesystem root
	}
	base, err := canonicalPath(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, filepath.Base(clean)), nil
}

// linkDestination is the canonical destination of the symlink at path: the
// link is read, a relative destination is anchored at the canonical directory
// holding the link, and the result is canonicalised hop by hop.
func linkDestination(path string) (string, error) {
	dest, err := os.Readlink(path)
	if err != nil {
		return "", fmt.Errorf("install: reading link %s: %w", path, err)
	}
	if !filepath.IsAbs(dest) {
		dir, err := canonicalPath(filepath.Dir(path))
		if err != nil {
			return "", err
		}
		dest = filepath.Join(dir, dest)
	}
	return canonicalPath(dest)
}
