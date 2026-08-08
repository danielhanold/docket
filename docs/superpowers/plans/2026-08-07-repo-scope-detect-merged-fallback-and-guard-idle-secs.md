<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0250 — Repo-scope detect-merged's fallback and guard the idle-secs duplication](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-08-0250-repo-scope-detect-merged-s-fallback-and-guard-the-idle-secs.md)**
<!-- docket:backlink:end -->

# Repo-scope detect_merged's fallback and guard the idle-secs duplication — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Pass `--repo "$repo"` in `detect_merged`'s `gh pr list` fallback so a `--repo`-scoped pass queries the repository it was told to, and add a correspondence guard proving `ORPHAN_PR_IDLE_SECS` and `ABORTED_RUN_IDLE_SECS` still hold the same value.

**Architecture:** Two independent parts in one small, test-heavy PR. Part 1 is a one-token behavioral fix in `scripts/docket-status.sh` plus the doc sentence that quotes the command, driven by a new argv-recording `gh` stub in `tests/test_docket_status.sh` (the 0219 fixture idiom). Part 2 adds no script code at all — it is a test-only textual extraction of the two `NAME=` assignment lines, evaluated arithmetically in the test shell and compared by value, with an in-suite sed-mutation witness proving the guard reddens on a one-sided retune.

**Tech Stack:** Bash 4+ (`scripts/docket-status.sh`), the repo's hand-rolled `assert` test harness in `tests/test_docket_status.sh`, `jq`, `gh` (stubbed in tests), `scripts/run-tests.sh` as the suite runner.

## Global Constraints

- **Do not modify `scripts/board-checks.sh`.** ADR-0072 stands: the two idle-secs constants stay duplicated **by value**, with no shared file, no import, and no third component. Part 2 reads `board-checks.sh` as *data* (a file path), never as a dependency.
- **Never source either script to read a constant.** Docket scripts are not pure — sourcing to probe has caused real damage. Part 2 is pure text extraction plus `$(( ))` evaluation in the test shell. Never `eval` a raw file-derived line.
- **Keep grep patterns BSD-safe.** The local PATH `grep` is ugrep and accepts constructs `/usr/bin/grep` rejects. Use plain anchors and literal `-F` matching where possible; no bounded repetition.
- **Never put backticks in an `assert` description.** The harness is `assert(){ if eval "$2"; ... }` and descriptions are interpolated into a double-quoted string — backticks in a description execute.
- **Every mutation must be confirmed landed** with a `grep -c` count before and after, asserted, before its red/green result is believed. Mutate a **copy**, never the real file.
- **`--repo` is passed unconditionally**, not `${repo:+--repo "$repo"}`. `detect_merged`'s early returns (`sweep-skipped gh-unavailable`, `sweep-skipped repo-unresolved`) guarantee `$repo` is resolved and shape-valid before the fallback runs; a conditional would imply a reachable unresolved path that does not exist.
- Test command for the whole suite: `scripts/run-tests.sh`. Single file: `bash tests/test_docket_status.sh`.

## File Structure

| File | Responsibility in this change |
|---|---|
| `scripts/docket-status.sh` | Modify (Part 1): one line in `detect_merged`'s fallback arm gains `--repo "$repo"`, plus a two-line call-site comment mirroring `detect_orphan_pr`'s. No other edit. |
| `scripts/docket-status.md` | Modify (Part 1): section **5. Batched sweep detection** quotes the fallback command verbatim without `--repo`; that quote becomes stale the moment the script changes. |
| `tests/test_docket_status.sh` | Modify (both parts): a new argv-witness block for `detect_merged` (Part 1) and a new correspondence-guard block (Part 2). |
| `scripts/board-checks.sh` | **Untouched.** Read by the Part-2 guard as a file path only. |

---

### Task 1: Repo-scope `detect_merged`'s `gh pr list` fallback

