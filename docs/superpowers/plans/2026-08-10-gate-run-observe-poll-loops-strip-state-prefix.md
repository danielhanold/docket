<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0286 — Caller-authored gate-run --observe poll loops strip the state= prefix and never terminate](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0286-gate-run-observe-poll-loops-strip-state-prefix.md)**
<!-- docket:backlink:end -->

# gate-run `--observe` caller poll loops — canonical loop, keyed on the printed form

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Teach callers of `gate-run --observe` one copy-paste-correct poll loop that keys on the exact printed `state=<name>` line, so a finished gate can never read as unfinished until its budget burns.

**Architecture:** Three edits, no behavior change to `scripts/gate-run.sh`. (1) `scripts/gate-run.md` gains a `### The caller's loop` subsection under `## Usage` with one fenced bash example plus a one-line anti-pattern note. (2) `skills/docket-build/SKILL.md` § *Gate execution posture* gains one sentence pointing at that loop and restating the single keying rule. (3) The fenced example is **executable surface**, so `tests/test_gate_run.sh` extracts the fence and runs it against a stubbed `--observe`, proving the disposition on every terminal state, retry on `running` only, and a fail-closed unknown-line arm; `tests/test_gate_execution_posture.sh` gains the prose sentinel for the SKILL.md sentence.

**Tech Stack:** POSIX-ish bash (repo floor is GNU Bash 4+, resolved as `DOCKET_BASH_PATH`), the repo's own `assert`-based shell test harness, markdown contracts under `scripts/` and `skills/`.

## Global Constraints

Copied verbatim from the spec and from the repo rules this diff lands under. Every task's requirements implicitly include this section.

- **No change to `scripts/gate-run.sh`, the six-state vocabulary, or the `state=` print format.** Callers and tests already key on it. This branch touches no `.sh` under `scripts/`.
- **The helper never polls for the caller** — a stated invariant of `scripts/gate-run.md` (line beginning `- **The helper never polls for the caller.**` in `## Invariants`). **Do not weaken, reword, or delete it.** The new subsection is an example of how a caller **reads the report line**; the *disposition* policy for each state stays in `skills/docket-build/SKILL.md` § *Gate execution posture*, which that invariant points at. The new subsection must explicitly defer disposition policy there, or it contradicts the invariant it sits eight sections above.
- **No `--wait` verb.** Rejected at groom (spec assumption 1). Do not add one, do not hint at one.
- **`runner-dispatch --observe` is out of scope** (changes 0277 / 0284). Touch no `runner-dispatch` file.
- **Match shape is prefix-match on the whole printed line** (`state=passed*`), never extraction (`${out%% *}`, `${out#state=}`, `awk '{print $1}'`, `cut`). Spec assumption 3.
- **The capture must neutralize the exit status** — `out="$(… --observe "$run_dir")" || true` — because `--observe` exits 1 on `unavailable` and the contract's own rule is "callers key on the stdout report line, never on the exit code". Spec assumption 3.
- **The unknown-line arm is terminal, never a retry** (treat as `unavailable`, stop polling). Spec assumption 4. This is the single most important property in the diff — a retry `*)` arm *is* the observed production defect.
- **Capture-then-match, never a producer piped into an early-exiting consumer** (`AGENTS.md` § Shell, `tests/test_pipe_shapes.sh`). Applies to the fenced example and to every line of new test code.
- **Agent-executed markdown is code.** `scripts/*.md` is inside the scan population of `tests/test_bsd_tool_defaults.sh` (`md_scope_files`) and inside `tests/test_grep_portability.sh`'s tracked-file population (everything except `docs/`). The new fenced block must therefore satisfy the repo's shell rules exactly as a `.sh` file would: no bare `mktemp`, no bare `mv`, no `grep` bound-repetition beyond BSD limits.
- **Guards are code — mutation-test them.** Every assert added by this plan is paired with a named mutation that reddens it, and Step "prove the mutation" is a required step, not a suggestion. Restore mutated files from a **backup copy**, never `git checkout --` (that restores to HEAD and destroys the uncommitted work under test).
- **A prose guard binds phrase to claim**, over a whitespace-flattened haystack, with a **bounded** gap (~80–160 chars), and the slice it binds within carries a **non-vacuity anchor** assert.
- **No backtick may appear inside any pattern passed to `assert`** — `assert` runs its second argument through `eval`, so a backtick becomes command substitution. This is stated in `tests/test_gate_execution_posture.sh` and holds for `tests/test_gate_run.sh` too.
- Prose wraps at roughly 100 columns, matching the surrounding files.

---

## File Structure

