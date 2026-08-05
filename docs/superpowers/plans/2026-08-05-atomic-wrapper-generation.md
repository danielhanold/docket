<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0207 — sync-agents aborts mid-loop on a bad runner config, leaving a zero-length wrapper and stale siblings](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-05-0207-sync-agents-aborts-mid-loop-on-a-bad-runner-config-leaving-a.md)**
<!-- docket:backlink:end -->

# Atomic Wrapper Generation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `sync-agents.sh` wrapper generation atomic — a bad `runner:` configuration is detected and reported in full before the first wrapper write, instead of aborting mid-loop and leaving a zero-length wrapper plus stale siblings.

**Architecture:** Extract the two existing `runner:` rules out of `emit_wrapper` into one shared predicate (`runner_config_error`), add a pre-flight gate (`validate_runner_config`) that walks every candidate (pass, agent, harness) triple and reports all offenders at once, and wire that gate into `main()` above `user_level_pass` and into the `--check` branch. `emit_wrapper`'s inline checks collapse to a single call to the predicate and keep `exit 1` as a can't-happen assertion for future call sites.

**Tech Stack:** POSIX-ish bash (bash 3.2 compatible), `sync-agents.sh` at the repo root, `tests/test_sync_agents.sh` (a flat `assert`-based script suite).

## Global Constraints

Copied verbatim from the spec and `AGENTS.md`; every task's requirements implicitly include this section.

- **Registration is checked before required-model.** An unregistered *and* model-less runner must keep reporting the registration failure — the more specific one. There is an explicit `ORDERING FENCE` test at `tests/test_sync_agents.sh:1529` pinning this.
- **The predicate owns the rule's scope.** `runner_config_error` returns "no error" for an empty runner and for non-`claude` harnesses, so **callers carry no knowledge of the rule's scope**. Do not narrow the gate's enumeration to `claude` up front.
- **Both per-offender diagnostic texts keep their current wording** (the 0205 message is pinned by tests), extended with the offending `<harness>/<agent>` pair.
- **The `flag_model` argument is the provenance-filtered model**, i.e. what `emit_wrapper` computes today: `[ "${RES_MODEL_FROM_USER:-0}" = "1" ] && flag_model="$2"`. A shipped `agents/harness-defaults.yml` default is not a user model (change 0168); `inherit` is docket's no-pin sentinel. Both must continue to fail the required-model rule.
- **Gate 3's placement is bounded on both sides:** below `resolve_global_agent_harnesses` (because `USER_TARGETS` is not computable until the post-migration `$GLOBAL_CFG` is read) and above `user_level_pass` (the first `mkdir -p` / `emit_wrapper` redirection is already a partial generation).
- Shell: never `producer | early-exiting-consumer` (`grep -q`, `head`) under `set -o pipefail` — capture into a variable, then `grep <<<"$var"`.
- Shell: `grep` for a pattern leading with `--` must declare it: `grep -qF -- "<pat>"`.
- Tests: a guard is code — mutation-test it or it is decoration. Run the **whole suite** at the build gate, never only the tests this plan enumerates.
- Cross-references in source anchor on a **symbol name** or a **verbatim-quoted clause**, never a line number.

## File Structure

| File | Responsibility | Change |
|---|---|---|
| `sync-agents.sh` | wrapper generation | Modify: add `runner_config_error` + `validate_runner_config`; collapse `emit_wrapper`'s two inline blocks; wire the gate into `main()` and `--check` |
| `tests/test_sync_agents.sh` | the suite pinning all of it | Modify: migrate the `! -s` assert to `! -e`; add the four new atomicity tests |
| `README.md` | user-facing runner docs | Modify: state the stricter all-or-nothing posture |

No new files. `sync-agents.sh` has no co-located `.md` contract (it lives at the repo root, outside the `scripts/<name>.md` family), so there is no contract file to update.

---

### Task 1: Extract the shared `runner_config_error` predicate

Behavior-preserving refactor. After this task the suite must be **exactly as green as before it** — no test changes, no new tests. The user-visible failure mode (mid-loop abort, zero-length wrapper) is still present; Task 2 removes it.

