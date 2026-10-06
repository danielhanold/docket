// Package reposeed plans the parent-facing repository surfaces docket reconciles
// for a repository that explicitly opts in via agent_harnesses. It is pure: it
// renders declarative install.Target values and their harness ownership and
// touches nothing on disk. It emits parent-facing routing instructions ONLY —
// never per-repository agent definitions and never skills (change 0351). The
// machine installer (internal/install) applies the plan and the caller
// (internal/app) supplies the CLAUDE.md pre-state, since Plan never stats a
// path.
package reposeed

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/danielhanold/docket/internal/harness"
	"github.com/danielhanold/docket/internal/harness/cursor"
	"github.com/danielhanold/docket/internal/install"
	"github.com/danielhanold/docket/internal/layout"
)

// The four harness tokens this planner answers to. They mirror the validated
// agent_harnesses vocabulary; an unknown token is still rejected here as
// defense in depth, so a caller that skipped validation cannot smuggle one
// through into a planned path.
const (
	harnessClaude   = "claude"
	harnessCodex    = "codex"
	harnessOpencode = "opencode"
	harnessCursor   = "cursor"
)

// The repository-relative surface locations, joined under WorktreeRoot.
const (
	claudeMDName  = "CLAUDE.md"
	agentsMDName  = "AGENTS.md"
	cursorRuleRel = ".cursor/rules/docket-dispatch.mdc"
)

// The managed-block identity every dispatch surface shares. The annotation
// matches the harness adapters' user-global blocks so the same managed-block
// machinery reads and rewrites both.
const (
	dispatchBlockName  = "dispatch"
	dispatchAnnotation = "managed by docket — do not hand-edit"
	roleDispatch       = "dispatch"
)

// ClaudeMDState is the CLAUDE.md pre-state the caller computes (Plan never stats
// a path). It decides whether Claude can safely share the codex/opencode
// AGENTS.md surface via a relative link or must own its own managed block.
type ClaudeMDState int

const (
	// ClaudeMDAbsent — no CLAUDE.md exists; a share is a fresh link.
	ClaudeMDAbsent ClaudeMDState = iota
	// ClaudeMDRegularFile — a regular CLAUDE.md exists; keep its content and
	// give it its own managed block, never replace it with a link.
	ClaudeMDRegularFile
	// ClaudeMDLinkToAgents — CLAUDE.md is already a proven relative link to
	// AGENTS.md; a share re-plans the same link.
	ClaudeMDLinkToAgents
	// ClaudeMDOther — anything else (a foreign kind, an unreadable path).
	// Planned as a managed block so inspection reports the conflict with a
	// remedy rather than silently overwriting.
	ClaudeMDOther
	// ClaudeMDForeignLink — CLAUDE.md is a symlink that is not a proven
	// relative link to AGENTS.md. A link cannot carry a managed block, so the
	// plan skips it.
	ClaudeMDForeignLink
)

// PlanInput is the pure input to Plan. WorktreeRoot is a canonical absolute
// path; Harnesses are the repository's explicit, already-validated opt-in
// tokens; RunTracker is the run-tracker payload the interiors carry verbatim.
type PlanInput struct {
	WorktreeRoot  string
	Harnesses     []string
	RunTracker    []byte
	ClaudeMDState ClaudeMDState
}

