# Headless Finalize — Disposition Contract Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give `docket-finalize-change` the same four-disposition driver contract `docket-implement-next` already has, plus id-set scoping, mergeability-ordered selection, and a `## Finalize blocked` marker surfaced on the board — so a driver can close out merged work hands-free.

**Architecture:** Three of the four tasks are **prose on skill/README markdown** guarded by grep sentinels — this mirrors change 0088, which deliberately shipped no scripts for the implement-side contract. Exactly one task is real code: the board renderer gains a cell for the new `## Finalize blocked` body section, using the `has_section` primitive the parallel `auto-groom-blocked` token already uses. Selection and ordering are executed by the agent reading the skill, not by a script, so they are guarded as documented-order sentinels rather than unit tests.

**Tech Stack:** Bash 3.2-compatible shell (macOS default), grep/awk sentinel tests, markdown skills. No new dependencies.

## Global Constraints

Copied verbatim from the spec and `AGENTS.md`; every task's requirements implicitly include these.

- **Vocabulary parity is the point.** The four disposition words are `advanced`, `contended`, `drained`, `halted` — identical to `docket-implement-next`'s, code-formatted with backticks. Never invent a fifth or rename one.
- **One merge per invocation.** A run merges exactly one change and exits `advanced`; it never batches.
- **No loop primitive, no `docket-drain` skill, no new entry surface.** docket owns the contract, not the driver.
- **Not a change to the seven-state lifecycle.** `## Finalize blocked` is a body section, never an eighth status and never a reuse of `blocked`.
- **`drained` must keep meaning "genuinely nothing to do."** A non-empty candidate set in which every member needs a human is `halted`.
- Shell: never `producer | grep -q` under `set -o pipefail` — capture into a variable, then `grep <<<"$var"`.
- Shell: `grep` for a pattern leading with `--` must use `-e` or `-F --`.
- Guards: a guard is code — mutation-test it (strip what it guards, watch it redden) or it is decoration.
- Run the **whole suite** at the build gate, never only the tests this plan enumerates.
- Every skill file edit must keep `tests/test_skill_size_budgets.sh` green — raise the budget row **in the same diff** if the file grows past it (the guard explicitly permits this).

**Working tree:** all paths below are relative to `/Users/homer/dev/docket/.worktrees/headless-finalize-driver` (branch `feat/headless-finalize-driver`, cut from `origin/main` @ `e0fbf89`). Never write docket metadata (change files, `BOARD.md`, ADRs) from this tree.

**Whole-suite command** (used at the end of every task):

```bash
cd /Users/homer/dev/docket/.worktrees/headless-finalize-driver
fail=0
for t in tests/test_*.sh; do
  out="$(bash "$t" 2>&1)"
  if grep -q "NOT OK" <<<"$out"; then echo "### $t"; grep "NOT OK" <<<"$out"; fail=1; fi
done
echo "SUITE fail=$fail"
```

Expected: `SUITE fail=0`. This is a single **foreground** call — never background it.

---

## File Structure

| File | Responsibility | Task |
|---|---|---|
| `skills/docket-finalize-change/SKILL.md` | The disposition contract, id-set scoping, mergeability ordering, `## Finalize blocked` semantics | 1, 2 |
| `tests/test_finalize_disposition.sh` | **Create.** Sentinels for all of the above + the README doc | 1, 2, 4 |
| `tests/test_skill_size_budgets.sh` | Budget rows raised in-diff as the skill grows | 1, 2 |
| `skills/docket-convention/SKILL.md` | One line adding `## Finalize blocked` to the enumerated body-section list | 2 |
| `scripts/lib/docket-frontmatter.sh` | `finalize_blocked` predicate | 3 |
| `scripts/render-board.sh` | The `implemented` table's new Readiness cell + digest parity | 3 |
| `scripts/render-board.md` | Contract update for the new cell | 3 |
| `tests/test_render_board.sh` | Golden update + focused positive/negative + digest parity | 3 |
| `README.md` | The `/loop docket-finalize-change` drain-pattern subsection | 4 |

---

### Task 1: The terminal disposition contract + id-set scoping + mergeability ordering

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md` (add a section after the `## Selection` block; extend `## Selection` itself)
- Modify: `tests/test_skill_size_budgets.sh:31` (the `skills/docket-finalize-change/SKILL.md` row)
- Test: `tests/test_finalize_disposition.sh` (create)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: the exact section heading `## Terminal disposition (driver contract)` and the four code-formatted disposition tokens, which Task 4's README sentinels mirror. Task 2 appends a subsection to this same skill file and must re-check the budget raised here.

- [ ] **Step 1: Write the failing test**

Create `tests/test_finalize_disposition.sh` with exactly this content:

```bash
#!/usr/bin/env bash
# tests/test_finalize_disposition.sh — guards change 0087 (headless finalize: the finalize-side
# disposition contract, mirroring 0088). Asserts the four-disposition terminal contract, id-set
# scoping, the mergeability ordering keys IN ORDER, the `## Finalize blocked` marker semantics,
# and the README drain-pattern doc.
# Sentinels are sampling, not parsing (learnings: foundational-test-discipline) — pair with the
# whole-branch review; this test does not replace it.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if ( eval "$2" ); then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

