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
	// MetadataBranch is the branch blob links point at, e.g. "docket".
	MetadataBranch string
}

// BlobURL returns RepoWebURL + "/blob/" + MetadataBranch + "/" + repoRelPath,
// or "" when RepoWebURL is empty.
func (l LinkContext) BlobURL(repoRelPath string) string {
	if l.RepoWebURL == "" {
		return ""
	}
	return l.RepoWebURL + "/blob/" + l.MetadataBranch + "/" + repoRelPath
}
