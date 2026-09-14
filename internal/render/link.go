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
	// consulted only for rows whose file reaches the integration branch (the
	// Plan/Results rows of a done change); empty falls back to MetadataBranch
	// at the BlobURLOnBranch boundary, so a malformed URL is unrepresentable.
	IntegrationBranch string
}

// BlobURL returns the blob URL on the metadata branch — correct for records
// that live on that branch (change files, specs, ADRs) — or "" when
// RepoWebURL is empty.
func (l LinkContext) BlobURL(repoRelPath string) string {
	return l.BlobURLOnBranch(repoRelPath, l.MetadataBranch)
}

// BlobURLOnBranch returns RepoWebURL + "/blob/" + branch + "/" + repoRelPath,
// or "" when RepoWebURL is empty. An empty branch falls back to
// MetadataBranch: the defensive default for callers whose lifecycle ref is
// unresolvable (change 0417), never a malformed "/blob//" URL.
func (l LinkContext) BlobURLOnBranch(repoRelPath, branch string) string {
	if l.RepoWebURL == "" {
		return ""
	}
	if branch == "" {
		branch = l.MetadataBranch
	}
	return l.RepoWebURL + "/blob/" + branch + "/" + repoRelPath
}