FIN="$REPO/skills/docket-finalize-change/SKILL.md"

# --- SKILL.md: the four-disposition terminal contract ---
assert "SKILL has a Terminal disposition section" 'grep -Eqi "Terminal disposition" "$FIN"'
for d in advanced contended drained halted; do
  tok="\`$d\`"
  assert "SKILL names disposition $d (code-formatted)" 'grep -qF "$tok" "$FIN"'
done
# The binary driver rule — both halves must be present (non-vacuous).
assert "SKILL states continue-on advanced/contended" 'grep -Eqi "continue on .{0,4}advanced" "$FIN"'
assert "SKILL states stop-on drained/halted" 'grep -Eqi "stop on .{0,4}drained" "$FIN"'
assert "SKILL enumerates skipped-with-reason" 'grep -Eqi "skipped with (its|the) reason" "$FIN"'

# --- SKILL.md: the finalize-specific disposition semantics ---
assert "SKILL ties every abort-and-report point to halted" \
  'grep -Eqi "abort-and-report point.{0,40}(is|are|maps to|→).{0,20}\`?halted" "$FIN"'
assert "SKILL states a blocked-but-non-empty set is halted, not drained" \
  'grep -Eqi "halted.{0,30}(never|not).{0,10}\`?drained" "$FIN"'
assert "SKILL states one merge per invocation" \
  'grep -Eqi "exactly one|one merge per invocation" "$FIN"'
assert "SKILL states it never batches" 'grep -Eqi "never batch" "$FIN"'

# --- SKILL.md: id-set scoping ---
assert "SKILL documents an id allowlist" 'grep -Eqi "allowlist" "$FIN"'
assert "SKILL shows the comma-separated id-set form" 'grep -Eq "docket-finalize-change 90,92,94" "$FIN"'
assert "SKILL states naming the ids IS the authorization" \
  'grep -Eqi "naming the ids.{0,30}authorization" "$FIN"'
assert "SKILL ties the allowlist to the require_pr_approval override" \
  'grep -q "require_pr_approval" "$FIN"'

# --- SKILL.md: mergeability ordering, asserted IN ORDER (order is part of the contract) ---
# NOTE: never `grep … | head` under `set -o pipefail` (AGENTS.md) — the producer takes SIGPIPE and
# the 141 becomes an intermittent failure. Capture the whole match set, then take the first line
# with parameter expansion.
first_line_no(){ # first_line_no ERE -> line number of the first matching line, empty if none
  local m; m="$(grep -nEi -e "$1" "$FIN" || true)"
  [ -n "$m" ] || return 0
  m="${m%%$'\n'*}"        # first match only
  printf '%s' "${m%%:*}"  # strip everything from the first colon
}
p_dep="$(first_line_no '^[[:space:]]*1\..*depends_on')"
p_mrg="$(first_line_no '^[[:space:]]*2\..*mergeable')"
p_dif="$(first_line_no '^[[:space:]]*3\..*(smallest diff|changedFiles)')"
p_tie="$(first_line_no '^[[:space:]]*4\..*priority')"
assert "ordering key 1 is depends_on" '[ -n "$p_dep" ]'
assert "ordering key 2 is mergeable" '[ -n "$p_mrg" ]'
assert "ordering key 3 is diff size" '[ -n "$p_dif" ]'
assert "ordering key 4 is the priority tiebreak" '[ -n "$p_tie" ]'
assert "the four ordering keys appear in contract order" \
  '[ -n "$p_dep" ] && [ -n "$p_mrg" ] && [ -n "$p_dif" ] && [ -n "$p_tie" ] &&
   [ "$p_dep" -lt "$p_mrg" ] && [ "$p_mrg" -lt "$p_dif" ] && [ "$p_dif" -lt "$p_tie" ]'
assert "SKILL excludes CONFLICTING from selection" 'grep -q "CONFLICTING" "$FIN"'
assert "SKILL documents the lazy-mergeable poll" \
  'grep -q "UNKNOWN" "$FIN" && grep -Eqi "poll" "$FIN"'
assert "SKILL forbids pairwise file-overlap ranking" \
  'grep -Eqi "(not|never|do not|don.t) build pairwise|pairwise file-overlap" "$FIN"'

# --- Non-vacuity / mutation proof: the code-formatted disposition grep actually bites. ---
probe="$(mktemp)"; printf 'plain advanced word, no code formatting\n' > "$probe"
assert "the code-formatted disposition grep is non-vacuous" '! grep -qF "\`advanced\`" "$probe"'
# Non-vacuity for the ordering comparison: a reversed pair must fail the same test.
assert "the ordering comparison is non-vacuous (9 < 3 is caught)" '! [ 9 -lt 3 ]'
rm -f "$probe"

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/homer/dev/docket/.worktrees/headless-finalize-driver && bash tests/test_finalize_disposition.sh`

Expected: `FAIL`, with `NOT OK` lines for the disposition section, all four disposition tokens, the binary rule, id-set scoping, and every ordering key (the skill carries none of this today — verified at reconcile: 0 matches for all four words).

- [ ] **Step 3: Add the Selection additions to the skill**

In `skills/docket-finalize-change/SKILL.md`, find the `## Selection` section. Replace the line that currently reads:

