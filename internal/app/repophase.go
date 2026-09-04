package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/install"
	"github.com/danielhanold/docket/internal/reposeed"
)

// This file is the app-layer assembler that turns a repository selection into the
// install-package RepoPhase one installation transaction consumes. internal/app
// is the one layer that may import both internal/reposeed (the pure surface
// planner and the ownership record) and internal/gitcli (worktree discovery), so
// it does the discovery, the provenance judgement, the surface planning, and the
// ownership projection here, then hands the finished half across the seam as
// ordinary install.Targets, owners, a projected prior State, proof-gated
// removals, and the record bytes to publish. internal/install never learns that
// reposeed or gitcli exist.

// RepoResolutionError carries the stable machine reason a repository resolution
// failure classifies under, so the CLI boundary presents it through exactly the
// same install-reason table as the service's own refusals rather than a second
// opinion.
type RepoResolutionError struct {
	Reason string
	Err    error
}

func (e *RepoResolutionError) Error() string { return e.Err.Error() }
func (e *RepoResolutionError) Unwrap() error { return e.Err }

// ResolveRepoPhase turns a repository selection into the RepoPhase the installer
// applies. repoDir is the explicit --repo-dir value ("" means discover the Git
// working tree containing the current directory); harnessScope is the explicit
// --harness selection (nil means the full opt-in set); runGate is the run-gate
// payload the surfaces carry; legacy is the frozen reproducer that proof-gates a
// removal against a byte-exact legacy artifact. The second return is the selected
// working-tree root, for reporting.
//
// The outcomes are three: a machine-only run — an omitted --repo-dir with a
// current directory outside any Git tree — returns (nil, "", nil); an
// authorized repository returns a full phase; and everything unresolvable
// returns a *RepoResolutionError carrying the reason the CLI classifies on.
//
// rctx is the resolution context the caller owns — the CLI's install path
// passes a tolerant one (change 0392); this assembler carries no
// install-specific knowledge of why. The third return is the warning-severity
// diagnostics from the repository resolve, for the install result to surface;
// it is nil on the machine-only and error paths.
func ResolveRepoPhase(ctx context.Context, git *gitcli.Client, repoDir string, harnessScope []string, runGate []byte, legacy install.LegacyReproducer, rctx config.ResolveContext) (*install.RepoPhase, string, []config.Diagnostic, error) {
	explicit := strings.TrimSpace(repoDir) != ""
	invocation := repoDir
	if !explicit {
		wd, err := os.Getwd()
		if err != nil {
			return nil, "", nil, &RepoResolutionError{Reason: install.ReasonInvalidRepoDir,
				Err: fmt.Errorf("--repo-dir omitted and the current directory could not be determined: %w", err)}
		}
		invocation = wd
	}

	wt, err := git.DiscoverWorktree(ctx, gitcli.DiscoverOptions{InvocationPath: invocation})
	if err != nil {
		if explicit {
			return nil, "", nil, &RepoResolutionError{Reason: install.ReasonInvalidRepoDir,
				Err: fmt.Errorf("--repo-dir %q is not a Git working tree: %w", repoDir, err)}
		}
		// An omitted --repo-dir outside any Git tree is not a failure: the machine
		// install proceeds and the not-authorized action prints.
		return nil, "", nil, nil
	}
	root, gitDir := wt.Root, wt.GitDir
	recordPath := reposeed.RecordPath(gitDir)

	// The repository configuration layer. LoadFilesystemSources reads the global
	// layer too — which is exactly what lets the provenance guard below tell a
	// repository-declared opt-in from a global one that must never grant authority.
	sources, err := config.LoadFilesystemSources(config.FSOptions{RepoDir: root})
	if err != nil {
		return nil, "", nil, &RepoResolutionError{Reason: ReasonInvalidConfig, Err: err}
	}
	snap, diags, err := config.Resolve(sources, rctx)
	if err != nil {
		return nil, "", nil, &RepoResolutionError{Reason: ReasonInvalidConfig, Err: err}
	}
	warnings := config.Warnings(diags)

	ah := snap.Effective.AgentHarnesses
	// The write-authority guard (change 0351): agent_harnesses grants repository
	// write authority only when it is explicit AND resolved from the repository or
	// repository-local layer. A global-layer declaration, or a mere agents: table,
	// resolves but never authorizes.
	authorized := ah.Explicit && isRepositoryLayer(ah.Provenance.Layer)
	if !authorized {
		return &install.RepoPhase{Authorized: false, Worktree: root, RecordPath: recordPath}, root, warnings, nil
	}

	optIns := append([]string(nil), ah.Value...)
	inScope := scopeSet(harnessScope)
	effective := reconciledHarnesses(optIns, inScope)

	targets, owners, err := reposeed.Plan(reposeed.PlanInput{
		WorktreeRoot:  root,
		Harnesses:     effective,
		RunGate:       runGate,
		ClaudeMDState: classifyClaudeMD(root),
	})
	if err != nil {
		return nil, "", nil, &RepoResolutionError{Reason: ReasonInvalidConfig, Err: err}
	}

	prior, err := reposeed.LoadRecord(recordPath)
	if err != nil {
		return nil, "", nil, &RepoResolutionError{Reason: install.ReasonStateInvalid, Err: err}
	}
	var priorState *install.State
	if prior != nil {
		priorState = prior.ToState(root)
	}

	recordBytes, err := composeRecordBytes(targets, owners, prior, root, optIns)
	if err != nil {
		return nil, "", nil, &RepoResolutionError{Reason: install.ReasonInternal, Err: err}
	}

	removals, err := computeRemovals(prior, root, optIns, inScope, priorState, legacy)
	if err != nil {
		return nil, "", nil, err
	}

	return &install.RepoPhase{
		Authorized:  true,
		Targets:     targets,
		Owners:      owners,
		PriorState:  priorState,
		Removals:    removals,
		RecordPath:  recordPath,
		RecordBytes: recordBytes,
		Worktree:    root,
	}, root, warnings, nil
}

