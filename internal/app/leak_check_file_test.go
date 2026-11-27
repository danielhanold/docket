package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/leakscan"
	"github.com/danielhanold/docket/internal/testsupport"
)

// leakParityCorpus holds one rule or one deliberate miss per line; change 7
// exists in prReader's corpus, 42 does not. Lines 10-15 must never hit.
const leakParityCorpus = "docket:status set\n" +
	"Docket-Change: 7\n" +
	"see .docket/x and .git/dckt/y\n" +
	"the dckt tool\n" +
	"fixes #0007\n" +
	"see (0007)\n" +
	"change 0007\n" +
	"change #7 and changes #7\n" +
	"the docket word\n" +
	"ADR-0144 stays\n" +
	"#7 unpadded\n" +
	"#0042 unknown id\n" +
	"see 0007 bare\n" +
	"the backlog and the board\n" +
	"x#0007 glued\n" +
	"clean last line"

// writeLeakFile writes body to dir/name, creating dir, and returns the path.
func writeLeakFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func runFileCheck(t *testing.T, reader *fakeReader, files ...string) LeakCheckResult {
	t.Helper()
	return LeakCheck(context.Background(), contextDeps(reader), "", LeakCheckRequest{Files: files})
}

// leakLineHit is a hit projected to what both checks share.
type leakLineHit struct {
	Line int
	Text string
	Rule string
}

func appliedLeakContext(t *testing.T, res LeakCheckResult) *LeakCheckContext {
	t.Helper()
	if res.Result != ResultApplied || res.Context == nil {
		t.Fatalf("result=%q reason=%q message=%q", res.Result, res.Reason, res.Message)
	}
	return res.Context
}

// TestLeakCheckMatchesPublishCheck: the file check finds exactly what the
// publish check finds in a PR body, under both match_word settings and both
// line endings.
func TestLeakCheckMatchesPublishCheck(t *testing.T) {
	for _, matchWord := range []bool{true, false} {
		for _, ending := range []string{"lf", "crlf"} {
			name := ending + "/match_word=false"
			if matchWord {
				name = ending + "/match_word=true"
			}
			t.Run(name, func(t *testing.T) {
				body := leakParityCorpus
				if ending == "crlf" {
					body = strings.ReplaceAll(body, "\n", "\r\n")
				}
				wc, target := leakContext(t, matchWord)
				pubHits, err := runLeakCheck(context.Background(), &fakeLeakGit{fetchRev: gitcli.Revision{Commit: leakBase}}, wc, target, leakHead, &leakscan.PRText{Body: body})
				if err != nil {
					t.Fatalf("runLeakCheck: %v", err)
				}
				var publish []leakLineHit
				for _, h := range pubHits {
					if h.Source == string(leakscan.SourcePRBody) {
						publish = append(publish, leakLineHit{h.Line, h.Text, h.Rule})
					}
				}

				reader := &fakeReader{pin: wc.pin, corpus: prReader(t).corpus}
				path := writeLeakFile(t, testsupport.TempDir(t), "body.md", body)
				c := appliedLeakContext(t, runFileCheck(t, reader, path))
				var op []leakLineHit
				rules := map[string]bool{}
				for _, h := range c.Hits {
					if h.Line > 0 {
						op = append(op, leakLineHit{h.Line, h.Text, h.Rule})
						rules[h.Rule] = true
					}
					if h.Line >= 10 && h.Line <= 15 {
						t.Errorf("deliberate miss on line %d hit: %+v", h.Line, h)
					}
				}
				if !reflect.DeepEqual(op, publish) {
					t.Fatalf("file check hits differ from publish check:\n op=%+v\npub=%+v", op, publish)
				}
				if matchWord {
					for _, r := range []string{"marker", "trailer", "path", "alias", "change-ref", "word"} {
						if !rules[r] {
							t.Errorf("no %s hit in %+v", r, op)
						}
					}
				} else {
					if rules["word"] {
						t.Errorf("word hit with match_word false: %+v", op)
					}
					if !rules["change-ref"] {
						t.Errorf("no change-ref hit with match_word false: %+v", op)
					}
				}
				if c.MatchWord != matchWord {
					t.Errorf("MatchWord=%v, want %v", c.MatchWord, matchWord)
				}
			})
		}
	}
}

