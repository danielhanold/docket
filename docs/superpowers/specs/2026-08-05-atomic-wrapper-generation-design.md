<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0207 — sync-agents aborts mid-loop on a bad runner config, leaving a zero-length wrapper and stale siblings](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0207-sync-agents-aborts-mid-loop-on-a-bad-runner-config-leaving-a.md)**
<!-- docket:backlink:end -->

# Atomic wrapper generation — a pre-flight gate for runner config

Design doc for change 0207.

## Problem

`sync-agents.sh` generates one wrapper per (harness, agent) pair through a redirected function
call:

```sh
emit_wrapper "$src" "$RES_MODEL" "$RES_EFFORT" "$RES_RUNNER" "$harness" "$name" > "$dir/docket-$name.$(harness_ext "$harness")"
```

`emit_wrapper` enforces two rules about `runner:` inline, and fails each with `log ERROR` followed
by `exit 1`:

1. the runner names a registered adapter (change 0079);
2. a delegated agent carries a USER-configured model (change 0205, ADR-0067).

Both failure modes are structurally wrong at that position, for two independent reasons:

- **The redirect has already truncated the target.** The shell opens `> "$dir/…"` before the
  function body runs, so the offending agent is left as a **zero-length wrapper** — a file that
  exists, is picked up by the harness, and contains nothing.
- **`exit 1` kills the script mid-loop.** Every agent later in glob order is never regenerated, and
  a failure during `user_level_pass` means `project_level_pass` never runs at all. The user is left
  with a directory that is part fresh, part stale, part empty, with no statement that generation
  stopped early.

The mechanism is old — the registration check has always worked this way — but change 0205 raised
its trip rate sharply. A model-less `runner:` was a documented supported configuration until that
change, and docket cannot reach or migrate the config layers that may carry one (a machine-local
`.docket.local.yml`, or the global `~/.config/docket/config.yml`). 0205 accepted this deliberately
rather than expand its own scope; its tests encode `! -s` (absent **or** empty) precisely because
the zero-length file is expected today.

## The invariant

**Wrapper generation is atomic.** A run either regenerates every wrapper or changes nothing on
disk. A configuration error is detected before the first write and reported in full; previously
generated wrappers survive byte-untouched.

The model is `nginx -t` and `nginx -s reload`: the whole configuration is validated as a set, and a
failed validation leaves the running configuration in place — on the assumption that what was
already there was working. `sync-agents.sh --check` is docket's `nginx -t`.

This is deliberately **stricter** than today's behavior, and the cost is real: a user with one bad
runner entry gets *zero* wrappers refreshed until it is fixed, and on a fresh machine that means no
wrappers at all. That is the correct trade. Today's alternative is not "more wrappers" but "an
unknown mixture of fresh, stale, and empty wrappers, silently". The diagnostic names every offender
in one pass, so the fix is a single edit and a re-run — not a flag.

An escape hatch (a flag downgrading runner errors to warnings and emitting the offenders natively)
was considered and rejected: it adds a knob, a second code path, and a way to end up with an agent
silently *not* delegated when the config says it should be.

### This is not a new mechanism

`sync-agents.sh` already gates this way, twice, and documents the posture in its own comments:

- `validate_harness_defaults` — *"Validate BEFORE writing any wrapper, so a malformed file cannot
  leave a half-regenerated agent directory behind."*
- `validate_user_agent_values` (change 0173) — *"Collect every offender across every layer, report
  them all, and fail BEFORE any wrapper is written: partial generation carrying a known-bad pin is
  exactly the harm this exists to prevent."* And: *"This must stay ABOVE
  `migrate_legacy_global`/`user_level_pass` — the first `mkdir -p` or `emit_wrapper` redirection
  past this point is already a partial generation."*

The runner checks are a **third leg of that existing gate that was never migrated into it**. The
curated learning [`validate-the-whole-input-set-first`](../../changes/learnings/validate-the-whole-input-set-first.md)
(from change 0185) states the same rule at the batch level.

The rejected alternative — accumulate a failure flag inside `emit_wrapper` and exit nonzero after
both passes complete, as change 0207's stub originally suggested — would have left the file holding
two contradictory postures at once, and still shipped a mixed-state agent directory.

## Components

### `runner_config_error <harness> <agent> <runner> <flag_model>`

New shared predicate; the single source of truth for both rules and both diagnostics. Emits the
diagnostic on stdout and returns nonzero, or returns 0 silently.

It returns "no error" for an empty runner and for non-`claude` harnesses, so **callers carry no
knowledge of the rule's scope**. That matters: `runner:` under a non-claude harness is currently
"reserved" (warned and ignored, emitting native), which implies a future where the scope moves. The
scope lives in exactly one place.

**Registration is checked before required-model.** `tests/test_sync_agents.sh:1498` pins that an
unregistered *and* model-less runner reports the registration failure — the more specific one.
Folding both rules into one function must not lose that order.

The `flag_model` argument is the **provenance-filtered** model, i.e. what `emit_wrapper` computes
as `flag_model` today:

```sh
[ "${RES_MODEL_FROM_USER:-0}" = "1" ] && flag_model="$2"
```

A shipped `agents/harness-defaults.yml` default is not a user model (change 0168), and `inherit` is
docket's own no-pin sentinel — both must continue to fail the required-model rule.

### `validate_runner_config`

New gate function. Accumulates rather than short-circuits: it walks every candidate triple, calls
the predicate, logs each diagnostic, and returns nonzero if any fired.

It mirrors both passes, because they resolve differently:

| pass | harness list | layer set | precondition |
|---|---|---|---|
| user-level | `USER_TARGETS` | `[GLOBAL_CFG]` | — |
| project-level | `HARNESSES` | `[LOCAL_CFG, DOCKET_YML, GLOBAL_CFG]` | `per_repo_opted_in` |

For each pass it loops **every agent × every harness** and lets the predicate decide applicability.
It does not narrow to `claude` up front: narrowing would put the rule's scope in a second place, and
the day that scope moves the gate would silently under-enumerate — which reintroduces the mid-loop
abort this change exists to remove.

A structural refactor extracting a shared triple-iterator used by the gate and both passes was
considered and rejected as premature: it is a real refactor of two working passes plus
`check_project_level`'s leg (c), inventing an abstraction for a two-consumer problem.

### `emit_wrapper`

Its two inline check blocks collapse to a single call to the predicate, **keeping `exit 1`**. This
is now a can't-happen assertion, not the user-facing mechanism: it covers a future call site added
without the gate. There are three call sites today (`user_level_pass`, `project_level_pass`, and
`check_project_level`'s leg (c) drift loop), and nothing structurally prevents a fourth.

## Placement

**Real-run path** — the gate sits after `migrate_legacy_global` and `resolve_global_agent_harnesses`,
immediately above `user_level_pass`:

```
resolve_agent_harnesses
  [--check branch → exit]
validate_harness_defaults            gate 1
validate_user_agent_values           gate 2
migrate_legacy_global                writes $GLOBAL_CFG, renames agents.yaml
resolve_global_agent_harnesses
validate_runner_config               gate 3   ← NEW
user_level_pass                      first wrapper write
…
```

`USER_TARGETS` is not computable until `resolve_global_agent_harnesses` has read the
**post-migration** `$GLOBAL_CFG`, and enumeration accuracy is load-bearing: any triple the gate
fails to see trips `emit_wrapper`'s assertion mid-loop, which is the original bug. Gating after the
migration still honours the invariant — `migrate_legacy_global` writes *config*, not wrappers.

The consequence is that the three gates are no longer contiguous. Gate 3 carries a comment
mirroring the existing "must stay ABOVE the first `mkdir -p` or `emit_wrapper` redirection" note,
stating both its lower bound (below `resolve_global_agent_harnesses`) and its upper bound (above
`user_level_pass`).

Hoisting `migrate_legacy_global` above all three gates was considered — it would make them
contiguous and would incidentally close gate 2's pre-migration blind spot — but it drags a *write*
above every gate, and on the `--check` path it would make a read-only command perform a migration.
Rejected.

**`--check` path** — its own leg beside the other two gates, wording matched to theirs:

```
check: runner configuration is invalid — a real run would refuse to write wrappers.
```

It reads pre-migration config. That is the same asymmetry gates 1 and 2 already have, and it
matches what `check_project_level`'s leg (c) drift loop itself resolves, so no gap opens between
the gate and that loop's own `emit_wrapper` calls.

## Diagnostics

Follows the established gate shape exactly: one `log` line per offender from inside the gate, then
a single summary line from `main`:

```
ERROR runner configuration is invalid — no wrappers were written.
```

Both per-offender messages keep their current text (the 0205 diagnostic is a long, deliberately
explanatory paragraph and is pinned by tests), extended with the offending
`<harness>/<agent>` pair, since the gate now reports several at once and is no longer speaking from
inside a single agent's generation.

## Testing

New:

- Bad runner config on a **fresh** tree → **no wrapper files exist at all**, for any agent.
- Bad runner config over **pre-existing** wrappers → every wrapper byte-identical to before the run.
  This is the invariant the change exists to create and has no test today.
- **Multiple** offenders across different agents → all named in a single run.
- `--check` reports the failure and exits nonzero.

Migrated:

- `tests/test_sync_agents.sh:1462-1469` asserts `[ ! -s "$SBX/.claude/agents/docket-status.md" ]`
  (absent **or** empty). Under the new invariant the file is never created, so this becomes `! -e`,
  and the comment explaining why `-s` was chosen goes with it.

Unchanged and must keep passing:

- 0079: unregistered runner fails generation nonzero; the diagnostic names it.
- 0205: model-less runner fails nonzero across all three registered runners; `model: inherit` does
  not satisfy the rule; a reserved (non-claude) runner does not trip it and still emits native.
- 0205: an unregistered **and** model-less runner reports the **registration** failure first.

## Performance

The gate adds roughly 16 agents × ~4 harnesses × 2 passes of `resolve_agent_layers`.
`prime_layer_body` caches layer bodies, and `check_project_level`'s leg (c) already runs a
comparable loop over agents × harnesses plus a `diff` per file, so this should be invisible.

The plan should **measure it rather than assume**, and narrow the enumeration only if it actually
bites — narrowing is the fallback, not the design.

## Out of scope

- The required-model rule itself and its runner-wide scope (ADR-0067, settled).
- Gate 2's pre-migration blind spot: `validate_user_agent_values` validates `$GLOBAL_CFG` before
  `migrate_legacy_global` runs, so an offender arriving via the legacy-`agents.yaml` migration is
  not seen by it. Real and pre-existing; only fixable by hoisting a write above every gate, which
  would break `--check`'s read-only property. Noted, not fixed.

## Dependencies

**Depends on change 0205** (`feat/opencode-runner-adapter`, PR #156, status `implemented`). The
required-model rule this change restructures exists only on that branch; building against `main`
would rewrite `emit_wrapper` underneath it.

Related: change 0206 (delegated runner runs anchored at the wrong worktree) also touches runner
delegation, in `runner-dispatch.sh` rather than `sync-agents.sh`. No collision expected.
