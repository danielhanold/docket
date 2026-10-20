package export

import "testing"

func TestFileName(t *testing.T) {
	cases := []struct {
		kind               Kind
		source, created, s string
		want               string
	}{
		{KindSpec, "docs/superpowers/specs/2026-10-17-close-a-change-to-done-without-a-pr-design.md", "2026-10-17", "close-a-change-to-done-without-a-pr",
			"2026-10-17-spec-close-a-change-to-done-without-a-pr.md"},
		{KindPlan, "docs/superpowers/plans/2026-10-17-close-a-change-to-done-without-a-pr.md", "2026-10-17", "close-a-change-to-done-without-a-pr",
			"2026-10-17-plan-close-a-change-to-done-without-a-pr.md"},
		{KindResults, "docs/superpowers/results/2026-10-17-close-a-change-to-done-without-a-pr-results.md", "2026-10-17", "close-a-change-to-done-without-a-pr",
			"2026-10-17-results-close-a-change-to-done-without-a-pr.md"},
		{KindIssue, "docs/changes/archive/2026-10-17-0559-close-a-change-to-done-without-a-pr.md", "2026-10-17", "close-a-change-to-done-without-a-pr",
			"2026-10-17-issue-close-a-change-to-done-without-a-pr.md"},
		{KindSpec, "docs/foo-design.md", "2026-01-02", "foo", "2026-01-02-spec-foo.md"},
		{KindSpec, "docs/foo-design.md", "", "foo", "spec-foo.md"},
		{KindPlan, "docs/2026-10-17-design-notes-design.md", "2026-01-02", "x", "2026-10-17-plan-design-notes-design.md"},
		{KindIssue, "docs/changes/active/0559-x.md", "", "x", "issue-x.md"},
	}
	for _, tc := range cases {
		if got := FileName(tc.kind, tc.source, tc.created, tc.s); got != tc.want {
			t.Errorf("FileName(%s, %q, %q, %q) = %q, want %q", tc.kind, tc.source, tc.created, tc.s, got, tc.want)
		}
	}
}
