package app

import (
	"fmt"
	"strings"
	"testing"
)

func TestWithLinkedSpecLinksAndRefusesMissingAnchor(t *testing.T) {
	got := withLinkedSpec(lifecycleChange(3, "widget", "proposed"), fixtureSpecPath("widget"))
	if !strings.Contains(got, "\nspec: 'docs/superpowers/specs/2026-08-01-widget-design.md'\n") {
		t.Errorf("record does not link the fixture spec:\n%s", got)
	}
	if strings.Contains(got, "\nspec:\n") {
		t.Errorf("record still carries an empty spec line:\n%s", got)
	}

	panicked := func() (p bool) {
		defer func() { p = recover() != nil }()
		withLinkedSpec("---\nid: 3\n---\n", fixtureSpecPath("widget"))
		return false
	}()
	if !panicked {
		t.Errorf("withLinkedSpec on a record with no empty spec line did not panic")
	}
}

// fixtureSpecPath is the spec path a build-ready fixture change links.
func fixtureSpecPath(slug string) string {
	return "docs/superpowers/specs/2026-08-01-" + slug + "-design.md"
}

// withLinkedSpec links specPath on a record's empty spec line, so a proposed
// fixture carries the design every build-ready change needs. It panics when the
// record has no empty spec line, so a fixture can never silently stay spec-less.
func withLinkedSpec(src, specPath string) string {
	const anchor = "\nspec:\n"
	if !strings.Contains(src, anchor) {
		panic("withLinkedSpec: record has no empty \"spec:\" line to link")
	}
	return strings.Replace(src, anchor, "\nspec: '"+specPath+"'\n", 1)
}

// fixtureSpecFile is a spec as the metadata branch stores it: a
// docket:backlink block above the authored title.
func fixtureSpecFile(id int, slug string) string {
	return "<!-- docket:backlink:start (generated — do not hand-edit) -->\n" +
		"> ↩ **[Change " + fmt.Sprintf("%04d", id) + " — A change](../../changes/active/" + fmt.Sprintf("%04d", id) + "-" + slug + ".md)**\n" +
		"<!-- docket:backlink:end -->\n\n" +
		"# Widget: design\n\nBuild the widget.\n"
}
