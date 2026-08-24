package app

import (
	"context"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/render"
)

// TestPinContextDerivesGitHubRepoWebURL is 0341's wiring regression guard:
// given a GitHub origin remote, the production reader's pin carries the derived
// web base and rendered link output carries blob URLs, not bare code spans.
// Hermetic: origin's CONFIGURED url is the GitHub spelling; the insteadOf
// rewrite routes all real network traffic to the local bare origin (RemoteURL
// reads raw config, so it still sees the GitHub spelling).
// Mutation probes (each must redden this test, run with -count=1):
//   - in PinContext, drop the RemoteURL call / the RepoWebURL assignment;
//   - in linkContextOf, drop the RepoWebURL field.
func TestPinContextDerivesGitHubRepoWebURL(t *testing.T) {
	repo := newMainModeRepo(t, map[string]string{
		"docs/changes/active/0007-widget.md": changeRecord(7, "widget", "Widget"),
	})
	runGit(t, repo.invocation, "remote", "set-url", "origin", "git@github.com:owner/widgets.git")
	runGit(t, repo.invocation, "config", "url."+repo.origin+".insteadOf", "git@github.com:owner/widgets.git")

	ctx := context.Background()
	reader := NewGitStatusReader(newGitClient(t))
	pin, err := reader.PinContext(ctx, repo.invocation)
	if err != nil {
		t.Fatalf("PinContext: %v", err)
	}
	if pin.RepoWebURL != "https://github.com/owner/widgets" {
		t.Fatalf("pin.RepoWebURL = %q, want https://github.com/owner/widgets", pin.RepoWebURL)
	}

	link := linkContextOf(pin)
	if got := link.BlobURL("docs/x.md"); got != "https://github.com/owner/widgets/blob/main/docs/x.md" {
		t.Fatalf("BlobURL = %q", got)
	}

	corpus, err := reader.ReadCorpus(ctx, pin)
	if err != nil {
		t.Fatalf("ReadCorpus: %v", err)
	}
	c := changeByPath(t, pin, corpus, "docs/changes/active/0007-widget.md")
	block, err := render.BacklinkContent(c, link)
	if err != nil {
		t.Fatalf("BacklinkContent: %v", err)
	}
	if !strings.Contains(block, "(https://github.com/owner/widgets/blob/main/docs/changes/active/0007-widget.md)") {
		t.Fatalf("backlink is not a GitHub link:\n%s", block)
	}
	if strings.Contains(block, "`docs/changes/active/0007-widget.md`") {
		t.Fatalf("backlink still renders the bare code span:\n%s", block)
	}
}

// TestPinContextNonGitHubOriginYieldsEmptyWebURL pins the fallback: a plain
// local-path origin derives "", and rendering stays in repo-relative mode.
func TestPinContextNonGitHubOriginYieldsEmptyWebURL(t *testing.T) {
	repo := newMainModeRepo(t, map[string]string{
		"docs/changes/active/0007-widget.md": changeRecord(7, "widget", "Widget"),
	})
	reader := NewGitStatusReader(newGitClient(t))
	pin, err := reader.PinContext(context.Background(), repo.invocation)
	if err != nil {
		t.Fatalf("PinContext: %v", err)
	}
	if pin.RepoWebURL != "" {
		t.Fatalf("pin.RepoWebURL = %q, want \"\"", pin.RepoWebURL)
	}
	if url := linkContextOf(pin).BlobURL("docs/x.md"); url != "" {
		t.Fatalf("BlobURL = %q, want \"\" (bare-path mode)", url)
	}
}
