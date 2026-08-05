<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0202 — Clear the unfixed review findings from change 0113](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-05-0202-clear-the-unfixed-review-findings-from-change-0113.md)**
<!-- docket:backlink:end -->

# Clear the unfixed review findings from change 0113 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the five non-blocking review findings left unfixed when change 0113 (the `aborted-run` health check) merged — one real false-positive bug in `branch_only_artifact`, three test-coverage holes, and one already-satisfied claim to verify.

**Architecture:** Five independent fixes, nothing sequenced. One shipped-script change (`scripts/board-checks.sh`: NUL-delimited `ls-tree` so C-quoted paths stop reading as branch-only), and four test-side changes that pin predicates the current suite leaves free to regress silently. Every new predicate is mutation-tested, and every mutation is asserted *landed* with a `grep -c` before/after transition per the file's existing house rule.

**Tech Stack:** Bash, git plumbing (`ls-tree`, `cat-file`), the repo's hand-rolled `assert` test harness in `tests/`.

## Global Constraints

- **Shipped-script bash floor is 4.0.** `mapfile -d` (bash 4.4) is FORBIDDEN in anything under `scripts/`. Use `while IFS= read -r -d ''`. (Test-side `mapfile -d` exists in `tests/test_grep_portability.sh:102`; that is a pre-existing inconsistency captured as change 0213 — it does NOT license new usage.)
- **`grep` on PATH is ugrep 7.5.0, not BSD grep.** Every grep pattern added by this change must also be run under `/usr/bin/grep` before the task is called done. ugrep accepts constructs BSD grep rejects, so a PATH-only check proves nothing about portability.
- **Mutation house rule** (`tests/test_board_checks.sh`): every mutation runs against a FRESH pristine copy via `armreseed` (never a cumulative chain) and must be CONFIRMED LANDED with a `grep -c` before/after assert before its outcome is believed.
- **One mutation breaks exactly one predicate.** Do not widen an existing mutation to cover a second read; add a separate lettered mutation.
- **Anchored reads only.** All four optional fields (`plan`, `results`, `branch`, `claimed_at`) in the `aborted-run` block are read with `fm_field`, never `field` (ADR-0057). Do not "simplify" one to `field`.
- **`--results-dir` is repo-relative**, addressed as `<ref>:<path>` and via `ls-tree --full-tree`. It is not a filesystem path.
- Existing constants to reuse verbatim: `AR_PLAN_NEW="docs/superpowers/plans/2026-08-03-aborted.md"`, `AR_RESULTS_NEW="docs/results/2026-08-03-aborted-results.md"`, `AR_STALE_CLAIM` (13h), `AR_FRESH_CLAIM` (11h), helpers `new_repo`, `ar_branch`, `iso`, `has_finding`, `git_quiet`, `armreseed`, `armrun`.
- The ARM fixture repo (`$ARM`) already carries two branches: `feat/arm-plan` (holds `$AR_PLAN_NEW`, absent from `main`) and `feat/arm-results` (holds `$AR_RESULTS_NEW`, absent from `main`). Existing ARM fixture ids are 220-223; **224, 225, 226 are free** (re-verified at reconcile).

---

## File Structure

| File | Responsibility | Tasks |
|---|---|---|
| `scripts/board-checks.sh` | `branch_only_artifact` NUL-delimited rewrite + corrected rationale comment | 1 |
| `tests/test_board_checks.sh` | Non-ASCII sanity + inherited fixtures, mutation F, ARM fixtures 224/225/226, mutation D2, mutation A + E assert claims | 1, 2, 4 |
| `tests/test_docket_status.sh` | Caller-side `--results-dir` resolved-value pin | 3 |
| `tests/test_skill_size_budgets.sh` | **Read-only verification. No edit.** | 5 |

---

## Task 1: `branch_only_artifact` — NUL-delimited listing (finding 4)

The load-bearing bug. `git ls-tree -r --name-only` C-quotes any path containing a quote, a backslash, a control character, or — under the default `core.quotePath=true` — any non-ASCII byte. `git_has` then runs `cat-file -e 'main:"docs/…caf\303\251.md"'`, which fails, and the function reports an **inherited** artifact as branch-only. That is a false positive in a check whose entire value is credibility.

**Files:**
- Modify: `scripts/board-checks.sh:96-112` (the comment block and `branch_only_artifact`)
- Test: `tests/test_board_checks.sh` (two new fixtures near the leg-A section; one new mutation F in the mutation section)

