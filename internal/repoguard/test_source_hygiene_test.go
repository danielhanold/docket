package repoguard

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// This file ports the STILL-MEANINGFUL half of the retired source-hygiene
// preflight (scripts/check-test-source-hygiene.sh, run by the Bash oracle and
// mirrored into internal/suiterunner as an exit-5 preflight) into a build-gate Go
// guard (change 0370, Task 6 disposition). Exit code 5 is retired from the runner
// contract; this guard replaces the preflight.
//
// # What survives, and why it is not mechanism-only
//
// The retired checker existed because a backtick in a shell TEST file runs when
// the shell READS the line — before any assert helper is called — so a
// verbatim-quoted guard anchor pasted into an assert description is not data, it
// is a command, and the test then reports green on output it produced itself
// (the 0212 incident: a multi-line double-quoted SITES="…" block whose anchor
// carried `git checkout .`). The surviving shell test surface still DEFINES
// assert-family helpers and executes shell (test_install_bootstrap.sh, the three
// posix-downloader suites, and the 31 test_go_*.sh wrappers that source
// tests/lib/go-integration-shard.sh), so the self-report hazard is a live
// property of maintained source, not a deleted-corpus idiom. This guard keeps
// that property; it fires on a backtick the shell would EXECUTE at source-read.
//
// # What was dropped as mechanism-only
//
// The retired checker's rule (a) — a BYTE-EXACT allowlist normalizing the ~88
// assert-family definitions of the legacy corpus — was calibrated to make rule
// (b)'s eval-detection trustworthy across a large heterogeneous tree. Over the
// now-small surviving surface it is not load-bearing, and a forward-ported
// byte-exact spelling allowlist would be exactly the spelling-pinned guard
// AGENTS.md forbids. It is not ported.
//
// # Population
//
// The scan population is the surviving runner's admitted shell surface: the
// *.sh files under tests/ that DECLARE a `# docket-suite:` category (the same
// contract internal/suiterunner discovery enforces). tests/fixtures is already
// categorically excluded by ExecutableSurface. Undeclared legacy files (deleted
// in Task 8) are out of population, which is why this guard is green during the
// Task 6→8 transitional window even though the legacy corpus is still on disk.
//
// # Stated limitations (byte-pattern-guard-matches-a-spelling: assert them here)
//
//   - The scanner carries quote state across lines (so the multi-line
//     double-quoted 0212 shape is caught) but does NOT model command
//     substitution `$(...)` frames or heredoc-delimiter quoting. A backtick in a
//     quoted-delimiter heredoc body (inert) would be a false positive; the
//     surviving suites contain none. An unquoted-heredoc-body backtick (a real
//     hazard) IS caught, as an ordinary code-position backtick.
//   - Single-quoted spans are treated as fully inert, so the retired
//     EVAL-BACKTICK sub-hazard — a backtick in a single-quoted assert CONDITION
//     that `eval "$2"` later re-runs — is NOT detected. The surviving house-style
//     suites carry no such construct (verified: no non-comment backtick exists in
//     any surviving suite file). The primary NORMAL- and DQ-BACKTICK hazards,
//     including the 0212 shape, ARE detected.
//   - tests/lib helpers are not themselves `# docket-suite:`-declared, so a
//     helper such as tests/lib/go-integration-shard.sh is out of population; it
//     is small, reviewed, and carries no code-position backtick.

// suiteHeaderLineRe mirrors internal/suiterunner's category-declaration contract
// exactly: the whole line is `# docket-suite: ` followed by one of the three
// categories and nothing else.
var suiteHeaderLineRe = regexp.MustCompile(`^# docket-suite: (go|posix-install|posix-downloader)$`)

// suiteHeaderScanLines bounds how far a declaration may sit, matching the runner.
const suiteHeaderScanLines = 10

// suiteSourceCorpus returns the maintained shell test files under tests/ that
// declare a `# docket-suite:` category — the surviving runner's admitted shell
// surface. Read fail-closed via readMaintained.
func suiteSourceCorpus(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	for _, rel := range execPop(t, root) {
		if !underDir(rel, "tests") || !hasExt(rel, ".sh") {
			continue
		}
		if declaresSuiteCategory(readMaintained(t, root, rel)) {
			out = append(out, rel)
		}
	}
	return out
}

// declaresSuiteCategory reports whether content carries an exact `# docket-suite:`
// header in its first suiteHeaderScanLines lines.
func declaresSuiteCategory(content string) bool {
	for i, line := range strings.Split(content, "\n") {
		if i >= suiteHeaderScanLines {
			break
		}
		if suiteHeaderLineRe.MatchString(line) {
			return true
		}
	}
	return false
}

// isShellWordStart reports whether prev (the previously scanned byte) leaves the
// scanner at the start of a word, which is the only position where `#` opens a
// comment. Start-of-file is modeled as prev == '\n'.
func isShellWordStart(prev byte) bool {
	switch prev {
	case '\n', ' ', '\t', ';', '|', '&', '(', ')', '<', '>', '{', '}':
		return true
	}
	return false
}

