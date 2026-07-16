# terminal_publish Opt-In Default Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Flip docket's `terminal_publish` default from `true` to `false` at every layer that encodes it, so writing machine commits onto a repo's integration branch becomes a conscious per-repo opt-in instead of a fail-open default.

**Architecture:** Three executable sites encode the default today (`docket-config.sh:199`, `terminal-publish.sh:30`, `docket-status.sh:389`) and a fourth asserts it in tests (`test_docket_status.sh` Case B). Each fallback flips fail-safe. `terminal-publish.sh` additionally gains an *unset sentinel*: an omitted `--enabled` becomes a **loud** no-op (stderr `WARNING`, exit 0) rather than a silent publish, because a caller that forgot the flag is a bug — while an explicit `--enabled false` stays a silent, intentional no-op. Docs invert from "default true / opt out" to "opt-in, with a risks callout". This repo pins `terminal_publish: true` so its own archive-parity practice is unchanged, now explicit.

**Tech Stack:** POSIX-ish bash (`set -uo pipefail`), hand-rolled `assert` test suites under `tests/*.sh`, markdown docs + skill files.

## Global Constraints

- **The default-encoding site inventory is EXACTLY four, derived by whole-repo grep at reconcile — do not hand-extend it:** `scripts/docket-config.sh:199` · `scripts/terminal-publish.sh:30` · `scripts/docket-status.sh:389` · `tests/test_docket_status.sh` Case B (~L919-950). Change 0064 shipped having hand-listed *prose* sites and missed the one executable sweep site; if you believe you found a fifth, re-derive by grep rather than assuming.
- **Never replace a `${TERMINAL_PUBLISH:-…}` expansion with a bare `$TERMINAL_PUBLISH`.** The `:-` fallback is also the `set -u` unbound-variable guard for a stale/mocked config export that omits the key. Only the fallback *value* flips.
- **Historical artifacts stay untouched:** `docs/changes/archive/**`, `docs/results/**`, `docs/superpowers/plans/**`, and old specs keep their original wording. ADR-0027's text stays as written (it stays `Accepted` — it decided the fence and gate location, not the default value).
- **`--enabled` semantics after this change:** `true` ⇒ publish · `false` ⇒ silent no-op, exit 0 · **omitted** ⇒ no-op, exit 0, **prominent stderr `WARNING`** · anything else ⇒ die (fail closed, unchanged).
- **Exit 0 on the omitted-flag path is deliberate:** skill callers trust the exit code and an omitted flag must never abort a close-out. The warning — not a non-zero exit — is what makes the skipped publish impossible to miss.
- **Do not create the ADR.** The change's ADR is recorded by the `docket-adr` subagent at step 6 of `docket-implement-next`, outside this plan.
- **Do not touch the change file or the spec** (`docs/changes/**`, `docs/superpowers/specs/**` on the metadata branch). The feature branch carries only plan + code + results.
- Test idiom: `assert "<name>" '<shell expr>'`; suites print `PASS`/`FAIL` and exit non-zero on failure. Match the surrounding house style in each file.

---

### Task 1: Config resolver default flips to `false`

**Files:**
- Modify: `scripts/docket-config.sh:199`
- Test: `tests/test_docket_config.sh:53` (absent-config default) and `tests/test_docket_config.sh:575-604` (the two coordination-key fence blocks)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `TERMINAL_PUBLISH` resolves to `false` when `.docket.yml` omits `terminal_publish`. Tasks 2 and 3 mirror this same fallback value in their own scripts; nothing imports a symbol from here.

**Context you need:** `docket-config.sh` reads `.docket.yml` authoritatively via `git show origin/HEAD:.docket.yml` (line 129), not from the working tree. `terminal_publish` is a *coordination key*: it is read from the repo-committed `.docket.yml` **only** (no `lcl`/`gbl` rungs), and a value set in `~/.config/docket/config.yml` or `.docket.local.yml` is warned-and-ignored by the Stage 2c fence.

**Why the fence fixtures must invert (do not skip this):** both fence blocks currently write `terminal_publish: false` into a machine-scoped layer and assert the resolved value "stays true" — i.e. they prove the fence worked by showing the ignored value did *not* win. Once the default is `false`, the ignored value and the default **coincide**, so asserting `= false` would pass whether or not the fence works: a vacuous test. Inverting the probe to `true` restores the assertion's power — if the fence ever regresses, `true` wins and the test goes red.

- [ ] **Step 1: Update the absent-config default assertion (it should now fail)**

In `tests/test_docket_config.sh`, replace line 53:

```bash
assert "absent cfg: TERMINAL_PUBLISH default true"     '[ "$TERMINAL_PUBLISH" = true ]'
```

with:

```bash
assert "absent cfg: TERMINAL_PUBLISH default false"    '[ "$TERMINAL_PUBLISH" = false ]'
```

- [ ] **Step 2: Invert the GLOBAL fence block's probe value and assertion**

In `tests/test_docket_config.sh`, replace the global fence block (lines 575-590) with:

```bash
# fence: a GLOBAL terminal_publish is warned-and-ignored, never honored, never fatal
# change 0084: the probe value is `true` — the NON-default. With the default at `false`, probing
# with `false` would make the assertion vacuous (the ignored value and the default coincide, so it
# would pass even if the fence honored the value). Probing with `true` keeps it discriminating: if
# the fence ever regresses, `true` wins and this goes red.
mkrepo "$tmp/tp3"
mkdir -p "$tmp/tp3.xdg/docket"
printf 'terminal_publish: true\n' > "$tmp/tp3.xdg/docket/config.yml"
tperr="$(rung "$tmp/tp3.xdg" "$tmp/tp3" --export 2>&1 >/dev/null)"
# Unset before the eval that follows: a run that ABORTS emits nothing, so eval "" is a no-op —
# without unsetting first, TERMINAL_PUBLISH would silently keep its value from an earlier block
# in this file and the "stays false" assert below would pass vacuously on stale state instead of
# on this run's actual (non-)output. `${TERMINAL_PUBLISH-unset}` below reads it back safely under
# `set -u` whether the eval set it or left it unset.
unset TERMINAL_PUBLISH
out="$(rung "$tmp/tp3.xdg" "$tmp/tp3" --export 2>/dev/null)"; eval "$out"
assert "0064 fence: global terminal_publish warns"        'printf "%s" "$tperr" | grep -q "terminal_publish"'
assert "0064 fence: warning says per-repo-only"           'printf "%s" "$tperr" | grep -qi "per-repo-only"'
assert "0064 fence: global value NOT honored (stays false)" '[ "${TERMINAL_PUBLISH-unset}" = false ]'
assert "0064 fence: global terminal_publish is not fatal"  '[ "$(rung_rc "$tmp/tp3.xdg" "$tmp/tp3" --export)" -eq 0 ]'
```

- [ ] **Step 3: Invert the MACHINE-LOCAL fence block's probe value and assertion**

In `tests/test_docket_config.sh`, replace the `.docket.local.yml` fence block (lines 592-604) with:

```bash
# fence: a MACHINE-LOCAL .docket.local.yml terminal_publish is warned-and-ignored too
# change 0084: probes with `true` (the non-default) for the same reason as the global block above.
mkrepo "$tmp/tp4"
printf 'terminal_publish: true\n' > "$tmp/tp4/.docket.local.yml"
lerr="$(run "$tmp/tp4" --export 2>&1 >/dev/null)"; rc=$?
# Same stale-value hazard as the global block above — unset before the eval, and read back via
# the safe default-expansion so an abort (empty eval) is caught as "unset", not misread as a
# leftover "false" from an earlier block.
unset TERMINAL_PUBLISH
out="$(run "$tmp/tp4" --export 2>/dev/null)"; eval "$out"
assert "0064 fence: .docket.local.yml terminal_publish warns" 'printf "%s" "$lerr" | grep -q "terminal_publish"'
assert "0064 fence: local names .docket.local.yml"            'printf "%s" "$lerr" | grep -q ".docket.local.yml"'
assert "0064 fence: local value NOT honored (stays false)"     '[ "${TERMINAL_PUBLISH-unset}" = false ]'
assert "0064 fence: local terminal_publish is not fatal (rc=0)" '[ "$rc" -eq 0 ]'
```

- [ ] **Step 4: Run the tests to verify they fail**

Run: `bash tests/test_docket_config.sh 2>&1 | grep -E "NOT OK|^FAIL|^PASS"`

Expected: FAIL, with `NOT OK` lines for `absent cfg: TERMINAL_PUBLISH default false`, `0064 fence: global value NOT honored (stays false)`, and `0064 fence: local value NOT honored (stays false)` — each still resolving `true` because the resolver has not flipped yet.

- [ ] **Step 5: Flip the resolver's fallback**

In `scripts/docket-config.sh`, replace lines 196-199:

```bash
# change 0064: coordination-key fenced — repo-committed .docket.yml ONLY (no lcl/gbl rungs; a
# machine-scoped value is warned-and-ignored by the Stage 2c fence above). Fail closed on garbage:
# silently defaulting a typo to `true` would publish onto the integration branch against intent.
TERMINAL_PUBLISH="$(yaml_get "$CFG" terminal_publish)"; TERMINAL_PUBLISH="${TERMINAL_PUBLISH:-true}"
```

with:

```bash
# change 0064: coordination-key fenced — repo-committed .docket.yml ONLY (no lcl/gbl rungs; a
# machine-scoped value is warned-and-ignored by the Stage 2c fence above). Fail closed on garbage:
# silently defaulting a typo to `true` would publish onto the integration branch against intent.
# change 0084: the default is `false` — publishing onto the integration branch is opt-in. A repo
# that never set the key must never get direct machine commits on its code line.
TERMINAL_PUBLISH="$(yaml_get "$CFG" terminal_publish)"; TERMINAL_PUBLISH="${TERMINAL_PUBLISH:-false}"
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `bash tests/test_docket_config.sh 2>&1 | tail -3`

Expected: `PASS` (no `NOT OK` lines). The explicit-value round-trips at lines 559-573 (`repo terminal_publish false is honored`, `repo terminal_publish true is honored`) and the fail-closed garbage case must stay green untouched.

- [ ] **Step 7: Commit**

```bash
git add scripts/docket-config.sh tests/test_docket_config.sh
git commit -m "feat(0084): config resolver defaults terminal_publish to false