**Interfaces:**
- Consumes: `git_has REF PATH`, `$GIT`, `$CHANGES_DIR`, `$INTEGRATION_BRANCH` — all already in scope.
- Produces: `branch_only_artifact REF DIR` — unchanged contract. Prints the first path under DIR present on REF but not on `$INTEGRATION_BRANCH` and returns 0; returns 1 with empty stdout otherwise. Callers at `scripts/board-checks.sh:390` and `:393` are unchanged.
- Produces (test-side): `armrun_at DIR` — a runner used by Task 1's mutation F, consumed by nothing else.

- [ ] **Step 1: Add the two fixtures — sanity, then inherited**

Insert immediately **after** the `ar8_custom` assert block (which ends the leg-A `--results-dir` fixtures, around `tests/test_board_checks.sh:1090`) and **before** the `# ---------------- aborted-run, leg B` banner.

Only the second fixture discriminates the mutation. The first exists to prove the NUL plumbing reads a real path at all — a branch-only path fails `git_has` whether or not it was C-quoted, so the sanity fixture is green in both directions by construction. Both are needed: without the sanity fixture, an inherited-fixture-only test passes vacuously if the loop never runs.

```bash
# ---------------- branch_only_artifact: C-quoted paths (change 0202, finding 4) ----------------
# `git ls-tree --name-only` C-quotes any path with a quote, a backslash, a control character, or
# (under the default core.quotePath=true) a non-ASCII byte. git_has would then look up the literal
# quoted string, fail, and report an INHERITED artifact as branch-only — a false positive. The fix
# reads the listing NUL-delimited (-z), which suppresses quoting entirely.
# core.quotePath is set explicitly per-repo: it is git's default, but a developer's global config
# may turn it off, which would make the mutation below silently unreproducible.
AR_PLAN_UTF8="docs/superpowers/plans/2026-06-01-café-plan.md"

# ARQ1 — SANITY: the non-ASCII plan is branch-only. Leg A fires. Proves the NUL plumbing reads a
# real path; does NOT discriminate the mutation (a branch-only path fails git_has either way).
read -r ARQ1 _ < <(new_repo)
git -C "$ARQ1" config core.quotePath true
ar_branch "$ARQ1" feat/arq1 "$AR_PLAN_UTF8"
cat > "$ARQ1/docs/changes/active/0230-utf8-branchonly.md" <<EOF
---
id: 230
slug: utf8-branchonly
title: Non-ASCII plan committed on the branch only
status: in-progress
priority: medium
depends_on: []
branch: feat/arq1
plan:
results:
claimed_at: $AR_FRESH_CLAIM
---
EOF
arq1out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$ARQ1/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "0202: leg A fires for a branch-only plan with a non-ASCII path (id 230, NUL plumbing reads it)" \
  'has_finding "$arq1out" aborted-run 230'
# The reported path must be the REAL path, not a C-quoted rendering of it.
arq1line="$(grep -E "$(printf "^aborted-run\t230\t")" <<<"$arq1out")"
assert "0202: the leg-A finding reports the unquoted non-ASCII path (id 230)" \
  'grep -qF "$AR_PLAN_UTF8" <<<"$arq1line"'

# ARQ2 — INHERITED (the discriminating fixture): the non-ASCII plan is on main, so the branch
# INHERITS it and it is NOT branch-only. Fixed script: SILENT. Mutated: FIRES (the false positive).
# The "only" is load-bearing — branch_only_artifact returns the FIRST non-inherited path it finds,
# so any stray branch-only plan in this repo would mask the assert.
read -r ARQ2 _ < <(new_repo)
git -C "$ARQ2" config core.quotePath true
git -C "$ARQ2" checkout main >/dev/null 2>&1
mkdir -p "$ARQ2/$(dirname "$AR_PLAN_UTF8")"
printf '# artifact\n' > "$ARQ2/$AR_PLAN_UTF8"
git -C "$ARQ2" add "$AR_PLAN_UTF8"; git_quiet -C "$ARQ2" commit -m "non-ASCII plan on main"
git -C "$ARQ2" branch feat/arq2 main
git -C "$ARQ2" checkout docket >/dev/null 2>&1
cat > "$ARQ2/docs/changes/active/0231-utf8-inherited.md" <<EOF
---
id: 231
slug: utf8-inherited
title: Non-ASCII plan inherited from the integration branch
status: in-progress
priority: medium
depends_on: []
branch: feat/arq2
plan:
results:
claimed_at: $AR_FRESH_CLAIM
---
EOF
arq2out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$ARQ2/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "0202: leg A SILENT for an INHERITED non-ASCII plan (id 231, no C-quoting false positive)" \
  '! has_finding "$arq2out" aborted-run 231'
# Non-vacuity: the fixture must actually have a plan file on its branch, or the silence above
# would be the trivial empty-listing silence rather than the inherited-path silence.
assert "0202: fixture 231's branch really does carry the non-ASCII plan (assert is not vacuous)" \
  'git -C "$ARQ2" cat-file -e "feat/arq2:$AR_PLAN_UTF8"'
assert "0202: fixture 231's non-ASCII plan is also on main (that is what makes it inherited)" \
  'git -C "$ARQ2" cat-file -e "main:$AR_PLAN_UTF8"'
```

