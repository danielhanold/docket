# Single-Source Board Vocabularies Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make status and priority vocabularies single-sourced, derive every set-shaped consumer from those sources, and pin exhaustive mappings against them in both directions.

**Architecture:** Extend the existing sourceable `scripts/lib/docket-frontmatter.sh` vocabulary block with priority constants and pure membership/rank helpers, preserving the landed `BOARD_CHECK_IDS` array. Convert six consumer scripts to call those helpers or iterate the arrays; retain genuine mappings as `case` statements and protect them with non-vacuous set-equality guards.

**Tech Stack:** Bash 3/4-compatible shell scripts, associative arrays already used by the repository, grep/awk/sed-based shell tests, git.

## Global Constraints

- Preserve `BOARD_CHECK_IDS` and change 0111's four-way correspondence guard unchanged in meaning.
- Preserve all board, digest, readiness, and archive output bytes except the two specified changes: GitHub Project status options become `in-progress,proposed,blocked,deferred,implemented`, and invalid-priority findings list `critical high medium low`.
- Single-status predicates remain explicit and out of scope; only set enumerations are derived.
- Every mapping guard must assert extractor cardinality before set equality and must redden when a real arm is removed or a phantom arm is added.
- The two column-zero CLI contract comments containing literal `done|killed` remain exempt from the executable-literal sentinel.
- Baseline note: the full suite has one unrelated pre-existing pipefail failure at `tests/test_docket_config.sh` R7; captured as change 0129. All change-0116 tests and every other full-suite test must pass.

---

### Task 1: Declare and prove the shared vocabularies

**Files:**
- Modify: `scripts/lib/docket-frontmatter.sh`
- Modify: `tests/test_docket_frontmatter.sh`

**Interfaces:**
- Consumes: existing `DOCKET_STATUSES_ACTIVE`, `DOCKET_STATUSES_TERMINAL`, `DOCKET_STATUSES`, and `BOARD_CHECK_IDS`.
- Produces: `DOCKET_PRIORITIES`, `DOCKET_PRIORITY_DEFAULT`, `docket_status_is_active`, `docket_status_is_terminal`, `docket_priority_is_member`, and `docket_priority_rank`.

- [x] **Step 1: Add failing vocabulary and helper tests**

Append assertions that source the real library and exercise membership, strict empty handling, default rank, unknown-value rank, order, cardinality, and default membership:

```bash
assert "DOCKET_PRIORITIES is rank-ordered critical > high > medium > low" \
  '[ "${DOCKET_PRIORITIES[*]}" = "critical high medium low" ]'
assert "DOCKET_PRIORITIES has exactly four members" '[ "${#DOCKET_PRIORITIES[@]}" = 4 ]'
assert "DOCKET_PRIORITY_DEFAULT is a declared priority" \
  'docket_priority_is_member "$DOCKET_PRIORITY_DEFAULT"'
assert "active helper accepts proposed" 'docket_status_is_active proposed'
assert "active helper rejects terminal done" '! docket_status_is_active done'
assert "active helper rejects empty" '! docket_status_is_active ""'
assert "terminal helper accepts killed" 'docket_status_is_terminal killed'
assert "terminal helper rejects active implemented" '! docket_status_is_terminal implemented'
assert "terminal helper rejects empty" '! docket_status_is_terminal ""'
assert "priority membership accepts high" 'docket_priority_is_member high'
assert "priority membership rejects empty" '! docket_priority_is_member ""'
assert "priority membership rejects unknown" '! docket_priority_is_member urgent'
assert "priority rank derives critical as zero" '[ "$(docket_priority_rank critical)" = 0 ]'
assert "priority rank derives low as three" '[ "$(docket_priority_rank low)" = 3 ]'
assert "priority rank defaults empty to medium's index" '[ "$(docket_priority_rank "")" = 2 ]'
assert "priority rank defaults unknown to medium's index" '[ "$(docket_priority_rank urgent)" = 2 ]'
```

- [x] **Step 2: Run the helper test and verify it fails for missing symbols**

Run: `bash tests/test_docket_frontmatter.sh`

Expected: `NOT OK` assertions naming `DOCKET_PRIORITIES` and the four undefined helper functions, followed by `FAIL`.

- [x] **Step 3: Add the priority vocabulary and pure helper implementations**

Update the library header to call the file a shared frontmatter, dependency-resolution, and vocabulary helper. Extend the existing vocabulary block without moving or rewriting `BOARD_CHECK_IDS`:

```bash
DOCKET_STATUSES_ACTIVE=(in-progress proposed blocked deferred implemented)
DOCKET_STATUSES_TERMINAL=(done killed)
DOCKET_STATUSES=("${DOCKET_STATUSES_ACTIVE[@]}" "${DOCKET_STATUSES_TERMINAL[@]}")

DOCKET_PRIORITIES=(critical high medium low)
DOCKET_PRIORITY_DEFAULT=medium

_docket_array_has(){
  local needle="$1"; shift
  local value
  [ -n "$needle" ] || return 1
  for value in "$@"; do [ "$needle" = "$value" ] && return 0; done
  return 1
}
docket_status_is_active(){ _docket_array_has "$1" "${DOCKET_STATUSES_ACTIVE[@]}"; }
docket_status_is_terminal(){ _docket_array_has "$1" "${DOCKET_STATUSES_TERMINAL[@]}"; }
docket_priority_is_member(){ _docket_array_has "$1" "${DOCKET_PRIORITIES[@]}"; }
docket_priority_rank(){
  local wanted="$1" value i=0
  docket_priority_is_member "$wanted" || wanted="$DOCKET_PRIORITY_DEFAULT"
  for value in "${DOCKET_PRIORITIES[@]}"; do
    [ "$wanted" = "$value" ] && { printf '%s' "$i"; return 0; }
    i=$(( i + 1 ))
  done
  return 1
}
```

- [x] **Step 4: Run focused tests and mutation-check both helper directions**

Run: `bash tests/test_docket_frontmatter.sh`

Expected: `PASS`.

Temporarily remove `killed` from `DOCKET_STATUSES_TERMINAL`; rerun and confirm the terminal-helper assertion fails. Restore it. Temporarily append `urgent` to `DOCKET_PRIORITIES`; rerun and confirm the exact-order/cardinality assertions fail. Restore it and rerun to `PASS`.

- [x] **Step 5: Commit the shared vocabulary**

```bash
git add scripts/lib/docket-frontmatter.sh tests/test_docket_frontmatter.sh
git commit -m "feat(0116): add shared board vocabulary helpers"
```

### Task 2: Derive every set-shaped consumer

**Files:**
- Modify: `scripts/render-board.sh`
- Modify: `scripts/board-checks.sh`
- Modify: `scripts/github-mirror.sh`
- Modify: `scripts/archive-change.sh`
- Modify: `scripts/terminal-publish.sh`
- Modify: `scripts/docket-status.sh`
- Modify: `tests/test_render_board.sh`
- Modify: `tests/test_board_checks.sh`
- Modify: `tests/test_github_mirror.sh`
- Modify: `tests/test_closeout.sh`
- Modify: `tests/test_terminal_publish.sh`
- Modify: `tests/test_docket_status.sh`

**Interfaces:**
- Consumes: Task 1's arrays and helper functions.
- Produces: no hand-written executable terminal-status pair, a priority rank derived from array order, active-section iteration driven by `DOCKET_STATUSES_ACTIVE`, and archive summary composition driven by `DOCKET_STATUSES_TERMINAL`.

- [ ] **Step 1: Add failing consumer assertions before changing production code**

Add producer-anchored assertions to the appropriate test files:

```bash
assert "render-board derives ready priority rank from the shared helper" \
  'grep -qF -- '\''prank="$(docket_priority_rank "$(field "$f" priority)")"'\'' "$SCRIPT"'
assert "board-checks derives priority membership from the shared helper" \
  'grep -qF -- '\''docket_priority_is_member "$fd_priority"'\'' "$SCRIPT"'
assert "github Project options derive from active statuses after the library source" \
  '[ "$(grep -nF '\''STATUS_OPTIONS="$(IFS=,; printf '\''\''%s'\''\'' "${DOCKET_STATUSES_ACTIVE[*]}")"'\'' "$SCRIPT" | cut -d: -f1)" -gt "$(grep -nF '\''source "$(dirname "${BASH_SOURCE[0]}")/lib/docket-frontmatter.sh"'\'' "$SCRIPT" | cut -d: -f1)" ]'
```

Add behavior assertions for the two intentional output changes:

```bash
assert "invalid priority message follows shared rank order" \
  'grep -qF -- "not one of: critical high medium low (empty = medium)" <<<"$out"'
assert "Project status options follow active board order" \
  'grep -qF -- "--single-select-options in-progress,proposed,blocked,deferred,implemented" "$gh_log"'
```

- [ ] **Step 2: Run the consumer tests and verify the new assertions fail**

