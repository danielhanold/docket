package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/layout"
	"github.com/danielhanold/docket/internal/reposetup"
)

// This file is `docket repository init`'s private mode: the metadata branch
// `dckt` published to a bare `dckt` remote on this machine, its checkout under
// that store, and the repository's config in .git/dckt/config.yml — nothing
// docket-named in the repository and nothing pushed to origin.

// InitOptions carries init's mode flags. Private and Shared override the
// effective `visibility` value for a fresh repository; MetadataRemote, valid
// only for a private result, names the git URL the dckt branch is pushed to in
// place of the default bare repository.
type InitOptions struct {
	Private, Shared bool
	MetadataRemote  string
}

// privateInitRecovery is appended to every private-init failure after the
// config write: from that point .git/dckt/ exists, so the repository reads as
// private, and the state itself names its recovery (presence-encoded state).
const privateInitRecovery = "`.git/dckt/` now exists, so this repository reads as private; re-run `docket repository init` to finish (every step is idempotent)."

// decideInitMode decides the mode init sets a repository up in, or a refusal
// ("" = admitted). Config never moves a repository: a detected private
// repository stays private and an already-set-up shared one stays shared, and a
// flag that would switch either refuses. Only a fresh repository takes the
// effective visibility value, overridden by a flag.
func decideInitMode(detected layout.Mode, state reposetup.State, o InitOptions, effective string) (layout.Mode, string) {
	if o.Private && o.Shared {
		return "", "--private and --shared are mutually exclusive"
	}
	if detected == layout.Private {
		if o.Shared {
			return "", "this repository is already private; init never switches a repository's visibility"
		}
		return layout.Private, ""
	}
	if state != reposetup.StateFresh {
		if o.Private || o.MetadataRemote != "" {
			return "", "this repository is already set up as shared; init never switches a repository's visibility"
		}
		return layout.Shared, ""
	}
	mode := layout.Shared
	if effective == string(layout.Private) {
		mode = layout.Private
	}
	switch {
	case o.Private:
		mode = layout.Private
	case o.Shared:
		mode = layout.Shared
	}
	if mode != layout.Private && o.MetadataRemote != "" {
		return "", "--metadata-remote applies only to a private repository"
	}
	return mode, ""
}

// repoConfigTarget names where a generated repository config edit lands: a
// private repository's .git/dckt/config.yml (clone-local, never a pending
// review path), or a shared repository's root .docket.yml (a pending, unstaged
// review path).
func repoConfigTarget(sc setupContext) (absPath, display string, pending bool) {
	if sc.layout.Mode == layout.Private {
		return sc.layout.ConfigPath, layout.PrivateConfigDisplay, false
	}
	return filepath.Join(sc.repo.PrimaryWorktree, docketYMLRel), docketYMLRel, true
}

// recoverUnconfiguredPrivateRemote turns a private repository's unprobeable
// metadata presence into a proven absence when the reason is that the dckt
// remote is not configured yet — the window a private init that died after its
// config write leaves. Init's create-only publish then creates the branch or
// adopts the one already in the store, so a re-run finishes. Any other probe
// failure stays Unknown.
func recoverUnconfiguredPrivateRemote(ctx context.Context, git *gitcli.Client, facts *reposetup.Facts, sc setupContext) {
	if sc.layout.Mode != layout.Private || facts.RemoteMetadata.Presence != reposetup.PresenceUnknown {
		return
	}
	if _, err := git.RemoteURL(ctx, sc.repo, metadataRemote(sc.layout)); isRemoteUnconfigured(err) {
		facts.RemoteMetadata.Presence = reposetup.PresenceAbsent
	}
}

// isRemoteUnconfigured reports whether err is gitcli's "remote is not
// configured" failure.
func isRemoteUnconfigured(err error) bool {
	fail, ok := gitcli.AsFailure(err)
	return ok && fail.Kind == gitcli.KindRemoteUnavailable
}