| File | Action | Responsibility |
|---|---|---|
| `scripts/gate-run.md` | Modify — add `### The caller's loop` under `## Usage`, after the `stdout is the protocol` verb table (currently ends at the `launch-failed` paragraph, ~line 78) and before `## Run-directory layout` | Carries the one canonical, copy-paste-correct poll loop and the anti-pattern line. The contract callers are already told to read. |
| `tests/test_gate_run.sh` | Modify — append a new section after the existing state-table asserts (the block ending with the "the table marks running — and only running — retryable" assert, ~line 630) | Extracts the fence and **executes** it against a stubbed `--observe`. Seven behavioral fixtures + three prose asserts. |
| `skills/docket-build/SKILL.md` | Modify — one sentence appended to the `**The shipped implementation of clauses 1–3**` paragraph (currently ends `…and **only \`running\` is retryable**.`, line 281) | The one keying rule, restated at the site where loops are actually authored, plus the pointer at the canonical loop. |
| `tests/test_gate_execution_posture.sh` | Modify — three asserts appended to the `# (12a) the helper…` group (after the "helper: only running is retryable" assert, ~line 495) | The prose sentinel binding the SKILL.md sentence to its claim, inside the already-sliced `$helper_blk` / `$helper_flat`. |
| `tests/test_skill_size_budgets.sh` | Modify — raise the `skills/docket-build/SKILL.md` row and add the required rationale comment | The SKILL.md sentence does not fit in the current headroom (measured pre-change: 372 lines / 3613 words against a 375 / 3650 budget). |

No files are created. No new test file, therefore no new `tests/runtime-budgets.tsv` row: both touched test files already have rows (`tests/test_gate_run.sh 20 parallel`, `tests/test_gate_execution_posture.sh 10 parallel`), and the added fixtures are sub-second stubbed subprocesses.

---

### Task 1: The canonical loop in `scripts/gate-run.md`, and the executable guard that runs it

**Files:**
- Modify: `scripts/gate-run.md` — insert a `### The caller's loop` subsection at the end of `## Usage`, immediately after the paragraph beginning "`launch-failed` is one shape and never a taxonomy" and immediately before the line `## Run-directory layout`
- Test: `tests/test_gate_run.sh` — append after the existing state-table assert block