**Files:**
- Modify: `sync-agents.sh` — add `runner_config_error` immediately above `emit_wrapper` (currently line 832); rewrite the two check blocks inside `emit_wrapper` (currently lines 839-842 and 862-866)
- Test: `tests/test_sync_agents.sh` (run unchanged — it is the regression oracle for this task)

**Interfaces:**
- Consumes: `is_registered_runner` (line 88), `REGISTERED_RUNNERS` (line 87), `log`
- Produces: `runner_config_error <harness> <agent> <runner> <flag_model>` — writes one diagnostic line to **stdout** and returns 1 on error; returns 0 silently otherwise. Task 2's gate consumes exactly this signature.

Note the stdout choice: the predicate *returns* its diagnostic rather than logging it, so the gate can `log` each one with its own prefix while `emit_wrapper` logs it as an assertion. `log` writes to stderr, and `emit_wrapper`'s stdout is redirected into the wrapper file — a predicate that logged directly could not be reused by the gate, and one that wrote its diagnostic to stdout from inside `emit_wrapper` would inject the text into the generated file. Capture it into a variable at both call sites; never let it reach `emit_wrapper`'s stdout.

- [ ] **Step 1: Read the current `emit_wrapper` to confirm the anchors**

Run: `sed -n '826,870p' sync-agents.sh`

Confirm you see: the `if ! is_registered_runner "$runner"; then` block, the change-0168 provenance comment, the `local flag_model="" flag_effort=""` pair, and the `if [ -z "$flag_model" ] || [ "$flag_model" = "inherit" ]; then` block. If the line numbers have drifted, locate by content — the constraint above forbids anchoring on them.

- [ ] **Step 2: Add the `runner_config_error` predicate above `emit_wrapper`**

Insert immediately **above** the `# Emit either the native wrapper (via emit_for_harness …` comment block that heads `emit_wrapper`:

```sh
# The single source of truth for both `runner:` rules, their diagnostics, and their ORDER
# (registration before required-model). Emits ONE diagnostic on stdout and returns 1, or returns 0
# silently. Callers capture stdout — never let it reach emit_wrapper's stdout, which is redirected
# into the wrapper file.
#
# Scope lives HERE, not in callers: an empty runner and a non-claude harness both return "no error".
# `runner:` under a non-claude harness is currently reserved (warned and ignored, emitting native),
# which implies a future where that scope moves; keeping the test in one place means the gate and
# the assertion cannot drift apart when it does.
#
# $4 is the PROVENANCE-FILTERED model (change 0168): a shipped agents/harness-defaults.yml default
# is not a user model, so it must arrive here empty. `inherit` is docket's own no-pin sentinel —
# every adapter normalizes it to "no flag", so accepting it would leave a one-word bypass.
runner_config_error(){  # $1=harness $2=agent $3=runner $4=flag_model  (diagnostic on stdout)
  local harness="$1" agent="$2" runner="$3" flag_model="$4"
  [ -n "$runner" ] || return 0
  [ "$harness" = "claude" ] || return 0
  # Registration FIRST: an unregistered AND model-less runner must report the more specific
  # failure. tests/test_sync_agents.sh pins this with its "ORDERING FENCE" fixture.
  if ! is_registered_runner "$runner"; then
    printf '%s\n' "$harness/docket-$agent: runner '$runner' is not a registered runner (registered: $REGISTERED_RUNNERS)"
    return 1
  fi
  if [ -z "$flag_model" ] || [ "$flag_model" = "inherit" ]; then
    printf '%s\n' "$harness/docket-$agent: runner '$runner' requires an explicit model — add a 'model:' to the agents.$harness.$agent entry in a config layer, then re-run. docket never forwards its own shipped default to another harness (that ID means nothing to the child), so without one the run would silently use $runner's own default model, of unknown identity and cost."
    return 1
  fi
  return 0
}
```

Both messages keep their current wording. The only edit is the leading subject: `docket-$6:` becomes `$harness/docket-$agent:`, adding the harness because the gate now reports several offenders at once and is no longer speaking from inside one agent's generation. The existing tests assert on substrings (`grep -qF "docket-status"`, `grep -qF "$rnr"`, `grep -qiE "model"`), so the prefix change keeps them green.

