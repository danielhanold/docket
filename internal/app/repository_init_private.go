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
// place of the default bare repository. Harnesses carries --harnesses and the
// CLI's interactive chooser.
type InitOptions struct {
	Private, Shared bool
	MetadataRemote  string
	Harnesses       HarnessesOptions
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
// writes nothing into the working tree, so `git status` stays clean. choice is
// the harness choice RunRepositoryInit resolved before any write; step 8 writes
// it to the private config.
func runPrivateInit(ctx context.Context, d SetupDeps, sc setupContext, cls reposetup.Classification, o InitOptions, debris setupDebrisReport, choice harnessChoice) RepositoryOpResult {
	common := sc.repo.CommonDir
	changed := false

	// 0. A .docket.local.yml beside the private config refuses before any write:
	// once .git/dckt/ exists every operation, an init re-run included, reads the
	// repository as private and refuses on that conflict, so writing first would
	// wedge it. The refusal is the same one config.LoadPrivateRepositorySource
	// gives every private operation.
	if _, err := config.LoadPrivateRepositorySource(common, sc.repo.PrimaryWorktree); err != nil {
		var conflict *config.ConflictingLocalConfigError
		if errors.As(err, &conflict) {
			out := newRepositoryOpResult(OperationRepositoryInit, ResultUnsupportedConfig, RepositoryOpResult{
				RepositoryState: string(cls.State),
			})
			out.human = fmt.Sprintf("%s: %s: %s", OperationRepositoryInit, ResultUnsupportedConfig, err.Error())
			return out
		}
		return repositoryExternalFailure(OperationRepositoryInit, cls.State, "inspecting the repository config", err)
	}

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

	// 4-5. The bare remote and the dckt git remote. A repository that already
	// detected private before this run reuses the dckt remote it configured (a
	// flagless re-run after --metadata-remote).
	want, remoteChanged, err := ensurePrivateMetadataRemote(ctx, d.Git, sc.repo, lay, o.MetadataRemote, sc.layout.Mode == layout.Private)
	if err != nil {
		var mre *metadataRemoteError
		if !errors.As(err, &mre) {
			return fail(repositoryInternalFailure(OperationRepositoryInit, cls.State, "setting up the dckt git remote", err))
		}
		if mre.Conflict {
			return fail(initRefusal(reposetup.StateConflict, mre.Err.Error()))
		}
		return fail(repositoryExternalFailure(OperationRepositoryInit, cls.State, mre.Stage, mre.Err))
	}
	changed = changed || remoteChanged
	remote := metadataRemote(lay)

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

	// 7. This clone's checkout under the store, hooks off. A stale checkout at
	// this clone's path (a same-path re-clone's leftover) is replaced first.
	if err := os.MkdirAll(lay.CheckoutsDir, 0o755); err != nil {
		return fail(repositoryExternalFailure(OperationRepositoryInit, cls.State, "creating the checkouts folder", err))
	}
	if err := removeStaleOwnCheckout(lay); err != nil {
		return fail(repositoryExternalFailure(OperationRepositoryInit, cls.State, "replacing the stale metadata checkout", err))
	}
	createdWorktree, err := ensureMetadataWorktree(ctx, d.Git, sc.repo, lay.MetadataWorktree, metaRef, tip)
	if err != nil {
		return fail(repositoryExternalFailure(OperationRepositoryInit, cls.State, "attaching the metadata checkout", err))
	}
	changed = changed || createdWorktree
	if err := d.Git.DisableWorktreeHooks(ctx, lay.MetadataWorktree); err != nil {
		return fail(repositoryExternalFailure(OperationRepositoryInit, cls.State, "disabling metadata-checkout hooks", err))
	}

	// 8. The chosen agent_harnesses goes into the private config, then the
	// parent-facing instructions: when the private config's agent_harnesses
	// authorizes them, the dispatch block goes into .git/dckt/AGENTS.md through
	// the installer's repository phase (which authorizes itself from the config
	// re-read after this write). Private init edits no .gitignore and writes
	// nothing in the working tree.
	//
	// The layout resolved in step 3: a fresh repository's gather-time layout is
	// still shared and would point the write at .docket.yml.
	psc := sc
	psc.layout = lay
	applied, herr := applyHarnessChoice(ctx, d.Git, psc, choice)
	if herr != nil {
		return fail(harnessApplyFailure(OperationRepositoryInit, cls.State, herr))
	}
	changed = changed || applied.wroteConfig || applied.wroteSurfaces

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
	setHarnessResult(&out, choice, applied)
	return out
}

// metadataRemoteError is ensurePrivateMetadataRemote's failure. Conflict marks
// a refusal a human resolves (a foreign default store, or a dckt remote that
// points elsewhere); otherwise it is an external failure at Stage.
type metadataRemoteError struct {
	Conflict bool
	Stage    string
	Err      error
}

func (e *metadataRemoteError) Error() string {
	if e.Conflict {
		return e.Err.Error()
	}
	return e.Stage + ": " + e.Err.Error()
}

func (e *metadataRemoteError) Unwrap() error { return e.Err }

// ensurePrivateMetadataRemote sets up the bare remote a private repository's
// metadata branch is pushed to, and the dckt git remote naming it. The URL is
// flagURL as given; else, when reuseConfigured, the dckt remote already
// configured; else the default store, created when absent. Only that last case
// ever creates the default store. The default store's name is derived from
// origin's URL and can collide across distinct origins, so the store records
// the origin that created it and a repository with a different origin refuses
// before attaching to it. The dckt git remote is added when absent, kept when
// it already points at the URL, and refused otherwise: docket never rewrites a
// remote. It returns the URL and whether it changed anything; every failure is
// a *metadataRemoteError.
func ensurePrivateMetadataRemote(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, lay layout.Layout, flagURL string, reuseConfigured bool) (string, bool, error) {
	external := func(stage string, err error) (string, bool, error) {
		return "", false, &metadataRemoteError{Stage: stage, Err: err}
	}
	conflict := func(msg string) (string, bool, error) {
		return "", false, &metadataRemoteError{Conflict: true, Err: errors.New(msg)}
	}
	changed := false
	remote := metadataRemote(lay)
	got, err := git.RemoteURL(ctx, repo, remote)
	unconfigured := isRemoteUnconfigured(err)
	if err != nil && !unconfigured {
		return external("reading the dckt git remote", err)
	}
	want, err := absMetadataRemote(flagURL)
	if err != nil {
		return external("resolving the --metadata-remote path", err)
	}
	if want == "" && !unconfigured && reuseConfigured {
		want = got
	}
	createStore := want == ""
	if createStore {
		want = lay.DefaultBareRemote
	}
	var recordOrigin func() (bool, error)
	if want == lay.DefaultBareRemote {
		origin, oerr := git.RemoteURL(ctx, repo, originRemote)
		if oerr != nil {
			return external("reading origin's URL", oerr)
		}
		record, rerr := checkStoreOrigin(lay.StoreDir, origin)
		if rerr != nil {
			return external("reading the metadata store's origin record", rerr)
		}
		if record.foreign != "" {
			return conflict(fmt.Sprintf(
				"the metadata store %s belongs to origin %s, not this repository's origin %s (both derive the store name %s); re-run `docket repository init --metadata-remote <url>` with a bare repository of this repository's own",
				lay.StoreDir, record.foreign, origin, filepath.Base(lay.StoreDir)))
		}
		recordOrigin = record.write
	}
	if createStore {
		_, statErr := os.Stat(want)
		if err := git.InitBare(ctx, want); err != nil {
			return external("creating the bare metadata store", err)
		}
		changed = changed || os.IsNotExist(statErr)
	}
	if recordOrigin != nil {
		wrote, werr := recordOrigin()
		if werr != nil {
			return external("recording the metadata store's origin", werr)
		}
		changed = changed || wrote
	}

	switch {
	case unconfigured:
		if aerr := git.AddRemote(ctx, repo, remote, want); aerr != nil {
			return external("adding the dckt git remote", aerr)
		}
		changed = true
	case got != want:
		return conflict(fmt.Sprintf(
			"the dckt git remote already points at %s, not %s; resolve it by hand (docket never rewrites a remote)", got, want))
	}
	return want, changed, nil
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

// storeOriginRecordName is the file beside the default store's remote.git that
// records the origin URL of the repository that created the store.
const storeOriginRecordName = "origin-url"

// storeOriginCheck is checkStoreOrigin's verdict: foreign names the recorded
// origin when it is not this repository's, and write records this repository's
// origin when the store has no record yet.
type storeOriginCheck struct {
	foreign string
	write   func() (bool, error)
}

// checkStoreOrigin reads the default store's origin record under storeDir. A
// record naming a different origin is foreign; a matching one needs no write; an
// absent one (a new store, or one created before the record existed) is adopted
// and its write records origin once the store directory exists.
func checkStoreOrigin(storeDir, origin string) (storeOriginCheck, error) {
	path := filepath.Join(storeDir, storeOriginRecordName)
	b, err := os.ReadFile(path)
	switch {
	case err == nil:
		if recorded := strings.TrimSpace(string(b)); !sameOrigin(recorded, origin) {
			return storeOriginCheck{foreign: recorded}, nil
		}
		return storeOriginCheck{write: func() (bool, error) { return false, nil }}, nil
	case !os.IsNotExist(err):
		return storeOriginCheck{}, err
	}
	return storeOriginCheck{write: func() (bool, error) {
		if _, serr := os.Stat(storeDir); serr != nil {
			if os.IsNotExist(serr) {
				return false, nil // no store to record (a flagless re-run whose store is gone)
			}
			return false, serr
		}
		return true, os.WriteFile(path, []byte(strings.TrimSpace(origin)+"\n"), 0o644)
	}}, nil
}

// sameOrigin reports whether two origin URLs name one repository: the
// transport (scheme), the user, and a trailing "/" or ".git" do not matter;
// host and path, case included, do.
func sameOrigin(a, b string) bool { return originIdentity(a) == originIdentity(b) }

// originIdentity reduces an origin URL to host/path ("github.com/acme/app"), or
// to the cleaned path for a local path.
func originIdentity(u string) string {
	s := strings.TrimSuffix(strings.TrimRight(strings.TrimSpace(u), "/"), ".git")
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	} else if colon := strings.IndexByte(s, ':'); colon > 0 && !strings.ContainsAny(s[:colon], `/\`) {
		s = s[:colon] + "/" + s[colon+1:] // scp-like host:path
	} else {
		return s
	}
	if slash := strings.IndexByte(s, '/'); slash >= 0 {
		if at := strings.LastIndexByte(s[:slash], '@'); at >= 0 {
			s = s[at+1:]
		}
	}
	return s
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

// absMetadataRemote makes a local-path --metadata-remote absolute and clean:
// metadata git operations run from a checkout outside the clone, so a relative
// path stored as typed would resolve against the wrong directory. A URL
// ("scheme://...") or an scp-style "host:path" (a colon before any slash, as
// git reads it) is returned unchanged, as is the empty value.
func absMetadataRemote(v string) (string, error) {
	if v == "" || strings.Contains(v, "://") {
		return v, nil
	}
	if c := strings.Index(v, ":"); c >= 0 && !strings.Contains(v[:c], "/") {
		return v, nil
	}
	return filepath.Abs(v)
}
