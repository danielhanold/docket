<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0140 — Normalize the inherit model sentinel once for every runner adapter](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-08-0140-normalize-the-inherit-model-sentinel-once-for-every-runner-a.md)**
<!-- docket:backlink:end -->

# Normalize the `inherit` model sentinel once for every runner adapter — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give docket's `inherit` no-pin model sentinel a single owner — `scripts/runner-dispatch.sh` — so every runner adapter behaves identically, and close the one adapter (`codex.sh`) that still forwards the literal sentinel to its child.

**Architecture:** `inherit` is **docket's own sentinel**, never a vendor model ID. Today three adapters decide it independently and `codex.sh` decides differently, because the layer they are all dispatched through (`runner-dispatch.sh`) does not decide it at all. This change normalizes `inherit` → empty **once**, in the facade, immediately after argument parsing — where `[ -n "$MODEL" ]` already gates the `--model` flag, so the sentinel becomes byte-indistinguishable from "no model supplied", which is exactly the documented model-less hand path every adapter already handles. The three adapters **keep or gain** a one-line defensive twin, because each adapter contract documents direct hand invocation that bypasses the facade and each is directly tested; their comments are retargeted to name the facade as the owner. Normalize, not reject: ADR-0067 already rejects the sentinel at *generation* time, so a *dispatch*-time `inherit` is a hand invocation, and the hand contract is tolerant.

**Tech Stack:** POSIX-ish Bash 4+ shell scripts (`scripts/runner-dispatch.sh`, `scripts/runners/*.sh`), co-located Markdown script contracts (`scripts/**/*.md`), and the repo's hand-rolled shell test harness (`tests/test_runner_dispatch.sh`, run via `scripts/run-tests.sh`).

## Global Constraints

Copied verbatim from the repo's always-in-context rules (`AGENTS.md`) and the spec. **Every task's requirements implicitly include this section.**

- **ADR-0015 boundary is unamended.** Real model IDs pass **verbatim** and are never validated or rewritten. This change introduces **no vendor allowlist** and inspects **no vendor value** — it normalizes docket's own sentinel only. Every comment and doc sentence added by this change must say so.
- **ADR-0067 is unamended.** The generation-time gate in `sync-agents.sh`'s `runner_config_error()` — which rejects an empty-or-`inherit` model for any `runner:`-bearing claude agent — is **not touched and not reopened** by this change.
- **No new ADR.** ADR-0015 and ADR-0067 already carry the decisions this leans on.
- **Cross-references anchor on a symbol name or a verbatim-quoted clause, never a line number.** `tests/test_comment_anchor_style.sh` mechanically rejects the `<file>.<ext>:<digits>` form in maintained source. Write `runner-dispatch.sh`'s normalization line, not `runner-dispatch.sh:41`.
- **Never `producer | early-exiting-consumer`** (`grep -q`, `head`, `head -n1`) under `set -o pipefail`. Capture into a variable first, then `grep <<<"$var"`.
- **A `grep` pattern that leads with `--` must declare it:** `grep -qF -- "--model"`. A bare leading `--` is parsed as an option (exit 2) — and inside a negated assert (`! grep …`) that error inverts into a permanently green, vacuous guard.
- **A guard is code: mutation-test it.** Every new assert in this plan carries an explicit mutation step — revert the source line it guards, watch the assert redden, restore. An assert that stays green under its own mutation is a defect.
- **Run the whole suite at the build gate**, not only the enumerated tests. The suite command is `scripts/run-tests.sh` (this is what `finalize.test_command` resolves to in `.docket.yml` — read it there, never from a second copy). A trailing `OVER BUDGET:` line is a finding to act on, not noise.
- **Quote any YAML scalar** carrying a colon-space, trailing colon, ` #`, leading indicator character, or boolean keyword. (No YAML is written by this plan, but the rule stands.)

---

## File Structure

Six files, three responsibilities. No file is created; no file is split.

