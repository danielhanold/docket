package leakscan

import (
	"reflect"
	"testing"
)

func TestLineRules(t *testing.T) {
	ids := map[int]bool{1: true, 12: true, 36: true, 612: true}
	cases := []struct {
		line string
		w    bool
		ok   bool
		text string
		rule Rule
	}{
		{"<!-- docket:backlink:start (generated) -->", true, true, "docket:", RuleMarker},
		{"<!-- docket:backlink:start -->", false, true, "docket:", RuleMarker},
		{"# dckt:start", false, true, "dckt:", RuleMarker},
		{"Docket: 12-345 was filed", false, false, "", ""},
		{"Docket: 12-345 was filed", true, true, "Docket", RuleWord},
		{"Docket-Operation: change.claim", false, true, "Docket-Operation:", RuleTrailer},
		{"  docket-request-id: abc", false, true, "docket-request-id:", RuleTrailer},
		{"see .docket/docs/x.md", false, true, ".docket", RulePath},
		{"edit .docket.yml", false, true, ".docket", RulePath},
		{"cat .git/dckt/config.yml", false, true, ".git/dckt", RulePath},
		{"run dckt status", false, true, "dckt", RuleAlias},
		{"the dckts list", true, false, "", ""},
		{"fix the widget (0612)", false, true, "(0612)", RuleChangeRef},
		{"change 0612 lands", false, true, "change 0612", RuleChangeRef},
		{"Changes #612 and more", false, true, "Changes #612", RuleChangeRef},
		{"tracked as #0612", false, true, "#0612", RuleChangeRef},
		{"the (2026) layout", true, false, "", ""},
		// 612 is a backlog id, but "(612)" is not zero-padded, so it passes.
		{"the (612) layout", true, false, "", ""},
		{"fixes #612", true, false, "", ""},
		{"change 3 files", true, false, "", ""},
		{"per ADR-0036 and ADR 0012", true, false, "", ""},
		{"fix (0613)", true, false, "", ""},
		{"the docket was filed", true, true, "docket", RuleWord},
		{"the docket was filed", false, false, "", ""},
		{"docketBranch := x", true, true, "docket", RuleWord},
		{"myDocket := x", true, true, "Docket", RuleWord},
		{"court dockets", true, false, "", ""},
		{"pocketdocket", true, false, "", ""},
		{"DOCKET", true, true, "DOCKET", RuleWord},
		{"docket_branch", true, true, "docket", RuleWord},
	}
	for _, c := range cases {
		m, ok := Line(c.line, Options{MatchWord: c.w, ChangeIDs: ids})
		if ok != c.ok {
			t.Errorf("Line(%q, W=%v) ok = %v, want %v (got %+v)", c.line, c.w, ok, c.ok, m)
			continue
		}
		if ok && (m.Text != c.text || m.Rule != c.rule) {
			t.Errorf("Line(%q, W=%v) = %+v, want {%q %s}", c.line, c.w, m, c.text, c.rule)
		}
	}
}

func TestTextLineNumbersAndCRLF(t *testing.T) {
	got := Text(SourcePRBody, "ok\r\nsee .docket/x\r\n", Options{MatchWord: true})
	want := []Hit{{Source: SourcePRBody, Line: 2, Rule: RulePath, Text: ".docket"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Text = %+v, want %+v", got, want)
	}
}

func TestScanAttributesEverySource(t *testing.T) {
	in := Input{
		Commits:    []Commit{{ID: "c1", Message: "Add widget\n\nRefs (0612)\n"}},
		AddedPaths: []string{"notes/.docket-old.md"},
		AddedLines: []AddedLine{{Path: "a.go", Line: 7, Text: "// dckt"}},
		PR:         &PRText{Title: "change 0012", Body: "plain\n"},
	}
	got := Scan(in, Options{MatchWord: true, ChangeIDs: map[int]bool{12: true, 612: true}})
	want := []Hit{
		{Source: SourceCommitMessage, Commit: "c1", Line: 3, Text: "(0612)", Rule: RuleChangeRef},
		{Source: SourceAddedPath, File: "notes/.docket-old.md", Text: ".docket", Rule: RulePath},
		{Source: SourceAddedLine, File: "a.go", Line: 7, Text: "dckt", Rule: RuleAlias},
		{Source: SourcePRTitle, Line: 1, Text: "change 0012", Rule: RuleChangeRef},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scan =\n%+v\nwant\n%+v", got, want)
	}
}

func TestScanNilPRAndNilIDs(t *testing.T) {
	in := Input{
		Commits:    []Commit{{ID: "c1", Message: "fix (0612) and change 0012\n"}},
		AddedLines: []AddedLine{{Path: "a.go", Line: 1, Text: "tracked as #0612"}},
	}
	for _, h := range Scan(in, Options{MatchWord: true}) {
		if h.Rule == RuleChangeRef {
			t.Fatalf("nil ChangeIDs produced a change-ref hit: %+v", h)
		}
	}
}