**Interfaces:**
- Consumes: nothing from earlier tasks (this is the first).
- Produces, for Task 2:
  - the heading text `### The caller's loop` (Task 2's SKILL.md sentence points at it by this exact name);
  - the section slice helpers already defined in `tests/test_gate_run.sh` at lines 590–592 — `csection(){…}`, `cfrom(){…}`, `flat(){…}` — and the variable `$contract` (the whole `scripts/gate-run.md` body) and `$state_names` (the six state tokens derived from the contract's own state table). Task 1 reuses them; it defines no new global helper that Task 2 depends on.

---

- [ ] **Step 1: Read the insertion point so the edit anchors on real bytes**

Run:

```bash
cd /Users/homer/dev/docket/.worktrees/gate-run-observe-poll-loops-strip-state-prefix
sed -n '70,82p' scripts/gate-run.md
```

Expected: the `| Verb | stdout payload |` table, the `launch-failed is one shape and never a taxonomy` paragraph, a blank line, then `## Run-directory layout`. The new subsection goes in that blank gap — inside `## Usage`, above `## Run-directory layout`.

---

- [ ] **Step 2: Write the failing test — the prose half**

Append to `tests/test_gate_run.sh`, **after** the existing assert `"the table marks running — and only running — retryable (got '$retryable_rows')"` (it is the last assert of the state-table block, around line 630). These asserts read `$contract`, `csection`, `flat` and `$state_names`, all already in scope at that point.

```bash
# ---- THE CALLER'S LOOP IS A TAUGHT, EXECUTABLE SURFACE (change 0286) -------------
# The helper prints `state=<name>`; the caller owns the loop. A live agent-authored loop
# re-tokenized the line (`awk '{print $1}'`) and matched BARE state names, so `state=passed` fell
# to its `*)` arm and a finished gate looked unfinished until the 30-minute budget burned. The
# contract now ships one copy-paste-correct loop, and because an agent runs those bytes verbatim
# (learnings: agent-executed-markdown-is-code) the asserts below EXECUTE the fence rather than
# grepping it.
usage_blk="$(csection Usage)"
assert "the Usage section was located (non-vacuity anchor)" \
  '[ "$(grep -c . <<<"$usage_blk")" -ge 30 ]'
assert "the contract carries a caller-loop subsection, inside Usage" \
  'grep -qxF -- "### The caller'"'"'s loop" <<<"$usage_blk"'

# The fence itself. Named terminator, and the extraction is asserted non-vacuous before anything
# reads it: an unlocated fence is an empty string, and every behavioural assert below would then
# pass against a loop that is not there (learnings: section-slice-needs-a-named-terminator).
loop_fence="$(awk '
  /^### The caller'"'"'s loop$/ {insec=1; next}
  insec && /^#/ {insec=0}
  insec && /^```bash$/ {infence=1; next}
  infence && /^```$/ {infence=0; insec=0; next}
  infence {print}
' <<<"$contract")"
assert "the canonical loop fence was located (non-vacuity anchor)" \
  '[ "$(grep -c . <<<"$loop_fence")" -ge 12 ]'

# EVERY state the contract's own table defines gets a prefix-matched arm — derived from that
# table, never hand-listed, so a seventh state reddens this instead of ageing into a stale list.
for st in $state_names; do
  assert "the canonical loop gives state=$st a prefix-matched arm" \
    'grep -qF -- "state='"$st"'*" <<<"$loop_fence"'
done
# The capture neutralizes the exit status: --observe exits 1 on unavailable, and an errexit caller
# without this dies before any arm runs.
assert "the canonical loop neutralizes the observe exit status" \
  'grep -qF -- "|| true" <<<"$loop_fence"'
# The anti-pattern is stated beside the example, bound to what it forbids rather than merely
# present (learnings: prose-guard-binds-phrase-to-claim). Read on a flattened haystack so a pure
# re-flow cannot redden it.
loop_sec="$(awk '/^### The caller'"'"'s loop$/{f=1;next} f && /^#/{f=0} f' <<<"$contract")"
loop_flat="$(flat "$loop_sec")"
assert "the caller-loop section was located (non-vacuity anchor)" \
  '[ "$(grep -c . <<<"$loop_sec")" -ge 20 ]'
assert "the section forbids re-tokenizing the report line into bare state names" \
  'grep -qiE "(never|not)[^.]{0,120}bare state name" <<<"$loop_flat"'
assert "and it names the unknown-line arm as terminal, never a retry" \
  'grep -qiE "unknown[^.]{0,140}(never a retry|stop polling)" <<<"$loop_flat"'
# Disposition policy is the CALLER's and stays in the skill: the page's own invariant says the
# helper never polls for the caller, so the example must defer rather than restate policy here.
assert "the section defers disposition policy to the build skill's posture" \
  'grep -qF -- "Gate execution posture" <<<"$loop_sec"'
```

---

- [ ] **Step 3: Run it to make sure it fails**

Run:

```bash
cd /Users/homer/dev/docket/.worktrees/gate-run-observe-poll-loops-strip-state-prefix
bash tests/test_gate_run.sh 2>&1 | grep -v '^ok - ' | sed -n '1,40p'
```

Expected: `NOT OK - the contract carries a caller-loop subsection, inside Usage`, `NOT OK - the canonical loop fence was located (non-vacuity anchor)`, and the derived per-state arm asserts, all red. (`grep -v` then `sed -n` — never `head`, per the pipefail rule.)

---

- [ ] **Step 4: Write the subsection in `scripts/gate-run.md`**

Insert exactly this, between the `launch-failed is one shape and never a taxonomy` paragraph and the `## Run-directory layout` heading:

````markdown
### The caller's loop

The helper never polls for you, so the loop is yours to write — and there is exactly one correct
shape for reading what it prints. Copy this one; it is executed verbatim by
`tests/test_gate_run.sh` against every state below.

```bash
# `run_dir` is the handle --launch printed. GATE_OBSERVATION_BUDGET is the docket execution policy
# from the Step-0 config export, in minutes; 0 is legal and buys exactly one observation.
deadline=$(( $(date +%s) + GATE_OBSERVATION_BUDGET * 60 ))
state=""
while :; do
  # Capture, THEN match. The `|| true` is load-bearing: --observe exits 1 on `unavailable`, and the
  # rule is that callers key on the stdout report line, never on the exit code — without it an
  # errexit caller dies before any arm below runs.
  out="$("${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh gate-run --observe "$run_dir")" || true
  case "$out" in
    state=running*)     : ;;                         # the only retryable state
    state=passed*)      state=passed;      break ;;
    state=failed*)      state=failed;      break ;;
    state=died*)        state=died;        break ;;  # the trailing `cause=…` is matched by the `*`
    state=stopped*)     state=stopped;     break ;;
    state=unavailable*) state=unavailable; break ;;
    *)                  state=unavailable; break ;;  # unknown line: fail closed, NEVER a retry arm
  esac
  [ "$(date +%s)" -lt "$deadline" ] || break         # budget spent; `state` stays empty
  sleep 10
done
# An empty `state` means the budget ran out with the run still `running` — that is the fail-closed
# case, not a verdict about the child.
```

**Never re-tokenize the report line and match bare state names.** `awk '{print $1}'`, `cut -d= -f2`,
or stripping the `state=` prefix all re-introduce a parsing step that can drift, and the observed
failure is silent: the first field *is* `state=passed`, so a `case` on bare `passed` matches
nothing, every observation falls to `*)`, and a gate that finished in seconds is polled until its
budget is exhausted. Match the **whole printed line by its prefix** instead — no state name is a
prefix of another, and the closed vocabulary is validated helper-side, so the prefix match is exact.

The **unknown-line arm is terminal**: a line outside the vocabulary means something is wrong with
the invocation or the environment, so the loop stops polling and treats it as `unavailable`. It is
never a retry arm — that is precisely the shape that never terminates.

What to *do* with each state is the caller's policy, not the helper's: dispositions (which states
may be relaunched, when a budget exhaustion halts) are stated in `skills/docket-build/SKILL.md`
§ *Gate execution posture*.
````

Line-wrap the prose at ~100 columns to match the rest of the page. Do **not** touch the
`- **The helper never polls for the caller.**` invariant in `## Invariants`.

---

- [ ] **Step 5: Run the prose asserts to verify they pass**

Run:

```bash
cd /Users/homer/dev/docket/.worktrees/gate-run-observe-poll-loops-strip-state-prefix
bash tests/test_gate_run.sh 2>&1 | grep -v '^ok - ' | sed -n '1,40p'
```

Expected: no `NOT OK` lines from the new block. (The file's final line prints the count; a fully green run prints only that.)

---

- [ ] **Step 6: Write the failing test — the executable half**

Append to `tests/test_gate_run.sh`, directly after the Step-2 block. This is the assert that makes the example an *oracle* rather than a decoration.

```bash
# ---- AND THE FENCE ACTUALLY RUNS ------------------------------------------------
# The fence is extracted and EXECUTED against a stub that answers a scripted sequence of observe
# lines, so the asserts key on what the loop DOES, not on how it is spelled. Two mutation keys:
#   (a) rewrite `state=passed*` to bare `passed` -> `state=passed` falls to the fail-closed `*)`
#       arm and the loop terminates with the WRONG disposition (unavailable). Reddens fixture 1.
#   (b) rewrite the `*)` arm to a retry (the observed defect shape) -> the malformed line is polled
#       instead of disposed. Reddens fixture 6 on BOTH the disposition and the observation count,
#       in milliseconds, under the fixture's own budget — never the 30-minute production one.
#   (c) drop the `|| true` -> fixture 5 (the stub exits 1) aborts under the harness's errexit.
LOOPBOX="$SBX/loopbox"; mkdir -p "$LOOPBOX"
cat >"$LOOPBOX/docket.sh" <<'STUB'
#!/usr/bin/env bash
# Stub facade: answers the Nth line of $OBS_SCRIPT, repeating the last line forever after. Exits 1
# on `unavailable` exactly as the real --observe does, which is what makes the `|| true` testable.
n=$(( $(cat "$OBS_COUNT") + 1 )); printf '%s' "$n" >"$OBS_COUNT"
line="$(sed -n "${n}p" "$OBS_SCRIPT")"
[ -n "$line" ] || line="$(sed -n '$p' "$OBS_SCRIPT")"
printf '%s\n' "$line"
case "$line" in state=unavailable*) exit 1 ;; esac
exit 0
STUB
chmod +x "$LOOPBOX/docket.sh"
printf '%s\n' "$loop_fence" >"$LOOPBOX/loop.body"

# Run the extracted fence under `set -euo pipefail` — the posture an agent-authored caller actually
# carries, and the only way mutation key (c) can be observed. `sleep` is shadowed by a no-op shell
# function so a `running` fixture costs microseconds instead of the fence's real 10s interval; the
# fence itself is byte-unmodified.
run_loop(){ # $1 = budget (minutes), $2… = the scripted observe lines -> prints "state|observations"
  local budget="$1"; shift
  printf '%s\n' "$@" >"$LOOPBOX/script"
  printf '0' >"$LOOPBOX/count"
  {
    printf '%s\n' 'set -euo pipefail' 'sleep(){ :; }' \
      'run_dir=/nonexistent-run-dir' \
      "DOCKET_SCRIPTS_DIR=$LOOPBOX" \
      "GATE_OBSERVATION_BUDGET=$budget"
    cat "$LOOPBOX/loop.body"
    printf '%s\n' 'printf "%s" "${state}"'
  } >"$LOOPBOX/harness.sh"
  local st
  st="$(OBS_SCRIPT="$LOOPBOX/script" OBS_COUNT="$LOOPBOX/count" \
        "$DOCKET_BASH_PATH" "$LOOPBOX/harness.sh" 2>/dev/null)" || st="ERRExit"
  printf '%s|%s' "$st" "$(cat "$LOOPBOX/count")"
}

# 1 — a terminal state on the first look: one observation, the state's own disposition.
assert "the loop disposes state=passed as passed, in one observation" \
  '[ "$(run_loop 5 state=passed)" = "passed|1" ]'
assert "the loop disposes state=failed as failed" \
  '[ "$(run_loop 5 state=failed)" = "failed|1" ]'
assert "the loop disposes state=stopped as stopped" \
  '[ "$(run_loop 5 state=stopped)" = "stopped|1" ]'
# 2 — running is the ONLY retryable state, and the retry actually happens.
assert "the loop retries state=running and takes the next state's verdict" \
  '[ "$(run_loop 5 state=running state=running state=failed)" = "failed|3" ]'
# 3 — the trailing cause= qualifier is absorbed by the prefix match, not parsed.
assert "the loop disposes a died line carrying a cause= suffix" \
  '[ "$(run_loop 5 "state=died cause=vanished")" = "died|1" ]'
# 5 — unavailable exits 1; the loop keys on the LINE, never the status. Mutation key (c).
assert "the loop reads state=unavailable off the line despite the nonzero exit" \
  '[ "$(run_loop 5 state=unavailable)" = "unavailable|1" ]'
# 6 — THE DEFECT THIS CHANGE EXISTS FOR. An unknown line is disposed, never polled: exactly one
# observation, and `unavailable`. A retry arm turns both halves of this into their opposites.
assert "the loop fails closed on a malformed line, in exactly one observation" \
  '[ "$(run_loop 5 "hello world")" = "unavailable|1" ]'
assert "the loop fails closed on an empty line too" \
  '[ "$(run_loop 5 "")" = "unavailable|1" ]'
# 7 — a zero budget buys exactly ONE observation, then leaves `state` empty: the fail-closed
# budget-exhaustion case, distinct from every verdict above (SKILL.md posture clauses 5 and 6).
assert "a zero budget buys one observation and reports no verdict" \
  '[ "$(run_loop 0 state=running)" = "|1" ]'
```