```markdown
Given an explicit change id, OR auto-detect.
```

with:

```markdown
Given an explicit change id or an **id allowlist**, OR auto-detect.
```

Then, immediately after the existing **Explicit id** paragraph (the one ending "The approval policy governs only the auto-detect path."), insert this new paragraph:

```markdown
**Id allowlist** (`docket-finalize-change 90,92,94`) — generalizes the explicit-id form; a single id is the degenerate case. The set bounds *which changes are eligible*; the run still merges only the best-ordered one of them (see *Ordering* below). **Naming the ids IS the authorization** the multi-candidate prompt would otherwise have collected, so an allowlist never prompts and overrides `require_pr_approval` exactly as a single explicit id does. A scoped id that is not eligible is **skipped with its reason**, never force-merged, and never aborts the run. Unset ⇒ every eligible `implemented` change is a candidate.
```

Then, immediately after the Selection matrix's closing paragraph (the one ending "covers only states the gate can't act on."), insert:

```markdown
**Ordering — by mergeability, not priority.** The goal is to close out as many changes as possible per drain, so selection maximizes each attempt's chance of success. Among eligible candidates, take the head of:

1. **`depends_on` order** — a hard correctness constraint, not a preference: a dependency is satisfied only at `done`, so a dependent never merges ahead of its dependency however mergeable it looks.
2. **GitHub's `mergeable` field** — `CONFLICTING` is excluded from selection entirely and marked per *Finalize blocked* below.
3. **Smallest diff first** — `changedFiles`, then `additions + deletions`: cheaper to re-test, less likely to redden the suite after rebase, and lands the most changes before any halt.
4. **`priority` → `created` → lowest id** — the final tiebreak. Priority is *demoted*, not deleted: it still encodes human intent and guarantees a total, reproducible order.

Probe with `gh pr view <n> --json mergeable,mergeStateStatus,changedFiles,additions,deletions`. **GitHub computes `mergeable` lazily** — the first query returns `UNKNOWN` and only *triggers* the computation — so poll, bounded, and treat a still-`UNKNOWN` result as "attempt it": the rebase-retest gate is the real arbiter, and a wrong guess costs one gate run, not correctness. Do **not** build pairwise file-overlap ranking — measured against this repo's real backlog on 2026-07-18, it discriminates nothing and costs O(n) extra `gh` calls per invocation. Revisit only on evidence.

**Re-selection replaces sequencing.** Each invocation re-derives "best next" against the **current** `origin/<integration_branch>`, so no precomputed order can go stale — the direct answer to the moving base, since every merge moves it and an order authored before the first merge is a prediction about a base that no longer exists by the third.
```

- [ ] **Step 4: Add the terminal disposition section to the skill**

In the same file, insert this section immediately **before** the `## Per-change steps` heading:

