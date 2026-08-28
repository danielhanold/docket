package app

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/repository"
)

// This file is the production StatusReader: the one seam implementation that
// turns the abstract PinContext/ReadCorpus/BranchFacts/ArtifactExists concerns
// into real Git reads. It composes the landed gitcli adapter — discovery,
// targeted fetch, and immutable revision-pinned object sources — with config
// resolution; it decides no policy of its own. The one local mutation it ever
// causes is the fetch that advances a remote-tracking ref, exactly the mutation
// the spec permits; it never touches the worktree, index, HEAD, or any branch.

// originRemote is the remote docket reads authoritative state from. Discovery
// and every fetch/ls-remote go through it.
const originRemote gitcli.RemoteName = "origin"

// The two metadata-mode spellings the operation reports and keys on. A
// configuration whose metadata_branch resolves to metadataModeMain keeps
// planning records on the default branch; any other value names a distinct
// metadata branch (docket mode).
const (
	metadataModeMain   = "main"
	metadataModeDocket = "docket"
)

// branchRefPrefix is the fully-qualified local-branch namespace. A short branch
// name is fetched under it and a fully-qualified default ref is shortened by
// stripping it.
const branchRefPrefix = "refs/heads/"

// gitStatusReader is the production StatusReader over one gitcli client. It is
// constructed fresh per operation; PinContext records the discovered repository
// identity so the later concern calls — which receive only the StatusPin, never
// the invocation path — can re-open pinned sources against it. It caches no
// object CONTENT: every read re-opens the source at the pinned (immutable)
// revision the caller threads back in.
type gitStatusReader struct {
	client *gitcli.Client
	repo   gitcli.Repository
}

// NewGitStatusReader returns the production StatusReader over one gitcli client.
func NewGitStatusReader(client *gitcli.Client) StatusReader {
	return &gitStatusReader{client: client}
}

// PinContext discovers the repository, resolves origin's default branch, reads
// .docket.yml from the pinned default-branch source, resolves configuration
// over the global (filesystem) + repository (Git blob) + repository-local
// (filesystem) layers, and fetches and pins every branch the resolved metadata
// mode requires. The whole read is pinned against exact commit ids so a later
// concurrent fetch cannot change what any concern observes.
func (r *gitStatusReader) PinContext(ctx context.Context, repoDir string) (StatusPin, error) {
	dir := repoDir
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return StatusPin{}, fmt.Errorf("%w: cannot resolve current directory: %v", ErrStatusInvalidInput, err)
		}
		dir = wd
	}

	repo, err := r.client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: dir})
	if err != nil {
		return StatusPin{}, classifyGitFailure(err)
	}
	r.repo = repo

	ref, err := r.client.RemoteDefaultBranch(ctx, repo, originRemote)
	if err != nil {
		return StatusPin{}, classifyGitFailure(err)
	}
	defaultBranch, ok := shortBranch(ref)
	if !ok {
		return StatusPin{}, fmt.Errorf("%w: remote default ref %q is not a branch", ErrStatusExternal, ref)
	}

	defaultRev, err := r.fetchRevision(ctx, ref)
	if err != nil {
		return StatusPin{}, err
	}

	// 0341: derive the repository web URL once per pin, from origin's raw
	// configured URL. Bash-renderer parity: an unreadable URL degrades to
	// bare-path links, never a failed pin.
	remoteURL, uerr := r.client.RemoteURL(ctx, repo, originRemote)
	if uerr != nil {
		remoteURL = ""
	}

	// Configuration is read from the pinned default-branch source (never the
	// working tree), then layered under the filesystem-only machine layers.
	docketYML, err := r.readOptionalBlob(ctx, defaultRev, ".docket.yml")
	if err != nil {
		return StatusPin{}, err
	}
	sources, err := r.configSources(repo, docketYML)
	if err != nil {
		return StatusPin{}, err
	}
	snap, diags, err := config.Resolve(sources, config.ResolveContext{DefaultBranch: defaultBranch})
	if err != nil {
		// A resolution error blocks topology (invalid config, or an unresolvable
		// auto integration branch) — an invalid-input condition.
		return StatusPin{}, fmt.Errorf("%w: %v", ErrStatusInvalidInput, err)
	}
	eff := snap.Effective

	integrationBranch := eff.IntegrationBranch.Value
	integrationRev := defaultRev
	if integrationBranch != defaultBranch {
		integrationRev, err = r.fetchRevision(ctx, gitcli.RefName(branchRefPrefix+integrationBranch))
		if err != nil {
			return StatusPin{}, err
		}
	}

	pin := StatusPin{
		DefaultBranch:       defaultBranch,
		DefaultRevision:     defaultRev,
		IntegrationBranch:   integrationBranch,
		IntegrationRevision: integrationRev,
		Config:              *snap,
		ConfigDiags:         diags,
		RepoWebURL:          githubWebURL(remoteURL),
	}

	// 0363 Task 4 removes this: config.Effective.MetadataBranch is gone (obsolete
	// tombstone), so the metadata branch is fixed at "docket". Task 4 deletes
	// StatusPin.Mode/MetadataBranch and the metadataModeMain selector entirely and
	// sources the fixed branch from reposetup.MetadataBranchName.
	metadataBranchBridge := "docket" // 0363 Task 4 removes this
	if metadataBranchBridge == metadataModeMain {
		pin.Mode = metadataModeMain
	} else {
		pin.Mode = metadataModeDocket
		metaBranch := metadataBranchBridge
		pin.MetadataBranch = metaBranch
		switch metaBranch {
		case defaultBranch:
			pin.MetadataRevision = defaultRev
		case integrationBranch:
			pin.MetadataRevision = integrationRev
		default:
			metaRev, err := r.fetchRevision(ctx, gitcli.RefName(branchRefPrefix+metaBranch))
			if err != nil {
				return StatusPin{}, err
			}
			pin.MetadataRevision = metaRev
		}
	}
	return pin, nil
}

