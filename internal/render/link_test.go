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