Publishing onto the integration branch becomes opt-in. The fence fixtures
invert their probe to `true` (the non-default): with the default at false, an
ignored machine-scoped `false` is indistinguishable from the default, so the
assertion would pass vacuously whether or not the fence works."
```

---

### Task 2: `terminal-publish.sh` — unset sentinel + loud omitted-flag no-op

**Files:**
- Modify: `scripts/terminal-publish.sh` (usage header lines 18-21; `ENABLED` init line 30; `--enabled` parse line 46; validation lines 65-67)
- Test: `tests/test_terminal_publish.sh`

**Interfaces:**
- Consumes: nothing from Task 1 (this script never reads `TERMINAL_PUBLISH` itself — callers pass `--enabled`).
- Produces: the `--enabled` contract every caller relies on — `true` ⇒ publish · `false` ⇒ silent no-op exit 0 · omitted ⇒ no-op exit 0 + stderr `WARNING` · other ⇒ die. Task 3's `docket-status.sh` call site passes `--enabled "${TERMINAL_PUBLISH:-false}"`, so it always passes the flag explicitly and never relies on the omitted path.

**Context you need:** the script's guards run in this order — arg validation (lines 54-67) → mode guard (`META_BRANCH == INT_BRANCH` ⇒ main-mode no-op, line 70) → knob guard (`ENABLED = false` ⇒ no-op, line 78) → `git fetch` (line 84). **Everything through the knob guard happens before any git work**, which is why the tests below need no repo fixture. `log()` already prefixes each line with `terminal-publish: ` and writes to stderr.

**Why a separate `ENABLED_PASSED` flag rather than an empty-string sentinel:** an empty `ENABLED=""` would make an explicit `--enabled ""` indistinguishable from an omitted flag, silently downgrading a garbage value from `die` to a warn-and-continue. Tracking "was the flag passed" separately keeps `--enabled ""` failing closed exactly as today.

- [ ] **Step 1: Write the failing tests**

In `tests/test_terminal_publish.sh`, insert the following after the existing `--id 5 passes the int guard` assertion and before the closing `if [ "$fail" = 0 ]` line:

```bash
# --- change 0084: the --enabled contract ------------------------------------------------------
# Publish is opt-in. An OMITTED --enabled is a caller bug rather than a decision, so it no-ops
# LOUDLY; an explicit `--enabled false` is a decision, so it stays silent. Exit 0 on both paths:
# callers trust the exit code and a missing flag must never abort a close-out — the WARNING, not a
# non-zero exit, is what keeps a skipped publish from hiding (the #0043 silent-gap failure mode).
# The arg/mode/knob guards all run before any git work, so these need no repo fixture.
pub_args=(--id 5 --outcome done --integration-branch main --metadata-branch docket
          --changes-dir docs/changes --adrs-dir docs/adrs)

err="$(bash "$SCRIPT" "${pub_args[@]}" 2>&1)"; rc=$?
assert "omitted --enabled exits zero (never aborts a close-out)" '[ "$rc" -eq 0 ]'
assert "omitted --enabled warns on stderr"                       'printf "%s" "$err" | grep -q "WARNING"'
assert "omitted --enabled says NOTHING was published"            'printf "%s" "$err" | grep -qi "nothing was published"'
assert "omitted --enabled names the fix (--enabled true)"        'printf "%s" "$err" | grep -q -- "--enabled true"'

err="$(bash "$SCRIPT" "${pub_args[@]}" --enabled false 2>&1)"; rc=$?
assert "explicit --enabled false exits zero"                     '[ "$rc" -eq 0 ]'
assert "explicit --enabled false is SILENT (no WARNING)"          '! printf "%s" "$err" | grep -q "WARNING"'
assert "explicit --enabled false logs the suppression"           'printf "%s" "$err" | grep -q "terminal_publish: false"'

err="$(bash "$SCRIPT" "${pub_args[@]}" --enabled maybe 2>&1)"; rc=$?
assert "invalid --enabled exits non-zero"                        '[ "$rc" -ne 0 ]'
assert "invalid --enabled diagnostic names the value"            'printf "%s" "$err" | grep -q "maybe"'

# an explicit EMPTY value stays fail-closed — it must not be mistaken for an omitted flag
err="$(bash "$SCRIPT" "${pub_args[@]}" --enabled "" 2>&1)"; rc=$?
assert "empty --enabled exits non-zero (not treated as omitted)" '[ "$rc" -ne 0 ]'
assert "empty --enabled does NOT warn"                           '! printf "%s" "$err" | grep -q "WARNING"'
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_terminal_publish.sh 2>&1 | grep -E "NOT OK|^PASS|^FAIL"`

Expected: FAIL. `omitted --enabled warns on stderr` fails (today an omitted flag defaults to `true` and proceeds to `git fetch` — no `WARNING`), and `empty --enabled does NOT warn` may pass only incidentally. `omitted --enabled exits zero` will also fail — today the omitted path reaches the fetch and dies.

- [ ] **Step 3: Replace the `ENABLED` initialiser with an unset sentinel**

In `scripts/terminal-publish.sh`, replace line 30:

```bash
ENABLED="true"   # change 0064: default true == today's behavior
```

with:

```bash
# change 0084: NO default — publish is opt-in. ENABLED_PASSED distinguishes "flag omitted" (a
# caller bug: loud no-op) from an explicit `--enabled false` (a decision: silent no-op). Tracking
# it separately, rather than sniffing an empty ENABLED, keeps an explicit `--enabled ""` failing
# closed instead of silently downgrading to the warn-and-continue path.
ENABLED="" ENABLED_PASSED=false
```

- [ ] **Step 4: Record that the flag was passed**

In `scripts/terminal-publish.sh`, replace line 46:

```bash
    --enabled) ENABLED="$2"; shift ;;
```

with:

```bash
    --enabled) ENABLED="$2"; ENABLED_PASSED=true; shift ;;
```

- [ ] **Step 5: Add the loud omitted-flag default ahead of validation**

In `scripts/terminal-publish.sh`, replace lines 65-67:

```bash
# change 0064: fail closed on an unparseable value — never silently coerce to true, which would
# publish onto the integration branch against the repo's stated intent.
case "$ENABLED" in true|false) ;; *) die "invalid --enabled: '$ENABLED' (expected true|false)" ;; esac
```

with:

```bash
# change 0084: an OMITTED --enabled defaults to DISABLED and says so LOUDLY. A caller that forgot
# the flag is a bug, not a decision — but exit 0 is deliberate (callers trust the exit code, and a
# missing flag must not abort a close-out), so the stderr WARNING is what keeps the skipped publish
# from going unnoticed the way #0043's did. An explicit `--enabled false` stays silent below.
if [ "$ENABLED_PASSED" = false ]; then
  log "WARNING: --enabled not passed — defaulting to DISABLED; NOTHING was published. Pass --enabled true (from the resolved TERMINAL_PUBLISH) to publish."
  ENABLED=false
