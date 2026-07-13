# Encode the disabled board positively — an empty surfaces value is a wiring bug, not a configuration

**Change:** 0071
**Date:** 2026-07-13
**Depends on:** 0072 (facade skill rewiring), which depends on 0068 (docket command facade)

## Context

Every docket skill's Board pass invokes `board-refresh.sh --changes-dir … --surfaces "$BOARD_SURFACES"`.
`$BOARD_SURFACES` comes from the Step-0 config `eval`, but shell state does not survive between an
agent's Bash tool calls — so by the time the Board pass runs, the variable is typically **unset** and
the command sends `--surfaces ""`.

An empty value is also how a repo that sets `board_surfaces: []` legitimately says "no board."
`docket-config.sh` maps `[]` to exactly `BOARD_SURFACES=""`, so at the script boundary a **disabled
repo and a mis-wired caller are byte-identical**. `board-refresh.sh` prints `inline disabled — no-op`
and exits 0; the caller's `git status --porcelain -- BOARD.md` is then empty, which the skill prose
explicitly licenses as "a genuine no-op," so it commits nothing and proceeds believing the board is
current. The board silently goes stale with a success exit code the whole way down. This was hit live
while filing change 0070.

Change 0059 defended the *adjacent* hole — a caller who **forgets the flag** exits 2, and
`board-refresh.sh` tracks `SURFACES_SET` precisely so the two cases stay distinct. What it did not
anticipate is a *present flag carrying an unresolved variable*, which lands in the legitimate-empty
branch. This is the same defect class ADR-0028 and change 0069 eliminated on the report channel: an
exit-0 no-op indistinguishable from success.

### What changes 0068 and 0072 already fix — and what survives

Change 0068 (in-progress) diagnosed this root cause independently and retires the pattern outright:

> `eval "$(docket-config.sh --export)"` stores resolved config in the agent's **shell**, and
> shell-state persistence across tool calls is harness-dependent — Claude Code keeps none. The model's
> context window is the only state guaranteed to persist across tool calls in every harness.

0068 ships the `docket.sh` facade whose `env`/`preflight` ops **print** resolved `KEY=value` lines to
stdout; 0072 rewires the skills and the Step-0 preamble to read those values and interpolate them as
literals. Together they remove the **trigger**: after 0072, prose carries `--surfaces inline` as a
literal, and the unresolved-variable failure cannot occur.

Three things survive them, and they are this change's entire scope:

1. **The encoding remains ambiguous.** `board-refresh.sh` still cannot distinguish "this repo disabled
   the board" from "the caller passed me nothing." A future call site that hand-passes an empty value —
   or an agent that misreads the env block — still gets a silent exit-0 no-op. 0072 makes the mistake
   *unlikely*; it does not make it *detectable*.
2. **Six duplicated Board-pass prose blocks remain.** 0072 rewires call sites to the facade without
   consolidating them, so six hand-rolled copies of `git add`/`commit`/`push`/rebase-retry persist.
3. **`board_surfaces: []` remains the sole legitimate empty**, keeping the defect class permanently
   latent.

This change therefore removes the **ambiguity that made the trigger silent**, so that the next
mis-wiring — whatever its cause — fails loudly instead of no-opping.

## Decision

Three parts: a positive sentinel, a consolidated Board pass, and a structural guard.

### 1. The positive sentinel

**`docket-config.sh` never emits an empty `BOARD_SURFACES`.** `board_surfaces: []` resolves to the
reserved token `none`. Empty therefore has exactly one meaning left: *nobody resolved this*.

**`board-refresh.sh`:**

- `--surfaces` stays **required as a flag** — change 0059's missing-flag exit-2 guard is untouched.
- An **empty value now exits 2** (was: no-op, exit 0). This is the reversal at the heart of the change.
- The **`none` token** takes over the deliberate-disable path: no-op, exit 0, `BOARD.md` never created,
  never truncated. Preserving that is non-negotiable — it is change 0059's entire point — and the
  existing truncation-trap regression test carries over unchanged, keyed on `none` instead of `""`.
- `none` is **reserved and exclusive**: `--surfaces "none inline"` is a contradiction and exits 2
  rather than silently picking a winner.
- Unknown tokens still warn-and-ignore (unchanged) — a typo must never abort a build.

**`docket-status.sh`:** `board_pass` gains a `none` arm mapping to its existing `board off` report line,
so a disabled repo's output stays byte-identical to today. Its current guard
(`[ -n "$surfaces" ] || { echo "board off"; return 0; }`) becomes an assertion that the resolver
produced a value at all.

This **inverts the polarity of the bug**. Today, failing to resolve config produces the *disabled*
behavior. After this, it produces a *loud error* — and disabling the board requires positively saying so.

### 2. One Board pass

The six duplicated Board-pass prose blocks collapse into a single facade call:

```
docket.sh docket-status --board-only
```