// runPrivateInit sets a repository up private. Every step is idempotent and
// writes nothing into the working tree, so `git status` stays clean.
func runPrivateInit(ctx context.Context, d SetupDeps, sc setupContext, cls reposetup.Classification, o InitOptions, debris setupDebrisReport) RepositoryOpResult {
	common := sc.repo.CommonDir
	changed := false

	// 1. The neutral ignore block in the clone-local exclude file. A malformed
	// block refuses with the file untouched.
	wroteExclude, err := ensureExcludeFile(filepath.Join(common, "info", "exclude"))
	if err != nil {
		var mal *reposetup.MalformedExcludeError
		if errors.As(err, &mal) {
			return initRefusal(cls.State, err.Error()+"; fix the markers by hand, then re-run")
		}
		return repositoryExternalFailure(OperationRepositoryInit, cls.State, "writing .git/info/exclude", err)
	}
	changed = changed || wroteExclude

	// 2. The private config. From here the repository detects as private.
	wroteConfig, err := ensurePrivateConfig(layout.PrivateConfigPath(common), sc.repo.PrimaryWorktree, sc.cfg)
	if err != nil {
		return repositoryInternalFailure(OperationRepositoryInit, cls.State, "writing "+layout.PrivateConfigDisplay, err)
	}
	changed = changed || wroteConfig

	fail := func(r RepositoryOpResult) RepositoryOpResult {
		r.human += "\n" + privateInitRecovery
		return r
	}

	// 3. The layout, now resolved from state.
	lay, err := resolveLayout(ctx, d.Git, sc.repo)
	if err != nil {
		return fail(repositoryExternalFailure(OperationRepositoryInit, cls.State, "resolving the private layout", err))
	}
	if lay.Mode != layout.Private {
		return fail(repositoryInternalFailure(OperationRepositoryInit, cls.State, "resolving the private layout", errors.New("the repository does not detect as private after the config write")))
	}

	// 4. The bare remote: the flag URL as given, or the default store, created
	// when absent. A flag URL never creates the default store.
	want := o.MetadataRemote
	if want == "" {
		want = lay.DefaultBareRemote
		_, statErr := os.Stat(want)
		if err := d.Git.InitBare(ctx, want); err != nil {
			return fail(repositoryExternalFailure(OperationRepositoryInit, cls.State, "creating the bare metadata store", err))
		}
		changed = changed || os.IsNotExist(statErr)
	}

	// 5. The dckt git remote: added when absent, kept when it already points at
	// want, refused otherwise. Docket never rewrites a remote.
	remote := metadataRemote(lay)
	got, err := d.Git.RemoteURL(ctx, sc.repo, remote)
	switch {
	case isRemoteUnconfigured(err):
		if aerr := d.Git.AddRemote(ctx, sc.repo, remote, want); aerr != nil {
			return fail(repositoryExternalFailure(OperationRepositoryInit, cls.State, "adding the dckt git remote", aerr))
		}
		changed = true
	case err != nil:
		return fail(repositoryExternalFailure(OperationRepositoryInit, cls.State, "reading the dckt git remote", err))
	case got != want:
		return fail(initRefusal(reposetup.StateConflict, fmt.Sprintf(
			"the dckt git remote already points at %s, not %s; resolve it by hand (docket never rewrites a remote)", got, want)))
	}

	// 6. The metadata branch: a branch the store already holds is adopted at its
	// tip when it is a verified init lineage (a re-run, or the second-clone path)
	// and never republished; an absent one is created create-only. Probing first
	// matters: a re-run inside the same second rebuilds a byte-identical root,
	// and the create-only push reports that up-to-date ref as a success, so
	// publishing would claim a branch this run did not create.
	metaRef := metadataRef(lay)
	published, err := d.Git.ProbeRemoteBranch(ctx, sc.repo, remote, metaRef)
	if err != nil {
		return fail(repositoryExternalFailure(OperationRepositoryInit, cls.State, "reading the dckt metadata branch", err))
	}
	var tip gitcli.ObjectID
	if published.State == gitcli.RemoteRefFound {
		adopted, refusal := adoptPublishedMetadataRoot(ctx, d.Git, sc.repo, remote, metaRef, published.Commit, sc.sourceRevision, sc.defaultBranch)
		if refusal != nil {
			return fail(*refusal)
		}
		tip = adopted
	} else {
		created, createdBranch, refusal := publishOrAdoptMetadataRoot(ctx, d.Git, sc.repo, remote, metaRef, sc.sourceRevision, sc.defaultBranch)
		if refusal != nil {
			return fail(*refusal)
		}
		tip = created
		changed = changed || createdBranch
	}

	// 7. This clone's checkout under the store, hooks off.
	if err := os.MkdirAll(lay.CheckoutsDir, 0o755); err != nil {
		return fail(repositoryExternalFailure(OperationRepositoryInit, cls.State, "creating the checkouts folder", err))
	}
	createdWorktree, err := ensureMetadataWorktree(ctx, d.Git, sc.repo, lay.MetadataWorktree, metaRef, tip)
	if err != nil {
		return fail(repositoryExternalFailure(OperationRepositoryInit, cls.State, "attaching the metadata checkout", err))
	}
	changed = changed || createdWorktree
	if err := d.Git.DisableWorktreeHooks(ctx, lay.MetadataWorktree); err != nil {
		return fail(repositoryExternalFailure(OperationRepositoryInit, cls.State, "disabling metadata-checkout hooks", err))
	}

	// 8. No .gitignore edit and no parent-facing dispatch surfaces: both are
	// committed root files. Surfaces in a private repository belong to the
	// follow-up change that keeps docket out of commits.

	// 9. Report the state the repository now classifies in.
	facts, sc2, err := GatherSetupFacts(ctx, d, false)
	if err != nil {
		return fail(repositoryExternalFailure(OperationRepositoryInit, cls.State, "re-reading the repository", err))
	}
	if facts.RemoteMetadata.Presence == reposetup.PresencePresent {
		augmentCheckFacts(ctx, d.Git, &facts, sc2)
	}
	state := reposetup.Classify(facts).State

	result := ResultNoOp
	if changed {
		result = ResultApplied
	}
	pending := debris.pending()
	sort.Strings(pending)
	out := newRepositoryOpResult(OperationRepositoryInit, result, RepositoryOpResult{
		RepositoryState: string(state),
		PendingPaths:    pending,
		MetadataTip:     string(tip),
		SourceRevision:  sc.sourceRevision,
	})
	out.human = fmt.Sprintf("initialized private docket metadata: branch %s on %s, worktree %s",
		lay.MetadataBranch, want, lay.MetadataWorktree)
	if len(pending) > 0 {
		out.human += "\n" + strings.Join(pending, "\n")
	}
	return out
}

