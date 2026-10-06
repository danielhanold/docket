package app

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/layout"
	"github.com/danielhanold/docket/internal/reposetup"
)

// This file reports docket-named refs on a private repository's origin: a
// `docket` or `dckt` branch, or any refs/docket/ ref. A private repository keeps
// its metadata off origin, so such a ref is visible to everyone with access to
// origin. It is reported as a warning only — it never blocks and never changes
// a classified state — and a listing failure is reported as unverified, never as
// a proven absence. A shared repository never lists origin for it.

// The finding codes.
const (
	FindingMetadataOnSharedRemote           = "metadata-on-shared-remote"
	FindingMetadataOnSharedRemoteUnverified = "metadata-on-shared-remote-unverified"
)

// remoteRefLister is the Git seam the report reads origin through.
type remoteRefLister interface {
	ListRemoteRefs(ctx context.Context, repo gitcli.Repository, remote gitcli.RemoteName, patterns []string) (map[gitcli.RefName]gitcli.ObjectID, error)
}

var _ remoteRefLister = (*gitcli.Client)(nil)

// metadataRefsOnOrigin lists origin's docket-named refs, sorted: exactly
// refs/heads/docket, exactly refs/heads/dckt, and anything under refs/docket/.
// ls-remote patterns match by trailing path component, so the result is
// filtered to those exact names (refs/heads/docketeer and refs/x/heads/docket
// are not docket-named). A listing failure is returned, never an empty list.
func metadataRefsOnOrigin(ctx context.Context, g remoteRefLister, repo gitcli.Repository) ([]string, error) {
	shared := "refs/heads/" + layout.SharedName
	private := "refs/heads/" + layout.PrivateName
	prefix := "refs/" + layout.SharedName + "/"
	refs, err := g.ListRemoteRefs(ctx, repo, originRemote, []string{shared, private, prefix + "*"})
	if err != nil {
		return nil, err
	}
	var out []string
	for ref := range refs {
		name := string(ref)
		if name == shared || name == private || strings.HasPrefix(name, prefix) {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out, nil
}

// sharedRemoteMetadataFindings reports each docket-named ref on a private
// repository's origin as a warning, or one unverified warning when origin
// cannot be listed. A shared repository returns nil without listing.
func sharedRemoteMetadataFindings(ctx context.Context, g remoteRefLister, lay layout.Layout, repo gitcli.Repository) []reposetup.Finding {
	if lay.Mode != layout.Private {
		return nil
	}
	refs, err := metadataRefsOnOrigin(ctx, g, repo)
	if err != nil {
		return []reposetup.Finding{{
			Code:     FindingMetadataOnSharedRemoteUnverified,
			Severity: reposetup.SeverityWarning,
			Message:  fmt.Sprintf("origin could not be listed for docket-named refs (unverified, not proven absent): %v", err),
			Remedy:   "Restore access to origin, then re-run docket repository check.",
		}}
	}
	var out []reposetup.Finding
	for _, ref := range refs {
		out = append(out, reposetup.Finding{
			Code:     FindingMetadataOnSharedRemote,
			Severity: reposetup.SeverityWarning,
			Ref:      ref,
			Message:  fmt.Sprintf("origin holds %s, a docket-named ref, but this repository is private; everyone with access to origin can see it.", ref),
			Remedy:   fmt.Sprintf("If nothing still needs it, delete it from origin: `git push origin --delete %s`.", ref),
		})
	}
	return out
}

// sharedRemoteMetadataStatusFindings is sharedRemoteMetadataFindings in the
// StatusFinding shape a workflow operation result carries (the ref as Path).
func sharedRemoteMetadataStatusFindings(ctx context.Context, g remoteRefLister, lay layout.Layout, repo gitcli.Repository) []StatusFinding {
	fs := sharedRemoteMetadataFindings(ctx, g, lay, repo)
	if fs == nil {
		return nil
	}
	out := make([]StatusFinding, 0, len(fs))
	for _, f := range fs {
		out = append(out, StatusFinding{
			Code:     f.Code,
			Severity: string(f.Severity),
			Path:     f.Ref,
			Message:  f.Message,
			Remedy:   f.Remedy,
		})
	}
	return out
}