- [ ] **Step 3: Collapse `emit_wrapper`'s two inline blocks to one predicate call**

Inside `emit_wrapper`, **delete** the registration block:

```sh
  if ! is_registered_runner "$runner"; then
    log "ERROR docket-$6: runner '$runner' is not a registered runner (registered: $REGISTERED_RUNNERS)"
    exit 1
  fi
```

and **delete** the required-model block (keeping the `local flag_model="" flag_effort=""` lines and both provenance assignments, which are still needed to build the shim):

```sh
  if [ -z "$flag_model" ] || [ "$flag_model" = "inherit" ]; then
    log "ERROR docket-$6: runner '$runner' requires an explicit model — …"
    exit 1
  fi
```

Then insert a single call **after** the two provenance assignments and **before** `emit_shim`:

```sh
  # Can't-happen assertion, not the user-facing mechanism: validate_runner_config gates every
  # triple before the first wrapper write. This covers a FUTURE call site added without that gate —
  # there are three today (user_level_pass, project_level_pass, check_project_level's leg (c)) and
  # nothing structurally prevents a fourth. Reaching it means the gate under-enumerated, so it dies
  # loudly rather than emitting a wrapper the config says must not exist.
  local rc_err
  if ! rc_err="$(runner_config_error "$5" "$6" "$runner" "$flag_model")"; then
    log "ERROR $rc_err"
    exit 1
  fi
```

Keep the change-0168 provenance comment where it is; keep the change-0205 comment but re-point its tail — replace the clause "and AFTER the registration check above so an unregistered runner still reports its own (more specific) failure" with "the ordering now lives in runner_config_error".

Update `emit_wrapper`'s own header comment: the sentences describing the two error rules should now say the rules live in `runner_config_error` and are gated up front by `validate_runner_config`.

- [ ] **Step 4: Run the sync-agents suites and confirm nothing changed**

Run: `bash tests/test_sync_agents.sh 2>&1 | grep -c "^ok -"; bash tests/test_sync_agents.sh 2>&1 | grep "NOT OK" || echo "ALL GREEN"`

Expected: `ALL GREEN`, and the ok-count matches what the same command reported before your edit. Also run the three adapter suites, which exercise the same generation path:

Run: `for t in tests/test_sync_agents_codex.sh tests/test_sync_agents_cursor.sh tests/test_sync_agents_opencode.sh; do echo "== $t"; bash "$t" 2>&1 | grep "NOT OK" || echo "green"; done`

Expected: `green` for all three.

- [ ] **Step 5: Mutation-test the ordering fence**

The `ORDERING FENCE` test is the one behavior most at risk in this extraction. Prove it still bites: temporarily swap the two `if` blocks inside `runner_config_error` (required-model first), run `bash tests/test_sync_agents.sh 2>&1 | grep "NOT OK"`, and confirm at least one assertion fails. Then **revert the swap** and re-confirm green.

Expected: red on the swap, green after reverting. If the swap stays green, the fence is decoration — stop and report it rather than proceeding.

- [ ] **Step 6: Commit**

```bash
git add sync-agents.sh
git commit -m "refactor(0207): extract the runner: rules into one shared predicate

Both runner: rules — registration (0079) and required-model (0205, ADR-0067) —
move out of emit_wrapper into runner_config_error, the single source of truth for
the rules, their diagnostics, their scope, and their ordering. emit_wrapper's
inline checks collapse to one call that keeps exit 1 as a can't-happen assertion.

Behavior-preserving: the diagnostics gain a <harness>/ subject prefix and nothing
else. The pre-flight gate that makes this useful lands next."
```

---

### Task 2: Add the `validate_runner_config` pre-flight gate and wire it in

This is the task that creates the invariant. It changes user-visible behavior, so its tests come first.

**Files:**
- Modify: `sync-agents.sh` — add `validate_runner_config` after `validate_user_agent_values` (which ends at line 606); wire into the `--check` branch (after the `validate_user_agent_values` leg, currently line 1338-1342) and into the real-run path (between `resolve_global_agent_harnesses` and `user_level_pass`, currently lines 1362-1363)
- Modify: `tests/test_sync_agents.sh` — migrate the `! -s` assert (line 1506) to `! -e`; append the new atomicity block
- Test: `tests/test_sync_agents.sh`