Run:

```bash
bash tests/test_render_board.sh
bash tests/test_board_checks.sh
bash tests/test_github_mirror.sh
bash tests/test_closeout.sh
bash tests/test_terminal_publish.sh
bash tests/test_docket_status.sh
```

Expected: the new producer/ordering assertions fail on the hand-written ladders and pairs; existing behavior tests remain green.

- [ ] **Step 3: Convert renderer counts, ranking, sections, and archive composition**

In both backlog-count loops use the terminal helper:

```bash
if docket_status_is_terminal "$st"; then n=${ARC_COUNT[$st]:-0}
else n="$(count_of "$st")"
fi
```

Replace the priority ladder with:

```bash
prank="$(docket_priority_rank "$(field "$f" priority)")"
```

Add the sparse suffix mapping and derive the section calls:

```bash
suffix_for(){ case "$1" in implemented) printf ' — awaiting merge' ;; esac; }
for st in "${DOCKET_STATUSES_ACTIVE[@]}"; do
  print_section "$st" "$(suffix_for "$st")"
done
```

Replace the archive gate/count/label hand-list with:

```bash
archive_count=0; em=""; lbl=""
for st in "${DOCKET_STATUSES_TERMINAL[@]}"; do
  n=${ARC_COUNT[$st]:-0}
  archive_count=$(( archive_count + n ))
  [ "$n" -gt 0 ] || continue
  em+="$(emoji_for "$st")"
  [ -n "$lbl" ] && lbl+=" + $st" || lbl="$st"
done
if [ "$archive_count" -gt 0 ]; then
  printf '\n<details><summary>%s Archive — %s (%d)</summary>\n\n' "$em" "$lbl" "$archive_count"
```

Keep the archive row's `[ "$st" = done ]` collapse predicate explicit because it is a single-status behavior, not a vocabulary enumeration.

- [ ] **Step 4: Convert checker, mirror, archive, publish, and sweep consumers**

In `board-checks.sh`, replace `renders_row`'s loop with `docket_status_is_active "$rr_st"`, and replace priority validation with:

```bash
if [ -n "$fd_priority" ] && ! docket_priority_is_member "$fd_priority"; then
  emit field-domain "$cid" "priority '$fd_priority' is not one of: ${DOCKET_PRIORITIES[*]} (empty = $DOCKET_PRIORITY_DEFAULT)"
fi
```

Move the Project options assignment below the library `source` in `github-mirror.sh`:

```bash
STATUS_OPTIONS="$(IFS=,; printf '%s' "${DOCKET_STATUSES_ACTIVE[*]}")"
```

Keep the two close-reason arms as a mapping. Replace the Project item skip set with:

```bash
[ -n "$st" ] || continue
docket_status_is_terminal "$st" && continue
```

Replace each terminal outcome/idempotence validator in `archive-change.sh`, `terminal-publish.sh`, and `docket-status.sh` with the shared helper while retaining each caller's existing diagnostic and empty-value handling, for example:

```bash
docket_status_is_terminal "$OUTCOME" || die "missing/invalid --outcome (done|killed)"
```

- [ ] **Step 5: Run focused suites and compare behavior**

Run the six commands from Step 2 plus `bash tests/test_docket_frontmatter.sh`.

Expected: all seven scripts print `PASS`; golden output changes are limited to Project option order and priority diagnostic order. Verify `git diff -- tests` contains no unrelated golden re-blessing.

- [ ] **Step 6: Commit the derived consumers**

```bash
git add scripts/render-board.sh scripts/board-checks.sh scripts/github-mirror.sh scripts/archive-change.sh scripts/terminal-publish.sh scripts/docket-status.sh tests/test_render_board.sh tests/test_board_checks.sh tests/test_github_mirror.sh tests/test_closeout.sh tests/test_terminal_publish.sh tests/test_docket_status.sh
git commit -m "refactor(0116): derive board vocabulary consumers"
```

### Task 3: Pin exhaustive mappings and prove the guard net

**Files:**
- Modify: `scripts/render-board.sh`
- Modify: `tests/test_render_board.sh`
- Modify: `tests/test_github_mirror.sh`

**Interfaces:**
- Consumes: the shared arrays and the Task 2 consumer shapes.
- Produces: exact set-equality guards for the table-header, row-format, and issue-close mappings, plus a producer-anchored sentinel against executable `done|killed` restatements.

- [ ] **Step 1: Add failing non-vacuous mapping extractors and set comparisons**