- [ ] **Step 2: Run the two fixtures against the UNFIXED script — 231 must be RED**

```bash
cd /Users/homer/dev/docket/.worktrees/clear-the-unfixed-review-findings-from-change-0113
bash tests/test_board_checks.sh 2>&1 | grep -E "0202|FAIL" | head -20
```

Expected: the id-230 asserts PASS (branch-only fails `git_has` either way), and
**`0202: leg A SILENT for an INHERITED non-ASCII plan (id 231…)` FAILS.** That failure IS the bug — it is the false positive the finding describes, reproduced.

If 231 unexpectedly PASSES, stop and diagnose before touching the script: the most likely cause is that `core.quotePath` did not take effect, meaning the fixture is not exercising the defect. Verify directly:

```bash
git -C "$ARQ2" -c core.quotePath=true ls-tree -r --name-only --full-tree feat/arq2 -- docs/superpowers/plans
```

Expected: the path comes back wrapped in double quotes with `\303\251` escapes. If it comes back bare, the fixture cannot discriminate and the mutation is not testable.

- [ ] **Step 3: Rewrite `branch_only_artifact` and its comment**

Replace `scripts/board-checks.sh:96-112` entirely — the comment block AND the function. The existing three-sentence comment argues for the capture-then-here-string shape; that shape **cannot be kept** (command substitution strips NUL bytes, so the captured string would lose every delimiter and read as one concatenated path), and its stated rationale is also wrong on its own terms, so it must be replaced rather than carried forward.

```bash
# branch_only_artifact REF DIR — print the first path under DIR that exists on REF but NOT on
# INTEGRATION_BRANCH, and exit 0; exit 1 with empty stdout when DIR is empty on REF or every path
# under it is already on the integration branch (inherited, i.e. already-merged work).
# --full-tree makes DIR worktree-root-relative regardless of the `-C "$CHANGES_DIR"` cwd, which is
# a subdirectory.
#
# -z is REQUIRED, not a style choice (change 0202). Plain `--name-only` C-quotes any path holding a
# quote, a backslash, a control character, or — under the default core.quotePath=true — any
# non-ASCII byte. git_has would then look up the literal quoted string, fail, and this function
# would report an INHERITED file as branch-only: a false positive in a check whose whole value is
# that it is believable. -z suppresses quoting and delimits with NUL.
#
# The NUL listing CANNOT be captured into a variable first: `$(…)` strips NUL bytes, so the
# delimiters would vanish and every path would concatenate into one string. Hence the
# process-substituted redirect. Do not "simplify" this back to a capture with a here-string.
#
# It is also not a pipeline, which matters for the early `return 0`. The hazard the previous
# comment called a race was really the SUBSHELL of `producer | while … done`: there, the in-loop
# `return 0` exits only the subshell, the function falls through to `return 1`, and the caller's
# `if` fails even though the path was printed. A redirect runs the loop in THIS shell, so the
# return is real. On that early return the process-substituted producer is orphaned and reaped with
# its remaining output discarded — harmless for a pure reader like ls-tree, and the reason never to
# swap in a producer with side effects.
#
# No empty-listing early-out is needed: an empty listing yields zero loop iterations and falls
# through to `return 1`.
branch_only_artifact(){
  local boa_ref="$1" boa_dir="$2" boa_p
  while IFS= read -r -d '' boa_p; do
    [ -n "$boa_p" ] || continue
    git_has "$INTEGRATION_BRANCH" "$boa_p" || { printf '%s' "$boa_p"; return 0; }
  done < <("$GIT" -C "$CHANGES_DIR" ls-tree -r -z --name-only --full-tree "$boa_ref" -- "$boa_dir" 2>/dev/null)
  return 1
}
```

- [ ] **Step 4: Re-run the fixtures — 231 must now be GREEN**

```bash
bash tests/test_board_checks.sh 2>&1 | tail -5
```

Expected: PASS, including all four new 0202 asserts. Every pre-existing `aborted-run` assert must still pass — this function backs both leg-A arms.

