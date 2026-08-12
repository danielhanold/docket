<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0298 — Stacked changes — build a new change on top of a parent change's branch](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0298-stacked-changes-build-a-new-change-on-top-of-a-parent-change.md)**
<!-- docket:backlink:end -->

# Stacked Changes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a docket change declare `stacked_on: <parent id>` so it is built on the parent's unmerged feature branch, reviewed as its own PR against that branch, merged into the parent, and promoted to `done` only when the stack root's code reaches the integration branch.

**Architecture:** One new sourced library (`scripts/lib/docket-stack.sh`) owns the three shared routines — the effective-base resolver, the descendant-graph scan, and chain validation — and two new executables expose them (`scripts/stack-base.sh` for the resolver, `scripts/stack-closeout.sh` for the idempotent stack close-out). A new non-terminal lifecycle status `stacked-merged` joins the existing status vocabulary array, so every derived view (board, digest, GitHub mirror, health checks) picks it up through the arrays that already drive them. Skill bodies gain only trigger lines; the mechanics live in a new `references/stacked-changes.md` read on trigger.

**Tech Stack:** POSIX-ish Bash (GNU bash 4+ at runtime via `$DOCKET_BASH_PATH`), `awk`/`sed`/`grep` for text processing, `git` plumbing, `gh` for PR state. Tests are standalone Bash files under `tests/`, run by `scripts/run-tests.sh`.

## Global Constraints

Copy these verbatim into every task's working assumptions. They come from the repo's always-in-context rules (`AGENTS.md`), the spec, and the learnings ledger.

- **Governing invariant (spec):** `done` means the change's code is reachable from the integration branch. A child merged only into its parent is NOT `done`.
- **Shell:** never `producer | early-exiting-consumer` (`grep -q`, `head`) under `set -o pipefail` — capture into a variable first, then `grep <<<"$var"`.
- **Shell:** `grep` for a pattern leading with `--` must declare it: `grep -E -e "<pat>"` or `grep -qF -- "<pat>"`.
- **Shell:** awk indent classes are `[^[:space:]]`, never `[^ ]`.
- **Shell:** always `mv -f` on install/replace paths. Always pass a template to `mktemp`: `"${TMPDIR:-/tmp}/<name>.XXXXXX"` — unless the temp file must sit beside its destination for a same-filesystem atomic rename, in which case template it there.
- **Shell:** never redirect a renderer straight into the file it generates — render to a temp file, gate on exit status AND non-empty output, then `mv -f`.
- **Ids:** docket displays zero-padded 4-digit ids, so ids arrive padded. Canonicalize at the argument boundary with `id=$(( 10#$raw ))` — bash reads a leading `0` as octal and `printf %d 0237` is 159, silently. Match the precedent in `scripts/board-checks.sh` and `scripts/adr-checks.sh`.
- **Frontmatter reads:** `stacked_on:` is an **optional** key, so every read of it uses the anchored `fm_field` (or `fm_field_raw`), never `field`. The selection rule is the canonical table in the header of `scripts/lib/docket-frontmatter.sh`. The census guard in `tests/test_frontmatter_read_shapes.sh` fails closed on an unknown `(script, file-argument token)` pair, so every new script that reads change files must be added to its `CORPUS_MAP`.
- **Frontmatter writes:** anchor any field edit to the first `---…---` block, never a bare column-0 line match.
- **YAML:** quote any scalar carrying a colon-space, trailing colon, ` #`, a leading indicator character, or a boolean keyword. `stacked_on:` is an **integer scalar**, not a flow collection — do not bracket it.
- **Guards are code:** mutation-test every new assert — strip the thing it guards and watch it redden. A mutation that leaves an assert green is a defect until proven otherwise.
- **Guards:** key on syntactic shape, never an enumerated list of spellings. Never hand-list the sites of a literal you are gating — derive them from a whole-repo grep.
- **Cross-references:** anchor on a symbol name or a verbatim-quoted clause, never a line number. `tests/test_comment_anchor_style.sh` rejects the filename-plus-line-number form.
- **Vocabulary mappings (ADR-0055):** every exhaustive `case` over the status vocabulary must be pinned by exact set equality against the array, with an extractor-cardinality non-vacuity assert, mutation-tested in both directions.
- **Test suite:** run the whole suite at the build gate via `scripts/run-tests.sh`. A trailing `OVER BUDGET:` line is a finding to act on, not noise.
- **Test placement:** extend the existing topical shard if `tests/runtime-budgets.tsv` has room; start a sibling shard when it does not. No budget row may exceed 60s, and `EXPECTED_TOTAL` in `tests/test_runtime_budgets.sh` pins the sum of all ceilings — every new row must re-seed it, with the rationale written into the TSV header comment.
- **Test source:** backticks in test source execute when the shell reaches that line. Carry verbatim anchors in single quotes or `<<'EOF'` heredocs.
- **Assert helper:** there is no shared assertion library. Each test file defines its own `assert`, and `scripts/check-test-source-hygiene.sh` enforces that the definition is byte-exact one of six canonical forms. Use exactly:
  `assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }`
- **Script contracts:** every `scripts/<name>.sh` has a co-located `scripts/<name>.md` — Purpose / Usage / Behavior / Exit codes / Invariants. A new script without its contract is an incomplete task.

---

## File Structure

**New files**