// isRepositoryLayer mirrors config's own write-authority predicate: only the
// repository and repository-local layers grant the installer authority to touch
// repository surfaces.
func isRepositoryLayer(layer config.LayerKind) bool {
	return layer == config.LayerRepository || layer == config.LayerRepositoryLocal
}

// scopeSet is the set of harnesses an explicit --harness selection allows the run
// to touch. A nil selection is the unscoped default and returns a nil set, which
// inScope reads as "every harness is in scope".
func scopeSet(scope []string) map[string]bool {
	if len(scope) == 0 {
		return nil
	}
	set := make(map[string]bool, len(scope))
	for _, h := range scope {
		set[h] = true
	}
	return set
}

// inScope reports whether harness h may be touched this run. A nil set (no
// --harness) puts every harness in scope.
func inScope(set map[string]bool, h string) bool { return set == nil || set[h] }

// reconciledHarnesses is the opt-in harnesses this run actually plans surfaces
// for: the intersection of the opt-ins with the in-scope set, in a stable order.
func reconciledHarnesses(optIns []string, scope map[string]bool) []string {
	var out []string
	for _, h := range optIns {
		if inScope(scope, h) {
			out = append(out, h)
		}
	}
	return out
}

// classifyClaudeMD computes the CLAUDE.md pre-state reposeed.Plan needs (it never
// stats a path): absent, a proven relative link to AGENTS.md, a regular file, or
// anything else. A stat error other than "absent" degrades to ClaudeMDOther so
// reposeed plans a managed block and inspection reports the conflict rather than
// this assembler guessing.
func classifyClaudeMD(root string) reposeed.ClaudeMDState {
	path := filepath.Join(root, "CLAUDE.md")
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return reposeed.ClaudeMDAbsent
		}
		return reposeed.ClaudeMDOther
	}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		dest, err := os.Readlink(path)
		if err != nil {
			return reposeed.ClaudeMDOther
		}
		// A relative link that resolves to the sibling AGENTS.md is the shareable
		// state; anything else (an absolute link, a foreign destination) is other.
		if !filepath.IsAbs(dest) &&
			filepath.Clean(filepath.Join(root, dest)) == filepath.Join(root, "AGENTS.md") {
			return reposeed.ClaudeMDLinkToAgents
		}
		return reposeed.ClaudeMDOther
	case info.Mode().IsRegular():
		return reposeed.ClaudeMDRegularFile
	default:
		return reposeed.ClaudeMDOther
	}
}