- [ ] **Step 5: Add `armrun_at`, then mutation F**

`armrun` is hardcoded to `$ARM`; mutation F must run against `$ARQ2`. Generalize the runner in place at `tests/test_board_checks.sh:1311` — a one-line refactor that leaves `armrun`'s call sites untouched:

```bash
armrun_at(){ NOW=$NOW_EPOCH bash "$ARMSCRIPT" --changes-dir "$1/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null; }
armrun(){ armrun_at "$ARM"; }
```

Then append mutation F **after** mutation E's `rm -rf "$armcopy"` line, at the end of the mutation section:

```bash
# Mutation F — restore the C-quoting bug in branch_only_artifact (change 0202). BOTH halves must
# revert together. Reverting -z ALONE is not a usable mutation: `read -d ''` would hit EOF on
# newline-delimited input, the loop body would never run, the function would return 1 for every
# input, and both fixtures would go green for entirely the wrong reason. So the read form reverts
# with it. The here-string capture is NOT restored and does not need to be — the C-quoting is
# produced by ls-tree, not by how the output is consumed, so these two edits reproduce the defect
# exactly. Runs against ARQ2 (inherited non-ASCII plan), the only fixture that discriminates.
armreseed
armF_z_before="$(grep -cF 'ls-tree -r -z --name-only' "$ARMSCRIPT")"
armF_d_before="$(grep -cF "read -r -d ''" "$ARMSCRIPT")"
sed -e 's/ls-tree -r -z --name-only/ls-tree -r --name-only/' \
    -e "s/while IFS= read -r -d '' boa_p; do/while IFS= read -r boa_p; do/" \
    "$ARMSCRIPT" > "$ARMSCRIPT.t"; mv "$ARMSCRIPT.t" "$ARMSCRIPT"
armF_z_after="$(grep -cF 'ls-tree -r -z --name-only' "$ARMSCRIPT")"
armF_d_after="$(grep -cF "read -r -d ''" "$ARMSCRIPT")"
assert "mutation F landed: -z is gone from the ls-tree listing (count 1 -> 0)" \
  '[ "$armF_z_before" = 1 ] && [ "$armF_z_after" = 0 ]'
assert "mutation F landed: the NUL read form is gone (count 1 -> 0)" \
  '[ "$armF_d_before" = 1 ] && [ "$armF_d_after" = 0 ]'
assert "mutation F landed: the mutated copy is still valid bash" 'bash -n "$ARMSCRIPT"'
armFout="$(armrun_at "$ARQ2")"
assert "mutation F (restore C-quoting): the INHERITED non-ASCII fixture 231 MISFIRES — the false positive" \
  'has_finding "$armFout" aborted-run 231'
armFsan="$(armrun_at "$ARQ1")"
assert "mutation F: the branch-only fixture 230 still fires (the arm itself survives the mutation)" \
  'has_finding "$armFsan" aborted-run 230'
rm -rf "$armcopy"
```

- [ ] **Step 6: Verify the mutation actually discriminates**

The mutation asserts are only trustworthy if they have been seen to depend on the fix. Confirm the `grep -c` transitions are real (not `0 -> 0`, which would make both landed-asserts vacuous) and that the suite is green:

```bash
bash tests/test_board_checks.sh 2>&1 | tail -5
grep -cF 'ls-tree -r -z --name-only' scripts/board-checks.sh   # expect exactly 1
grep -cF "read -r -d ''" scripts/board-checks.sh                # expect exactly 1
```

Then re-check every grep pattern this task added under BSD grep, since PATH `grep` is ugrep:

```bash
/usr/bin/grep -cF 'ls-tree -r -z --name-only' scripts/board-checks.sh
/usr/bin/grep -cF "read -r -d ''" scripts/board-checks.sh
```

Expected: `1` from each. Both must agree with the ugrep results above.

- [ ] **Step 7: Commit**

```bash
git add scripts/board-checks.sh tests/test_board_checks.sh
git commit -m "fix(0202): read ls-tree NUL-delimited so C-quoted paths stop reading as branch-only"
```

---

## Task 2: Pin the `results` anchored read (finding 2)

`scripts/board-checks.sh:393` reads `fm_field "$f" results`, but mutation D unanchors only the `plan` read and the only body-prose fixtures (205, 223) carry a body `plan:` line. Swapping `fm_field "$f" results` to `field` therefore leaves the suite fully green while reintroducing exactly the silent false negative ADR-0057 and 0113's own anchored-read rule exist to prevent.

**Files:**
- Modify: `tests/test_board_checks.sh` (ARM fixture 224; new mutation D2)

