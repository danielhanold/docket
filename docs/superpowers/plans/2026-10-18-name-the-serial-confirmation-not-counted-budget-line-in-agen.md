<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0558 — Point the budget-report rule in AGENTS.md at tests/README.md instead of listing the lines](../../changes/archive/2026-10-18-0558-name-the-serial-confirmation-not-counted-budget-line-in-agen.md)**
<!-- docket:backlink:end -->

# Point the budget-report rule in AGENTS.md at tests/README.md — Implementation Plan

> **For agentic workers:** this plan is executed by `docket-build`, one task at a time. Steps use
> checkbox (`- [ ]`) syntax for tracking.

**Goal:** Shrink the AGENTS.md budget-report bullet to a reminder plus a pointer, and make
`tests/README.md` the one place that sorts all five budget lines into screening / breach / needs
nothing; trim the `tests/runtime-budgets.tsv` header copy to a pointer and add `OVER BUDGET:` to the
glossary.

**Architecture:** Docs-only. Four maintained files change; no code, no runner output, no tests are
added. Verification is `git grep` checks plus the whole-suite build gate.

**Tech Stack:** Markdown, a TSV registry's `#` comment header.

**Spec:** `docs/superpowers/specs/2026-10-18-name-the-serial-confirmation-not-counted-budget-line-in-agen-design.md`
(on the metadata branch; the feature branch carries a copy).

## Global Constraints

- Touch only `AGENTS.md`, `tests/README.md`, `tests/runtime-budgets.tsv`, `docs/reference/glossary.md`.
- Never edit `CLAUDE.md` separately: it is a symlink to `AGENTS.md`.
- Maintained docs describe current behavior only; add no change numbers or PR citations.
- No change to the runner, its output wording, or which lines exist. No new guard test.
- `.github/workflows/release-candidate.yml` and point-in-time records (plans, specs, results,
  release records) stay untouched.
- In `tests/runtime-budgets.tsv`, edit `#` comment lines only, never a data row.
- The five line prefixes, verbatim: `BUDGET WATCH:`, `PARALLEL-SENSITIVE:`,
  `SERIAL CONFIRMED OVER BUDGET:`, `OVER BUDGET:`, `SERIAL CONFIRMATION NOT COUNTED:`.
- Do not rename the glossary heading `### Budget watch / serially confirmed breach` (its anchor is
  linked from the glossary index).

## Review Focus

- The new AGENTS.md bullet promises the README sorts every line into one of three kinds; a reviewer
  should find all five prefixes in the README budget paragraph, each next to its kind word. (Task 1
  Step 3 checks this.)
- A `#`-comment rewrap in `tests/runtime-budgets.tsv` that accidentally drops the leading `# ` turns
  prose into a malformed row; `TestRuntimeBudgetsCorrespondence` must stay green. (Task 2 Step 3.)
- `OVER BUDGET:` is a substring of `SERIAL CONFIRMED OVER BUDGET:`; a grep for absence must use the
  broad pattern so neither survives in AGENTS.md or the TSV. (Task 1 Step 2, Task 2 Step 2.)
- A rewrap of the AGENTS.md bullet must keep the two-space continuation indent of its neighbors.
- The README edit must keep every existing fact (thresholds `ceiling * 5/2` and `ceiling * 3/2`,
  repeated-overrun scheduling, the quiet-run rule, single-job / `serial` direct measurement).

---

### Task 1: AGENTS.md pointer and README classification

These two land together: the new bullet makes a promise only the README edit keeps.

**Files:**
- Modify: `AGENTS.md` — the *Guards and tests* bullet beginning "Read the budget report even on a
  green run:"
- Modify: `tests/README.md` — the two bullets after "Budgets are checked two ways." in *Running it*

**Interfaces:**
- Consumes: nothing.
- Produces: the README budget paragraph is the authoritative classification that Task 2's pointers
  name.

- [ ] **Step 1: Replace the AGENTS.md bullet**

Replace the whole five-line bullet with exactly this text (the spec fixes it):

```markdown
- Read the budget report even on a green run: no budget line fails the run, so nothing else
  will catch a real breach. `tests/README.md` says which lines are screening notes, which are
  breaches to act on, and which need nothing.
```

- [ ] **Step 2: Verify AGENTS.md names no budget line**

