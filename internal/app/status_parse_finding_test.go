package app

import (
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/layout"
	"github.com/danielhanold/docket/internal/repository"
)

// TestStatusParseFindingIsPlain pins the plain-language text `docket status`
// prints for a record it cannot parse: the same wording `repository check`
// gives the broken record (no internal error-kind prefix, no column), an
// impact, and a hand-edit remedy naming the record in the real checkout.
// Mutation probe: restore `Message: err.Error()` in parseFinding — the
// message assertion must redden.
func TestStatusParseFindingIsPlain(t *testing.T) {
	rel := "docs/changes/active/0007-x.md"
	blobs := []StatusBlob{{Kind: repository.KindChange, Path: rel, Data: []byte("---\nid: 7\nid: 8\n---\nbody\n")}}

	shared := statusRecordLocator(layout.SharedLayout("/repo/.git", "/repo"))
	_, findings := parseCorpusAt(blobs, shared)
	if len(findings) != 1 {
		t.Fatalf("findings = %+v, want exactly one", findings)
	}
	f := findings[0]
	if f.Code != "duplicate-field" || f.Path != rel || f.Entity != "change" {
		t.Errorf("code/path/entity = %q/%q/%q", f.Code, f.Path, f.Entity)
	}
	wantMsg := "docket can't read this record's frontmatter: for `id` on line 3, field is declared more than once (first declared on line 2)."
	if f.Message != wantMsg {
		t.Errorf("Message = %q, want %q", f.Message, wantMsg)
	}
	if want := "docket skips the record, so the change is missing from the board and from every other check."; f.Impact != want {
		t.Errorf("Impact = %q, want %q", f.Impact, want)
	}
	wantRemedy := "In `.docket/docs/changes/active/0007-x.md`, fix the frontmatter between the two `---` lines; then commit and push it from `.docket`, and run `docket repository repair` so the board and link blocks catch up."
	if f.Remedy != wantRemedy {
		t.Errorf("Remedy = %q, want %q", f.Remedy, wantRemedy)
	}

	markers := []StatusBlob{{Kind: repository.KindChange, Path: rel, Data: []byte("---\nid: 7\n---\n\n<!-- docket:artifacts:start -->\n- x\n")}}
	_, mf := parseCorpusAt(markers, shared)
	if len(mf) != 1 || !strings.HasPrefix(mf[0].Message, "The `<!-- docket:… -->` block markers in this record don't pair up: ") ||
		strings.Contains(mf[0].Message, "marker-imbalance:") || !strings.Contains(mf[0].Remedy, "`.docket/docs/changes/active/0007-x.md`") {
		t.Errorf("marker finding = %+v", mf)
	}

	private := statusRecordLocator(layout.PrivateLayout("/repo/.git", "/repo", "/home/u/.local/share", "o/r"))
	_, pf := parseCorpusAt(blobs, private)
	if len(pf) != 1 || strings.Contains(pf[0].Remedy, "/home/u") || !strings.Contains(pf[0].Remedy, "your private metadata checkout") {
		t.Errorf("private Remedy = %+v", pf)
	}
}