fi
# change 0064: fail closed on an unparseable value — never silently coerce to true, which would
# publish onto the integration branch against the repo's stated intent.
case "$ENABLED" in true|false) ;; *) die "invalid --enabled: '$ENABLED' (expected true|false)" ;; esac
```

- [ ] **Step 6: Update the usage header**

In `scripts/terminal-publish.sh`, replace lines 18-21:

```bash
# --enabled false (change 0064: the per-repo `terminal_publish` knob) makes this script a no-op:
# the record stays on the metadata branch and nothing is committed onto the integration branch.
# Default true — omitting the flag behaves exactly as before the knob existed. The guard sits
# BEFORE the --id/--adr mode dispatch, so one guard covers BOTH publish shapes.
```

with:

```bash
# --enabled gates the publish (change 0064: the per-repo `terminal_publish` knob; opt-in by default
# since change 0084). --enabled false makes this script a no-op: the record stays on the metadata
# branch and nothing is committed onto the integration branch. There is NO default — an omitted flag
# no-ops too, but LOUDLY (a stderr WARNING), since a caller that forgot it is a bug rather than a
# decision. Pass the resolved TERMINAL_PUBLISH straight through. The guard sits BEFORE the
# --id/--adr mode dispatch, so one guard covers BOTH publish shapes.
```

- [ ] **Step 7: Run the tests to verify they pass**

Run: `bash tests/test_terminal_publish.sh 2>&1 | tail -3`

Expected: `PASS`. The pre-existing `--id abc` / `--adr 1.5` / `--id 5` int-guard assertions must stay green — they die during arg validation, before the new warn block.

- [ ] **Step 8: Commit**

```bash
git add scripts/terminal-publish.sh tests/test_terminal_publish.sh
git commit -m "feat(0084): terminal-publish no-ops loudly when --enabled is omitted