// scanExecutableBacktick returns the 1-based line numbers (deduped, at most one
// per line) carrying a backtick the shell would EXECUTE at source-read: a
// backtick in bare code position or inside double quotes (bare or
// backslash-escaped — the escape is consumed at source-eval and a bare backtick
// reaches eval). Backticks in a `#` comment or a single-quoted span are inert.
// See the file header for the stated limitations.
func scanExecutableBacktick(content string) []int {
	const (
		normal = iota
		singleQ
		doubleQ
	)
	state := normal
	line := 1
	var prev byte = '\n' // start-of-file behaves like just after a newline
	var hits []int
	seen := map[int]bool{}
	record := func() {
		if !seen[line] {
			seen[line] = true
			hits = append(hits, line)
		}
	}
	n := len(content)
	for i := 0; i < n; {
		c := content[i]
		if c == '\n' {
			line++
			prev = '\n'
			i++
			continue
		}
		switch state {
		case normal:
			switch c {
			case '\\':
				if i+1 < n && content[i+1] == '\n' {
					line++
					prev = '\n'
					i += 2
					continue
				}
				prev = 'x'
				i += 2 // escape the next char
				continue
			case '\'':
				state = singleQ
				prev = c
				i++
			case '"':
				state = doubleQ
				prev = c
				i++
			case '`':
				record()
				prev = c
				i++
			case '#':
				if isShellWordStart(prev) {
					for i < n && content[i] != '\n' {
						i++ // consume the comment; the newline is handled next loop
					}
					prev = '#'
					continue
				}
				prev = c
				i++
			default:
				prev = c
				i++
			}
		case singleQ:
			if c == '\'' {
				state = normal
			}
			prev = c
			i++
		case doubleQ:
			switch c {
			case '\\':
				if i+1 < n && content[i+1] == '`' {
					record() // escaped backtick still reaches eval
					prev = '`'
					i += 2
					continue
				}
				if i+1 < n && content[i+1] == '\n' {
					line++
					prev = '\n'
					i += 2
					continue
				}
				prev = 'x'
				i += 2
				continue
			case '`':
				record()
				prev = c
				i++
			case '"':
				state = normal
				prev = c
				i++
			default:
				prev = c
				i++
			}
		}
	}
	return hits
}

// TestNoExecutableBacktickInSuiteSource is the build-gate replacement for the
// retired exit-5 source-hygiene preflight (change 0370, Task 6 disposition).
func TestNoExecutableBacktickInSuiteSource(t *testing.T) {
	root := guardRoot(t)
	corpus := suiteSourceCorpus(t, root)
	// Population floor (marker-scoped-guard-needs-a-population-floor): the two
	// retained POSIX products (install + the downloader suites) are the durable
	// minimum; a collapse below that means the category filter or the walk broke,
	// which is a guard failure, not a clean pass.
	if len(corpus) < 4 {
		t.Fatalf("population floor: declared-category shell suite corpus collapsed to %d files (expected >= 4)", len(corpus))
	}
	var violations []string
	for _, rel := range corpus {
		for _, ln := range scanExecutableBacktick(readMaintained(t, root, rel)) {
			violations = append(violations, fmt.Sprintf("%s:%d: backtick in executable position — the shell would run it at source-read; carry the text in single quotes or a quoted-delimiter heredoc", rel, ln))
		}
	}
	if len(violations) != 0 {
		t.Errorf("executable-backtick violations in declared suite source:\n%s", strings.Join(violations, "\n"))
	}

	t.Run("non_vacuity", func(t *testing.T) {
		bt := "\x60"
		bad := []string{
			"x=" + bt + "date" + bt,                          // NORMAL-BACKTICK, bare code position
			"echo \"" + bt + "whoami" + bt + "\"",            // DQ-BACKTICK, bare inside double quotes
			"echo \"\\" + bt + "cmd\\" + bt + "\"",           // DQ-BACKTICK, backslash-escaped inside double quotes
			"assert \"d\" \"[ " + bt + "id -u" + bt + " ]\"", // DQ backtick inside an assert argument
		}
		for _, b := range bad {
			if got := scanExecutableBacktick(b); len(got) == 0 {
				t.Errorf("scanner missed an executable backtick: %q", b)
			}
		}
		good := []string{
			"# a comment mentioning " + bt + "docket.sh" + bt + " is inert",
			"echo 'a single-quoted " + bt + "literal" + bt + "'",
			"echo \"$(date)\"", // command substitution, no backtick
			"printf '%s' \"$var\"",
			"cmd # trailing " + bt + "note" + bt,
			"assert 'go is on PATH' 'command -v go >/dev/null 2>&1'",
		}
		for _, g := range good {
			if got := scanExecutableBacktick(g); len(got) != 0 {
				t.Errorf("scanner wrongly flagged inert content %q at lines %v", g, got)
			}
		}
		// Cross-line double-quoted span (the 0212 shape): a backtick two lines
		// into an unclosed double-quoted assignment is still executable.
		multiline := "SITES=\"\nfirst line\n" + bt + "git checkout ." + bt + "\n\""
		if got := scanExecutableBacktick(multiline); len(got) == 0 {
			t.Errorf("scanner missed the multi-line double-quoted 0212 shape: %q", multiline)
		}
		// Line-number accuracy: the multi-line hit is on line 3.
		if got := scanExecutableBacktick(multiline); len(got) != 1 || got[0] != 3 {
			t.Errorf("multi-line hit line = %v, want [3]", got)
		}
	})
}
