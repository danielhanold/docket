package render_test

import (
	"testing"

	"github.com/danielhanold/docket/internal/render"
)

func TestBlobURLWithRepoWebURL(t *testing.T) {
	l := render.LinkContext{
		RepoWebURL:     "https://github.com/danielhanold/docket",
		MetadataBranch: "docket",
	}
	got := l.BlobURL("docs/changes/active/0312-slug.md")
	want := "https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0312-slug.md"
	if got != want {
		t.Fatalf("BlobURL = %q, want %q", got, want)
	}
}

func TestBlobURLWithoutRepoWebURL(t *testing.T) {
	l := render.LinkContext{RepoWebURL: "", MetadataBranch: "docket"}
	if got := l.BlobURL("docs/changes/active/0312-slug.md"); got != "" {
		t.Fatalf("BlobURL with empty RepoWebURL = %q, want empty", got)
	}
}

func TestBlobURLOnBranch(t *testing.T) {
	l := render.LinkContext{
		RepoWebURL:        "https://github.com/danielhanold/docket",
		MetadataBranch:    "docket",
		IntegrationBranch: "main",
	}
	cases := []struct{ name, branch, want string }{
		{"feature branch", "fix/some-change",
			"https://github.com/danielhanold/docket/blob/fix/some-change/docs/superpowers/plans/x.md"},
		{"integration branch", "main",
			"https://github.com/danielhanold/docket/blob/main/docs/superpowers/plans/x.md"},
		{"empty branch falls back to metadata", "",
			"https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/x.md"},
	}
	for _, c := range cases {
		if got := l.BlobURLOnBranch("docs/superpowers/plans/x.md", c.branch); got != c.want {
			t.Errorf("%s: BlobURLOnBranch = %q, want %q", c.name, got, c.want)
		}
	}
}

// TestPrivateMetadataRendersNoMetadataURL: a private repository's metadata
// branch is published to a local bare remote, never to the web host, so a blob
// URL on it would point at nothing. Metadata-branch links (explicit or via the
// empty-branch fallback) render "" — the repo-relative fallback — while a link
// on the integration branch, which does live on the web host, stays absolute.
func TestPrivateMetadataRendersNoMetadataURL(t *testing.T) {
	l := render.LinkContext{
		RepoWebURL:        "https://github.com/o/r",
		MetadataBranch:    "dckt",
		IntegrationBranch: "main",
		PrivateMetadata:   true,
	}
	const p = "docs/changes/active/0001-x.md"
	if got := l.BlobURL(p); got != "" {
		t.Errorf("BlobURL = %q, want empty for a private metadata branch", got)
	}
	if got := l.BlobURLOnBranch(p, ""); got != "" {
		t.Errorf("BlobURLOnBranch(empty) = %q, want empty for a private metadata branch", got)
	}
	if got := l.BlobURLOnBranch(p, "dckt"); got != "" {
		t.Errorf("BlobURLOnBranch(dckt) = %q, want empty for a private metadata branch", got)
	}
	if got, want := l.BlobURLOnBranch(p, "main"), "https://github.com/o/r/blob/main/"+p; got != want {
		t.Errorf("BlobURLOnBranch(main) = %q, want %q", got, want)
	}
}

func TestBlobURLOnBranchWithoutRepoWebURL(t *testing.T) {
	l := render.LinkContext{RepoWebURL: "", MetadataBranch: "docket", IntegrationBranch: "main"}
	if got := l.BlobURLOnBranch("docs/x.md", "fix/some-change"); got != "" {
		t.Fatalf("BlobURLOnBranch with empty RepoWebURL = %q, want empty", got)
	}
}