---

- [ ] **Step 7: Run the executable asserts to verify they pass**

Run:

```bash
cd /Users/homer/dev/docket/.worktrees/gate-run-observe-poll-loops-strip-state-prefix
bash tests/test_gate_run.sh 2>&1 | grep -v '^ok - ' | sed -n '1,40p'
```

Expected: no `NOT OK` lines.

If fixture 4 (`state=running state=running state=failed`) reports `failed|2` rather than `failed|3`, the fence's `running` arm is falling through to `break` — re-read Step 4's `case`. If any fixture reports `ERRExit`, the fence aborted under errexit: the `|| true` is missing or the harness prologue is wrong. **Treat a red supplied assert as a possible defect in the assert itself before debugging the loop** (learnings: `plan-supplied-test-code-is-unverified`) — check `$loop_fence` is non-empty first with `printf '%s\n' "$loop_fence"`.

---

- [ ] **Step 8: Prove the mutations redden — all three keys**

For each key: copy the file aside, mutate, run, restore from the **copy** (never `git checkout --`, which restores to HEAD and would destroy this task's uncommitted work — learnings: `mutation-restore-needs-a-backup-copy`).

```bash
cd /Users/homer/dev/docket/.worktrees/gate-run-observe-poll-loops-strip-state-prefix
cp scripts/gate-run.md /tmp/gate-run.md.bak

# (a) bare-token arm -> wrong disposition on fixture 1
perl -pi -e 's/^(\s*)state=passed\*\)/$1passed)/' scripts/gate-run.md
out="$(bash tests/test_gate_run.sh 2>&1)"; grep -c '^NOT OK' <<<"$out"
grep '^NOT OK' <<<"$out"
cp /tmp/gate-run.md.bak scripts/gate-run.md

# (b) retry `*)` arm -> the observed defect: fixture 6 stops disposing
perl -pi -e 's/^(\s*)\*\)(\s+)state=unavailable; break ;;/$1*)$2: ;;/' scripts/gate-run.md
out="$(bash tests/test_gate_run.sh 2>&1)"; grep '^NOT OK' <<<"$out"
cp /tmp/gate-run.md.bak scripts/gate-run.md

# (c) drop the `|| true` -> fixture 5 aborts under errexit
perl -pi -e 's/\)" \|\| true$/)"/' scripts/gate-run.md
out="$(bash tests/test_gate_run.sh 2>&1)"; grep '^NOT OK' <<<"$out"
cp /tmp/gate-run.md.bak scripts/gate-run.md

# confirm the restore is byte-clean
diff /tmp/gate-run.md.bak scripts/gate-run.md && bash tests/test_gate_run.sh 2>&1 | grep -c '^NOT OK'
```

