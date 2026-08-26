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
	// ClaudeMDOther — anything else (an unowned link, a foreign kind). Planned
	// as a managed block so inspection reports the conflict with a remedy
	// rather than silently overwriting.
	ClaudeMDOther
)

// PlanInput is the pure input to Plan. WorktreeRoot is a canonical absolute
// path; Harnesses are the repository's explicit, already-validated opt-in
// tokens; RunGate is the run-gate payload the interiors carry verbatim.
type PlanInput struct {
	WorktreeRoot  string
	Harnesses     []string
	RunGate       []byte
	ClaudeMDState ClaudeMDState
}

// Plan renders the parent-facing repository targets for the opted-in harnesses.
// It is pure and emits NO agent definitions and NO skills. The second return
// maps each cleaned target path to its harness owners — a shared surface carries
// several. Every planned path must resolve inside WorktreeRoot after
// filepath.Clean; a path that escapes is an error.
func Plan(in PlanInput) ([]install.Target, map[string][]string, error) {
	selected := map[string]bool{}
	for _, h := range in.Harnesses {
		switch h {
		case harnessClaude, harnessCodex, harnessOpencode, harnessCursor:
			selected[h] = true
		default:
			return nil, nil, fmt.Errorf("reposeed: unknown harness token %q", h)
		}
	}
	if len(selected) == 0 {
		return nil, nil, nil
	}

	root := filepath.Clean(in.WorktreeRoot)
	interior := []byte(harness.DispatchInterior(in.RunGate))

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
			Content:    interior,
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
		// and `other` is a block so inspection reports the conflict.
		share := sharedAgents &&
			(in.ClaudeMDState == ClaudeMDAbsent || in.ClaudeMDState == ClaudeMDLinkToAgents)
		claudePath := filepath.Join(root, claudeMDName)
		var t install.Target
		if share {
			t = install.Target{
				Path:       claudePath,
				Kind:       install.KindSymlink,
				LinkTarget: filepath.Join(root, agentsMDName),
				Role:       roleDispatch,
			}
		} else {
			t = install.Target{
				Path:       claudePath,
				Kind:       install.KindManagedBlock,
				Content:    interior,
				BlockName:  dispatchBlockName,
				Annotation: dispatchAnnotation,
				Role:       roleDispatch,
			}
		}
		if err := add(t, harnessClaude); err != nil {
			return nil, nil, err
		}
	}

	if selected[harnessCursor] {
		if err := add(install.Target{
			Path:    filepath.Join(root, cursorRuleRel),
			Kind:    install.KindFile,
			Content: cursor.DispatchRuleContent(in.RunGate),
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

// contained reports whether cleaned path p lies at or under cleaned root. Both
// are already filepath.Clean'd; the separator guard keeps "/repofoo" from
// reading as inside "/repo".
func contained(root, p string) bool {
	if p == root {
		return true
	}
	return strings.HasPrefix(p, root+string(filepath.Separator))
}
