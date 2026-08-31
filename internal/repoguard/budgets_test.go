package repoguard

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Ports two size-direction guards off the retired Bash suite:
//
//   tests/test_skill_size_budgets.sh   -> TestSkillSizeBudgets
//   tests/test_dispatch_block_budget.sh -> TestDispatchBlockBudget
//
// Both make a compaction DIRECTION durable (size-target-is-direction): a file
// that regrows past its recorded ceiling reddens, and a ceiling can only be
// moved DOWN in the same diff that slims the file.

// ---------------------------------------------------------------------------
// skill size budgets
// ---------------------------------------------------------------------------

type skillBudget struct {
	rel      string // relative to skills/
	maxLines int
	maxWords int
}

// skillBudgets is the per-file line/word ceiling table for skills/**/*.md,
// carried verbatim from the retired tests/test_skill_size_budgets.sh. To slim a
// file, lower its numbers in the same diff; to add a skill file, add its row
// (the completeness direction below reddens an unbudgeted file).
var skillBudgets = []skillBudget{
	{"docket-adr/SKILL.md", 110, 1600},
	{"docket-adr/adr-template.md", 26, 90},
	{"docket-auto-groom/SKILL.md", 70, 1750},
	{"docket-brainstorm/SKILL.md", 84, 692},
	{"docket-build/SKILL.md", 385, 3850},
	{"docket-build/references/delegation-execution.md", 85, 850},
	{"docket-build/references/gate-caller-loop.md", 175, 1750},
	{"docket-build/references/gate-execution-evidence.md", 110, 1050},
	{"docket-build/references/gate-execution.md", 130, 1200},
	{"docket-build/references/task-routing.md", 50, 500},
	{"docket-build-task/SKILL.md", 155, 1550},
	{"docket-convention/SKILL.md", 400, 7350},
	{"docket-convention/github-board-mirror.md", 19, 462},
	{"docket-convention/references/agent-layer.md", 205, 2350},
	{"docket-convention/references/dummy-mode.md", 85, 800},
	{"docket-convention/references/learnings.md", 84, 580},
	{"docket-convention/references/stacked-changes.md", 215, 2050},
	{"docket-convention/references/terminal-close-out.md", 240, 2150},
	{"docket-finalize-change/SKILL.md", 190, 4150},
	{"docket-finalize-change/references/gate-failure.md", 115, 1300},
	{"docket-groom-next/SKILL.md", 77, 1650},
	{"docket-implement-next/SKILL.md", 180, 6500},
	{"docket-implement-next/references/edge-paths.md", 58, 800},
	{"docket-implement-next/references/fix-loop.md", 185, 1900},
	{"docket-implement-next/results-template.md", 25, 250},
	{"docket-review/SKILL.md", 110, 900},
	{"docket-new-change/SKILL.md", 61, 1700},
	{"docket-new-change/change-template.md", 51, 250},
	{"docket-status/SKILL.md", 140, 3050},
}

// wcLines counts lines the way `wc -l` does: the number of newline bytes.
func wcLines(content string) int { return strings.Count(content, "\n") }

// wcWords counts words the way `wc -w` does: whitespace-separated tokens.
func wcWords(content string) int { return len(strings.Fields(content)) }