Expected:
- (a) reddens `the loop disposes state=passed as passed, in one observation` (it reports `unavailable|1`).
- (b) reddens `the loop fails closed on a malformed line, in exactly one observation` **and** `the loop fails closed on an empty line too` (both report `|N` — budget-exhausted, no verdict — instead of `unavailable|1`).
- (c) reddens `the loop reads state=unavailable off the line despite the nonzero exit` (reports `ERRExit|1`).
- The final restore prints no diff and `0` NOT OK lines.

If a mutation prints **zero** `NOT OK` lines, the perl substitution did not land (check with `git diff scripts/gate-run.md` while mutated) — a mutation that did not apply is not a passing guard (learnings: `assert-detects-removal-not-replacement`).

---

- [ ] **Step 9: Run the two touched test files in full, plus the markdown-shape guards this fence is now inside**

The fence is agent-executed markdown, so it is inside the scan populations of the repo-wide shell guards. Run them explicitly rather than discovering it at the branch gate:

```bash
cd /Users/homer/dev/docket/.worktrees/gate-run-observe-poll-loops-strip-state-prefix
for t in test_gate_run test_gate_run_stop test_bsd_tool_defaults test_grep_portability \
         test_pipe_shapes test_script_contracts_coverage test_comment_anchor_style; do
  printf '== %s: ' "$t"
  out="$(bash "tests/$t.sh" 2>&1)"; printf '%s NOT OK\n' "$(grep -c '^NOT OK' <<<"$out")"
done
```

