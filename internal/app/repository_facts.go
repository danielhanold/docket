package app

import (
	"context"
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/reposetup"
)

// This file is the app-layer fact gatherer for the `repository` command family.
// It composes the landed gitcli adapter — discovery, targeted fetch, remote-ref
// probing, immutable object sources, and worktree/status facts — with
// filesystem configuration resolution, and fills a reposetup.Facts value that
// the pure classifier decides over. Its single discipline is the three-valued
// probe contract: every probe error lands as reposetup.PresenceUnknown WITH the
// error retained in the returned diagnostics, and is NEVER collapsed into
// absence (learning probe-error-is-not-clean-absence). A Git behavior that
// needs shell assembly or CLI-text parsing lives in a gitcli method, never here.

// setupRemote is the ONE spelling site of the product's remote name for the
// repository command family. It returns the existing originRemote constant so no
// new "origin" literal appears anywhere in the repository services; minting a
// remote config key is out of scope for this change.
func setupRemote() gitcli.RemoteName { return originRemote }

// SetupDeps carries the seams every repository service shares: the gitcli client
// that performs all Git effects and the invocation directory from which the
// canonical primary worktree is discovered.
type SetupDeps struct {
	Git     *gitcli.Client
	RepoDir string // invocation dir; Discover resolves the canonical primary
}

// setupProber is the narrow interface over the gitcli methods the fact gatherer
// calls. *gitcli.Client satisfies it in production; package tests inject a fake
// to force a per-probe failure and prove it maps to PresenceUnknown, never to a
// silent absence.
type setupProber interface {
	Discover(ctx context.Context, opts gitcli.DiscoverOptions) (gitcli.Repository, error)
	RemoteDefaultBranch(ctx context.Context, repo gitcli.Repository, remote gitcli.RemoteName) (gitcli.RefName, error)
	FetchBranch(ctx context.Context, repo gitcli.Repository, remote gitcli.RemoteName, branch gitcli.RefName) (gitcli.Revision, error)
	ProbeRemoteBranch(ctx context.Context, repo gitcli.Repository, remote gitcli.RemoteName, ref gitcli.RefName) (gitcli.RemoteRef, error)
	OpenObjectSource(ctx context.Context, repo gitcli.Repository, rev gitcli.Revision) (gitcli.ObjectSource, error)
	ChangedPaths(ctx context.Context, dir string) ([]gitcli.PathChange, error)
	ResolveRef(ctx context.Context, repo gitcli.Repository, ref gitcli.RefName) (gitcli.ObjectID, error)
	ListWorktrees(ctx context.Context, repo gitcli.Repository) ([]gitcli.WorktreeInfo, error)
}

// setupDiag records one probe that could not be resolved: the probe name and the
// retained error. A diagnostic is emitted only alongside a PresenceUnknown fact,
// so a caller can distinguish "not proven because a probe errored" from a clean
// absence.
type setupDiag struct {
	Probe string
	Err   error
}

// setupContext carries everything the later init/migrate/check phases need that
// is not part of the pure Facts value: the resolved configuration, the
// discovered repository, the resolved branch names, and the exact pinned
// revisions captured at gather time. The pinned integration revision is the
// authoritative source revision every later phase reads and keys on.
type setupContext struct {
	cfg               config.Effective
	repo              gitcli.Repository
	defaultBranch     string
	integrationBranch string
	metadataBranch    string
	sourceRevision    string // pinned authoritative integration tip
	metadataTip       string // remote docket tip when present, else ""
	diagnostics       []setupDiag
}

// docketWorktreeName is the fixed root-level metadata worktree directory.
const docketWorktreeName = ".docket"

// GatherSetupFacts discovers the canonical repository, resolves configuration
// (a mutation preflight for init/migrate, a read preflight for check), fetches
// the authoritative remote default/integration/metadata refs, and fills a
// reposetup.Facts value. Every probe error lands as PresenceUnknown with the
// error retained in setupContext.diagnostics — never as Absent — so the pure
// classifier can never read an errored probe as a clean absence.
func GatherSetupFacts(ctx context.Context, d SetupDeps, forMutation bool) (reposetup.Facts, setupContext, error) {
	return gatherSetupFacts(ctx, d.Git, d.RepoDir, forMutation)
}