Extract the row-format arms by their syntactic shape, not by every `word)` token in the body:

```bash
row_format_labels(){
  awk '
    /# row_format_mapping$/ { inb=1; next }
    inb && /^[[:space:]]{6}[a-z][a-z-]*\)/ {
      line=$0; sub(/^[[:space:]]+/, "", line); sub(/\).*/, "", line); print line
    }
    inb && /esac/ { exit }
  ' "$SCRIPT" | sort -u
}
```

Add exact cardinality then equality assertions for `table_header_for` and `row_format_labels`, using `DOCKET_STATUSES_ACTIVE`; add the same pattern in `tests/test_github_mirror.sh` for the issue-close mapping against `DOCKET_STATUSES_TERMINAL`. The close-mapping extractor must anchor on a named marker comment immediately above that `case`, not scan the sparse `readiness_label` mapping.

Add a sentinel that derives the converted executable files by whole-repo search, excludes only the two column-zero CLI usage comments, and requires zero executable `done|killed` pairs. Capture grep output before testing it; never use a producer-to-`grep -q` pipeline under `pipefail`.

- [ ] **Step 2: Run mapping tests and verify they fail on missing extraction anchors**

Run:

```bash
bash tests/test_render_board.sh
bash tests/test_github_mirror.sh
```

Expected: failures name `table_header_for`, the row-format marker, and the issue-close marker; extractor cardinality asserts fail before the set comparisons are trusted.

- [ ] **Step 3: Extract the renderer header mapping and mark the retained mappings**

Move only the table-header `case` into the one-line-header form required by the proven tokenizer:

```bash
table_header_for(){ case "$1" in
  in-progress) printf '| # | Title | Priority | Spec | Branch |\n|---|-------|----------|------|--------|\n' ;;
  proposed)    printf '| # | Title | Priority | Readiness |\n|---|-------|----------|-----------|\n' ;;
  blocked)     printf '| # | Title | Priority | Blocked by |\n|---|-------|----------|------------|\n' ;;
  deferred)    printf '| # | Title | Priority |\n|---|-------|----------|\n' ;;
  implemented) printf '| # | Title | Priority | PR | Readiness |\n|---|-------|----------|----|-----------|\n' ;;
esac; }
```

Call `table_header_for "$st"` from `print_section`. Put `# row_format_mapping` immediately above the retained row-format `case`. Put `# terminal_close_reason_mapping` immediately above the retained `github-mirror.sh` close-reason `case`. Document beside the tests that exhaustive mappings are pinned, sparse mappings such as `suffix_for` and `readiness_label` are intentionally not pinned, and an un-arrayed exhaustive vocabulary receives an array first.

- [ ] **Step 4: Run focused tests and mutation-test both correspondence directions**

Run the two focused suites; expected `PASS`.

Perform and restore each mutation independently:

1. Remove the `deferred)` arm from `table_header_for`; the table-header exact-count/equality guard must fail.
2. Add `retired)` to `table_header_for`; the same set-equality guard must fail.
3. Remove the `blocked)` row-format arm; the row-format guard must fail.
4. Add a `retired)` row-format arm; the row-format guard must fail.
5. Remove the `killed)` issue-close arm; the terminal mapping guard must fail.
6. Add a `retired)` issue-close arm; the terminal mapping guard must fail.
7. Restore every mutation and rerun both suites to `PASS`.

- [ ] **Step 5: Run the whole suite and audit the expected baseline exception**

Run:

```bash
for test_file in tests/test_*.sh; do bash "$test_file" || exit 1; done
```

Expected: every change-0116-related suite passes. If the already-captured change-0129 pipefail assertion remains red, rerun `tests/test_docket_config.sh` on unmodified `origin/main` to confirm the same R7 failure and record it as the sole baseline exception; any other failure blocks completion.

Then run:

```bash
git diff --check origin/main...HEAD
git status --short
```

Expected: no whitespace errors; only the plan and intended source/test files are changed.

- [ ] **Step 6: Mark the plan executed and commit the guard net**

Change every completed checkbox in this plan from `[ ]` to `[x]`, then:

```bash
git add scripts/render-board.sh tests/test_render_board.sh tests/test_github_mirror.sh docs/superpowers/plans/2026-07-22-single-source-board-vocabularies.md
git commit -m "test(0116): pin exhaustive board mappings"
```

The docket workflow records the total-vs-sparse mapping rule as an ADR after whole-branch review; the ADR is metadata and must not be authored in this feature worktree.