```markdown
## Terminal disposition (driver contract)

Every run ends by declaring exactly **one** of four dispositions — the **same four words** `docket-implement-next` uses, so one driver keys on both skills without knowing which it is driving:

| Disposition | Meaning | Driver action |
|---|---|---|
| `advanced` | Merged one change → closed out (the per-change steps ran to completion). | continue |
| `contended` | Another writer got there first — the `docket-status` sweep archived it between selection and close-out; the archive is an idempotent no-op, so **nothing merged**. | continue — re-select next |
| `drained` | No eligible `implemented` change in scope. | **stop** |
| `halted` | Any abort-and-report point fired, **or** every member of a non-empty eligible set needs a human. | **stop + surface** |

The driver's decision is binary: **continue on `advanced`/`contended`, stop on `drained`/`halted`.** The contract is **driver-agnostic** — `/loop`, cron, a scheduled agent, or a human re-typing the command are all equally valid; docket owns the contract, never the driver.

**One merge per invocation.** A run merges **exactly one** change and exits `advanced`; it **never batches**. Consecutive close-outs come from the driver re-invoking, not from an in-run loop — which is also what keeps the blast-radius posture the multi-candidate prompt was protecting.

**Every abort-and-report point maps to `halted`.** The set enumerated in *abort-and-report points (the full set)* below is unchanged — this **names** existing behavior and adds none.

**A blocked-but-non-empty set is `halted`, never `drained`.** There *is* work; it just needs a human. `drained` must keep meaning "genuinely nothing to do," or the driver's stop signal loses its meaning.

The final report **enumerates** the change merged (if any), each change **skipped with its reason** (outside the id allowlist / not git-mergeable / unapproved under `require_pr_approval` / already carrying `## Finalize blocked` / waiting on an unmerged `depends_on`), and which disposition ended the run.
```

- [ ] **Step 5: Raise the size budget in the same diff**

Measure the grown file:

```bash
cd /Users/homer/dev/docket/.worktrees/headless-finalize-driver
wc -l < skills/docket-finalize-change/SKILL.md
wc -w < skills/docket-finalize-change/SKILL.md
```

In `tests/test_skill_size_budgets.sh`, edit the row

```
skills/docket-finalize-change/SKILL.md                     160 2699
```

replacing `160` and `2699` with **ceil(measured_lines × 1.1)** and **ceil(measured_words × 1.1)** — the same +10% headroom convention the table's comment documents. Keep the column alignment of the surrounding rows.

- [ ] **Step 6: Run tests to verify they pass**

Run: `cd /Users/homer/dev/docket/.worktrees/headless-finalize-driver && bash tests/test_finalize_disposition.sh && bash tests/test_skill_size_budgets.sh`

Expected: both print `PASS` with no `NOT OK` lines.

- [ ] **Step 7: Mutation-prove the new sentinels bite**

For each of three representative assertions, delete what it guards, confirm `NOT OK`, then restore:

```bash
cd /Users/homer/dev/docket/.worktrees/headless-finalize-driver
cp skills/docket-finalize-change/SKILL.md /tmp/fin.bak
# (a) remove the `contended` row
sed -i '' '/| `contended` |/d' skills/docket-finalize-change/SKILL.md
bash tests/test_finalize_disposition.sh | grep "NOT OK"   # expect the contended assert to fire
cp /tmp/fin.bak skills/docket-finalize-change/SKILL.md
# (b) swap ordering keys 2 and 3 so the order assert must redden
python3 - <<'PY'
import re,io
p='skills/docket-finalize-change/SKILL.md'
s=open(p).read()
s=s.replace('2. **GitHub','2. **TEMPSWAP').replace('3. **Smallest','2. **GitHub').replace('2. **TEMPSWAP','3. **Smallest')
open(p,'w').write(s)
PY
bash tests/test_finalize_disposition.sh | grep "NOT OK"   # expect the in-order assert to fire
cp /tmp/fin.bak skills/docket-finalize-change/SKILL.md
# (c) remove the id-set example
sed -i '' 's/docket-finalize-change 90,92,94/docket-finalize-change <id>/' skills/docket-finalize-change/SKILL.md
bash tests/test_finalize_disposition.sh | grep "NOT OK"   # expect the comma-separated-form assert to fire
cp /tmp/fin.bak skills/docket-finalize-change/SKILL.md
rm -f /tmp/fin.bak
bash tests/test_finalize_disposition.sh                    # expect PASS again
```

Expected: each mutation prints at least one `NOT OK` naming the guarded clause; the restore returns `PASS`. If any mutation leaves the suite green, that assertion is decoration — fix it before continuing.

- [ ] **Step 8: Run the whole suite**

Run the **Whole-suite command** from Global Constraints (one foreground call).

Expected: `SUITE fail=0`.

- [ ] **Step 9: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/headless-finalize-driver
chmod +x tests/test_finalize_disposition.sh
git add skills/docket-finalize-change/SKILL.md tests/test_finalize_disposition.sh tests/test_skill_size_budgets.sh
git commit -m "feat(0087): finalize terminal disposition contract + id-set scoping + mergeability ordering"
```

---

### Task 2: The `## Finalize blocked` marker semantics

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md` (new subsection under the gate)
- Modify: `skills/docket-convention/SKILL.md` (one line in the enumerated body-section list)
- Modify: `tests/test_finalize_disposition.sh` (add the marker sentinels)
- Modify: `tests/test_skill_size_budgets.sh` (re-check both rows)

**Interfaces:**
- Consumes: Task 1's `## Terminal disposition (driver contract)` section and its *Ordering* block, which already forward-reference "*Finalize blocked* below".
- Produces: the exact literal section name **`## Finalize blocked`** and the exact board cell wording **`finalize blocked — needs you`** (em dash, lowercase). Task 3 keys its renderer and its golden on both strings verbatim; Task 4's README repeats the cell wording.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_finalize_disposition.sh`, immediately **before** the `# --- Non-vacuity / mutation proof` block:

```bash
# --- SKILL.md: the `## Finalize blocked` marker (D4) ---
assert "SKILL names the Finalize blocked section" 'grep -qF "## Finalize blocked" "$FIN"'
assert "SKILL states it is NOT a new status" \
  'grep -Eqi "not (a new|an eighth) status|never an eighth status" "$FIN"'
assert "SKILL states it is not a reuse of blocked" \
  'grep -Eqi "(not|never) a reuse of .{0,3}\`?blocked" "$FIN"'
assert "SKILL states selection SKIPS a marked change" \
  'grep -Eqi "skip.{0,40}(carrying|marked|section)" "$FIN"'
assert "SKILL states a CONFLICTING PR met during selection is marked too" \
  'grep -Eqi "CONFLICTING.{0,80}mark|mark.{0,80}CONFLICTING" "$FIN"'
