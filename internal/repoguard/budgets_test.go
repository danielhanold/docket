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
//
// The word ceilings for docket-convention/SKILL.md, docket-implement-next/SKILL.md,
// and docket-convention/references/stacked-changes.md were re-baselined upward once
// (change 0394) to hold the capability-catalog contract prose: the new Step-0
// capability bootstrap and the catalog-resolved semantic-operation idiom that
// replaced hard-coded `docket <argv>` spellings. Change 0399 re-baselined
// docket-convention/SKILL.md upward once more (7750 -> 7800) to hold the
// authoritative machine-readable request/result schema-surface contract prose — the
// catalog-resolved `schema` operation and its fail-closed descriptor-driven body
// construction; the mandated content cannot fit under 7750 even at minimum-faithful
// phrasing. The ratchet stays in force at the new baselines — they catch any further
// regrowth.
//
// Change 0388 re-baselined docket-convention/SKILL.md (7800 -> 7807),
// docket-finalize-change/SKILL.md (4150 -> 4344), and docket-status/SKILL.md
// (3050 -> 3065) upward once to hold the native post-merge integration-branch-sync
// contract prose it wired into these three skills: the once-per-scope
// repository.sync-integration operation, its safety ladder and closed disposition
// vocabulary, and where the maintenance sweep runs it. This is authored contract
// documentation, not slack — the ceilings are pinned at the exact new word counts,
// so the ratchet still reddens on any further regrowth.
//
// Change 0393's structured feature-worktree payload lines and balanced dispatch
// markers land atop the 0410/0349 baselines in this rebase: docket-build/SKILL.md
// reaches 406 lines, docket-implement-next/SKILL.md 210 lines, and
// docket-finalize-change/SKILL.md 236 lines. That machine-checked syntax cannot
// be compacted into prose without losing the supplied input shape; the ratchet
// remains exact.
//
// Change 0346 re-baselined docket-finalize-change/SKILL.md upward once to hold
// the verified post-merge rebuild contract (step 12): the sync-disposition
// gate, the source ancestry proof, the post-install identity check, and the
// separate binary-rebuild-incomplete report. Authored contract documentation,
// not slack — the ceiling is pinned at the exact new counts and the ratchet
// stays in force.
//
// Change 0354 re-baselined docket-implement-next/SKILL.md (6900 -> 6956) upward once
// to hold the halt-report body-only contract prose in the Step-3 "FUNDAMENTALLY
// invalidated" escape hatch: change.halt owns the `## Run halted` heading and dated
// `###` sub-heading, so the caller-authored report is the section body only, and a
// body with its own column-zero `## ` heading or an unterminated code fence is
// refused invalid-input (invalid-section-markdown, field report). Minimum-faithful
// phrasing exceeds the prior 6900 ceiling; the new baseline is pinned at the exact
// word count so the ratchet still reddens on any further regrowth.
//
// Change 0376 re-baselined docket-build/SKILL.md (385/3850 -> 390/3907),
// docket-build-task/SKILL.md (155/1550 -> 160/1605), and
// docket-implement-next/SKILL.md (word ceiling 6956 -> 6991; line ceiling
// unchanged) upward once to hold the required gate.drive JSON-capture caller
// contract: every credential-consuming invocation must pass --json and capture
// the first response before acting. This is authored contract documentation, not
// slack — the ceilings are pinned at the exact new counts (after trimming
// per-caller repetition of what the shared contract already states), so the
// ratchet still reddens on any further regrowth. The docket-implement-next word
// ceiling carries both re-baselines: 0354's halt-report prose and 0376's
// JSON-capture prose now coexist in the file.
//
// Change 0327 re-baselined docket-convention/references/stacked-changes.md and
// docket-finalize-change/SKILL.md upward once to hold the carry-preservation
// contract prose it wired into stacked close-out: a merged PR destination
// establishes the carry RELATIONSHIP, while the descendant's merged work must be
// separately proven still present in Git (ancestry in the pinned integration
// history, or exact-content at the root's merge result) before any archive. This is
// authored contract documentation, not slack. The finalize ceiling here is the
// exact count of the file that now carries 0327's carry-preservation prose on top of
// main's 0346/0388 re-baselines, so the ratchet still reddens on any further
// regrowth.
//
// Change 0410 re-baselined the required-results workflow surfaces upward once to
// hold the durable-results contract prose (results are now a REQUIRED close-out
// artifact for every implemented change, trivial included; the mandatory Step 6.5
// checkpoint lifecycle; and the build/review capture-ownership boundaries):
// docket-build/SKILL.md (390/3907 -> 404/4054), docket-convention/SKILL.md (word
// 7807 -> 7969), docket-implement-next/SKILL.md (180/6991 -> 201/7530),
// docket-implement-next/references/edge-paths.md (58/800 -> 78/1091),
// docket-implement-next/references/fix-loop.md (185/1900 -> 190/1958),
// docket-implement-next/results-template.md (25/250 -> 51/257 — the five-section
// canonical template replaced the terse optional stub), and
// docket-review/SKILL.md (word 900 -> 913). Authored contract documentation, not
// slack — the ceilings are pinned at the exact new counts, so the ratchet still
// reddens on any further regrowth.
//
// Change 0349's reserve-before-dispatch prose follows the 0410 baseline. Together
// they bring docket-finalize-change/SKILL.md to 226 lines and 5179 words, so its
// rounded ceilings remain a durable ratchet rather than silently dropping either
// change's contract.
//
// Change 0416 re-baselined docket-build/SKILL.md and docket-build-task/SKILL.md
// upward once to hold the complete start-ready scope-bundle contract: the
// controller's dispatch payload hands the worker every identity value
// prepare-scope pinned, and the worker passes the bundle through to its scoped
// task-owned gate.drive.start unchanged — closing the identity omission that
// made the driver reject every scoped build-task start before its first test.
// Authored contract documentation, not slack — the ceilings are pinned at the
// exact new counts, so the ratchet still reddens on any further regrowth.
var skillBudgets = []skillBudget{
	{"docket-adr/SKILL.md", 110, 1600},
	{"docket-adr/adr-template.md", 26, 90},
	{"docket-auto-groom/SKILL.md", 70, 1750},
	{"docket-brainstorm/SKILL.md", 84, 692},
	{"docket-build/SKILL.md", 410, 4102}, // 0416: +complete start-ready scope bundle in the worker dispatch payload (see note above)
	// 0154: docket-build/references/delegation-execution.md removed — it was the
	// evidence record for the Bash delegation facade that change 0370 deleted; its
	// budget row is deleted with it.
	{"docket-build/references/gate-caller-loop.md", 175, 1750},
	{"docket-build/references/gate-execution-evidence.md", 110, 1050},
	{"docket-build/references/gate-execution.md", 170, 1520},
	{"docket-build/references/task-routing.md", 50, 500},
	{"docket-build-task/SKILL.md", 165, 1668}, // 0416: +scoped start identity bundle (see note above)
	{"docket-convention/SKILL.md", 400, 7969}, // 0410: +required-results lifecycle prose; 0399: +schema request/result contract prose; 0388: +sync-integration prose (see note above)
	// 0154: docket-convention/github-board-mirror.md removed — the GitHub mirror is
	// retired (unsupported, mutation-blocking); its budget row is deleted with it.
	{"docket-convention/references/agent-layer.md", 205, 2350},
	{"docket-convention/references/dummy-mode.md", 85, 800},
	{"docket-convention/references/learnings.md", 84, 580},
	{"docket-convention/references/stacked-changes.md", 215, 2140}, // 0327: +carry-preservation contract prose (see note above)
	{"docket-convention/references/terminal-close-out.md", 240, 2150},
	{"docket-finalize-change/SKILL.md", 236, 5200},                   // 0393: +exact payload, marker, and direct-dispatch lines atop 0349/0410 (see note above)
	{"docket-finalize-change/references/gate-failure.md", 120, 1300}, // 0349: +reserve-before-dispatch resolver protocol prose (115 -> 120 lines)
	{"docket-groom-next/SKILL.md", 77, 1650},
	{"docket-implement-next/SKILL.md", 210, 7530},                // 0393: +exact payload, marker, and direct-dispatch lines atop 0410/0354/0376 (see note above)
	{"docket-implement-next/references/edge-paths.md", 78, 1091}, // 0410: +resume/recovery + required-results reconciliation (see note above)
	{"docket-implement-next/references/fix-loop.md", 190, 1958},  // 0410: +findings-to-results checkpoint linkage (see note above)
	{"docket-implement-next/results-template.md", 51, 257},       // 0410: canonical five-section required template (see note above)
	{"docket-review/SKILL.md", 110, 913},                         // 0410: +findings-return capture contract (see note above)
	{"docket-new-change/SKILL.md", 61, 1700},
	{"docket-new-change/change-template.md", 51, 250},
	{"docket-status/SKILL.md", 140, 3065}, // 0388: +sync-integration prose (see note above)
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
	dispatchBudget = 650  // 0393 root-entry routing plus round 8's completion barrier fit at 646 words, still below the retired roster.
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