// ReadCorpus lists and reads every configured record from the pinned metadata
// source: active and archived changes, ADRs, and — when enabled — learnings.
// Classification is by directory prefix; derived index views (README/LEARNINGS)
// are not records and are skipped. Every returned blob carries the exact bytes
// and object id of the pinned revision.
func (r *gitStatusReader) ReadCorpus(ctx context.Context, pin StatusPin) ([]StatusBlob, error) {
	eff := pin.Config.Effective
	src, err := r.openSource(ctx, metadataRevision(pin))
	if err != nil {
		return nil, classifyGitFailure(err)
	}
	entries, err := src.ListTree(ctx, corpusPrefixes(eff))
	if err != nil {
		return nil, classifyGitFailure(err)
	}

	type classified struct {
		kind     repository.RecordKind
		location repository.RecordLocation
	}
	var paths []gitcli.RepoPath
	var meta []classified
	for _, e := range entries {
		if e.Type != "blob" {
			continue
		}
		kind, loc, ok := classifyCorpusPath(eff, string(e.Path))
		if !ok {
			continue
		}
		paths = append(paths, e.Path)
		meta = append(meta, classified{kind: kind, location: loc})
	}
	if len(paths) == 0 {
		return []StatusBlob{}, nil
	}

	blobs, err := src.ReadBlobs(ctx, paths)
	if err != nil {
		return nil, classifyGitFailure(err)
	}
	// ReadBlobs returns results in request order, so meta[i] aligns with blobs[i].
	out := make([]StatusBlob, 0, len(blobs))
	for i, br := range blobs {
		if !br.Found {
			// A path just listed cannot vanish from the same pinned commit; treat a
			// gap defensively as nothing to record rather than a phantom blob.
			continue
		}
		out = append(out, StatusBlob{
			Kind:     meta[i].kind,
			Location: meta[i].location,
			Path:     string(br.Path),
			Version:  string(br.Blob.ObjectID),
			Data:     br.Blob.Bytes,
		})
	}
	return out, nil
}