| File | Responsibility after this change |
|---|---|
| `scripts/runner-dispatch.sh` | **The single owner.** Normalizes `inherit` → empty once, after argument parsing, for every adapter. |
| `scripts/runner-dispatch.md` | Records the sentinel rule in the argument contract, in `## Behavior`, and as an `## Invariants` bullet, with the ADR-0015 boundary stated. |
| `scripts/runners/codex.sh` | Gains the defensive twin it lacks (the actual defect). |
| `scripts/runners/codex.md` | `--model` bullet records the normalization. |
| `scripts/runners/cursor.sh`, `scripts/runners/opencode.sh` | Keep their twins; comments retargeted to name the facade as owner. |
| `scripts/runners/cursor.md` | `--model` bullet records the normalization (folds in change 0135's doc drift). |
| `tests/test_runner_dispatch.sh` | Gains two assert groups: facade-level normalization (via the `RUNNERS_DIR` probe seam) and the codex adapter twin. |

**`scripts/runners/opencode.md` is deliberately untouched** — its `--model` bullet already carries the sentence (`Docket's own `inherit` no-pin sentinel is normalized to "no flag" and never reaches opencode.`). `tests/test_runner_cursor.sh` and `tests/test_runner_opencode.sh` are **deliberately untouched** — their existing `inherit` asserts become this change's regression net for the defensive twins.

**Why the facade assert must not go through `codex.sh`.** After Task 2 both layers normalize, so an assert that dispatches `--runner codex --model inherit` and checks the child's argv passes on the strength of *either* layer alone — it pins an outcome, not a mechanism, and deleting the facade's line leaves it green. Task 1's asserts therefore route through a throwaway probe adapter dropped into a `RUNNERS_DIR` of our own (the seam `tests/test_runner_dispatch.sh` already uses for its `0173 rd:` value-class asserts), which records the argv **the facade handed the adapter**. That is the only place the facade's own decision is observable.

## Task Ordering

Task 1 (facade + its test + its contract) is the owner and lands first, so its assert is written and mutation-tested while `codex.sh` is still un-normalized. Task 2 (codex twin) and Task 3 (comment retarget + `cursor.md`) each stand alone afterward. A reviewer can reject any one while approving its neighbors.

---

### Task 1: The single owner — normalize `inherit` in `scripts/runner-dispatch.sh`

**Files:**
- Modify: `scripts/runner-dispatch.sh` — insert the normalization immediately after the required-flag validation (`[ -n "$AGENT" ]  || die "--agent is required"`), before the `case "$RUNNER" in` path-traversal check.
- Modify: `scripts/runner-dispatch.md` — the `--model` / `--effort` bullet under `## Usage`; a new sentence on step 4 (`**Handoff**`) under `## Behavior`; a new bullet under `## Invariants`.
- Test: `tests/test_runner_dispatch.sh` — new assert group appended after the `0173 rd:` tolerant-posture group and before the `# ---- 0237: exec -> call-and-return` group.

**Interfaces:**
- Consumes: nothing from an earlier task (this is the first).
- Produces: the invariant later tasks reference in their comments — after argument parsing in `runner-dispatch.sh`, the shell variable `MODEL` is **empty whenever the caller passed `--model inherit`**, so the existing `[ -n "$MODEL" ] && args+=( --model "$MODEL" )` handoff line emits no `--model` flag. Tasks 2 and 3 add/keep adapter-local twins that must describe this line as "the owner", using the phrase **`runner-dispatch.sh`'s normalization**, and must call themselves **defensive twins**. No function is added or renamed anywhere in this plan.

- [ ] **Step 1: Write the failing test**

Append this block to `tests/test_runner_dispatch.sh`, immediately **after** the `rm -rf "$SBX"` that closes the `0173 rd: and it still MASKS the lower layer` group and **before** the `# ---- 0237: exec -> call-and-return, exit code preserved verbatim -------------------` banner:

```bash
# ---- 0140: `inherit` is DOCKET'S OWN no-pin sentinel, owned by the FACADE ------------
# The facade normalizes `inherit` -> empty right after argument parsing, so no adapter re-decides
# it. These asserts route through a THROWAWAY PROBE ADAPTER (the same RUNNERS_DIR seam the 0173
# value-class asserts use), never through codex.sh: codex.sh carries its own defensive twin, so an
# assert dispatched through it would pass on the strength of either layer and would stay green with
# the facade's line deleted — an outcome assert, not a mechanism one. The probe records the argv the
# FACADE handed the adapter, which is the only place the facade's own decision is observable.
# DOCKET_HARNESS_ROOT is pinned into the sandbox so the global config layer cannot reach the
# developer's real ~/.config/docket/config.yml.
make_fixture
mkdir -p "$SBX/runners"
cat > "$SBX/runners/probe.sh" <<'PROBE'
#!/usr/bin/env bash
printf '%s\n' "$@" > "$MOCK_ARGV"
PROBE
chmod +x "$SBX/runners/probe.sh"
PARGV="$SBX/probe-argv.txt"
dispatch_probe(){  # $@ = extra facade args -> fills PARGV with the adapter's argv, one entry per line
  : > "$PARGV"
  ( cd "$SBX" && RUNNERS_DIR="$SBX/runners" DOCKET_HARNESS_ROOT="$SBX" MOCK_ARGV="$PARGV" \
      bash "$FACADE" --runner probe --agent status "$@" >/dev/null 2>&1 )
}

dispatch_probe --model inherit --effort high
pargv="$(cat "$PARGV")"
assert "0140 rd: inherit sentinel => no --model flag reaches the adapter" \
  '! grep -qxF -- "--model" <<<"$pargv"'
assert "0140 rd: the literal sentinel never reaches the adapter" \
  '! grep -qxF -- "inherit" <<<"$pargv"'
# Effort is a SEPARATE knob: normalizing the model must not disturb it. Without this, dropping the
# whole flag pair would satisfy both asserts above.
assert "0140 rd: --effort survives model normalization" \
  'grep -qxF -- "--effort" <<<"$pargv" && grep -qxF -- "high" <<<"$pargv"'

# Non-regression control (ADR-0015): a REAL model ID is not a sentinel and still passes verbatim.
# Without this leg, deleting the `[ -n "$MODEL" ]` guard outright — i.e. never forwarding a model at
# all — would keep every assert above green.
dispatch_probe --model gpt-5.1-codex
pargv="$(cat "$PARGV")"
assert "0140 rd: a real model ID still passes verbatim (ADR-0015)" \
  'grep -qxF -- "gpt-5.1-codex" <<<"$pargv"'
assert "0140 rd: ... carried by its own --model flag" \
  'grep -qxF -- "--model" <<<"$pargv"'
rm -rf "$SBX"
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_runner_dispatch.sh 2>&1 | grep -n '0140 rd'`

Expected: the first two `0140 rd:` lines print `NOT OK` (the facade forwards `--model inherit` verbatim today, so both `--model` and `inherit` appear in the probe's argv). The `--effort survives`, the two non-regression control lines, and every pre-existing assert print `ok`.

If the first two print `ok`, **stop** — either the normalization already exists or the probe never ran. Confirm by `cat` of the probe argv file inside a scratch run before proceeding.

- [ ] **Step 3: Write the minimal implementation**

In `scripts/runner-dispatch.sh`, insert immediately after the line `[ -n "$AGENT" ]  || die "--agent is required"` and before the comment block beginning `# The runner name becomes a path component below`:

```bash
# `inherit` is DOCKET'S OWN no-pin sentinel (never a vendor model ID), normalized to "no model"
# HERE — the one layer every adapter is dispatched through — so no adapter re-decides it. The
# handoff below already gates the flag on `[ -n "$MODEL" ]`, which makes the sentinel
# indistinguishable from "no model supplied": exactly the model-less hand path every adapter
# contract documents. NORMALIZE, not reject — ADR-0067 already rejects the sentinel at GENERATION
# time (sync-agents.sh's runner_config_error), so a dispatch-time `inherit` is a hand invocation,
# and the hand contract is tolerant. This is sentinel normalization, NOT model-ID validation
# (ADR-0015): no vendor value is inspected and no allowlist is introduced.
[ "$MODEL" = "inherit" ] && MODEL=""
```

Note the trailing `MODEL=""` assignment: it is the last command of a `&&` list whose left side is
false whenever the model is not the sentinel, so under `set -uo pipefail` (this script does **not**
set `-e`) the non-sentinel case leaves a non-zero `$?` behind harmlessly — nothing between here and
the next command reads it. Written as an `if` it would be equivalent; the one-line form matches the
`case "$EFFORT" in auto) EFFORT="" ;; esac` shape the adapters already use for the sibling sentinel.

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_runner_dispatch.sh 2>&1 | grep -n '0140 rd'`

Expected: all six `0140 rd:` asserts print `ok`.

Then run the whole file: `bash tests/test_runner_dispatch.sh; echo "exit=$?"`
Expected: `exit=0`, no `NOT OK` line anywhere (the pre-existing `0237` run-gate and `0206` anchor groups must be untouched).

- [ ] **Step 5: Mutation-test the new asserts**

A guard is code. Prove each new assert detects the state it was written for.

```bash
cp scripts/runner-dispatch.sh /tmp/rd-backup.sh   # NOT `git checkout --`: that restores to HEAD
                                                  # and would silently destroy the uncommitted work
                                                  # being tested.
# Mutation A: delete the normalization entirely.
grep -v '^\[ "\$MODEL" = "inherit" \] && MODEL=""$' /tmp/rd-backup.sh > scripts/runner-dispatch.sh
bash tests/test_runner_dispatch.sh 2>&1 | grep -c 'NOT OK'
cp /tmp/rd-backup.sh scripts/runner-dispatch.sh
```

Expected for Mutation A: at least the two asserts `0140 rd: inherit sentinel => no --model flag reaches the adapter` and `0140 rd: the literal sentinel never reaches the adapter` print `NOT OK`.

```bash
# Mutation B: over-normalize — drop the model unconditionally. The sentinel asserts stay green;
# the non-regression control must catch it.
sed 's/^\[ "\$MODEL" = "inherit" \] && MODEL=""$/MODEL=""/' /tmp/rd-backup.sh > scripts/runner-dispatch.sh
bash tests/test_runner_dispatch.sh 2>&1 | grep 'NOT OK'
cp /tmp/rd-backup.sh scripts/runner-dispatch.sh
rm -f /tmp/rd-backup.sh
```

Expected for Mutation B: `0140 rd: a real model ID still passes verbatim (ADR-0015)` and `0140 rd: ... carried by its own --model flag` print `NOT OK`. If Mutation B reddens nothing, the control leg is decoration — fix it before continuing.

After both mutations, re-run `bash tests/test_runner_dispatch.sh; echo "exit=$?"` and confirm `exit=0` — i.e. the restore actually landed.

- [ ] **Step 6: Update `scripts/runner-dispatch.md`**

Three edits, all in `scripts/runner-dispatch.md`.

**(a)** Under `## Usage`, replace the `--model` / `--effort` bullet — currently ending `…so the model-less case reaches this facade only on a direct hand invocation.` — with:

```markdown
- `--model` / `--effort` — forwarded to the adapter verbatim (model is ADR-0015 opaque
  passthrough end-to-end). The generated shim bakes these only from **user-configured** values;
  a value that came from docket's shipped `agents/harness-defaults.yml` is never forwarded. Since
  change 0205 a `runner:`-bearing agent with **no** user-configured model is a generation-time
  error, so the model-less case reaches this facade only on a direct hand invocation.
  **`--model inherit` is docket's own no-pin sentinel, never a vendor model ID** — this facade
  normalizes it to "no model" for **every** adapter, so a sentinel dispatch is byte-identical to
  omitting `--model` and no adapter re-decides it. Normalizing docket's own sentinel is **not
  model-ID validation**: the ADR-0015 boundary is unamended, no vendor value is inspected, and no
  allowlist of model IDs is introduced.
```

**(b)** Under `## Behavior`, append to step 4 (`**Handoff**`), after the sentence ending `…on every path where the run gate takes no action.`:

```markdown
   `--model` is omitted whenever no model resolved — including when the caller passed the
   `inherit` sentinel, which is normalized to empty right after argument parsing, above every
   adapter.
```

**(c)** Under `## Invariants`, insert a new bullet immediately after the `Never runs a child harness itself; all child specifics live in the adapter.` bullet:

```markdown
- `inherit` is docket's own no-pin sentinel and is normalized to "no model" **here**, once, for
  every adapter — adapters keep a one-line defensive twin for their documented hand-invocation
  path, never as a second decision. Real model IDs are untouched (ADR-0015).
```

- [ ] **Step 7: Verify the house guards still pass**

Run:
```bash
bash tests/test_comment_anchor_style.sh; echo "anchor=$?"
bash tests/test_script_contracts_coverage.sh; echo "contracts=$?"
bash tests/test_grep_portability.sh; echo "grep=$?"
```
Expected: `anchor=0`, `contracts=0`, `grep=0`. The anchor guard is the one at real risk — it rejects the `<file>.<ext>:<digits>` form, so confirm the comment added in Step 3 names `sync-agents.sh`'s `runner_config_error` as a symbol and carries **no** line number.

- [ ] **Step 8: Commit**

```bash
git add scripts/runner-dispatch.sh scripts/runner-dispatch.md tests/test_runner_dispatch.sh
git commit -m "fix(0140): give the inherit model sentinel a single owner in runner-dispatch

Normalize \`inherit\` -> empty immediately after argument parsing, so the
sentinel is byte-identical to 'no model supplied' at every adapter and no
adapter re-decides it. Sentinel normalization only — ADR-0015's opaque
model-ID passthrough is unamended and ADR-0067's generation-time gate is
untouched.

Asserts route through a throwaway probe adapter via the RUNNERS_DIR seam,
not through codex.sh, so they pin the facade's own decision rather than an
outcome either layer could produce."
```

---

### Task 2: The codex defensive twin — `scripts/runners/codex.sh`

**Files:**
- Modify: `scripts/runners/codex.sh` — beside the existing `auto` effort-sentinel normalization (`case "$EFFORT" in auto) EFFORT="" ;; esac`), inside the `# --- flag mapping ---` section.
- Modify: `scripts/runners/codex.md` — the `--model <m>` bullet under `## Usage`.
- Test: `tests/test_runner_dispatch.sh` — new assert group appended after Task 1's `0140 rd:` group.

**Interfaces:**
- Consumes: Task 1's invariant — `runner-dispatch.sh` is the sentinel's owner. This task's comment must name it with the phrase **`runner-dispatch.sh`'s normalization** and must label itself a **defensive twin**, matching what Task 3 writes into `cursor.sh` and `opencode.sh`.
- Produces: nothing later tasks consume. Task 3 is independent.

**Why this is the real defect.** `codex.sh` already normalizes docket's *other* sentinel — `auto` for effort — one line above, with a comment saying it does so "exactly as runners/cursor.sh and runners/opencode.sh do". The model sentinel is the same shape and the same file, and only it was missed. Separately, `sync-agents.sh`'s `runner_config_error` comment already asserts *"every adapter normalizes it to 'no flag'"* — a claim that is **false today** and that this task makes true.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_runner_dispatch.sh`, immediately after the `rm -rf "$SBX"` closing Task 1's `0140 rd:` group:

```bash
# ---- 0140: codex adapter DEFENSIVE TWIN ---------------------------------------------
# runner-dispatch.sh owns the sentinel (asserted above, through the probe seam). The adapter keeps
# its own one-line normalization because codex.md documents direct hand invocation that bypasses
# the facade, and this file exercises the adapter directly — exactly that path. Mirrors the
# existing inherit groups in tests/test_runner_cursor.sh and tests/test_runner_opencode.sh, with
# one deliberate difference: on codex, effort SURVIVES the model-less case, because codex carries
# reasoning effort on a separate `-c model_reasoning_effort=` flag rather than encoding it inside
# the model value. That asymmetry is correct, not a bug, and pinning it is what stops a later
# "make the adapters consistent" edit from silently deleting a working effort pin.
make_fixture
run_adapter --agent status --model inherit --effort high >/dev/null 2>&1
argv="$(cat "$LOG")"
assert "0140 codex: inherit sentinel => no -m flag" '! grep -qxF -- "-m" <<<"$argv"'
assert "0140 codex: the literal sentinel never reaches codex exec" \
  '! grep -qxF -- "inherit" <<<"$argv"'
assert "0140 codex: effort SURVIVES the model-less case (separate -c flag)" \
  'grep -qxF -- "model_reasoning_effort=high" <<<"$argv"'
assert "0140 codex: child still ran (normalization is not an abort)" \
  'grep -qxF -- "--output-last-message" <<<"$argv"'
# Non-regression control (ADR-0015): a real model ID still reaches codex exec verbatim.
: > "$LOG"
run_adapter --agent status --model gpt-5.1-codex >/dev/null 2>&1
argv="$(cat "$LOG")"
assert "0140 codex: a real model ID still passes verbatim (ADR-0015)" \
  'grep -qxF -- "gpt-5.1-codex" <<<"$argv"'
assert "0140 codex: ... carried by its own -m flag" 'grep -qxF -- "-m" <<<"$argv"'
rm -rf "$SBX"
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_runner_dispatch.sh 2>&1 | grep -n '0140 codex'`

Expected: `0140 codex: inherit sentinel => no -m flag` and `0140 codex: the literal sentinel never reaches codex exec` print `NOT OK`; the effort, child-ran, and both non-regression asserts print `ok`.

This is the load-bearing observation of the whole change: **the adapter is invoked directly here**, so Task 1's facade normalization cannot mask the defect. If these two print `ok` at this step, the test is not exercising the adapter — check that `run_adapter` (not `$FACADE`) is being called.

- [ ] **Step 3: Write the minimal implementation**

In `scripts/runners/codex.sh`, inside `# --- flag mapping ---`, insert immediately **after** the line `case "$EFFORT" in auto) EFFORT="" ;; esac` and **before** `case "$EFFORT" in max) EFFORT="xhigh" ;; esac`:

```bash
# `inherit` is DOCKET'S OWN "no pin" sentinel for the MODEL, the twin of the `auto` effort sentinel
# directly above and never a vendor model ID. DEFENSIVE TWIN: runner-dispatch.sh's normalization is
# the single owner and a dispatched run never arrives here holding the sentinel — this line covers
# the direct hand invocation codex.md documents, which bypasses the facade entirely. Without it,
# `codex exec -m inherit` hands the child a non-existent model ID. Sentinel normalization, not
# model-ID validation (ADR-0015): no vendor value is inspected.
case "$MODEL" in inherit) MODEL="" ;; esac
```

The `case` form (rather than `cursor.sh`'s `if`) matches the two `case "$EFFORT"` lines it sits between, keeping the file's own idiom; the behavior is identical.

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_runner_dispatch.sh; echo "exit=$?"`
Expected: `exit=0`, all six `0140 codex:` asserts `ok`, all six `0140 rd:` asserts still `ok`, no `NOT OK` anywhere.

- [ ] **Step 5: Mutation-test the new asserts**

```bash
cp scripts/runners/codex.sh /tmp/codex-backup.sh   # NOT `git checkout --` — see Task 1 Step 5.
# Mutation A: delete the twin.
grep -v '^case "\$MODEL" in inherit) MODEL="" ;; esac$' /tmp/codex-backup.sh > scripts/runners/codex.sh
bash tests/test_runner_dispatch.sh 2>&1 | grep 'NOT OK'
cp /tmp/codex-backup.sh scripts/runners/codex.sh
# Mutation B: over-normalize — blank the model unconditionally.
sed 's/^case "\$MODEL" in inherit) MODEL="" ;; esac$/MODEL=""/' /tmp/codex-backup.sh > scripts/runners/codex.sh
bash tests/test_runner_dispatch.sh 2>&1 | grep 'NOT OK'
cp /tmp/codex-backup.sh scripts/runners/codex.sh
rm -f /tmp/codex-backup.sh
```

Expected for Mutation A: the two sentinel asserts redden. Expected for Mutation B: the two `0140 codex:` non-regression asserts redden. Then re-run `bash tests/test_runner_dispatch.sh; echo "exit=$?"` → `exit=0`, confirming the restore landed.

- [ ] **Step 6: Update `scripts/runners/codex.md`**

In `scripts/runners/codex.md`, under `## Usage`, replace the `--model <m>` bullet — currently ending `The model-less case is reachable only by invoking this adapter by hand.` — with:

```markdown
- `--model <m>` (optional **at this CLI**, required in practice) — passed to `codex exec -m`
  **verbatim** (ADR-0015 opaque passthrough; docket never validates model IDs). Omitted here ⇒ the
  child's own default model — but since change 0205 a `runner:`-bearing agent with no
  user-configured model is a **generation-time error** (ADR-0067), so a generated shim never omits
  it. The model-less case is reachable only by invoking this adapter by hand. Docket's own
  `inherit` no-pin sentinel is normalized to "no flag" and never reaches `codex exec`;
  `runner-dispatch.sh` owns that normalization for every adapter and this adapter keeps a
  defensive twin for the hand path above. Unlike Cursor, **effort survives** the model-less case
  here, because `model_reasoning_effort` is a separate flag rather than an encoding inside the
  model value.
```

- [ ] **Step 7: Commit**

```bash
git add scripts/runners/codex.sh scripts/runners/codex.md tests/test_runner_dispatch.sh
git commit -m "fix(0140): normalize the inherit sentinel in the codex adapter too

codex.sh forwarded \`-m inherit\` verbatim, handing the child a
non-existent model ID — the last adapter still deciding the sentinel
differently from its two siblings, and the one that decided wrong. It
already normalized the sibling \`auto\` effort sentinel one line above.

Defensive twin, not a second owner: runner-dispatch.sh owns the
normalization, and this covers the hand invocation codex.md documents,
which bypasses the facade. Effort deliberately still survives the
model-less case on codex — a separate flag, not a model-value encoding."
```

---

### Task 3: Retarget the surviving twins — `cursor.sh`, `opencode.sh`, `cursor.md`

**Files:**
- Modify: `scripts/runners/cursor.sh` — the comment block above `if [ "$MODEL" = "inherit" ]; then MODEL=""; fi`. **The code line itself does not change.**
- Modify: `scripts/runners/opencode.sh` — the comment block above `if [ "$MODEL" = "inherit" ]; then MODEL=""; fi`. **The code line itself does not change.**
- Modify: `scripts/runners/cursor.md` — the `--model <m>` bullet under `## Usage`.
- Test: no new test. `tests/test_runner_cursor.sh` and `tests/test_runner_opencode.sh` already assert the behavior these comments describe and stay **untouched** — they are the regression net proving the retarget changed no behavior.

**Interfaces:**
- Consumes: Task 1's invariant and the exact phrases Task 2 established — **`runner-dispatch.sh`'s normalization** and **defensive twin** — so all three adapters describe the ownership identically.
- Produces: nothing.

**Why comment-only, and why not deletion.** Leaving `cursor.sh`'s comment claiming *"the same normalization emit_cursor_md performs"* recreates the drift this change exists to end: three files each claiming local ownership of one rule. Deleting the lines instead of retargeting them was considered and rejected — every adapter contract documents direct hand invocation, `tests/test_runner_cursor.sh` and `tests/test_runner_opencode.sh` exercise the adapters directly, so removal would regress the change-0135 defect on the hand path and force rewriting passing tests. Cost of keeping: one line plus a comment, per adapter. `scripts/runners/opencode.md` needs no edit — its `--model` bullet already carries the sentence.

- [ ] **Step 1: Capture the baseline (this task must change no behavior)**

```bash
bash tests/test_runner_cursor.sh    > /tmp/cursor-before.txt 2>&1; echo "cursor=$?"
bash tests/test_runner_opencode.sh  > /tmp/opencode-before.txt 2>&1; echo "opencode=$?"
```
Expected: `cursor=0`, `opencode=0`. Keep both files — Step 5 diffs against them.

- [ ] **Step 2: Retarget the `cursor.sh` comment**

In `scripts/runners/cursor.sh`, replace the three comment lines beginning `# \`inherit\` is DOCKET'S OWN "no pin" sentinel` and ending `# something and takes the effort pin down with it, while this WARN branch stays unreachable.` — i.e. everything between the `# dropped — LOUDLY, because a silently-dropped pin is this change's own root cause.` line and the `# This is sentinel normalization, not model-ID validation (ADR-0015): no vendor value is inspected.` line — with:

```bash
# `inherit` is DOCKET'S OWN "no pin" sentinel (never a vendor model ID). DEFENSIVE TWIN:
# runner-dispatch.sh's normalization is the single owner and a dispatched run never arrives here
# holding the sentinel — this line covers the direct hand invocation cursor.md documents, which
# bypasses the facade. Without it, an explicit `model: inherit` reaches cursor-agent as a literal
# model ID (`--model inherit[effort=xhigh]`), where its compatible-model fallback silently
# substitutes something and takes the effort pin down with it, while the WARN branch below stays
# unreachable — change 0135's defect, reproduced on the hand path.
```

Leave the following two lines exactly as they are:

```bash
# This is sentinel normalization, not model-ID validation (ADR-0015): no vendor value is inspected.
if [ "$MODEL" = "inherit" ]; then MODEL=""; fi
```

- [ ] **Step 3: Retarget the `opencode.sh` comment**

In `scripts/runners/opencode.sh`, replace the three comment lines beginning `# \`inherit\` is DOCKET'S OWN "no pin" sentinel` and ending `# Without this, \`--model inherit\` would reach opencode as a literal provider/model string.` with:

```bash
# `inherit` is DOCKET'S OWN "no pin" sentinel (never a vendor model ID). DEFENSIVE TWIN:
# runner-dispatch.sh's normalization is the single owner and a dispatched run never arrives here
# holding the sentinel — this line covers the direct hand invocation opencode.md documents, which
# bypasses the facade. Without it, `--model inherit` would reach opencode as a literal
# provider/model string. Sentinel normalization, not model-ID validation (ADR-0015).
```

Leave the code line `if [ "$MODEL" = "inherit" ]; then MODEL=""; fi` exactly as it is.

- [ ] **Step 4: Update `scripts/runners/cursor.md`**

In `scripts/runners/cursor.md`, under `## Usage`, replace the `--model <m>` bullet — currently ending `The model-less case is reachable only by invoking this adapter by hand.` — with:

```markdown
- `--model <m>` (optional **at this CLI**, required in practice) — passed to `cursor-agent --model`
  **verbatim** (ADR-0015 opaque passthrough; docket never validates or rewrites model IDs). Omitted
  here ⇒ the child's own default model — but since change 0205 a `runner:`-bearing agent with no
  user-configured model is a **generation-time error** (ADR-0067), so a generated shim never omits
  it. The model-less case is reachable only by invoking this adapter by hand. Docket's own
  `inherit` no-pin sentinel is normalized to "no flag" and never reaches `cursor-agent`;
  `runner-dispatch.sh` owns that normalization for every adapter and this adapter keeps a
  defensive twin for the hand path above. Because Cursor encodes effort inside the model value,
  the sentinel behaves exactly like no model at all — the effort-dropped WARN below fires.
```

Preserve the surrounding bullet text verbatim where this block repeats it; the only additions are the last two sentences.

- [ ] **Step 5: Verify no behavior changed**

```bash
bash tests/test_runner_cursor.sh    > /tmp/cursor-after.txt 2>&1;   echo "cursor=$?"
bash tests/test_runner_opencode.sh  > /tmp/opencode-after.txt 2>&1; echo "opencode=$?"
diff /tmp/cursor-before.txt   /tmp/cursor-after.txt   && echo "cursor output identical"
diff /tmp/opencode-before.txt /tmp/opencode-after.txt && echo "opencode output identical"
```
Expected: `cursor=0`, `opencode=0`, and **both** diffs empty — this task edits comments and one contract bullet only, so any output change means a code line was touched by mistake. Then `rm -f /tmp/cursor-before.txt /tmp/cursor-after.txt /tmp/opencode-before.txt /tmp/opencode-after.txt`.

- [ ] **Step 6: Verify the house guards still pass**

```bash
bash tests/test_comment_anchor_style.sh;    echo "anchor=$?"
bash tests/test_cursor_contract_docs.sh;    echo "cursor-docs=$?"
bash tests/test_cursor_permissions_docs.sh; echo "cursor-perm=$?"
bash tests/test_script_contracts_coverage.sh; echo "contracts=$?"
```
Expected: all four `=0`. `test_cursor_contract_docs.sh` greps `cursor.md`, so the Step-4 rewrite is the one at real risk — if it reddens, read which phrase it pins and preserve that phrase verbatim rather than loosening the guard.

- [ ] **Step 7: Commit**

```bash
git add scripts/runners/cursor.sh scripts/runners/opencode.sh scripts/runners/cursor.md
git commit -m "docs(0140): retarget the surviving adapter twins at the single owner

cursor.sh and opencode.sh keep their normalization — both adapters are
documented as hand-invocable and are directly tested, so removing the
lines would regress change 0135's defect on that path. Their comments now
name runner-dispatch.sh as the owner and themselves as defensive twins,
instead of each claiming the rule locally. cursor.md's --model bullet
records the sentinel, discharging 0135's doc drift; opencode.md already
carried it. No code line changed: the cursor and opencode suites emit
byte-identical output."
```

---

### Task 4: Full-suite gate

**Files:** none modified. This task produces the build-evidence record, not a diff.

**Interfaces:**
- Consumes: the branch state left by Tasks 1–3.
- Produces: a green full-suite run whose `head_sha` equals branch HEAD.

- [ ] **Step 1: Run the whole suite**

Run: `scripts/run-tests.sh; echo "exit=$?"`

This is the command `finalize.test_command` resolves to in `.docket.yml` — read it there, never from a second copy. Expected: `exit=0`.

- [ ] **Step 2: Act on any `OVER BUDGET:` line**

A trailing `OVER BUDGET:` line does **not** fail the run and nothing else will catch it. This change adds ~12 asserts and two `make_fixture` cycles to `tests/test_runner_dispatch.sh`; if that file now exceeds its budget in `tests/runtime-budgets.tsv`, record the measured number in the run report and raise the budget only with the measurement written down beside it — a tolerance constant measured on one machine is wrong in both directions elsewhere.

- [ ] **Step 3: Record the head SHA**

```bash
git rev-parse HEAD
git diff --shortstat origin/main...HEAD
```

Capture both for the build-evidence record. The suite must have been run at this exact SHA.

---

## Self-Review

**1. Spec coverage.**

| Spec section | Task |
|---|---|
| §1 Single owner: normalize in `runner-dispatch.sh` after argument parsing | Task 1, Step 3 |
| §2 `codex.sh` gains the twin | Task 2, Step 3 |
| §2 `cursor.sh` / `opencode.sh` keep theirs, comments retargeted | Task 3, Steps 2–3 |
| §3 `runner-dispatch.md` records the rule + the ADR-0015 boundary | Task 1, Step 6 (a)(b)(c) |
| §3 `cursor.md` + `codex.md` `--model` bullets record the normalization | Task 3 Step 4; Task 2 Step 6 |
| §3 `opencode.md` already has it ⇒ untouched | File Structure, Task 3 preamble |
| §4 dispatch-level inherit asserts | Task 1, Step 1 |
| §4 codex-adapter inherit asserts, incl. effort survives | Task 2, Step 1 |
| §4 existing cursor/opencode inherit tests stay untouched | Task 3, Step 5 (diffed byte-identical) |
| §5 No new ADR | Global Constraints |
| Out of scope: sentinel meaning, model-ID validation, ADR-0067's gate | Global Constraints |

No gaps.

**2. Placeholder scan.** No `TBD`, no "add error handling", no "similar to Task N" — Tasks 2 and 3 repeat their code and their non-regression legs in full rather than referring back. Every code step carries a literal block.

**3. Type consistency.** No functions, types, or signatures are introduced. The one cross-task identifier is the shell variable `MODEL`, spelled identically in all four files it appears in. The two cross-task comment phrases — **`runner-dispatch.sh`'s normalization** and **defensive twin** — are spelled identically in Tasks 2 and 3. The test helper `dispatch_probe` is defined and used within Task 1 only; `run_adapter` and `make_fixture` are the test file's pre-existing helpers, used unmodified.

**One deviation from the spec worth flagging to the reviewer:** the spec places the codex twin "beside its flag mapping" without pinning the order relative to the `auto` effort sentinel. This plan puts it immediately after `case "$EFFORT" in auto) EFFORT="" ;; esac`, grouping docket's two sentinels together and above codex's `max`→`xhigh` vendor mapping. That ordering is behaviorally irrelevant (the two variables are independent) and chosen for readability.