**Files:**
- Modify: `scripts/docket-status.sh:555` (the fallback arm inside `detect_merged`)
- Modify: `scripts/docket-status.md:159-163` (section 5's verbatim command quote)
- Test: `tests/test_docket_status.sh` — new block inserted after the existing `detect_merged | sweep_execute` asserts (currently ending at line 773) and **before** the `# ============ detect_orphan_pr` header (currently line 776)

**Interfaces:**
- Consumes: nothing from other tasks.
- Produces: nothing other tasks rely on. Task 2 is fully independent and may be built in either order.

**Context an implementer needs.** `detect_merged` builds a candidate list of `status: implemented` changes, resolves `repo` once (from `REPO_FLAG`, else `gh repo view`), shape-checks it (`owner`/`name` split), and then takes one of two arms per candidate: a batched `gh api graphql` arm when the change has a `pr:` number, and a per-change `gh pr list` fallback when it does not. Only the fallback is missing `--repo`. The correct shape already exists one function away in `detect_orphan_pr` (~line 728), comment included.

- [ ] **Step 1: Write the failing test**

Insert this block into `tests/test_docket_status.sh` immediately after the line

```bash
assert "detect_merged | sweep_execute: no bogus close-out output for the skip line" \
  '! printf "%s\n" "$pipe_out" | grep -Eq "^(swept|harvest|sweep-failed) "'
```

and before the `# ============ detect_orphan_pr` header:

```bash
# ---- detect_merged's FALLBACK arm is repo-scoped (change 0250) ----
# The graphql arm names owner/name inside the query, so it always honored the resolution. The
# per-change `gh pr list` fallback did not pass --repo at all, so under a --repo-scoped pass it
# queried whatever repository the process CWD implies — a different repository than board_pass and
# github-mirror.sh, which both forward the flag. Same defect, same fix, and the same witness shape
# as the detect_orphan_pr argv block below: stdout cannot see an argv, so the stub records its own.
detect_fb_dir="$tmp/detect-fallback-case"
mkdir -p "$detect_fb_dir/docs/changes/active"
# pr: is EMPTY on purpose — that is the only way the fallback arm is reached.
cat > "$detect_fb_dir/docs/changes/active/0012-fallback-thing.md" <<'EOF'
---
id: 12
slug: fallback-thing
title: Fallback thing
status: implemented
priority: high
depends_on: []
branch: feat/fallback-thing
pr:
EOF

cat > "$tmp/gh-detect-argv.sh" <<EOF
#!/usr/bin/env bash
printf '%s\n' "\$*" >> "$tmp/gh-detect-argv.log"
if [ "\$1" = repo ] && [ "\$2" = view ]; then echo "x/y"; exit 0; fi
if [ "\$1" = pr ] && [ "\$2" = list ]; then
  echo '[{"number":301,"mergedAt":"2026-07-06T10:00:00Z"}]'
  exit 0
fi
exit 1
EOF
chmod +x "$tmp/gh-detect-argv.sh"

rm -f "$tmp/gh-detect-argv.log"
detect_fb_out="$( cd "$detect_fb_dir" && \
  DOCKET_MODE=main CHANGES_DIR=docs/changes GH="$tmp/gh-detect-argv.sh" \
  bash -c '. "'"$SCRIPT"'"; detect_merged' )"
detect_fb_prlist="$(grep -E '^pr list' "$tmp/gh-detect-argv.log" || true)"
assert "detect_merged fallback: the pr list call happens at all (the argv witness is not vacuous)" \
  '[ -n "$detect_fb_prlist" ]'
assert "detect_merged fallback: the resolved repo REACHES the pr list call as --repo x/y" \
  '[ -n "$detect_fb_prlist" ] && ! grep -qvF -- "--repo x/y" <<<"$detect_fb_prlist"'
detect_fb_expected="$(printf '12\tfallback-thing\t301\t2026-07-06')"
assert "detect_merged fallback: the argv-witness run still emits its merged candidate" \
  'printf "%s\n" "$detect_fb_out" | grep -qF "$detect_fb_expected"'

# REPO_FLAG end-to-end. REPO_FLAG is assigned AFTER sourcing on purpose: the source runs the
# script's argument-parsing prologue, which resets REPO_FLAG to the empty string, so an environment
# value would be clobbered. Mirrors the detect_orphan_pr REPO_FLAG block below.
rm -f "$tmp/gh-detect-argv.log"
detect_fb_flag_out="$( cd "$detect_fb_dir" && \
  DOCKET_MODE=main CHANGES_DIR=docs/changes GH="$tmp/gh-detect-argv.sh" \
  bash -c '. "'"$SCRIPT"'"; REPO_FLAG=someone/elsewhere; detect_merged' )"
detect_fb_flag_prlist="$(grep -E '^pr list' "$tmp/gh-detect-argv.log" || true)"
assert "detect_merged fallback: REPO_FLAG is honored end-to-end on every pr list call" \
  '[ -n "$detect_fb_flag_prlist" ] && ! grep -qvF -- "--repo someone/elsewhere" <<<"$detect_fb_flag_prlist"'
assert "detect_merged fallback: with REPO_FLAG set no gh repo view subprocess is spent" \
  '! grep -q "^repo view" "$tmp/gh-detect-argv.log"'
assert "detect_merged fallback: the REPO_FLAG run still emits its merged candidate" \
  'printf "%s\n" "$detect_fb_flag_out" | grep -qF "$detect_fb_expected"'
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_docket_status.sh 2>&1 | grep -n "detect_merged fallback"`

Expected: the two `--repo` asserts print `NOT OK` —

```
NOT OK - detect_merged fallback: the resolved repo REACHES the pr list call as --repo x/y
NOT OK - detect_merged fallback: REPO_FLAG is honored end-to-end on every pr list call
```

The four other asserts in the block (witness non-vacuity, both merged-candidate emissions, and the no-`repo view` assert on the REPO_FLAG run) must print `ok` **already**. If any of those four is red, the fixture is wrong — fix the fixture before touching the script, because a red witness cannot prove the fix landed.

- [ ] **Step 3: Write the minimal implementation**

In `scripts/docket-status.sh`, replace this single line in `detect_merged`'s fallback arm (currently line 555):

```bash
      pl_json="$("$GH" pr list --head "feat/$slug" --state merged --json number,mergedAt 2>/dev/null)"
```

with the comment plus the scoped call:

```bash
      # --repo "$repo" is what SPENDS the resolution above. Without it gh infers the repository
      # from the process CWD, so a pass invoked with --repo would query one repository here and a
      # different one in board_pass / github-mirror.sh, which both forward the flag. Unconditional,
      # not ${repo:+...}: the early returns above guarantee $repo is resolved and shape-valid by
      # the time this arm runs. Same shape as detect_orphan_pr's single batched call.
      pl_json="$("$GH" pr list --repo "$repo" --head "feat/$slug" --state merged --json number,mergedAt 2>/dev/null)"
```

Change nothing else: not the `$?` check on the next line, not the batched graphql arm, not the skip reasons, not any output token.

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_docket_status.sh 2>&1 | grep -c "^NOT OK"`

Expected: `0`. Then confirm the new block specifically is green:

Run: `bash tests/test_docket_status.sh 2>&1 | grep "detect_merged fallback"`

Expected: six lines, all beginning `ok - `.

- [ ] **Step 5: Update the script contract**

`scripts/docket-status.md` section **5. Batched sweep detection** quotes the fallback command verbatim and is now stale. Replace this text (currently around lines 159-163):

```
(for changes with a known `pr:` number) plus a per-change `gh pr list --head feat/<slug> --state
merged` fallback for changes without one, and emits merged changes as TAB-separated
```

with:

```
(for changes with a known `pr:` number) plus a per-change `gh pr list --repo <repo> --head
feat/<slug> --state merged` fallback for changes without one — the fallback is repo-scoped with the
same resolved `<repo>` as the batched arm, so a `--repo`-scoped pass never falls back to the
repository the process CWD implies — and emits merged changes as TAB-separated
```

- [ ] **Step 6: Verify the doc edit and re-run the suite**

Run: `grep -n 'pr list' scripts/docket-status.md`

Expected: the section-5 line now reads `gh pr list --repo <repo> --head`, and the `detect_orphan_pr` quote further down (line ~252) is unchanged.

Run: `scripts/run-tests.sh`

Expected: the full suite passes.

- [ ] **Step 7: Commit**

```bash
git add scripts/docket-status.sh scripts/docket-status.md tests/test_docket_status.sh
git commit -m "fix(0250): repo-scope detect_merged's gh pr list fallback"
```

---

### Task 2: Correspondence guard over the idle-secs duplication

**Files:**
- Modify: `tests/test_docket_status.sh` — new block inserted after the `rm -rf "$orphan_mutcopy"` line that closes the detect_orphan_pr mutation arms (currently line 1323) and **before** the `# sweep_execute: chained close-out (task 5).` comment (currently line 1325)
- Read as data (never modified): `scripts/docket-status.sh`, `scripts/board-checks.sh`

**Interfaces:**
- Consumes: nothing from Task 1.
- Produces: nothing. Independent of Task 1 in both directions.

**Context an implementer needs.** ADR-0072 deliberately accepted a by-value duplication: `ORPHAN_PR_IDLE_SECS=$(( 2 * 3600 ))` in `scripts/docket-status.sh` (currently line 577) and `ABORTED_RUN_IDLE_SECS=$(( 2 * 3600 ))` in `scripts/board-checks.sh` (currently line 189). `board-checks.sh` must stay independently runnable and offline, so no shared file, import, or third component is allowed — the ADR named the drift cost as accepted and unguarded. Today `grep -c ORPHAN_PR_IDLE_SECS tests/test_docket_status.sh` returns `0`: a one-sided retune breaks the agreement and nothing goes red. This task closes exactly that, and only that: the **values**, not the predicate shape.

Comparing *evaluated values* rather than RHS strings is deliberate — it tolerates a reformat (`7200` vs `2 * 3600`) while still reddening on any real retune.

- [ ] **Step 1: Write the guard and its mutation witness**

Insert this block into `tests/test_docket_status.sh` immediately after the `rm -rf "$orphan_mutcopy"` line closing orphan mutation 3, and before the `# sweep_execute: chained close-out (task 5).` comment:

```bash
# ---- ADR-0072 correspondence guard: the two idle-secs constants agree BY VALUE (change 0250) ----
# ORPHAN_PR_IDLE_SECS (docket-status.sh) is a deliberate by-value COPY of ABORTED_RUN_IDLE_SECS
# (board-checks.sh): the two scripts share no library and board-checks.sh must stay independently
# runnable offline, so ADR-0072 accepted the duplication and priced the drift. The price was that a
# one-sided retune breaks the agreement silently. This guard is that price paid: pure textual
# extraction of each assignment line, arithmetic evaluation in THIS shell, value comparison.
# NOT by sourcing either script — docket scripts are not pure, and sourcing to probe has cost real
# damage. NOT by string-comparing the RHS — 7200 and 2 * 3600 are the same constant. board-checks.sh
# is read here as a FILE PATH, which adds nothing to either script's dependency graph.
idle_secs_line(){  # idle_secs_line FILE VARNAME -> the single matching assignment line
  grep -e "^$2=" "$1"
}
idle_secs_value(){  # idle_secs_value FILE VARNAME -> the evaluated integer, or empty on refusal
  local isv_raw isv_expr
  isv_raw="$(idle_secs_line "$1" "$2")" || return 1
  isv_expr="${isv_raw#*=}"
  # Unwrap $(( ... )) when present; a bare NAME=7200 form is tolerated unwrapped.
  case "$isv_expr" in
    '$(('*'))') isv_expr="${isv_expr#\$((}"; isv_expr="${isv_expr%))}" ;;
  esac
  # Refuse to evaluate anything that is not plain integer arithmetic. A trailing comment, a
  # variable reference, or a command substitution reddens the guard rather than being mis-stripped.
  case "$isv_expr" in
    *[!0-9\ \(\)\*\+-]*) return 1 ;;
    '') return 1 ;;
  esac
  printf '%s' "$(( isv_expr ))"
}

IDLE_DS="$REPO/scripts/docket-status.sh"
IDLE_BC="$REPO/scripts/board-checks.sh"
# Non-vacuity FIRST: each anchor must match EXACTLY ONE line. A rename, a removal, or a second
# assignment makes the extraction meaningless, and that IS the finding — an extraction that quietly
# returns nothing would otherwise read as the property holding.
idle_ds_n="$(grep -c -e '^ORPHAN_PR_IDLE_SECS=' "$IDLE_DS" || true)"
idle_bc_n="$(grep -c -e '^ABORTED_RUN_IDLE_SECS=' "$IDLE_BC" || true)"
assert "ADR-0072 guard: ORPHAN_PR_IDLE_SECS is assigned exactly once in docket-status.sh" \
  '[ "${idle_ds_n:-0}" -eq 1 ]'
assert "ADR-0072 guard: ABORTED_RUN_IDLE_SECS is assigned exactly once in board-checks.sh" \
  '[ "${idle_bc_n:-0}" -eq 1 ]'

idle_ds_v="$(idle_secs_value "$IDLE_DS" ORPHAN_PR_IDLE_SECS || true)"
idle_bc_v="$(idle_secs_value "$IDLE_BC" ABORTED_RUN_IDLE_SECS || true)"
assert "ADR-0072 guard: both idle-secs values extract and evaluate to a positive integer" \
  '[ -n "$idle_ds_v" ] && [ -n "$idle_bc_v" ] && [ "$idle_ds_v" -gt 0 ] && [ "$idle_bc_v" -gt 0 ]'
assert "ADR-0072 guard: ORPHAN_PR_IDLE_SECS equals ABORTED_RUN_IDLE_SECS by value" \
  '[ "$idle_ds_v" = "$idle_bc_v" ]'

# The guard's own sensitivity, proven IN-SUITE rather than by a one-off manual check: retune one
# side on a throwaway COPY and assert the comparison reddens. cp, never git checkout -- : the
# restore-to-HEAD idiom discards uncommitted work and produces a meaningless reading.
idle_mutcopy="$(mktemp -d)"
cp "$IDLE_DS" "$idle_mutcopy/docket-status.sh"
idle_mut_before="$(grep -c -e '^ORPHAN_PR_IDLE_SECS=' "$idle_mutcopy/docket-status.sh" || true)"
sed 's|^ORPHAN_PR_IDLE_SECS=.*|ORPHAN_PR_IDLE_SECS=$(( 9 * 3600 ))|' \
  "$idle_mutcopy/docket-status.sh" > "$idle_mutcopy/docket-status.sh.t"
mv "$idle_mutcopy/docket-status.sh.t" "$idle_mutcopy/docket-status.sh"
idle_mut_after="$(grep -c -e '^ORPHAN_PR_IDLE_SECS=$(( 9 \* 3600 ))' "$idle_mutcopy/docket-status.sh" || true)"
assert "ADR-0072 guard: the one-sided retune mutation actually landed on the copy" \
  '[ "${idle_mut_before:-0}" -eq 1 ] && [ "${idle_mut_after:-0}" -eq 1 ]'
assert "ADR-0072 guard: the mutated copy is still valid bash" \
  'bash -n "$idle_mutcopy/docket-status.sh"'
idle_mut_v="$(idle_secs_value "$idle_mutcopy/docket-status.sh" ORPHAN_PR_IDLE_SECS || true)"
assert "ADR-0072 guard: the mutated copy still extracts cleanly (the witness is not vacuous)" \
  '[ -n "$idle_mut_v" ]'
assert "ADR-0072 guard REDDENS on a one-sided retune — the guard can actually fail" \
  '[ "$idle_mut_v" != "$idle_bc_v" ]'
rm -rf "$idle_mutcopy"
```

- [ ] **Step 2: Run the suite and verify every new assert is green**

Run: `bash tests/test_docket_status.sh 2>&1 | grep "ADR-0072 guard"`

Expected: eight lines, all beginning `ok - `. The two equality-side asserts are green on arrival by construction — that is expected and is exactly why the mutation witness in the same block is the load-bearing part.

Run: `bash tests/test_docket_status.sh 2>&1 | grep -c "^NOT OK"`

Expected: `0`.

- [ ] **Step 3: Prove the guard reddens against the REAL files, not only against the copy**

The in-suite witness above compares the mutated copy to `board-checks.sh`. Confirm once, by hand, that a real one-sided retune reddens the real equality assert — then restore from the backup, never from git:

```bash
cp scripts/board-checks.sh /tmp/bc-0250.bak
sed 's|^ABORTED_RUN_IDLE_SECS=.*|ABORTED_RUN_IDLE_SECS=$(( 5 * 3600 ))|' scripts/board-checks.sh > /tmp/bc-0250.t
mv /tmp/bc-0250.t scripts/board-checks.sh
grep -c '^ABORTED_RUN_IDLE_SECS=$(( 5 \* 3600 ))' scripts/board-checks.sh
bash tests/test_docket_status.sh 2>&1 | grep "ADR-0072 guard"
mv /tmp/bc-0250.bak scripts/board-checks.sh
git diff --stat scripts/board-checks.sh
```

Expected: the `grep -c` prints `1` (the mutation landed); the assert run shows

```
NOT OK - ADR-0072 guard: ORPHAN_PR_IDLE_SECS equals ABORTED_RUN_IDLE_SECS by value
```

with the other seven still `ok`; and after the restore `git diff --stat scripts/board-checks.sh` prints **nothing** — `board-checks.sh` must leave this change byte-identical.

- [ ] **Step 4: Run the full suite**

Run: `scripts/run-tests.sh`

Expected: the full suite passes, and `git status --short scripts/board-checks.sh` is empty.

- [ ] **Step 5: Commit**

```bash
git add tests/test_docket_status.sh
git commit -m "test(0250): correspondence guard over the ADR-0072 idle-secs duplication"
```

---

## Self-Review

**1. Spec coverage.**

| Spec requirement | Task |
|---|---|
| Part 1 — `--repo "$repo"` unconditionally in the fallback | Task 1 Step 3 |
| Part 1 — mirror `detect_orphan_pr`'s call-site "SPENDS the resolution" comment | Task 1 Step 3 |
| Part 1 tests — dedicated argv-recording GH stub (assumption 7) | Task 1 Step 1, `gh-detect-argv.sh` |
| Part 1 tests — fixture with `pr:` empty so the fallback arm is taken | Task 1 Step 1, `0012-fallback-thing.md` |
| Part 1 tests — assert 1: non-vacuous `pr list` witness | Task 1 Step 1 |
| Part 1 tests — assert 2: every `pr list` carries `--repo x/y` | Task 1 Step 1 |
| Part 1 tests — assert 3: `REPO_FLAG` end-to-end, assigned after sourcing, no `repo view` spent | Task 1 Step 1 |
| Part 1 tests — assert 4: the merged-candidate line still emitted | Task 1 Step 1 (both runs) |
| Part 1 — mandatory `scripts/docket-status.md` update (assumption 5) | Task 1 Steps 5-6 |
| Part 2 — exactly-one-match extraction anchors on both files | Task 2 Step 1 |
| Part 2 — strip `NAME=` and the `$(( ))` wrapper, tolerate a bare value | Task 2 Step 1, `idle_secs_value` |
| Part 2 — arithmetic eval in the test shell, no sourcing, no bare `eval` | Task 2 Step 1 |
| Part 2 — value-equality assert | Task 2 Step 1 |
| Part 2 — in-suite sed-mutation witness with before/after counts | Task 2 Step 1 |
| Part 2 — guard lives in `tests/test_docket_status.sh` (assumption 3) | Task 2 file list |
| Frontmatter `adrs: [72]`, no ADR body touch, no `docket-adr` invocation | Already set on the change; no task needed |
| No change to `scripts/board-checks.sh` | Global Constraints; asserted in Task 2 Steps 3-4 |
| TDD order: argv asserts red first, then the one-line fix | Task 1 Steps 1-4 |

No gaps.

**2. Placeholder scan.** No TBD/TODO, no "similar to Task N", no "add appropriate error handling". Every code step carries the literal text to insert and the exact insertion anchor; every run step names the command and the expected output.

**3. Type consistency.** Shell function names used consistently: `idle_secs_line` and `idle_secs_value` are defined and called only in Task 2. Variable prefixes are collision-free against the existing file — `detect_fb_*` and `gh-detect-argv*` in Task 1 (the file's existing names are `detect_*`, `gh-detect-ok`, `gh-detect-fail`, `orphan_*`, `gh-orphan-argv`), `idle_*`/`IDLE_*` in Task 2. `$REPO`, `$SCRIPT`, `$tmp`, and `assert` are the file's own pre-existing harness names, used as defined at its top.
