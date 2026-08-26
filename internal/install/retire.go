package install

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/danielhanold/docket/internal/document"
)

// Retirement is the other side of the change that stopped the harness adapters
// planning user-global dispatch surfaces (change 0351). Removing a target from
// the desired plan is not enough for these destinations: a managed block lives
// inside a file the user also owns, and a whole-file rule may have been edited,
// so an install that simply stopped writing them would leave stale parent-facing
// instructions behind forever. It must instead RETIRE the leftovers it can prove
// are Docket's — and refuse, never guess, on anything it cannot.
//
// The proof is bytes, not markers (spec "Alternatives considered"): a managed
// block is removed only when its normalized interior still digests to the prior
// installation record or to the frozen legacy reproducer; a Cursor rule only
// when the whole file matches one of those two. A marker that merely spells
// "docket" proves nothing. And the probe is three-valued, never two: a
// destination that is cleanly absent is nothing to do, one that is present and
// proven is a removal, and one that cannot be inspected at all — a read error —
// refuses the whole run rather than being mistaken for "absent" (Global
// Constraints: three probe outcomes; learnings: probe-error-is-not-clean-absence).

// PlanGlobalRetirements probes each historical global dispatch destination and
// decides what to do with a leftover: remove a proven one, refuse an unprovable
// one, and leave a cleanly absent or already-retired one alone. The historical
// targets carry only their location and identity (Content nil) — the adapters
// build them from GlobalDispatchTarget — because retirement never renders a
// desired body, it only proves ownership of what is already on disk.
//
// It returns three independent results so one refusal can collect every
// conflict: the proven removals to fold into the transaction, the conflicts to
// fold into the plan's own conflict list, and an error that stops the run
// outright. A stat or read failure is that error (never a silent skip); a
// present-but-unprovable destination is a conflict; and everything provable or
// absent flows through cleanly. roots is the user-level anchor every historical
// path was built under; it is carried for symmetry with the rest of the
// installer's signatures and to document the containment the caller guarantees.
func PlanGlobalRetirements(roots UserRoots, historical []Target, prior *State, legacy LegacyReproducer) (removals []TargetRecord, conflicts []Inspection, err error) {
	_ = roots // historical paths are already absolute, built from GlobalDispatchTarget(roots).
	for _, t := range historical {
		remove, conflict, perr := inspectGlobalRetirement(t, prior, legacy)
		if perr != nil {
			// The unknown probe outcome: refuse the whole run rather than treat an
			// unreadable destination as absent.
			return nil, nil, perr
		}
		if conflict != nil {
			conflicts = append(conflicts, *conflict)
			continue
		}
		if remove {
			removals = append(removals, retirementRecord(t, prior))
		}
	}
	return removals, conflicts, nil
}

// inspectGlobalRetirement classifies one historical destination. It never
// mutates: like InspectTarget it answers a question about disk, and the answer
// is one of remove / conflict / nothing, or a filesystem error.
func inspectGlobalRetirement(t Target, prior *State, legacy LegacyReproducer) (remove bool, conflict *Inspection, err error) {
	info, lerr := os.Lstat(t.Path)
	switch {
	case errors.Is(lerr, fs.ErrNotExist):
		return false, nil, nil // cleanly absent: nothing to retire.
	case lerr != nil:
		return false, nil, fmt.Errorf("install: inspecting %s: %w", t.Path, lerr)
	}

	rec, hasRec := priorRecord(prior, t.Path)
	switch t.Kind {
	case KindManagedBlock:
		return retireManagedBlock(t, info, rec, hasRec, legacy)
	case KindFile:
		return retireFile(t, info, rec, hasRec, legacy)
	default:
		return false, nil, fmt.Errorf(
			"%w: %s is a global dispatch target of kind %q, which retirement cannot reason about",
			ErrInvalidTarget, t.Path, t.Kind)
	}
}

