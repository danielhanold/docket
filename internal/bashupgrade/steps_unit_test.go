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

// TestDocketCommandLinesShapes: docket counts as a command wherever a shell command
// word can stand, and never as an argument, a value, or part of a longer word.
func TestDocketCommandLinesShapes(t *testing.T) {
	for name, line := range map[string]string{
		"line start":            "docket version",
		"prompt":                "$ docket version",
		"after &&":              "cd <repo> && docket repository check",
		"after ||":              "false || docket repository check",
		"after ;":               "cd <repo>; docket repository check",
		"after |":               "yes | docket repository repair",
		"inside $(":             "echo \"$(docket version)\"",
		"after sudo":            "sudo docket install check",
		"after VAR=value":       "DOCKET_HOME=/x docket install check",
		"after quoted VAR":      "A='x y' B=\"z\" docket version",
		"assignment then sudo":  "A=1 sudo docket version",
		"path to the binary":    "~/.local/bin/docket version",
		"after && with no gaps": "cd x&&docket version",
	} {
		if got := docketCommandLines(line + "\n"); len(got) != 1 {
			t.Errorf("%s: %q was not recognised as a docket command (got %q)", name, line, got)
		}
	}
	for name, line := range map[string]string{
		"argument":          "echo docket",
		"assignment value":  "VERSION=docket",
		"longer word":       "docketx run",
		"comment":           "# docket version",
		"url path segment":  "curl https://x/docket/install.sh",
		"quoted in echo":    "echo 'run docket version'",
		"sudo other":        "sudo rm -f docket",
		"assignment, other": "A=1 rm docket",
	} {
		if got := docketCommandLines(line + "\n"); len(got) != 0 {
			t.Errorf("%s: %q was taken for a docket command", name, line)
		}
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