Run: `git -C <worktree> grep -nE "BUDGET WATCH|PARALLEL-SENSITIVE|OVER BUDGET|NOT COUNTED" -- AGENTS.md`
Expected: no output, exit 1.

- [ ] **Step 3: Edit the README budget bullets**

Add only classification words; keep every existing fact and the overall shape. Required outcome
(builder picks the wording):

- `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` stay called (advisory) screening findings/notes.
- `SERIAL CONFIRMED OVER BUDGET:` stays "an authoritative breach" and gains "to act on".
- `OVER BUDGET:` (second bullet) is explicitly called an authoritative breach to act on — e.g.
  "a crossing prints `OVER BUDGET:`, also an authoritative breach to act on".
- After the two breach kinds, say acting on one means bringing the file back under its ceiling and
  point at the "**Never grow a file past its budget and raise the number.**" paragraph in *Where new
  tests go* rather than restating it.
- `SERIAL CONFIRMATION NOT COUNTED:` is said to need no action, because the runner re-checks the
  test on the next quiet run.

Check: read the edited paragraph and confirm each of the five prefixes appears next to its kind
(screening / breach to act on / needs nothing), and that `ceiling * 5/2`, `ceiling * 3/2`,
`DOCKET_RUNTESTS_JOBS=1`, and `serial` are all still present.

- [ ] **Step 4: Commit**

```bash
git add AGENTS.md tests/README.md
git commit -m "docs: point the AGENTS.md budget-report rule at tests/README.md"
```

### Task 2: runtime-budgets.tsv header pointer and glossary `OVER BUDGET:`

**Files:**
- Modify: `tests/runtime-budgets.tsv` — the `# ENFORCEMENT ...` comment paragraph in the header
- Modify: `docs/reference/glossary.md` — entry `### Budget watch / serially confirmed breach`

**Interfaces:**
- Consumes: Task 1's README budget paragraph (the pointer target).
- Produces: nothing.

- [ ] **Step 1: Cut the TSV header's line list to a pointer**

In the *ENFORCEMENT* paragraph, replace the parenthetical "(BUDGET WATCH / PARALLEL-SENSITIVE are
screening findings; SERIAL CONFIRMED OVER BUDGET is an authoritative breach)" with a pointer such as
"(tests/README.md says what each budget line means)". Rewrap within the paragraph; every line keeps
its leading `# `. Leave all other header sentences, including the existing change numbers, as they
are.

- [ ] **Step 2: Verify the TSV names no budget line**

Run: `git -C <worktree> grep -nE "BUDGET WATCH|PARALLEL-SENSITIVE|OVER BUDGET|NOT COUNTED" -- tests/runtime-budgets.tsv`
Expected: no output, exit 1.

- [ ] **Step 3: Verify the registry guard still passes**

Run (from the worktree): `go test ./internal/repoguard/ -run TestRuntimeBudgetsCorrespondence -count=1`
Expected: `ok`.

- [ ] **Step 4: Add `OVER BUDGET:` to the glossary entry**

After the sentence ending "`SERIAL CONFIRMED OVER BUDGET:` is an authoritative breach to act on.",
add one clause/sentence saying an `OVER BUDGET:` line, measured directly by a single-job run or for
a `serial` budget row, is also an authoritative breach to act on. Keep the heading and the rest of
the entry unchanged.

- [ ] **Step 5: Commit**

```bash
git add tests/runtime-budgets.tsv docs/reference/glossary.md
git commit -m "docs: point the budget registry header at tests/README.md and name OVER BUDGET in the glossary"
```

### Task 3: Whole-change verification

**Files:** none modified.

- [ ] **Step 1: Confirm the diff scope**

Run: `git -C <worktree> diff --name-only <pre-build-base>..HEAD`
Expected: only `AGENTS.md`, `tests/README.md`, `tests/runtime-budgets.tsv`,
`docs/reference/glossary.md` (plus the spec copy, if the branch carries it).

- [ ] **Step 2: Whole-suite build gate**

Run the command `build.test_command` resolves to (currently `go run ./cmd/docket development test`)
from the worktree. Expected: exit `0`. Read the budget report even though it is green. No test reads
these files' budget wording, so any red is unrelated and handled the usual way.