// Plan renders the parent-facing repository targets for the opted-in harnesses.
// It is pure and emits NO agent definitions and NO skills. The second return
// maps each cleaned target path to its harness owners — a shared surface carries
// several. Every planned path must resolve inside WorktreeRoot after
// filepath.Clean; a path that escapes is an error.
func Plan(in PlanInput) ([]install.Target, map[string][]string, error) {
	selected, err := selectHarnesses(in.Harnesses)
	if err != nil {
		return nil, nil, err
	}
	if len(selected) == 0 {
		return nil, nil, nil
	}

	root := filepath.Clean(in.WorktreeRoot)
	interior := []byte(harness.DispatchInterior(in.RunTracker))
	codexInterior := interior
	if selected[harnessCodex] {
		codexInterior = []byte(harness.CodexDispatchInterior(in.RunTracker))
	}

	var targets []install.Target
	owners := map[string][]string{}

	add := func(t install.Target, harnessOwners ...string) error {
		cleaned := filepath.Clean(t.Path)
		if !contained(root, cleaned) {
			return fmt.Errorf("reposeed: planned path %q escapes worktree root %q", cleaned, root)
		}
		t.Path = cleaned
		targets = append(targets, t)
		owners[cleaned] = append(owners[cleaned], harnessOwners...)
		return nil
	}

	// The codex/opencode co-owned AGENTS.md block. Present iff at least one of
	// the two opted in; each present one is an owner. Claude never co-owns this
	// block — when it shares, it does so through a CLAUDE.md link, planned
	// below.
	sharedAgents := selected[harnessCodex] || selected[harnessOpencode]
	if sharedAgents {
		var agentsOwners []string
		if selected[harnessCodex] {
			agentsOwners = append(agentsOwners, harnessCodex)
		}
		if selected[harnessOpencode] {
			agentsOwners = append(agentsOwners, harnessOpencode)
		}
		if err := add(install.Target{
			Path:       filepath.Join(root, agentsMDName),
			Kind:       install.KindManagedBlock,
			Content:    codexInterior,
			BlockName:  dispatchBlockName,
			Annotation: dispatchAnnotation,
			Role:       roleDispatch,
		}, agentsOwners...); err != nil {
			return nil, nil, err
		}
	}

	if selected[harnessClaude] {
		// Claude shares AGENTS.md only when that shared surface exists AND
		// CLAUDE.md is absent or already a proven link — replacing a link
		// loses no user content. A regular file keeps its content (own block),
		// and `other` is a block so inspection reports the conflict. Any other
		// symlinked CLAUDE.md is skipped: a link cannot carry a managed block.
		share := sharedAgents &&
			(in.ClaudeMDState == ClaudeMDAbsent || in.ClaudeMDState == ClaudeMDLinkToAgents)
		isLink := in.ClaudeMDState == ClaudeMDLinkToAgents || in.ClaudeMDState == ClaudeMDForeignLink
		claudePath := filepath.Join(root, claudeMDName)
		var t install.Target
		switch {
		case share:
			t = install.Target{
				Path:       claudePath,
				Kind:       install.KindSymlink,
				LinkTarget: filepath.Join(root, agentsMDName),
				Role:       roleDispatch,
			}
		case isLink:
			// Skipped: no target, so t.Path stays empty.
		default:
			t = install.Target{
				Path:       claudePath,
				Kind:       install.KindManagedBlock,
				Content:    interior,
				BlockName:  dispatchBlockName,
				Annotation: dispatchAnnotation,
				Role:       roleDispatch,
			}
		}
		if t.Path != "" {
			if err := add(t, harnessClaude); err != nil {
				return nil, nil, err
			}
		}
	}

	if selected[harnessCursor] {
		if err := add(install.Target{
			Path:    filepath.Join(root, cursorRuleRel),
			Kind:    install.KindFile,
			Content: cursor.DispatchRuleContent(in.RunTracker),
			Role:    roleDispatch,
		}, harnessCursor); err != nil {
			return nil, nil, err
		}
	}

	for path := range owners {
		sort.Strings(owners[path])
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].Path < targets[j].Path })
	return targets, owners, nil
}

// selectHarnesses validates the opt-in tokens and returns them as a set. An
// unknown token is an error (defense in depth; see the harness constants).
func selectHarnesses(tokens []string) (map[string]bool, error) {
	selected := map[string]bool{}
	for _, h := range tokens {
		switch h {
		case harnessClaude, harnessCodex, harnessOpencode, harnessCursor:
			selected[h] = true
		default:
			return nil, fmt.Errorf("reposeed: unknown harness token %q", h)
		}
	}
	return selected, nil
}

// PrivatePlanInput is the pure input to PlanPrivate. WorktreeRoot is the
// PRIMARY worktree, a canonical absolute path; CommonDir is its git common dir
// and must be WorktreeRoot/.git. Harnesses are the repository's validated
// opt-in tokens; RunTracker is the run-tracker payload the interior carries.
type PrivatePlanInput struct {
	WorktreeRoot string
	CommonDir    string
	Harnesses    []string
	RunTracker   []byte
}

// PlanPrivate renders a private repository's single parent-facing target: the
// dispatch managed block in <CommonDir>/dckt/AGENTS.md, owned by every selected
// harness, carrying the Codex interior when codex is selected. It plans nothing
// in the working tree and nothing at all when no harness is selected. A
// CommonDir other than WorktreeRoot/.git (a separate git dir) is an error.
func PlanPrivate(in PrivatePlanInput) ([]install.Target, map[string][]string, error) {
	selected, err := selectHarnesses(in.Harnesses)
	if err != nil {
		return nil, nil, err
	}
	root := filepath.Clean(in.WorktreeRoot)
	common := filepath.Clean(in.CommonDir)
	if common != filepath.Join(root, ".git") {
		return nil, nil, fmt.Errorf("reposeed: unsupported separate-git-dir layout: git common dir %q is not %q",
			common, filepath.Join(root, ".git"))
	}
	if len(selected) == 0 {
		return nil, nil, nil
	}

	content := harness.DispatchInterior(in.RunTracker)
	if selected[harnessCodex] {
		content = harness.CodexDispatchInterior(in.RunTracker)
	}
	path := filepath.Clean(layout.PrivateInstructionsPath(common))
	if !contained(root, path) {
		return nil, nil, fmt.Errorf("reposeed: planned path %q escapes worktree root %q", path, root)
	}
	owners := make([]string, 0, len(selected))
	for h := range selected {
		owners = append(owners, h)
	}
	sort.Strings(owners)
	return []install.Target{{
		Path:       path,
		Kind:       install.KindManagedBlock,
		Content:    []byte(content),
		BlockName:  dispatchBlockName,
		Annotation: dispatchAnnotation,
		Role:       roleDispatch,
	}}, map[string][]string{path: owners}, nil
}

// contained reports whether cleaned path p lies at or under cleaned root. Both
// are already filepath.Clean'd; the separator guard keeps "/repofoo" from
// reading as inside "/repo".
func contained(root, p string) bool {
	if p == root {
		return true
	}
	return strings.HasPrefix(p, root+string(filepath.Separator))
}
