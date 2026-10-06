// Package render owns docket's canonical record serialization, source-preserving
// authored-section edits, and the derived-view renderers (board, artifact block,
// spec backlink, ADR index). It is pure: it receives typed values, source bytes,
// and an explicit LinkContext, and returns bytes or an error. It never reads the
// filesystem, invokes Git, reads the clock, parses flags, or commits. Equal input
// yields byte-identical output.
package render

// LinkContext carries everything link rendering needs; render never derives it.
type LinkContext struct {
	// RepoWebURL is the https base of the repository, no trailing slash,
	// e.g. "https://github.com/danielhanold/docket". Empty means "render
	// repo-relative links only" (callers without a resolvable web remote).
	RepoWebURL string
	// MetadataBranch is the metadata branch blob links for metadata-branch
	// records point at, e.g. "docket".
	MetadataBranch string
	// IntegrationBranch is the branch PR merges land on, e.g. "main". It is
	// consulted only for rows whose file is on the integration branch (a done
	// change's "Spec (merged)" row, and a legacy done change's Plan/Results
	// rows); empty falls back to MetadataBranch at the BlobURLOnBranch
	// boundary, so a malformed URL is unrepresentable.
	IntegrationBranch string
	// PrivateMetadata marks a private repository, whose metadata branch is
	// published to a local bare remote rather than the web host: no web page
	// exists for it, so every blob link on MetadataBranch renders "" (the
	// repo-relative fallback). Links on other branches are unaffected.
	PrivateMetadata bool
}

// BlobURL returns the blob URL on the metadata branch — correct for records
// that live on that branch (change files, specs, ADRs) — or "" when
// RepoWebURL is empty.
func (l LinkContext) BlobURL(repoRelPath string) string {
	return l.BlobURLOnBranch(repoRelPath, l.MetadataBranch)
}

// BlobURLOnBranch returns RepoWebURL + "/blob/" + branch + "/" + repoRelPath,
// or "" when RepoWebURL is empty. An empty branch falls back to
// MetadataBranch: the defensive default for a caller whose branch is
// unresolvable, never a malformed "/blob//" URL. A private repository's
// metadata branch (after that fallback) has no web page, so it also yields "".
func (l LinkContext) BlobURLOnBranch(repoRelPath, branch string) string {
	if l.RepoWebURL == "" {
		return ""
	}
	if branch == "" {
		branch = l.MetadataBranch
	}
	if l.PrivateMetadata && branch == l.MetadataBranch {
		return ""
	}
	return l.RepoWebURL + "/blob/" + branch + "/" + repoRelPath
}
