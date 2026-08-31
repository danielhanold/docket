package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/reposetup"
)

// This file is the maintenance sweep's metadata-observation seam. The sweep no
// longer re-probes the remote per work item behind a single stale pin; instead a
// session, bound once to the discovered repository and the invocation's captured
// configuration, hands each dispatched operation attempt exactly ONE fresh
// metadata observation. The observation combines the captured setup with one
// fresh metadata fetch and the corpus read at that revision, and is discarded
// after the operation returns — so a long sweep never mutates against a tip it
// read minutes earlier, yet never re-runs default-branch discovery, the setup
// fetches, or the topology probe.

// sweepObservation is one immutable metadata observation: the captured setup
// combined with exactly one fresh metadata fetch, plus the corpus/snapshot/blob
// versions read at that revision. It serves ONE dispatched operation attempt and
// is discarded after the operation returns.
type sweepObservation struct {
	pin   StatusPin      // captured setup + this attempt's fresh MetadataRevision
	inv   sweepInventory // snapshot + versionByPath at that revision
	blobs []StatusBlob   // the exact corpus bytes, for the bound reader
}

// sweepPreparer prepares one observation per operation attempt.
type sweepPreparer interface {
	Prepare(ctx context.Context) (*sweepObservation, error)
}

// sweepSession is the production sweepPreparer. It is bound once to the
// discovered repository and the sweep's initial pin, and reruns none of the
// setup work: every Prepare is one fresh metadata fetch plus one corpus read at
// that revision, over the captured client, repository, and configuration.
type sweepSession struct {
	client *gitcli.Client
	repo   gitcli.Repository
	base   StatusPin
}

var _ sweepPreparer = (*sweepSession)(nil)

// newSweepSession derives the production preparer from the sweep's initial pin.
// The session is bound to the discovered repository and the invocation's
// captured configuration; it never reruns default-branch discovery, setup
// fetches, or the topology probe.
func newSweepSession(client *gitcli.Client, repo gitcli.Repository, base StatusPin) *sweepSession {
	return &sweepSession{client: client, repo: repo, base: base}
}

// Prepare produces one fresh metadata observation: exactly one fetch of the
// fixed metadata branch, a pin that copies the captured setup with only its
// metadata tip replaced, and one corpus read at that revision. No pre-fetch
// probe, no cache, no TTL — the bounded failure-classification behavior is
// FetchBranch's own (its diagnostic probe rides inside the same call).
func (s *sweepSession) Prepare(ctx context.Context) (*sweepObservation, error) {
	rev, err := fetchPinnedRevision(ctx, s.client, s.repo, gitcli.RefName(branchRefPrefix+reposetup.MetadataBranchName))
	if err != nil {
		// A failed metadata fetch is the classified error, never a stale
		// fallback and never an inferred absence.
		return nil, err
	}

	// The initial pin's configuration is used consistently throughout the
	// invocation: copy the captured base and replace only the metadata tip.
	pin := s.base
	pin.MetadataRevision = rev

	// Read the corpus ONCE at that revision through gitStatusReader mechanics
	// bound to the session's client+repo — not a second PinContext. The
	// capturing wrapper retains the exact blobs the one read returned so the
	// inventory and the bound reader share a single corpus read.
	capture := &corpusCapturingReader{StatusReader: &gitStatusReader{client: s.client, repo: s.repo}}
	inv, refusal := sweepBuildSnapshot(ctx, capture, pin, s.base.Config.Effective)
	if refusal != nil {
		return nil, errors.New(refusal.Message)
	}
	return &sweepObservation{pin: pin, inv: inv, blobs: capture.blobs}, nil
}

// corpusCapturingReader wraps a StatusReader to retain the exact blobs the one
// ReadCorpus call returned, so sweepSession.Prepare can build the inventory and
// serve the bound reader from a single corpus read at the pinned revision.
type corpusCapturingReader struct {
	StatusReader
	blobs []StatusBlob
}

