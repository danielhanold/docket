package app

import (
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/layout"
	"github.com/danielhanold/docket/internal/repository"
)

func specFixture() string {
	return "<!-- docket:backlink:start (generated — do not hand-edit) -->\n" +
		"> ↩ **[Change 0042 — Widget](https://example.invalid/blob/docket/docs/changes/active/0042-widget.md)**\n" +
		"<!-- docket:backlink:end -->\n\n" +
		"# Widget: design\n\nBody paragraph.\n\n## Decisions\n\n1. One.\n"
}

// specChange builds change 42 "Widget" through the same corpus path the
// operations use (snapshotOf / mustChange live in named_branch_facts_test.go).
func specChange(t *testing.T) domain.Change {
	t.Helper()
	rec := strings.Replace(lifecycleChange(42, "widget", "in-progress"), "title: 'A change'", "title: 'Widget'", 1)
	snap := snapshotOf(t, []StatusBlob{{
		Kind:     repository.KindChange,
		Location: repository.LocationActive,
		Path:     groomPath(42, "widget"),
		Revision: "blobspec0042",
		Data:     []byte(rec),
	}})
	return mustChange(t, snap, 42)
}

func TestSpecCopyBytesStripsBacklinkAndAddsChangeLine(t *testing.T) {
	got, err := specCopyBytes([]byte(specFixture()), specChange(t), layout.Shared)
	if err != nil {
		t.Fatal(err)
	}
	want := "# Widget: design\n\nChange 0042 — Widget\n\nBody paragraph.\n\n## Decisions\n\n1. One.\n"
	if string(got) != want {
		t.Fatalf("copy:\n%q\nwant\n%q", got, want)
	}
}

// TestSpecCopyPrivateOmitsChangeLine: a private repository's copy is the spec
// minus its backlink with leading newlines trimmed, and carries no change line;
// the shared copy is unchanged.
func TestSpecCopyPrivateOmitsChangeLine(t *testing.T) {
	got, err := specCopyBytes([]byte(specFixture()), specChange(t), layout.Private)
	if err != nil {
		t.Fatal(err)
	}
	want := "# Widget: design\n\nBody paragraph.\n\n## Decisions\n\n1. One.\n"
	if string(got) != want {
		t.Fatalf("private copy:\n%q\nwant\n%q", got, want)
	}
	if strings.Contains(string(got), "Change ") {
		t.Fatalf("private copy carries a change line:\n%q", got)
	}
	shared, err := specCopyBytes([]byte(specFixture()), specChange(t), layout.Shared)
	if err != nil {
		t.Fatal(err)
	}
	if wantShared := "# Widget: design\n\nChange 0042 — Widget\n\nBody paragraph.\n\n## Decisions\n\n1. One.\n"; string(shared) != wantShared {
		t.Fatalf("shared copy:\n%q\nwant\n%q", shared, wantShared)
	}
}

func TestSpecCopyBytesSkipsFencedH1(t *testing.T) {
	src := "```\n# not a title\n```\n\n# Real title\n\ntext\n"
	got, err := specCopyBytes([]byte(src), specChange(t), layout.Shared)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "# Real title\n\nChange 0042 — Widget\n\ntext\n") {
		t.Fatalf("change line not after the real title:\n%s", got)
	}
	if strings.Contains(string(got), "# not a title\n\nChange") {
		t.Fatalf("change line placed inside the fence:\n%s", got)
	}
}

func TestSpecCopyBytesNoTitle(t *testing.T) {
	got, err := specCopyBytes([]byte("Just prose.\n"), specChange(t), layout.Shared)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "Change 0042 — Widget\n\nJust prose.\n" {
		t.Fatalf("no-title copy = %q", got)
	}
}

// TestSpecCopyBytesTitleIsLastLine: a title with no line terminator (the
// spec ends on it) still gets the change line after it, separated by one
// blank line, and the copy ends with a terminator.
func TestSpecCopyBytesTitleIsLastLine(t *testing.T) {
	got, err := specCopyBytes([]byte("# Only a title"), specChange(t), layout.Shared)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "# Only a title\n\nChange 0042 — Widget\n" {
		t.Fatalf("title-only copy = %q", got)
	}
}

func TestSpecCopyBytesKeepsCRLF(t *testing.T) {
	src := strings.ReplaceAll(specFixture(), "\n", "\r\n")
	got, err := specCopyBytes([]byte(src), specChange(t), layout.Shared)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ReplaceAll(string(got), "\r\n", ""), "\n") {
		t.Fatalf("mixed line endings:\n%q", got)
	}
	if strings.Contains(string(got), "docket:backlink") {
		t.Fatalf("backlink survived:\n%q", got)
	}
	want := strings.ReplaceAll("# Widget: design\n\nChange 0042 — Widget\n\nBody paragraph.\n\n## Decisions\n\n1. One.\n", "\n", "\r\n")
	if string(got) != want {
		t.Fatalf("CRLF copy:\n%q\nwant\n%q", got, want)
	}
}

func TestSpecCopyBytesRefusesMalformedMarkers(t *testing.T) {
	src := "<!-- docket:backlink:start (generated — do not hand-edit) -->\n# T\n"
	if _, err := specCopyBytes([]byte(src), specChange(t), layout.Shared); err == nil {
		t.Fatal("a dangling backlink marker must refuse, never be copied")
	}
}