No default: an omitted flag is a caller bug, so it no-ops with a prominent
stderr WARNING (exit 0 — a missing flag must not abort a close-out). An
explicit --enabled false stays a silent, intentional no-op; an explicit empty
value still fails closed."
```

---

### Task 3: Sweep fallback flips + Case B inverted

**Files:**
- Modify: `scripts/docket-status.sh:389`
- Test: `tests/test_docket_status.sh:919-950` (Case B)
- Test: `tests/test_closeout.sh` — the ungated `$PUBLISH` fixture invocations (see Step 5), plus the exclusion comment at lines 41-42

**Interfaces:**
- Consumes: Task 2's `--enabled` contract (this call site always passes the flag explicitly, so it never exercises the omitted path).
- Produces: nothing later tasks import.

**Context you need — do NOT delete Case B.** It exists to guard a real crash: `sweep_execute_one` runs under `set -u`, so a bare `$TERMINAL_PUBLISH` would abort the whole sweep with an unbound-variable error whenever a stale or mocked config export omits the key. The `:-` fallback *is* that guard. Keep the expansion, flip only its value; keep the block's `exits zero (no unbound-variable crash)` and `emits swept` assertions verbatim, and invert only the direction assertion at line 949.

**Correction (surfaced by Task 2's review — supersedes this plan's earlier claim that `test_closeout.sh` needs "no behavioral change"):** that claim was wrong, and the spec always said otherwise ("fixtures that relied on the implicit default now pass/pin `--enabled true` … so they keep testing the publish path"). After Task 2, **19 of `test_closeout.sh`'s 134 assertions are red**: its own `$PUBLISH` fixture invocations omit `--enabled`, so they now no-op instead of publishing. Step 5 repairs them; the spec governs over the plan's earlier text.

What *is* still true: the suite's **0064 sentinel** — which requires every real call site (`skills/`, `scripts/*.sh`, root `*.sh`) to pass `--enabled` — needs no change and must stay green. It excludes `tests/`, which is exactly why the fixtures could drift ungated, and it remains the evidence that no in-repo *caller* shifts behavior under this flip.

- [ ] **Step 1: Invert Case B's direction assertion and its comment (it should now fail)**

In `tests/test_docket_status.sh`, replace lines 919-923:

```bash
# Case B: TERMINAL_PUBLISH entirely UNSET by the config mock (not merely "true") — reproduces the
# exact hazard the fix guards against: sweep_execute_one runs under `set -u`, so a bare
# $TERMINAL_PUBLISH would abort the sweep with an unbound-variable error under a stale/mocked
# config export that doesn't emit the key. "${TERMINAL_PUBLISH:-true}" must default to enabled
# instead, matching pre-0064 behavior.
```

with:

```bash
# Case B: TERMINAL_PUBLISH entirely UNSET by the config mock (not merely "false") — reproduces the
# exact hazard the fix guards against: sweep_execute_one runs under `set -u`, so a bare
# $TERMINAL_PUBLISH would abort the sweep with an unbound-variable error under a stale/mocked
# config export that doesn't emit the key. "${TERMINAL_PUBLISH:-false}" must keep guarding that
# crash (the `:-` expansion is the guard) while defaulting to DISABLED — change 0084: a repo that
# never set the key must never get a direct machine commit on its integration branch.
```

Then replace lines 949-950:

```bash
assert "0064 gate(TERMINAL_PUBLISH unset): defaults to enabled — archived record DOES reach the integration branch" \
  'git -C "$gate_dir2/work" ls-tree -r --name-only origin/main | grep -q "docs/changes/archive/2026-07-11-0060-gate-thing.md"'
```

with:

```bash
assert "0084 gate(TERMINAL_PUBLISH unset): defaults to DISABLED — archived record does NOT reach the integration branch" \
  '! git -C "$gate_dir2/work" ls-tree -r --name-only origin/main | grep -q "docs/changes/archive/2026-07-11-0060-gate-thing.md"'
```

Leave lines 945-947 (`sweep exits zero (no unbound-variable crash)`, `sweep emits swept`) exactly as they are.

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_docket_status.sh 2>&1 | grep -E "NOT OK|^PASS|^FAIL"`

Expected: FAIL with `NOT OK - 0084 gate(TERMINAL_PUBLISH unset): defaults to DISABLED — archived record does NOT reach the integration branch` — the sweep still defaults to enabled and publishes the record onto `main`.

- [ ] **Step 3: Flip the sweep's fallback**

In `scripts/docket-status.sh`, at line 389, replace:

```bash
        --id "$id" --outcome done --enabled "${TERMINAL_PUBLISH:-true}" \
```

with:

```bash
        --id "$id" --outcome done --enabled "${TERMINAL_PUBLISH:-false}" \
```

Keep the `:-` expansion — it is the `set -u` guard, not merely a default.

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_docket_status.sh 2>&1 | tail -3`

Expected: `PASS`. Case A (`0064 gate(disabled): …`, lines ~910-917) must stay green untouched.

- [ ] **Step 5: Re-gate `test_closeout.sh`'s publish-path fixtures**

Confirm the damage first — run `bash tests/test_closeout.sh 2>&1 | grep -c "NOT OK"`; expect `19`.

Every `$PUBLISH` invocation in this suite that is meant to **actually publish** must now pass `--enabled true` explicitly, since the implicit default is gone. Add `--enabled true` to each such invocation, preserving its existing flags and line-wrapping.

Three groups need care — do not blanket-add the flag:

1. **Publish-path fixtures** (the `--id 7 …` and `--adr 3|5 …` invocations that assert a record lands on the integration branch, that a re-run is idempotent, that the ADR index refreshes, that a CAS conflict retries, …) — **add `--enabled true`**. These are the 19 red assertions.
2. **Main-mode fixtures** (`--metadata-branch main`, e.g. lines ~257 and ~293, asserting the mode guard no-ops) — **add `--enabled true`**. They pass either way, but with the flag they assert something strictly stronger and truer to their name: main-mode no-ops *even when publishing is enabled*. Without it they would pass for the wrong reason (the omitted-flag no-op firing first).
3. **Pure arg-validation fixtures** (e.g. `--id 7` with no `--outcome`; `--id 7 --adr 3` mutual exclusion; the bare invocation with neither `--id` nor `--adr`, around lines ~354-363) — **leave them alone**. They die during arg validation *before* the omitted-flag branch is reached, so they are green already and adding the flag would only obscure what they test.

The suite's 0064 sentinel assertions (`0064 wiring: …`) must stay green and untouched throughout — they scan `skills/`/`scripts/`/root, never `tests/`.

- [ ] **Step 5b: Reword the closeout sentinel's exclusion comment**

In `tests/test_closeout.sh`, replace lines 41-42:

```bash
# separate non-continued comment line), and excludes tests/ (which deliberately exercises the
# back-compat default-omitted-enabled path).
```

with:

```bash
# separate non-continued comment line), and excludes tests/ (which deliberately exercises the
# omitted-`--enabled` loud no-op path — change 0084).
```

- [ ] **Step 6: Run the closeout suite to verify it is fully green**

Run: `bash tests/test_closeout.sh 2>&1 | tail -3`

Expected: `PASS`, with zero `NOT OK` lines (down from 19).

Sanity-check that you fixed the fixtures rather than the assertions: `git diff tests/test_closeout.sh` should show `--enabled true` additions to `$PUBLISH` invocations plus the one comment reword — and **no** changes to any `assert` line. If you find yourself editing an assertion to match the new behavior, stop: the publish path is supposed to still publish, and an assertion that stopped holding means the fixture, not the expectation, is wrong.

- [ ] **Step 7: Commit**

```bash
git add scripts/docket-status.sh tests/test_docket_status.sh tests/test_closeout.sh
git commit -m "feat(0084): merge sweep defaults terminal_publish to false

Case B is inverted, not deleted: it guards a real set -u unbound-variable
crash in sweep_execute_one, and the :- expansion is that guard — only the
fallback value flips. Its no-crash and emits-swept assertions stay verbatim.

test_closeout.sh's publish-path fixtures now pass --enabled true explicitly
(the implicit default is gone), keeping the publish path under test; its
arg-validation fixtures are untouched, and its call-site sentinel stays green."
```

---

### Task 4: This repo opts in explicitly

**Files:**
- Modify: `.docket.yml:34-41`

**Interfaces:**
- Consumes: Task 1's flipped resolver default (this pin is what overrides it for this repo).
- Produces: nothing later tasks import.

**Context you need:** `docket-config.sh` reads `.docket.yml` via `git show origin/HEAD:.docket.yml` — the **default branch**, not the working tree. So this pin has no effect until the PR merges, at which point it lands **atomically with the flip**: this repo never has a window where it silently stops publishing. The key is currently present but commented out, resting on the old default.

This task is deliberately separate: a reviewer could reasonably approve the flip while questioning whether *this* repo should keep publishing, and vice versa.

- [ ] **Step 1: Replace the commented-out key with an explicit opt-in**

In `.docket.yml`, replace lines 34-41:

```yaml
# Terminal-publish opt-out (change 0064). Default true: on a terminal transition, the archived
# change file + its spec + its Accepted ADRs are copied from `docket` onto the integration branch
# in a direct commit (and docket-adr publishes ADRs the same way). Set false in a repo where every
# write to the integration branch must go through a PR — records then stay on `docket` only, and
# the integration branch gets code/plans/results via PRs alone. Per-repo-only (coordination-key
# fenced): a value in the global config or .docket.local.yml is warned-and-ignored. Inert in
# main-mode. This repo publishes its terminal records, so the default stands.
# terminal_publish: true
```

with:

```yaml
# Terminal-publish opt-in (changes 0064/0084). Default FALSE: terminal records stay on `docket`
# only, and the integration branch gets code/plans/results via PRs alone. Set true to ALSO copy
# the archived change file + its spec + its Accepted ADRs from `docket` onto the integration
# branch in a direct commit at close-out (and to let docket-adr publish ADRs the same way). That
# is a direct push to the code line: it bypasses PRs, can trip branch protection, and can be
# denied mid-run by an agent permission classifier — so opt in deliberately. Per-repo-only
# (coordination-key fenced): a value in the global config or .docket.local.yml is
# warned-and-ignored. Inert in main-mode.
# This repo mirrors its terminal records onto main, so it opts in explicitly.
terminal_publish: true
```

- [ ] **Step 2: Verify the key parses and resolves to `true`**

Run: `bash -c 'grep -n "^terminal_publish:" .docket.yml'`

Expected: `terminal_publish: true` on one uncommented line (the resolver reads this file from `origin/HEAD` at runtime, so a working-tree export cannot confirm the value until merge — this grep is the check that fits the branch).

- [ ] **Step 3: Commit**

```bash
git add .docket.yml
git commit -m "chore(0084): pin terminal_publish: true for this repo

Its archive-parity practice is unchanged — now explicit rather than resting on
a default that is about to flip. Read from origin/HEAD, so it takes effect
atomically with the flip at merge."
```

---

### Task 5: Docs invert to opt-in framing

**Files:**
- Modify: `README.md:188-190` (config sample), `README.md:329` (terminal-transition paragraph), `README.md:331-350` (the section)
- Modify: `scripts/docket-config.md:107` (default column)
- Modify: `scripts/terminal-publish.md:55-57` (`--enabled` contract)
- Modify: `skills/docket-convention/SKILL.md:27` (config sample), `skills/docket-convention/SKILL.md:264` (Branch model)
- Modify: `skills/docket-convention/references/terminal-close-out.md:66`
- Modify: `skills/docket-adr/SKILL.md:51` and `skills/docket-adr/SKILL.md:70`

**Interfaces:**
- Consumes: the final `--enabled` contract from Task 2 (the docs describe it).
- Produces: nothing later tasks import.

**Context you need — sites verified at reconcile as needing NO change** (do not "fix" them): `scripts/docket-status.md` mentions the knob only conditionally and already spells `--enabled true` explicitly; `skills/docket-finalize-change/SKILL.md` and `skills/docket-status/SKILL.md` pass `<terminal_publish>` through without asserting a default; `skills/docket-implement-next/SKILL.md` and `skills/docket-new-change/SKILL.md` mention terminal-publish generically; `config.yml.example` correctly omits the key (it is per-repo fenced), so `tests/test_config_example.sh` is unaffected. ADR-0027 and all archived changes/plans/results/old specs stay as written.

- [ ] **Step 1: Invert the README config sample**

In `README.md`, replace lines 188-190:

```
terminal_publish: true       # default: copy a closed change's record (change file, spec, Accepted ADRs)
                             # onto the integration branch. false = keep it all on the metadata branch,
                             # for repos where every write to the integration branch must go via a PR
```

with:

```
terminal_publish: false      # default: a closed change's record (change file, spec, Accepted ADRs)
                             # stays on the metadata branch. true = ALSO copy it onto the integration
                             # branch in a direct commit — opt in only if direct pushes suit your workflow
```

- [ ] **Step 2: Requalify the terminal-transition paragraph**

In `README.md`, replace line 329:

```
On a **terminal transition** — a change reaching `done` (PR merged) or `killed` (abandoned) — the driving skill by default copies that change's terminal records onto the integration branch in one dedicated commit: the archived change file, its spec (if any), and the **`Accepted`** ADRs from its manifest, sourced from `origin/docket`. This is a selective **file copy**, never a branch merge, so none of the planning churn comes with it. The **live board stays on `docket`** and is never published. The result: your code history reads as code plus a clean trail of closed-out changes, while the working backlog churns entirely on `docket`.
```

with:

```
On a **terminal transition** — a change reaching `done` (PR merged) or `killed` (abandoned) — the driving skill archives that change on `docket`. A repo that opts in with `terminal_publish: true` (see below) *also* copies the change's terminal records onto the integration branch in one dedicated commit: the archived change file, its spec (if any), and the **`Accepted`** ADRs from its manifest, sourced from `origin/docket`. That copy is selective — a **file copy**, never a branch merge — so none of the planning churn comes with it, and the **live board stays on `docket`** and is never published. The result for a repo that opts in: your code history reads as code plus a clean trail of closed-out changes, while the working backlog churns entirely on `docket`.
```

- [ ] **Step 3: Rewrite the README section as opt-in, with the risks callout**

In `README.md`, replace the section at lines 331-350 (from the `### Keeping metadata off the integration branch (\`terminal_publish\`)` heading through the paragraph ending `never retroactively removes records already published.`) with:

```markdown
### Publishing terminal records to the integration branch (`terminal_publish`, opt-in)

By default docket keeps **all** metadata on the `docket` branch. When a change reaches a terminal
state its record — the archived change file, its spec, and its `Accepted` ADRs — stays there, and
the integration branch accumulates **only** code, plans, and results, every one of them through a
pull request.

Opt in by setting `terminal_publish: true` in the repo's committed `.docket.yml`:

```yaml
terminal_publish: true   # ALSO copy closed change files, specs, and ADRs onto the integration branch
```

Each terminal transition then adds one direct commit to the integration branch carrying that
change's record, and `docket-adr` publishes `Accepted` ADRs the same way — so the code history
reads as code plus a clean trail of closed-out changes and decisions, browsable without switching
branches.

**Opt in deliberately — `true` writes to your code line.** It pushes machine commits **directly**
to the integration branch, bypassing PRs: that fights branch protection on a protected or PR-only
branch, and an autonomous agent's push can be denied mid-run by a permission classifier. A publish
that fails can also gap **silently** — the record simply never arrives, with nothing flagging its
absence. Leave the key unset unless direct commits on the integration branch genuinely suit your
workflow.

The knob gates both publish shapes: the change close-out *and* `docket-adr`'s ADR publish. It is
**per-repo-only** (a machine-scoped value is warned-and-ignored), because the headless
`docket-status` merge sweep must see the same policy as every other agent. It is inert in
`main`-mode, and it is never retroactive — it neither removes records already published nor
back-fills ones it skipped.
```

- [ ] **Step 4: Flip the `docket-config.md` default column**

In `scripts/docket-config.md`, replace line 107:

```
| `terminal_publish` | `true` | no (fenced) | `true`/`false`; `false` makes `terminal-publish.sh` a no-op for BOTH shapes — archived change files, specs, and ADRs stay on the metadata branch. Anything else aborts |
```

with:

```
| `terminal_publish` | `false` | no (fenced) | `true`/`false`; the default `false` makes `terminal-publish.sh` a no-op for BOTH shapes — archived change files, specs, and ADRs stay on the metadata branch. `true` opts in to the direct-commit publish onto the integration branch. Anything else aborts |
```

- [ ] **Step 5: Rewrite the `--enabled` contract**

In `scripts/terminal-publish.md`, replace lines 55-57:

```
`--enabled` defaults to `true`. `--enabled false` (change 0064 — the per-repo `terminal_publish`
knob, resolved by `docket-config.sh`) makes the script a no-op. An unparseable value is rejected
before any git work, like `--id`/`--adr`.
```

with:

```
`--enabled` has **no default** (change 0084 — publishing is opt-in): pass the resolved
`TERMINAL_PUBLISH` value straight through. `--enabled true` publishes; `--enabled false` (change
0064 — the per-repo `terminal_publish` knob, resolved by `docket-config.sh`) makes the script a
silent no-op that exits 0. **Omitting the flag** is treated as disabled — the script no-ops and
exits 0, but prints a prominent `WARNING` to stderr naming that nothing was published, because a
caller that forgot the flag is a bug rather than a decision. Exit 0 on that path is deliberate:
callers trust the exit code and a missing flag must not abort a close-out, so the warning is what
keeps the skipped publish visible. An unparseable value — including an explicit empty one — is
rejected before any git work, like `--id`/`--adr`.
```

- [ ] **Step 6: Invert the convention's config sample**

In `skills/docket-convention/SKILL.md`, replace lines 27-30:

```
terminal_publish: true       # true (default) = copy terminal records (change file, spec, Accepted ADRs)
                             # onto the integration branch at close-out. false = keep them on the
                             # metadata branch only — for repos where every write to the integration
                             # branch must go through a PR. Per-repo-only (coordination-key fenced).
```

with:

```
terminal_publish: false      # false (default) = terminal records (change file, spec, Accepted ADRs)
                             # stay on the metadata branch. true = ALSO copy them onto the
                             # integration branch at close-out — a direct commit to the code line,
                             # so opt in deliberately. Per-repo-only (coordination-key fenced).
```

- [ ] **Step 7: Requalify the convention's Branch model paragraph**

In `skills/docket-convention/SKILL.md` line 264, replace this sentence:

```
A repo may set **`terminal_publish: false`** (per-repo-only; change 0064) to suppress that copy entirely — the archived change file, its spec, and its `Accepted` ADRs then stay on `metadata_branch`, and the integration branch receives only code, plans, and results through the normal PR merge.
```

with:

```
**`terminal_publish` is `false` by default** (per-repo-only; changes 0064/0084), so that copy does not happen unless the repo opts in — the archived change file, its spec, and its `Accepted` ADRs stay on `metadata_branch`, and the integration branch receives only code, plans, and results through the normal PR merge. A repo that wants its records on the code line sets **`terminal_publish: true`**, accepting a direct machine commit onto the integration branch.
```

Leave the rest of line 264 (the close-out sequence description, the `main`-mode note, and the `sync-integration-branch.sh` sentence) unchanged.

- [ ] **Step 8: Note the new default in the close-out reference**

In `skills/docket-convention/references/terminal-close-out.md`, replace lines 66-67:

```
   When the repo sets `terminal_publish: false`, the script is a **no-op that exits 0** — the
   record stays on `docket`, and a suppressed publish is *success*: it does NOT trip the
```

with:

```
   When `terminal_publish` is `false` — **the default** since change 0084; publishing is opt-in —
   the script is a **no-op that exits 0** — the record stays on `docket`, and a suppressed publish
   is *success*: it does NOT trip the
```

- [ ] **Step 9: Requalify `docket-adr`'s publish rule**

In `skills/docket-adr/SKILL.md` line 51, replace this fragment:

```
The rule: **an `Accepted` ADR publishes to the integration branch by default** — the decision ledger is a durable record that belongs with the code (a repo may suppress the copy with `terminal_publish: false`; see the gate at the end of this section).
```

with:

```
The rule: **an `Accepted` ADR publishes to the integration branch only when the repo opts in** with `terminal_publish: true` — the decision ledger is then a durable record sitting with the code (the default is `false`, which keeps it on `docket`; see the gate at the end of this section).
```

Then in `skills/docket-adr/SKILL.md` line 70, replace this fragment:

```
In a repo that sets `terminal_publish: false`, the ADR publish is a no-op that exits 0
```

with:

```
In a repo where `terminal_publish` is `false` (**the default** since change 0084), the ADR publish is a no-op that exits 0
```

- [ ] **Step 10: Verify no stale "default true" claim survives**

Run:

```bash
grep -rn "terminal_publish" README.md scripts/docket-config.md scripts/terminal-publish.md scripts/terminal-publish.sh skills/ .docket.yml | grep -iE "default.*true|true.*\(default\)|defaults to .true|by default"
```

Expected: **no output**. Any hit is a stale claim that this task must fix. (The `.docket.yml` line `terminal_publish: true` and the README's `terminal_publish: true` opt-in sample are intentional and contain no "default" wording, so they will not match.)

- [ ] **Step 11: Commit**

```bash
git add README.md scripts/docket-config.md scripts/terminal-publish.md skills/docket-convention/SKILL.md skills/docket-convention/references/terminal-close-out.md skills/docket-adr/SKILL.md
git commit -m "docs(0084): reframe terminal_publish as opt-in, with a risks callout

Inverts the default-true framing everywhere it was encoded and names what
opting in costs: direct pushes to the code line, branch-protection friction,
classifier denials in autonomous runs, and silently gapped records (#0043)."
```

---

### Task 6: Whole-suite gate

**Files:**
- Test: all of `tests/*.sh` (44 suites)

**Interfaces:**
- Consumes: every preceding task.
- Produces: the green-suite evidence for the PR.

**Context you need:** the repo has no runner script — the suite is the `tests/*.sh` glob. It takes roughly ten minutes. **Run it in ONE foreground call with a long timeout; never background it.** The whole suite matters here, not just the four suites this change names: the blast radius of retiring a string is every guard keyed on that string, and this repo has repeatedly been bitten by a *pre-existing* sentinel in a file the plan never enumerated going red — caught only by a full run.

- [ ] **Step 1: Run every suite in one foreground call**

Run (single Bash call, timeout 600000):

```bash
cd /Users/homer/dev/docket/.worktrees/terminal-publish-opt-in-default && \
fails=""; for f in tests/*.sh; do
  out="$(bash "$f" 2>&1)"; rc=$?
  if [ "$rc" -ne 0 ]; then fails="$fails $f"; echo "=== FAIL: $f ==="; printf '%s\n' "$out" | grep -E "NOT OK" | head -5; fi
done; echo "---"; [ -z "$fails" ] && echo "ALL SUITES PASS" || { echo "FAILED:$fails"; exit 1; }
```

Expected: `ALL SUITES PASS`.

- [ ] **Step 2: Fix any red suite, then re-run**

If a suite outside the four this change names went red, do **not** weaken its assertion to match the new behavior without first establishing which is wrong — a pre-existing guard keyed on the old default is exactly the signal this step exists to surface. Fix the cause, then re-run Step 1 until it prints `ALL SUITES PASS`.

- [ ] **Step 3: Commit any fixes**

```bash
git add -A
git commit -m "test(0084): fix suites keyed on the old terminal_publish default"
```

(Skip this step entirely if Step 1 passed clean with nothing to fix.)

---

## Self-Review

**Spec coverage:**

| Spec requirement | Task |
|---|---|
| `docket-config.sh` default `true` → `false` | Task 1 |
| `terminal-publish.sh` unset sentinel; omitted ⇒ loud no-op; explicit `false` ⇒ silent no-op; `true` ⇒ publish | Task 2 |
| `docket-status.sh:389` sweep fallback flip (keeping the `set -u` guard) | Task 3 |
| Full 4-site inventory honored, incl. `test_docket_status.sh` Case B inverted not deleted | Global Constraints + Task 3 |
| Fence fixtures invert their probe to `true` (anti-vacuity) | Task 1 |
| `test_closeout.sh` back-compat-path comment reworded | Task 3 |
| README sample + section rewrite + risks callout | Task 5 |
| `docket-config.md`, `terminal-publish.md`, convention SKILL + close-out reference, `docket-adr` SKILL | Task 5 |
| This repo pins `terminal_publish: true` | Task 4 |
| Sites verified as needing no change are left alone | Task 5 context |
| Historical artifacts + ADR-0027 untouched | Global Constraints |
| Four affected test suites updated; unset ⇒ no publish covered | Tasks 1, 2, 3 |
| New ADR recording the flip | **Not in this plan** — recorded by the `docket-adr` subagent at step 6 of `docket-implement-next` (Global Constraints) |

**Placeholder scan:** no TBDs; every code step carries the literal before/after text and every test step names its command and expected output.

**Type consistency:** `ENABLED` / `ENABLED_PASSED` are introduced in Task 2 Step 3 and used consistently in Steps 4-5. `TERMINAL_PUBLISH` is the resolver's emitted name (Task 1), consumed as `${TERMINAL_PUBLISH:-false}` in Task 3. The `pub_args` / `gate_dir2` / `tperr` / `lerr` fixture names match the surrounding suites' existing idiom.