**Interfaces:**
- Consumes: `runner_config_error` (Task 1), `compute_user_targets` → `$USER_TARGETS`, `per_repo_opted_in`, `$HARNESSES`, `resolve_agent_layers <harness> <agent> <layer…>` → `$RES_RUNNER` / `$RES_MODEL` / `$RES_MODEL_FROM_USER`, `$AGENTS_SRC`, `short_name`, `log`
- Produces: `validate_runner_config` — returns 1 if any triple is bad, having `log`ged one line per offender; returns 0 silently otherwise. No arguments.

The gate mirrors both passes because they resolve differently:

| pass | harness list | layer set | precondition |
|---|---|---|---|
| user-level | `USER_TARGETS` | `[GLOBAL_CFG]` | — |
| project-level | `HARNESSES` | `[LOCAL_CFG, DOCKET_YML, GLOBAL_CFG]` | `per_repo_opted_in` |

- [ ] **Step 1: Write the failing tests**

Append this block to `tests/test_sync_agents.sh`, immediately after the `ORDERING FENCE` block (which ends with its `rm -rf "$SBX"`):

```bash
# ---- change 0207: wrapper generation is ATOMIC ----------------------------------------------
# A bad runner: config is detected before the FIRST wrapper write. Previously emit_wrapper failed
# inline, mid-loop, with its stdout already redirected into the target — so the offending agent was
# left zero-length and every agent later in glob order was never regenerated. The invariant now:
# a run either regenerates every wrapper or changes nothing on disk (nginx -t semantics).

# (1) FRESH tree + bad runner => NO wrapper files exist at all, for ANY agent.
mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agents:\n  claude:\n    status: { runner: codex }\n' > "$SBX/.docket.yml"
err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null )"; rc=$?
assert "0207: a bad runner config fails the run nonzero" '[ "$rc" != "0" ]'
# `! -e`, not `! -s`: change 0207 makes this a fail-BEFORE-write property. The offending agent's
# file is never created, so the zero-length-wrapper case the 0205 comment described is gone.
assert "0207: fresh tree — the offending agent has no wrapper at all" \
  '[ ! -e "$SBX/.claude/agents/docket-status.md" ]'
# The whole point: OTHER agents are not written either. Under the old mid-loop abort, every agent
# ahead of docket-status in glob order was already on disk by the time it failed.
assert "0207: fresh tree — NO wrapper was written for any agent" \
  '[ "$(find "$SBX/.claude/agents" -name "docket-*.md" 2>/dev/null | wc -l | tr -d " ")" = "0" ]'
assert "0207: the summary names the whole-run consequence" \
  'grep -qiE "no wrappers were written" <<<"$err"'
rm -rf "$SBX"

# (2) PRE-EXISTING wrappers + bad runner => every wrapper BYTE-IDENTICAL to before the run.
# This is the invariant the change exists to create and had no test before it.
mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agents:\n  claude:\n    status: { runner: codex, model: some/model-id }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "0207: (fixture) the good config generated wrappers to preserve" \
  '[ -s "$SBX/.claude/agents/docket-status.md" ]'
before="$(mktemp -d)"; cp -R "$SBX/.claude/agents/." "$before/"
# Now break it: drop the model: from the SAME entry.
printf 'agents:\n  claude:\n    status: { runner: codex }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 ); rc=$?
assert "0207: the run over pre-existing wrappers still fails nonzero" '[ "$rc" != "0" ]'
assert "0207: every pre-existing wrapper survives byte-untouched" \
  'diff -r "$before" "$SBX/.claude/agents" >/dev/null'
rm -rf "$SBX" "$before"

# (3) MULTIPLE offenders across different agents => all named in ONE run.
mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agents:\n  claude:\n    status: { runner: codex }\n    adr: { runner: gemini-cli }\n' > "$SBX/.docket.yml"
err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null )"; rc=$?
assert "0207: multiple offenders fail the run nonzero" '[ "$rc" != "0" ]'
assert "0207: the first offender is named" 'grep -qF "docket-status" <<<"$err"'
assert "0207: the SECOND offender is named in the same run" 'grep -qF "docket-adr" <<<"$err"'
# Accumulating, not short-circuiting: the unregistered one must report its OWN rule, not be
# swallowed by whichever offender the walk happened to reach first.
assert "0207: the unregistered offender reports the registration rule" \
  'grep -qF "gemini-cli" <<<"$err"'
rm -rf "$SBX"

# (4) --check reports the failure and exits nonzero (docket's `nginx -t`).
mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agents:\n  claude:\n    status: { runner: codex }\n' > "$SBX/.docket.yml"
err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check 2>&1 >/dev/null )"; rc=$?
assert "0207: --check fails on a bad runner config" '[ "$rc" != "0" ]'
assert "0207: --check says a real run would refuse to write wrappers" \
  'grep -qiE "would refuse to write wrappers" <<<"$err"'
assert "0207: --check wrote no wrappers" \
  '[ "$(find "$SBX/.claude/agents" -name "docket-*.md" 2>/dev/null | wc -l | tr -d " ")" = "0" ]'
rm -rf "$SBX"

# NON-VACUITY COMPANION for the whole 0207 block: the same shape with a VALID runner config must
# generate the full set. Without this, every assert above stays green if sync-agents.sh broke for
# an unrelated reason and wrote nothing at all.
mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agents:\n  claude:\n    status: { runner: codex, model: some/model-id }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 ); rc=$?
assert "0207: a VALID runner config still generates (the guards above are not vacuous)" '[ "$rc" = "0" ]'
assert "0207: and the full built-in set lands" \
  '[ "$(find "$SBX/.claude/agents" -name "docket-*.md" | wc -l | tr -d " ")" = "16" ]'
rm -rf "$SBX"
```

