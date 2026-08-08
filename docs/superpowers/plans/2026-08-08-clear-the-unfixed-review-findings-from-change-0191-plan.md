<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0200 — Board-checks hardening — sanitize LF escape, capture-shape mutation, minor-finding clearance](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0200-clear-the-unfixed-review-findings-from-change-0191.md)**
<!-- docket:backlink:end -->

# Board-checks hardening — sanitize LF escape, capture-shape mutation, minor-finding clearance — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close change 0191's and 0202's surviving minor findings, fix `sanitize`'s missed raw-LF case, and make the "do not capture the `-z` listing" constraint executable instead of decorative.

**Architecture:** Five independent tasks against two implementation files (`scripts/board-checks.sh`, `skills/docket-convention/SKILL.md`) and three guard/doc files (`tests/test_board_checks.sh`, `tests/test_skill_size_budgets.sh`, `scripts/board-checks.md`). Task 1 is a real behavior fix driven by a new fixture; Task 2 converts a comment into a mutation arm; Tasks 3–4 are structural cleanups where the *test* edit is the risky half; Task 5 records a docs-lifecycle rule and pays its size-budget toll.

**Tech Stack:** Bash 4.4+, `awk`/`sed` (BSD-compatible forms only), git plumbing, the repo's own `tests/test_*.sh` assert harness, `scripts/run-tests.sh`.

## Global Constraints

- **Shell floor is bash 4.4** (change 0222). BSD-compatible `sed`/`awk` only — **BSD `sed` does not interpret `\t` in a pattern**, so escape work uses bash parameter expansion, and `sed` bounded repetition beyond `{0,255}` is banned.
- **`grep` on PATH is ugrep locally.** Any new regex must also be valid for `/usr/bin/grep`. Prefer `grep -F` for fixed strings.
- **Guards are code** (`AGENTS.md`): every new assert must have been observed RED against the real defect before it is believed. Confirm each mutation actually landed with a `grep -c` before/after pair — a substitution that silently fails to match yields a green run with nothing mutated.
- **Assert the state you removed, not the one you added** (learnings: `assert-detects-removal-not-replacement`). Absence asserts need a live non-vacuity companion through the same extractor.
- **Pin the mechanism, not the outcome** (learnings: `assert-pins-outcome-not-mechanism`).
- **Test code in this plan is unverified code** (learnings: `plan-supplied-test-code-is-unverified`). Prove each new assert *can* pass and *can* fail.
- Every commit message is scoped `(0200)`.
- Full-suite command: `scripts/run-tests.sh`. Single file: `scripts/run-tests.sh tests/test_board_checks.sh`.
- Out of scope, do not touch: the `-z` read shape itself; mutation F; the `scalar-form` check's behavior; the bash floor; `docs/superpowers/plans/2026-08-05-clear-the-unfixed-review-findings-from-change-0113.md` (Task 5 exists precisely to rule it untouchable).

## File Structure

| File | Responsibility in this change |
|---|---|
| `scripts/board-checks.sh` | `sanitize` gains an LF escape (T1); dead `boa_p` guard removed (T3); `scalar_form_check` hoisted to top level with a named end marker (T4) |
| `scripts/board-checks.md` | the contract's "columns are not forgeable" invariant restated to cover records/LF (T1) |
| `tests/test_board_checks.sh` | new ARQ3 fixture 249 (T1); new mutation O (T2); baseline comment reword (T3); mutation 4 redesigned as a two-region delete plus a `bash -n` landed assert (T4) |
| `skills/docket-convention/SKILL.md` | the frozen-merged-plans rule (T5) |
| `tests/test_skill_size_budgets.sh` | the convention file's budget row raised, with the in-diff `references/`-considered argument (T5) |

---

### Task 1: `sanitize` escapes an interior LF

