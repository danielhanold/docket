package app

import (
	"strings"

	"github.com/danielhanold/docket/internal/render"
)

// This file owns the app layer's link-context derivation (change 0341). The
// render package is pure and never derives its own LinkContext; before 0341
// every operation built the context inline and every one of them forgot
// RepoWebURL, so all Go-rendered artifact tables, backlinks, and PR links came
// out as bare code spans. linkContextOf is now the SOLE constructor (a source
// guard in link_context_guard_test.go enforces that), and githubWebURL is the
// pure parser matching the bash renderers' accepted remote forms.

// githubWebURL converts a GitHub remote URL to its https web base
// ("https://github.com/owner/repo"), accepting exactly the three forms the
// bash renderers accept: git@github.com:owner/repo(.git),
// https://github.com/owner/repo(.git), ssh://git@github.com/owner/repo(.git),
// one trailing ".git" stripped. Any other spelling — non-GitHub hosts, path
// remotes, empty — yields "", which render treats as the bare-path fallback.
// An empty owner/repo remainder also yields "" (bash would emit a broken
// empty-repo URL here; "" is the strictly safer reading of the same degenerate
// input).
func githubWebURL(remoteURL string) string {
	var rest string
	switch {
	case strings.HasPrefix(remoteURL, "git@github.com:"):
		rest = strings.TrimPrefix(remoteURL, "git@github.com:")
	case strings.HasPrefix(remoteURL, "https://github.com/"):
		rest = strings.TrimPrefix(remoteURL, "https://github.com/")
	case strings.HasPrefix(remoteURL, "ssh://git@github.com/"):
		rest = strings.TrimPrefix(remoteURL, "ssh://git@github.com/")
	default:
		return ""
	}
	rest = strings.TrimSuffix(rest, ".git")
	if rest == "" {
		return ""
	}
	return "https://github.com/" + rest
}

// linkContextOf is the sole constructor of the LinkContext app operations hand
// to render: the repository web URL and the metadata branch travel together,
// so no call site can silently omit the URL again — the exact defect 0341
// fixes. Companion to metadataBranchOf.
func linkContextOf(pin StatusPin) render.LinkContext {
	return render.LinkContext{
		RepoWebURL:     pin.RepoWebURL,
		MetadataBranch: metadataBranchOf(pin),
	}
}