Two details worth understanding before you run it. Test (3) uses `adr` as the second offender because `agents/docket-adr.md` is a real built-in wrapper source — an entry naming no built-in is warned-and-ignored as a typo and would never reach the gate. Test (2)'s `diff -r` is the byte-identity check; it covers mode and content across the whole directory, which is stronger than comparing one file.

- [ ] **Step 2: Run the new tests to verify they fail**

Run: `bash tests/test_sync_agents.sh 2>&1 | grep "NOT OK"`

Expected: FAIL. Specifically you should see the fresh-tree "NO wrapper was written for any agent" assert fail (the old code writes the agents ahead of `docket-status` in glob order), the byte-untouched assert fail, the second-offender assert fail (the old code exits on the first), and both `--check` asserts fail (the `--check` branch has no runner leg yet). The "fails nonzero" asserts will already pass — the old code does fail, just too late.

Also expected to fail at this point: the migrated `! -e` assert from Step 3 if you do them in either order. Do Step 3 now.

- [ ] **Step 3: Migrate the 0205 `! -s` assert to `! -e`**

In `tests/test_sync_agents.sh`, find the 0205 block's assert (currently line 1506, inside the `for rnr in codex cursor opencode` loop). Replace this:

```bash
  # The error must not leave a USABLE shim behind. Note this is deliberately `! -s` (absent OR
  # empty), NOT `! -e`: emit_wrapper's call sites redirect into the target path, so the shell
  # creates and truncates the file BEFORE the function body runs and exits. The offending agent is
  # therefore left with a zero-length wrapper, which is inert — the harness has nothing to
  # dispatch — and is overwritten on the next successful run. Asserting `! -e` here would be
  # asserting a fail-before-write property this rule does not have.
  assert "0205/$rnr: no usable wrapper was written for the offending agent" \
    '[ ! -s "$SBX/.claude/agents/docket-status.md" ]'
```

with this:

```bash
  # `! -e`, not `! -s`: change 0207 gave this rule the fail-before-write property it lacked. The
  # gate runs above the first emit_wrapper redirection, so the offending agent's file is never
  # created rather than created-and-truncated.
  assert "0205/$rnr: no wrapper was written for the offending agent" \
    '[ ! -e "$SBX/.claude/agents/docket-status.md" ]'
```

The comment goes with the assert — it documented a property the change removes. This is the "ask what the block GUARDS, not what it asserts" rule: the block guards *no usable shim survives a rejected config*, which still holds and gets **stronger**, so it is narrowed rather than deleted.

