package app

import "testing"

// TestAssemblePRBodyArtifactsBlock pins the PR description's Docket-owned
// blocks: the backlink at the top, the docket:artifacts plan/results links right
// after it when there are any, every authored byte preserved, and re-assembling
// an assembled body is a fixed point.
func TestAssemblePRBodyArtifactsBlock(t *testing.T) {
	const (
		backlinkStart  = "<!-- docket:backlink:start (generated — do not hand-edit) -->\n"
		backlinkEnd    = "<!-- docket:backlink:end -->\n"
		artifactsStart = "<!-- docket:artifacts:start (generated — do not hand-edit) -->\n"
		artifactsEnd   = "<!-- docket:artifacts:end -->\n"
		authored       = "## Summary\n\nDoes the thing.\n"
	)
	backlink := backlinkStart + "> ↩ **[Change 0007 — X](https://example.test/blob/docket/docs/changes/active/0007-x.md)**\n" + backlinkEnd
	staleBacklink := backlinkStart + "> ↩ **[Change 0007 — X](https://example.test/old)**\n" + backlinkEnd
	links := "- Plan: [p.md](https://example.test/blob/docket/docs/superpowers/plans/p.md)\n"
	staleLinks := "- Plan: [old.md](https://example.test/old)\n"
	wantFull := backlink + artifactsStart + links + artifactsEnd + authored

	cases := []struct {
		name, body, artifacts, want string
	}{
		{"fresh body gains both blocks", authored, links, wantFull},
		{"no plan or results: backlink only", authored, "", backlink + authored},
		{"both present are replaced in place",
			staleBacklink + artifactsStart + staleLinks + artifactsEnd + authored, links, wantFull},
		{"a backlink-only body gains the artifacts block after its backlink",
			staleBacklink + authored, links, wantFull},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := assemblePRBody([]byte(tc.body), backlink, tc.artifacts)
			if err != nil {
				t.Fatalf("assemblePRBody: %v", err)
			}
			if string(got) != tc.want {
				t.Fatalf("body =\n%q\nwant\n%q", got, tc.want)
			}
			again, err := assemblePRBody(got, backlink, tc.artifacts)
			if err != nil {
				t.Fatalf("re-assemble: %v", err)
			}
			if string(again) != string(got) {
				t.Fatalf("re-assembling is not a fixed point:\n%q\nthen\n%q", got, again)
			}
		})
	}
}
