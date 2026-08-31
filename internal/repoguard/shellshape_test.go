package repoguard

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// This file ports the maintained-source SHELL-SHAPE guards (change 0370, Gate 2)
// off the retired Bash suite:
//
//   tests/test_grep_portability.sh  -> TestGrepPortability
//   tests/test_pipe_shapes.sh       -> TestPipeShapes
//   tests/test_bsd_tool_defaults.sh -> TestBSDToolDefaults
//
// Each keys on a syntactic SHAPE, bounded on both sides, and scans the population
// where that shape is load-bearing. Go's regexp is RE2, the same engine the
// portability rule protects a POSIX/BSD grep from lacking, so the patterns are
// re-expressed rather than copied byte-for-byte; the enforced property is
// identical.

// ---------------------------------------------------------------------------
// Shared corpus helpers
// ---------------------------------------------------------------------------

// shellPortabilityCorpus is where grep/sed/awk ERE portability is load-bearing:
// the executable surface (*.sh/*.bash, exec-bit files, and the scripts/+skills/
// command-markdown that carries runnable grep recipes) plus the always-loaded
// rule files AGENTS.md / CLAUDE.md, which state and demonstrate the grep rules.
//
// LIMITATION (byte-pattern-guard-matches-a-spelling): Go source is deliberately
// out of scope. The retired Bash guard scanned *.go for the word-boundary class
// and exempted it from the interval class; that exemption existed precisely
// because Go's own regexp engine (RE2) supports \b and large {n,m} intervals
// natively, so a \b or {0,999} in Go is correct, not a portability defect. The
// portability property therefore governs the shell-tool surface only.
func shellPortabilityCorpus(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	for _, rel := range execPop(t, root) {
		out = append(out, rel)
	}
	for _, rel := range []string{"AGENTS.md", "CLAUDE.md"} {
		for _, m := range maintainedPop(t, root) {
			if m == rel {
				out = append(out, rel)
			}
		}
	}
	return out
}