// gatherSetupFacts is GatherSetupFacts over the narrow prober seam, so a fake can
// inject per-probe failures. forMutation is retained for the read/mutation
// preflight distinction later phases layer on; the probe set is the same either
// way — the classifier, not the gatherer, gates a destructive write.
func gatherSetupFacts(ctx context.Context, p setupProber, repoDir string, forMutation bool) (reposetup.Facts, setupContext, error) {
	var f reposetup.Facts
	var sc setupContext

	repo, err := p.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		return f, sc, classifyGitFailure(err)
	}
	sc.repo = repo

	// The remote default branch is the first authoritative read. A failure here
	// means the remote is not configured or not reachable; both leave the
	// remote-configured and default-branch probes Unknown (safe), and the
	// classifier reports `unknown` — never a fabricated absence that could
	// authorize a create.
	defaultRef, err := p.RemoteDefaultBranch(ctx, repo, setupRemote())
	if err != nil {
		sc.diagnostics = append(sc.diagnostics, setupDiag{Probe: "remote-default-branch", Err: err})
		return f, sc, nil
	}
	defaultBranch, ok := shortBranch(defaultRef)
	if !ok {
		sc.diagnostics = append(sc.diagnostics, setupDiag{Probe: "remote-default-branch", Err: errors.New("remote default ref is not a branch")})
		return f, sc, nil
	}
	f.RemoteConfigured = reposetup.PresencePresent
	f.RemoteDefaultBranch.Presence = reposetup.PresencePresent
	sc.defaultBranch = defaultBranch
	if rev, ferr := p.FetchBranch(ctx, repo, setupRemote(), defaultRef); ferr == nil {
		f.RemoteDefaultBranch.Tip = string(rev.Commit)
	}

	// Configuration is resolved from the primary worktree's filesystem layers,
	// exactly as the installer's repository-phase resolver does, so an explicit
	// repository/repository-local agent_harnesses declaration authorizes surfaces
	// and a global-layer one never does. An invalid configuration fails closed.
	eff, err := resolveSetupConfig(repo.PrimaryWorktree, defaultBranch)
	if err != nil {
		return f, sc, err
	}
	sc.cfg = eff
	sc.integrationBranch = eff.IntegrationBranch.Value
	sc.metadataBranch = eff.MetadataBranch.Value
	f.SurfacesAuthorized = eff.AgentHarnesses.Explicit && isRepositoryLayer(eff.AgentHarnesses.Provenance.Layer)

	// The authoritative integration tip is the source revision every later phase
	// pins and keys on. A fetch failure leaves the integration probe Unknown.
	integrationRef := gitcli.RefName(branchRefPrefix + sc.integrationBranch)
	integrationRev, ierr := p.FetchBranch(ctx, repo, setupRemote(), integrationRef)
	if ierr != nil {
		sc.diagnostics = append(sc.diagnostics, setupDiag{Probe: "remote-integration-branch", Err: ierr})
	} else {
		f.RemoteIntegration.Presence = reposetup.PresencePresent
		f.RemoteIntegration.Tip = string(integrationRev.Commit)
		sc.sourceRevision = string(integrationRev.Commit)
	}

	// The remote docket branch presence is an authoritative ls-remote probe. Its
	// root shape is deliberately NOT inspected here: init publishes with
	// create-only protection and re-reads the exact remote shape at that boundary
	// (adopt on the expected empty orphan, refuse on anything foreign), so the
	// authoritative adopt/conflict decision keys on the promised remote state, not
	// a gather-time proxy.
	metaRef := gitcli.RefName(branchRefPrefix + sc.metadataBranch)
	rr, merr := p.ProbeRemoteBranch(ctx, repo, setupRemote(), metaRef)
	if merr != nil {
		sc.diagnostics = append(sc.diagnostics, setupDiag{Probe: "remote-metadata-branch", Err: merr})
	} else {
		switch rr.State {
		case gitcli.RemoteRefFound:
			f.RemoteMetadata.Presence = reposetup.PresencePresent
			f.RemoteMetadata.Tip = string(rr.Commit)
			sc.metadataTip = string(rr.Commit)
		case gitcli.RemoteRefAbsent:
			f.RemoteMetadata.Presence = reposetup.PresenceAbsent
		}
	}

	// The live planning surface is proven from the authoritative integration
	// COMMIT tree, never the working tree: it separates fresh from legacy. It is
	// only meaningful (and only consulted by the classifier) when the metadata
	// branch is absent, but it is gathered whenever the integration tip is known.
	if sc.sourceRevision != "" {
		f.LiveSurface = liveSurfacePresence(ctx, p, repo, sc.sourceRevision, eff, &sc)
	}

	// Primary-worktree supported-contract facts. The clean check ignores the
	// docket-managed working-tree surfaces (the unstaged .gitignore edit and the
	// parent-facing dispatch surfaces) and ignored paths, so a repeat init whose
	// only working-tree change is its own pending review edits is still clean.
	f.PrimaryClean = primaryCleanPresence(ctx, p, repo, &sc)
	if sc.sourceRevision != "" {
		f.PrimaryAtRemoteTip = primaryAtTipPresence(ctx, p, repo, sc.sourceRevision, &sc)
	}

	// The .docket path: absent, a correctly registered owned worktree on the
	// metadata branch, or a foreign directory / conflicting registration.
	f.DocketWorktree = docketWorktreeFact(ctx, p, repo, metaRef)

	return f, sc, nil
}