func (r *corpusCapturingReader) ReadCorpus(ctx context.Context, pin StatusPin) ([]StatusBlob, error) {
	blobs, err := r.StatusReader.ReadCorpus(ctx, pin)
	if err != nil {
		return nil, err
	}
	r.blobs = blobs
	return blobs, nil
}

// pinnedRepository is implemented by the production StatusReader so a bound
// reader can recover the client and repository its PinContext resolved and
// enforce the same-repository guard without a re-pin. A reader that does not
// implement it (a test fake, or one not yet pinned) leaves the guard inert.
type pinnedRepository interface {
	pinnedRepo() (*gitcli.Client, gitcli.Repository, bool)
}

func (r *gitStatusReader) pinnedRepo() (*gitcli.Client, gitcli.Repository, bool) {
	return r.client, r.repo, r.repo.PrimaryWorktree != ""
}

// boundStatusReader serves a prepared observation. PinContext returns the
// supplied pin WITHOUT fetching (touching the repoDir argument only for the
// same-repository guard); ReadCorpus returns the same immutable corpus WITHOUT
// rereading the remote; artifact and branch reads delegate to the live reader,
// because those operation-specific proofs stay live.
type boundStatusReader struct {
	obs  *sweepObservation
	live StatusReader
	// client and repo enforce the same-repository guard; both are recovered from
	// live when it is the production reader. guard is false for a test fake or an
	// unpinned reader, leaving the check inert.
	client *gitcli.Client
	repo   gitcli.Repository
	guard  bool
}

var _ StatusReader = (*boundStatusReader)(nil)

// newBoundStatusReader wraps a prepared observation and the live reader the
// operation-specific reads delegate to. When live is the production reader, the
// bound reader recovers its client+repository so PinContext can refuse a repoDir
// naming a different repository.
func newBoundStatusReader(obs *sweepObservation, live StatusReader) StatusReader {
	b := &boundStatusReader{obs: obs, live: live}
	if pr, ok := live.(pinnedRepository); ok {
		if client, repo, bound := pr.pinnedRepo(); bound {
			b.client, b.repo, b.guard = client, repo, true
		}
	}
	return b
}

// PinContext returns the observation's pin without fetching. It touches repoDir
// only for the same-repository guard: a repoDir resolving to a different
// repository errors rather than silently reusing the captured facts.
func (b *boundStatusReader) PinContext(ctx context.Context, repoDir string) (StatusPin, error) {
	if b.guard {
		got, err := b.client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
		if err != nil {
			return StatusPin{}, classifyGitFailure(err)
		}
		if got.PrimaryWorktree != b.repo.PrimaryWorktree {
			return StatusPin{}, fmt.Errorf("%w: sweep session bound to %s, refusing repoDir resolving to %s",
				ErrStatusInvalidInput, b.repo.PrimaryWorktree, got.PrimaryWorktree)
		}
	}
	return b.obs.pin, nil
}

// ReadCorpus returns the observation's immutable corpus without rereading the
// remote.
func (b *boundStatusReader) ReadCorpus(ctx context.Context, pin StatusPin) ([]StatusBlob, error) {
	return b.obs.blobs, nil
}

// BranchFacts delegates to the live reader: feature-branch existence is an
// operation-specific proof that stays live.
func (b *boundStatusReader) BranchFacts(ctx context.Context, pin StatusPin, branches []string) (domain.BranchFacts, error) {
	return b.live.BranchFacts(ctx, pin, branches)
}

// ArtifactExists delegates to the live reader: an artifact's presence is an
// operation-specific proof that stays live.
func (b *boundStatusReader) ArtifactExists(ctx context.Context, pin StatusPin, source, artifactPath string) (bool, error) {
	return b.live.ArtifactExists(ctx, pin, source, artifactPath)
}

// ReadArtifact delegates to the live reader: an artifact's bytes are an
// operation-specific proof that stays live.
func (b *boundStatusReader) ReadArtifact(ctx context.Context, pin StatusPin, source, artifactPath string) (StatusArtifact, error) {
	return b.live.ReadArtifact(ctx, pin, source, artifactPath)
}
