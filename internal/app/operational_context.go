package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/reposetup"
)

// This file is the shared operational-repository loader (change 0363): the one
// ordered read every ordinary repository-aware command reaches through
// StatusReader.PinContext. It is an EXTRACTION of the former PinContext body —
// discovery, config resolution from the pinned repository blob, and the
// fetch-and-pin of every branch the read needs — augmented with change 0352's
// repository classifier: the loader gathers the classifier facts through the
// same probe set the repository command family uses (gatherRepoFacts — one set
// of Git probes, never two) and classifies ONCE. A `legacy` classification is
// the one refusal the gate adds: the extraction preserves the existing
// single-pin contract, so every non-legacy state PinContext accepted before
// the gate existed is still admitted, and repository check/init/migrate keep
// their own classifier sub-gates below this one (they never route through the
// ordinary-command refusal).

// ReasonLegacyRepository is the stable machine reason an ordinary command
// refused by the operational gate reports. It is the classifier's own
// `legacy-repository` finding code, never a command-specific spelling.
const ReasonLegacyRepository = "legacy-repository"

// errRepositoryNotOperational is the typed refusal the operational gate
// returns for a repository state ordinary commands must not operate on. It
// carries the classifier's verdict and the exact health findings `repository
// check` would report for the same facts, so every rendering of the refusal is
// the classifier's value.
type errRepositoryNotOperational struct {
	State          reposetup.State
	Classification reposetup.Classification
	Findings       []reposetup.Finding
}

// Error renders the state and — when present — the first finding's message and
// remedy, so a command that only surfaces error text still names the exact
// human remedy for the exact reported state.
func (e *errRepositoryNotOperational) Error() string {
	msg := fmt.Sprintf("repository is not operational (state %q)", e.State)
	if len(e.Findings) > 0 {
		f := e.Findings[0]
		msg += ": " + f.Message
		if f.Remedy != "" {
			msg += " " + f.Remedy
		}
	}
	return msg
}

// operationalRefusal is the gate's pure predicate: it refuses EXACTLY the
// `legacy` classification — the removed main-mode topology, whose only exit is
// `docket repository migrate` — and admits every other state the single-pin
// contract already resolved (fresh, partial, needs-review, healthy, and the
// conflict/unknown dispositions change 0352 owns). An unknown or conflicting
// classification keeps 0352's own state and findings and never collapses into
// the legacy remedy.
func operationalRefusal(cls reposetup.Classification, f reposetup.Facts) error {
	if cls.State != reposetup.StateLegacy {
		return nil
	}
	return &errRepositoryNotOperational{
		State:          cls.State,
		Classification: cls,
		Findings:       reposetup.EvaluateHealth(cls, f, nil),
	}
}

// operationalContext is everything the shared loader pinned and proved: the
// discovered repository, the pinned branch identities and revisions, the
// resolved configuration snapshot with its diagnostics, the repository web
// identity, and the classifier's verdict.
type operationalContext struct {
	repo                gitcli.Repository
	defaultBranch       string
	defaultRevision     string
	integrationBranch   string
	integrationRevision string
	metadataRevision    string // pinned tip of the fixed docket metadata branch
	repoWebURL          string
	snapshot            config.Snapshot
	diags               []config.Diagnostic
	classification      reposetup.Classification
}