// BranchFacts fetches each distinct feature branch and records whether it
// exists on the remote. A fetch that classifies as "no such remote branch" is a
// clean absent (false); any other failure propagates as an external error.
func (r *gitStatusReader) BranchFacts(ctx context.Context, pin StatusPin, branches []string) (domain.BranchFacts, error) {
	remote := make(map[string]bool, len(branches))
	for _, b := range branches {
		if b == "" {
			continue
		}
		_, err := r.client.FetchBranch(ctx, r.repo, originRemote, gitcli.RefName(branchRefPrefix+b))
		if err == nil {
			remote[b] = true
			continue
		}
		if f, ok := gitcli.AsFailure(err); ok {
			switch f.Kind {
			case gitcli.KindRefUnavailable:
				remote[b] = false
				continue
			case gitcli.KindCancelled:
				return domain.BranchFacts{}, err
			default:
				return domain.BranchFacts{}, fmt.Errorf("%w: %s", ErrStatusExternal, f.Error())
			}
		}
		return domain.BranchFacts{}, err
	}
	return domain.NewBranchFacts(remote), nil
}

// ArtifactExists reports whether a repo-relative path exists on the named
// pinned source: "metadata" for specs, "integration" for plans and results. An
// absent path is a clean (false, nil).
func (r *gitStatusReader) ArtifactExists(ctx context.Context, pin StatusPin, source, artifactPath string) (bool, error) {
	rev, err := sourceRevision(pin, source)
	if err != nil {
		return false, err
	}
	src, err := r.openSource(ctx, rev)
	if err != nil {
		return false, classifyGitFailure(err)
	}
	results, err := src.ReadBlobs(ctx, []gitcli.RepoPath{gitcli.RepoPath(artifactPath)})
	if err != nil {
		return false, classifyGitFailure(err)
	}
	return results[0].Found, nil
}

// ReadArtifact reads a repo-relative path from the named pinned source,
// returning its exact bytes and blob object id. An absent path is a clean
// (Found=false, nil) — the same benign absence ArtifactExists reports.
func (r *gitStatusReader) ReadArtifact(ctx context.Context, pin StatusPin, source, artifactPath string) (StatusArtifact, error) {
	rev, err := sourceRevision(pin, source)
	if err != nil {
		return StatusArtifact{}, err
	}
	src, err := r.openSource(ctx, rev)
	if err != nil {
		return StatusArtifact{}, classifyGitFailure(err)
	}
	results, err := src.ReadBlobs(ctx, []gitcli.RepoPath{gitcli.RepoPath(artifactPath)})
	if err != nil {
		return StatusArtifact{}, classifyGitFailure(err)
	}
	br := results[0]
	if !br.Found {
		return StatusArtifact{Found: false}, nil
	}
	return StatusArtifact{Found: true, Version: string(br.Blob.ObjectID), Data: br.Blob.Bytes}, nil
}

// fetchRevision fetches one fully-qualified branch through origin and returns
// its pinned commit id as a string, mapping any adapter failure to a status
// classification.
func (r *gitStatusReader) fetchRevision(ctx context.Context, branch gitcli.RefName) (string, error) {
	rev, err := r.client.FetchBranch(ctx, r.repo, originRemote, branch)
	if err != nil {
		return "", classifyGitFailure(err)
	}
	return string(rev.Commit), nil
}

// openSource opens the immutable object source pinned at rev.
func (r *gitStatusReader) openSource(ctx context.Context, rev string) (gitcli.ObjectSource, error) {
	return r.client.OpenObjectSource(ctx, r.repo, gitcli.Revision{Commit: gitcli.ObjectID(rev)})
}