**Interfaces:**
- Consumes: `$ARM`, `feat/arm-results` (carries `$AR_RESULTS_NEW`, absent from `main`), `$AR_FRESH_CLAIM`, `armreseed`, `armrun` — all existing.
- Produces: nothing consumed by later tasks.

- [ ] **Step 1: Add ARM fixture 224**

Insert after fixture 223's heredoc and before the `armcopy=""` line.

224 is the exact mirror of 223: 223 pins the `plan` read, 224 pins the `results` read. `plan:` is SET so the plan arm stays out of the way; `results:` is absent from frontmatter but present in body prose, so an anchored read sees empty (fires) and an unanchored read sees the prose (silent).

```bash
# 224: results absent from FRONTMATTER, present in BODY prose, unrecorded results on the branch,
# fresh claim -> leg A's RESULTS arm fires under the ANCHORED read and goes silent under an
# unanchored one. The exact mirror of 223, which pins the same property for the plan arm.
# plan: is SET so the plan arm cannot contribute the finding and mask what this fixture measures.
cat > "$ARM/docs/changes/active/0224-manchor-results.md" <<EOF
---
id: 224
slug: manchor-results
title: Body prose mentions results
status: in-progress
priority: medium
depends_on: []
branch: feat/arm-results
plan: docs/superpowers/plans/2026-06-01-present.md
claimed_at: $AR_FRESH_CLAIM
---

## Notes
results: docs/results/2026-06-01-present-results.md
EOF
```

- [ ] **Step 2: Add the 224 baseline assert**

Append to the existing baseline block, after the fixture-223 baseline assert:

```bash
assert "mutation baseline: unmutated copy fires leg A on 224 (anchored results read)" \
  'has_finding "$arm0out" aborted-run 224'
```

- [ ] **Step 3: Run — the baseline assert must pass**

```bash
bash tests/test_board_checks.sh 2>&1 | grep -E "224|FAIL" | head
```

Expected: the 224 baseline assert PASSES. If it fails, the fixture is not producing leg A at all and mutation D2 below would be vacuous — diagnose before continuing. Most likely cause: `feat/arm-results` does not carry `$AR_RESULTS_NEW`, or `plan:` was left unset so the plan arm fired instead of the results arm.

- [ ] **Step 4: Add mutation D2**

A separate mutation from D, not a widening of it — one mutation per predicate is this file's existing shape, and a widened D would leave one green/red signal covering two independent reads. Insert immediately after mutation D's asserts, before mutation E's banner:

```bash
# Mutation D2 — unanchor the RESULTS read (fm_field -> field), the mirror of D. The body-prose
# fixture 224 goes GREEN (proving the anchoring is what makes it fire), while 221 — which has no
# body results: line — still fires, proving the arm itself survived the mutation.
armreseed
armD2_before="$(grep -cF 'fm_field "$f" results' "$ARMSCRIPT")"
awk '{ sub(/fm_field "\$f" results/, "field \"$f\" results"); print }' "$ARMSCRIPT" > "$ARMSCRIPT.t"
mv "$ARMSCRIPT.t" "$ARMSCRIPT"
armD2_after="$(grep -cF 'fm_field "$f" results' "$ARMSCRIPT")"
armD2out="$(armrun)"
assert "mutation D2 landed: the results read is unanchored (fm_field count 1 -> 0)" \
  '[ "$armD2_before" = 1 ] && [ "$armD2_after" = 0 ]'
assert "mutation D2 landed: the mutated copy is still valid bash" 'bash -n "$ARMSCRIPT"'
assert "mutation D2 (unanchor the results read): the body-prose fixture 224 goes GREEN — proves the anchoring" \
  '! has_finding "$armD2out" aborted-run 224'
assert "mutation D2: fixture 221, which has no body results: line, still fires" \
  'has_finding "$armD2out" aborted-run 221'
```

- [ ] **Step 5: Run and verify the mutation discriminates**

```bash
bash tests/test_board_checks.sh 2>&1 | tail -5
/usr/bin/grep -cF 'fm_field "$f" results' scripts/board-checks.sh
```

Expected: suite PASSES; the `/usr/bin/grep` count is `1`, matching what the mutation's before-count asserts. A `0` here would make the landed-assert vacuous and must be fixed before proceeding.

- [ ] **Step 6: Commit**

```bash
git add tests/test_board_checks.sh
git commit -m "test(0202): pin the results anchored read with fixture 224 and mutation D2"
```

---

## Task 3: Pin the `--results-dir` caller wiring (finding 1)

