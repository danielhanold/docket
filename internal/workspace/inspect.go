package workspace

// This file owns Inspect: a strictly read-only classification of one feature
// workspace's state. Inspect never deletes, repairs, resets, fetches, or
// normalizes anything — it reads the manifest, live Git registration, the
// feature ref tip, the registered worktree HEAD, recorded base ancestry, and the
// exact dirty/staged/untracked path summary, and folds them into one of the
// StateKind classes the spec §Inspect enumerates. A malformed or foreign
// manifest is DATA in a StateForeign result carrying the parse detail — never an
// error that hides the state — but an unreadable filesystem or Git probe is
// still an error: an unknown probe is never silently treated as a clean answer.

import (
	"context"
	"path/filepath"
	"sort"

	"github.com/danielhanold/docket/internal/gitcli"
)

// inspectOp is the Op recorded on every Inspect Failure.
const inspectOp = "inspect"

// InspectRequest names the repository and the fully validated target to inspect.
// Inspect takes no remote: it never fetches.
type InspectRequest struct {
	Repository gitcli.Repository
	Target     Target
}

// StateKind is the closed set of workspace states Inspect distinguishes. The
// string values double as the coarse phase/label a caller logs.
type StateKind string

const (
	StateReady      StateKind = "ready"          // registered, on the feature ref, base reachable, clean
	StateCleaned    StateKind = "cleaned"        // tombstone: cleaned manifest, no registration
	StateResumable  StateKind = "allocating"     // allocating manifest, safely resumable partial
	StateDirty      StateKind = "dirty-owned"    // registered and consistent, but dirty/staged/untracked
	StateBranchGone StateKind = "branch-missing" // feature ref missing, or head no longer reaches base
	StateMismatch   StateKind = "mismatch"       // path/registration/manifest disagree
	StateForeign    StateKind = "foreign"        // absent, foreign, malformed, or unowned manifest
)

// Inspection is the read-only fact bundle Inspect returns. Detail carries the
// bounded, redacted reason a foreign/malformed manifest was classified
// StateForeign (empty otherwise); it is data, not an error. Fields not
// established by the observed state (a branch head when the branch is missing, a
// registered HEAD when nothing is registered) are left zero.
type Inspection struct {
	Kind        StateKind
	Phase       Phase
	Path        string
	Registered  bool
	Branch      gitcli.RefName
	BranchHead  gitcli.ObjectID
	HeadCommit  gitcli.ObjectID
	BaseCommit  gitcli.ObjectID
	BaseReached bool
	DirtyPaths  []string // exact tracked-dirty + staged + untracked paths, sorted
	Detail      string   // bounded reason for a StateForeign classification
}

// Inspect classifies one workspace read-only. It returns a typed Failure only
// for a genuine probe error — an unreadable manifest slot, or a Git probe that
// fails for a reason other than a cleanly-absent ref. Every legible state,
// including a foreign or malformed manifest, is returned as an Inspection.
func (s *Service) Inspect(ctx context.Context, req InspectRequest) (Inspection, error) {
	if err := validateInspectTarget(req.Target); err != nil {
		return Inspection{}, err
	}
	repo := req.Repository
	if repo.CommonDir == "" || !filepath.IsAbs(repo.CommonDir) || repo.PrimaryWorktree == "" || !filepath.IsAbs(repo.PrimaryWorktree) {
		return Inspection{}, &Failure{Op: inspectOp, Stage: "validate", Kind: KindInvalidInput, Detail: "repository paths are not absolute"}
	}
	target := req.Target
	dir := workspaceDir(repo.CommonDir, target.FeatureRef)
	intendedPath := filepath.Join(repo.PrimaryWorktree, ".worktrees", target.Slug)

	// Read the manifest slot. Absent, foreign, or unowned is StateForeign data; an
	// unreadable slot is the one manifest condition that is an error, not data.
	m, status, cerr := classifyManifest(dir)
	switch status {
	case manifestUnknown:
		return Inspection{}, &Failure{Op: inspectOp, Stage: "inventory", Kind: KindExternal, Detail: "workspace manifest is unreadable", Err: cerr}
	case manifestAbsent:
		return Inspection{Kind: StateForeign, Path: intendedPath, Detail: "no workspace manifest"}, nil
	case manifestForeign:
		return Inspection{Kind: StateForeign, Path: intendedPath, Detail: boundedDetail(cerr.Error())}, nil
	}
	if !ownsManifest(m, repo, target, intendedPath) {
		return Inspection{Kind: StateForeign, Phase: m.Phase, Path: m.Path, BaseCommit: m.BaseCommit, Detail: "manifest identity does not match this repository or target"}, nil
	}

	insp := Inspection{Phase: m.Phase, Path: m.Path, BaseCommit: m.BaseCommit}

	// Feature ref tip: absent (ref-unavailable) is a legible state; any other
	// probe error is a real failure and is surfaced, never swallowed.
	branchHead, branchErr := s.git.ResolveRef(ctx, repo, target.FeatureRef)
	branchExists := false
	if branchErr == nil {
		branchExists = true
		insp.BranchHead = branchHead
	} else if f, ok := gitcli.AsFailure(branchErr); !ok || f.Kind != gitcli.KindRefUnavailable {
		return Inspection{}, mapGitFailure(inspectOp, "inventory", branchErr)
	}

	// Live registration at the recorded canonical path.
	infos, err := s.git.ListWorktrees(ctx, repo)
	if err != nil {
		return Inspection{}, mapGitFailure(inspectOp, "inventory", err)
	}
	reg, registered := worktreeAt(infos, m.Path)
	insp.Registered = registered
	if registered {
		insp.Branch = reg.Branch
		insp.HeadCommit = reg.Head
	}

	if err := s.classifyState(ctx, repo, m, target, branchExists, branchHead, reg, registered, &insp); err != nil {
		return Inspection{}, err
	}
	return insp, nil
}

