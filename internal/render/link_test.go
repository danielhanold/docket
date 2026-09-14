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

func TestBlobURLOnBranchWithoutRepoWebURL(t *testing.T) {
	l := render.LinkContext{RepoWebURL: "", MetadataBranch: "docket", IntegrationBranch: "main"}
	if got := l.BlobURLOnBranch("docs/x.md", "fix/some-change"); got != "" {
		t.Fatalf("BlobURLOnBranch with empty RepoWebURL = %q, want empty", got)
	}
}