// resolveSetupConfig resolves the effective configuration from the primary
// worktree's filesystem layers (global ⊕ repository ⊕ repository-local), the same
// layer stack the installer's repository phase resolves from. An invalid
// configuration is a fail-closed error.
func resolveSetupConfig(primaryWorktree, defaultBranch string) (config.Effective, error) {
	sources, err := config.LoadFilesystemSources(config.FSOptions{RepoDir: primaryWorktree})
	if err != nil {
		return config.Effective{}, &RepoResolutionError{Reason: ReasonInvalidConfig, Err: err}
	}
	snap, _, err := config.Resolve(sources, config.ResolveContext{DefaultBranch: defaultBranch})
	if err != nil {
		return config.Effective{}, &RepoResolutionError{Reason: ReasonInvalidConfig, Err: err}
	}
	return snap.Effective, nil
}

// liveSurfacePresence reports whether the authoritative integration commit tree
// carries a live planning surface — any blob under the configured changes
// active/ directory, or the changes BOARD.md. A read error is PresenceUnknown
// with a diagnostic, never a false absence.
func liveSurfacePresence(ctx context.Context, p setupProber, repo gitcli.Repository, rev string, eff config.Effective, sc *setupContext) reposetup.Presence {
	src, err := p.OpenObjectSource(ctx, repo, gitcli.Revision{Commit: gitcli.ObjectID(rev)})
	if err != nil {
		sc.diagnostics = append(sc.diagnostics, setupDiag{Probe: "live-surface", Err: err})
		return reposetup.PresenceUnknown
	}
	changes := eff.ChangesDir.Value
	activePrefix := gitcli.RepoPath(path.Join(changes, "active"))
	entries, err := src.ListTree(ctx, []gitcli.RepoPath{activePrefix})
	if err != nil {
		sc.diagnostics = append(sc.diagnostics, setupDiag{Probe: "live-surface", Err: err})
		return reposetup.PresenceUnknown
	}
	for _, e := range entries {
		if e.Type == "blob" {
			return reposetup.PresencePresent
		}
	}
	boardPath := gitcli.RepoPath(path.Join(changes, "BOARD.md"))
	results, err := src.ReadBlobs(ctx, []gitcli.RepoPath{boardPath})
	if err != nil {
		sc.diagnostics = append(sc.diagnostics, setupDiag{Probe: "live-surface", Err: err})
		return reposetup.PresenceUnknown
	}
	if len(results) == 1 && results[0].Found {
		return reposetup.PresencePresent
	}
	return reposetup.PresenceAbsent
}