- [ ] **Step 4: Write the `validate_runner_config` gate**

Insert into `sync-agents.sh` immediately **after** `validate_user_agent_values`'s closing `}` (currently line 606) and before the `# --- emit a resolved wrapper to stdout ---` banner:

```sh
# Gate 3 (change 0207): every `runner:` rule, checked across every candidate triple, BEFORE the
# first wrapper write. Wrapper generation is atomic — a run regenerates every wrapper or changes
# nothing on disk, so a configuration error leaves the previously generated wrappers in place on
# the assumption that what was already there was working (nginx -t / nginx -s reload).
#
# PLACEMENT IS BOUNDED ON BOTH SIDES. It must stay BELOW resolve_global_agent_harnesses —
# USER_TARGETS is not computable until the POST-migration $GLOBAL_CFG has been read, and any triple
# this gate fails to see trips emit_wrapper's assertion mid-loop, which is the original bug. It must
# stay ABOVE user_level_pass — the first `mkdir -p` or emit_wrapper redirection past this point is
# already a partial generation. (Gates 1 and 2 sit above migrate_legacy_global, so the three are
# deliberately not contiguous.)
#
# ACCUMULATES rather than short-circuits: one run names every offender, so the fix is a single edit
# and a re-run. It loops every agent x every harness and lets runner_config_error decide
# applicability — narrowing to `claude` here would put the rule's scope in a second place, and the
# day that scope moves this gate would silently under-enumerate.
validate_runner_config() {
  local rc=0 src name harness err
  for src in "$AGENTS_SRC"/docket-*.md; do
    [ -e "$src" ] || continue
    name="$(short_name "$src")"
    # user-level pass: USER_TARGETS resolved over the global layer only
    for harness in $USER_TARGETS; do
      resolve_agent_layers "$harness" "$name" "$GLOBAL_CFG"
      if ! err="$(runner_config_error "$harness" "$name" "$RES_RUNNER" "$(user_flag_model)")"; then
        log "ERROR $err"; rc=1
      fi
    done
    # project-level pass: HARNESSES resolved over local + committed + global
    per_repo_opted_in || continue
    for harness in $HARNESSES; do
      resolve_agent_layers "$harness" "$name" "$LOCAL_CFG" "$DOCKET_YML" "$GLOBAL_CFG"
      if ! err="$(runner_config_error "$harness" "$name" "$RES_RUNNER" "$(user_flag_model)")"; then
        log "ERROR $err"; rc=1
      fi
    done
  done
  return $rc
}
```

This needs one tiny helper so the provenance filter is not spelled twice. Add it directly **above** `runner_config_error` (from Task 1):

```sh
# The provenance-filtered model (change 0168), read from the RES_* globals resolve_agent_layers just
# set. ONLY a user-configured value may become a child-runner flag, so a shipped
# agents/harness-defaults.yml default must read as absent here. Spelled once: emit_wrapper and
# validate_runner_config must agree exactly, or the gate passes a triple the assertion then kills.
user_flag_model(){ [ "${RES_MODEL_FROM_USER:-0}" = "1" ] && printf '%s' "${RES_MODEL:-}"; return 0; }
```

Then, in `emit_wrapper`, leave the existing `[ "${RES_MODEL_FROM_USER:-0}" = "1" ] && flag_model="$2"` assignment alone — it builds the shim from the positional argument and is not the gate's concern. `$2` and `$RES_MODEL` are the same value at every call site (each site calls `resolve_agent_layers` for that (harness, agent) immediately before passing `$RES_MODEL`), which is exactly why the two spellings agree.

Note `per_repo_opted_in || continue` sits inside the per-agent loop rather than guarding the whole function: `continue` there skips only the project-level leg for that agent, preserving the user-level leg. Verify that reading before you move on — if you hoist it to an early `return 0`, the user-level leg stops running for a non-opted-in repo and the gate under-enumerates.

- [ ] **Step 5: Wire the gate into both paths**

In the `--check` branch, after the `validate_user_agent_values` leg (currently lines 1338-1342), add a third leg with wording matched to the other two:

