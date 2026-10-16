// Package reposeed plans the parent-facing repository surfaces docket reconciles
// for a repository that explicitly opts in via agent_harnesses. It is pure: it
// renders declarative install.Target values and their harness ownership and
// touches nothing on disk. It emits parent-facing routing instructions ONLY —
// never per-repository agent definitions and never skills (change 0351). The
// machine installer (internal/install) applies the plan and the caller
// (internal/app) supplies the CLAUDE.md and AGENTS.md pre-states, since Plan
// never stats a path.
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
// a path). It decides whether Claude can safely share AGENTS.md via a relative
// link — co-owning the AGENTS.md dispatch block — or must own its own managed
// block in CLAUDE.md.
type ClaudeMDState int

const (
	// ClaudeMDAbsent — no CLAUDE.md exists; a share is a fresh link.
	ClaudeMDAbsent ClaudeMDState = iota
	// ClaudeMDRegularFile — a regular CLAUDE.md exists; keep its content and
	// give it its own managed block, never replace it with a link.
	ClaudeMDRegularFile
	// ClaudeMDLinkToAgents — CLAUDE.md is already a proven link to the
	// sibling AGENTS.md (relative, or the absolute form this plan writes). The
	// link shares AGENTS.md, so the dispatch block goes into the file the link
	// points to and the plan re-plans the same link.
	ClaudeMDLinkToAgents
	// ClaudeMDOther — anything else (a foreign kind, an unreadable path).
	// Planned as a managed block so inspection reports the conflict with a
	// remedy rather than silently overwriting.
	ClaudeMDOther
	// ClaudeMDForeignLink — CLAUDE.md is a symlink that is not a proven
	// link to the sibling AGENTS.md. A link cannot carry a managed block, so the
	// plan skips it.
	ClaudeMDForeignLink
)

// PlanInput is the pure input to Plan. WorktreeRoot is a canonical absolute
// path; Harnesses are the already-validated opt-in tokens this run plans for (a
// --harness scope narrows them); OptIns is the repository's full opt-in set,
// which picks the AGENTS.md interior and decides the no-CLAUDE.md share, and nil
// means Harnesses is already the full set. RunTracker is the run-tracker payload
// the interiors carry verbatim. ClaudeMDState and AgentsMDState are the
// caller-computed pre-states of the two files, since Plan never stats a path.
type PlanInput struct {
	WorktreeRoot  string
	Harnesses     []string
	OptIns        []string
	RunTracker    []byte
	ClaudeMDState ClaudeMDState
	AgentsMDState AgentsMDState
}

// AgentsMDState is the AGENTS.md pre-state the caller computes with os.Lstat.
type AgentsMDState int

const (
	// AgentsMDMissing — no AGENTS.md exists (the zero value).
	AgentsMDMissing AgentsMDState = iota
	// AgentsMDRegularFile — AGENTS.md is a regular file.
	AgentsMDRegularFile
	// AgentsMDOther — a symlink (circular included), a directory, or a path the
	// probe could not read. Never read as missing; Claude does not share it.
	AgentsMDOther
)

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

	full := selected
	if in.OptIns != nil {
		if full, err = selectHarnesses(in.OptIns); err != nil {
			return nil, nil, err
		}
	}

	root := filepath.Clean(in.WorktreeRoot)
	interior := []byte(harness.DispatchInterior(in.RunTracker))
	// AGENTS.md is shared, so its interior follows the full opt-in set: a run
	// scoped away from codex must not strip the clause codex still owns.
	codexInterior := interior
	if full[harnessCodex] {
		codexInterior = []byte(harness.CodexDispatchInterior(in.RunTracker))
	}

	// Claude shares AGENTS.md — co-owning its dispatch block and linking
	// CLAUDE.md to it — when CLAUDE.md already links to AGENTS.md and AGENTS.md
	// is missing or a regular file (the link resolves to where the block goes),
	// or when there is no CLAUDE.md and AGENTS.md exists as a regular file or
	// codex/opencode is opted in anywhere. Replacing a link or filling an absent
	// path loses no user content. The no-CLAUDE.md share follows the full opt-in
	// set, so a scoped run decides it the same way an unscoped one does.
	claudeShares := false
	if selected[harnessClaude] {
		switch in.ClaudeMDState {
		case ClaudeMDLinkToAgents:
			claudeShares = in.AgentsMDState == AgentsMDMissing || in.AgentsMDState == AgentsMDRegularFile
		case ClaudeMDAbsent:
			claudeShares = full[harnessCodex] || full[harnessOpencode] || in.AgentsMDState == AgentsMDRegularFile
		}
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

	// The co-owned AGENTS.md block. Present iff codex or opencode is selected or
	// Claude shares it; each of those is an owner. Claude co-owns the block when
	// it shares, and links CLAUDE.md to it below.
	if selected[harnessCodex] || selected[harnessOpencode] || claudeShares {
		var agentsOwners []string
		if claudeShares {
			agentsOwners = append(agentsOwners, harnessClaude)
		}
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
		// A sharing Claude links CLAUDE.md to AGENTS.md (see claudeShares). A
		// regular file keeps its content (own block), and `other` is a block so
		// inspection reports the conflict. A symlinked CLAUDE.md that does not
		// share — a foreign link, or a link to a non-regular AGENTS.md — is
		// skipped: a link cannot carry a managed block.
		isLink := in.ClaudeMDState == ClaudeMDLinkToAgents || in.ClaudeMDState == ClaudeMDForeignLink
		claudePath := filepath.Join(root, claudeMDName)
		var t install.Target
		switch {
		case claudeShares:
			// Relative, because CLAUDE.md is committed: an absolute link
			// names this machine's checkout and dangles in every other clone.
			t = install.Target{
				Path:         claudePath,
				Kind:         install.KindSymlink,
				LinkTarget:   filepath.Join(root, agentsMDName),
				LinkRelative: true,
				Role:         roleDispatch,
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

// SurfacePaths lists, sorted, every repository-relative path Plan can emit for
// any harness selection and CLAUDE.md state — the working-tree surfaces a
// shared repository may commit. TestSurfacePathsCoverPlan derives the set from
// Plan itself, so a new surface cannot be missed here.
func SurfacePaths() []string {
	return []string{cursorRuleRel, agentsMDName, claudeMDName}
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
// and must be WorktreeRoot/.git. Harnesses are the validated opt-in tokens this
// run plans for (a --harness scope narrows them); OptIns is the repository's
// full opt-in set, which picks the shared file's interior, and nil means
// Harnesses is already the full set. RunTracker is the run-tracker payload the
// interior carries.
type PrivatePlanInput struct {
	WorktreeRoot string
	CommonDir    string
	Harnesses    []string
	OptIns       []string
	RunTracker   []byte
}

// PlanPrivate renders a private repository's single parent-facing target: the
// dispatch managed block in <CommonDir>/dckt/AGENTS.md, owned by every selected
// harness, carrying the Codex interior when codex is opted in. The interior
// follows the full opt-in set, not the selection: the file is shared, so a run
// scoped away from codex must not strip the clause codex still owns. It plans nothing
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

	interiorSet := selected
	if in.OptIns != nil {
		if interiorSet, err = selectHarnesses(in.OptIns); err != nil {
			return nil, nil, err
		}
	}
	content := harness.DispatchInterior(in.RunTracker)
	if interiorSet[harnessCodex] {
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