Expected: `0 NOT OK` for every one.

---

- [ ] **Step 10: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/gate-run-observe-poll-loops-strip-state-prefix
git add scripts/gate-run.md tests/test_gate_run.sh
git commit -m "fix(0286): canonical --observe poll loop in gate-run.md, executed by its guard

The helper prints state=<name> and the caller owns the loop. An agent-authored
loop re-tokenized the line and matched bare state names, so every observation
fell to its *) arm and a finished gate polled until its budget burned.

Ships one copy-paste-correct loop in the contract's Usage section: capture with
the exit status neutralized, case arms prefix-matched on the whole printed line,
only state=running retried, and a fail-closed unknown-line arm. The fence is
executable surface, so tests/test_gate_run.sh extracts and RUNS it against a
stubbed --observe across every terminal state, the running retry, the cause=
suffix, the nonzero-exit leg, a malformed line, and a zero budget."
```

---

### Task 2: The keying rule in `skills/docket-build/SKILL.md`, its budget row, and its sentinel

**Files:**
- Modify: `skills/docket-build/SKILL.md:274-281` — append to the `**The shipped implementation of clauses 1–3**` paragraph
- Modify: `tests/test_skill_size_budgets.sh` — the `skills/docket-build/SKILL.md` row (line 967 in the `BUDGETS` heredoc) plus a rationale comment in the block above it
- Test: `tests/test_gate_execution_posture.sh` — three asserts in the `# (12a) the helper…` group

**Interfaces:**
- Consumes from Task 1: the heading `### The caller's loop` in `scripts/gate-run.md`. The SKILL.md sentence names it verbatim; if Task 1's heading text differs, this task's sentence and its sentinel must match whatever Task 1 shipped — check with `grep -n "caller's loop" scripts/gate-run.md` before writing.
- Produces: nothing later tasks consume. This is the last task.

---

- [ ] **Step 1: Write the failing test — the prose sentinel**

Append to `tests/test_gate_execution_posture.sh`, immediately after the existing assert `"helper: only running is retryable"` (~line 495). `$helper_blk` and `$helper_flat` are the paragraph slice already built at that point, and its non-vacuity anchor already ran — so these asserts inherit both.

Remember: **no backticks in any pattern** (the harness `eval`s them), and read the flattened haystack for anything spanning a line break.

```bash
# (12a-ii) THE CALLER'S LOOP IS NOT REINVENTED PER CALL SITE (change 0286). A live loop matched
# bare state names against a line whose first field is `state=passed`, so a finished gate read as
# unfinished until its budget burned. This is where loops are actually authored, so the keying rule
# is restated here — bound to what it is asserted ABOUT, not merely present (learnings:
# prose-guard-binds-phrase-to-claim). Mutation: delete the added sentence -> all three redden.
assert "helper: the posture points at the contract's canonical loop rather than inviting a new one" \
  'grep -qiE "canonical[^.]{0,80}loop" <<<"$helper_flat"'
assert "helper: the keying rule is bound to the full printed state= form" \
  'grep -qiE "(key|match)[^.]{0,120}state=[^.]{0,60}(form|printed|line)" <<<"$helper_flat"'
assert "helper: and the bare-token loop is named as the thing that never terminates" \
  'grep -qiE "bare[^.]{0,60}(never terminat|does not terminat)" <<<"$helper_flat"'
```

---

- [ ] **Step 2: Run it to make sure it fails**