// shellScriptCorpus is the *.sh/*.bash executable surface — the surface whose
// bytes a shell parses as pipelines.
func shellScriptCorpus(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	for _, rel := range execPop(t, root) {
		if hasExt(rel, ".sh", ".bash") {
			out = append(out, rel)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// grep_portability: no ERE interval bound > 255, no \b/\</\> word boundary
// ---------------------------------------------------------------------------

// intervalRe matches an ERE {m}/{m,}/{m,n} or its BRE \{...\} spelling, capturing
// the two numeric bounds so the > 255 test is arithmetic, not textual.
var intervalRe = regexp.MustCompile(`\\?\{([0-9]+)(?:,([0-9]*))?\\?\}`)

// wordBoundaryRe matches two literal backslashes followed by b/</>, the
// double-backslash spelling of a \b/\</\> word boundary that a POSIX/BSD or
// git-grep ERE silently treats as zero-width-nothing (returns no match, no error).
var wordBoundaryRe = regexp.MustCompile(`\\\\[b<>]`)

const intervalMaxBound = 255

func scanInterval(rel, content string) []string {
	var v []string
	for i, line := range strings.Split(content, "\n") {
		for _, m := range intervalRe.FindAllStringSubmatch(line, -1) {
			over := false
			for _, g := range []string{m[1], m[2]} {
				if g == "" {
					continue
				}
				if n, err := strconv.Atoi(g); err == nil && n > intervalMaxBound {
					over = true
				}
			}
			if over {
				v = append(v, fmt.Sprintf("%s:%d: interval bound > %d: %s", rel, i+1, intervalMaxBound, m[0]))
			}
		}
	}
	return v
}

func scanWordBoundary(rel, content string) []string {
	var v []string
	for i, line := range strings.Split(content, "\n") {
		if wordBoundaryRe.MatchString(line) {
			v = append(v, fmt.Sprintf("%s:%d: \\b/\\<\\> word boundary is not portable ERE: %s", rel, i+1, strings.TrimSpace(line)))
		}
	}
	return v
}

func TestGrepPortability(t *testing.T) {
	root := guardRoot(t)
	corpus := shellPortabilityCorpus(t, root)
	if len(corpus) < 30 {
		t.Fatalf("population floor: grep-portability corpus collapsed to %d files (expected >= 30)", len(corpus))
	}
	var violations []string
	for _, rel := range corpus {
		c := readMaintained(t, root, rel)
		violations = append(violations, scanInterval(rel, c)...)
		violations = append(violations, scanWordBoundary(rel, c)...)
	}
	if len(violations) != 0 {
		t.Errorf("grep-portability violations:\n%s", strings.Join(violations, "\n"))
	}

	t.Run("non_vacuity", func(t *testing.T) {
		// The interval scanner fires on > 255 and only > 255.
		if got := scanInterval("x.sh", "grep -E '.{0,600}'"); len(got) == 0 {
			t.Errorf("interval scanner missed a {0,600} bound")
		}
		if got := scanInterval("x.sh", `grep -E '\{0,600\}'`); len(got) == 0 {
			t.Errorf("interval scanner missed a BRE \\{0,600\\} bound")
		}
		if got := scanInterval("x.sh", "grep -E '.{0,256}'"); len(got) == 0 {
			t.Errorf("interval scanner missed the boundary {0,256}")
		}
		if got := scanInterval("x.sh", "grep -E '.{0,255}'"); len(got) != 0 {
			t.Errorf("interval scanner wrongly flagged the legal boundary {0,255}: %v", got)
		}
		if got := scanInterval("x.sh", "printf '%s' \"${VAR}\" && awk '{print}'"); len(got) != 0 {
			t.Errorf("interval scanner wrongly flagged ${VAR}/awk action: %v", got)
		}
		// The word-boundary scanner fires only on the two-backslash spelling.
		if got := scanWordBoundary("x.sh", `grep -E '\\bword'`); len(got) == 0 {
			t.Errorf("word-boundary scanner missed a \\\\b")
		}
		if got := scanWordBoundary("x.sh", `grep -E '\bword'`); len(got) != 0 {
			t.Errorf("word-boundary scanner wrongly flagged a single-backslash \\b: %v", got)
		}
	})
}

// ---------------------------------------------------------------------------
// pipe_shapes: no producer | early-exiting-consumer on one line
// ---------------------------------------------------------------------------

var (
	pipeCPfx  = `([^[:space:]|]*/)?`
	pipeCGrep = `grep[[:space:]]+(-[A-Za-z]+[[:space:]]+)*(-[A-Za-z]*q|-m|--quiet|--silent|--max-count)`
	pipeCHead = `head([[:space:]]+-|[[:space:]]*$)`
	pipeCAwk  = `awk[[:space:]][^|]*[^[:alnum:]_]exit([^[:alnum:]_]|$)`
	pipeCSed  = `sed[[:space:]][^|]*[^[:alpha:]_/^|-]q([^[:alnum:]_]|$)`
	pipeCRead = `([A-Za-z_][A-Za-z0-9_]*=[^[:space:]]*[[:space:]]+)*read([[:space:]]|$)`

	pipeBadShape = regexp.MustCompile(
		`(^|[^|])[|][[:space:]]*` + pipeCPfx +
			`(` + pipeCGrep + `|` + pipeCHead + `|` + pipeCAwk + `|` + pipeCSed + `|` + pipeCRead + `)`)

	// awk END{...} actions are blanked before matching: an `exit` inside an END
	// block runs after the input is drained and cannot take a SIGPIPE.
	pipeEndAction = regexp.MustCompile(`END[[:space:]]*\{[^{}]*\}`)
	pipeComment   = regexp.MustCompile(`^[[:space:]]*#`)
)

func scanPipeShapes(rel, content string) []string {
	var v []string
	for i, line := range strings.Split(content, "\n") {
		if pipeComment.MatchString(line) {
			continue
		}
		probe := pipeEndAction.ReplaceAllString(line, " ")
		if pipeBadShape.MatchString(probe) {
			v = append(v, fmt.Sprintf("%s:%d: producer | early-exiting consumer: %s", rel, i+1, strings.TrimSpace(line)))
		}
	}
	return v
}

func TestPipeShapes(t *testing.T) {
	root := guardRoot(t)
	corpus := shellScriptCorpus(t, root)
	if len(corpus) < 30 {
		t.Fatalf("population floor: pipe-shape corpus collapsed to %d shell files (expected >= 30)", len(corpus))
	}
	var violations []string
	for _, rel := range corpus {
		violations = append(violations, scanPipeShapes(rel, readMaintained(t, root, rel))...)
	}
	if len(violations) != 0 {
		t.Errorf("pipe-shape violations:\n%s", strings.Join(violations, "\n"))
	}

	t.Run("non_vacuity", func(t *testing.T) {
		bar := "|"
		bad := []string{
			"git show HEAD " + bar + " grep -q foo",
			"printf '%s' x " + bar + " grep -qF bar",
			"cmd " + bar + " grep -Eqi baz",
			"cmd " + bar + " head -n1",
			"cmd " + bar + " grep --max-count=1 x",
			"cmd " + bar + " /usr/bin/grep -q x",
			"cmd " + bar + " awk '/x/{print; exit}'",
			"cmd " + bar + " sed 1q",
			"cmd " + bar + " read -r x",
			"cmd " + bar + " IFS= read -r x",
		}
		for _, b := range bad {
			if got := scanPipeShapes("x.sh", b); len(got) == 0 {
				t.Errorf("pipe scanner missed a bad shape: %q", b)
			}
		}
		good := []string{
			"grep -q foo <<<\"$var\"",
			"cmd " + bar + " grep foo >/dev/null",
			"cmd " + bar + " sed -n 1p",
			"cmd " + bar + " awk 'END{exit !ok}'",
			"cmd " + bar + " while read -r x; do :; done",
			"a " + bar + bar + " b",
			"# prose mentioning head and grep -q in a comment",
		}
		for _, g := range good {
			if got := scanPipeShapes("x.sh", g); len(got) != 0 {
				t.Errorf("pipe scanner wrongly flagged a safe shape %q: %v", g, got)
			}
		}
		// A line carrying BOTH an END exit and a real early exit must still fire.
		mixed := "cmd " + bar + " awk 'END{exit 1} /x/{exit}'"
		if got := scanPipeShapes("x.sh", mixed); len(got) == 0 {
			t.Errorf("pipe scanner missed a mixed END+early-exit awk: %q", mixed)
		}
	})
}

// ---------------------------------------------------------------------------
// bsd_tool_defaults: mv that replaces a file must pass -f; mktemp must template
// ---------------------------------------------------------------------------

var (
	bsdMvLine  = regexp.MustCompile(`(^mv[[:space:]])|([^-[:alnum:]_./]mv[[:space:]])`)
	bsdMvAllow = regexp.MustCompile(`mv[[:space:]]+-f([^-[:alnum:]_]|$)`)
	bsdGitMv   = regexp.MustCompile(`(git|\$GIT)[^|;&]* mv `)
	bsdMvDeny  = regexp.MustCompile(`mv[[:space:]]+-[in][[:space:]]`)
	bsdComment = regexp.MustCompile(`^[[:space:]]*#`)

	bsdMktemp   = regexp.MustCompile(`\$\([[:space:]]*mktemp`)
	bsdTemplate = regexp.MustCompile(`XXXXXX`)
)

func scanBadMv(rel, content string) []string {
	var v []string
	for i, line := range strings.Split(content, "\n") {
		if bsdComment.MatchString(line) {
			continue
		}
		deny := bsdMvDeny.MatchString(line)
		base := bsdMvLine.MatchString(line) && !bsdMvAllow.MatchString(line) && !bsdGitMv.MatchString(line)
		if deny || base {
			v = append(v, fmt.Sprintf("%s:%d: mv that can prompt (needs -f): %s", rel, i+1, strings.TrimSpace(line)))
		}
	}
	return v
}

func scanBadMktemp(rel, content string) []string {
	var v []string
	for i, line := range strings.Split(content, "\n") {
		if bsdComment.MatchString(line) {
			continue
		}
		if bsdMktemp.MatchString(line) && !bsdTemplate.MatchString(line) {
			v = append(v, fmt.Sprintf("%s:%d: $(mktemp) without an explicit template: %s", rel, i+1, strings.TrimSpace(line)))
		}
	}
	return v
}

// bsdCorpus is the executable surface MINUS tests/ (the retired Bash suite owns
// its own hygiene and embeds these defects as runtime-assembled fixtures).
func bsdCorpus(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	for _, rel := range execPop(t, root) {
		if underDir(rel, "tests") {
			continue
		}
		out = append(out, rel)
	}
	return out
}

func TestBSDToolDefaults(t *testing.T) {
	root := guardRoot(t)
	corpus := bsdCorpus(t, root)
	if len(corpus) < 20 {
		t.Fatalf("population floor: bsd-defaults corpus collapsed to %d files (expected >= 20)", len(corpus))
	}
	var mvV, mkV []string
	for _, rel := range corpus {
		c := readMaintained(t, root, rel)
		mvV = append(mvV, scanBadMv(rel, c)...)
		mkV = append(mkV, scanBadMktemp(rel, c)...)
	}
	if len(mvV) != 0 {
		t.Errorf("mv-without-f violations:\n%s", strings.Join(mvV, "\n"))
	}
	if len(mkV) != 0 {
		t.Errorf("mktemp-without-template violations:\n%s", strings.Join(mkV, "\n"))
	}

	t.Run("non_vacuity", func(t *testing.T) {
		bad := []string{
			`mv "$t" "$f"`,
			`mv $t $f`,
			`  mv -i "$a" "$b"`,
			`out=$(mktemp)`,
			`d=$( mktemp -d )`,
		}
		for _, b := range bad {
			mv := scanBadMv("x.sh", b)
			mk := scanBadMktemp("x.sh", b)
			if len(mv) == 0 && len(mk) == 0 {
				t.Errorf("bsd scanner missed a defect: %q", b)
			}
		}
		good := []string{
			`mv -f "$t" "$f"`,
			`git mv -k "$a" "$b"`,
			`$GIT mv "$a" "$b"`,
			"out=$(mktemp \"${TMPDIR:-/tmp}/x.XXXXXX\")",
			"a code span `mv -f` in prose",
		}
		for _, g := range good {
			if mv := scanBadMv("x.sh", g); len(mv) != 0 {
				t.Errorf("bsd mv scanner wrongly flagged %q: %v", g, mv)
			}
			if mk := scanBadMktemp("x.sh", g); len(mk) != 0 {
				t.Errorf("bsd mktemp scanner wrongly flagged %q: %v", g, mk)
			}
		}
	})
}
