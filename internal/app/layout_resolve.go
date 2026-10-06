package app

import (
	"context"
	"fmt"
	"os"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/layout"
)

// This file resolves a discovered repository's per-repo layout — where its
// metadata branch is published and how the branch is spelled — and owns the
// two accessors every metadata transaction and metadata fetch reads it
// through. The mode is decided from STATE (layout.Detect), never from config.

// remoteURLReader is the one Git read resolveLayout needs: origin's configured
// URL, which names a private repository's metadata store. *gitcli.Client
// satisfies it.
type remoteURLReader interface {
	RemoteURL(ctx context.Context, repo gitcli.Repository, remote gitcli.RemoteName) (string, error)
}

// resolveLayout resolves repo's layout. A shared repository needs nothing but
// its common dir — origin's URL is never read, so an unreadable origin URL
// never breaks a shared repository. A private repository locates its store
// from origin's URL and the data home. Every failure is an external failure:
// the mode probe and the store location are facts docket cannot guess.
func resolveLayout(ctx context.Context, r remoteURLReader, repo gitcli.Repository) (layout.Layout, error) {
	mode, err := layout.Detect(repo.CommonDir)
	if err != nil {
		return layout.Layout{}, fmt.Errorf("%w: resolving the repository's visibility mode: %v", ErrStatusExternal, err)
	}
	if mode == layout.Shared {
		return layout.SharedLayout(repo.CommonDir, repo.PrimaryWorktree), nil
	}
	url, err := r.RemoteURL(ctx, repo, originRemote)
	if err != nil {
		return layout.Layout{}, fmt.Errorf("%w: private repository: reading origin's URL to locate the metadata store: %v", ErrStatusExternal, err)
	}
	ownerRepo, err := layout.OwnerRepo(url)
	if err != nil {
		return layout.Layout{}, fmt.Errorf("%w: private repository: locating the metadata store: %v", ErrStatusExternal, err)
	}
	dataHome, err := layout.DataHome(os.Getenv, os.UserHomeDir)
	if err != nil {
		return layout.Layout{}, fmt.Errorf("%w: private repository: locating the metadata store: %v", ErrStatusExternal, err)
	}
	return layout.PrivateLayout(repo.CommonDir, repo.PrimaryWorktree, dataHome, ownerRepo), nil
}

// metadataRemote is the remote the metadata branch is fetched from and
// lease-pushed to: origin in a shared repository, the bare dckt remote in a
// private one. Every metadata transaction.Request names its Remote through
// this accessor (TestTransactionRequestsUseResolvedMetadataRemote).
func metadataRemote(l layout.Layout) gitcli.RemoteName { return gitcli.RemoteName(l.MetadataRemote) }

// metadataRef is the fully-qualified metadata branch ref. Every metadata
// transaction.Request names its TargetRef through this accessor
// (TestNoIntegrationPushOutsidePRMerge).
func metadataRef(l layout.Layout) gitcli.RefName { return gitcli.RefName(l.MetadataRef()) }
