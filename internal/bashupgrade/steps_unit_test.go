package bashupgrade

import (
	"reflect"
	"strings"
	"testing"
)

const sampleGuide = "# Guide\n\n<!-- upgrade-step: repo-prepare -->\n\n```sh\ncd <repo>\ndocket repository prepare\n```\n\nProse.\n\n```sh\ncurl -fsSLO https://example.invalid/docket/install.sh\n```\n\n<!-- upgrade-step: repo-check -->\n```sh\n$ docket repository check\n```\n"

func TestParseGuideOrderAndFences(t *testing.T) {
	g, err := parseGuide(sampleGuide)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, s := range g.Steps {
		names = append(names, s.Name)
	}
	if !reflect.DeepEqual(names, []string{"repo-prepare", "repo-check"}) {
		t.Fatalf("steps = %v", names)
	}
	if g.Steps[0].Body != "cd <repo>\ndocket repository prepare\n" {
		t.Fatalf("body = %q", g.Steps[0].Body)
	}
	if len(g.Unmarked) != 1 || !strings.Contains(g.Unmarked[0].Body, "curl") {
		t.Fatalf("unmarked = %#v", g.Unmarked)
	}
}

func TestParseGuideRejects(t *testing.T) {
	for name, src := range map[string]string{
		"marker without fence": "<!-- upgrade-step: a -->\nprose\n",
		"malformed marker":     "<!-- upgrade-step: Bad_Name -->\n```sh\nx\n```\n",
		"duplicate":            "<!-- upgrade-step: a -->\n```\nx\n```\n<!-- upgrade-step: a -->\n```\ny\n```\n",
		"unterminated":         "<!-- upgrade-step: a -->\n```\nx\n",
	} {
		if _, err := parseGuide(src); err == nil {
			t.Errorf("%s: parseGuide accepted it", name)
		}
	}
}

func TestDocketCommandLines(t *testing.T) {
	got := docketCommandLines("cd <repo>\n  docket version\n$ docket install check\n# docket in a comment\ncurl https://x/docket/install.sh\ndocketx run\n")
	want := []string{"docket version", "docket install check"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestSubstitutePlaceholders(t *testing.T) {
	out, err := substitutePlaceholders("cd <repo>\n", map[string]string{"repo": "/r"})
	if err != nil || out != "cd /r\n" {
		t.Fatalf("got %q, %v", out, err)
	}
	if _, err := substitutePlaceholders("cd <other>\n", map[string]string{"repo": "/r"}); err == nil {
		t.Fatal("an unknown placeholder was accepted")
	}
}

func TestDispatchBlockSpan(t *testing.T) {
	const s, e = "<!-- docket:dispatch:start (managed by docket) -->", "<!-- docket:dispatch:end -->"
	for _, tc := range []struct {
		name       string
		lines      []string
		start, end int
		bad        bool
	}{
		{"balanced", []string{"x", s, "body", e, "y"}, 1, 3, false},
		{"absent", []string{"x", "y"}, -1, -1, false},
		{"start only", []string{s, "body"}, 0, 0, true},
		{"end only", []string{"body", e}, 0, 0, true},
		{"out of order", []string{e, "body", s}, 0, 0, true},
		{"two starts", []string{s, s, "body", e}, 0, 0, true},
		{"two ends", []string{s, "body", e, e}, 0, 0, true},
	} {
		start, end, err := dispatchBlockSpan(tc.lines)
		if tc.bad {
			if err == nil {
				t.Errorf("%s: want an error, got span %d..%d", tc.name, start, end)
			}
			continue
		}
		if err != nil || start != tc.start || end != tc.end {
			t.Errorf("%s: got %d..%d, %v; want %d..%d", tc.name, start, end, err, tc.start, tc.end)
		}
	}
}