// TestLeakCheckSplitsRealLineBreaksOnly: an escaped "\n" is not a line break.
func TestLeakCheckSplitsRealLineBreaksOnly(t *testing.T) {
	dir := testsupport.TempDir(t)
	escaped := writeLeakFile(t, dir, "escaped.md", "ok\\ndocket here")
	c := appliedLeakContext(t, runFileCheck(t, prReader(t), escaped))
	for _, h := range c.Hits {
		if h.Line == 2 || h.Rule == "word" {
			t.Errorf("escaped body hit: %+v", h)
		}
	}

	real := writeLeakFile(t, dir, "real.md", "ok\ndocket here")
	c = appliedLeakContext(t, runFileCheck(t, prReader(t), real))
	want := []LeakCheckHit{{File: real, Line: 2, Text: "docket", Rule: "word"}}
	if !reflect.DeepEqual(c.Hits, want) {
		t.Fatalf("hits=%+v, want %+v", c.Hits, want)
	}
}

func TestLeakCheckScansFileNameAsLineZero(t *testing.T) {
	path := writeLeakFile(t, testsupport.TempDir(t), "dckt-notes.md", "clean\n")
	res := runFileCheck(t, prReader(t), path)
	c := appliedLeakContext(t, res)
	want := []LeakCheckHit{{File: path, Line: 0, Text: "dckt", Rule: "alias"}}
	if !reflect.DeepEqual(c.Hits, want) || c.Clean {
		t.Fatalf("clean=%v hits=%+v, want %+v", c.Clean, c.Hits, want)
	}
}

// TestLeakCheckNeverScansDirectory: a directory named after docket never taints
// the files inside it; only the base name is scanned.
func TestLeakCheckNeverScansDirectory(t *testing.T) {
	dir := filepath.Join(testsupport.TempDir(t), "docket-update-ticket.abc123", ".docket")
	path := writeLeakFile(t, dir, "notes.md", "all clear\n")
	res := runFileCheck(t, prReader(t), path)
	c := appliedLeakContext(t, res)
	if !c.Clean || len(c.Hits) != 0 {
		t.Fatalf("clean=%v hits=%+v", c.Clean, c.Hits)
	}
	if js := mustJSON(t, res); !strings.Contains(js, `"hits":[]`) {
		t.Fatalf("JSON lacks an empty hits array: %s", js)
	}
}

func TestLeakCheckReportsHitsUnderApplied(t *testing.T) {
	dir := testsupport.TempDir(t)
	path := writeLeakFile(t, dir, "body.md", "see .docket/x\n")
	res := runFileCheck(t, prReader(t), path)
	if res.Result != ResultApplied || res.Reason != "" || res.Context == nil || res.Context.Clean {
		t.Fatalf("result=%q reason=%q context=%+v", res.Result, res.Reason, res.Context)
	}
	want := []LeakCheckHit{{File: path, Line: 1, Text: ".docket", Rule: "path"}}
	if !reflect.DeepEqual(res.Context.Hits, want) {
		t.Fatalf("hits=%+v, want %+v", res.Context.Hits, want)
	}

	named := writeLeakFile(t, dir, "dckt.md", "fine\n")
	res = runFileCheck(t, prReader(t), named)
	if js := mustJSON(t, res); !strings.Contains(js, `"line":0`) {
		t.Fatalf("name hit JSON lacks line 0: %s", js)
	}
}