**Files:**
- Test: `tests/test_board_checks.sh` — new ARQ3 block inserted immediately after ARQ2's last assert (currently the `0202: fixture 231's non-ASCII plan is also on main …` assert, ~line 1388)
- Modify: `scripts/board-checks.sh:135-142` (the `sanitize` comment block and the function)
- Modify: `scripts/board-checks.md:470-473` (the "columns are not forgeable" invariant bullet)

**Interfaces:**
- Consumes: existing helpers `new_repo`, `ar_branch`, `has_finding`, `assert`, and the globals `NOW_EPOCH`, `SCRIPT`, `AR_FRESH_CLAIM` — all defined above the insertion point, which is why the block goes after ARQ2 and not earlier (the file runs under `set -u`).
- Produces: nothing later tasks consume. Fixture id **249** is claimed by this task (220–226 are the ARM mutation repo, 230–231 ARQ, 232–248 and 260–263 are taken; 249 is the next free id).

- [ ] **Step 1: Write the failing test**

Insert this block in `tests/test_board_checks.sh` directly after ARQ2's final assert:

```bash
# ARQ3 — a branch-only plan whose PATH embeds an interior newline (change 0200). Since change 0202
# leg A reads the listing NUL-delimited, so a git path arrives RAW and $ar_hit carries the LF all
# the way into emit. sanitize must escape it: otherwise one finding becomes TWO TSV records, and
# the caller (docket-status.sh's health_checks, `IFS=$'\t' read -r check_id change_id message`)
# reads the orphaned tail as a finding of its own — and the trailing `sort` reorders it anywhere.
AR_PLAN_LF="$(printf 'docs/superpowers/plans/2026-06-01-multi\nline-plan.md')"
AR_LF_ESCAPED='multi\nline-plan.md'
read -r ARQ3 _ < <(new_repo)
ar_branch "$ARQ3" feat/arq3 "$AR_PLAN_LF"
cat > "$ARQ3/docs/changes/active/0249-lf-path-branchonly.md" <<EOF
---
id: 249
slug: lf-path-branchonly
title: Plan path with an embedded newline committed on the branch only
status: in-progress
priority: medium
depends_on: []
branch: feat/arq3
plan:
results:
claimed_at: $AR_FRESH_CLAIM
---
EOF
arq3out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$ARQ3/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "0200: leg A fires for a branch-only plan whose path embeds a newline (id 249)" \
  'has_finding "$arq3out" aborted-run 249'
# Non-vacuity, through the same tree the check reads: without this, the discriminating assert below
# could be measuring "the fixture never had an LF-named plan" rather than "the LF was escaped".
assert "0200: fixture 249's branch really carries the LF-named plan (assert is not vacuous)" \
  'git -C "$ARQ3" cat-file -e "feat/arq3:$AR_PLAN_LF"'
# THE DISCRIMINATOR — and note what it deliberately is NOT. A "exactly one line matches the
# aborted-run<TAB>249<TAB> prefix" count passes in BOTH directions: unfixed, that prefix line still
# exists, it just ends at "…multi" with "line-plan.md) but plan: is unset …" orphaned on the next
# record. What only the fixed script can produce is the two-character escape \n WITH the post-LF
# tail on the SAME line.
arq3line="$(grep -E "$(printf "^aborted-run\t249\t")" <<<"$arq3out")"
assert "0200: the leg-A finding stays ONE TSV record — the interior LF is escaped to a visible \\n" \
  'grep -qF "$AR_LF_ESCAPED" <<<"$arq3line"'
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `scripts/run-tests.sh tests/test_board_checks.sh`

Expected: FAIL. Exactly one new `NOT OK` line — `0200: the leg-A finding stays ONE TSV record …`. The two asserts above it must be `ok` already; if either of them fails, the fixture is wrong, not the script — fix the fixture before touching `sanitize`.

- [ ] **Step 3: Rewrite the `sanitize` comment and add the LF substitution**

Replace `scripts/board-checks.sh` lines 135–142 (from `# sanitize VALUE —` through the `sanitize(){ … }` line) with:

```bash
# sanitize VALUE — render TAB, CR, and LF as the visible two-character escapes \t, \r and \n
# (change 0104; the LF leg added by change 0200).
#
# Findings are TAB-separated and the caller splits them with `IFS=$'\t' read -r check_id change_id
# message` (docket-status.sh's `health_checks`). An interior TAB in ANY embedded value shifts every
# later field; an interior LF is worse — it splits one finding into TWO records, and the caller
# reads the orphaned tail as a finding in its own right, with the trailing `sort` free to move it
# anywhere in the output.
#
# Do NOT re-justify the escape set by where the values come from. That premise is what went stale:
# it once read "every embedded value arrives via field()/fm_field(), which truncate at the first
# newline". Since change 0202, leg A's $ar_hit is a GIT PATH read NUL-delimited
# (`ls-tree -r -z`) — and a git path may hold any byte but NUL, newline included — so it reaches
# emit raw. The escape therefore lives HERE, wrapping both embedded columns of every emit, rather
# than at that one call site: every current and future caller is covered without an audit.
#
# The LF coverage is deliberately partial and record-shaped, not a completeness guarantee: leg A's
# call sites capture branch_only_artifact through $(…), which strips a TRAILING newline before
# sanitize is ever reached, so only INTERIOR newlines are ever seen here. That is the whole job —
# keep one finding on one record.
#
# Pure bash parameter expansion: BSD sed does not interpret \t in a pattern, so a sed form would be
# silently wrong.
sanitize(){ local v="$1"; v="${v//$'\t'/\\t}"; v="${v//$'\r'/\\r}"; v="${v//$'\n'/\\n}"; printf '%s' "$v"; }
```

- [ ] **Step 4: Update the script contract's invariant**

In `scripts/board-checks.md`, replace the bullet beginning `- **The findings channel's COLUMNS are not forgeable.**` (lines 470–473) with:

```markdown
- **The findings channel's COLUMNS and RECORDS are not forgeable.** `emit` escapes TAB, CR and LF
  to the visible `\t` / `\r` / `\n` in both embedded columns, and the change-id column never
  carries a raw frontmatter value. The caller splits findings with `IFS=$'\t' read -r check_id
  change_id message`, so an un-escaped TAB in an untrusted value would shift every later field and
  an un-escaped LF would split one finding into two records. The LF case is not hypothetical: since
  change 0202 the `aborted-run` legs embed a git path read NUL-delimited, which may legally contain
  a newline (change 0200).
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `scripts/run-tests.sh tests/test_board_checks.sh`
Expected: PASS, with all three new `ok -` lines present.

- [ ] **Step 6: Prove the new assert can go red for the right reason**

Run:

```bash
cp scripts/board-checks.sh /tmp/bc-t1.bak
sed -i.tmp "s|v=\"\${v//\$'\\\\n'/\\\\\\\\n}\"; ||" scripts/board-checks.sh
grep -c "n}\"" scripts/board-checks.sh   # confirm the LF substitution is actually gone
scripts/run-tests.sh tests/test_board_checks.sh   # expect FAIL on the ONE-TSV-record assert only
cp /tmp/bc-t1.bak scripts/board-checks.sh; rm -f scripts/board-checks.sh.tmp
scripts/run-tests.sh tests/test_board_checks.sh   # expect PASS again
```

If the hand-mutation does not cleanly remove only the LF substitution, do it with an editor instead — the point is the observation, not the sed. Do not proceed until the assert has been seen RED with the LF escape removed and GREEN with it restored.

- [ ] **Step 7: Commit**

```bash
git add scripts/board-checks.sh scripts/board-checks.md tests/test_board_checks.sh
git commit -m "fix(0200): escape an interior LF in sanitize so one finding stays one record"
```

---

### Task 2: Mutation O — the capture-shape constraint becomes executable

**Files:**
- Test: `tests/test_board_checks.sh` — new block inserted immediately after mutation F's `rm -rf "$armcopy"` and before the `# ---------------- leg C mutations (change 0211) ----------------` header

**Interfaces:**
- Consumes: `armreseed`, `ARMSCRIPT`, `armrun_at`, and the `ARQ1` repo — all defined earlier in the file. Fixture 230's baseline firing is already pinned by the `0202: leg A fires for a branch-only plan with a non-ASCII path (id 230 …)` assert, which is what stops this arm's GREEN assert from being vacuous.
- Produces: nothing later tasks consume.

- [ ] **Step 1: Write the mutation arm**

Insert after mutation F's `rm -rf "$armcopy"`:

```bash
# Mutation O — rewrite branch_only_artifact's consumption into the FORBIDDEN capture shape (change
# 0200). Letters A-N are taken; O is the next free one. This arm is the enforcement the comment
# above the function ("Do not 'simplify' this back to a capture with a here-string") never had.
#
# Everything that LOOKS like a correctness signal survives the rewrite, which is the entire trap:
# -z stays, `read -r -d ''` stays, and `bash -n` still passes. But `$(…)` strips NUL bytes, so the
# here-string carries one NUL-free blob, `read -d ''` hits EOF on the first iteration, the loop body
# never runs, and the function returns 1 for EVERY input. Leg A would go permanently, silently
# false-negative with a fully green suite.
#
# The GREEN assert is not vacuous: fixture 230's baseline firing is pinned in the ARQ1 block above.
# The two "still present" asserts matter as much as the landed ones — they prove this arm reproduces
# the CAPTURE defect specifically and has not accidentally degenerated into mutation F.
armreseed
armO_ps_before="$(grep -cF 'done < <("$GIT" -C "$CHANGES_DIR" ls-tree -r -z' "$ARMSCRIPT")"
awk '
  $0 ~ /^  while IFS= read -r -d .. boa_p; do$/ {
    print "  boa_list=\"$(\"$GIT\" -C \"$CHANGES_DIR\" ls-tree -r -z --name-only --full-tree \"$boa_ref\" -- \"$boa_dir\" 2>/dev/null)\"";
    print; next
  }
  $0 ~ /^  done < <\("\$GIT" -C "\$CHANGES_DIR" ls-tree -r -z/ { print "  done <<<\"$boa_list\""; next }
  { print }
' "$ARMSCRIPT" > "$ARMSCRIPT.t"; mv "$ARMSCRIPT.t" "$ARMSCRIPT"
armO_ps_after="$(grep -cF 'done < <("$GIT" -C "$CHANGES_DIR" ls-tree -r -z' "$ARMSCRIPT")"
armO_hs="$(grep -cF 'done <<<"$boa_list"' "$ARMSCRIPT")"
armO_cap="$(grep -cF 'boa_list="$("$GIT" -C "$CHANGES_DIR" ls-tree -r -z' "$ARMSCRIPT")"
armO_z="$(grep -cF 'ls-tree -r -z --name-only' "$ARMSCRIPT")"
armO_d="$(grep -cF "read -r -d ''" "$ARMSCRIPT")"
assert "mutation O landed: the process-substituted listing is gone (count 1 -> 0)" \
  '[ "$armO_ps_before" = 1 ] && [ "$armO_ps_after" = 0 ]'
assert "mutation O landed: the forbidden here-string consumption is in place" '[ "$armO_hs" = 1 ]'
assert "mutation O landed: the listing is captured into a variable first" '[ "$armO_cap" = 1 ]'
assert "mutation O landed: the mutated copy is still valid bash — the broken shape is SYNTACTICALLY FINE" \
  'bash -n "$ARMSCRIPT"'
assert "mutation O is the CAPTURE defect, not mutation F: -z survives the rewrite" \
  '[ "$armO_z" = 2 ]'
assert "mutation O is the CAPTURE defect, not mutation F: the NUL read form survives the rewrite" \
  '[ "$armO_d" = 1 ]'
armOout="$(armrun_at "$ARQ1")"
assert "mutation O (capture the -z listing): the branch-only fixture 230 goes GREEN — NULs stripped, the loop never runs" \
  '! has_finding "$armOout" aborted-run 230'
rm -rf "$armcopy"
```

- [ ] **Step 2: Run and check the landed asserts before trusting the GREEN one**

Run: `scripts/run-tests.sh tests/test_board_checks.sh --verbose 2>&1 | grep -i "mutation O"`

Expected: all seven `ok -` lines. If `mutation O landed: the listing is captured into a variable first` fails, the awk did not match and **nothing was mutated** — the GREEN assert below it would then be passing for no reason at all. Fix the awk before reading any other result.

Note on `armO_z = 2`: after the rewrite the `ls-tree -r -z --name-only` text appears on both the inserted capture line and nowhere else in the original — verify the count empirically with `grep -cF 'ls-tree -r -z --name-only' "$ARMSCRIPT"` on a hand-made mutant and set the literal to whatever the mutant actually holds. Do **not** weaken this to `-ge 1`; the assert exists to distinguish this arm from mutation F.

- [ ] **Step 3: Prove the arm fails when the constraint is honoured**

The arm must be capable of going red. Temporarily change the GREEN assert to its negation (`has_finding` instead of `! has_finding`), re-run, and confirm it now FAILS — i.e. the mutant really does suppress finding 230. Restore the negation.

- [ ] **Step 4: Full file green**