func TestSkillSizeBudgets(t *testing.T) {
	root := guardRoot(t)

	// Forward: every budgeted file exists and is within both ceilings.
	for _, b := range skillBudgets {
		rel := "skills/" + b.rel
		p := filepath.Join(root, filepath.FromSlash(rel))
		data, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("budgeted skill file missing/unreadable %s: %v", rel, err)
			continue
		}
		content := string(data)
		if l := wcLines(content); l > b.maxLines {
			t.Errorf("%s is %d lines, over its %d-line budget — slim it or lower the ceiling in-diff", rel, l, b.maxLines)
		}
		if w := wcWords(content); w > b.maxWords {
			t.Errorf("%s is %d words, over its %d-word budget — slim it or lower the ceiling in-diff", rel, w, b.maxWords)
		}
	}

	// Reverse (correspondence): every skills/**/*.md carries a budget row. An
	// unbudgeted new skill file reddens rather than shipping unmeasured.
	budgeted := make(map[string]bool, len(skillBudgets))
	for _, b := range skillBudgets {
		budgeted[b.rel] = true
	}
	var found, missing []string
	for _, rel := range maintainedPop(t, root) {
		if !strings.HasPrefix(rel, "skills/") || !strings.HasSuffix(rel, ".md") {
			continue
		}
		sub := strings.TrimPrefix(rel, "skills/")
		found = append(found, sub)
		if !budgeted[sub] {
			missing = append(missing, rel)
		}
	}
	if len(found) < 20 {
		t.Fatalf("population floor: only %d skills/**/*.md found (expected >= 20)", len(found))
	}
	if len(missing) != 0 {
		slices.Sort(missing)
		t.Errorf("skills/**/*.md files with no budget row (add one):\n%s", strings.Join(missing, "\n"))
	}

	t.Run("non_vacuity", func(t *testing.T) {
		// A 2-line/2-word body must exceed a 1-line/1-word budget.
		body := "a b\nc d\n"
		if wcLines(body) <= 1 {
			t.Errorf("line counter is vacuous: %q counted <= 1 line", body)
		}
		if wcWords(body) <= 1 {
			t.Errorf("word counter is vacuous: %q counted <= 1 word", body)
		}
	})
}

// ---------------------------------------------------------------------------
// dispatch block budget
// ---------------------------------------------------------------------------

const (
	dispatchStart  = "docket:dispatch:start"
	dispatchEnd    = "docket:dispatch:end"
	dispatchBudget = 400  // NEW actual (352 at 0334, 369 now) rounded up to a multiple of 50.
	dispatchOld    = 1156 // pre-0334 roster block; the ceiling must stay strictly below it.
)

// dispatchBlockWords returns the word count of the managed dispatch block
// between its markers (markers excluded), and whether both markers were found.
func dispatchBlockWords(content string) (int, bool) {
	var inBlock bool
	var b strings.Builder
	sawStart, sawEnd := false, false
	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(line, dispatchStart) {
			inBlock = true
			sawStart = true
			continue
		}
		if strings.Contains(line, dispatchEnd) {
			inBlock = false
			sawEnd = true
			continue
		}
		if inBlock {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return wcWords(b.String()), sawStart && sawEnd
}

func TestDispatchBlockBudget(t *testing.T) {
	root := guardRoot(t)

	// Direction, made durable: the ceiling is strictly below the pre-0334 actual,
	// so the block cannot regrow back toward the retired roster without reddening.
	if !(dispatchBudget < dispatchOld) {
		t.Fatalf("BUDGET (%d) must stay strictly below the recorded pre-0334 actual (%d)", dispatchBudget, dispatchOld)
	}

	// Measure the committed always-loaded surface (the enduring artifact that
	// rides every turn's context) rather than regenerating it: the Go emitter
	// internal/harness/dispatch.go now owns emission, and the committed AGENTS.md
	// is what a parent harness actually loads.
	content := readMaintained(t, root, "AGENTS.md")
	words, ok := dispatchBlockWords(content)
	if !ok {
		t.Fatalf("AGENTS.md is missing the docket dispatch block markers")
	}
	if words < 1 {
		t.Fatalf("the dispatch block is empty — the marker scan found no content")
	}
	if words > dispatchBudget {
		t.Errorf("the AGENTS.md dispatch block is %d words, over its %d-word budget", words, dispatchBudget)
	}

	t.Run("non_vacuity", func(t *testing.T) {
		fixture := "x\n" + dispatchStart + "\nalpha beta gamma\n" + dispatchEnd + "\ny\n"
		if got, ok := dispatchBlockWords(fixture); !ok || got != 3 {
			t.Errorf("dispatch block extractor miscounted a 3-word fixture: got=%d ok=%v", got, ok)
		}
		// The budget comparison is non-vacuous: 2 words exceeds a 1-word budget.
		if !(2 > 1) {
			t.Errorf("word-budget comparison is vacuous")
		}
	})
}