Run:

```bash
cd /Users/homer/dev/docket/.worktrees/gate-run-observe-poll-loops-strip-state-prefix
bash tests/test_gate_execution_posture.sh 2>&1 | grep '^NOT OK' || echo "NO FAILURES"
```

Expected: three `NOT OK` lines — the canonical-loop pointer, the keying rule, and the bare-token clause. Not `NO FAILURES`.

---

- [ ] **Step 3: Write the sentence in `skills/docket-build/SKILL.md`**

Append to the end of the paragraph that currently ends `…and **only \`running\` is retryable**.` (line 281). The sentence must stay **inside** that paragraph — `tests/test_gate_execution_posture.sh`'s `para()` closes the slice at the next column-0 `**` or heading, so a new blank line plus a bolded lead-in would put it in a slice nothing reads.

```markdown
**Reuse the canonical loop** in `gate-run.md` § *The caller's loop* verbatim rather than authoring
one, and key each `case` arm on the full printed `state=<name>` line: a loop that re-tokenizes that
line and matches bare state names matches nothing, so it never terminates on a state — it polls a
finished gate until the budget is spent.
```

Append it with a single space after the existing final period, then re-wrap the paragraph at ~100 columns.

---

- [ ] **Step 4: Raise the size budget row and record why**

The pre-change actuals are 372 lines / 3613 words against a `375 3650` budget — 3 lines and 37 words of headroom, less than the sentence needs. Measure the post-edit actuals and set the row per the rule stated at the top of `tests/test_skill_size_budgets.sh`: lines up to the next multiple of 5, words to the next multiple of 50, and if that lands within 25 words of the actual, take the multiple *after* it.

```bash
cd /Users/homer/dev/docket/.worktrees/gate-run-observe-poll-loops-strip-state-prefix
printf 'lines=%s words=%s\n' "$(wc -l < skills/docket-build/SKILL.md | tr -d ' ')" \
                              "$(wc -w < skills/docket-build/SKILL.md | tr -d ' ')"
```

With the sentence as drafted this reads about `lines=376 words=3663`, giving the row `380 3700`. **Use the measured numbers, not these** — if the measurement differs, apply the rule to what you measured.

Edit the `BUDGETS` heredoc row (currently `skills/docket-build/SKILL.md                               375 3650`) to the computed values, keeping the column alignment of the surrounding rows. Then add the rationale comment immediately after the `# Change 0282 raises skills/docket-build/SKILL.md 335/3150 -> 375/3650 …` block, matching that block's voice:

```bash
# Change 0286 raises skills/docket-build/SKILL.md 375/3650 -> 380/3700. § *Gate execution posture*
# gains ONE sentence: reuse the canonical poll loop in scripts/gate-run.md § *The caller's loop*
# verbatim, and key each case arm on the full printed state=<name> line, because a loop matching
# bare state names never terminates on a state. WHERE ELSE IT WAS CONSIDERED, per the naming
# requirement above: the loop ITSELF lives in scripts/gate-run.md and is executed there by
# tests/test_gate_run.sh, so only the keying rule is restated here — a second full copy of the loop
# is the restatement class the learnings ledger warns accumulates its own guards. It cannot live
# only in the contract, because this file is where a caller authoring a loop is actually reading;
# it cannot live in references/gate-execution.md, whose neutrality invariant is one-directional and
# whose read-once-ahead-of-the-act placement does not intervene at the act. The pre-change actuals
# were 372/3613 against 375/3650, i.e. 3 lines and 37 words of headroom — less than the sentence.
```

---

- [ ] **Step 5: Run the sentinel and the budget guard to verify they pass**

```bash
cd /Users/homer/dev/docket/.worktrees/gate-run-observe-poll-loops-strip-state-prefix
for t in test_gate_execution_posture test_skill_size_budgets; do
  printf '== %s: ' "$t"
  out="$(bash "tests/$t.sh" 2>&1)"; printf '%s NOT OK\n' "$(grep -c '^NOT OK' <<<"$out")"
done
```

Expected: `0 NOT OK` for both.

---

- [ ] **Step 6: Prove the mutation reddens**

```bash
cd /Users/homer/dev/docket/.worktrees/gate-run-observe-poll-loops-strip-state-prefix
cp skills/docket-build/SKILL.md /tmp/build-skill.md.bak

# Delete the added sentence (keep the paragraph, keep the vocabulary elsewhere in the file):
# the guard must redden on the CLAIM going away, not on a word disappearing from the file.
python3 - <<'PY'
import re, pathlib
p = pathlib.Path("skills/docket-build/SKILL.md")
s = p.read_text()
i = s.index("**Reuse the canonical loop**")
j = s.index("\n\n", i)
p.write_text(s[:i].rstrip() + "\n" + s[j:])
PY
out="$(bash tests/test_gate_execution_posture.sh 2>&1)"; grep '^NOT OK' <<<"$out"
cp /tmp/build-skill.md.bak skills/docket-build/SKILL.md
diff /tmp/build-skill.md.bak skills/docket-build/SKILL.md && echo RESTORED
```