// classifyState assigns insp.Kind (and the ancestry/dirty facts it depends on)
// from the recorded phase and the live registration, feature ref, and HEAD. It
// is read-only: the only Git it may run beyond what the caller already gathered
// is an ancestry check and a status read, both non-mutating.
func (s *Service) classifyState(ctx context.Context, repo gitcli.Repository, m Manifest, target Target, branchExists bool, branchHead gitcli.ObjectID, reg gitcli.WorktreeInfo, registered bool, insp *Inspection) error {
	switch m.Phase {
	case PhaseCleaned:
		// A tombstone is consistent only with NO registration; a still-registered
		// tombstone disagrees with the recorded phase.
		if registered {
			insp.Kind = StateMismatch
		} else {
			insp.Kind = StateCleaned
		}
		return nil

	case PhaseAllocating:
		// A branch already created by this manifest that no longer contains the
		// recorded base was rewritten out of band: the partial is not safely
		// resumable — the states disagree.
		if branchExists {
			reachable, err := s.git.IsAncestor(ctx, repo, m.BaseCommit, branchHead)
			if err != nil {
				return mapGitFailure(inspectOp, "inventory", err)
			}
			insp.BaseReached = reachable
			if !reachable {
				insp.Kind = StateMismatch
				return nil
			}
		}
		if registered {
			if reg.Detached || reg.Branch != target.FeatureRef {
				insp.Kind = StateMismatch
				return nil
			}
			dirty, err := s.dirtyPaths(ctx, m.Path)
			if err != nil {
				return err
			}
			insp.DirtyPaths = dirty
		}
		insp.Kind = StateResumable
		return nil

	case PhaseReady:
		if !branchExists {
			insp.Kind = StateBranchGone
			return nil
		}
		if !registered {
			insp.Kind = StateMismatch
			return nil
		}
		if reg.Detached || reg.Branch != target.FeatureRef || reg.Head != branchHead {
			insp.Kind = StateMismatch
			return nil
		}
		reachable, err := s.git.IsAncestor(ctx, repo, m.BaseCommit, reg.Head)
		if err != nil {
			return mapGitFailure(inspectOp, "inventory", err)
		}
		insp.BaseReached = reachable
		if !reachable {
			// A ready worktree whose head no longer reaches the recorded base has a
			// moved branch.
			insp.Kind = StateBranchGone
			return nil
		}
		dirty, err := s.dirtyPaths(ctx, m.Path)
		if err != nil {
			return err
		}
		insp.DirtyPaths = dirty
		if len(dirty) > 0 {
			insp.Kind = StateDirty
		} else {
			insp.Kind = StateReady
		}
		return nil

	default:
		insp.Kind = StateForeign
		insp.Detail = "unknown manifest phase"
		return nil
	}
}

// dirtyPaths reads the exact tracked-dirty + staged + untracked path summary of
// the workspace at path via ChangedPaths and returns the unique paths sorted. A
// probe error is a real failure — an unreadable workspace is never reported as
// clean.
func (s *Service) dirtyPaths(ctx context.Context, path string) ([]string, error) {
	changes, err := s.git.ChangedPaths(ctx, path)
	if err != nil {
		return nil, mapGitFailure(inspectOp, "inventory", err)
	}
	seen := make(map[string]bool, len(changes))
	out := make([]string, 0, len(changes))
	for _, c := range changes {
		p := string(c.Path)
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out, nil
}

// validateInspectTarget rejects a target that is not self-consistent with its
// slug and base, reusing NewTarget's rules and re-tagging any rejection to the
// inspect operation.
func validateInspectTarget(t Target) error {
	derived, err := NewTarget(t.ChangeID, t.Slug, t.Base)
	if err != nil {
		return &Failure{Op: inspectOp, Stage: "validate", Kind: KindInvalidInput, Detail: "target is not self-consistent with its slug and base"}
	}
	if derived != t {
		return &Failure{Op: inspectOp, Stage: "validate", Kind: KindInvalidInput, Detail: "target fields are not self-consistent with its slug and base"}
	}
	return nil
}

// boundedDetail caps a diagnostic string so a Failure/Inspection detail stays
// bounded (never unbounded stderr or decode dumps).
func boundedDetail(s string) string {
	const max = 200
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