```sh
    if ! validate_runner_config; then
      log "check: runner configuration is invalid — a real run would refuse to write wrappers."
      exit 1
    fi
```

This leg reads pre-migration config. That is the same asymmetry gates 1 and 2 already have, and it matches what `check_project_level`'s leg (c) drift loop itself resolves, so no gap opens between the gate and that loop's own `emit_wrapper` calls.

In the real-run path, insert between `resolve_global_agent_harnesses` and `user_level_pass` (currently lines 1362-1363):

```sh
  # Gate 3 — see validate_runner_config. Must stay BELOW resolve_global_agent_harnesses (USER_TARGETS
  # needs the post-migration $GLOBAL_CFG) and ABOVE user_level_pass (the first mkdir -p or
  # emit_wrapper redirection past this point is already a partial generation).
  if ! validate_runner_config; then
    log "ERROR runner configuration is invalid — no wrappers were written."
    exit 1
  fi
```

One thing to check while you are here: the `--check` branch calls `validate_runner_config` before `compute_user_targets` has ever run, so `$USER_TARGETS` may be unset under `set -u`. Confirm how the script sets it — `compute_user_targets` is called from inside `user_level_pass` (line 1096). If `USER_TARGETS` is not initialized at declaration time, either call `compute_user_targets` at the top of `validate_runner_config` (it is idempotent and only reads config) or reference it as `${USER_TARGETS:-}`. Prefer calling `compute_user_targets` — an empty list would silently skip the whole user-level leg, and a skipped leg is the under-enumeration this gate exists to prevent. Whichever you choose, test (4) is what proves it.

- [ ] **Step 6: Run the tests to verify they pass**

Run: `bash tests/test_sync_agents.sh 2>&1 | grep "NOT OK" || echo "ALL GREEN"`

Expected: `ALL GREEN`, including all four new 0207 blocks, the non-vacuity companion, the migrated `! -e` assert, and every pre-existing 0079 / 0205 assert.

- [ ] **Step 7: Mutation-test the new gate**

Two mutations, each reverted immediately after:

1. Comment out the `validate_runner_config` call in the **real-run** path. Run the suite. Expected: the fresh-tree and byte-untouched asserts go red. (If they stay green, the tests are not reaching the gate.)
2. Change the gate's accumulate to a short-circuit — `return 1` instead of `rc=1` on the first offender. Run the suite. Expected: test (3)'s second-offender assert goes red.

Run after each: `bash tests/test_sync_agents.sh 2>&1 | grep "NOT OK"`

Expected: red on each mutation, `ALL GREEN` after reverting both. A mutation that leaves the suite green is a defect — stop and report it.

- [ ] **Step 8: Measure the gate's cost**

The spec asks you to measure rather than assume. The gate adds roughly 16 agents x ~4 harnesses x 2 passes of `resolve_agent_layers`; `prime_layer_body` caches layer bodies and `check_project_level`'s leg (c) already runs a comparable loop plus a `diff` per file, so it should be invisible.

```bash
cd "$(mktemp -d)" && git init --quiet && git config user.email t@t.test && git config user.name Test
mkdir -p .claude
printf 'agents:\n  claude:\n    status: { model: opus-5, effort: high }\n' > .docket.yml
time ( DOCKET_HARNESS_ROOT="$PWD" bash /path/to/sync-agents.sh >/dev/null 2>&1 )
```

Run it against your branch and against `git stash`-ed / `origin/main` state for comparison. Record both numbers in the commit message. Narrowing the enumeration is the **fallback, not the design** — only do it if the measurement actually bites (a human-noticeable regression, not a few milliseconds), and if you do, say so explicitly in the commit message so review sees it.

- [ ] **Step 9: Update the README's runner rules**

In `README.md`, the runner rules list (around lines 800-812) documents both rules but not the new all-or-nothing posture. Extend the unregistered-runner bullet:

```markdown
- `runner:` under a non-`claude` harness key is reserved and warned-and-ignored; an
  unregistered runner name fails generation loudly.
- **Generation is all-or-nothing.** Any bad `runner:` entry is detected before the first wrapper is
  written, and the run refuses rather than regenerating some wrappers and not others: one bad entry
  refreshes *zero* wrappers, and previously generated ones survive untouched. The diagnostic names
  every offender in one pass, so the fix is a single edit and a re-run. `sync-agents.sh --check`
  reports the same failure without writing anything.
```