assert "SKILL states a successful finalize CLEARS the section" \
  'grep -Eqi "(remove|clear)s?.{0,40}section|section.{0,40}(removed|cleared)" "$FIN"'
assert "SKILL names the board cell wording" 'grep -qF "finalize blocked — needs you" "$FIN"'
assert "SKILL says the marker is a metadata write" \
  'grep -Eqi "metadata (write|branch)" "$FIN"'

CONV="$REPO/skills/docket-convention/SKILL.md"
assert "convention lists the Finalize blocked body section" 'grep -qF "## Finalize blocked" "$CONV"'
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/homer/dev/docket/.worktrees/headless-finalize-driver && bash tests/test_finalize_disposition.sh`

Expected: `FAIL` with `NOT OK` for all nine new assertions.

- [ ] **Step 3: Add the marker subsection to the skill**

In `skills/docket-finalize-change/SKILL.md`, insert this subsection immediately **after** the `### abort-and-report points (the full set)` block (i.e. after the "**Where the reason surfaces.**" paragraph) and **before** `## Where finishing-a-development-branch fits`:

```markdown
### `## Finalize blocked` — marking a change that needs a human

A gate failure is recorded as a dated `## Finalize blocked` body section on the change file — a **metadata write** on `metadata_branch` like any other field write, appended in the metadata working tree and pushed immediately. It is deliberately **not a new status** and **not a reuse of `blocked`**: the change really *is* `implemented` with an open PR, and an eighth status would flatten the six distinct abort reasons into one label while forcing changes to the lifecycle diagram, the board renderer, the GitHub mirror's seven-state mapping, and the health checks. Shape mirrors the proven `## Auto-groom blocked`: a dated entry naming **which** reason fired and what the human must do.

- **Selection skips** any change already carrying the section — without this a re-run re-selects the same known-bad change forever and the drain never progresses past it.
- **A `CONFLICTING` PR met during selection is marked too** — a cheap, idempotent metadata write — so the board surfaces every change needing attention, not only the one this run touched.
- **A successful finalize removes the section.** Unlike auto-groom's human-only re-arm, the condition is machine-verifiable (the gate passed), so requiring a human to delete it would strand stale markers on changes that are fine. State encoded by an artifact's presence must be cleared by every transition out of that state.
- The board renders a change carrying it as **`finalize blocked — needs you`**, parallel to `auto-groom blocked — needs you`.
```

- [ ] **Step 4: Add the body section to the convention's enumerated list**

In `skills/docket-convention/SKILL.md`, find the `### Change body sections` list and the bullet beginning `- \`## Auto-groom blocked\``. Insert directly **after** that bullet:

```markdown
- `## Finalize blocked` — dated record appended by `docket-finalize-change` when a gate failure leaves a change needing a human; presence drives the board's `finalize blocked — needs you` cell and makes later finalize runs skip the change. Cleared automatically by a successful finalize (not a human re-arm).
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /Users/homer/dev/docket/.worktrees/headless-finalize-driver && bash tests/test_finalize_disposition.sh && bash tests/test_skill_size_budgets.sh`

Expected: `test_finalize_disposition.sh` prints `PASS`. If `test_skill_size_budgets.sh` reports `NOT OK` for either `skills/docket-finalize-change/SKILL.md` or `skills/docket-convention/SKILL.md`, re-measure that file with `wc -l` / `wc -w` and raise its row to ceil(actual × 1.1) — in this same diff — then re-run until both print `PASS`.

- [ ] **Step 6: Mutation-prove the clearing assertion bites**

```bash
cd /Users/homer/dev/docket/.worktrees/headless-finalize-driver
cp skills/docket-finalize-change/SKILL.md /tmp/fin2.bak
sed -i '' '/successful finalize removes the section/d' skills/docket-finalize-change/SKILL.md
bash tests/test_finalize_disposition.sh | grep "NOT OK"   # expect the clears-the-section assert to fire
cp /tmp/fin2.bak skills/docket-finalize-change/SKILL.md
rm -f /tmp/fin2.bak
bash tests/test_finalize_disposition.sh                    # expect PASS
```

Expected: the deletion prints a `NOT OK` naming the clearing assertion; the restore returns `PASS`.

- [ ] **Step 7: Run the whole suite**

Run the **Whole-suite command**. Expected: `SUITE fail=0`.

- [ ] **Step 8: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/headless-finalize-driver
git add skills/docket-finalize-change/SKILL.md skills/docket-convention/SKILL.md tests/test_finalize_disposition.sh tests/test_skill_size_budgets.sh
git commit -m "feat(0087): the Finalize blocked marker — semantics, clearing rule, convention entry"
```

---

### Task 3: The board cell for `## Finalize blocked`

**Files:**
- Modify: `scripts/lib/docket-frontmatter.sh` (add `finalize_blocked` beside `readiness`)
- Modify: `scripts/render-board.sh` (implemented-table header + row + `digest_readiness`)
- Modify: `scripts/render-board.md` (contract)
- Test: `tests/test_render_board.sh` (fixture, golden, focused asserts, digest parity)

