package repoguard

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/testsupport"
)

// TestLivingDocsAlignment keeps two kinds of drift out of docket's living,
// human-facing documentation (the roots in livingDocRoots; docs/release/ is
// deliberately excluded because dated release evidence lands there):
//
//  1. Citations of individual changes or PRs ("change 0363", "changes 0064/0084",
//     "change-0365", "pre-0051", "since 0392", "PR #344", "#0471"). ADR citations
//     are allowed. Matched case-insensitively on shape, with no exception list.
//  2. Configuration keys the schema registry marks unsupported (obsolete, inert,
//     inert-companion, deferred, deferred-active), derived from
//     config.SettingPaths at test time — never hand-listed.
//
// # Limits (pinned by non_vacuity, not merely stated)
//
//   - Citations inside fenced code blocks and inline code spans are not checked:
//     sample output and placeholder ids live there. A citation written inside a
//     code span therefore escapes.
//   - A four-digit "since"/"pre-" number is a citation only when zero-padded
//     (0NNN), so years ("since 2024") pass.
//   - A key is matched as a dotted path (`skills.build`, `learnings.cap`,
//     `agents.<h>.<a>.runner`), a dotted child of an all-unsupported block
//     (`skills.<role>`), a YAML key at a line start — commented, or commented
//     inside an already-commented block — or opening a code span (`skills:`,
//     `# terminal_publish:`, `#   # terminal_publish:`), or — inside a block
//     that also holds supported keys — an unsupported leaf key at a line start
//     (commented or nested-commented) or inside a flow mapping (`checkpoint:`,
//     `cap:`, `{ …, runner: … }`). The shapes come from
//     config.UnsupportedKeyShapes, shared with the example-config guard. A bare
//     mention without the dot or colon (`terminal_publish` in prose) is not
//     matched, and a wrapper's own `skills` frontmatter field must be described
//     without the `skills:` spelling. A markdown heading spelled
//     `## <unsupported-key>:` is matched too.
//   - Refused VALUES of supported keys (`finalize.gate: ci`, the `github` board
//     token) are out of reach; review catches those.
//   - An unterminated code fence is reported, so it cannot mask the rest of a file.
func TestLivingDocsAlignment(t *testing.T) {
	root := guardRoot(t)
	files, err := livingDocFiles(root, livingDocRoots)
	if err != nil {
		t.Fatalf("%v (fail closed)", err)
	}
	// Population floor: a renamed or emptied root must not let the guard pass
	// over a shrunken surface.
	const livingDocFloor = 42
	if len(files) < livingDocFloor {
		t.Fatalf("population floor: only %d living doc files scanned (expected >= %d)", len(files), livingDocFloor)
	}
	shapes := config.UnsupportedKeyShapes(config.SettingPaths())
	if len(shapes) == 0 {
		t.Fatalf("no unsupported-key shapes derived from the schema registry")
	}
	for _, rel := range files {
		content := readMaintained(t, root, rel)
		for _, v := range scanCitations(rel, content) {
			t.Error(v)
		}
		for _, v := range scanUnsupportedKeys(rel, content, shapes) {
			t.Error(v)
		}
	}

	t.Run("non_vacuity", func(t *testing.T) {
		for _, s := range []string{
			"see change 0363 for", "Changes 0064/0084 split it", "the change #502 work",
			"the change-0365 guard", "a pre-0051 repo", "since 0392 the install",
			"PR #344 landed", "tracked as (#0471).", "fixed in #502.", "see change\n0363",
		} {
			if len(scanCitations("x.md", s)) == 0 {
				t.Errorf("citation shape not flagged: %q", s)
			}
		}
		for _, s := range []string{
			"ADR-0071 decides it", "since 2024", "[step one](#1-capture)",
			"run `docket change show --id 0042`", "an em dash &#8212; here",
			"model `deepseek-v4-flash-0731`", "docs/changes/active/0412-<slug>.md",
			"```\nchange 0042\n```\n",
		} {
			if got := scanCitations("x.md", s); len(got) != 0 {
				t.Errorf("non-citation flagged: %q -> %v", s, got)
			}
		}
		if len(scanCitations("x.md", "intro\n```yaml\nkey: 1\n")) == 0 {
			t.Errorf("unterminated fence not reported")
		}

		for _, s := range []string{
			"bind `skills.build` to", "raise learnings.cap", "set `skills:` to",
			"skills:\n  build: x\n", "  terminal_publish: true", "# auto_groom: false",
			"#   # terminal_publish: true",
			"agents.claude.build-max.runner", "{ model: x, runner: codex }",
			"runners.codex.shim_model", "`dummy_mode.persona`", "build:\n  checkpoint: true\n",
			"the skills.<role> map",
		} {
			if len(scanUnsupportedKeys("x.md", s, shapes)) == 0 {
				t.Errorf("unsupported key not flagged: %q", s)
			}
		}
		for _, s := range []string{
			"auto_groomable: true", "two kinds of skills: workflow and worker",
			"finalize.gate: local", "board_surfaces: [inline]", "agents.claude.status.model",
			"learnings.enabled", "enabled: true", "docs/install/codex.md", "the skills.",
			"see docket-skills.md", "plan: docs/superpowers/plans/x.md",
			"build:\n  gate: local\n", "review:\n  max_fix_tasks: 10\n", "types: [feat]",
		} {
			if got := scanUnsupportedKeys("x.md", s, shapes); len(got) != 0 {
				t.Errorf("supported text flagged: %q -> %v", s, got)
			}
		}
		// Registry-derived population: every unsupported path, spelled as a
		// config key, is caught — so a new unsupported row is guarded with no
		// edit here.
		for _, p := range config.SettingPaths() {
			if p.Supported {
				continue
			}
			spelled := strings.ReplaceAll(p.Path, "*", "x")
			probe := "set `" + spelled + "` here"
			if !strings.Contains(p.Path, ".") {
				probe = spelled + ": x\n"
			}
			if len(scanUnsupportedKeys("x.md", probe, shapes)) == 0 {
				t.Errorf("registry path %s not caught by %q", p.Path, probe)
			}
		}

		tmp := testsupport.TempDir(t)
		for _, f := range []string{"a/one.md", "a/fixtures/skip.md", "a/testdata/skip.md", "a/notes.txt", "top.md", "b/notes.txt"} {
			if err := os.MkdirAll(filepath.Join(tmp, filepath.Dir(f)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(tmp, f), []byte("x\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		got, err := livingDocFiles(tmp, []string{"a", "top.md"})
		if err != nil || strings.Join(got, ",") != "a/one.md,top.md" {
			t.Errorf("livingDocFiles = %v, %v; want [a/one.md top.md]", got, err)
		}
		if _, err := livingDocFiles(tmp, []string{"missing"}); err == nil {
			t.Errorf("a missing root must fail closed")
		}
		if _, err := livingDocFiles(tmp, []string{"b"}); err == nil {
			t.Errorf("a root yielding no files must fail closed")
		}
	})
}

// livingDocRoots lists the living documentation the guard covers: files or
// directories, slash paths relative to the repo root. `docs/release/` is
// deliberately excluded (dated release evidence).
var livingDocRoots = []string{
	"README.md",
	"docs/README.md",
	"docs/guide",
	"docs/install",
	"docs/concepts",
	"docs/reference",
}

// livingDocFiles returns every .md file under roots (a root may be a file),
// sorted. Directories named "fixtures" or "testdata" below a root are pruned.
// A missing root, or a root that yields no .md file, is an error.
func livingDocFiles(base string, roots []string) ([]string, error) {
	var out []string
	for _, r := range roots {
		abs := filepath.Join(base, filepath.FromSlash(r))
		info, err := os.Stat(abs)
		if err != nil {
			return nil, fmt.Errorf("living doc root %s: %w", r, err)
		}
		before := len(out)
		if !info.IsDir() {
			out = append(out, r)
			continue
		}
		err = filepath.WalkDir(abs, func(p string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				if p != abs && (d.Name() == "fixtures" || d.Name() == "testdata") {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(d.Name(), ".md") {
				return nil
			}
			rel, rerr := filepath.Rel(base, p)
			if rerr != nil {
				return rerr
			}
			out = append(out, filepath.ToSlash(rel))
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk living doc root %s: %w", r, err)
		}
		if len(out) == before {
			return nil, fmt.Errorf("living doc root %s holds no .md file", r)
		}
	}
	sort.Strings(out)
	return out, nil
}

var citationShapes = []struct {
	name string
	re   *regexp.Regexp
}{
	{"change N", regexp.MustCompile(`(?i)\bchanges?\s+#?[0-9]{1,4}\b`)},
	{"change-N", regexp.MustCompile(`(?i)\bchange-[0-9]+\b`)},
	{"pre-NNNN", regexp.MustCompile(`(?i)\bpre-0[0-9]{3}\b`)},
	{"since NNNN", regexp.MustCompile(`(?i)\bsince\s+0[0-9]{3}\b`)},
	{"PR #N", regexp.MustCompile(`(?i)\bPRs?\s+#[0-9]+\b`)},
	{"#N", regexp.MustCompile(`(?:^|[^&\w/#])#[0-9]+(?:[^\w-]|$)`)},
}

var codeSpanRe = regexp.MustCompile("`[^`\n]*`")

// maskCode blanks fenced code blocks and inline code spans (keeping line
// structure) and reports whether a fence was left unterminated.
func maskCode(content string) (string, bool) {
	lines := strings.Split(content, "\n")
	inFence, marker := false, ""
	for i, ln := range lines {
		trimmed := strings.TrimLeft(ln, " \t")
		if inFence {
			if strings.HasPrefix(trimmed, marker) {
				inFence = false
			}
			lines[i] = ""
			continue
		}
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence, marker = true, trimmed[:3]
			lines[i] = ""
			continue
		}
		lines[i] = codeSpanRe.ReplaceAllStringFunc(ln, func(s string) string { return strings.Repeat(" ", len(s)) })
	}
	return strings.Join(lines, "\n"), inFence
}

func lineOf(content string, idx int) int { return strings.Count(content[:idx], "\n") + 1 }

// scanCitations reports every change/PR citation outside code.
func scanCitations(rel, content string) []string {
	masked, open := maskCode(content)
	var v []string
	if open {
		v = append(v, fmt.Sprintf("%s: unterminated code fence", rel))
	}
	for _, s := range citationShapes {
		for _, m := range s.re.FindAllStringIndex(masked, -1) {
			v = append(v, fmt.Sprintf("%s:%d: %s citation %q — cite no individual change or PR", rel, lineOf(masked, m[0]), s.name, strings.TrimSpace(masked[m[0]:m[1]])))
		}
	}
	return v
}

// scanUnsupportedKeys reports every unsupported config key spelled in content
// (code included: config examples live in code).
func scanUnsupportedKeys(rel, content string, shapes []config.UnsupportedKeyShape) []string {
	var v []string
	for _, s := range shapes {
		for _, m := range s.Re.FindAllStringIndex(content, -1) {
			v = append(v, fmt.Sprintf("%s:%d: unsupported config key %s (%q) — describe only supported settings", rel, lineOf(content, m[0]), s.Name, strings.TrimSpace(content[m[0]:m[1]])))
		}
	}
	return v
}