| Path | Responsibility |
|---|---|
| `scripts/lib/docket-stack.sh` | Sourced library. The three shared routines: chain validation (parent exists, no cycle), the effective-base resolver (spec §3's four rules), and the descendant-graph scan. No git writes, no network. |
| `scripts/stack-base.sh` | CLI over the resolver — prints one change's effective base branch, or a diagnostic. Consumed by `docket-implement-next` (branch cut, PR base) and `docket-finalize-change` (rebase target). |
| `scripts/stack-base.md` | Contract for the above. |
| `scripts/stack-closeout.sh` | CLI for spec §7 — the idempotent stack close-out: snapshot the descendant graph rooted at a merged change, promote each `stacked-merged` descendant through the normal terminal close-out, and write the root's **Stack carried** table. |
| `scripts/stack-closeout.md` | Contract for the above. |
| `skills/docket-convention/references/stacked-changes.md` | The progressive-disclosure mechanics file every touched skill reads on trigger. |
| `tests/test_docket_stack.sh` | New subsystem: the library plus `stack-base.sh`. |
| `tests/test_stack_closeout.sh` | New subsystem: `stack-closeout.sh`. |
| `tests/test_docket_status_stack.sh` | Sibling shard — the sweep's stacked legs. `tests/test_docket_status.sh` is already pinned at the 60s ceiling, so its legs cannot land there. |
| `tests/test_board_checks_stack.sh` | Sibling shard — the two new health checks. `tests/test_board_checks.sh` sits at 55s. |

**Modified files**

| Path | Change |
|---|---|
| `scripts/lib/docket-frontmatter.sh` | `stacked-merged` joins `DOCKET_STATUSES_ACTIVE`; `stacked_on` joins the absent-capable key list in the selection-rule header. |
| `scripts/render-board.sh` | New arms in `emoji_for`, `label_for_title`, `table_header_for`, and the row-format `case`; the stack cells; count-free M3 wording. |
| `scripts/render-change-links.sh` | Derived **Stacked children** row; `stacked-merged` keeps branch-addressed plan/results links. |
| `scripts/board-checks.sh` | Two new checks: `stack-invalid`, `stack-parent-killed`. |
| `scripts/docket-status.sh` | Sweep: a PR merged into a parent branch moves the change to `stacked-merged`; a root merge invokes `stack-closeout.sh`. |
| `scripts/github-mirror.sh` | Projects v2 status options pick up the new active status through the array. |
| `scripts/verify-run.sh` | The claim gate and the `status` conjunct accept `stacked-merged`. |
| `scripts/docket.sh` | Facade ops for `stack-base` and `stack-closeout`. |
| `skills/docket-new-change/change-template.md` | `stacked_on:` seeded empty. |
| `skills/docket-convention/SKILL.md` | Eight states; manifest field; branch-model exception; readiness; board cells; pointer to the new reference. |
| `skills/docket-convention/github-board-mirror.md` | The status→issue mapping covers the new state. |
| `skills/docket-implement-next/SKILL.md`, `skills/docket-finalize-change/SKILL.md`, `skills/docket-status/SKILL.md`, `skills/docket-new-change/SKILL.md`, `skills/docket-groom-next/SKILL.md` | One trigger line each. |
| `skills/docket-finalize-change/references/gate-failure.md` | Scope its "an eighth status is the wrong shape" argument to the finalize-blocked case. |
| `scripts/render-board.md`, `scripts/board-checks.md`, `scripts/github-mirror.md`, `scripts/docket-status.md`, `scripts/verify-run.md`, `README.md`, `.docket.example.yml` | Documentation follow-through. |
| `tests/runtime-budgets.tsv`, `tests/test_runtime_budgets.sh` | Four new rows; `EXPECTED_TOTAL` re-seeded; header rationale. |
| `tests/test_render_board.sh`, `tests/test_render_change_links.sh`, `tests/test_github_mirror.sh`, `tests/test_docket_frontmatter.sh`, `tests/test_frontmatter_read_shapes.sh` | Vocabulary cardinality re-seed, golden re-bless, corpus map. |

---

## Locked design decisions

These are decided here so no task re-litigates them.

1. **`stacked-merged` is an ACTIVE status, last in display order.** The array becomes
   `DOCKET_STATUSES_ACTIVE=(in-progress proposed blocked deferred implemented stacked-merged)`.
   Array order is the contract for BOARD.md section order and the digest `backlog` rollup, and the
   state sits after `implemented` in the lifecycle, so it sorts last. Cardinality moves 5 → 6
   active, 7 → 8 total.
2. **The M3 malformed-status diagnostic becomes count-free.** `render-board.sh` currently says
   `is not one of the seven lifecycle statuses`. Hardcoding a new count just re-arms the same trap,
   so the wording becomes `is not one of the lifecycle statuses: <list>`, interpolating
   `${DOCKET_STATUSES[*]}` — the same shape `board-checks.sh`'s `field-domain` already uses.
3. **`stacked_on:` is a single integer scalar, unquoted, never a flow collection** — one parent,
   unlike `depends_on`. It is never copied into `related:` or `depends_on:`.
4. **The parent-side link is derived, never stored.** `render-change-links.sh` scans for
   `stacked_on: <parent id>` and renders a **Stacked children** row into the parent's generated
   `## Artifacts` block. No `stacked_children:` field exists.
5. **`stack-base.sh` exit codes:** `0` resolved (branch name on stdout), `3` killed parent (spec
   §9 — a human decision, never the merge fallback), `4` invalid resolution (missing parent, cycle,
   or a `branch:` whose remote ref is missing), `2` usage. A non-stacked change resolves to the
   integration branch at exit `0`, so every caller can invoke it unconditionally.
6. **Two new health checks, not one.** `stack-invalid` covers exit-4 resolutions; `stack-parent-killed`
   covers exit-3. They are distinct because the remedies differ — one is a data repair, the other a
   scoping decision.
7. **New test shards, not extensions.** `tests/test_docket_status.sh` (60s) and
   `tests/test_board_checks.sh` (55s) have no headroom, and a budget row at its ceiling is already
   spent. New legs land in `tests/test_docket_status_stack.sh` and `tests/test_board_checks_stack.sh`.

---

### Task 1: The eighth lifecycle status

Adds `stacked-merged` to the shared vocabulary and every derived view that iterates it. Nothing yet
*produces* the status — this task makes the system able to render and validate a change that carries
it. Splitting it any finer would leave the suite red between tasks, because the set-equality guards
fire the moment the array changes.

**Files:**
- Modify: `scripts/lib/docket-frontmatter.sh` (the `--- status vocabulary (change 0104) ---` block)
- Modify: `scripts/render-board.sh` (`emoji_for`, `label_for_title`, `table_header_for`, the `print_section` row-format `case` marked `# row_format_mapping`, the M3 validation diagnostic)
- Modify: `scripts/render-board.md`, `scripts/board-checks.md`
- Modify: `skills/docket-convention/SKILL.md` (the `### Lifecycle — seven states` section), `skills/docket-convention/github-board-mirror.md`, `README.md`
- Test: `tests/test_docket_frontmatter.sh`, `tests/test_render_board.sh`, `tests/test_github_mirror.sh`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `DOCKET_STATUSES_ACTIVE` with 6 members and `DOCKET_STATUSES` with 8; `docket_status_is_active stacked-merged` returns 0; `docket_status_is_terminal stacked-merged` returns non-zero. Every later task relies on those three facts.

- [ ] **Step 1: Write the failing membership test**

Append to `tests/test_docket_frontmatter.sh`, beside the existing membership-helper asserts:

```bash
assert "stacked-merged is a vocabulary member" 'docket_status_is_member stacked-merged'
assert "stacked-merged is ACTIVE, not terminal" 'docket_status_is_active stacked-merged && ! docket_status_is_terminal stacked-merged'
assert "ACTIVE has six members" '[ "${#DOCKET_STATUSES_ACTIVE[@]}" = 6 ]'
assert "vocabulary has eight members" '[ "${#DOCKET_STATUSES[@]}" = 8 ]'
assert "stacked-merged sorts last in ACTIVE" '[ "${DOCKET_STATUSES_ACTIVE[5]}" = stacked-merged ]'
```

- [ ] **Step 2: Run to verify it fails**

Run: `scripts/run-tests.sh --verbose tests/test_docket_frontmatter.sh`
Expected: FAIL — five `NOT OK` lines, the first being "stacked-merged is a vocabulary member".

- [ ] **Step 3: Extend the vocabulary array**

In `scripts/lib/docket-frontmatter.sh`, in the `--- status vocabulary (change 0104) ---` block:

```sh
DOCKET_STATUSES_ACTIVE=(in-progress proposed blocked deferred implemented stacked-merged)
```

Leave `DOCKET_STATUSES_TERMINAL` and the derived `DOCKET_STATUSES` composition untouched. Update the
block's own comment so it states that `stacked-merged` is non-terminal — the change file stays in
`active/` and is promoted to `done` by the stack close-out — and that display order places it after
`implemented`.

- [ ] **Step 4: Run to verify the library test passes and the renderer tests now fail**

Run: `scripts/run-tests.sh --verbose tests/test_docket_frontmatter.sh`
Expected: PASS.

Run: `scripts/run-tests.sh --verbose tests/test_render_board.sh`
Expected: FAIL — the set-equality guards report that `emoji_for`, `label_for_title`,
`table_header_for`, and the row-format mapping no longer cover the vocabulary, and the cardinality
asserts pinned at `7`/`5` are wrong.

- [ ] **Step 5: Add the renderer arms**

In `scripts/render-board.sh`:

- `emoji_for()` — add an arm `stacked-merged) printf '🪆' ;;` (a distinct emoji not already used by
  another arm; verify by reading the seven existing arms first).
- `label_for_title()` — add `stacked-merged) printf 'Stacked-merged' ;;`.
- `table_header_for()` — add an arm whose columns are the `implemented` arm's columns with the PR
  column retained and the readiness column replaced by a **Stack** column, because the useful fact
  about a `stacked-merged` change is which parent it merged into:

```sh
    stacked-merged) printf '| # | Change | Type | Stack | PR |\n|---|---|---|---|---|\n' ;;
```

- The `print_section` row-format `case` — add the matching row template. Read `stacked_on` with the
  anchored accessor and render the cell as `merged into #<padded parent>`:

```sh
    stacked-merged)
      printf '| %s | %s | %s | merged into #%s | %s |\n' \
        "$pid" "$link" "$(type_cell "$f")" \
        "$(printf '%04d' "$(( 10#$(fm_field "$f" stacked_on) ))")" \
        "$(pr_cell "$f")"
      ;;
```

Match the surrounding arms' exact variable names — read them before writing; the names above
(`pid`, `link`, `f`) are the ones the neighbouring arms use, and a mismatch is a silent empty cell.

- [ ] **Step 6: Make the M3 diagnostic count-free**

In `scripts/render-board.sh`'s upfront malformed-file validation, replace the literal
`is not one of the seven lifecycle statuses` with an interpolated form:

```sh
      bad_reason="status '$st' is not one of the lifecycle statuses: ${DOCKET_STATUSES[*]}"
```

Match the surrounding assignment's variable name exactly. Then update the M3 row in
`scripts/render-board.md` to quote the new wording, and the `field-domain` prose in
`scripts/board-checks.md` so neither says "seven".

- [ ] **Step 7: Re-seed the vocabulary guards and re-bless the golden board**

In `tests/test_render_board.sh`, in the change-0116 vocabulary guard block:

- change the cardinality asserts from `7` to `8` and from `5` to `6`;
- leave the set-equality extractors alone — they derive from the arrays and will now demand the new
  arms, which Step 5 supplied.

Then extend the golden BOARD.md fixture: add one fixture change carrying
`status: stacked-merged` and `stacked_on: 1`, and update the golden's count line and section
headings to include the new section. Run the renderer against the fixture tree and diff against the
golden until they match — but read the produced output before blessing it, rather than copying
output into the golden blind.

Add one assert that the new section renders its stack cell, using a single-quoted anchor:

```bash
assert "stacked-merged row names its parent" 'grep -qF "merged into #0001" "$out"'
```

Match `$out` to the variable the surrounding golden asserts use.

- [ ] **Step 8: Re-seed the GitHub mirror literal**

`tests/test_github_mirror.sh` asserts the Projects v2 field-create call with a hardcoded string.
Update it to the new list — the order is the array's order:

```bash
assert "project status options carry every active status" 'grep -qF "in-progress,proposed,blocked,deferred,implemented,stacked-merged" "$log"'
```

Match `$log` to the variable the surrounding asserts use. `scripts/github-mirror.sh` itself needs no
edit — it joins `DOCKET_STATUSES_ACTIVE[*]`, so it picks the token up automatically. Confirm that by
reading the `STATUS_OPTIONS` assignment before concluding the task; if it spells the list literally,
change it to the array join.

- [ ] **Step 9: Update the prose that counts to seven**

Derive the sites from a whole-repo grep rather than this list, then fix each:

```bash
grep -rn "seven lifecycle statuses\|seven states\|all seven\|seven-name" . --exclude-dir=.git
```

At minimum: `skills/docket-convention/SKILL.md`'s `### Lifecycle — seven states` heading, its ASCII
diagram, its seven-row status table, and the frontmatter-template comment spelling the alternation;
`skills/docket-convention/github-board-mirror.md`'s "all seven" mapping; `README.md`'s lifecycle
section. The convention's status table gains a row:

```markdown
| `stacked-merged` | merged into its stack parent — awaiting the stack root | `active/` |
```

and the diagram gains the `implemented → stacked-merged → done` limb. Sort the grep's hits into
prose vs executable and fix both — only the executable ones can violate a gate.

- [ ] **Step 10: Run the full suite**

Run: `scripts/run-tests.sh`
Expected: PASS, with no `OVER BUDGET:` line. Investigate any budget breach before committing.

- [ ] **Step 11: Mutation-test the new guards**

Delete the `stacked-merged` arm from `emoji_for` and re-run `tests/test_render_board.sh` — it must
redden. Restore from your own backup copy, not `git checkout --` (which restores to HEAD and would
destroy the rest of the task's uncommitted work). Repeat for `table_header_for` and the row-format
arm. Record any assert that stays green as a defect and fix the assert.

- [ ] **Step 12: Commit**

```bash
git add scripts/lib/docket-frontmatter.sh scripts/render-board.sh scripts/render-board.md \
        scripts/board-checks.md skills/docket-convention/SKILL.md \
        skills/docket-convention/github-board-mirror.md README.md \
        tests/test_docket_frontmatter.sh tests/test_render_board.sh tests/test_github_mirror.sh
git commit -m "feat(0298): add the stacked-merged lifecycle status to the shared vocabulary"
```

---

### Task 2: The `stacked_on:` field and chain validation

Introduces the manifest field and the library routine that decides whether a `stacked_on` chain is
well-formed. No consumer yet — this is the data layer the resolver builds on.

**Files:**
- Create: `scripts/lib/docket-stack.sh`
- Create: `tests/test_docket_stack.sh`
- Modify: `skills/docket-new-change/change-template.md`, `skills/docket-convention/SKILL.md` (manifest block), `scripts/lib/docket-frontmatter.sh` (selection-rule header's absent-capable key list)
- Modify: `tests/runtime-budgets.tsv`, `tests/test_runtime_budgets.sh`

**Interfaces:**
- Consumes: `fm_field`, `field`, `_docket_array_has` from `scripts/lib/docket-frontmatter.sh`.
- Produces:
  - `stack_parent_id CHANGES_DIR ID` → prints the parent id (canonical, unpadded) or empty; exit 0 always.
  - `stack_chain CHANGES_DIR ID` → prints the ancestor chain, nearest parent first, one id per line; exit 0 well-formed, exit 1 on a cycle or a missing parent, with a diagnostic on stderr.
  - `stack_find_file CHANGES_DIR ID` → prints the absolute path of the change file for `ID`, searching `active/` then `archive/`; empty and exit 1 when absent.

- [ ] **Step 1: Write the failing test file**

Create `tests/test_docket_stack.sh`:

```bash
#!/usr/bin/env bash
# tests/test_docket_stack.sh — unit tests for the stacked-changes library and the stack-base CLI
# (change 0298). Sources scripts/lib/docket-stack.sh directly and drives scripts/stack-base.sh
# against hermetic fixture trees. Run: bash tests/test_docket_stack.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LIB="$REPO/scripts/lib/docket-stack.sh"
SCRIPT="$REPO/scripts/stack-base.sh"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

assert "library exists" '[ -f "$LIB" ]'
# shellcheck source=/dev/null
source "$REPO/scripts/lib/docket-frontmatter.sh"
# shellcheck source=/dev/null
source "$LIB"

tmp="$(mktemp -d "${TMPDIR:-/tmp}/docket-stack.XXXXXX")"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/active" "$tmp/archive"

mkchange(){ # mkchange <id> <slug> <status> [stacked_on] [branch]
  printf '%04d' "$1" >/dev/null
  cat > "$tmp/active/$(printf '%04d' "$1")-$2.md" <<EOF
---
id: $1
slug: $2
title: "Change $1"
status: $3
priority: medium
created: 2026-08-12
updated: 2026-08-12
depends_on: []
stacked_on: ${4:-}
branch: ${5:-}
---

## Why

Fixture.
EOF
}

mkchange 1 alpha implemented "" feat/alpha
mkchange 2 beta proposed 1
mkchange 3 gamma proposed 2

assert "an unstacked change has no parent" '[ -z "$(stack_parent_id "$tmp" 1)" ]'
assert "a stacked change names its parent" '[ "$(stack_parent_id "$tmp" 2)" = 1 ]'
assert "a padded id resolves to the same change" '[ "$(stack_parent_id "$tmp" 0002)" = 1 ]'
assert "a nested chain lists nearest parent first" '[ "$(stack_chain "$tmp" 3 | tr "\n" " ")" = "2 1 " ]'
assert "a well-formed chain exits 0" 'stack_chain "$tmp" 3 >/dev/null 2>&1'

# a cycle: 4 -> 5 -> 4
mkchange 4 delta proposed 5
mkchange 5 epsilon proposed 4
assert "a cycle is refused" '! stack_chain "$tmp" 4 >/dev/null 2>&1'
assert "a cycle names the cycle on stderr" '[ -n "$(stack_chain "$tmp" 4 2>&1 >/dev/null | grep -F cycle)" ]'

# a missing parent
mkchange 6 zeta proposed 999
assert "a missing parent is refused" '! stack_chain "$tmp" 6 >/dev/null 2>&1'

# the absent-key hazard: frontmatter omits stacked_on, body opens a stacked_on line
cat > "$tmp/active/0007-eta.md" <<'EOF'
---
id: 7
slug: eta
title: "Change 7"
status: proposed
priority: medium
created: 2026-08-12
updated: 2026-08-12
depends_on: []
---

## Why

stacked_on: 42 is discussed here as prose, not as frontmatter.
EOF
assert "an absent stacked_on does not fall through to body prose" '[ -z "$(stack_parent_id "$tmp" 7)" ]'

printf '%s\n' "--- done"
exit "$fail"
```

- [ ] **Step 2: Run to verify it fails**

Run: `scripts/run-tests.sh --verbose tests/test_docket_stack.sh`
Expected: FAIL with `NOT OK - library exists` — the library does not exist yet.

- [ ] **Step 3: Write the library**

Create `scripts/lib/docket-stack.sh`. Header comment states: sourced, never executed; declares
functions only; no git and no network; requires `scripts/lib/docket-frontmatter.sh` to have been
sourced first. Then:

```sh
# stack_find_file CHANGES_DIR ID -> absolute path, or empty + exit 1
stack_find_file(){
  local dir="$1" id padded f
  id=$(( 10#$2 )) || return 1
  padded="$(printf '%04d' "$id")"
  for f in "$dir"/active/"$padded"-*.md "$dir"/archive/*-"$padded"-*.md; do
    [ -f "$f" ] || continue
    printf '%s\n' "$f"
    return 0
  done
  return 1
}

# stack_parent_id CHANGES_DIR ID -> parent id (canonical) or empty; always exit 0
stack_parent_id(){
  local f raw
  f="$(stack_find_file "$1" "$2")" || return 0
  raw="$(fm_field "$f" stacked_on)"
  [ -n "$raw" ] || return 0
  case "$raw" in (*[!0-9]*) return 0 ;; esac
  printf '%d\n' "$(( 10#$raw ))"
}

# stack_chain CHANGES_DIR ID -> ancestors nearest-first, one per line; exit 1 on cycle/missing
stack_chain(){
  local dir="$1" cur seen=" " parent
  cur=$(( 10#$2 ))
  seen="$seen$cur "
  while :; do
    parent="$(stack_parent_id "$dir" "$cur")"
    [ -n "$parent" ] || return 0
    if ! stack_find_file "$dir" "$parent" >/dev/null; then
      printf 'stack: change %s names a missing stacked_on parent %s\n' "$cur" "$parent" >&2
      return 1
    fi
    case "$seen" in (*" $parent "*)
      printf 'stack: cycle in the stacked_on chain at change %s\n' "$parent" >&2
      return 1 ;;
    esac
    printf '%s\n' "$parent"
    seen="$seen$parent "
    cur="$parent"
  done
}
```

Note the `10#` canonicalization on every id boundary, and that `stack_parent_id` reads with the
anchored `fm_field` because `stacked_on` is optional.

- [ ] **Step 4: Run to verify it passes**

Run: `scripts/run-tests.sh --verbose tests/test_docket_stack.sh`
Expected: PASS.

- [ ] **Step 5: Seed the field in the template and the manifest doc**

In `skills/docket-new-change/change-template.md`, add below `depends_on:`:

```yaml
stacked_on:               # optional: parent change id whose branch this one is built on
```

Add the same line with the same comment to the manifest block in
`skills/docket-convention/SKILL.md`, and add one sentence to the *Change manifest* prose stating
that `stacked_on:` is a single integer scalar naming one parent, that it is the single source of
truth for the relationship, and that the parent id is never copied into `related:` or `depends_on:`.

- [ ] **Step 6: Register `stacked_on` as absent-capable**

In `scripts/lib/docket-frontmatter.sh`'s selection-rule header, add `stacked_on` to the
absent-capable key list — the sentence naming `spec plan results branch pr issue blocked_by type
claimed_at trivial auto_groomable …`. Do **not** add it to any guaranteed-present set.

- [ ] **Step 7: Add the budget row**

In `tests/runtime-budgets.tsv`, add (tab-separated):

```
tests/test_docket_stack.sh	15	parallel
```

Then raise `EXPECTED_TOTAL` in `tests/test_runtime_budgets.sh` by 15, and write the rationale into
the TSV header comment ledger: the row covers a new subsystem (the stacked-changes library and the
`stack-base` CLI, change 0298) and 15s is the measured runtime rounded up.

Measure first: `scripts/run-tests.sh --verbose tests/test_docket_stack.sh` prints the elapsed time —
use it. If it exceeds 15s, set the row to the measured value rounded up and adjust the total to match.

- [ ] **Step 8: Run the full suite**

Run: `scripts/run-tests.sh`
Expected: PASS, no `OVER BUDGET:` line.

- [ ] **Step 9: Mutation-test the anchoring**

Swap `fm_field "$f" stacked_on` to `field "$f" stacked_on` in `stack_parent_id` and re-run
`tests/test_docket_stack.sh` — the "absent stacked_on does not fall through to body prose" assert
must redden. Restore from your backup copy. If it stays green, the fixture is not discriminating and
the fixture is the defect.

- [ ] **Step 10: Commit**

```bash
git add scripts/lib/docket-stack.sh tests/test_docket_stack.sh tests/runtime-budgets.tsv \
        tests/test_runtime_budgets.sh skills/docket-new-change/change-template.md \
        skills/docket-convention/SKILL.md scripts/lib/docket-frontmatter.sh
git commit -m "feat(0298): stacked_on manifest field and stack chain validation"
```

---

### Task 3: The effective-base resolver and its CLI

Implements spec §3's four rules and exposes them as `stack-base.sh`.

**Files:**
- Modify: `scripts/lib/docket-stack.sh` (add `stack_effective_base`)
- Create: `scripts/stack-base.sh`, `scripts/stack-base.md`
- Modify: `scripts/docket.sh` (facade op), `tests/test_docket_stack.sh`, `tests/test_frontmatter_read_shapes.sh` (`CORPUS_MAP`)

**Interfaces:**
- Consumes: `stack_parent_id`, `stack_chain`, `stack_find_file` from Task 2.
- Produces:
  - `stack_effective_base CHANGES_DIR ID INTEGRATION_BRANCH` → prints the base branch name; exit `0` resolved, `3` killed parent, `4` invalid.
  - `scripts/stack-base.sh --changes-dir DIR --id N --integration-branch B [--remote R]` → same, on stdout, with the same exit codes plus `2` for usage.

- [ ] **Step 1: Write the failing tests**

Append to `tests/test_docket_stack.sh`, before the trailing `exit "$fail"`:

```bash
# --- effective base resolution (spec §3) ---
GIT_STUB="$tmp/bin"; mkdir -p "$GIT_STUB"
cat > "$GIT_STUB/git" <<'EOF'
#!/usr/bin/env bash
# stub: `git show-ref --verify --quiet refs/remotes/origin/<b>` succeeds only for listed branches
if [ "$1" = show-ref ]; then
  for b in $DOCKET_TEST_REMOTE_BRANCHES; do
    case " $* " in (*" refs/remotes/origin/$b "*) exit 0 ;; esac
  done
  exit 1
fi
exit 0
EOF
chmod +x "$GIT_STUB/git"
export DOCKET_TEST_REMOTE_BRANCHES="feat/alpha"
GIT="$GIT_STUB/git"; export GIT

assert "rule 1: a live parent with a pushed branch resolves to that branch" \
  '[ "$(stack_effective_base "$tmp" 2 main)" = "feat/alpha" ]'
assert "an unstacked change resolves to the integration branch" \
  '[ "$(stack_effective_base "$tmp" 1 main)" = "main" ]'

# rule 1 is remote-ref gated: an in-progress parent whose branch was never pushed is NOT a base
mkchange 10 iota in-progress "" feat/iota
mkchange 11 kappa proposed 10
assert "rule 4: a branch with no remote ref is invalid" \
  'stack_effective_base "$tmp" 11 main >/dev/null 2>&1; [ "$?" = 4 ]'

# rule 2: a merged parent resolves upward
mkchange 12 lambda done "" feat/lambda
mkchange 13 mu proposed 12
assert "rule 2: a done parent resolves to the integration branch" \
  '[ "$(stack_effective_base "$tmp" 13 main)" = "main" ]'

# rule 2, nested: grandparent still live
mkchange 14 nu stacked-merged 1 feat/nu
mkchange 15 xi proposed 14
export DOCKET_TEST_REMOTE_BRANCHES="feat/alpha"
assert "rule 2 nested: a stacked-merged parent whose branch is gone resolves to the grandparent" \
  '[ "$(stack_effective_base "$tmp" 15 main)" = "feat/alpha" ]'

# rule 3: killed parent
mkchange 16 omicron killed "" feat/omicron
mkchange 17 pi proposed 16
assert "rule 3: a killed parent stops with exit 3" \
  'stack_effective_base "$tmp" 17 main >/dev/null 2>&1; [ "$?" = 3 ]'

assert "rule 4: a cycle is invalid" \
  'stack_effective_base "$tmp" 4 main >/dev/null 2>&1; [ "$?" = 4 ]'
assert "rule 4: a missing parent is invalid" \
  'stack_effective_base "$tmp" 6 main >/dev/null 2>&1; [ "$?" = 4 ]'

# --- the CLI ---
assert "CLI prints the resolved base" \
  '[ "$(GIT="$GIT_STUB/git" "$SCRIPT" --changes-dir "$tmp" --id 2 --integration-branch main)" = "feat/alpha" ]'
assert "CLI accepts a padded id" \
  '[ "$(GIT="$GIT_STUB/git" "$SCRIPT" --changes-dir "$tmp" --id 0002 --integration-branch main)" = "feat/alpha" ]'
assert "CLI exits 3 on a killed parent" \
  'GIT="$GIT_STUB/git" "$SCRIPT" --changes-dir "$tmp" --id 17 --integration-branch main >/dev/null 2>&1; [ "$?" = 3 ]'
assert "CLI exits 2 on a missing required flag" \
  '"$SCRIPT" --changes-dir "$tmp" >/dev/null 2>&1; [ "$?" = 2 ]'
```

Note the "padded id" assert — it is the guard against the octal trap, and it discriminates only
because `0002` would otherwise parse as octal 2 by luck; add a second padded case with a digit above
7 if any fixture id reaches that range, since `0008` is where the trap actually bites.

Add exactly that case:

```bash
mkchange 8 theta proposed 1
assert "CLI accepts a padded id with a digit above 7" \
  '[ "$(GIT="$GIT_STUB/git" "$SCRIPT" --changes-dir "$tmp" --id 0008 --integration-branch main)" = "feat/alpha" ]'
```

- [ ] **Step 2: Run to verify it fails**

Run: `scripts/run-tests.sh --verbose tests/test_docket_stack.sh`
Expected: FAIL — `stack_effective_base: command not found` on the first new assert.

- [ ] **Step 3: Implement the resolver**

Add to `scripts/lib/docket-stack.sh`:

```sh
# stack_effective_base CHANGES_DIR ID INTEGRATION_BRANCH
#   exit 0 = resolved (branch on stdout), 3 = killed parent, 4 = invalid
stack_effective_base(){
  local dir="$1" id="$2" integration="$3" parent f status branch git="${GIT:-git}"
  stack_chain "$dir" "$id" >/dev/null 2>&1 || return 4
  parent="$(stack_parent_id "$dir" "$id")"
  [ -n "$parent" ] || { printf '%s\n' "$integration"; return 0; }
  f="$(stack_find_file "$dir" "$parent")" || return 4
  status="$(field "$f" status)"
  case "$status" in
    killed) return 3 ;;
    done)   stack_effective_base "$dir" "$parent" "$integration"; return $? ;;
  esac
  branch="$(fm_field "$f" branch)"
  if [ -n "$branch" ] && "$git" show-ref --verify --quiet "refs/remotes/origin/$branch"; then
    printf '%s\n' "$branch"
    return 0
  fi
  # a stacked-merged parent whose branch is gone falls back upward (spec §3 rule 2)
  if [ "$status" = stacked-merged ]; then
    stack_effective_base "$dir" "$parent" "$integration"
    return $?
  fi
  return 4
}
```

The `GIT` indirection is the repo's standard mock seam — match how `scripts/board-checks.sh` spells
it before writing, and use that spelling.

- [ ] **Step 4: Write the CLI**

Create `scripts/stack-base.sh`. Shape it on `scripts/render-change-links.sh`: a `^#` header printed
by `-h|--help`, flag parsing with `die` on an unknown flag (exit 2), then source both libraries and
call the resolver. Required flags: `--changes-dir`, `--id`, `--integration-branch`. Optional:
`--remote` (default `origin`). Canonicalize `--id` with `10#` at the argument boundary. Validate the
whole flag set before doing any work.

- [ ] **Step 5: Run to verify it passes**

Run: `scripts/run-tests.sh --verbose tests/test_docket_stack.sh`
Expected: PASS.

- [ ] **Step 6: Write the contract and wire the facade**

Create `scripts/stack-base.md` with the five standard sections. Under *Exit codes*, state all four
(`0`, `2`, `3`, `4`) and what each obliges the caller to do — `3` is a human decision, never the
merge fallback; `4` is a data repair. Under *Invariants*, state that rule 1 requires the remote ref
to exist, and why: `branch:` is stamped at claim but the branch is only pushed at the PR step, so an
`in-progress` parent can carry a valid-looking `branch:` with nothing behind it.

Add `stack-base` to `scripts/docket.sh`'s op dispatch and to `scripts/docket.md`'s op table, matching
the surrounding entries exactly.

- [ ] **Step 7: Register the new script in the read-shapes census**

`tests/test_frontmatter_read_shapes.sh` fails closed on an unknown `(script, file-argument token)`
pair. `scripts/lib/docket-stack.sh` reads change files, so add its `(script, token)` pairs to
`CORPUS_MAP` with corpus `change`. Read the existing entries and match their exact format. Then run
that test and confirm it passes rather than assuming the mapping took.

- [ ] **Step 8: Run the full suite, then mutation-test**

Run: `scripts/run-tests.sh`
Expected: PASS.

Mutation: delete the `show-ref` conjunct from rule 1 so any populated `branch:` is accepted. The
"a branch with no remote ref is invalid" assert must redden. Restore from your backup copy.

- [ ] **Step 9: Commit**

```bash
git add scripts/lib/docket-stack.sh scripts/stack-base.sh scripts/stack-base.md \
        scripts/docket.sh scripts/docket.md tests/test_docket_stack.sh \
        tests/test_frontmatter_read_shapes.sh
git commit -m "feat(0298): effective-base resolver and the stack-base CLI"
```

---

### Task 4: Readiness gating and the board's stack cells

A stacked change is build-ready only when its effective base resolves; the board says so.

**Files:**
- Modify: `scripts/render-board.sh` (`readiness_cell`, `digest_readiness`, the ready-queue producer)
- Modify: `scripts/render-board.md`, `scripts/docket-status.md`
- Modify: `skills/docket-convention/SKILL.md` (*Build-readiness & selection*)
- Test: `tests/test_render_board.sh`

**Interfaces:**
- Consumes: `stack_effective_base` (Task 3).
- Produces: the digest readiness token `stack-base-unresolved` and the board cell `waiting on #A — stack base not built`. Task 9's board assertions rely on both spellings.

- [ ] **Step 1: Write the failing tests**

Append to `tests/test_render_board.sh` (matching its `ok`/`no` helper style if that is what the file
uses — read the top of the file and match it):

```bash
assert "a stacked change whose base resolves stays build-ready" \
  'grep -qF "change 21 proposed build-ready" "$digest"'
assert "a stacked change whose base is unresolved is not in the ready queue" \
  '! grep -E -e "^ready( |.*[^0-9])22( |$)" "$digest"'
assert "an unresolved stack base renders its own digest token" \
  'grep -qF "change 22 proposed stack-base-unresolved" "$digest"'
assert "the board cell names the parent" \
  'grep -qF "waiting on #0020 — stack base not built" "$board"'
```

Match `$digest` and `$board` to the variables the surrounding legs use, and add the fixtures: change
20 `in-progress` with a `branch:` that has no remote ref, change 21 stacked on a parent whose branch
does resolve, change 22 stacked on 20.

- [ ] **Step 2: Run to verify it fails**

Run: `scripts/run-tests.sh --verbose tests/test_render_board.sh`
Expected: FAIL on all four.

- [ ] **Step 3: Gate readiness on the resolver**

In `scripts/render-board.sh`, source `scripts/lib/docket-stack.sh` alongside the frontmatter library.
In `digest_readiness`, before returning `build-ready`, add the stack conjunct: when the change
carries a `stacked_on`, call `stack_effective_base` and, on a non-zero exit, return
`stack-base-unresolved` instead. In `readiness_cell`, render the matching prose cell using the
padded parent id. Leave the deterministic selection order untouched — stacking adds an eligibility
condition, not a ranking one.

The ready-queue producer already filters on `digest_readiness … = build-ready`, so it inherits the
gate with no edit. Confirm that by reading it rather than assuming.

- [ ] **Step 4: Run to verify it passes**

Run: `scripts/run-tests.sh --verbose tests/test_render_board.sh`
Expected: PASS.

- [ ] **Step 5: Document the widened definition**

Update the *Build-readiness & selection* shared definition in `skills/docket-convention/SKILL.md`:
build-ready now also requires that a `stacked_on` change's effective base resolves. Add the new
digest token to the `change`-line row in `scripts/docket-status.md` and the new cell to
`scripts/render-board.md`'s cell inventory.

- [ ] **Step 6: Run the full suite and mutation-test**

Run: `scripts/run-tests.sh`
Expected: PASS.

Mutation: remove the stack conjunct from `digest_readiness`. The "not in the ready queue" assert must
redden. Restore from your backup copy.

- [ ] **Step 7: Commit**

```bash
git add scripts/render-board.sh scripts/render-board.md scripts/docket-status.md \
        skills/docket-convention/SKILL.md tests/test_render_board.sh
git commit -m "feat(0298): gate build-readiness on the effective base and render the stack cells"
```

---

### Task 5: The derived Stacked children row

Spec §11 — the parent's reciprocal link, derived at render time, never denormalized.

**Files:**
- Modify: `scripts/render-change-links.sh`
- Test: `tests/test_render_change_links.sh`

**Interfaces:**
- Consumes: `stack_find_file`, `stack_parent_id` (Task 2).
- Produces: a `| Stacked children | … |` row inside the `## Artifacts` block, and branch-addressed plan/results links for a `stacked-merged` change.

- [ ] **Step 1: Write the failing tests**

Append to `tests/test_render_change_links.sh`:

```bash
assert "a parent renders its stacked children" \
  'grep -qF "| Stacked children |" "$out" && grep -qF "#0031" "$out"'
assert "a child with no children renders no such row" \
  '! grep -qF "| Stacked children |" "$child_out"'
assert "a stacked-merged change keeps branch-addressed plan links" \
  'grep -qF "/blob/feat/beta/docs/superpowers/plans/" "$sm_out"'
```

Match `$out`, `$child_out`, `$sm_out` to the file variables the surrounding legs use, and add the
fixtures: change 30 (the parent), change 31 (`stacked_on: 30`), and a `stacked-merged` change
carrying `branch: feat/beta` and a `plan:`.

- [ ] **Step 2: Run to verify it fails**

Run: `scripts/run-tests.sh --verbose tests/test_render_change_links.sh`
Expected: FAIL on the first and third.

- [ ] **Step 3: Implement the row**

In `scripts/render-change-links.sh`, after the existing rows are built, scan `active/` and
`archive/` under the change file's own changes directory for files whose anchored `stacked_on`
equals this change's id, and emit one row listing each child as `#<padded id> <title> (<status>)`,
linking to the child on the metadata branch. Emit nothing when the set is empty — a blank row is
already stripped, but an empty-set row is a different defect: do not emit the label either.

Separately, extend the `build_ref` decision: it currently switches to the integration branch when
`status = done`. `stacked-merged` is non-terminal, so it must keep the branch ref — verify that the
existing condition tests `= done` exactly and does not test "is terminal" or "is not active", since
those would now behave differently.

- [ ] **Step 4: Run to verify it passes**

Run: `scripts/run-tests.sh --verbose tests/test_render_change_links.sh`
Expected: PASS.

- [ ] **Step 5: Run the full suite and mutation-test**

Run: `scripts/run-tests.sh`
Expected: PASS.

Mutation: make the child scan read `field` instead of `fm_field`, and add a fixture whose body opens
a line with `stacked_on:` — the row must gain a phantom child and an assert must redden. If none
does, add the discriminating assert. Restore from your backup copy.

- [ ] **Step 6: Commit**

```bash
git add scripts/render-change-links.sh tests/test_render_change_links.sh
git commit -m "feat(0298): derive the parent's Stacked children row at render time"
```

---

### Task 6: The sweep moves a child merged into its parent to `stacked-merged`

Spec §6's third bullet — the producer of the new state.

**Files:**
- Modify: `scripts/docket-status.sh` (`detect_merged`, `sweep_execute_one`)
- Modify: `scripts/docket-status.md`
- Create: `tests/test_docket_status_stack.sh`
- Modify: `tests/runtime-budgets.tsv`, `tests/test_runtime_budgets.sh`

**Interfaces:**
- Consumes: `stack_parent_id` (Task 2), the `stacked-merged` status (Task 1).
- Produces: the sweep report line `stacked-merged <id> <parent-id>` on stdout, distinct from `swept <id> <date>`. Task 8 keys on that distinction.

- [ ] **Step 1: Write the failing test file**

Create `tests/test_docket_status_stack.sh` with the bash-4-gate boilerplate copied verbatim from the
top of `tests/test_docket_status.sh` (the `DOCKET_BASH_PATH` probe loop), then a temp repo with a
local bare origin and a stubbed `gh` whose GraphQL reply reports the child's PR as `MERGED` with
`baseRefName` equal to the parent's branch. Assert:

```bash
assert "a PR merged into the parent branch reports stacked-merged" \
  'grep -qF "stacked-merged 41 40" "$out"'
assert "the child is not archived" \
  '[ -f "$mw/docs/changes/active/0041-child.md" ]'
assert "the child status is stacked-merged" \
  '[ "$(sed -n "s/^status: //p" "$mw/docs/changes/active/0041-child.md" | head -n 1)" = stacked-merged ]'
assert "no terminal publish ran for the child" \
  '! grep -qF "terminal-publish" "$ghlog"'
assert "a PR merged into the integration branch still sweeps to done" \
  'grep -qF "swept 42 " "$out"'
```

The third assert pipes into `head -n 1` under `set -uo pipefail` without `-o pipefail`'s SIGPIPE
hazard because the file top sets `set -uo pipefail`, not `-e`; still, prefer capturing first:

```bash
st="$(sed -n 's/^status: //p' "$mw/docs/changes/active/0041-child.md")"
assert "the child status is stacked-merged" '[ "$(printf "%s\n" "$st" | sed -n 1p)" = stacked-merged ]'
```

- [ ] **Step 2: Run to verify it fails**

Run: `scripts/run-tests.sh --verbose tests/test_docket_status_stack.sh`
Expected: FAIL — the child is archived to `done` today.

- [ ] **Step 3: Extend the merge detection**

In `detect_merged`, add `baseRefName` to the aliased GraphQL per-PR selection and carry it into the
TSV as a fifth field. In `sweep_execute_one`, before the archive call, compare the merged PR's base
against the change's resolved parent branch: when the change carries a `stacked_on` **and** the PR's
base equals that parent's `branch:`, set `status: stacked-merged` in place (anchored frontmatter
edit), commit and push on the metadata branch, print `stacked-merged <id> <parent>`, and return
without archiving, publishing, or cleaning up the branch — the branch still carries code the root
needs.

Keep the existing idempotent guard shape: a change already at `stacked-merged` is a no-op.

- [ ] **Step 4: Run to verify it passes**

Run: `scripts/run-tests.sh --verbose tests/test_docket_status_stack.sh`
Expected: PASS.

- [ ] **Step 5: Add the budget row and document the report line**

Add `tests/test_docket_status_stack.sh` at a measured budget (`parallel`) to
`tests/runtime-budgets.tsv`, re-seed `EXPECTED_TOTAL`, and write the rationale into the header
ledger — naming that this is a **sibling shard** because `tests/test_docket_status.sh` is already at
the 60s ceiling and has no headroom for new legs.

Add the `stacked-merged <id> <parent>` line to the report-line vocabulary in
`scripts/docket-status.md`, beside `swept` and `sweep-failed`.

- [ ] **Step 6: Run the full suite and mutation-test**

Run: `scripts/run-tests.sh`
Expected: PASS, no `OVER BUDGET:` line.

Mutation: drop the base-ref comparison so any merged PR on a stacked change is treated as
merged-into-parent. The "a PR merged into the integration branch still sweeps to done" assert must
redden — that is the assert proving the gate discriminates rather than firing on every stacked
change. Restore from your backup copy.

- [ ] **Step 7: Commit**

```bash
git add scripts/docket-status.sh scripts/docket-status.md tests/test_docket_status_stack.sh \
        tests/runtime-budgets.tsv tests/test_runtime_budgets.sh
git commit -m "feat(0298): sweep a PR merged into its stack parent to stacked-merged"
```

---

### Task 7: The idempotent stack close-out

Spec §7 and §10's Stack carried table.

**Files:**
- Create: `scripts/stack-closeout.sh`, `scripts/stack-closeout.md`, `tests/test_stack_closeout.sh`
- Modify: `scripts/lib/docket-stack.sh` (add `stack_descendants`), `scripts/docket.sh`, `scripts/docket.md`
- Modify: `tests/runtime-budgets.tsv`, `tests/test_runtime_budgets.sh`

**Interfaces:**
- Consumes: `stack_parent_id`, `stack_find_file` (Task 2); `archive-change.sh`, `render-change-links.sh`, `terminal-publish.sh`, `cleanup-feature-branch.sh`.
- Produces:
  - `stack_descendants CHANGES_DIR ROOT_ID` → every transitive descendant id, one per line, parents before children.
  - `scripts/stack-closeout.sh --changes-dir DIR --root-id N --date YYYY-MM-DD --integration-branch B --metadata-branch M --adrs-dir DIR --terminal-publish true|false` → promotes each `stacked-merged` descendant to `done` and writes the root's Stack carried table. Report lines `promoted <id>`, `promote-skipped <id> <reason>`, `promote-failed <id> <reason>`.

- [ ] **Step 1: Write the failing test file**

Create `tests/test_stack_closeout.sh` with the standard boilerplate and a temp repo with a bare
origin. Fixtures: root 50 (`done`, archived), children 51 and 52 (`stacked-merged`, stacked on 50),
grandchild 53 (`stacked-merged`, stacked on 51), and an unrelated 54 (`implemented`). Assert:

```bash
assert "every stacked-merged descendant is promoted" \
  'grep -qF "promoted 51" "$out" && grep -qF "promoted 52" "$out" && grep -qF "promoted 53" "$out"'
assert "an unrelated change is untouched" \
  '[ -f "$mw/docs/changes/active/0054-unrelated.md" ]'
assert "a promoted descendant is archived with the root merge date" \
  '[ -f "$mw/docs/changes/archive/2026-08-12-0051-child-one.md" ]'
assert "the root record carries the Stack carried table" \
  'grep -qF "| Stack carried |" "$root_archived" || grep -qF "## Stack carried" "$root_archived"'
assert "the table lists every descendant" \
  'grep -qF "#0051" "$root_archived" && grep -qF "#0053" "$root_archived"'
assert "a second run is a no-op" \
  '[ -z "$(grep -F "promoted 51" "$out2")" ]'
assert "a second run still exits 0" '[ "$rc2" = 0 ]'
assert "a partial run is completed by the next" \
  'grep -qF "promoted 52" "$out3"'
```

The partial-run leg: run the close-out with `terminal-publish` stubbed to fail for change 52 only,
confirm 51 and 53 promoted and 52 reported failed, then re-run with the stub removed and assert 52
promotes on the retry.

- [ ] **Step 2: Run to verify it fails**

Run: `scripts/run-tests.sh --verbose tests/test_stack_closeout.sh`
Expected: FAIL — the script does not exist.

- [ ] **Step 3: Add the descendant scan**

Add to `scripts/lib/docket-stack.sh`:

```sh
# stack_descendants CHANGES_DIR ROOT_ID -> transitive descendant ids, parents before children
stack_descendants(){
  local dir="$1" frontier next f id parent
  frontier="$(( 10#$2 ))"
  while [ -n "$frontier" ]; do
    next=""
    for f in "$dir"/active/*.md "$dir"/archive/*.md; do
      [ -f "$f" ] || continue
      parent="$(fm_field "$f" stacked_on)"
      [ -n "$parent" ] || continue
      case "$parent" in (*[!0-9]*) continue ;; esac
      parent=$(( 10#$parent ))
      case " $frontier " in (*" $parent "*) ;; (*) continue ;; esac
      id="$(( 10#$(field "$f" id) ))"
      printf '%s\n' "$id"
      next="$next$id "
    done
    frontier="$next"
  done
}
```

This is breadth-first, so parents are emitted before children — the order the promotion loop needs.
Note it reads `stacked_on` anchored and `id` with `field` (guaranteed-present).

- [ ] **Step 4: Write the close-out script**

Create `scripts/stack-closeout.sh`. For each descendant, in emitted order:

1. Skip unless its status is `stacked-merged` (a still-`implemented` child is not this pass's
   business; a change already archived is a no-op — key the no-op probe on the **archived file
   existing on the metadata branch**, not on a local clean tree).
2. Run the shared terminal close-out for it: `archive-change.sh --outcome done --date <root merge
   date>`, then `render-change-links.sh` on the archived path committed as its own follow-on commit,
   then `terminal-publish.sh --outcome done --enabled <terminal-publish>`, then
   `cleanup-feature-branch.sh --slug <slug>`.
3. Report `promoted <id>` on success, `promote-failed <id> <reason>` on any non-zero exit, and
   continue to the next descendant — a per-descendant failure must not abandon its siblings, since
   each promotion is independently re-runnable.

Then regenerate the root's **Stack carried** table into the root's archived change file: a
marker-bounded block `<!-- docket:stack-carried:start (generated — do not hand-edit) -->` /
`<!-- docket:stack-carried:end -->` holding one row per descendant (`| #<padded id> | <title> |
<pr> |`). Before rewriting, validate marker order and balance and refuse on dangling, out-of-order,
or nested markers, leaving the file untouched — an unbounded range consumes to EOF and eats the
record. Render to a temp file beside the destination and `mv -f`.

Exit 0 whenever every descendant reached a verdict; exit 1 only when the pass itself could not run.

- [ ] **Step 5: Run to verify it passes**

Run: `scripts/run-tests.sh --verbose tests/test_stack_closeout.sh`
Expected: PASS.

- [ ] **Step 6: Contract, facade, budget**

Write `scripts/stack-closeout.md` with the five sections; state the idempotency key explicitly (the
archived file on the metadata branch, never a local proxy) and the per-descendant continue-on-failure
posture. Add the op to `scripts/docket.sh` and `scripts/docket.md`. Add the budget row at the
measured value, re-seed `EXPECTED_TOTAL`, and write the header rationale.

- [ ] **Step 7: Run the full suite and mutation-test**

Run: `scripts/run-tests.sh`
Expected: PASS, no `OVER BUDGET:` line.

Mutation: key the no-op probe on a clean local tree instead of the archived file. The "a second run
is a no-op" assert should stay green (that is the point — it does not discriminate), so instead add
the discriminating fixture: a descendant archived on the remote but with a dirty local tree, and
assert the second run still reports no promotion. Restore from your backup copy.

- [ ] **Step 8: Commit**

```bash
git add scripts/lib/docket-stack.sh scripts/stack-closeout.sh scripts/stack-closeout.md \
        scripts/docket.sh scripts/docket.md tests/test_stack_closeout.sh \
        tests/runtime-budgets.tsv tests/test_runtime_budgets.sh
git commit -m "feat(0298): idempotent stack close-out with the root's Stack carried table"
```

---

### Task 8: Wire the close-out into the sweep

The root merging through the GitHub button is closed out by the sweep, so the sweep must call it.

**Files:**
- Modify: `scripts/docket-status.sh` (`sweep_execute_one`), `scripts/docket-status.md`
- Test: `tests/test_docket_status_stack.sh`

**Interfaces:**
- Consumes: `scripts/stack-closeout.sh` (Task 7), the `stacked-merged <id> <parent>` line (Task 6).
- Produces: nothing new for later tasks.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_docket_status_stack.sh`: a root whose PR merged into the integration branch,
with two `stacked-merged` descendants.

```bash
assert "a root merge promotes its descendants" \
  'grep -qF "promoted 61" "$out" && grep -qF "promoted 62" "$out"'
assert "the root is still swept to done" 'grep -qF "swept 60 " "$out"'
assert "a root sweep racing a just-merged child handles both in one pass" \
  'grep -qF "stacked-merged 63" "$out" && grep -qF "promoted 63" "$out"'
```

The third leg is the race the spec calls out: the graph snapshot is taken fresh **after** the
per-change sweep loop, so a child swept to `stacked-merged` earlier in the same pass is promoted in
that same pass.

- [ ] **Step 2: Run to verify it fails**

Run: `scripts/run-tests.sh --verbose tests/test_docket_status_stack.sh`
Expected: FAIL on all three new asserts.

- [ ] **Step 3: Invoke the close-out after a root's done-sweep**

In `sweep_execute_one`, after the change reaches `done` and its own close-out steps complete, call
`stack-closeout.sh --root-id <id> --date <merged date> …` and relay its report lines. Take the
descendant snapshot inside `stack-closeout.sh` at call time — not from an earlier scan — so a child
swept moments ago in the same pass is included.

Failure posture matches the sweep's own: log and continue to the next change; the next sweep
self-heals idempotently.

- [ ] **Step 4: Run to verify it passes**

Run: `scripts/run-tests.sh --verbose tests/test_docket_status_stack.sh`
Expected: PASS.

- [ ] **Step 5: Run the full suite**

Run: `scripts/run-tests.sh`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add scripts/docket-status.sh scripts/docket-status.md tests/test_docket_status_stack.sh
git commit -m "feat(0298): invoke the stack close-out from the merge sweep"
```

---

### Task 9: The two health checks

`stack-invalid` and `stack-parent-killed`.

**Files:**
- Modify: `scripts/board-checks.sh`, `scripts/board-checks.md`, `scripts/docket-status.md`, `scripts/lib/docket-frontmatter.sh` (`BOARD_CHECK_IDS`)
- Create: `tests/test_board_checks_stack.sh`
- Modify: `tests/runtime-budgets.tsv`, `tests/test_runtime_budgets.sh`, `tests/test_board_checks.sh` (the four-surface correspondence pins)

**Interfaces:**
- Consumes: `stack_effective_base` (Task 3).
- Produces: two new check ids. `tests/test_board_checks.sh` pins the four surfaces in both directions, so all four must move together.

- [ ] **Step 1: Write the failing test file**

Create `tests/test_board_checks_stack.sh` with the standard boilerplate and a `has_finding` helper
copied in shape from `tests/test_board_checks.sh`. Assert:

```bash
assert "a missing stacked_on parent is flagged" 'has_finding stack-invalid 71'
assert "a cycle is flagged" 'has_finding stack-invalid 72'
assert "a populated branch with no remote ref is flagged" 'has_finding stack-invalid 73'
assert "a killed parent flags its descendant separately" 'has_finding stack-parent-killed 74'
assert "a well-formed stack produces no finding" '! has_finding stack-invalid 75 && ! has_finding stack-parent-killed 75'
assert "an unstacked change produces no finding" '! has_finding stack-invalid 76'
```

- [ ] **Step 2: Run to verify it fails**

Run: `scripts/run-tests.sh --verbose tests/test_board_checks_stack.sh`
Expected: FAIL on the first four.

- [ ] **Step 3: Add the checks**

In `scripts/board-checks.sh`, source `scripts/lib/docket-stack.sh` and add two `# --- <name>` blocks
inside the per-file walk, scoped to non-terminal changes that carry a `stacked_on`. Call
`stack_effective_base` once and branch on its exit code: `3` → `emit stack-parent-killed`, `4` →
`emit stack-invalid`, `0` → silent. Write the message so it names the parent and the concrete
remedy — re-scope, re-parent, or kill for the killed case; repair the id, break the cycle, or push
the parent's branch for the invalid case.

Call the resolver **once** and reuse the code; calling it twice for two checks would let the two
legs disagree on the same input.

- [ ] **Step 4: Run to verify it passes**

Run: `scripts/run-tests.sh --verbose tests/test_board_checks_stack.sh`
Expected: PASS.

- [ ] **Step 5: Move all four pinned surfaces together**

Add both ids to `BOARD_CHECK_IDS` in `scripts/lib/docket-frontmatter.sh`; add both to the
enumeration in `scripts/board-checks.sh`'s own `--help` header; add a per-check section to
`scripts/board-checks.md`; add a `check` report-line row to `scripts/docket-status.md`. Then run
`tests/test_board_checks.sh` — its bidirectional correspondence guard is what proves all four moved,
so a pass there is the receipt, and a failure names the surface you missed.

- [ ] **Step 6: Budget row, full suite, mutation**

Add the budget row (sibling shard — `tests/test_board_checks.sh` is at 55s), re-seed
`EXPECTED_TOTAL`, write the header rationale.

Run: `scripts/run-tests.sh`
Expected: PASS, no `OVER BUDGET:` line.

Mutation: collapse the two checks into one that emits `stack-invalid` for exit 3 as well. The
"a killed parent flags its descendant separately" assert must redden. Restore from your backup copy.

- [ ] **Step 7: Commit**

```bash
git add scripts/board-checks.sh scripts/board-checks.md scripts/docket-status.md \
        scripts/lib/docket-frontmatter.sh tests/test_board_checks_stack.sh \
        tests/test_board_checks.sh tests/runtime-budgets.tsv tests/test_runtime_budgets.sh
git commit -m "feat(0298): stack-invalid and stack-parent-killed health checks"
```

---

### Task 10: `verify-run` accepts the new state

A change that reached `stacked-merged` completed its implement-next run; `verify-run` must not call
it unclaimed.

**Files:**
- Modify: `scripts/verify-run.sh`, `scripts/verify-run.md`
- Test: the existing `verify-run` test file — find it with `grep -rln verify-run tests/` and extend the one that drives the CLI

**Interfaces:**
- Consumes: the `stacked-merged` status (Task 1).
- Produces: nothing for later tasks.

- [ ] **Step 1: Write the failing test**

```bash
assert "a stacked-merged change reports run-complete" \
  '[ "$("$SCRIPT" 81 --changes-dir "$tmp")" = "run-complete 81" ]'
assert "a stacked-merged change is not reported unclaimed" \
  '! "$SCRIPT" 81 --changes-dir "$tmp" | grep -qF run-unclaimed'
```

The second assert pipes a producer into `grep -q`; capture first instead:

```bash
v="$("$SCRIPT" 81 --changes-dir "$tmp")"
assert "a stacked-merged change is not reported unclaimed" '! grep -qF run-unclaimed <<<"$v"'
```

Fixture 81: `status: stacked-merged`, with `pr:` and `branch:` set.

- [ ] **Step 2: Run to verify it fails**

Run: `scripts/run-tests.sh --verbose tests/<the file you extended>`
Expected: FAIL — today the claim gate's alternation excludes the status and the run reports
`run-unclaimed`.

- [ ] **Step 3: Extend the claim gate**

In `scripts/verify-run.sh`, add `stacked-merged` to the claim-gate alternation, and wherever the
`status` conjunct of `run-complete` tests for `implemented`, accept `stacked-merged` too — a change
past `implemented` has satisfied that conjunct. Read the conjunct's actual spelling before editing;
do not assume it is a string equality.

- [ ] **Step 4: Run to verify it passes, then the full suite**

Run: `scripts/run-tests.sh --verbose tests/<the file you extended>`
Expected: PASS.

Run: `scripts/run-tests.sh`
Expected: PASS.

- [ ] **Step 5: Document and commit**

Update `scripts/verify-run.md`'s status list. Then:

```bash
git add scripts/verify-run.sh scripts/verify-run.md tests/
git commit -m "feat(0298): verify-run treats stacked-merged as a completed run"
```

---

### Task 11: The mechanics reference and the skill trigger lines

Progressive disclosure — the mechanics land in one reference; skill bodies gain one trigger line
each. This is also where the finalize gate, the retarget rule, and the killed-parent policy are
written down as agent-facing procedure, since they are skill behavior rather than script behavior.

**Files:**
- Create: `skills/docket-convention/references/stacked-changes.md`
- Modify: `skills/docket-convention/SKILL.md`, `skills/docket-implement-next/SKILL.md`, `skills/docket-finalize-change/SKILL.md`, `skills/docket-status/SKILL.md`, `skills/docket-new-change/SKILL.md`, `skills/docket-groom-next/SKILL.md`, `skills/docket-finalize-change/references/gate-failure.md`
- Modify: `README.md`, `.docket.example.yml` (if it enumerates statuses)
- Test: whichever suite file guards skill-body/reference correspondence — find it with `grep -rln "references/" tests/`

**Interfaces:**
- Consumes: every script interface from Tasks 3, 7, and 9.
- Produces: nothing for later tasks.

- [ ] **Step 1: Write the reference**

Create `skills/docket-convention/references/stacked-changes.md` covering, in order: the governing
invariant; `stacked_on:` and the cycle rule; the `stacked-merged` state and what it does and does not
satisfy (it satisfies `depends_on` for nothing); the effective-base resolver's four rules and the
`stack-base.sh` exit codes with the caller's obligation for each; the branch cut and PR base for a
stacked child; the finalize gate for a parent with open children — **autonomous finalize hard-blocks,
interactive finalize warns and lets the human override**; the explicit child-PR retarget **before**
the parent's branch is deleted, and the rule that a parent branch with open child PRs that cannot be
retargeted is retained rather than deleted; the killed-parent policy — open descendants flip to
`blocked` with `blocked_by: stack parent #A killed — re-scope, re-parent, or kill`, and
`stacked-merged` descendants flip to `blocked` too with their branches preserved and no false `done`
record; and the stack close-out's idempotency.

State plainly that the child's rebase is lazy — performed at the child's own next finalize gate — and
that docket never relies on GitHub's delete-time base retargeting.

- [ ] **Step 2: Add the trigger lines**

One line per skill body, phrased as a blocking on-trigger read. The trigger is: the change at hand
carries `stacked_on:`, or — for finalize and status — the affected change has stacked children.
Example shape for `docket-implement-next`, placed in Step 4 beside the worktree instruction:

```markdown
When the claimed change carries `stacked_on:`, **read
[`../docket-convention/references/stacked-changes.md`](../docket-convention/references/stacked-changes.md)
now (blocking)** — the feature branch is then cut from the resolved effective base, not from
`origin/<integration_branch>`, and the PR targets that base.
```

Adapt the sentence per skill; never restate the mechanics in the body.

- [ ] **Step 3: State the branch-model exception in the convention**

`skills/docket-convention/SKILL.md`'s *Feature branch invariants* and *Branch model* both say a
feature branch is ALWAYS cut from `origin/<integration_branch>`. Add the exception **alongside the
rule**, not as a silent contradiction: a change carrying `stacked_on:` is cut from its resolved
effective base, and the reference owns the resolution. Add the new reference to the convention's
pointer list.

- [ ] **Step 4: Scope the gate-failure argument**

`skills/docket-finalize-change/references/gate-failure.md` argues that an eighth status is the wrong
shape for recording a blocked finalize. That argument is still correct **for that case** and now
reads as contradicted by the existence of `stacked-merged`. Add a scoping clause: the objection is
to encoding a *transient, multi-cause abort* as a status; `stacked-merged` is a durable lifecycle
position with one cause and one exit, which is why it earns a status while finalize-blocked does not.
Do not delete the argument — it is load-bearing for its own decision.

- [ ] **Step 5: Verify no doc still contradicts the new state**

```bash
grep -rn "always cut from\|ALWAYS cut from\|seven\b" skills/ README.md scripts/*.md
```

Sort the hits into prose vs executable and fix every one that is now false.

- [ ] **Step 6: Run the full suite**

Run: `scripts/run-tests.sh`
Expected: PASS. Skill-body and reference-correspondence guards are the ones most likely to fire
here; read each failure as naming a surface you missed rather than as a test to relax.

- [ ] **Step 7: Commit**

```bash
git add skills/ README.md .docket.example.yml tests/
git commit -m "docs(0298): stacked-changes mechanics reference and skill trigger lines"
```

---

## Self-review

**Spec coverage.** §1 declaration → Task 2. §2 the `stacked-merged` state → Task 1 (vocabulary and
rendering) plus Task 6 (the producer) plus Task 7 (the promotion). §3 resolver → Task 3. §4 readiness
→ Task 4. §5 build mechanics → Task 11's trigger line into `docket-implement-next` (the branch cut is
skill behavior over `stack-base.sh`). §6 child PR and merge gate → Task 6 plus Task 11. §7 stack
close-out → Tasks 7 and 8. §8 parent finalize gate and retargeting → Task 11. §9 killed-parent policy
→ Task 9 (detection) plus Task 11 (the policy). §10 artifact flow → Task 5 (branch-addressed links)
plus Task 7 (the Stack carried table). §11 reciprocal visibility → Task 5. §12 board → Tasks 1 and 4.

**Known gap, stated rather than hidden:** §8's *explicit child-PR retarget before parent-branch
deletion* is specified in Task 11 as skill procedure, not as a deterministic script. That is a
deliberate scoping call — the retarget is a `gh pr edit --base` call the finalize skill already has
the surface for, and giving it its own script would need its own contract, tests, and a mock `gh`
seam for a single call. If the build's reviewer judges that it belongs in a script, that is a
legitimate finding; it is not an oversight.

**Placeholder scan.** No step says TBD, "add error handling", or "similar to Task N". Every code step
carries the code. Where a step says to match a surrounding variable's name, that is a deliberate
instruction to read before writing — the alternative would be a plan that hardcodes a name it has not
verified, which is the more dangerous failure.

**Type consistency.** `stack_find_file`, `stack_parent_id`, `stack_chain` (Task 2) are used under
exactly those names in Tasks 3, 5, and 7. `stack_effective_base` (Task 3) is used under that name in
Tasks 4 and 9. `stack_descendants` (Task 7) is used only within Task 7. The digest token
`stack-base-unresolved` and the board cell `waiting on #A — stack base not built` (Task 4) are
spelled identically in their asserts. The sweep line `stacked-merged <id> <parent>` (Task 6) is
consumed under that spelling in Task 8.