func TestLeakCheckOrdersFilesAsGiven(t *testing.T) {
	dir := testsupport.TempDir(t)
	b := writeLeakFile(t, dir, "b-dckt.md", "x\n.docket\n")
	a := writeLeakFile(t, dir, "a-dckt.md", "x\n.docket\n")
	c := appliedLeakContext(t, runFileCheck(t, prReader(t), b, a))
	var files []string
	var lines []int
	for _, h := range c.Hits {
		files = append(files, h.File)
		lines = append(lines, h.Line)
	}
	if !reflect.DeepEqual(files, []string{b, b, a, a}) || !reflect.DeepEqual(lines, []int{0, 2, 0, 2}) {
		t.Fatalf("files=%v lines=%v", files, lines)
	}
}

func TestLeakCheckReportsMatchWordUsed(t *testing.T) {
	path := writeLeakFile(t, testsupport.TempDir(t), "body.md", "fine\n")
	for _, want := range []bool{false, true} {
		reader := prReader(t)
		reader.pin.Config.Effective.LeakCheck.MatchWord = config.Value[bool]{Value: want, Explicit: true}
		c := appliedLeakContext(t, runFileCheck(t, reader, path))
		if c.MatchWord != want {
			t.Errorf("MatchWord=%v, want %v", c.MatchWord, want)
		}
	}
}

func TestLeakCheckRefusesMissingFile(t *testing.T) {
	for _, req := range []LeakCheckRequest{{}, {Files: []string{""}}} {
		reader := prReader(t)
		res := LeakCheck(context.Background(), contextDeps(reader), "", req)
		if res.Result != ResultInvalidInput || res.Reason != ReasonLeakCheckMissingFile || res.Context != nil {
			t.Errorf("req=%+v: result=%q reason=%q context=%+v", req, res.Result, res.Reason, res.Context)
		}
		if reader.pinCount != 0 {
			t.Errorf("req=%+v: pinned %d time(s) before refusing", req, reader.pinCount)
		}
	}
}

func TestLeakCheckRefusesUnreadableFile(t *testing.T) {
	dir := testsupport.TempDir(t)
	clean := writeLeakFile(t, dir, "clean.md", "fine\n")
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{filepath.Join(dir, "absent.md"), sub} {
		reader := prReader(t)
		res := runFileCheck(t, reader, clean, bad)
		if res.Result != ResultInvalidInput || res.Reason != ReasonLeakCheckUnreadableFile || res.Context != nil {
			t.Errorf("%s: result=%q reason=%q context=%+v", bad, res.Result, res.Reason, res.Context)
		}
		if !strings.Contains(res.Message, bad) {
			t.Errorf("%s: message %q does not name the path", bad, res.Message)
		}
		if reader.pinCount != 0 {
			t.Errorf("%s: pinned %d time(s) before refusing", bad, reader.pinCount)
		}
	}
}

func TestLeakCheckPinFailureIsNotClean(t *testing.T) {
	path := writeLeakFile(t, testsupport.TempDir(t), "clean.md", "fine\n")
	reader := prReader(t)
	reader.pinErr = errors.New("boom")
	res := runFileCheck(t, reader, path)
	if res.Result == ResultApplied || res.Context != nil {
		t.Fatalf("result=%q context=%+v", res.Result, res.Context)
	}
}

func TestLeakCheckHumanText(t *testing.T) {
	dir := testsupport.TempDir(t)
	clean := writeLeakFile(t, dir, "clean.md", "fine\n")
	if got := runFileCheck(t, prReader(t), clean).HumanText(); !strings.Contains(got, "leak.check: clean") {
		t.Errorf("clean text = %q", got)
	}
	dirty := writeLeakFile(t, dir, "body.md", "see .docket/x\n")
	got := runFileCheck(t, prReader(t), dirty).HumanText()
	for _, want := range []string{"hit(s)", dirty, ".docket"} {
		if !strings.Contains(got, want) {
			t.Errorf("hit text %q lacks %q", got, want)
		}
	}
	if got := runFileCheck(t, prReader(t)).HumanText(); !strings.Contains(got, "leak.check: invalid-input (missing-file)") {
		t.Errorf("refusal text = %q", got)
	}
}
