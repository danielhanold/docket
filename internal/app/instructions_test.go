package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/harness"
	"github.com/danielhanold/docket/internal/reposeed"
	"github.com/danielhanold/docket/internal/testsupport"
)

// writeFixture writes content at path, creating its parent directories.
func writeFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// mkdirFixture creates dir and its parents.
func mkdirFixture(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

// skipIfAncestorHasGit skips when an ancestor of dir carries a .git entry: the
// walk-up would find it, so "outside git" cannot be fabricated there.
func skipIfAncestorHasGit(t *testing.T, dir string) {
	t.Helper()
	for d := filepath.Dir(dir); ; d = filepath.Dir(d) {
		if _, err := os.Lstat(filepath.Join(d, ".git")); err == nil {
			t.Skipf("ancestor %s of the temp dir has a .git entry", d)
		}
		if filepath.Dir(d) == d {
			return
		}
	}
}

// privateFixture fabricates a private primary worktree r (r/.git/dckt/) under
// base, with the instructions file holding content when content is non-empty.
func privateFixture(t *testing.T, base, content string) string {
	t.Helper()
	r := filepath.Join(base, "r")
	mkdirFixture(t, filepath.Join(r, ".git", "dckt"))
	if content != "" {
		writeFixture(t, filepath.Join(r, ".git", "dckt", "AGENTS.md"), content)
	}
	return r
}

func assertRead(t *testing.T, label, dir string, wantContent string, wantPrivate bool) {
	t.Helper()
	got, private, err := ReadPrivateInstructions(dir)
	if err != nil {
		t.Fatalf("%s: ReadPrivateInstructions(%s) error %v", label, dir, err)
	}
	if private != wantPrivate {
		t.Errorf("%s: private = %v, want %v", label, private, wantPrivate)
	}
	if wantContent == "" {
		if got != nil {
			t.Errorf("%s: content = %q, want nil", label, got)
		}
		return
	}
	if string(got) != wantContent {
		t.Errorf("%s: content = %q, want %q", label, got, wantContent)
	}
}

func TestReadPrivateInstructionsOutsideGitAndShared(t *testing.T) {
	base := testsupport.TempDir(t)
	outside := filepath.Join(base, "plain", "dir")
	mkdirFixture(t, outside)

	shared := filepath.Join(base, "shared")
	mkdirFixture(t, filepath.Join(shared, ".git"))
	sub := filepath.Join(shared, "sub", "dir")
	mkdirFixture(t, sub)

	assertRead(t, "shared root", shared, "", false)
	assertRead(t, "shared subdir", sub, "", false)

	skipIfAncestorHasGit(t, outside)
	assertRead(t, "outside git", outside, "", false)
}

func TestReadPrivateInstructionsFromEveryWorktreeShape(t *testing.T) {
	base := testsupport.TempDir(t)
	r := privateFixture(t, base, "rules\n")

	sub := filepath.Join(r, "sub")
	mkdirFixture(t, sub)
	assertRead(t, "primary subdirectory", sub, "rules\n", true)

	// A feature worktree inside the clone: a .git file with a relative gitdir.
	feature := filepath.Join(r, ".worktrees", "f")
	writeFixture(t, filepath.Join(feature, ".git"), "gitdir: ../../.git/worktrees/f\n")
	writeFixture(t, filepath.Join(r, ".git", "worktrees", "f", "commondir"), "../..\n")
	assertRead(t, "feature worktree", feature, "rules\n", true)

	// A metadata checkout outside the clone: an absolute gitdir.
	meta := filepath.Join(base, "s", "checkouts", "c")
	writeFixture(t, filepath.Join(meta, ".git"), "gitdir: "+filepath.Join(r, ".git", "worktrees", "c")+"\n")
	writeFixture(t, filepath.Join(r, ".git", "worktrees", "c", "commondir"), "../..\n")
	assertRead(t, "metadata checkout", meta, "rules\n", true)
}

func TestReadPrivateInstructionsPrivateWithoutFile(t *testing.T) {
	r := privateFixture(t, testsupport.TempDir(t), "")
	assertRead(t, "private, no file", r, "", true)
}

func TestReadPrivateInstructionsUnreadableIsAnError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file")
	}
	r := privateFixture(t, testsupport.TempDir(t), "rules\n")
	path := filepath.Join(r, ".git", "dckt", "AGENTS.md")
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
	got, _, err := ReadPrivateInstructions(r)
	if err == nil {
		t.Fatalf("unreadable file: content %q, nil error; want an error", got)
	}
	if got != nil {
		t.Errorf("unreadable file: content = %q, want nil", got)
	}
}

