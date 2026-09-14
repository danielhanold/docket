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
// regression guard, extended by 0417 with the integration branch. Mutation
// probes: drop RepoWebURL from linkContextOf — reddens; drop the
// IntegrationBranch assignment — reddens (defaulted-param-hides-caller-wiring:
// the assert pins the RESOLVED non-default value).
func TestLinkContextOfCarriesBothFields(t *testing.T) {
	pin := StatusPin{
		RepoWebURL:        "https://github.com/owner/repo",
		IntegrationBranch: "main",
	}
	got := linkContextOf(pin)
	want := render.LinkContext{
		RepoWebURL:        "https://github.com/owner/repo",
		MetadataBranch:    "docket",
		IntegrationBranch: "main",
	}
	if got != want {
		t.Fatalf("linkContextOf = %+v, want %+v", got, want)
	}
	if url := got.BlobURL("docs/x.md"); url != "https://github.com/owner/repo/blob/docket/docs/x.md" {
		t.Fatalf("BlobURL = %q", url)
	}
}

// TestLinkContextOfIntegrationFallsBackToDefaultBranch mirrors
// closeoutContext's integration-branch fallback: an unresolved
// IntegrationBranch on the pin falls back to DefaultBranch.
func TestLinkContextOfIntegrationFallsBackToDefaultBranch(t *testing.T) {
	got := linkContextOf(StatusPin{DefaultBranch: "trunk"})
	if got.IntegrationBranch != "trunk" {
		t.Fatalf("IntegrationBranch = %q, want fallback to DefaultBranch %q", got.IntegrationBranch, "trunk")
	}
}