**Interfaces:**
- Consumes: Task 2's literal section name `## Finalize blocked` and cell wording `finalize blocked — needs you`.
- Produces: `finalize_blocked FILE` → exit 0 when the file carries the section, non-zero otherwise (sourced from `scripts/lib/docket-frontmatter.sh`, same contract shape as the existing `has_section FILE STRING`). Digest token `finalize-blocked` for a marked `implemented` change, `-` otherwise.

**Why this is a new render path, not a `readiness()` change** (confirmed at reconcile): `readiness()` is by contract meaningful only for a `proposed` change, and `render-board.sh`'s `readiness_cell` is reached only from the `proposed` branch of `print_section`. The marker applies to an `implemented` change. Do not extend `readiness()`.

- [ ] **Step 1: Write the failing test — fixture + golden**

In `tests/test_render_board.sh`, add a new fixture immediately after the `0009-india.md` heredoc block and before the `0010-juliet` archive block:

```bash
cat > "$tmp/active/0013-mike.md" <<'EOF'
---
id: 13
slug: mike
title: Mike feature
status: implemented
priority: high
depends_on: []
pr: https://github.com/o/r/pull/151
---

## Finalize blocked

2026-07-18 — ambiguous rebase conflict; resolve by hand and re-run.
EOF
```

Then update the golden in three places.

(a) The count line becomes:

```
**13 changes** — 🟢 1 in progress · 🟡 5 proposed · 🔴 1 blocked · ⚪ 1 deferred · 🔵 2 implemented · ✅ 2 done · 🗑️ 1 killed
```

(b) The implemented section becomes:

```
## 🔵 Implemented — awaiting merge (2)

| # | Title | Priority | PR | Readiness |
|---|-------|----------|----|-----------|
| [0008](active/0008-hotel.md) | Hotel feature | `high` | [#142](https://github.com/o/r/pull/142) |  |
| [0013](active/0013-mike.md) | Mike feature | `high` | [#151](https://github.com/o/r/pull/151) | finalize blocked — needs you |
```

Note the unmarked row's trailing empty cell renders as `|  |` — a pipe, two spaces, a pipe.

(c) In the mermaid block, add a bare node line `  0013` immediately after `  0009`:

```
  0008
  0009
  0013
  0010:::done
```

- [ ] **Step 2: Add the focused positive/negative and digest-parity asserts**

In the same file, immediately after the existing `rm -rf "$bare"` line (the end of the bare-PR fallback block), insert:

```bash
# --- change 0087: the `## Finalize blocked` cell on the implemented table -----------------------
# Positive and negative in one render: 0013 carries the section, 0008 does not. The golden already
# byte-checks both; these focused asserts name the invariant so a golden re-blessing cannot quietly
# drop it (learnings: guards-are-code).
assert "a marked implemented change renders the finalize-blocked cell" \
  'grep -qF "| finalize blocked — needs you |" "$rendered"'
assert "an unmarked implemented change renders an empty readiness cell" \
  'grep -qF "| [#142](https://github.com/o/r/pull/142) |  |" "$rendered"'
assert "the implemented table carries the Readiness column" \
  'grep -qF "| # | Title | Priority | PR | Readiness |" "$rendered"'

# Digest parity (change 0069's invariant: the digest can never disagree with the board).
digest="$(bash "$SCRIPT" --changes-dir "$tmp" --format digest 2>/dev/null)"
assert "digest reports finalize-blocked for the marked change" \
  'grep -qF "change 13 implemented finalize-blocked mike" <<<"$digest"'
assert "digest reports - for the unmarked implemented change" \
  'grep -qF "change 8 implemented - hotel" <<<"$digest"'

# Non-vacuity: the marker predicate must key on the section, not on status alone. A copy of the
# marked fixture with the section stripped must render the EMPTY cell.
nomark="$(mktemp -d)"; mkdir -p "$nomark/active" "$nomark/archive"
sed '/## Finalize blocked/,$d' "$tmp/active/0013-mike.md" > "$nomark/active/0013-mike.md"
nomarkout="$(bash "$SCRIPT" --changes-dir "$nomark" --repo o/r 2>/dev/null)"
assert "stripping the section drops the cell (predicate is non-vacuous)" \
  '! grep -qF "finalize blocked — needs you" <<<"$nomarkout"'