func TestReadPrivateInstructionsStateFileIsAnError(t *testing.T) {
	r := filepath.Join(testsupport.TempDir(t), "r")
	writeFixture(t, filepath.Join(r, ".git", "dckt"), "not a directory\n")
	if got, private, err := ReadPrivateInstructions(r); err == nil {
		t.Fatalf(".git/dckt as a file: (%q, %v, nil); want an error", got, private)
	}
}

const sectionFixture = "lead\n<!-- docket:dispatch:start (a) -->\nX\n<!-- docket:dispatch:end -->\ntail\n"

func TestSelectInstructionsSection(t *testing.T) {
	cases := []struct {
		name, content, section string
		want                   []byte
	}{
		{"all verbatim", sectionFixture, InstructionsSectionAll, []byte(sectionFixture)},
		{"dispatch block", sectionFixture, InstructionsSectionDispatch,
			[]byte("<!-- docket:dispatch:start (a) -->\nX\n<!-- docket:dispatch:end -->\n")},
		{"lessons", sectionFixture, InstructionsSectionLessons, []byte("lead\ntail\n")},
		{"only the block: no lessons", "<!-- docket:dispatch:start (a) -->\nX\n<!-- docket:dispatch:end -->\n",
			InstructionsSectionLessons, nil},
		{"no block: no dispatch", "just lessons\n", InstructionsSectionDispatch, nil},
		{"no block: whole file is lessons", "just lessons\n", InstructionsSectionLessons, []byte("just lessons\n")},
	}
	for _, c := range cases {
		got, err := SelectInstructionsSection([]byte(c.content), c.section)
		if err != nil {
			t.Errorf("%s: error %v", c.name, err)
			continue
		}
		if c.want == nil {
			if got != nil {
				t.Errorf("%s: got %q, want nil", c.name, got)
			}
			continue
		}
		if !bytes.Equal(got, c.want) {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}

	unbalanced := "lead\n<!-- docket:dispatch:start (a) -->\nX\n"
	for _, section := range []string{InstructionsSectionDispatch, InstructionsSectionLessons} {
		if got, err := SelectInstructionsSection([]byte(unbalanced), section); err == nil {
			t.Errorf("unbalanced markers, section %q: got %q, nil error; want an error", section, got)
		}
	}
	if got, err := SelectInstructionsSection([]byte(sectionFixture), "bogus"); err == nil {
		t.Errorf("unknown section: got %q, nil error; want an error", got)
	}
}

func TestClaudeSessionStartOutput(t *testing.T) {
	const content = "a\"b<!--\n"
	raw := ClaudeSessionStartOutput([]byte(content))
	if !bytes.HasSuffix(raw, []byte("\n")) || bytes.Count(raw, []byte("\n")) != 1 {
		t.Fatalf("output %q is not one JSON line", raw)
	}
	if !bytes.Contains(raw, []byte("<!--")) {
		t.Errorf("output %q escapes HTML; want the raw <!--", raw)
	}
	var doc struct {
		HookSpecificOutput struct {
			HookEventName     string `json:"hookEventName"`
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decode %q: %v", raw, err)
	}
	if doc.HookSpecificOutput.HookEventName != "SessionStart" || doc.HookSpecificOutput.AdditionalContext != content {
		t.Errorf("decoded %+v, want SessionStart / %q", doc, content)
	}
	for _, blank := range [][]byte{nil, []byte(" \n")} {
		if got := ClaudeSessionStartOutput(blank); got != nil {
			t.Errorf("ClaudeSessionStartOutput(%q) = %q, want nil", blank, got)
		}
	}
}

func TestCursorSessionStartOutput(t *testing.T) {
	const content = "a\"b<!--\n"
	raw := CursorSessionStartOutput([]byte(content))
	if !bytes.Contains(raw, []byte("<!--")) || bytes.Count(raw, []byte("\n")) != 1 {
		t.Errorf("output %q: want one line with the raw <!--", raw)
	}
	var doc struct {
		AdditionalContext string `json:"additional_context"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decode %q: %v", raw, err)
	}
	if doc.AdditionalContext != content {
		t.Errorf("additional_context = %q, want %q", doc.AdditionalContext, content)
	}
	for _, blank := range [][]byte{nil, []byte(" \n")} {
		if got := CursorSessionStartOutput(blank); got != nil {
			t.Errorf("CursorSessionStartOutput(%q) = %q, want nil", blank, got)
		}
	}
}

func TestCursorProjectDir(t *testing.T) {
	env := func(v string) func(string) string {
		return func(k string) string {
			if k == "CURSOR_PROJECT_DIR" {
				return v
			}
			return ""
		}
	}
	cases := []struct {
		name, envDir string
		stdin        *string
		want         string
	}{
		{"env wins over stdin", "/env/dir", ptr(`{"workspace_roots":["/w/a"]}`), "/env/dir"},
		{"first root", "", ptr(`{"workspace_roots":["/w/a","/w/b"]}`), "/w/a"},
		{"file URL", "", ptr(`{"workspace_roots":["file:///w/a"]}`), "/w/a"},
		{"garbage", "", ptr(`not json`), ""},
		{"empty object", "", ptr(`{}`), ""},
		{"relative root", "", ptr(`{"workspace_roots":["w/a"]}`), ""},
		{"nil stdin", "", nil, ""},
	}
	for _, c := range cases {
		var got string
		if c.stdin == nil {
			got = CursorProjectDir(env(c.envDir), nil)
		} else {
			got = CursorProjectDir(env(c.envDir), strings.NewReader(*c.stdin))
		}
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func ptr(s string) *string { return &s }

func TestInstructionsJSONResult(t *testing.T) {
	r := privateFixture(t, testsupport.TempDir(t), sectionFixture)
	res := Instructions(r, InstructionsSectionLessons)
	if res.Operation != "instructions" || res.Result != ResultApplied || !res.Private ||
		res.Section != InstructionsSectionLessons || res.Content != "lead\ntail\n" {
		t.Errorf("Instructions = %+v", res)
	}
	if res.HumanText() != "lead\ntail\n" {
		t.Errorf("HumanText = %q", res.HumanText())
	}
	bad := filepath.Join(testsupport.TempDir(t), "r")
	writeFixture(t, filepath.Join(bad, ".git", "dckt"), "file\n")
	if res := Instructions(bad, InstructionsSectionAll); res.Result != ResultExternalFailed || res.Failure == nil {
		t.Errorf("Instructions over a broken layout = %+v, want external-failed with a failure", res)
	}
}

// TestDispatchBlockFitsOneClaudeHook is acceptance 5: for every non-empty
// harness selection, the dispatch section as install renders it stays under
// Claude Code's 10,000-character per-hook cap.
func TestDispatchBlockFitsOneClaudeHook(t *testing.T) {
	rt, err := buildRunTracker()
	if err != nil {
		t.Fatal(err)
	}
	all := []string{"claude", "codex", "cursor", "opencode"}
	root := filepath.Join(testsupport.TempDir(t), "r")
	largest, largestSel, sawCodex := 0, "", false
	for mask := 1; mask < 1<<len(all); mask++ {
		var sel []string
		for i, h := range all {
			if mask&(1<<i) != 0 {
				sel = append(sel, h)
			}
		}
		targets, _, err := reposeed.PlanPrivate(reposeed.PrivatePlanInput{
			WorktreeRoot: root, CommonDir: filepath.Join(root, ".git"), Harnesses: sel, RunTracker: rt})
		if err != nil || len(targets) != 1 {
			t.Fatalf("PlanPrivate(%v) = %d targets, %v", sel, len(targets), err)
		}
		tg := targets[0]
		doc, err := document.Parse(nil)
		if err != nil {
			t.Fatal(err)
		}
		var ps document.PatchSet
		ps.InsertBlock(tg.BlockName, tg.Annotation, string(tg.Content), document.AtDocumentStart)
		rendered, err := doc.Apply(ps)
		if err != nil {
			t.Fatalf("render %v: %v", sel, err)
		}
		section, err := SelectInstructionsSection(rendered, InstructionsSectionDispatch)
		if err != nil {
			t.Fatalf("select %v: %v", sel, err)
		}
		if !strings.Contains(string(section), harness.DispatchHeading) {
			t.Fatalf("%v: dispatch section lacks %q; the size check would be vacuous", sel, harness.DispatchHeading)
		}
		if strings.Contains(string(section), "### Codex root-coordinator entry") {
			sawCodex = true
		}
		n := utf8.RuneCount(section)
		if n >= 10000 {
			t.Errorf("%v: dispatch section is %d characters; Claude Code caps one hook at 10,000", sel, n)
		}
		if n > largest {
			largest, largestSel = n, strings.Join(sel, ",")
		}
	}
	if !sawCodex {
		t.Fatal("no selection carried the Codex clause; the largest case was never measured")
	}
	t.Logf("largest dispatch section: %d characters ([%s])", largest, largestSel)
}