`scripts/docket-status.sh:734` passes `--results-dir "${RESULTS_DIR:-docs/results}"`, and `tests/test_docket_status.sh` contains zero occurrences of `results-dir`. Deleting that argument reddens nothing. The callee's own `RESULTS_DIR_REL="${RESULTS_DIR_REL:-docs/results}"` fallback (`scripts/board-checks.sh:54`) makes the regression silent: a repo configuring a non-default `results_dir` would scan a nonexistent directory forever, green.

**Files:**
- Modify: `tests/test_docket_status.sh` (the `health_checks` mock block, after the existing `TERMINAL_PUBLISH unset` invocation around line 1256)

**Interfaces:**
- Consumes: `$health_dir`, `$tmp/mock-health/board-checks.sh` (logs `board-checks $*` to `$HEALTH_LOG`), `$SCRIPT` — all existing in that block.
- Produces: nothing consumed by later tasks.

- [ ] **Step 1: Write the failing assert**

The pin must use a **non-default** value. An assert on `--results-dir docs/results` would pass identically if the caller hardcoded the default, or if the callee's own fallback were the only thing supplying it — which is precisely the silent-regression path the finding describes. Add after the `TERMINAL_PUBLISH unset` assert:

```bash
# --- change 0202 (finding 1): the --results-dir caller wiring ---------------------------------
# board-checks.sh defaults RESULTS_DIR_REL to docs/results on its own, so asserting the DEFAULT
# string would stay green even if docket-status stopped passing the flag entirely. Pin the
# RESOLVED value with a non-default RESULTS_DIR: only the caller can be supplying that.
health_log_rd="$tmp/health-calls-resultsdir.log"; : > "$health_log_rd"
health_out_rd="$( cd "$health_dir" && \
  DOCKET_MODE=docket CHANGES_DIR=docs/changes ADRS_DIR=docs/adrs \
  INTEGRATION_BRANCH=main METADATA_BRANCH=docket RESULTS_DIR=docs/custom-results \
  SCRIPTS_DIR="$tmp/mock-health" HEALTH_LOG="$health_log_rd" \
  bash -c '. "'"$SCRIPT"'"; health_checks' )"
assert "0202: health_checks passes the RESOLVED --results-dir, not the callee's fallback" \
  'grep -q -- "--results-dir docs/custom-results" "$health_log_rd"'
```

- [ ] **Step 2: Run it — it must PASS immediately, then prove it can fail**

```bash
cd /Users/homer/dev/docket/.worktrees/clear-the-unfixed-review-findings-from-change-0113
bash tests/test_docket_status.sh 2>&1 | tail -5
```

Expected: PASS. The production code is already correct here — the finding is a *missing assert*, not a bug, so there is no red-first step against the real script.

That makes the mutation check mandatory rather than optional: an assert that has never been seen red is untested code. Prove it detects the deletion it exists to catch:

```bash
cp scripts/docket-status.sh /tmp/ds-orig.sh
grep -n 'results-dir' scripts/docket-status.sh
# delete the --results-dir argument line (734) from the health_checks invocation
sed -i '' '/--results-dir "\${RESULTS_DIR:-docs\/results}" \\/d' scripts/docket-status.sh
grep -c 'results-dir' scripts/docket-status.sh   # expect 0
bash tests/test_docket_status.sh 2>&1 | grep -E "0202|FAIL" | head
```

Expected: the new 0202 assert FAILS. Then restore, unconditionally:

```bash
cp /tmp/ds-orig.sh scripts/docket-status.sh
git diff --stat scripts/docket-status.sh   # expect EMPTY — the restore must be byte-exact
bash tests/test_docket_status.sh 2>&1 | tail -3
```

Expected: `git diff --stat` prints nothing and the suite is green again. If the diff is non-empty, `git checkout -- scripts/docket-status.sh` and re-run.

- [ ] **Step 3: Check the new grep under BSD grep**

```bash
/usr/bin/grep -q -- "--results-dir docs/custom-results" /dev/null; echo "exit=$?"
```

Expected: `exit=1` (no match on an empty file, no *parse* error). A usage or parse error here means the pattern is not BSD-portable and must be rewritten. The leading `--` is what keeps `--results-dir` from being read as a grep option; it must not be dropped.

- [ ] **Step 4: Commit**

```bash
git add tests/test_docket_status.sh
git commit -m "test(0202): pin the resolved --results-dir caller wiring in health_checks"
```

---

## Task 4: Make the two unreachable mutation claims assertable (finding 3)

