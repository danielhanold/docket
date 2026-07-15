# link-skills.sh creates a missing skills subdir when the harness is present — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix `link-skills.sh` so a harness the user actually uses (parent dir present) but whose `skills/` subdir does not yet exist gets that subdir created and docket's skills linked into it, instead of being silently skipped.

**Architecture:** The linker loops over a fixed list of per-harness `skills` dirs (`~/.claude/skills`, `~/.cursor/skills`, …). Today the inner guard `[ -d "$dir" ] || continue` gates on the **skills subdir** itself, so a present harness with no `skills/` yet is skipped. Re-anchor the guard to the **parent** harness dir (`dirname "$dir"`): skip only when the parent is absent (the user genuinely does not use that harness); when the parent is present but the `skills/` subdir is missing, `mkdir -p` it and link as before.

**Tech Stack:** POSIX Bash (`set -euo pipefail`), run on both GNU/Linux and macOS/BSD. Test harness is the repo's plain `assert`-based `tests/test_link_skills.sh`, driven through the `DOCKET_HARNESS_ROOT` env seam.

## Global Constraints

- The script and test run under **both GNU and BSD** userlands — use only POSIX-portable constructs (`dirname`, `mkdir -p`, `[ -d ]`); no GNU-only flags.
- macOS `mktemp -d` returns a `/var/…` path that git/readlink report as `/private/var/…`; the existing test compares `readlink` output against `$REPO/skills/...` (the repo path, not the tmp path), so this trap does not bite here — do **not** introduce any tmp-path-prefix comparison.
- The fix must apply **uniformly** to all six listed harnesses via the single shared loop — no per-harness special-casing.
- Preserve every existing behavior: absolute symlink targets, idempotency (`Created: 0` on re-run), pre-existing files left untouched, dangling symlinks left untouched, and **never materialize a fully-absent harness** (no parent dir → still skipped).

---

### Task 1: Re-anchor the harness guard to the parent dir + cover the new contract

**Files:**
- Modify: `link-skills.sh` (inner guard at the `for dir in "${HARNESS_SKILL_DIRS[@]}"` loop; header comment)
- Test: `tests/test_link_skills.sh` (fixture setup + assertions)

**Interfaces:**
- Consumes: nothing new — the `DOCKET_HARNESS_ROOT` test seam and `HARNESS_SKILL_DIRS` array already exist.
- Produces: no new callable interface; behavior change only. New guarantee: a harness whose parent dir exists but whose `skills/` subdir is absent gets `skills/` created and all `skills/*/` linked into it.

- [ ] **Step 1: Write the failing tests**

Edit `tests/test_link_skills.sh`. Change the fixture setup block so one harness has its **parent present but `skills/` subdir absent**, and keep at least one harness **fully absent** (no parent dir) for the invariant.

Replace this block:

```bash
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
# Fake harness root: SOME dirs present, some absent on purpose.
mkdir -p "$tmp/.claude/skills" "$tmp/.agents/skills"   # present
# .cursor/.codex/.kiro/.windsurf intentionally absent

DOCKET_HARNESS_ROOT="$tmp" bash "$REPO/link-skills.sh" >/dev/null
```

with:

```bash
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
# Fake harness root: three states on purpose.
mkdir -p "$tmp/.claude/skills" "$tmp/.agents/skills"   # present WITH skills subdir
mkdir -p "$tmp/.cursor"                                 # harness present, skills subdir ABSENT
# .codex/.kiro/.windsurf fully absent (no parent dir at all)

DOCKET_HARNESS_ROOT="$tmp" bash "$REPO/link-skills.sh" >/dev/null
```

Then, immediately after the existing `assert "does NOT create an absent harness dir" ...` line, replace that one assertion and add the two new-contract assertions. Replace:

```bash
assert "does NOT create an absent harness dir" '[ ! -d "$tmp/.cursor/skills" ]'
```

with:

```bash
assert "creates missing skills subdir under a present harness" '[ -d "$tmp/.cursor/skills" ]'
assert "links into the created .cursor/skills"                 '[ -L "$tmp/.cursor/skills/docket-status" ]'
assert "does NOT create a fully-absent harness dir" '[ ! -e "$tmp/.codex" ] && [ ! -e "$tmp/.codex/skills" ]'
```

- [ ] **Step 2: Run the tests to verify the new-contract assertions FAIL**

Run: `bash tests/test_link_skills.sh; echo "exit=$?"`
Expected: FAIL — `NOT OK - creates missing skills subdir under a present harness` and `NOT OK - links into the created .cursor/skills` (the current script's `[ -d "$dir" ] || continue` skips `.cursor` because its `skills/` subdir is absent), `exit=1`. The fully-absent and other assertions still print `ok`.

- [ ] **Step 3: Re-anchor the guard in `link-skills.sh`**

In `link-skills.sh`, change the inner harness guard. Replace:

```bash
  for dir in "${HARNESS_SKILL_DIRS[@]}"; do
    [ -d "$dir" ] || continue           # only link into harnesses that exist
    link="$dir/$name"
```

with:

```bash
  for dir in "${HARNESS_SKILL_DIRS[@]}"; do
    [ -d "$(dirname "$dir")" ] || continue   # only into harnesses the user actually uses (parent present)
    [ -d "$dir" ] || mkdir -p "$dir"         # harness present but skills/ subdir missing → create it
    link="$dir/$name"
```

Also update the header comment so the prose matches the new behavior. Replace:

```bash
# Idempotent: only creates MISSING links, and only into harness dirs that ALREADY EXIST
# (we never create a harness you don't use). Verify each harness's exact skills dir if
# this list drifts.
```

with:

```bash
# Idempotent: only creates MISSING links, and only into harnesses that ALREADY EXIST —
# the parent harness dir must be present (we never create a harness you don't use); a
# present harness whose skills/ subdir is absent gets that subdir created. Verify each
# harness's exact skills dir if this list drifts.
```

- [ ] **Step 4: Run the tests to verify they PASS**

Run: `bash tests/test_link_skills.sh; echo "exit=$?"`
Expected: every line `ok - …`, `exit=0`. In particular `ok - creates missing skills subdir under a present harness`, `ok - links into the created .cursor/skills`, `ok - does NOT create a fully-absent harness dir`, and the unchanged idempotency / pre-existing-file / dangling-symlink assertions.

- [ ] **Step 5: Mutation-test the new assertions (guard-is-code)**

Temporarily revert only the guard line to the old `[ -d "$dir" ] || continue` (keep the `mkdir` line removed), re-run `bash tests/test_link_skills.sh`, and confirm the two new-contract assertions go **NOT OK** (the guard is real, not decoration). Then restore the fix. Do the same reverse check for the invariant: temporarily change the parent guard to always-true and confirm `does NOT create a fully-absent harness dir` goes NOT OK, then restore.

Expected: with the fix reverted the suite reports `NOT OK - creates missing skills subdir under a present harness`; with the parent guard defeated the suite reports `NOT OK - does NOT create a fully-absent harness dir`. Both restore to all-green.

- [ ] **Step 6: Run the full test suite**

There is no unified runner; the suite is the set of `tests/test_*.sh` files run directly. Run them all and surface any non-green:

```bash
fail=0
for t in tests/test_*.sh; do
  if bash "$t" >/tmp/lk_$$.out 2>&1; then :; else echo "FAILED: $t"; tail -5 /tmp/lk_$$.out; fail=1; fi
done
echo "suite exit=$fail"; rm -f /tmp/lk_$$.out
```

Expected: `suite exit=0` — no new failures versus `origin/main`. If any pre-existing test fails for environment reasons (per LEARNINGS: ambient `DOCKET_SCRIPTS_DIR`, unresolvable `origin/HEAD`, umask/timeout), re-run that same test against unmodified `origin/main`, byte-compare, and record the differential rather than treating it as a regression.

- [ ] **Step 7: Commit**

```bash
git add link-skills.sh tests/test_link_skills.sh
git commit -m "fix(0080): link-skills.sh creates a missing skills/ subdir when the harness is present"
```