// ensureExcludeFile writes the neutral `# dckt:` block into the exclude file at
// path (absent reads as empty) when it is missing or stale, and reports whether
// it wrote. Malformed markers return *reposetup.MalformedExcludeError with the
// file untouched.
func ensureExcludeFile(path string) (bool, error) {
	current, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	out, changed, err := reposetup.EnsureExcludeBlock(current)
	if err != nil || !changed {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return false, err
	}
	return true, nil
}

// privateConfigSeed is the first content of a new private config file.
const privateConfigSeed = "visibility: private\n"

// ensurePrivateConfig writes the private config at path: the existing bytes, or
// privateConfigSeed for a new file, with the generated test policy applied. It
// reports whether it wrote.
func ensurePrivateConfig(path, primaryWorktree string, cfg config.Effective) (bool, error) {
	tree := newOSTree(primaryWorktree)
	return writeRepoConfig(path, func(existing []byte) ([]byte, error) {
		base := existing
		if base == nil {
			base = []byte(privateConfigSeed)
		}
		edited, _, perr := reposetup.TestPolicyEdit(cfg, base, tree)
		if perr != nil {
			return nil, perr
		}
		if edited == nil && existing == nil {
			return base, nil // a new file is written even with no test policy
		}
		return edited, nil
	})
}