Expected: all three new asserts red under the mutation; `RESTORED` at the end.

If fewer than three redden, the surviving assert is matching text from elsewhere in the paragraph rather than the new sentence — tighten its gap and re-run. A prose assert that stays green with its sentence deleted is decoration.

---

- [ ] **Step 7: Run the whole suite**

The branch gate runs everything; run it here so the task ends verified rather than hopeful.

```bash
cd /Users/homer/dev/docket/.worktrees/gate-run-observe-poll-loops-strip-state-prefix
scripts/run-tests.sh 2>&1 | tail -30
```

Expected: a green summary. A trailing `OVER BUDGET:` line is a finding to act on, not noise.

---

- [ ] **Step 8: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/gate-run-observe-poll-loops-strip-state-prefix
git add skills/docket-build/SKILL.md tests/test_gate_execution_posture.sh tests/test_skill_size_budgets.sh
git commit -m "fix(0286): key the gate wait on the printed state= form, at the site loops are authored

Gate execution posture now points at gate-run.md's canonical loop and restates
the one rule a caller cannot derive from the state table: key each case arm on
the full printed state=<name> line. A loop matching bare state names matches
nothing and polls a finished gate until the budget is spent.

The sentinel binds the phrase to its claim inside the already-sliced helper
paragraph, so a rewrite that keeps the words and drops the rule reddens. The
size budget row is raised 375/3650 -> 380/3700 with its rationale, since the
paragraph had 37 words of headroom against a ~50-word sentence."
```

---

## Self-Review

**1. Spec coverage.**

| Spec requirement | Task |
|---|---|
| Canonical poll loop subsection in `gate-run.md`, under *Usage* | Task 1 Step 4 |
| Capture with `\|\| true`, capture-then-match | Task 1 Step 4 (fence), asserted Step 6 fixture 5, mutation key (c) |
| Prefix-matched `case` arms on the whole line | Task 1 Step 4, asserted Step 2 (derived from the state table) and Step 6 fixtures 1–5 |
| Only `state=running*` retried | Task 1 Step 6 fixture 4 |
| Fail-closed unknown-line arm, never a retry | Task 1 Step 4, fixtures 6, mutation key (b) |
| Bounded by `GATE_OBSERVATION_BUDGET` | Task 1 Step 4, fixture 7 (zero budget buys one observation) |
| One-line anti-pattern note | Task 1 Step 4, asserted Step 2 |
| SKILL.md § *Gate execution posture* one sentence | Task 2 Step 3 |
| Mutation key (a): bare `passed` -> wrong terminal disposition | Task 1 Step 8 (a) |
| Mutation key (b): retry `*)` -> non-termination under a fixture-local budget | Task 1 Step 8 (b) |
| Prose sentinel binding the SKILL.md sentence to its `state=` claim | Task 2 Steps 1, 6 |
| Assumption 1 (no `--wait`), 5 (no `gate-run.sh` change), 6 (`runner-dispatch` untouched) | Global Constraints |
| Assumption 7 (fixture-local budget, milliseconds) | Task 1 Step 6 `run_loop` shadows `sleep`; fixtures use budget 5 and 0 |

No spec requirement is unassigned. Spec *Files touched* named `tests/test_gate_run.sh` **or** `tests/test_docket_build_skill.sh` for the SKILL.md sentinel; the reconcile pass resolved that to `tests/test_gate_execution_posture.sh`, which is where this file's posture prose guards actually live, and Task 2 places it beside them.

**2. Placeholder scan.** No TBD/TODO, no "add appropriate error handling", no "similar to Task N" — the mutation and stub code is written out in both tasks. Every code step carries its block.

**3. Type consistency.** `$loop_fence` (Task 1 Step 2) is the variable Task 1 Step 6 writes to `loop.body`; `run_loop <budget> <lines…>` returns the single string `state|observations` and every fixture compares against that exact shape. `$helper_blk` / `$helper_flat` in Task 2 are the pre-existing slices from `tests/test_gate_execution_posture.sh` line ~453, not new variables. `csection`, `cfrom`, `flat` are the pre-existing helpers at `tests/test_gate_run.sh:590-592`. `$state_names` is defined in the same file's state-table block, above both new blocks.

One unresolved-by-design detail, flagged rather than guessed: Task 2's budget row values are **measured**, not asserted here — Step 4 prints the actual and states the rounding rule. The drafted `380 3700` is the expectation, not the instruction.