// docketManagedWorktreePaths are the repo-relative working-tree paths docket's
// own reconciliation writes and leaves unstaged for human review: the managed
// .gitignore edit and the parent-facing dispatch surfaces. The clean preflight
// ignores exactly these (and ignored paths), so a repeat init whose only
// difference from clean is its own pending review edits is not read as a dirty
// primary.
var docketManagedWorktreePaths = map[string]bool{
	".gitignore":                        true,
	"CLAUDE.md":                         true,
	"AGENTS.md":                         true,
	".cursor/rules/docket-dispatch.mdc": true,
}

// primaryCleanPresence reports whether the primary worktree is clean once the
// docket-managed pending-review surfaces and ignored paths are set aside. A
// status read error is PresenceUnknown with a diagnostic.
func primaryCleanPresence(ctx context.Context, p setupProber, repo gitcli.Repository, sc *setupContext) reposetup.Presence {
	changes, err := p.ChangedPaths(ctx, repo.PrimaryWorktree)
	if err != nil {
		sc.diagnostics = append(sc.diagnostics, setupDiag{Probe: "primary-clean", Err: err})
		return reposetup.PresenceUnknown
	}
	for _, ch := range changes {
		if isDocketManagedWorktreePath(string(ch.Path)) {
			continue
		}
		return reposetup.PresenceAbsent
	}
	return reposetup.PresencePresent
}

// isDocketManagedWorktreePath reports whether a repo-relative path is one docket
// itself manages in the working tree — the exact managed surfaces, or anything
// under the metadata worktree or the feature-worktree root.
func isDocketManagedWorktreePath(rel string) bool {
	if docketManagedWorktreePaths[rel] {
		return true
	}
	return rel == docketWorktreeName ||
		strings.HasPrefix(rel, docketWorktreeName+"/") ||
		strings.HasPrefix(rel, ".worktrees/")
}

// primaryAtTipPresence reports whether the primary worktree's HEAD is exactly the
// authoritative remote integration tip. A resolve error is PresenceUnknown with a
// diagnostic.
func primaryAtTipPresence(ctx context.Context, p setupProber, repo gitcli.Repository, rev string, sc *setupContext) reposetup.Presence {
	head, err := p.ResolveRef(ctx, repo, gitcli.RefName("HEAD"))
	if err != nil {
		sc.diagnostics = append(sc.diagnostics, setupDiag{Probe: "primary-head", Err: err})
		return reposetup.PresenceUnknown
	}
	if string(head) == rev {
		return reposetup.PresencePresent
	}
	return reposetup.PresenceAbsent
}

// docketWorktreeFact probes the .docket path: absent, a correctly registered
// owned worktree on the metadata branch, or a foreign directory / conflicting
// registration. A list error leaves the fact at its safe zero value (Unknown /
// not-foreign) with no false foreign flag.
func docketWorktreeFact(ctx context.Context, p setupProber, repo gitcli.Repository, metaRef gitcli.RefName) reposetup.WorktreeFact {
	worktreePath := filepath.Join(repo.PrimaryWorktree, docketWorktreeName)
	info, err := os.Lstat(worktreePath)
	if err != nil {
		if os.IsNotExist(err) {
			return reposetup.WorktreeFact{Presence: reposetup.PresenceAbsent}
		}
		// An unreadable path is not proven foreign and not proven absent.
		return reposetup.WorktreeFact{Presence: reposetup.PresenceUnknown}
	}
	fact := reposetup.WorktreeFact{Presence: reposetup.PresencePresent}
	if !info.IsDir() {
		fact.Foreign = true
		return fact
	}
	wts, err := p.ListWorktrees(ctx, repo)
	if err != nil {
		// Present but unprobeable registration: safe zero (not proven registered,
		// not proven foreign).
		return fact
	}
	for _, wt := range wts {
		if filepath.Clean(wt.Path) != filepath.Clean(worktreePath) {
			continue
		}
		if wt.Branch == metaRef {
			fact.Registered = reposetup.PresencePresent
			return fact
		}
		// Registered, but to a different branch: a conflicting registration.
		fact.Foreign = true
		return fact
	}
	// A .docket directory git does not know as a worktree of this repo is foreign.
	fact.Foreign = true
	return fact
}
