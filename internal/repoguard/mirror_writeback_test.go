package repoguard

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// Change 0154 retired the GitHub board mirror from the skill bodies: the `github`
// board surface is classified unsupported and mutation-blocking by
// internal/config/capability.go (fenceBoardSurface refuses any transaction while a
// `github` token is present), so no skill may teach an agent to run the mirror and
// record its minted identifiers back into a change file or `.docket.yml`.
//
// This guard is the standing floor that keeps that recipe from creeping back. It is
// keyed on SYNTACTIC SHAPE, not an enumerated sentence (AGENTS.md: the spelling you
// miss is the target file's own house idiom): a line is an ACTIVE mirror write-back
// instruction when it names a minted mirror identifier AND carries a record-it-back
// obligation on the same line. Binding BOTH subject and obligation is deliberate — a
// purely historical or negative mention ("the retired mirror once printed
// issue-minted lines") names the subject without instructing a write-back and must
// stay admissible, so the guard cannot ossify into a closed-token ban.

// mirrorMintSubjectRe matches the mint vocabulary of the retired mirror in either
// house spelling it shipped in: the hyphenated `issue-minted`/`project-minted`
// report tokens and the `minted issue`/`minted project` line labels.
var mirrorMintSubjectRe = regexp.MustCompile(`(?i)(issue-minted|project-minted|minted issue|minted project)`)

// mirrorWriteBackRe matches the record-it-back obligation ("write the value back",
// "record back into", "writes its {owner, number} back") — a write/record verb
// followed within a short span by "back".
var mirrorWriteBackRe = regexp.MustCompile(`(?i)(record|writes?|write)[^.\n]{0,60}?back`)

// isActiveMirrorWriteBack reports whether one line instructs an active mirror
// write-back: it names a minted mirror identifier AND carries the record-it-back
// obligation. The non_vacuity subtest exercises this classifier directly, so the
// guard is proven live independent of the on-disk corpus.
func isActiveMirrorWriteBack(line string) bool {
	return mirrorMintSubjectRe.MatchString(line) && mirrorWriteBackRe.MatchString(line)
}

// mirrorWriteBackCorpus is the maintained skills/ markdown surface. Unlike the
// config-read-channel corpus it excludes nothing: the convention body must not teach
// a mirror write-back either.
func mirrorWriteBackCorpus(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	for _, rel := range maintainedPop(t, root) {
		if !strings.HasPrefix(rel, "skills/") || !hasExt(rel, ".md") {
			continue
		}
		out = append(out, rel)
	}
	return out
}

func TestNoActiveMirrorWriteBack(t *testing.T) {
	root := guardRoot(t)
	corpus := mirrorWriteBackCorpus(t, root)

	// Population floor: a glob that matches nothing, or a walk that swallows the
	// tree, must NOT read as a green (no-violations) result.
	if len(corpus) < 10 {
		t.Fatalf("population floor: only %d skill files scanned (expected >= 10)", len(corpus))
	}

	// THE RULE: no line under skills/ instructs an active mirror write-back.
	var violations []string
	for _, rel := range corpus {
		content := readMaintained(t, root, rel)
		for i, line := range strings.Split(content, "\n") {
			if isActiveMirrorWriteBack(line) {
				violations = append(violations, fmt.Sprintf("%s:%d: %s", rel, i+1, strings.TrimSpace(line)))
			}
		}
	}
	if len(violations) != 0 {
		t.Errorf("active mirror write-back instruction(s) (%d) — the GitHub mirror is retired (unsupported, mutation-blocking); remove the record-it-back recipe:\n%s",
			len(violations), strings.Join(violations, "\n"))
	}

	t.Run("non_vacuity", func(t *testing.T) {
		// Positive: the two retired-recipe spellings are both flagged, so a re-entry
		// of either would redden the rule above.
		positives := []string{
			"a fresh mint prints `issue-minted`/`project-minted` lines to record back into the change file / `.docket.yml`",
			"`minted issue <id> <n>` / `minted project <owner> <n>` lines — write the value back into the change file (`issue:`)",
			"first sync mints a Projects v2 board and writes its `{owner, number}` back into `.docket.yml` — a minted project record",
		}
		for _, p := range positives {
			if !isActiveMirrorWriteBack(p) {
				t.Errorf("classifier missed an active mirror write-back: %q", p)
			}
		}
		// Negative: subject WITHOUT a record-it-back obligation stays admissible (a
		// historical/negative mention is not an instruction), and an unrelated
		// write-back sentence with no mint subject is not flagged either.
		negatives := []string{
			"the retired github mirror once printed `issue-minted` lines; it no longer runs",
			"minted issue numbers are historical data preserved in the `issue:` frontmatter, never acted on",
			"stage by explicit path and write your own commit back to the branch",
		}
		for _, n := range negatives {
			if isActiveMirrorWriteBack(n) {
				t.Errorf("classifier over-fired on an admissible line: %q", n)
			}
		}
	})
}