Run: `scripts/run-tests.sh tests/test_board_checks.sh`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add tests/test_board_checks.sh
git commit -m "test(0200): mutation O pins the no-capture constraint on the -z listing"
```

---

### Task 3: Remove the dead `boa_p` guard; drop the drifted baseline count

**Files:**
- Modify: `scripts/board-checks.sh:125` (delete) and the comment block above `branch_only_artifact` (lines 119–121)
- Modify: `tests/test_board_checks.sh:2305` (the mutation-baseline comment)

**Interfaces:**
- Consumes: nothing. Produces: nothing. No new test — fixtures 230/231 plus mutations F and O already exercise every path of `branch_only_artifact`, and this task removes an unreachable line rather than changing behavior.

- [ ] **Step 1: Delete the unreachable per-record guard**

In `scripts/board-checks.sh`, delete the line:

```bash
    [ -n "$boa_p" ] || continue
```

so the loop reads:

```bash
branch_only_artifact(){
  local boa_ref="$1" boa_dir="$2" boa_p
  while IFS= read -r -d '' boa_p; do
    git_has "$INTEGRATION_BRANCH" "$boa_p" || { printf '%s' "$boa_p"; return 0; }
  done < <("$GIT" -C "$CHANGES_DIR" ls-tree -r -z --name-only --full-tree "$boa_ref" -- "$boa_dir" 2>/dev/null)
  return 1
}
```

- [ ] **Step 2: Extend the comment to say why no emptiness guard exists at either level**

Replace the final paragraph of the comment block above the function (currently `# No empty-listing early-out is needed: an empty listing yields zero loop iterations and falls` / `# through to `return 1`.`) with:

```bash
# No emptiness guard is needed at EITHER level, and change 0200 removed the one that was here.
# Whole listing: an empty listing yields zero loop iterations and falls through to `return 1`.
# Per record: under -z, ls-tree never emits an empty record, and at EOF `read -d ''` returns
# nonzero with an empty accumulator, ending the loop before the body runs — so a
# `[ -n "$boa_p" ] || continue` inside the loop was unreachable. It is not re-added: by this repo's
# own rule an unenforced comment is decoration, and dead code invites "what does this protect?"
# archaeology.
```

- [ ] **Step 3: Reword the mutation-baseline comment**

In `tests/test_board_checks.sh`, replace:

```bash
# Baseline: the un-mutated copy fires exactly the three expected findings.
```

with:

```bash
# Baseline: the un-mutated copy fires the expected findings, pinned one by one below. Deliberately
# no count here (change 0200) — the per-fixture asserts beneath ARE the guard, and the number this
# line used to carry had already drifted past what follows it with nothing to redden.
```

- [ ] **Step 4: Verify nothing changed behaviorally**

Run: `scripts/run-tests.sh tests/test_board_checks.sh`
Expected: PASS — identical assert count to Task 2's run. Then confirm the removed line is genuinely unreachable rather than merely untested:

```bash
grep -n "boa_p" scripts/board-checks.sh   # only the local decl, the read, and the git_has call remain
bash -n scripts/board-checks.sh
```

- [ ] **Step 5: Commit**

```bash
git add scripts/board-checks.sh tests/test_board_checks.sh
git commit -m "refactor(0200): drop the unreachable boa_p guard and the drifted baseline count"
```

---

### Task 4: Hoist `scalar_form_check` to top level; redesign mutation 4

