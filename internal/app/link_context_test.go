package app

import (
	"testing"

	"github.com/danielhanold/docket/internal/render"
)

// TestGithubWebURLAcceptedForms pins the parser to exactly the three remote
// spellings the bash renderers accept, .git stripped; everything else is ""
// (bare-path fallback).
func TestGithubWebURLAcceptedForms(t *testing.T) {
	cases := []struct{ in, want string }{
		{"git@github.com:owner/repo.git", "https://github.com/owner/repo"},
		{"git@github.com:owner/repo", "https://github.com/owner/repo"},
		{"https://github.com/owner/repo.git", "https://github.com/owner/repo"},
		{"https://github.com/owner/repo", "https://github.com/owner/repo"},
		{"ssh://git@github.com/owner/repo.git", "https://github.com/owner/repo"},
		{"ssh://git@github.com/owner/repo", "https://github.com/owner/repo"},
		{"git@gitlab.com:owner/repo.git", ""},
		{"https://gitlab.com/owner/repo", ""},
		{"ssh://git@bitbucket.org/owner/repo", ""},
		{"/tmp/fixtures/origin.git", ""},
		{"../origin.git", ""},
		{"", ""},
		{"git@github.com:", ""},
		{"https://github.com/", ""},
		{"git@github.com.evil.example:owner/repo.git", ""},
	}
	for _, c := range cases {
		if got := githubWebURL(c.in); got != c.want {
			t.Errorf("githubWebURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestLinkContextOfCarriesBothFields is the constructor half of the 0341
// regression guard. Mutation probe: drop RepoWebURL from linkContextOf — this
// test must redden.
func TestLinkContextOfCarriesBothFields(t *testing.T) {
	pin := StatusPin{
		Mode:           metadataModeDocket,
		MetadataBranch: "docket",
		RepoWebURL:     "https://github.com/owner/repo",
	}
	got := linkContextOf(pin)
	want := render.LinkContext{RepoWebURL: "https://github.com/owner/repo", MetadataBranch: "docket"}
	if got != want {
		t.Fatalf("linkContextOf = %+v, want %+v", got, want)
	}
	if url := got.BlobURL("docs/x.md"); url != "https://github.com/owner/repo/blob/docket/docs/x.md" {
		t.Fatalf("BlobURL = %q", url)
	}

	pin = StatusPin{Mode: metadataModeMain, DefaultBranch: "main", RepoWebURL: "https://github.com/owner/repo"}
	if got := linkContextOf(pin); got.MetadataBranch != "main" || got.RepoWebURL != "https://github.com/owner/repo" {
		t.Fatalf("main-mode linkContextOf = %+v", got)
	}
}