Two mutation comments state claims their fixtures cannot reach. Mutation A's "the healthy-field fixture 221 (plan: SET) starts misfiring. Both directions" is false — 221's branch is `feat/arm-results`, which carries no plan file, so the misfire conjunct is unreachable. Mutation E's "stale-in-progress must stay unaffected" is asserted nowhere, and no ARM fixture produces a `stale-in-progress` finding at all (222's claim is 13h against a 72h lease).

Fixtures over comment-deletion: 0202 exists to close exactly the "claimed-in-prose, asserted-nowhere" class, so deleting the claims would resolve the finding by lowering the bar the change is here to raise.

**Files:**
- Modify: `tests/test_board_checks.sh` (ARM fixtures 225, 226; a new claim constant; mutation A and E asserts and comments)

**Interfaces:**
- Consumes: `$ARM`, `feat/arm-plan` (carries `$AR_PLAN_NEW`, absent from `main`), `$AR_FRESH_CLAIM`, `$NOW_EPOCH`, `iso`, `armreseed`, `armrun`.
- Produces: `AR_LEASE_STALE_CLAIM` — a ~100h claim string, used only by fixture 226.

- [ ] **Step 1: Add the long-lease claim constant**

`AR_STALE_CLAIM` is 13h — past `aborted-run`'s 12h window but far inside `stale-in-progress`'s 72h lease. Fixture 226 needs a claim past BOTH. Add beside the existing two at `tests/test_board_checks.sh:1098-1099`:

```bash
AR_LEASE_STALE_CLAIM="$(iso $(( NOW_EPOCH - 100*3600 )))"  # 100h > 72h lease AND > 12h => BOTH checks fire
```

- [ ] **Step 2: Add ARM fixtures 225 and 226**

Insert after fixture 224's heredoc (Task 2), before `armcopy=""`.

```bash
# 225: healthy fields, but the branch DOES carry a branch-only plan. Baseline: SILENT (plan: is
# set, so leg A's -z guard correctly declines). Under mutation A (-z -> -n) it MISFIRES. This is
# the fixture mutation A's "both directions" claim needs — 221 cannot serve, because its branch
# (feat/arm-results) carries no plan file, leaving the misfire conjunct unreachable.
cat > "$ARM/docs/changes/active/0225-mhealthy.md" <<EOF
---
id: 225
slug: mhealthy
title: Recorded plan and results, plan file present on the branch
status: in-progress
priority: medium
depends_on: []
branch: feat/arm-plan
plan: docs/superpowers/plans/2026-06-01-present.md
results: docs/results/2026-06-01-present-results.md
claimed_at: $AR_FRESH_CLAIM
---
EOF
# 226: no branch, claim 100h old — past BOTH the 12h aborted-run window and the 72h stale-in-progress
# lease, so BOTH checks fire at baseline. That is what makes mutation E's "stale-in-progress must
# stay unaffected" claim assertable: dropping the aborted-run block must remove exactly one of them.
cat > "$ARM/docs/changes/active/0226-mboth-checks.md" <<EOF
---
id: 226
slug: mboth-checks
title: Claim past both the run window and the lease
status: in-progress
priority: medium
depends_on: []
branch:
plan:
results:
claimed_at: $AR_LEASE_STALE_CLAIM
---
EOF
```

- [ ] **Step 3: Add the baseline asserts**

Append to the baseline block:

```bash
assert "mutation baseline: unmutated copy is SILENT on 225 (healthy fields, plan file on the branch)" \
  '! has_finding "$arm0out" aborted-run 225'
assert "mutation baseline: unmutated copy fires leg B on 226 (100h claim)" \
  'has_finding "$arm0out" aborted-run 226'
assert "mutation baseline: unmutated copy ALSO fires stale-in-progress on 226 (both checks, one change)" \
  'has_finding "$arm0out" stale-in-progress 226'
```

- [ ] **Step 4: Run the baselines**

```bash
bash tests/test_board_checks.sh 2>&1 | grep -E "225|226|FAIL" | head
```

Expected: all three PASS. The `stale-in-progress 226` assert is the one to watch: `armrun` passes no `--lease-ttl-hours`, so the TTL defaults to 72h and 100h clears it. If it fails, check that 226 has NO `branch:` — the has-branch path emits a different message and requires a >3d-idle branch instead.

- [ ] **Step 5: Fix mutation A's claim and comment**

Replace mutation A's comment (`tests/test_board_checks.sh:1321-1322`) and add the misfire assert after its existing 220 assert:

```bash
# Mutation A — invert leg A's plan emptiness test (-z becomes -n): the unrecorded-plan fixture 220
# goes GREEN and the healthy-field fixture 225 starts misfiring. Both directions. 225, not 221:
# 221's branch (feat/arm-results) carries no plan file, so the misfire conjunct is unreachable
# there — the guard would prove only half of what its comment claims (change 0202, finding 3).
```

```bash
assert "mutation A (invert plan emptiness): the healthy fixture 225 MISFIRES — the other direction" \
  'has_finding "$armAout" aborted-run 225'
```

- [ ] **Step 6: Fix mutation E's claim and comment**

Replace mutation E's comment (`tests/test_board_checks.sh:1384-1385`) and add the survival assert after its existing 223 assert:

```bash
# Mutation E — drop the whole aborted-run block: every red fixture goes GREEN, and stale-in-progress
# must stay unaffected (the two checks are genuinely separate code). Fixture 226 is what makes the
# second half assertable: its 100h claim fires BOTH checks at baseline, so dropping this block must
# remove the aborted-run finding and leave the stale-in-progress one standing (change 0202).
```

```bash
assert "mutation E (drop whole block): fixture 226's aborted-run finding goes GREEN" \
  '! has_finding "$armEout" aborted-run 226'
assert "mutation E: stale-in-progress on 226 SURVIVES — the two checks are separate code" \
  'has_finding "$armEout" stale-in-progress 226'
```

- [ ] **Step 7: Run — every mutation assert must pass**

```bash
bash tests/test_board_checks.sh 2>&1 | tail -5
```

Expected: the whole file PASSES. If `mutation E: stale-in-progress on 226 SURVIVES` fails, mutation E's awk range has swallowed more than the `aborted-run` block — check that `stale-in-progress`'s block still exists in the mutated copy:

```bash
grep -c 'stale-in-progress' "$ARMSCRIPT"   # must be > 0 after mutation E
```

- [ ] **Step 8: Commit**

```bash
git add tests/test_board_checks.sh
git commit -m "test(0202): make mutation A and E's prose claims assertable with fixtures 225 and 226"
```

---

## Task 5: Verify finding 5, make no edit

The finding reported that the 0113 budget-rationale comment in `tests/test_skill_size_budgets.sh` omits the measured actual and margin. **It no longer does** — a later merge (0201's slim, re-measured post-rebase) already restored the figures. This task is a verification with a recorded outcome, not an edit. Treating it as a live edit would add a duplicate rationale paragraph; treating it as silently closed would lose the check.

**Files:**
- Read-only: `tests/test_skill_size_budgets.sh:230-241`

**Interfaces:** none.

- [ ] **Step 1: Read the rationale comment and confirm both figures are present**

```bash
cd /Users/homer/dev/docket/.worktrees/clear-the-unfixed-review-findings-from-change-0113
sed -n '228,242p' tests/test_skill_size_budgets.sh
```

Expected: the comment records the word measurement `3728 words -> … 3800 (72 words of margin)` and the line measurement `139 actual, 145 budget`.

- [ ] **Step 2: Confirm the stale pre-rebase figures are absent**

The original finding quoted `4013 -> 4050` and `147 for 143`. Those predate 0201's slim and must NOT be restored.

```bash
grep -nE '4013|4050|147 for 143' tests/test_skill_size_budgets.sh
/usr/bin/grep -nE '4013|4050|147 for 143' tests/test_skill_size_budgets.sh
```

Expected: no matches from either (exit 1). If any match appears, the file has regressed to pre-rebase figures — stop and report; do not silently rewrite it.

- [ ] **Step 3: Record the verification — make NO source edit**

Make no change to `tests/test_skill_size_budgets.sh`. Record the outcome (both figures present, stale figures absent, no edit made) for the results file and PR body. Confirm nothing was touched:

```bash
git status --porcelain tests/test_skill_size_budgets.sh
```

Expected: empty output.

---

## Final verification

- [ ] **Run the two directly-affected suites**

```bash
cd /Users/homer/dev/docket/.worktrees/clear-the-unfixed-review-findings-from-change-0113
bash tests/test_board_checks.sh 2>&1 | tail -5
bash tests/test_docket_status.sh 2>&1 | tail -5
```

Expected: both green.

- [ ] **Run the full suite** — the single gate for this change.

- [ ] **Confirm the change touched only its four intended files**

```bash
git diff --stat origin/main...HEAD
```

Expected: `scripts/board-checks.sh`, `tests/test_board_checks.sh`, `tests/test_docket_status.sh`, plus this plan file. `tests/test_skill_size_budgets.sh` must NOT appear (Task 5 is verify-only), and `scripts/docket-status.sh` must NOT appear (Task 3 Step 2's mutation was restored).