- [ ] **Step 10: Run the whole suite**

Not only the tests this plan enumerates — the `AGENTS.md` rule exists because a change like this surfaces in unrelated-looking suites.

Run: `for t in tests/test_*.sh; do out="$(bash "$t" 2>&1)"; if grep -q "NOT OK" <<<"$out"; then echo "== FAIL $t"; grep "NOT OK" <<<"$out"; fi; done; echo "SUITE DONE"`

Expected: `SUITE DONE` with no `FAIL` lines.

- [ ] **Step 11: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents.sh README.md
git commit -m "fix(0207): gate runner config before the first wrapper write

sync-agents.sh failed a bad runner: config inline, mid-loop, with emit_wrapper's
stdout already redirected into the target path — so the offending agent was left
zero-length and every agent later in glob order was never regenerated. A failure
during user_level_pass meant project_level_pass never ran at all.

validate_runner_config walks every candidate (pass, agent, harness) triple, reports
ALL offenders in one run, and fails above user_level_pass. Generation is now atomic:
a run regenerates every wrapper or changes nothing on disk. --check gets the same leg.

Deliberately stricter: one bad entry refreshes zero wrappers. The alternative was not
more wrappers but an undetectable mixture of fresh, stale, and zero-length ones.

The 0205 '! -s' assert becomes '! -e' — the fail-before-write property that rule
lacked now exists."
```

---

## Self-Review

**1. Spec coverage.**

| Spec section | Task |
|---|---|
| `runner_config_error` predicate, scope in one place, registration-first ordering | Task 1, Steps 2-3, 5 |
| `flag_model` provenance filter (`RES_MODEL_FROM_USER`, `inherit`) | Task 1 Step 2 + Task 2 Step 4 (`user_flag_model`, spelled once) |
| `emit_wrapper` collapses to a can't-happen assertion keeping `exit 1` | Task 1 Step 3 |
| `validate_runner_config`, accumulating, mirroring both passes | Task 2 Step 4 |
| Placement: below `resolve_global_agent_harnesses`, above `user_level_pass`, with a bounding comment | Task 2 Steps 4-5 |
| `--check` leg, wording matched | Task 2 Step 5 |
| Summary diagnostic from `main` | Task 2 Step 5 |
| Per-offender messages keep text, gain `<harness>/<agent>` | Task 1 Step 2 |
| New test: fresh tree, no wrappers at all | Task 2 Step 1 test (1) |
| New test: pre-existing wrappers byte-identical | Task 2 Step 1 test (2) |
| New test: multiple offenders in one run | Task 2 Step 1 test (3) |
| New test: `--check` nonzero | Task 2 Step 1 test (4) |
| Migrated: `! -s` → `! -e`, comment goes with it | Task 2 Step 3 |
| Unchanged 0079 / 0205 tests keep passing | Task 1 Step 4, Task 2 Steps 6, 10 |
| Performance measured, not assumed; narrowing is the fallback | Task 2 Step 8 |
| Out of scope: required-model rule itself; gate 2's pre-migration blind spot | Not planned — correct |

No gaps.

**2. Placeholder scan.** No TBDs, no "add error handling", no "similar to Task N". Every code step carries the actual content. The one judgment call deliberately left to the implementer — `compute_user_targets` vs `${USER_TARGETS:-}` in Task 2 Step 5 — states the preferred answer, the reason, and the test that decides it.

**3. Type consistency.** `runner_config_error <harness> <agent> <runner> <flag_model>` is defined in Task 1 Step 2 and called with exactly four arguments in Task 1 Step 3 (`"$5" "$6" "$runner" "$flag_model"`) and Task 2 Step 4 (`"$harness" "$name" "$RES_RUNNER" "$(user_flag_model)"`). `user_flag_model` takes no arguments and reads `RES_MODEL_FROM_USER` / `RES_MODEL`, both set by `resolve_agent_layers`, which every call site invokes immediately beforehand. `validate_runner_config` takes no arguments and returns 0/1; both call sites test it with `if ! …; then`. Consistent.
