# Complete the `finalize.test_command` cross-layer masking matrix — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add three fixtures (`s7`, `s8`, `s9`) to section S of `tests/test_docket_config.sh` so all six ordered rung pairs of the `finalize.test_command` `auto` sentinel's cross-layer masking are pinned, and retitle the section header to span `S4`–`S9`.

**Architecture:** Test-only. `scripts/docket-config.sh` resolves `finalize.test_command` through a flat three-rung `:-` chain (`:195`) — local `.docket.local.yml` → committed `.docket.yml` → global `$XDG_CONFIG_HOME/docket/config.yml` — then collapses the literal `auto` to `""` **after** the chain (`:202`). That placement means a **higher** rung's `auto` masks a **lower** rung's real command (forward), while a **lower** rung's `auto` must never wipe a **higher** rung's real command (reverse). Change 0106 pinned three of the six ordered pairs (`s4`, `s5`, `s6`); this plan closes the other three. No resolver edit, no doc edit, no ADR.

**Tech Stack:** Bash (`set -uo pipefail`, no `-e`), the suite's own `mkrepo` / `rung` / `assert` helpers, git fixture repos with bare origins. No network.

## Global Constraints

- **Test-only.** No edit to `scripts/docket-config.sh` survives any task. The mutation runs edit it temporarily and **must** revert with `git checkout -- scripts/docket-config.sh` before the task commits. If a mutation run reveals the resolver does not behave as the matrix predicts, that is a genuine defect discovery — **record it and STOP**; it is not a licence to edit the resolver under this change (spec A7).
- **Do not refactor `s4`–`s6`, and do not touch 0106's archived change file or spec** (spec A3, A6).
- **The section-S header's `:202` / `:195` line citations are already correct** — change 0102's commit `43b1aca` re-anchored them after inserting `require_pr_approval` resolution. Retitle the header's *scope*; leave those two numbers alone. Do not "restore" the `:194` / `:201` values that appear in older records.
- **Established per-fixture shape, copied exactly** (spec A3, A5): own `$tmp/<n>` repo via `mkrepo`, own `$tmp/<n>.xdg` global root reached through the `rung` helper, and a literal `FINALIZE_TEST_COMMAND=__poison__` line immediately before **every** `eval`. The poison line is not decoration: an aborted resolver run emits nothing, and a bare `eval ""` would leave the previous fixture's value standing.
- **Hermeticity is load-bearing.** Every fixture roots the global layer at its own `$tmp/<n>.xdg` and reaches it only via `rung`. Never write to, or read from, the developer's real `~/.config/docket/config.yml`. The `.docket.local.yml` files are written into the fixture clone and **never** `git add`ed.
- **Distinct command strings per rung** (spec A5), matching the strings already in use: `make local-test` (local), `make test` (committed), `make global` (global). Distinct strings mean a cross-fixture leak cannot read as a pass.
- **"Committed rung absent" means the KEY is absent** from `.docket.yml`; the file itself still exists and still pins `metadata_branch: main` / `integration_branch: main`, exactly as `s5`'s first phase does. Dropping the file entirely also resolves correctly — it just routes the fixture through `BOOTSTRAP=CREATE_ORPHAN` / docket-mode instead. Keep the file for consistency with the main-mode shape every other section-S fixture uses. A fixture comment must not encode a false reason, so do not write that omitting it would break resolution.
- **Assert-name prefix:** new asserts use the `0112 s<N>:` prefix, matching the existing `0106 s<N>:` convention.
- **Read counts, never exit codes** (learning `agent-shell-noop-reads-as-success`): run the suite under an explicit `bash`, and count with `command grep -c` — the harness shell's `grep` may be shadowed. Zero matches and zero iterations are indistinguishable from success.

### Characterization testing — how "watch it fail" works here

This change pins **existing, already-correct** behavior. There is no red-then-green cycle: a new assert passes the moment it is written. The equivalent of watching a test fail is the **mutation run**, which is what makes these guards code rather than decoration (learning `guards-are-code`). Every task therefore runs this cycle:

1. Write the assert → run the suite → **expect PASS** (proves the assert *can* pass at all — an assert unsatisfiable by any correct implementation reads as a real regression and burns a cycle; learning `plan-supplied-test-code-is-unverified`).
2. Apply the mutation to `scripts/docket-config.sh` → run the suite → **expect the exact predicted RED cells, and no others**.
3. `git checkout -- scripts/docket-config.sh` → run the suite → **expect PASS** again.
4. Commit (test file only).