**Files:**
- Modify: `scripts/board-checks.sh` — move lines 320–394 (the `# --- scalar-form:` comment block through the function's closing `  }`) to top level, immediately after `renders_row`'s closing `}` (line 222); leave lines 395–398 (the two `sf_*` reads and two call sites) in the walk
- Modify: `tests/test_board_checks.sh` — mutation 4 (currently ~lines 1032–1050)

**Interfaces:**
- Consumes: the hoisted body keeps reading the walk's loop variable `$cid` by dynamic scope; bash resolves it at call time, so the hoist is behavior-neutral.
- Produces: a new named terminator line, `# --- end scalar-form helper ---`, which mutation 4's first region delete bounds on. Nothing else consumes it.

**Why the mutation must be redesigned, not re-run:** mutation 4 today is an awk range-delete from `# --- scalar-form:` to `# --- broken-spec:`. After the hoist the start marker sits at top level and the end marker is still inside the walk, so that single range would also swallow the `FILES` `mapfile`, the `for f in …` opening, and every check between — leaving an orphaned `done`. The result is a syntactically dead script whose landed assert (count 3 → 0) still passes and whose every "goes GREEN" assert passes vacuously, because `mrun` discards stderr. The redesign below is two regions plus the `bash -n` landed assert this arm has always lacked.

- [ ] **Step 1: Hoist the definition**

Cut `scripts/board-checks.sh` lines 320–394 and paste them immediately after `renders_row`'s closing `}` (line 222), **dedented by two spaces** at every level. Then append the named terminator. The result at top level:

```bash
# --- scalar-form: an unquoted frontmatter scalar that is not well-formed YAML (change 0191).
# … (the entire existing comment block, dedented by two spaces, unchanged in content) …
scalar_form_check(){ # scalar_form_check FIELD RAW
  # … the entire existing body, dedented by two spaces, unchanged in content …
}
# --- end scalar-form helper ---
# The definition lives at TOP LEVEL (hoisted by change 0200) instead of inside the per-file walk,
# where it was redefined once per change file. Its body still reads the walk's loop variable $cid
# unqualified: bash resolves that dynamically at call time, so the hoist is behavior-neutral and
# the call sites below stay the only place $cid has to be in scope.
# The end marker above is NOT decoration. It is the named terminator mutation 4's first region
# delete bounds on — without it the range would run past this point into the walk and produce a
# syntactically dead copy that still passes every assert.
```

- [ ] **Step 2: Mark the call sites left behind in the walk**

The four lines stay exactly where they are; add one comment line above them:

```bash
  # --- scalar-form call sites (the definition is hoisted to top level; change 0200). Mutation 4
  # deletes these four lines as its SECOND region, matched individually.
  sf_title="$(field_raw "$f" title)"
  sf_blocked_by="$(fm_field_verbatim "$f" blocked_by)"
  scalar_form_check title "$sf_title"
  scalar_form_check blocked_by "$sf_blocked_by"
```

- [ ] **Step 3: Verify the hoist is behavior-neutral before touching the tests**

Run:

```bash
bash -n scripts/board-checks.sh
grep -c 'scalar_form_check' scripts/board-checks.sh   # expect 3 (definition line + two calls)
grep -c '^# --- scalar-form:' scripts/board-checks.sh # expect 1 — the top-level marker only
grep -c '^# --- end scalar-form helper ---' scripts/board-checks.sh  # expect 1
scripts/run-tests.sh tests/test_board_checks.sh
```

Expected at this point: **FAIL** — mutation 4's range delete now spans the walk. Every other scalar-form assert (mutations 1, 1b, 2, 3, 3b and the plain fixtures) must still be `ok`, because their patterns match by content (`skip leg:`, `sf_blocked_by=`, the emit arm text) and are location-independent. If any of *those* went red, the hoist itself is wrong — fix it before proceeding.

- [ ] **Step 4: Redesign mutation 4**

Replace the whole mutation 4 block in `tests/test_board_checks.sh` (from its `# Mutation 4 —` comment through its last `assert`) with:

```bash
# Mutation 4 — drop the whole scalar-form probe: every red fixture goes GREEN. TWO regions, not one
# (change 0200). The definition now sits at TOP LEVEL, so the old marker-to-marker range delete
# (`# --- scalar-form:` .. `# --- broken-spec:`) would also swallow the FILES mapfile, the walk's
# own `for` line, and every check between, leaving an orphaned `done`. That copy is syntactically
# dead — and mrun discards stderr, so the landed assert and every "goes GREEN" assert below would
# pass without the script having run at all. Region 1 is the hoisted definition, bounded by its own
# start marker and the NAMED end marker; region 2 is the four call-site lines inside the walk,
# matched individually. The `bash -n` assert is what makes a future regression of this exact shape
# impossible to miss — it is the assert this arm has never had.
mreseed
m4_before="$(grep -c 'scalar_form_check' "$MUTSCRIPT")"
awk '
  /^# --- scalar-form:/               { insf=1; next }
  /^# --- end scalar-form helper ---/ { insf=0; next }
  insf                                { next }
  /^  sf_title=/                      { next }
  /^  sf_blocked_by=/                 { next }
  /^  scalar_form_check title /       { next }
  /^  scalar_form_check blocked_by /  { next }
  { print }
' "$MUTSCRIPT" > "$MUTSCRIPT.trim"; mv "$MUTSCRIPT.trim" "$MUTSCRIPT"
m4_after="$(grep -c 'scalar_form_check' "$MUTSCRIPT")"
m4out="$(mrun)"
assert "mutation 4 landed: the scalar-form probe is gone from BOTH regions (scalar_form_check count 3 -> 0)" \
  '[ "$m4_before" = 3 ] && [ "$m4_after" = 0 ]'
assert "mutation 4 landed: the mutated copy is STILL VALID BASH — a range delete that spanned the walk would not be" \
  'bash -n "$MUTSCRIPT"'
assert "mutation 4 landed: the walk survived the delete (the for-loop opening is still there)" \
  '[ "$(grep -c "^for f in " "$MUTSCRIPT")" = 1 ]'
assert "mutation 4 (drop whole probe block): colon-space title 90 goes GREEN" \
  '! has_finding "$m4out" scalar-form 90'
assert "mutation 4 (drop whole probe block): boolean title 91 goes GREEN" \
  '! has_finding "$m4out" scalar-form 91'
assert "mutation 4 (drop whole probe block): colon-space blocked_by 92 goes GREEN" \
  '! has_finding "$m4out" scalar-form 92'
assert "mutation 4 (drop whole probe block): boolean blocked_by 93 goes GREEN" \
  '! has_finding "$m4out" scalar-form 93'
assert "mutation 4 (drop whole probe block): uppercase boolean title 85 goes GREEN" \
  '! has_finding "$m4out" scalar-form 85'
assert "mutation 4 (drop whole probe block): trailing-colon title 86 goes GREEN" \
  '! has_finding "$m4out" scalar-form 86'
# NON-VACUITY for the six GREEN asserts above: the mutant must still emit findings for OTHER checks,
# or "no scalar-form finding" would just mean "the script produced nothing at all" — exactly the
# failure the bash -n assert exists to catch, measured a second way through the output itself.
assert "mutation 4: the mutated copy still RUNS and still emits other checks' findings" \
  '[ -n "$m4out" ]'
rm -rf "$mcopy"
```

Verify the `^for f in ` assert's literal against the real file first: run `grep -c "^for f in " scripts/board-checks.sh` and use the count it reports. If the walk's opening line is indented or spelled differently, anchor the assert on whatever stable top-level line the old range would have eaten.

- [ ] **Step 5: Run to verify it passes**

Run: `scripts/run-tests.sh tests/test_board_checks.sh`
Expected: PASS, including all four new/changed mutation-4 asserts.

- [ ] **Step 6: Prove the redesign detects the failure it was written for**

Temporarily replace the two-region awk with the OLD single-range form:

```bash
awk '/# --- scalar-form:/{insf=1;next} /# --- broken-spec:/{insf=0;print;next} !insf{print}' "$MUTSCRIPT" > "$MUTSCRIPT.trim"; mv "$MUTSCRIPT.trim" "$MUTSCRIPT"
```

Re-run the file. Expected: the `mutation 4 landed: the mutated copy is STILL VALID BASH` and `the walk survived the delete` asserts go **red** — proving they detect precisely the vacuous-green state the redesign exists to prevent. Restore the two-region awk and re-run to green. Do not commit until this has been observed.

- [ ] **Step 7: Commit**

```bash
git add scripts/board-checks.sh tests/test_board_checks.sh
git commit -m "refactor(0200): hoist scalar_form_check out of the walk; redesign mutation 4 as two regions"
```

---

### Task 5: Record the frozen-merged-plans rule and pay its size-budget toll

**Files:**
- Modify: `skills/docket-convention/SKILL.md` — insert after the change-manifest fenced block (the ``` closing the frontmatter example, ~line 180) and before `### Change body sections`
- Modify: `tests/test_skill_size_budgets.sh` — the `skills/docket-convention/SKILL.md` row (~line 832) and a new producer-comment entry in the BUDGETS header block

