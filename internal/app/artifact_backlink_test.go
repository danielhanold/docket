package app

import (
	"context"
	"github.com/danielhanold/docket/internal/testsupport"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/reposetup"
	"github.com/danielhanold/docket/internal/repository"
)

// backlinkDeps wraps a fake reader as the read-only deps the operation consumes.
// ArtifactBacklink reads the change from the corpus and writes a working-tree
// file; it opens no transaction, so a nil Engine is deliberate.
func backlinkDeps(fake *fakeReader) PlanningDeps {
	return PlanningDeps{Reader: fake, Clock: testClock()}
}

// changeByPath rebuilds the snapshot from the same corpus the operation reads
// and returns the change at path, so a test's golden ties to render.Backlink
// Content rather than a hand-copied string.
func changeByPath(t *testing.T, pin StatusPin, corpus []StatusBlob, path string) domain.Change {
	t.Helper()
	inputs, _ := parseCorpus(corpus)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: pin.Config.Effective, Documents: inputs})
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}
	for _, c := range build.Snapshot.Changes() {
		if c.Path() == path {
			return c
		}
	}
	t.Fatalf("no change at %q in corpus", path)
	return domain.Change{}
}

// backlinkGolden is the exact managed block render.BacklinkContent produces for
// the change at changePath, in the pin's repo-relative link mode.
func backlinkGolden(t *testing.T, pin StatusPin, corpus []StatusBlob, changePath string) string {
	t.Helper()
	c := changeByPath(t, pin, corpus, changePath)
	block, err := render.BacklinkContent(c, render.LinkContext{MetadataBranch: reposetup.MetadataBranchName})
	if err != nil {
		t.Fatalf("render backlink golden: %v", err)
	}
	return block
}

const backlinkChangePath = "docs/changes/active/0315-claim.md"

// backlinkCorpus is one change the backlink can target.
func backlinkCorpus() []StatusBlob {
	return []StatusBlob{changeBlob(315, "claim", "feat", "high", "")}
}

// TestArtifactBacklinkRendersBlock: a fresh artifact (content, no backlink block)
// gains the managed block at its top, byte-matching render.BacklinkContent.
func TestArtifactBacklinkRendersBlock(t *testing.T) {
	pin := docketPin(t)
	corpus := backlinkCorpus()
	root := testsupport.TempDir(t)
	artifact := filepath.Join(root, "plan.md")
	if err := os.WriteFile(artifact, []byte("# Plan\n\nAuthored body.\n"), 0o644); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}

	got := ArtifactBacklink(context.Background(), backlinkDeps(&fakeReader{pin: pin, corpus: corpus}), root,
		ArtifactBacklinkRequest{ArtifactPath: "plan.md", ChangePath: backlinkChangePath})
	if got.Result != ResultApplied {
		t.Fatalf("result=%q reason=%q message=%q", got.Result, got.Reason, got.Message)
	}

	out, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	golden := backlinkGolden(t, pin, corpus, backlinkChangePath)
	if !strings.HasPrefix(string(out), golden) {
		t.Fatalf("artifact does not open with the golden block:\n got %q\nwant prefix %q", out, golden)
	}
	if !strings.Contains(string(out), "# Plan\n\nAuthored body.\n") {
		t.Fatalf("authored body not preserved: %q", out)
	}
}

// TestArtifactBacklinkIdempotent: a second run yields a byte-identical file.
func TestArtifactBacklinkIdempotent(t *testing.T) {
	pin := docketPin(t)
	corpus := backlinkCorpus()
	root := testsupport.TempDir(t)
	artifact := filepath.Join(root, "plan.md")
	if err := os.WriteFile(artifact, []byte("# Plan\n\nBody.\n"), 0o644); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
	req := ArtifactBacklinkRequest{ArtifactPath: "plan.md", ChangePath: backlinkChangePath}
	deps := backlinkDeps(&fakeReader{pin: pin, corpus: corpus})

	if got := ArtifactBacklink(context.Background(), deps, root, req); got.Result != ResultApplied {
		t.Fatalf("first run result=%q reason=%q", got.Result, got.Reason)
	}
	first, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatalf("read first: %v", err)
	}

	got := ArtifactBacklink(context.Background(), deps, root, req)
	if got.Result != ResultNoOp {
		t.Fatalf("second run result=%q, want no-op (idempotent)", got.Result)
	}
	second, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatalf("read second: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("second run not byte-identical:\nfirst  %q\nsecond %q", first, second)
	}
}