### The mutation matrix (the completion bar)

| mutation | s4 | s5 | s6 | **s7** | **s8** | **s9** |
|---|---|---|---|---|---|---|
| **M1** collapse `auto` per-layer, before the chain | RED | RED | green | green | green | **RED** |
| **M2** blanket "any rung says `auto` ⇒ unset" | green | green | RED | **RED** | **RED** | green |
| **M3** committed-rung-specific clear | green | green | green | **RED** | green | green |

M1 and M2 are 0106's own two mutations, re-pointed at the new asserts. **M3 is new to this change** and is the one that uniquely isolates `s7`: under M3 every one of 0106's five asserts stays green, which is precisely why `s7` has to exist. `s7` is earned on unique discriminating power; `s8` and `s9` are matrix-completeness witnesses that share existing mutations (spec A2 — stated openly so a reviewer can weigh it). The mirror-image global-rung-specific clear is already recorded against `s6` in 0106's results file and is **not** re-run here.

### `ok` count contract

Read as part of the contract, not merely PASS/FAIL:

| state | total `ok` | `NOT OK` |
|---|---|---|
| baseline (before this change) | 244 | 0 |
| after Task 1 (`s7`, +1 assert) | 245 | 0 |
| after Task 2 (`s8`, +1 assert) | 246 | 0 |
| after Task 3 (`s9`, +2 asserts — control + main) | 248 | 0 |

## File Structure

- **Modify:** `tests/test_docket_config.sh` — the only file this change edits. Section S currently spans `:1029`–`:1105` (header comment at `:1029`, `s4` at `:1043`, `s5` at `:1062`, `s6` at `:1087`, last assert at `:1105`). All three new fixtures append after `s6`'s assert and before the `# ====` banner that opens the change-0102 section.
- **Temporarily modify, always reverted:** `scripts/docket-config.sh` — mutation runs only; never committed.

---

### Task 1: `s7` — the reverse committed-over-local pair, plus the section header

The fixture the change exists for, and the only one M3 reddens. Header retitle folds in here because it describes the section this task completes the core of.

**Files:**
- Modify: `tests/test_docket_config.sh:1029` (header comment) and after `:1105` (new fixture)
- Temporarily modify: `scripts/docket-config.sh:202` (M3, reverted before commit)

**Interfaces:**
- Consumes: the suite's existing `mkrepo <dir>`, `rung <xdgdir> <repodir> [args...]`, and `assert <name> <shell-condition-string>` helpers, all defined at the top of `tests/test_docket_config.sh`. `rung` runs the resolver with `XDG_CONFIG_HOME` pointed at `<xdgdir>`.
- Produces: fixture directories `$tmp/s7` and `$tmp/s7.xdg` — later tasks use `$tmp/s8`, `$tmp/s9` and must not reuse these. The `0112 s<N>:` assert-name prefix established here is used by Tasks 2 and 3.

- [ ] **Step 1: Retitle the section header**

In `tests/test_docket_config.sh`, replace this line (currently `:1029`):

```bash
# --- (S4/S5/S6) change 0106: the sentinel's CROSS-LAYER masking -------------
```

with:

```bash
# --- (S4-S9) changes 0106 + 0112: the sentinel's CROSS-LAYER masking --------
```

Then, immediately after the existing paragraph that ends with the line `# Sections s/s2/s3 above are all single-layer, so none of them can see this. These do.`, insert this new paragraph (leave the existing `#` blank line and the `s4 and s5 assert an EMPTY value...` paragraph below it untouched):

```bash
#
# 0106 pinned three of the six ordered rung pairs; 0112 completes the matrix with s7/s8/s9.
# Writing each pair as (rung holding `auto` -> rung holding the real command):
#   forward (higher `auto` masks lower real):  local->committed s4 | committed->global s5 | local->global s9
#   reverse (lower `auto` must NOT wipe higher real): global->committed s6 | committed->local s7 | global->local s8
# s7 is the one earned on unique discriminating power: a committed-rung-specific clear appended
# after the collapse leaves all five of 0106's asserts green and reddens s7 alone. s8 and s9 are
# matrix-completeness witnesses that share s6's and s4/s5's mutations respectively -- no claim of
# a unique witness is made for them.
```