**Interfaces:**
- Consumes: nothing from earlier tasks. Produces: nothing later tasks consume. This task is independent of Tasks 1–4 and may be built in any order relative to them.

**Why here and not in a `references/` file:** the raise rule (change 0201) requires the diff to *name* the reference file considered and *argue* why the prose cannot live there. That argument is written into the header comment in Step 3 and must not be skipped.

- [ ] **Step 1: Add the rule to the convention**

Insert, immediately after the manifest's closing ``` fence and before `### Change body sections`:

```markdown
**Merged plans and results are frozen build records.** Once a change's PR merges, its `plan:` and
`results:` files are never edited again — not to correct a stale line reference, not to update a
superseded instruction. They record what a build was *told* to do at the time it ran, which is the
only thing that makes a completed run auditable; editing one destroys that record while silently
changing what a re-read of the artifact would say the build was asked for. Corrections go in a new
change, never in the merged artifact.
```

- [ ] **Step 2: Measure the file and compute the new budgets**

Run:

```bash
wc -lw skills/docket-convention/SKILL.md
```

Apply the BUDGETS header block's own rounding rule to the measured actuals — do not copy numbers from this plan:

- **Lines:** round up to the next multiple of 5. If that leaves **2 or fewer** lines of margin, take the multiple *after* it (the header block records raising past exactly that near-zero mode).
- **Words:** round up to the next multiple of 50. If that leaves **fewer than 25** words of margin, take the multiple *after* it.