// TestArtifactBacklinkRefusesMalformedMarkers: a dangling backlink start marker
// is a typed refusal and the file bytes are left untouched.
func TestArtifactBacklinkRefusesMalformedMarkers(t *testing.T) {
	pin := docketPin(t)
	corpus := backlinkCorpus()
	root := testsupport.TempDir(t)
	artifact := filepath.Join(root, "plan.md")
	// A start marker with no partner end marker: document.Parse must reject the
	// whole population, and the operation must refuse without a write.
	original := "<!-- docket:backlink:start (generated — do not hand-edit) -->\n# Plan\n\nBody.\n"
	if err := os.WriteFile(artifact, []byte(original), 0o644); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}

	fake := &fakeReader{pin: pin, corpus: corpus}
	got := ArtifactBacklink(context.Background(), backlinkDeps(fake), root,
		ArtifactBacklinkRequest{ArtifactPath: "plan.md", ChangePath: backlinkChangePath})
	if got.Result == ResultApplied || got.Result == ResultNoOp {
		t.Fatalf("malformed markers accepted: result=%q", got.Result)
	}
	if got.Reason != ReasonBacklinkMalformedMarkers {
		t.Fatalf("reason=%q, want %q", got.Reason, ReasonBacklinkMalformedMarkers)
	}
	// The refusal must precede any authoritative read: a malformed artifact is
	// rejected before the corpus is pinned. This isolates the up-front marker
	// guard from document.Apply's phase-3 reparse backstop — removing the guard
	// lets the operation read the corpus before failing, reddening this assert.
	if fake.pinCount != 0 {
		t.Fatalf("malformed artifact pinned context %d times; want the refusal before any read", fake.pinCount)
	}
	out, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(out) != original {
		t.Fatalf("file mutated on refusal:\n got %q\nwant %q", out, original)
	}
}

// TestArtifactBacklinkPathContainment: absolute paths, ../ escapes, and a symlink
// leaf pointing outside the repo are each rejected with a stable reason, and no
// out-of-root file is written.
func TestArtifactBacklinkPathContainment(t *testing.T) {
	pin := docketPin(t)
	corpus := backlinkCorpus()

	t.Run("absolute", func(t *testing.T) {
		root := testsupport.TempDir(t)
		got := ArtifactBacklink(context.Background(), backlinkDeps(&fakeReader{pin: pin, corpus: corpus}), root,
			ArtifactBacklinkRequest{ArtifactPath: "/etc/hosts", ChangePath: backlinkChangePath})
		if got.Reason != ReasonBacklinkAbsolutePath {
			t.Fatalf("reason=%q, want %q (result=%q)", got.Reason, ReasonBacklinkAbsolutePath, got.Result)
		}
	})

	t.Run("dotdot-escape", func(t *testing.T) {
		root := testsupport.TempDir(t)
		got := ArtifactBacklink(context.Background(), backlinkDeps(&fakeReader{pin: pin, corpus: corpus}), root,
			ArtifactBacklinkRequest{ArtifactPath: "../escape.md", ChangePath: backlinkChangePath})
		if got.Reason != ReasonBacklinkPathEscape {
			t.Fatalf("reason=%q, want %q (result=%q)", got.Reason, ReasonBacklinkPathEscape, got.Result)
		}
	})

	t.Run("symlink-escape", func(t *testing.T) {
		root := testsupport.TempDir(t)
		outside := filepath.Join(testsupport.TempDir(t), "target.md")
		if err := os.WriteFile(outside, []byte("SECRET — outside the repo\n"), 0o644); err != nil {
			t.Fatalf("seed outside target: %v", err)
		}
		link := filepath.Join(root, "link.md")
		if err := os.Symlink(outside, link); err != nil {
			t.Fatalf("symlink: %v", err)
		}

		got := ArtifactBacklink(context.Background(), backlinkDeps(&fakeReader{pin: pin, corpus: corpus}), root,
			ArtifactBacklinkRequest{ArtifactPath: "link.md", ChangePath: backlinkChangePath})
		if got.Reason != ReasonBacklinkSymlinkEscape {
			t.Fatalf("reason=%q, want %q (result=%q)", got.Reason, ReasonBacklinkSymlinkEscape, got.Result)
		}
		// The out-of-root target must be untouched — the write must not follow
		// the symlink.
		body, err := os.ReadFile(outside)
		if err != nil {
			t.Fatalf("read outside target: %v", err)
		}
		if string(body) != "SECRET — outside the repo\n" {
			t.Fatalf("write followed the escaping symlink: %q", body)
		}
	})
}

// TestArtifactBacklinkUnknownChange: a --change path naming no record in the
// corpus is a typed refusal, never a fabricated backlink.
func TestArtifactBacklinkUnknownChange(t *testing.T) {
	pin := docketPin(t)
	corpus := backlinkCorpus()
	root := testsupport.TempDir(t)
	artifact := filepath.Join(root, "plan.md")
	if err := os.WriteFile(artifact, []byte("# Plan\n"), 0o644); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}

	got := ArtifactBacklink(context.Background(), backlinkDeps(&fakeReader{pin: pin, corpus: corpus}), root,
		ArtifactBacklinkRequest{ArtifactPath: "plan.md", ChangePath: "docs/changes/active/0999-absent.md"})
	if got.Reason != ReasonBacklinkUnknownChange {
		t.Fatalf("reason=%q, want %q (result=%q)", got.Reason, ReasonBacklinkUnknownChange, got.Result)
	}
	out, _ := os.ReadFile(artifact)
	if string(out) != "# Plan\n" {
		t.Fatalf("file mutated for an unknown change: %q", out)
	}
}