- [ ] **Step 2: Write the `s7` fixture**

Append immediately after `s6`'s assert (currently `:1105`, the line beginning `assert "0106 s6:`) and before the `# ====` banner that opens the change-0102 section:

```bash

# (s7) REVERSE, lcl() path: a committed `auto` must NOT wipe a LOCAL real command.
# The dangerous cell, and the reason this change exists. A real repo whose .docket.local.yml
# sets a command over a committed `test_command: auto` must keep its local command; the
# committed-rung-specific clear that would silently drop it passes every 0106 assert.
mkrepo "$tmp/s7"
mkdir -p "$tmp/s7.xdg/docket"
cat > "$tmp/s7/.docket.yml" <<'EOF'
metadata_branch: main
integration_branch: main
finalize:
  test_command: auto
EOF
git -C "$tmp/s7" add .docket.yml; git -C "$tmp/s7" commit --quiet -m cfg
git -C "$tmp/s7" push --quiet origin main
printf 'finalize:\n  test_command: make local-test\n' > "$tmp/s7/.docket.local.yml"
FINALIZE_TEST_COMMAND=__poison__
out="$(rung "$tmp/s7.xdg" "$tmp/s7" --export)"; eval "$out"
assert "0112 s7: committed auto does NOT wipe local real command" '[ "$FINALIZE_TEST_COMMAND" = "make local-test" ]'
```

Note there is no control assert: `s7` asserts a distinctive non-empty string, which no misconfiguration produces by accident. Controls exist only where the expectation is empty (spec A4) — that is why `s4`/`s5` have them and `s6` does not.

- [ ] **Step 3: Run the suite — prove the assert CAN pass**

```bash
cd /Users/homer/dev/docket/.worktrees/pin-the-reverse-cross-layer-masking-for-the-committed-over-l
out="$(bash tests/test_docket_config.sh 2>&1)"
echo "$out" | tail -1
echo "ok=$(echo "$out" | command grep -c '^ok - ')  notok=$(echo "$out" | command grep -c '^NOT OK - ')"
echo "$out" | command grep '0112 s7'
```

Expected: `PASS`, `ok=245  notok=0`, and the line `ok - 0112 s7: committed auto does NOT wipe local real command`.

If `s7` is `NOT OK` here, the resolver does not behave as the matrix predicts — that is a genuine defect discovery. Record it and STOP (Global Constraints).

- [ ] **Step 4: Apply mutation M3 — the committed-rung-specific clear**

In `scripts/docket-config.sh`, find this line (currently `:202`):

```bash
[ "$FINALIZE_TEST_COMMAND" = auto ] && FINALIZE_TEST_COMMAND=""
```

and append a second line directly beneath it, so the pair reads:

```bash
[ "$FINALIZE_TEST_COMMAND" = auto ] && FINALIZE_TEST_COMMAND=""
[ "$(yaml_get "$CFG" test_command)" = auto ] && FINALIZE_TEST_COMMAND=""
```

`$CFG` is the committed-config path and is in scope here; `yaml_get <file> <key>` echoes the value or empty when absent. The script runs `set -uo pipefail` with **no** `-e`, so a trailing `[ ... ] && VAR=""` whose test is false is safe.

- [ ] **Step 5: Run the suite under M3 — expect `s7` RED and nothing else**

```bash
out="$(bash tests/test_docket_config.sh 2>&1)"
echo "$out" | tail -1
echo "ok=$(echo "$out" | command grep -c '^ok - ')  notok=$(echo "$out" | command grep -c '^NOT OK - ')"
echo "$out" | command grep '^NOT OK - '
```

Expected: `FAIL`, `ok=244  notok=1`, and the single failing line:

```
NOT OK - 0112 s7: committed auto does NOT wipe local real command
```

This is the load-bearing result of the whole change. All five `0106 s4`/`s5`/`s6` asserts must remain `ok` — if any of them also reddens, M3 was mis-applied (or the fixture is not isolating what it claims) and the discrepancy must be resolved before continuing.

- [ ] **Step 6: Revert the mutation and confirm green**

```bash
git checkout -- scripts/docket-config.sh
git status --porcelain scripts/docket-config.sh
out="$(bash tests/test_docket_config.sh 2>&1)"
echo "$out" | tail -1
echo "ok=$(echo "$out" | command grep -c '^ok - ')  notok=$(echo "$out" | command grep -c '^NOT OK - ')"
```

