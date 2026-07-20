# Pin the `finalize.test_command` auto sentinel's cross-layer masking — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add three fixtures to section S of `tests/test_docket_config.sh` that pin the `finalize.test_command` `auto` sentinel's cross-layer masking behavior — a higher layer's `auto` masks a lower layer's real command — and prove each fixture reddens under a real mutation.

**Architecture:** Pure test-only change. No production code is modified — `scripts/docket-config.sh` is touched *only* temporarily during mutation runs and restored with `git checkout` before every commit. The three fixtures append to the existing section S, reusing the suite's `mkrepo` and `rung` helpers.

**Tech Stack:** Bash, POSIX shell test harness (`tests/test_docket_config.sh`), git fixtures with bare origins.

## Global Constraints

- **This change pins current behavior. It must NOT alter the sentinel's semantics or the placement of the collapse at `scripts/docket-config.sh:201`.** If a fixture is red against unmodified code, the fixture is wrong — never "fix" it by editing `docket-config.sh`.
- **Baseline suite state (measured on this branch's base commit, `3e26790`): `216 ok`, `0 NOT OK`, `PASS`.** The final state must be `221 ok`, `0 NOT OK`, `PASS` (five new asserts).
- **Read the `ok` COUNT as part of the contract on every mutation run.** A mutation that lowers the count while producing zero `NOT OK` is a vacuous guard announcing itself — record before/after counts for every mutation.
- **The suite takes 3–8 minutes.** Always run it in ONE foreground call with an explicit long timeout (`timeout: 600000`). Never background it and never run it more than once per invocation — redirect to a file and read counts from that file.
- Run the suite from the repo root: `bash tests/test_docket_config.sh`.
- All new asserts are prefixed `0106 s<N>` so they are greppable as this change's contribution.

## How the three config layers are read (verified against `scripts/docket-config.sh`)

Fixture correctness depends on this, so it is stated once here and every task relies on it:

| Layer | Precedence | Read from | Fixture must |
|---|---|---|---|
| `.docket.local.yml` | highest (`lcl`, `:148`) | the **working tree** (`$REPO_DIR/.docket.local.yml`) | just write the file — **no commit** |
| `.docket.yml` | middle (`:129`) | **`git show origin/HEAD:.docket.yml`** | write, `git add`, `git commit`, **and `git push`** |
| `<xdg>/docket/config.yml` | lowest (`gbl`, `:139`) | the XDG dir passed to `rung` | just write the file — no commit |

The `rung <xdgdir> <repodir>` helper (`tests/test_docket_config.sh:34`) roots the global layer at a per-fixture temp dir; the suite pins `XDG_CONFIG_HOME` at a void (`:31`), so no fixture reads the developer's real `~/.config`.

## File Structure

- **Modify:** `tests/test_docket_config.sh` — append fixtures `s4`, `s5`, `s6` to section S, immediately after the `s3` assert (currently line 1029) and **before** the trailing `if [ "$fail" = 0 ]; then echo PASS; else echo FAIL; fi` / `exit "$fail"` epilogue (currently lines 1030–1031).
- **Temporarily modify then restore (mutation runs only):** `scripts/docket-config.sh` — never committed in a modified state.

No new files. No production code changes.

---

### Task 1: Forward masking — a higher layer's `auto` masks a lower layer's real command (`s4`, `s5`)

**Files:**
- Modify: `tests/test_docket_config.sh` (append after line 1029, before the `if [ "$fail" = 0 ]` epilogue)
- Temporarily mutate then restore: `scripts/docket-config.sh:201`

**Interfaces:**
- Consumes: `mkrepo <dir>` (`:13`), `rung <xdgdir> <repodir> [args...]` (`:34`), `assert <name> <expr>` (`:8`), the `$tmp` scratch root (`:29`).
- Produces: fixtures `$tmp/s4`, `$tmp/s5` and XDG roots `$tmp/s4.xdg`, `$tmp/s5.xdg`. Task 2 adds `$tmp/s6` / `$tmp/s6.xdg` and depends on nothing from this task except that its code is appended *above* Task 2's.

**Why each fixture carries a CONTROL assert.** `s4` and `s5` both assert `FINALIZE_TEST_COMMAND` is **empty** — which is also exactly what an *absent* key resolves to. That is the "probe value coincides with the default" vacuity trap: if the fixture silently failed to populate its lower rung, the assert would pass anyway. Each fixture therefore first asserts the lower layer's real command **does** resolve, then adds the masking layer and asserts it is gone. The control makes the masking assert load-bearing without relying solely on the mutation run.

**Why each `eval` is preceded by a poison value.** A resolver run that aborts emits nothing, and `eval ""` silently leaves the *previous* fixture's value in place — a stale-state false pass. Setting `FINALIZE_TEST_COMMAND=__poison__` first makes a vacuous `eval` fail loudly instead.

- [ ] **Step 1: Write the failing fixtures**

Append to `tests/test_docket_config.sh`, immediately after the `s3` assert line
(`assert "test_command AUTO is NOT the sentinel (case-sensitive)" …`) and **before** the
`if [ "$fail" = 0 ]` epilogue:

```bash

# --- (S4/S5/S6) change 0106: the sentinel's CROSS-LAYER masking -------------
# The collapse at scripts/docket-config.sh:201 runs AFTER the :194 resolution chain. That
# placement is the whole point: a HIGHER layer writing `test_command: auto` MASKS a LOWER
# layer's real command, which is the correct reading of an explicit re-statement of the
# default. Collapse per-layer instead and the behavior silently INVERTS — the higher `auto`
# becomes empty, the `:-` chain falls through, and the lower command resurfaces.
# Sections s/s2/s3 above are all single-layer, so none of them can see this. These do.
#
# s4 and s5 assert an EMPTY value, which is also what an ABSENT key yields — so each first
# asserts its lower rung really does resolve (the control), then adds the masking layer.
# Each eval is preceded by a poison value: an aborted run emits nothing, and a bare
# `eval ""` would otherwise leave the previous fixture's value standing.

# (s4) FORWARD, lcl() path: .docket.local.yml `auto` over a committed real command.
mkrepo "$tmp/s4"
mkdir -p "$tmp/s4.xdg/docket"
cat > "$tmp/s4/.docket.yml" <<'EOF'
metadata_branch: main
integration_branch: main
finalize:
  test_command: make test
EOF
git -C "$tmp/s4" add .docket.yml; git -C "$tmp/s4" commit --quiet -m cfg
git -C "$tmp/s4" push --quiet origin main
FINALIZE_TEST_COMMAND=__poison__
out="$(rung "$tmp/s4.xdg" "$tmp/s4" --export)"; eval "$out"
assert "0106 s4 control: committed real command resolves before masking" '[ "$FINALIZE_TEST_COMMAND" = "make test" ]'
printf 'finalize:\n  test_command: auto\n' > "$tmp/s4/.docket.local.yml"
FINALIZE_TEST_COMMAND=__poison__
out="$(rung "$tmp/s4.xdg" "$tmp/s4" --export)"; eval "$out"
assert "0106 s4: local auto masks committed real command" '[ -z "$FINALIZE_TEST_COMMAND" ]'

# (s5) FORWARD, gbl() path: committed `auto` over a global real command.
mkrepo "$tmp/s5"
mkdir -p "$tmp/s5.xdg/docket"
printf 'finalize:\n  test_command: make global\n' > "$tmp/s5.xdg/docket/config.yml"
cat > "$tmp/s5/.docket.yml" <<'EOF'
metadata_branch: main
integration_branch: main
EOF
git -C "$tmp/s5" add .docket.yml; git -C "$tmp/s5" commit --quiet -m cfg
git -C "$tmp/s5" push --quiet origin main
FINALIZE_TEST_COMMAND=__poison__
out="$(rung "$tmp/s5.xdg" "$tmp/s5" --export)"; eval "$out"
assert "0106 s5 control: global real command resolves before masking" '[ "$FINALIZE_TEST_COMMAND" = "make global" ]'
cat > "$tmp/s5/.docket.yml" <<'EOF'
metadata_branch: main
integration_branch: main
finalize:
  test_command: auto
EOF
git -C "$tmp/s5" add .docket.yml; git -C "$tmp/s5" commit --quiet -m cfg2
git -C "$tmp/s5" push --quiet origin main
FINALIZE_TEST_COMMAND=__poison__
out="$(rung "$tmp/s5.xdg" "$tmp/s5" --export)"; eval "$out"
assert "0106 s5: committed auto masks global real command" '[ -z "$FINALIZE_TEST_COMMAND" ]'
```

- [ ] **Step 2: Run the suite to verify the new fixtures PASS against unmodified code**

This change pins EXISTING behavior, so the fixtures are green from the start — the red state
is produced by the mutation in Step 3, not by absent production code. This step confirms the
asserts can pass at all and that the control asserts really populate their rungs.

Run (ONE foreground call, `timeout: 600000`):

```bash
cd /Users/homer/dev/docket/.worktrees/pin-the-finalize-test-command-auto-sentinel-s-cross-layer-ma && \
  bash tests/test_docket_config.sh > /tmp/0106-base.txt 2>&1; echo "exit=$?"; \
  tail -1 /tmp/0106-base.txt; \
  echo "ok=$(grep -c '^ok - ' /tmp/0106-base.txt) notok=$(grep -c '^NOT OK' /tmp/0106-base.txt)"; \
  grep '0106 s' /tmp/0106-base.txt
```

Expected: `exit=0`, last line `PASS`, `ok=220 notok=0`, and all four `0106 s4`/`0106 s5` lines
present and prefixed `ok - `.

If `ok` is not 220, the fixtures did not all register — do not proceed.

- [ ] **Step 3: Mutation 1 — collapse the sentinel PER-LAYER instead of after the chain**

This is the exact refactor the change exists to prevent. Replace the single post-chain collapse
with per-rung collapses.

Apply by hand to `scripts/docket-config.sh`. Replace line 194 and line 201:

Line 194 currently reads:

```bash
FINALIZE_TEST_COMMAND="$(lcl test_command)"; FINALIZE_TEST_COMMAND="${FINALIZE_TEST_COMMAND:-$(yaml_get "$CFG" test_command)}"; FINALIZE_TEST_COMMAND="${FINALIZE_TEST_COMMAND:-$(gbl test_command)}"
```

Replace it with the per-layer form:

```bash
_m1_l="$(lcl test_command)"; [ "$_m1_l" = auto ] && _m1_l=""
_m1_c="$(yaml_get "$CFG" test_command)"; [ "$_m1_c" = auto ] && _m1_c=""
_m1_g="$(gbl test_command)"; [ "$_m1_g" = auto ] && _m1_g=""
FINALIZE_TEST_COMMAND="$_m1_l"; FINALIZE_TEST_COMMAND="${FINALIZE_TEST_COMMAND:-$_m1_c}"; FINALIZE_TEST_COMMAND="${FINALIZE_TEST_COMMAND:-$_m1_g}"
```

Leave line 201 (`[ "$FINALIZE_TEST_COMMAND" = auto ] && FINALIZE_TEST_COMMAND=""`) in place — it
becomes a harmless no-op, which is precisely what makes this a realistic refactor rather than a
strawman.

- [ ] **Step 4: Run the suite under Mutation 1 and confirm BOTH forward fixtures redden**

Run (ONE foreground call, `timeout: 600000`):

```bash
cd /Users/homer/dev/docket/.worktrees/pin-the-finalize-test-command-auto-sentinel-s-cross-layer-ma && \
  bash tests/test_docket_config.sh > /tmp/0106-mut1.txt 2>&1; echo "exit=$?"; \
  tail -1 /tmp/0106-mut1.txt; \
  echo "ok=$(grep -c '^ok - ' /tmp/0106-mut1.txt) notok=$(grep -c '^NOT OK' /tmp/0106-mut1.txt)"; \
  grep -E '^NOT OK' /tmp/0106-mut1.txt
```

Expected: `exit=1`, last line `FAIL`, `notok=2`, and the two `NOT OK` lines are exactly:

```
NOT OK - 0106 s4: local auto masks committed real command
NOT OK - 0106 s5: committed auto masks global real command
```

The two CONTROL asserts must still be `ok` (the lower rungs still resolve under this mutation);
`ok` drops from 220 to 218. **Record both counts.** If `ok` drops by more than 2 or any assert
outside section S reddens, stop and report — the mutation reached further than intended.

- [ ] **Step 5: Restore `scripts/docket-config.sh` and confirm the restore is byte-clean**

```bash
cd /Users/homer/dev/docket/.worktrees/pin-the-finalize-test-command-auto-sentinel-s-cross-layer-ma && \
  git checkout -- scripts/docket-config.sh && \
  git status --porcelain scripts/docket-config.sh && \
  echo "restored-clean: $(git diff --quiet scripts/docket-config.sh && echo yes || echo NO)"
```

Expected: no output from `git status --porcelain` for that path, and `restored-clean: yes`.

- [ ] **Step 6: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/pin-the-finalize-test-command-auto-sentinel-s-cross-layer-ma && \
  git add tests/test_docket_config.sh && \
  git commit -m "test(0106): pin forward cross-layer masking of the test_command auto sentinel

s4 covers the lcl() path (.docket.local.yml auto over a committed real command); s5 covers
the gbl() path (committed auto over a global real command). Each carries a control assert,
because the masked value is empty — the same thing an absent key yields.

Mutation-verified: collapsing the sentinel per-layer instead of after the :194 chain reddens
both (220 ok -> 218 ok, 2 NOT OK)."
```

Note: `scripts/docket-config.sh` must NOT appear in this commit. Verify with
`git show --stat HEAD` — only `tests/test_docket_config.sh` may be listed.

---

### Task 2: Reverse direction — a LOWER layer's `auto` must not wipe a higher layer's real command (`s6`)

**Files:**
- Modify: `tests/test_docket_config.sh` (append after the `s5` block from Task 1)
- Temporarily mutate then restore: `scripts/docket-config.sh:201`

**Interfaces:**
- Consumes: the same `mkrepo` / `rung` / `assert` helpers and `$tmp` root as Task 1.
- Produces: fixture `$tmp/s6`, XDG root `$tmp/s6.xdg`. Nothing consumes them.

**Why this task exists separately.** `s6` is required by the two-sided-proof rule: a guard that can
fail by being too *loose* and by being too *tight* must be mutation-tested in both directions,
because a one-sided test blesses whichever error it does not probe. Here the too-tight defect is a
*lower* layer's `auto` wrongly wiping out a *higher* layer's real command. **`s6` does not redden
under Mutation 1** — that is stated explicitly rather than left implied, and Step 4 below verifies
it, because a fixture that reddens under no mutation is decoration.

- [ ] **Step 1: Write the failing fixture**

Append to `tests/test_docket_config.sh`, immediately after the `s5` assert added in Task 1:

```bash

# (s6) REVERSE: a LOWER layer's `auto` must NOT wipe a HIGHER layer's real command.
# Required by the two-sided-proof rule: the forward cases above prove the collapse is not too
# LOOSE; this proves it is not too TIGHT. A blanket "any layer says auto => unset" scan would
# pass every forward case and fail only here. s6 deliberately does NOT redden under the
# per-layer mutation that reddens s4/s5 — which is exactly why it needs its own mutation.
mkrepo "$tmp/s6"
mkdir -p "$tmp/s6.xdg/docket"
printf 'finalize:\n  test_command: auto\n' > "$tmp/s6.xdg/docket/config.yml"
cat > "$tmp/s6/.docket.yml" <<'EOF'
metadata_branch: main
integration_branch: main
finalize:
  test_command: make test
EOF
git -C "$tmp/s6" add .docket.yml; git -C "$tmp/s6" commit --quiet -m cfg
git -C "$tmp/s6" push --quiet origin main
FINALIZE_TEST_COMMAND=__poison__
out="$(rung "$tmp/s6.xdg" "$tmp/s6" --export)"; eval "$out"
assert "0106 s6: global auto does NOT wipe committed real command" '[ "$FINALIZE_TEST_COMMAND" = "make test" ]'
```

- [ ] **Step 2: Run the suite to verify `s6` passes against unmodified code**

Run (ONE foreground call, `timeout: 600000`):

```bash
cd /Users/homer/dev/docket/.worktrees/pin-the-finalize-test-command-auto-sentinel-s-cross-layer-ma && \
  bash tests/test_docket_config.sh > /tmp/0106-s6.txt 2>&1; echo "exit=$?"; \
  tail -1 /tmp/0106-s6.txt; \
  echo "ok=$(grep -c '^ok - ' /tmp/0106-s6.txt) notok=$(grep -c '^NOT OK' /tmp/0106-s6.txt)"; \
  grep '0106 s' /tmp/0106-s6.txt
```

Expected: `exit=0`, last line `PASS`, `ok=221 notok=0`, and all five `0106 s*` lines present as
`ok - `. This is the change's final green state.

- [ ] **Step 3: Re-run Mutation 1 with `s6` present — confirm `s6` stays GREEN**

Task 1 ran Mutation 1 before `s6` existed, so nothing has yet demonstrated the spec's explicit
claim that `s6` does not redden under it. That claim is load-bearing: it is the reason `s6` needs
a mutation of its own rather than riding on Task 1's. Verify it now.

Re-apply the Mutation 1 edit from Task 1 Step 3 (replace line 194 with the per-layer form; leave
line 201 in place), then run (ONE foreground call, `timeout: 600000`):

```bash
cd /Users/homer/dev/docket/.worktrees/pin-the-finalize-test-command-auto-sentinel-s-cross-layer-ma && \
  bash tests/test_docket_config.sh > /tmp/0106-mut1b.txt 2>&1; echo "exit=$?"; \
  tail -1 /tmp/0106-mut1b.txt; \
  echo "ok=$(grep -c '^ok - ' /tmp/0106-mut1b.txt) notok=$(grep -c '^NOT OK' /tmp/0106-mut1b.txt)"; \
  grep -E '^NOT OK' /tmp/0106-mut1b.txt; \
  grep '0106 s6' /tmp/0106-mut1b.txt
```

Expected: `exit=1`, `FAIL`, `ok=219 notok=2`, the two `NOT OK` lines are the `s4` and `s5` masking
asserts (unchanged from Task 1), and the `0106 s6` line is `ok - ` — **`s6` does not redden here.**

Then restore before applying Mutation 2:

```bash
cd /Users/homer/dev/docket/.worktrees/pin-the-finalize-test-command-auto-sentinel-s-cross-layer-ma && \
  git checkout -- scripts/docket-config.sh && \
  echo "restored-clean: $(git diff --quiet scripts/docket-config.sh && echo yes || echo NO)"
```

- [ ] **Step 4: Mutation 2 — blanket "any layer says `auto` ⇒ unset"**

Replace the collapse at `scripts/docket-config.sh:201`:

```bash
[ "$FINALIZE_TEST_COMMAND" = auto ] && FINALIZE_TEST_COMMAND=""
```

with a precedence-ignoring scan across all three rungs:

```bash
if [ "$FINALIZE_TEST_COMMAND" = auto ] \
   || [ "$(lcl test_command)" = auto ] \
   || [ "$(yaml_get "$CFG" test_command)" = auto ] \
   || [ "$(gbl test_command)" = auto ]; then FINALIZE_TEST_COMMAND=""; fi
```

Leave line 194 as it is on the unmodified branch.

- [ ] **Step 5: Run the suite under Mutation 2 and confirm ONLY `s6` reddens**

Run (ONE foreground call, `timeout: 600000`):

```bash
cd /Users/homer/dev/docket/.worktrees/pin-the-finalize-test-command-auto-sentinel-s-cross-layer-ma && \
  bash tests/test_docket_config.sh > /tmp/0106-mut2.txt 2>&1; echo "exit=$?"; \
  tail -1 /tmp/0106-mut2.txt; \
  echo "ok=$(grep -c '^ok - ' /tmp/0106-mut2.txt) notok=$(grep -c '^NOT OK' /tmp/0106-mut2.txt)"; \
  grep -E '^NOT OK' /tmp/0106-mut2.txt
```

Expected: `exit=1`, last line `FAIL`, `notok=1`, `ok=220`, and the single `NOT OK` line is exactly:

```
NOT OK - 0106 s6: global auto does NOT wipe committed real command
```

The four `s4`/`s5` asserts must all stay `ok` under this mutation — that asymmetry is the proof
that `s6` guards a property the forward fixtures cannot see. **Record both counts.**

- [ ] **Step 6: Restore `scripts/docket-config.sh` and confirm the restore is byte-clean**

```bash
cd /Users/homer/dev/docket/.worktrees/pin-the-finalize-test-command-auto-sentinel-s-cross-layer-ma && \
  git checkout -- scripts/docket-config.sh && \
  echo "restored-clean: $(git diff --quiet scripts/docket-config.sh && echo yes || echo NO)"
```

Expected: `restored-clean: yes`.

- [ ] **Step 7: Final verification run — green, with the contracted `ok` count**

Run (ONE foreground call, `timeout: 600000`):

```bash
cd /Users/homer/dev/docket/.worktrees/pin-the-finalize-test-command-auto-sentinel-s-cross-layer-ma && \
  git status --porcelain && \
  bash tests/test_docket_config.sh > /tmp/0106-final.txt 2>&1; echo "exit=$?"; \
  tail -1 /tmp/0106-final.txt; \
  echo "ok=$(grep -c '^ok - ' /tmp/0106-final.txt) notok=$(grep -c '^NOT OK' /tmp/0106-final.txt)"
```

Expected: `git status --porcelain` shows only `tests/test_docket_config.sh` as modified (or
nothing, if Task 1 already committed and this is pre-commit for Task 2); `exit=0`; `PASS`;
`ok=221 notok=0`.

- [ ] **Step 8: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/pin-the-finalize-test-command-auto-sentinel-s-cross-layer-ma && \
  git add tests/test_docket_config.sh && \
  git commit -m "test(0106): pin the reverse direction — lower-layer auto must not wipe a higher real command

s6 completes the two-sided proof the forward fixtures cannot give: a blanket 'any layer says
auto => unset' scan passes s4 and s5 and fails only here.

Mutation-verified: the blanket scan reddens s6 alone (221 ok -> 220 ok, 1 NOT OK). s6 does NOT
redden under the per-layer mutation from the previous commit, which is why it needs its own."
```

Note: `scripts/docket-config.sh` must NOT appear in this commit. Verify with `git show --stat HEAD`.

---

## Verification checklist (run before declaring the plan complete)

- [ ] `git log --oneline $(git merge-base origin/main HEAD)..HEAD` shows exactly three commits: the
      `docs(0106):` plan commit plus the two `test(0106):` task commits. (Use the merge-base form —
      `origin/main..HEAD` alone also reports commits that landed on `main` after this branch was cut.)
- [ ] `git diff --stat origin/main..HEAD` lists **only** `tests/test_docket_config.sh` (plus this
      plan file). `scripts/docket-config.sh` must not appear — the change pins behavior, it does
      not alter it.
- [ ] Final suite: `PASS`, `221 ok`, `0 NOT OK`.
- [ ] Both mutation runs were performed against the REAL `scripts/docket-config.sh`, not a fixture
      copy, and both before/after `ok` counts are recorded for the results file.
- [ ] No fixture reads or writes a real `~/.config` — every new `rung` call passes a per-fixture
      `$tmp/s<N>.xdg` root.