The orchestrator already self-resolves config (fail-closed), checks the bootstrap verdict, syncs the
metadata worktree, gates on surfaces, renders through `board-refresh.sh`, and commits and pushes
`BOARD.md` with its own rebase-retry loop. It is immune to this bug precisely *because* it owns its
config resolution — this change generalizes that property to every caller.

The consequence worth stating plainly: **no surfaces value crosses the skill/script boundary anymore.**
The sentinel in part 1 then exists purely to defend script-to-script and future callers — belt to the
consolidation's suspenders.

Two behavioral notes:

- **`docket-new-change`'s board commit splits out** from its change+spec commit (today it commits them
  together). This aligns it with the separate-board-commit rule every other skill already follows —
  the convention mandates a separate board commit to keep the claim CAS byte-identical across
  concurrent agents, and `docket-new-change` was the outlier.
- **Must-land vs best-effort keys on stdout, not exit code.** `board_pass_inline` reports its outcome
  on stdout (`board off` · `board inline clean` · `board inline changed pushed` ·
  `board inline changed push-failed`) and always exits 0 — the self-evidencing report style ADR-0028
  established. Must-land skills (`docket-new-change`, `docket-groom-next`, `docket-finalize-change`,
  `docket-auto-groom`) re-invoke the facade until they see `board inline changed pushed` (or a clean/off
  line); `docket-implement-next` stays best-effort and logs whatever it gets.

### 3. Guards

- **Structural sentinel, grep-derived and mutation-tested** (LEARNINGS #64: derive gated call-site lists
  by grep, never by hand; guard completeness with a sentinel, not review attention). No skill or
  reference prose may contain `$BOARD_SURFACES`, `--surfaces`, or a direct `board-refresh` invocation.
  The facade's `docket-status --board-only` is the only permitted Board-pass spelling in prose.
- **`docket-config.sh` never emits an empty `BOARD_SURFACES`** — asserted directly, including for
  `board_surfaces: []` and for every layer combination that resolves to an empty list.
- **`board-refresh.sh`**: exit 2 on `--surfaces ""`; exit 2 on `--surfaces "none inline"`; no-op exit 0
  on `--surfaces "none"`; truncation-trap regression (a pre-existing `BOARD.md` + `--surfaces none`
  leaves the file byte-identical).

### 4. ADR

One new ADR. This genuinely reverses a decision change 0059 documented — *"an explicit empty value means
'no surfaces configured'"* — so the reversal must be recorded rather than silently edited into the
contract.

The rule it records is durable and generalizes well past the board:

> **A deliberate off-state must be encoded positively. Absence and emptiness are reserved for error.**

That is the same family as ADR-0028's *"silence is not evidence"*, and it is the rule that would have
caught this bug on the day 0059 shipped. `relates_to: [28]`.

## Out of scope

- **Changing `board_surfaces: []` semantics.** A repo that disables the board keeps a no-op that never
  truncates a prior `BOARD.md`. Only its *encoding* changes (`""` → `none`).
- **The `$SKILL_*` family** (`SKILL_BRAINSTORM`, `SKILL_PLAN`, `SKILL_BUILD`, `SKILL_REVIEW`,
  `SKILL_FINISH`) — the same unresolved-variable hazard on the model-consumed side. **Change 0072 owns
  this**: it rewrites the Step-0 preamble to read printed values and interpolate them as literals.
  Deliberately not duplicated here.
- **The Step-0 preamble rewrite and the `eval` retirement** — 0068 and 0072.
- **The `github` surface and `github-mirror.sh`.** Same caller pattern, but that surface is
  best-effort by design; this change scopes to the `inline` write decision.
- **Retrofitting stale boards.** Any board left stale by this bug self-heals at the next correctly
  wired Board pass.

## Risks

- **The exit-2-on-empty reversal could break an unfound caller.** Audited: the only callers of
  `board-refresh.sh` are (a) `docket-status.sh`, which passes the literal `inline`, and (b) the six
  skill-prose sites, which this change deletes. No third caller exists. The build must re-derive this
  list by grep rather than trusting this spec.
- **Sequencing.** This change must land *after* 0072, or the two will rewrite the same skill prose with
  different designs. `depends_on: [72]` enforces it; the implementer's reconcile pass re-validates.
- **Token collision.** `none` must never be a real surface name. It is not, and the reserved-and-
  exclusive rule makes any future collision an immediate exit 2 rather than a silent misread.

## Success criteria

- A caller that passes no resolved surfaces value gets a **non-zero exit and a message**, never a
  silent no-op.
- A repo with `board_surfaces: []` renders and commits nothing, and its pre-existing `BOARD.md` (if any)
  is left byte-identical — verified by the carried-over truncation-trap test.
- No skill or reference prose mentions `$BOARD_SURFACES` or `--surfaces`; the sentinel reddens if any
  does, and the sentinel itself reddens when mutated.
- A disabled repo's `docket-status` stdout is byte-identical to today (`board off`).