Expected: `git status --porcelain` prints **nothing** for the resolver (clean), then `PASS`, `ok=245  notok=0`.

- [ ] **Step 7: Commit**

```bash
git add tests/test_docket_config.sh
git commit -m "test(0112): pin the reverse committed-over-local rung pair (s7)

The dangerous unpinned cell: a committed \`test_command: auto\` must not wipe
a local real command. Witnessed by a committed-rung-specific clear appended
after the collapse, which leaves all five of 0106's asserts green and reddens
s7 alone. Section header retitled to span S4-S9."
```

Verify the resolver is not in the commit:

```bash
git show --stat --name-only HEAD
```

Expected: `tests/test_docket_config.sh` is the **only** file listed.

---

### Task 2: `s8` — the reverse skip-rung pair (global `auto` under a local real command)

**Files:**
- Modify: `tests/test_docket_config.sh` (append after `s7`'s assert)
- Temporarily modify: `scripts/docket-config.sh:202` (M2, reverted before commit)

**Interfaces:**
- Consumes: `mkrepo`, `rung`, `assert` (as Task 1); the `0112 s<N>:` assert-name prefix from Task 1.
- Produces: fixture directories `$tmp/s8` and `$tmp/s8.xdg`. Establishes the "committed key absent" fixture shape — `.docket.yml` present with only `metadata_branch` / `integration_branch` — which Task 3 reuses.

- [ ] **Step 1: Write the `s8` fixture**

Append immediately after `s7`'s assert:

```bash

# (s8) REVERSE, skip-rung: a global `auto` must NOT wipe a LOCAL real command.
# Committed rung leaves the KEY absent -- .docket.yml still exists and still pins main-mode,
# matching s5's first phase. (Dropping the file entirely also resolves correctly; it would just
# route this fixture through BOOTSTRAP=CREATE_ORPHAN, which is why the file is kept.)
mkrepo "$tmp/s8"
mkdir -p "$tmp/s8.xdg/docket"
printf 'finalize:\n  test_command: auto\n' > "$tmp/s8.xdg/docket/config.yml"
cat > "$tmp/s8/.docket.yml" <<'EOF'
metadata_branch: main
integration_branch: main
EOF
git -C "$tmp/s8" add .docket.yml; git -C "$tmp/s8" commit --quiet -m cfg
git -C "$tmp/s8" push --quiet origin main
printf 'finalize:\n  test_command: make local-test\n' > "$tmp/s8/.docket.local.yml"
FINALIZE_TEST_COMMAND=__poison__
out="$(rung "$tmp/s8.xdg" "$tmp/s8" --export)"; eval "$out"
assert "0112 s8: global auto does NOT wipe local real command (committed key absent)" '[ "$FINALIZE_TEST_COMMAND" = "make local-test" ]'
```

- [ ] **Step 2: Run the suite — prove the assert CAN pass**

```bash
out="$(bash tests/test_docket_config.sh 2>&1)"
echo "$out" | tail -1
echo "ok=$(echo "$out" | command grep -c '^ok - ')  notok=$(echo "$out" | command grep -c '^NOT OK - ')"
echo "$out" | command grep '0112 s8'
```

Expected: `PASS`, `ok=246  notok=0`, and `ok - 0112 s8: global auto does NOT wipe local real command (committed key absent)`.

- [ ] **Step 3: Apply mutation M2 — the blanket any-rung scan**

In `scripts/docket-config.sh`, find this line (currently `:202`):

```bash
[ "$FINALIZE_TEST_COMMAND" = auto ] && FINALIZE_TEST_COMMAND=""
```

and append a second line directly beneath it, so the pair reads:

```bash
[ "$FINALIZE_TEST_COMMAND" = auto ] && FINALIZE_TEST_COMMAND=""
if [ "$(lcl test_command)" = auto ] || [ "$(yaml_get "$CFG" test_command)" = auto ] || [ "$(gbl test_command)" = auto ]; then FINALIZE_TEST_COMMAND=""; fi
```

This models the plausible refactor "if any layer says `auto`, treat the key as unset" — too tight in exactly the direction the reverse cases exist to catch.

- [ ] **Step 4: Run the suite under M2 — expect `s6`, `s7`, `s8` RED**

```bash
out="$(bash tests/test_docket_config.sh 2>&1)"
echo "$out" | tail -1
echo "ok=$(echo "$out" | command grep -c '^ok - ')  notok=$(echo "$out" | command grep -c '^NOT OK - ')"
echo "$out" | command grep '^NOT OK - '
```

Expected: `FAIL`, `ok=243  notok=3`, and exactly these three failing lines (order as emitted):

```
NOT OK - 0106 s6: global auto does NOT wipe committed real command
NOT OK - 0112 s7: committed auto does NOT wipe local real command
NOT OK - 0112 s8: global auto does NOT wipe local real command (committed key absent)
```

Both forward asserts (`s4`, `s5`) and both control asserts must stay `ok` — M2 is a reverse-direction mutation. `s8` reddening here is its witness; that it shares the mutation with `s6` and `s7` is accepted openly (spec A2).

- [ ] **Step 5: Revert the mutation and confirm green**

```bash
git checkout -- scripts/docket-config.sh
git status --porcelain scripts/docket-config.sh
out="$(bash tests/test_docket_config.sh 2>&1)"
echo "$out" | tail -1
echo "ok=$(echo "$out" | command grep -c '^ok - ')  notok=$(echo "$out" | command grep -c '^NOT OK - ')"
```

Expected: nothing from `git status --porcelain`, then `PASS`, `ok=246  notok=0`.

- [ ] **Step 6: Commit**

```bash
git add tests/test_docket_config.sh
git commit -m "test(0112): pin the reverse skip-rung pair (s8)

Global \`auto\` must not wipe a local real command with the committed key
absent. Witnessed by the blanket any-rung scan, shared with s6 and s7."
```

---

### Task 3: `s9` — the forward skip-rung pair, and the full-matrix verification

Closes the last cell, then re-runs all three mutations against the completed six-fixture matrix so every predicted cell is confirmed in one pass.

**Files:**
- Modify: `tests/test_docket_config.sh` (append after `s8`'s assert)
- Temporarily modify: `scripts/docket-config.sh:195` and `:202` (M1, M2, M3 — each reverted)

**Interfaces:**
- Consumes: `mkrepo`, `rung`, `assert`; the "committed key absent" fixture shape from Task 2.
- Produces: fixture directories `$tmp/s9` and `$tmp/s9.xdg`. Final state of section S — six fixtures, nine asserts, 248 suite-wide `ok`.

- [ ] **Step 1: Write the `s9` fixture**

Append immediately after `s8`'s assert:

```bash

# (s9) FORWARD, skip-rung: a local `auto` masks a GLOBAL real command, committed key absent.
# Expects an EMPTY value, which is also what an absent key yields -- so it carries a control
# assert first, the same reason s4 and s5 do.
mkrepo "$tmp/s9"
mkdir -p "$tmp/s9.xdg/docket"
printf 'finalize:\n  test_command: make global\n' > "$tmp/s9.xdg/docket/config.yml"
cat > "$tmp/s9/.docket.yml" <<'EOF'
metadata_branch: main
integration_branch: main
EOF
git -C "$tmp/s9" add .docket.yml; git -C "$tmp/s9" commit --quiet -m cfg
git -C "$tmp/s9" push --quiet origin main
FINALIZE_TEST_COMMAND=__poison__
out="$(rung "$tmp/s9.xdg" "$tmp/s9" --export)"; eval "$out"
assert "0112 s9 control: global real command resolves before masking" '[ "$FINALIZE_TEST_COMMAND" = "make global" ]'
printf 'finalize:\n  test_command: auto\n' > "$tmp/s9/.docket.local.yml"
FINALIZE_TEST_COMMAND=__poison__
out="$(rung "$tmp/s9.xdg" "$tmp/s9" --export)"; eval "$out"
assert "0112 s9: local auto masks global real command (committed key absent)" '[ -z "$FINALIZE_TEST_COMMAND" ]'
```

- [ ] **Step 2: Run the suite — prove both asserts CAN pass**

```bash
out="$(bash tests/test_docket_config.sh 2>&1)"
echo "$out" | tail -1
echo "ok=$(echo "$out" | command grep -c '^ok - ')  notok=$(echo "$out" | command grep -c '^NOT OK - ')"
echo "$out" | command grep '0112 s9'
```

Expected: `PASS`, `ok=248  notok=0`, and both lines:

```
ok - 0112 s9 control: global real command resolves before masking
ok - 0112 s9: local auto masks global real command (committed key absent)
```

The control passing is what makes the empty-expecting assert meaningful — it proves the global command really does resolve through the skip-rung path before the masking layer is added, so the empty result below cannot be passing merely because the key was never readable.

- [ ] **Step 3: Apply mutation M1 — per-layer collapse, before the chain**

In `scripts/docket-config.sh`, find this line (currently `:195`):

```bash
FINALIZE_TEST_COMMAND="$(lcl test_command)"; FINALIZE_TEST_COMMAND="${FINALIZE_TEST_COMMAND:-$(yaml_get "$CFG" test_command)}"; FINALIZE_TEST_COMMAND="${FINALIZE_TEST_COMMAND:-$(gbl test_command)}"
```

and replace it with these two lines:

```bash
_m1(){ [ "$1" = auto ] || printf '%s' "$1"; }
FINALIZE_TEST_COMMAND="$(_m1 "$(lcl test_command)")"; FINALIZE_TEST_COMMAND="${FINALIZE_TEST_COMMAND:-$(_m1 "$(yaml_get "$CFG" test_command)")}"; FINALIZE_TEST_COMMAND="${FINALIZE_TEST_COMMAND:-$(_m1 "$(gbl test_command)")}"
```

`_m1` echoes its argument unless it is exactly `auto`, in which case it echoes nothing — collapsing the sentinel **per layer**, before the `:-` chain, instead of after it. This is the refactor that silently **inverts** the masking: the higher `auto` becomes empty, the chain falls through, and the lower command resurfaces.

- [ ] **Step 4: Run the suite under M1 — expect `s4`, `s5`, `s9` RED**

```bash
out="$(bash tests/test_docket_config.sh 2>&1)"
echo "$out" | tail -1
echo "ok=$(echo "$out" | command grep -c '^ok - ')  notok=$(echo "$out" | command grep -c '^NOT OK - ')"
echo "$out" | command grep '^NOT OK - '
```

Expected: `FAIL`, `ok=245  notok=3`, and exactly these three failing lines:

```
NOT OK - 0106 s4: local auto masks committed real command
NOT OK - 0106 s5: committed auto masks global real command
NOT OK - 0112 s9: local auto masks global real command (committed key absent)
```

All three **reverse** asserts (`s6`, `s7`, `s8`) and all three controls must stay `ok` — M1 is a forward-direction mutation. `s9` reddening here is its witness.

- [ ] **Step 5: Revert M1, then re-run M2 and M3 against the completed matrix**

```bash
git checkout -- scripts/docket-config.sh
```

Re-apply **M2** exactly as in Task 2 Step 3, run the suite, and confirm:

Expected: `FAIL`, `ok=245  notok=3` — failing lines `0106 s6`, `0112 s7`, `0112 s8`. (Task 2 saw `ok=243` for the same mutation because `s9`'s two asserts did not exist yet; both stay green under M2, so the count rises by two.)

```bash
git checkout -- scripts/docket-config.sh
```

Re-apply **M3** exactly as in Task 1 Step 4, run the suite, and confirm:

Expected: `FAIL`, `ok=247  notok=1` — the single failing line `0112 s7`. Every one of 0106's five asserts, plus `s8` and both `s9` asserts, stays green. This is the result that justifies `s7`'s existence.

```bash
git checkout -- scripts/docket-config.sh
```

- [ ] **Step 6: Confirm the resolver is pristine and the suite is green**

```bash
git status --porcelain scripts/docket-config.sh
git diff --stat origin/main -- scripts/docket-config.sh
out="$(bash tests/test_docket_config.sh 2>&1)"
echo "$out" | tail -1
echo "ok=$(echo "$out" | command grep -c '^ok - ')  notok=$(echo "$out" | command grep -c '^NOT OK - ')"
```

Expected: **no output** from either git command (the resolver is byte-identical to `origin/main`), then `PASS`, `ok=248  notok=0`.

The `git diff --stat origin/main` check is the one that matters: it asserts on the *effect* — that no mutation survived anywhere on the branch — rather than on a revert command having exited 0.

- [ ] **Step 7: Run the full repo suite**

This repo has **no top-level test runner** — the suite shape is hermetic per-file `tests/test_*.sh`. Use the repo's established idiom, wrapped in an explicit `bash -c` because the harness's interactive shell is not bash and an unquoted glob loop can silently iterate zero times (learning `agent-shell-noop-reads-as-success`):

```bash
cd /Users/homer/dev/docket/.worktrees/pin-the-reverse-cross-layer-masking-for-the-committed-over-l
bash -c 'fail=0; n=0; for t in tests/test_*.sh; do n=$((n+1)); bash "$t" >"/tmp/$(basename "$t").out" 2>&1 || { echo "FAIL: $t"; fail=1; }; done; echo "suite ran=$n fail=$fail"'
```

Expected: no `FAIL:` lines, and a final `suite ran=<N> fail=0` where **N is at least 50** (52 files as of change 0111). The `ran=` count is the point — a `fail=0` after zero iterations is indistinguishable from success, so read the count, not just the flag.

Run this as **one foreground call** and allow up to 10 minutes: `tests/test_sync_agents.sh` alone takes several minutes of sandboxed generation runs. Do not background it.

- [ ] **Step 8: Commit**

```bash
git add tests/test_docket_config.sh
git commit -m "test(0112): pin the forward skip-rung pair (s9); matrix complete

Local \`auto\` masks a global real command with the committed key absent,
with a control assert since the expectation is empty. All six ordered rung
pairs are now pinned; M1/M2/M3 re-run against the completed matrix and every
predicted cell confirmed."
```

---

## Self-Review

**1. Spec coverage.** Every section of the spec maps to a task:

| spec element | task |
|---|---|
| `s7` — reverse, committed `auto` under local real | Task 1 Step 2 |
| `s8` — reverse, skip-rung, global `auto` under local real | Task 2 Step 1 |
| `s9` — forward, skip-rung, local `auto` over global real, with control | Task 3 Step 1 |
| Section header retitled to span `S4`–`S9`, crediting `s7` alone (A6) | Task 1 Step 1 |
| M3 (new; isolates `s7`) | Task 1 Steps 4–6, re-run Task 3 Step 5 |
| M2 (0106's, re-pointed) | Task 2 Steps 3–5, re-run Task 3 Step 5 |
| M1 (0106's, re-pointed) | Task 3 Steps 3–5 |
| `ok` count read as contract (A-level `guards-are-code`) | every run step; table in Global Constraints |
| Established per-fixture shape, no `s4`–`s6` refactor (A3) | Global Constraints; fixtures copy the shape |
| Control asserts only where the expectation is empty (A4) | `s9` only; noted in Task 1 Step 2 |
| Distinct per-rung strings, hermetic `.xdg` roots (A5) | Global Constraints; every fixture |
| No resolver / doc / ADR change (A7) | Global Constraints; verified Task 3 Step 6 |
| 0106's archived record untouched (A6) | Global Constraints |

No spec requirement is unassigned. The `depends_on` / A8 dependency check was discharged at reconcile — the anchor drift it predicted materialized (0102 shifted `:194`→`:195`, `:201`→`:202`) and is already absorbed into the spec and this plan.

**2. Placeholder scan.** No `TBD`, no "add appropriate error handling", no "similar to Task N" — the `s8` and `s9` fixtures are written out in full rather than referring back to `s7`, and both mutation re-runs in Task 3 Step 5 name the task and step whose text to re-apply while stating their own distinct expected counts. Every code step carries its actual code; every run step carries its exact command and expected output.

**3. Type consistency.** Helper signatures are used identically across all three tasks: `mkrepo <dir>`, `rung <xdgdir> <repodir> --export`, `assert <name> <condition-string>`. Fixture directory names (`$tmp/s7`, `$tmp/s8`, `$tmp/s9`) and their `.xdg` siblings are distinct per task and never reused. Assert names are stable strings, quoted identically in each task's expected-output block and in the mutation tables. The three rung command strings (`make local-test`, `make test`, `make global`) match the spec and the existing `s4`–`s6` fixtures.

One inconsistency found and fixed during this review: the M2 expected `ok` count differs between Task 2 (`ok=243`) and its Task 3 re-run (`ok=245`), because `s9`'s two asserts exist by then and both stay green under M2. Both counts are now stated explicitly at their own step, with the reason, so an implementer reading Task 3 out of order does not read the difference as a regression.