// readOptionalBlob reads one path from a pinned source, returning a nil byte
// slice and no error when the path is absent — the "absent layer" case for the
// repository configuration blob.
func (r *gitStatusReader) readOptionalBlob(ctx context.Context, rev, p string) ([]byte, error) {
	src, err := r.openSource(ctx, rev)
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

// configSources assembles the resolution layer stack in the required low→high
// order: the filesystem global layer, the repository layer read from the pinned
// Git blob (never the working tree), and the filesystem repository-local layer
// read beside the discovered primary worktree. LoadFilesystemSources is not
// called wholesale because its repository .docket.yml read would come from the
// working tree, which the read-from-pinned-Git contract forbids.
func (r *gitStatusReader) configSources(repo gitcli.Repository, docketYML []byte) ([]config.Source, error) {
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

// metadataRevision is the revision the metadata source reads from: the metadata
// branch in docket mode, and the default branch in main mode (where planning
// records live on the default branch and MetadataRevision is empty).
func metadataRevision(pin StatusPin) string {
	if pin.MetadataRevision != "" {
		return pin.MetadataRevision
	}
	return pin.DefaultRevision
}

// sourceRevision maps a source name to the pinned revision it reads from. An
// unknown name is a contract violation, reported as a plain error so Status
// classifies it internal-error.
func sourceRevision(pin StatusPin, source string) (string, error) {
	switch source {
	case sourceMetadata:
		return metadataRevision(pin), nil
	case sourceIntegration:
		return pin.IntegrationRevision, nil
	default:
		return "", fmt.Errorf("status: unknown artifact source %q", source)
	}
}

// corpusPrefixes are the tree prefixes ReadCorpus scopes its listing to,
// derived from the resolved directory settings: the active and archive change
// subdirectories, the ADRs directory, and — when learnings are enabled — the
// learnings subdirectory.
func corpusPrefixes(eff config.Effective) []gitcli.RepoPath {
	changes := eff.ChangesDir.Value
	prefixes := []gitcli.RepoPath{
		gitcli.RepoPath(path.Join(changes, "active")),
		gitcli.RepoPath(path.Join(changes, "archive")),
		gitcli.RepoPath(path.Clean(eff.ADRsDir.Value)),
	}
	if eff.Learnings.Enabled.Value {
		prefixes = append(prefixes, gitcli.RepoPath(path.Join(changes, "learnings")))
	}
	return prefixes
}

// classifyCorpusPath maps a repository-relative path to the record kind and
// location a composer would declare for it, keyed on the configured directory
// prefixes. Only Markdown records classify; a derived index view under the ADR
// or learnings directory is skipped so it never masquerades as a record.
func classifyCorpusPath(eff config.Effective, p string) (repository.RecordKind, repository.RecordLocation, bool) {
	if !strings.HasSuffix(p, ".md") {
		return "", "", false
	}
	changes := eff.ChangesDir.Value
	activePfx := path.Join(changes, "active") + "/"
	archivePfx := path.Join(changes, "archive") + "/"
	learnPfx := path.Join(changes, "learnings") + "/"
	adrsPfx := path.Clean(eff.ADRsDir.Value) + "/"

	switch {
	case strings.HasPrefix(p, activePfx):
		return repository.KindChange, repository.LocationActive, true
	case strings.HasPrefix(p, archivePfx):
		return repository.KindChange, repository.LocationArchive, true
	case eff.Learnings.Enabled.Value && strings.HasPrefix(p, learnPfx):
		if isDerivedIndex(path.Base(p)) {
			return "", "", false
		}
		return repository.KindLearning, repository.LocationLedger, true
	case strings.HasPrefix(p, adrsPfx):
		if isDerivedIndex(path.Base(p)) {
			return "", "", false
		}
		return repository.KindADR, repository.LocationLedger, true
	}
	return "", "", false
}

// isDerivedIndex reports whether a basename names a generated index view rather
// than an authored record.
func isDerivedIndex(base string) bool {
	switch base {
	case "README.md", "LEARNINGS.md", "BOARD.md":
		return true
	}
	return false
}

// classifyGitFailure maps a gitcli adapter failure to the status sentinel it
// should be reported as. Discovery/argument problems and a topology-blocking
// bad repository are invalid input; a cancellation passes through so Status can
// report interrupted; everything else — missing remotes/refs, network, and a
// missing git executable — is an external failure. A non-adapter error passes
// through unchanged, so a bare context cancellation still reads as interrupted
// and anything unexpected reads as internal-error.
func classifyGitFailure(err error) error {
	f, ok := gitcli.AsFailure(err)
	if !ok {
		return err
	}
	switch f.Kind {
	case gitcli.KindInvalidRepository, gitcli.KindInvalidRequest:
		return fmt.Errorf("%w: %s", ErrStatusInvalidInput, f.Error())
	case gitcli.KindCancelled:
		return err
	default:
		return fmt.Errorf("%w: %s", ErrStatusExternal, f.Error())
	}
}

// shortBranch strips the refs/heads/ prefix from a fully-qualified branch ref,
// reporting false when the ref is not a local branch or is empty afterwards.
func shortBranch(ref gitcli.RefName) (string, bool) {
	s := string(ref)
	if !strings.HasPrefix(s, branchRefPrefix) {
		return "", false
	}
	short := s[len(branchRefPrefix):]
	return short, short != ""
}