// loadOperationalContext performs the spec's one ordered read: discover the
// canonical repository and resolve the remote default branch → resolve
// configuration from the pinned repository blob plus the machine layers (the
// obsolete tombstone is diagnostic-only and cannot influence the result; an
// invalid configuration fails closed here, BEFORE any topology classification)
// → fetch and pin the integration revision → probe the classifier facts and
// classify ONCE → refuse a legacy repository with the typed refusal → fetch
// and pin the fixed remote docket revision. The whole read is pinned against
// exact commit ids so a later concurrent fetch cannot change what any concern
// observes.
func loadOperationalContext(ctx context.Context, client *gitcli.Client, repoDir string) (operationalContext, error) {
	var oc operationalContext

	dir := repoDir
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return oc, fmt.Errorf("%w: cannot resolve current directory: %v", ErrStatusInvalidInput, err)
		}
		dir = wd
	}

	repo, err := client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: dir})
	if err != nil {
		return oc, classifyGitFailure(err)
	}
	oc.repo = repo

	ref, err := client.RemoteDefaultBranch(ctx, repo, originRemote)
	if err != nil {
		return oc, classifyGitFailure(err)
	}
	defaultBranch, ok := shortBranch(ref)
	if !ok {
		return oc, fmt.Errorf("%w: remote default ref %q is not a branch", ErrStatusExternal, ref)
	}
	oc.defaultBranch = defaultBranch

	defaultRev, err := fetchPinnedRevision(ctx, client, repo, ref)
	if err != nil {
		return oc, err
	}
	oc.defaultRevision = defaultRev

	// 0341: derive the repository web URL once per pin, from origin's raw
	// configured URL. Bash-renderer parity: an unreadable URL degrades to
	// bare-path links, never a failed pin.
	remoteURL, uerr := client.RemoteURL(ctx, repo, originRemote)
	if uerr != nil {
		remoteURL = ""
	}
	oc.repoWebURL = githubWebURL(remoteURL)

	// Configuration is read from the pinned default-branch source (never the
	// working tree), then layered under the filesystem-only machine layers.
	docketYML, err := readPinnedOptionalBlob(ctx, client, repo, defaultRev, ".docket.yml")
	if err != nil {
		return oc, err
	}
	sources, err := operationalConfigSources(repo, docketYML)
	if err != nil {
		return oc, err
	}
	snap, diags, err := config.Resolve(sources, config.ResolveContext{DefaultBranch: defaultBranch})
	if err != nil {
		// A resolution error blocks topology (invalid config, or an unresolvable
		// auto integration branch) — an invalid-input condition, reported BEFORE
		// classification so a broken configuration never masks as (or collapses
		// into) a topology remedy.
		return oc, fmt.Errorf("%w: %v", ErrStatusInvalidInput, err)
	}
	oc.snapshot = *snap
	oc.diags = diags
	eff := snap.Effective

	oc.integrationBranch = eff.IntegrationBranch.Value
	oc.integrationRevision = defaultRev
	if oc.integrationBranch != defaultBranch {
		oc.integrationRevision, err = fetchPinnedRevision(ctx, client, repo, gitcli.RefName(branchRefPrefix+oc.integrationBranch))
		if err != nil {
			return oc, err
		}
	}

	// Probe the classifier facts through the repository command family's own
	// probe set and classify once. The already-pinned revisions are handed in
	// so the shared probe performs no second fetch of them.
	facts, _ := gatherRepoFacts(ctx, client, repoFactsInput{
		repo:           repo,
		defaultBranch:  defaultBranch,
		defaultTip:     defaultRev,
		integrationTip: oc.integrationRevision,
		cfg:            eff,
	})
	oc.classification = reposetup.Classify(facts)
	if rerr := operationalRefusal(oc.classification, facts); rerr != nil {
		return oc, rerr
	}

	// Pin the fixed metadata branch. For any admitted state without a remote
	// docket branch (fresh, or an unproven probe), the fetch fails exactly the
	// way the pre-gate PinContext failed — the gate adds no acceptance the
	// single-pin contract did not already have.
	oc.metadataRevision, err = fetchPinnedRevision(ctx, client, repo, gitcli.RefName(branchRefPrefix+reposetup.MetadataBranchName))
	if err != nil {
		return oc, err
	}
	return oc, nil
}

// fetchPinnedRevision fetches one fully-qualified branch through origin and
// returns its pinned commit id, mapping any adapter failure to a status
// classification.
func fetchPinnedRevision(ctx context.Context, client *gitcli.Client, repo gitcli.Repository, branch gitcli.RefName) (string, error) {
	rev, err := client.FetchBranch(ctx, repo, originRemote, branch)
	if err != nil {
		return "", classifyGitFailure(err)
	}
	return string(rev.Commit), nil
}

// readPinnedOptionalBlob reads one path from the pinned revision, returning a
// nil byte slice and no error when the path is absent — the "absent layer"
// case for the repository configuration blob.
func readPinnedOptionalBlob(ctx context.Context, client *gitcli.Client, repo gitcli.Repository, rev, p string) ([]byte, error) {
	src, err := client.OpenObjectSource(ctx, repo, gitcli.Revision{Commit: gitcli.ObjectID(rev)})
	if err != nil {
		return nil, classifyGitFailure(err)
	}
	results, err := src.ReadBlobs(ctx, []gitcli.RepoPath{gitcli.RepoPath(p)})
	if err != nil {
		return nil, classifyGitFailure(err)
	}
	if !results[0].Found {
		return nil, nil
	}
	return results[0].Blob.Bytes, nil
}

// operationalConfigSources assembles the resolution layer stack in the required
// low→high order: the filesystem global layer, the repository layer read from
// the pinned Git blob (never the working tree), and the filesystem
// repository-local layer read beside the discovered primary worktree.
// LoadFilesystemSources is not called wholesale because its repository
// .docket.yml read would come from the working tree, which the
// read-from-pinned-Git contract forbids.
func operationalConfigSources(repo gitcli.Repository, docketYML []byte) ([]config.Source, error) {
	var sources []config.Source

	globalSources, err := config.LoadGlobalSource("")
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrStatusExternal, err)
	}
	sources = append(sources, globalSources...)

	if docketYML != nil {
		sources = append(sources, config.Source{Layer: config.LayerRepository, Name: ".docket.yml", Data: docketYML})
	}

	localPath := filepath.Join(repo.PrimaryWorktree, ".docket.local.yml")
	data, err := os.ReadFile(localPath)
	switch {
	case err == nil:
		sources = append(sources, config.Source{Layer: config.LayerRepositoryLocal, Name: ".docket.local.yml", Data: data})
	case os.IsNotExist(err):
		// repository-local layer absent
	default:
		return nil, fmt.Errorf("%w: reading %s: %v", ErrStatusExternal, ".docket.local.yml", err)
	}
	return sources, nil
}