Pre-change baseline for a sanity check: 343 lines / 5971 words against a `345 6000` row. The paragraph above adds roughly 6 lines and ~85 words, so the expected result is in the neighbourhood of **355 lines / 6100 words** — if your computed values differ substantially, re-check the measurement before overriding the rule.

- [ ] **Step 3: Raise the row and write the producer comment**

Update the row in `tests/test_skill_size_budgets.sh`:

```
skills/docket-convention/SKILL.md                          <LINES> <WORDS>
```

and append to the BUDGETS header comment block, immediately after the change-0237 entry:

```bash
# Change 0200 raised docket-convention/SKILL.md 345/6000 -> <LINES>/<WORDS> to record that merged
# plan and results artifacts are FROZEN build records. The references/ file considered and rejected
# is skills/docket-convention/references/terminal-close-out.md — it is the natural topical home
# (it already owns what happens to artifacts at close-out), and it is the wrong one: a references/
# file is read ON DEMAND, and this rule has to be in hand BEFORE an agent decides to touch a merged
# plan, which is a decision taken while doing something else entirely. Step 0 loads SKILL.md
# unconditionally for every workflow skill, so that is the only surface where the rule fires before
# the action it forbids. The rule's own origin is the proof: change 0217 surfaced it because a
# merged plan's verification grep had gone stale and the reflex was to edit the plan. Set per the
# rounding rule above from the measured actuals: <ACTUAL_LINES> lines -> <LINES>, <ACTUAL_WORDS>
# words -> <WORDS>.
```

Fill every `<…>` with the real measured and computed numbers. A leftover placeholder is a plan failure.

- [ ] **Step 4: Verify the budget guard passes**

Run: `scripts/run-tests.sh tests/test_skill_size_budgets.sh`
Expected: PASS.

Then prove the row is not slack: temporarily set the line budget to the measured actual minus 1, re-run, confirm it FAILS, and restore. A budget row that cannot fail is not a guard.

- [ ] **Step 5: Confirm the 0113 plan file is untouched**

Run: `git status --porcelain docs/superpowers/plans/2026-08-05-clear-the-unfixed-review-findings-from-change-0113.md`
Expected: empty output. This task exists to rule that file frozen; modifying it would violate the rule in the same diff that records it.

- [ ] **Step 6: Commit**

```bash
git add skills/docket-convention/SKILL.md tests/test_skill_size_budgets.sh
git commit -m "docs(0200): merged plans are frozen build records; raise the convention budget row"
```

---

## Final verification

- [ ] **Run the full suite**

Run: `scripts/run-tests.sh`
Expected: every file passes. `tests/test_board_checks.sh` and `tests/test_skill_size_budgets.sh` both green, and no runtime-budget breach reported for `tests/test_board_checks.sh` (its row is 55s; the new ARQ3 fixture adds one repo).

- [ ] **Confirm the diff touches only the intended files**

Run: `git diff --stat origin/main...HEAD`
Expected exactly: `scripts/board-checks.sh`, `scripts/board-checks.md`, `tests/test_board_checks.sh`, `skills/docket-convention/SKILL.md`, `tests/test_skill_size_budgets.sh`, and this plan file.