rm -rf "$nomark"
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd /Users/homer/dev/docket/.worktrees/headless-finalize-driver && bash tests/test_render_board.sh`

Expected: `NOT OK - rendered output matches the golden byte-for-byte` with a diff showing the missing `Readiness` column, plus `NOT OK` for each of the focused and digest asserts.

- [ ] **Step 4: Add the `finalize_blocked` predicate**

In `scripts/lib/docket-frontmatter.sh`, immediately after the `readiness()` function's closing brace, add:

```bash
finalize_blocked(){ # finalize_blocked FILE  (only meaningful for an implemented change)
  # `## Finalize blocked` is presence-encoded state written by docket-finalize-change when a gate
  # failure leaves a change needing a human. Deliberately NOT part of readiness(), which is by
  # contract meaningful only for a `proposed` change.
  has_section "$1" "## Finalize blocked"
}
```

- [ ] **Step 5: Render the cell**

In `scripts/render-board.sh`, immediately after the `readiness_cell()` function's closing brace, add:

```bash
implemented_cell(){ # implemented_cell FILE  (implemented)
  if finalize_blocked "$1"; then printf 'finalize blocked — needs you'; fi
}
```

In `print_section`, change the `implemented` header line from:

```bash
    implemented) printf '| # | Title | Priority | PR |\n|---|-------|----------|----|\n' ;;
```

to:

```bash
    implemented) printf '| # | Title | Priority | PR | Readiness |\n|---|-------|----------|----|-----------|\n' ;;
```

and change the `implemented` row emission from:

```bash
      implemented)
        printf '| [%s](active/%s) | %s | `%s` | %s |\n' \
          "$(pad "$id")" "$base" "$title" "$priority" "$(pr_cell "$f")" ;;
```

to:

```bash
      implemented)
        printf '| [%s](active/%s) | %s | `%s` | %s | %s |\n' \
          "$(pad "$id")" "$base" "$title" "$priority" "$(pr_cell "$f")" "$(implemented_cell "$f")" ;;
```

- [ ] **Step 6: Keep the digest in parity**

In `scripts/render-board.sh`, in `digest_readiness()`, replace the early-return line:

```bash
  [ "$st" = proposed ] || { printf '%s' '-'; return; }
```

with:

```bash
  # `implemented` carries its own presence-encoded readiness (change 0087); every other
  # non-proposed status has none. Readiness still has exactly one owner per status, so the
  # digest can never disagree with the board.
  if [ "$st" = implemented ]; then
    if finalize_blocked "$f"; then printf 'finalize-blocked'; else printf '%s' '-'; fi
    return
  fi
  [ "$st" = proposed ] || { printf '%s' '-'; return; }
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `cd /Users/homer/dev/docket/.worktrees/headless-finalize-driver && bash tests/test_render_board.sh`

Expected: `PASS`, with the golden diff clean. If the mermaid node placement differs from Step 1(c), inspect the actual render — the mermaid ordering logic is untouched by this task, so any difference means the golden edit was placed wrong; fix the golden, never the renderer.

- [ ] **Step 8: Update the script contract**

In `scripts/render-board.md`, find the section describing the rendered sections/columns and add to the `Implemented` description:

```markdown
The `implemented` table carries a `Readiness` column: a change whose body has a `## Finalize blocked`
section (written by `docket-finalize-change` when a gate failure needs a human) renders
`finalize blocked — needs you`; every other implemented change renders an empty cell. The `digest`
format reports the same state as the token `finalize-blocked` (or `-`), so the two projections
cannot disagree.
```

Run `bash tests/test_script_contracts_coverage.sh` and confirm `PASS`.

- [ ] **Step 9: Verify against the real repo (the suite cannot see the metadata branch)**

The hermetic fixtures prove the renderer; they say nothing about the live board. Render the real metadata tree read-only and confirm no change is spuriously marked:

```bash
cd /Users/homer/dev/docket/.worktrees/headless-finalize-driver
bash scripts/render-board.sh --changes-dir /Users/homer/dev/docket/.docket/docs/changes --repo danielhanold/docket \
  | grep -A6 "Implemented"
```

Expected: the implemented table renders with the new `Readiness` column and **every** cell empty (no change on the live backlog carries the section yet). Record the observation for the results file.

- [ ] **Step 10: Run the whole suite**

Run the **Whole-suite command**. Expected: `SUITE fail=0`. Pay particular attention to `test_board_refresh.sh`, `test_board_checks.sh`, `test_docket_status.sh`, and `test_board_refresh_on_transition.sh` — all consume the renderer's output shape.

- [ ] **Step 11: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/headless-finalize-driver
git add scripts/lib/docket-frontmatter.sh scripts/render-board.sh scripts/render-board.md tests/test_render_board.sh
git commit -m "feat(0087): board renders finalize-blocked on the implemented table, digest in parity"
```

---

### Task 4: The README drain-pattern documentation

**Files:**
- Modify: `README.md` (new subsection after `### Draining hands-free with /loop`)
- Modify: `tests/test_finalize_disposition.sh` (README sentinels)

**Interfaces:**
- Consumes: Task 1's four dispositions and id-set form, Task 2's cell wording `finalize blocked — needs you`.
- Produces: nothing consumed by later tasks (final task).

- [ ] **Step 1: Write the failing test**

Append to `tests/test_finalize_disposition.sh`, immediately **before** the `# --- Non-vacuity / mutation proof` block:

```bash
# --- README: the /loop finalize drain-pattern doc ---
README="$REPO/README.md"
fb='`/loop docket-finalize-change`'
assert "README documents the /loop finalize drain" 'grep -qF "$fb" "$README"'
assert "README documents the /loop finalize id-set drain" \
  'grep -Eq "/loop docket-finalize-change 90,92,94" "$README"'
assert "README names all four dispositions for finalize" \
  'for d in advanced contended drained halted; do grep -qiF "$d" "$README" || exit 1; done'
assert "README states the binary continue/stop rule" \
  'grep -Eqi "continue on .{0,4}advanced" "$README" && grep -Eqi "stop on .{0,4}drained" "$README"'
assert "README states naming the ids is the authorization" \
  'grep -Eqi "naming the ids.{0,40}authorization" "$README"'
assert "README names the finalize-blocked board cell" \
  'grep -qF "finalize blocked — needs you" "$README"'
# The implement-side driver never merges; THIS one does. The distinction must be explicit, or a
# reader carries the wrong mental model across the two subsections.
assert "README states the finalize driver DOES merge" \
  'grep -Eqi "this driver (does|merges)|unlike the implementer" "$README"'
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/homer/dev/docket/.worktrees/headless-finalize-driver && bash tests/test_finalize_disposition.sh`

Expected: `FAIL` with `NOT OK` for all seven README assertions.

- [ ] **Step 3: Write the README subsection**

In `README.md`, insert this subsection immediately **after** the final paragraph of `### Draining hands-free with /loop` (the one ending "harness behavior is version- and mode-scoped.") and **before** the `---` that precedes `## Configuration`:

```markdown
### Closing out hands-free with `/loop`

`docket-finalize-change` ends every run declaring one of the **same four dispositions** — `advanced` (merged one change and closed it out), `contended` (another writer got there first, nothing merged), `drained` (nothing eligible in scope), or `halted` (needs a human) — so a single driver keys on both halves of the loop without knowing which one it is running: **continue on `advanced`/`contended`, stop on `drained`/`halted`.**

- `/loop docket-finalize-change` — closes out every eligible `implemented` change, **one merge per iteration**, stopping on `drained`.
- `/loop docket-finalize-change 90,92,94` — bounds the run to that id set. **Naming the ids is the authorization** the interactive multi-candidate prompt would otherwise have collected, so a scoped run merges without prompting — including PRs that `require_pr_approval` would otherwise hold.

Unlike the implementer, **this driver does merge** — that is the whole point of it, and it is the one place docket itself merges. Every merge still passes the rebase-retest gate, so `finalize.gate` remains your correctness control; set it to `off` only if you trust each PR's own CI.

Selection is ordered by *mergeability* rather than priority — `depends_on` order first (a hard constraint), then GitHub's `mergeable`, then the smallest diff, with priority → age → id as the tiebreak — so each drain lands as many changes as it can before anything stops it. A change whose gate fails is marked with a dated `## Finalize blocked` section, shows on the board as **finalize blocked — needs you**, and is skipped by later runs until a successful finalize clears it automatically. As with the implementer, confirm `/loop` composes cleanly in your own harness before relying on it unattended.
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/homer/dev/docket/.worktrees/headless-finalize-driver && bash tests/test_finalize_disposition.sh && bash tests/test_readme_finalize_docs.sh && bash tests/test_loop_continuation.sh`

Expected: all three print `PASS`. `test_readme_finalize_docs.sh` carries a negative assert that no live `auto_approve` reference returns to the README — the new prose must not mention it.

- [ ] **Step 5: Mutation-prove the README sentinels bite**

```bash
cd /Users/homer/dev/docket/.worktrees/headless-finalize-driver
cp README.md /tmp/rm.bak
sed -i '' 's|/loop docket-finalize-change 90,92,94|/loop docket-finalize-change <ids>|' README.md
bash tests/test_finalize_disposition.sh | grep "NOT OK"   # expect the id-set assert to fire
cp /tmp/rm.bak README.md
rm -f /tmp/rm.bak
bash tests/test_finalize_disposition.sh                    # expect PASS
```

Expected: the mutation prints a `NOT OK` for the id-set drain assertion; the restore returns `PASS`.

- [ ] **Step 6: Run the whole suite**

Run the **Whole-suite command**. Expected: `SUITE fail=0`.

- [ ] **Step 7: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/headless-finalize-driver
git add README.md tests/test_finalize_disposition.sh
git commit -m "docs(0087): README drain pattern for /loop docket-finalize-change"
```

---

## Deviations from the spec, recorded deliberately

**§7's "fixture-driven test over synthetic candidates" for selection ordering is not buildable as written.** Selection and ordering are executed by the agent reading skill prose — D5 and §3.1 explicitly ship "prose on the skill, no scripts." There is no function to drive with fixtures. The honest analogue, built in Task 1, is a sentinel that extracts the line numbers of the four ordering keys and asserts they appear **in contract order** (`depends_on` < `mergeable` < diff size < priority tiebreak) — order is part of the contract, so it is asserted explicitly rather than inferred from presence. Record this in the results file.

**§7's board-cell test is buildable and is built** (Task 3), because the board cell *is* code.