// composeRecordBytes renders the ownership record this run publishes: the new
// surfaces it reconciled, plus every prior surface an opted-in harness still
// requires but this run did not reconcile — an unrelated harness left untouched
// by a scoped run, whose record must survive verbatim (change 0351 / Task 7
// carry-forward). A prior surface whose owners have all dropped out of the opt-in
// set is not carried; it is retired through computeRemovals instead.
func composeRecordBytes(targets []install.Target, owners map[string][]string, prior *reposeed.Record, root string, optIns []string) ([]byte, error) {
	desired, err := reposeed.DesiredRecord(targets, owners, root)
	if err != nil {
		return nil, err
	}
	byPath := make(map[string]int, len(desired.Surfaces))
	for i, s := range desired.Surfaces {
		byPath[s.Path] = i
	}

	optSet := make(map[string]bool, len(optIns))
	for _, h := range optIns {
		optSet[h] = true
	}

	if prior != nil {
		for _, s := range prior.Surfaces {
			surviving := intersectSorted(s.Harnesses, optSet)
			if len(surviving) == 0 {
				continue // every owner dropped: retired, never carried.
			}
			if idx, ok := byPath[s.Path]; ok {
				// Reconciled this run: keep an out-of-scope opted-in co-owner listed
				// so a future run still sees it (a shared AGENTS.md under a scope that
				// names only one of its owners).
				desired.Surfaces[idx].Harnesses = unionSorted(desired.Surfaces[idx].Harnesses, surviving)
				continue
			}
			// Not reconciled but still wanted: carry the prior surface forward with
			// its surviving owners.
			carried := s
			carried.Harnesses = surviving
			byPath[s.Path] = len(desired.Surfaces)
			desired.Surfaces = append(desired.Surfaces, carried)
		}
	}

	sort.Slice(desired.Surfaces, func(i, j int) bool { return desired.Surfaces[i].Path < desired.Surfaces[j].Path })
	return reposeed.EncodeRecord(desired)
}

// computeRemovals proof-gates the retirement of every prior surface this run
// drops. A surface is retired when NO opted-in owner still requires it AND at
// least one of its owners is in scope — so a shared AGENTS.md is retired only when
// none of its remaining owners is opted in, and a scoped run never touches an
// out-of-scope surface. The proof is delegated to install.PlanGlobalRetirements,
// which removes only an artifact byte-provably docket's (prior record or frozen
// legacy) and refuses the whole run on anything it cannot prove.
func computeRemovals(prior *reposeed.Record, root string, optIns []string, scope map[string]bool, priorState *install.State, legacy install.LegacyReproducer) ([]install.TargetRecord, error) {
	if prior == nil {
		return nil, nil
	}
	optSet := make(map[string]bool, len(optIns))
	for _, h := range optIns {
		optSet[h] = true
	}

	var historical []install.Target
	harnessByPath := map[string]string{}
	for _, s := range prior.Surfaces {
		stillWanted := false
		hasInScopeOwner := false
		for _, owner := range s.Harnesses {
			if optSet[owner] {
				stillWanted = true
			}
			if inScope(scope, owner) {
				hasInScopeOwner = true
			}
		}
		if stillWanted || !hasInScopeOwner {
			continue // kept by a remaining owner, or entirely out of scope.
		}
		abs := filepath.Clean(filepath.Join(root, filepath.FromSlash(s.Path)))
		tgt := install.Target{
			Path:      abs,
			Kind:      s.Kind,
			BlockName: s.BlockName,
			Role:      "dispatch",
		}
		if s.Kind == install.KindSymlink {
			// A shared link's retirement removal must name its destination, both to
			// prove ownership (the recorded link still matching disk) and so the
			// journaled removal records the link it retired.
			tgt.LinkTarget = filepath.Clean(filepath.Join(root, filepath.FromSlash(s.LinkTarget)))
		}
		historical = append(historical, tgt)
		harnessByPath[abs] = strings.Join(s.Harnesses, ",")
	}
	if len(historical) == 0 {
		return nil, nil
	}

	removals, conflicts, err := install.PlanGlobalRetirements(install.UserRoots{}, historical, priorState, legacy)
	if err != nil {
		return nil, &RepoResolutionError{Reason: install.ReasonFilesystemFailed, Err: err}
	}
	if len(conflicts) > 0 {
		return nil, &RepoResolutionError{Reason: install.ReasonOwnershipConflict,
			Err: fmt.Errorf("install: %d repository surface(s) to retire are not provably docket's", len(conflicts))}
	}
	// Attribute each removal to its recorded owners, since the projected prior
	// state carries no harness attribution.
	for i := range removals {
		if owner := harnessByPath[removals[i].Path]; owner != "" {
			removals[i].Harness = owner
		}
	}
	return removals, nil
}

// intersectSorted returns the members of list that are in set, sorted and
// de-duplicated.
func intersectSorted(list []string, set map[string]bool) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range list {
		if set[v] && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}

// unionSorted merges two string slices into a sorted, de-duplicated set.
func unionSorted(a, b []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range append(append([]string(nil), a...), b...) {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}