// retireManagedBlock decides the fate of a managed dispatch block inside a file
// the user also owns. Every path here either removes exactly that block or
// leaves the whole file untouched — the surrounding bytes are never Docket's to
// delete.
func retireManagedBlock(t Target, info fs.FileInfo, rec TargetRecord, hasRec bool, legacy LegacyReproducer) (bool, *Inspection, error) {
	if !info.Mode().IsRegular() {
		// A directory, or a symlink where a managed-block file belongs, is a
		// foreign kind. A block can never be installed through a symlink (the
		// transaction refuses to rewrite one), so a link here is not a retirable
		// block either — both preserve the destination and refuse.
		c := conflict(t, ReasonOwnershipConflict, remedyForPath(hasRec))
		return false, &c, nil
	}
	data, err := os.ReadFile(t.Path)
	if err != nil {
		return false, nil, fmt.Errorf("install: reading %s: %w", t.Path, err)
	}
	doc, err := document.Parse(data)
	if err != nil {
		// Malformed, unbalanced, or out-of-order markers: an unbounded range would
		// swallow the user's own bytes on the way out, so the file is preserved and
		// the run refuses with the by-hand remedy.
		c := conflict(t, ReasonManagedBlockInvalid, remedyBlockMarkers(t.BlockName))
		return false, &c, nil
	}
	block, ok := doc.Block(t.BlockName)
	if !ok {
		// The block is already gone — retired by a prior run, or never written.
		// The surrounding file is the user's and is left exactly as it is.
		return false, nil, nil
	}
	interior := doc.Source()[block.Interior.Start:block.Interior.End]
	// Ownership proof one: the interior still digests to what a prior install
	// recorded writing here.
	if hasRec && rec.Kind == KindManagedBlock && rec.BlockName == t.BlockName &&
		rec.SHA256 != "" && interiorDigest(interior) == rec.SHA256 {
		return true, nil, nil
	}
	// Ownership proof three: the interior is byte-exact (modulo the
	// rewrite-preserving normalisation) to what the frozen legacy installer wrote.
	if provenByLegacyInterior(t, interior, legacy) {
		return true, nil, nil
	}
	// An edited interior is the user's now: retirement cannot prove ownership, so
	// it preserves the block and refuses the run.
	remedy := remedyForeignBlock(t.BlockName)
	if hasRec {
		remedy = remedyDriftedBlock(t.BlockName)
	}
	c := conflict(t, ReasonOwnershipConflict, remedy)
	return false, &c, nil
}

// retireFile decides the fate of a whole-file dispatch rule (Cursor's
// docket-dispatch.mdc). Docket owns the entire file, so a proven one is deleted
// outright and an unprovable one is preserved.
func retireFile(t Target, info fs.FileInfo, rec TargetRecord, hasRec bool, legacy LegacyReproducer) (bool, *Inspection, error) {
	if !info.Mode().IsRegular() {
		// A directory or a symlink where the whole-file rule belongs is a foreign
		// kind; retirement will not delete through it.
		c := conflict(t, ReasonOwnershipConflict, remedyForPath(hasRec))
		return false, &c, nil
	}
	data, err := os.ReadFile(t.Path)
	if err != nil {
		return false, nil, fmt.Errorf("install: reading %s: %w", t.Path, err)
	}
	// Ownership proof one: the whole file still matches the recorded install.
	if owned, err := provenByRecord(rec, hasRec); err != nil {
		return false, nil, err
	} else if owned {
		return true, nil, nil
	}
	// Ownership proof three: the whole file is byte-exact to the frozen legacy rule.
	if provenByLegacy(t, data, legacy) {
		return true, nil, nil
	}
	c := conflict(t, ReasonOwnershipConflict, remedyForPath(hasRec))
	return false, &c, nil
}

// retirementRecord is the ownership record whose removal the transaction
// journals. It carries only what removalTarget consumes — path, kind, and the
// block name for a managed block — plus the harness attribution the prior record
// held, so the reported removal names the harness it retired.
func retirementRecord(t Target, prior *State) TargetRecord {
	rec := TargetRecord{
		Path:      filepath.Clean(t.Path),
		Kind:      t.Kind,
		BlockName: t.BlockName,
		Role:      t.Role,
	}
	if prev, ok := priorRecord(prior, t.Path); ok {
		rec.Harness = prev.Harness
	}
	return rec
}
