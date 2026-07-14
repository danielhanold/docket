# Encode the disabled board positively — an empty surfaces value is a wiring bug, not a configuration

**Change:** 0071
**Date:** 2026-07-13
**Reconciled:** 2026-07-14 — against merged `main` (0068 · 0070 · 0072 all landed). The
*"what 0068 and 0072 already fix"* section below asserted a prediction that **did not come true**;
it has been rewritten against the code. See *What actually landed*.
**Depends on:** 0072 (facade skill rewiring, **merged** — PR #79), which depends on 0068 (docket
command facade, **merged** — PR #78)

## Context

Every docket skill's Board pass invokes the board writer with
`--changes-dir … --surfaces "$BOARD_SURFACES"`. `$BOARD_SURFACES` is a **shell variable**, but shell
state does not survive between an agent's Bash tool calls — so by the time the Board pass runs, the
variable is typically **unset** and the command sends `--surfaces ""`.

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

### What actually landed in 0068 / 0072 — and what survives

This spec was written on 2026-07-13, while 0068 and 0072 were still in flight, and it *predicted*
what they would do. That prediction was **wrong in its central claim**, and the correction matters:
it makes this change more necessary, not less.

**What the spec predicted:** *"0072 rewires the skills and the Step-0 preamble to read those values
and interpolate them as literals. Together they remove the trigger: after 0072, prose carries
`--surfaces inline` as a literal, and the unresolved-variable failure cannot occur."*

**What is on `main`:** 0068 shipped the `docket.sh` facade, whose `env`/`preflight` ops **print**
resolved `KEY=value` lines to stdout (`scripts/docket.sh:42-46`), and 0072 rewrote the Step-0
preamble to say — *"read the printed `KEY=value` block off stdout and carry those values forward as
**literals** in later commands (no `eval`, no `source`)"* (`skills/docket-convention/SKILL.md:73`).
The `eval` is genuinely retired. **But the call-site prose was never re-spelled.** Every Board-pass
block on `main` still literally reads:

```
docket.sh board-refresh --changes-dir .docket/<changes_dir> --surfaces "$BOARD_SURFACES"
```

Grep-derived from `origin/main` (2026-07-14): **8 call sites across 6 files** — `docket-auto-groom`
(1), `docket-groom-next` (1), `docket-implement-next` (1), `docket-finalize-change` (1),
`docket-new-change` (3), `docket-convention/references/terminal-close-out.md` (1). **Zero** of them
carry a literal. The count of literal `--surfaces` spellings in skill prose is **0**.

**Therefore the trigger survives.** 0072 replaced a mechanism that *guaranteed* the variable was
unset (`eval` into a dead shell) with an instruction the agent must *remember to apply* against prose
that still shows `"$BOARD_SURFACES"`. An agent that copies the Board-pass line verbatim into a fresh
Bash call — the obvious reading, since the prose is a literal command block — still sends
`--surfaces ""` and still gets a silent exit-0 no-op. The failure is now *less likely* and *no more
detectable*.

Four things survive, and they are this change's scope:

1. **The trigger itself.** Not removed, only made contingent on agent discipline. Re-verified on
   `main`: 8 variable-spelled call sites, 0 literal ones.
2. **The encoding remains ambiguous.** `board-refresh.sh` still cannot distinguish "this repo disabled
   the board" from "the caller passed me nothing." Re-verified on `main`: `docket-config.sh:201` maps
   `[]` → `""`; `board-refresh.sh:56-59` takes an empty value into `inline disabled — no-op`, exit 0;
   `SURFACES_SET` (`board-refresh.sh:27,41-43`) still distinguishes only the *missing flag*.
3. **The duplicated Board-pass prose blocks remain.** 0072 rewired the call sites to the facade
   without consolidating them, so the hand-rolled copies of `git add`/`commit`/`push`/rebase-retry
   persist across the 6 files above. (The original spec said "six blocks"; the grep says 8 sites in
   6 files.)
4. **`board_surfaces: []` remains the sole legitimate empty**, keeping the defect class permanently
   latent. Re-verified: `tests/test_docket_config.sh:107` asserts `board []: BOARD_SURFACES empty`.

This change therefore removes the **ambiguity that makes the trigger silent**, so that the next
mis-wiring — whatever its cause — fails loudly instead of no-opping. Given that the trigger is still
live on `main`, the sentinel in part 1 is now load-bearing rather than defence-in-depth: it is what
turns the surviving bug from a silent stale board into an exit 2.

### Build-time reconcile (2026-07-14, `docket-implement-next` step 3)

Re-derived by grep against `origin/main` @ `edb37f9` — **not** trusted from the prose above. Every
claim in this spec re-verified; three findings the spec did not have.

**Confirmed exactly as written.** 8 `--surfaces` call sites across 6 files — `docket-auto-groom`
(1, L56), `docket-convention/references/terminal-close-out.md` (1, L82), `docket-finalize-change`
(1, L55), `docket-groom-next` (1, L71), `docket-implement-next` (1, L86), `docket-new-change`
(3, L43/51/59). Zero literal spellings. `docket-config.sh:198` maps unset → `inline`, `:201` maps
`[]` → `""`. `board-refresh.sh:41-43` exits 2 on the missing flag, `:56-59` takes an empty value
into `inline disabled — no-op` exit 0. `docket-status.sh:46` still reads
`[ -n "$surfaces" ] || { echo "board off"; return 0; }`. The trigger survives 0072.

**New — part 2 is smaller than the spec assumed.** `docket-status.sh` already parses
`--board-only` (`scripts/docket-status.sh:31`), and `docket-status` is already a facade op
(`scripts/docket.sh` `WRAPPED_OPS`; `scripts/docket.md` inventory row). So
`docket.sh docket-status --board-only` is **callable today** — part 2 is a prose rewiring plus the
must-land/best-effort report-line contract, not new script surface. Also:
`board_pass_inline` already passes a **literal** `--surfaces inline` to `board-refresh.sh`, so the
orchestrator is already immune to the trigger; only its `board_pass` empty-guard (part 1) is not.

**New — two ADRs landed after this spec was written, and they constrain part 3.**
- **ADR-0030** (change 0072) fixes the discrimination rule the structural sentinel must use: a guard
  over skill prose forbids **invocations**, not nouns. Descriptive mentions of `board-refresh.sh`
  are PERMITTED; only an instructed invocation is a violation. This is load-bearing here —
  `docket-convention/SKILL.md:212,238` and `docket-status/SKILL.md:57,80` legitimately *describe*
  `board-refresh.sh` and must stay green. The sentinel's third clause is therefore
  "no `docket.sh board-refresh` invocation in prose", never "no `board-refresh` token".
- **ADR-0031** (change 0070) forbids collapsing or deleting board-write guards, and publishes the
  bound of source-syntax scanning. `tests/test_render_board.sh`'s `REDIRECT_RE` prose scan and its
  write sentinel both stay; this change adds an **independent** scan rather than widening either.
  `tests/test_skill_facade_wiring.sh` (0072) already owns the exact scope this sentinel needs —
  `skills/*/SKILL.md` + `skills/docket-convention/references/*.md`, with code-unit extraction and
  canonical-form stripping — so the new assertions extend that file's Layer-1 sweep in its
  established idiom.

**Test fixture surface (grep-derived, for the plan).** `tests/test_docket_config.sh:107,397`
assert `BOARD_SURFACES` is empty for `[]` → must become `none`. `tests/test_board_refresh.sh`
sections 2 and 5 key the disabled path and the truncation trap on `""` → re-key to `none`, and the
empty-value case flips exit 0 → exit 2. `tests/test_docket_status.sh` drives `BOARD_SURFACES=`
empty fixtures at ~L306-312, 728, 773, 859-873, 1315-1510 (the `board off` report contract) →
re-key to `none`, with the `board off` stdout line unchanged.

**Verdict: build as specified.** Nothing is obsolete; the design is unchanged. Scope is confirmed
smaller in part 2 and tighter in part 3.

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

The 8 duplicated Board-pass call sites (6 files, grep-derived above) collapse into a single facade
call:

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
  `SKILL_FINISH`) — the same unresolved-variable hazard on the model-consumed side. 0072's Step-0
  preamble rewrite (carry printed values forward as literals) is the mitigation of record, and it
  landed. **Reconciliation note (2026-07-14):** that mitigation is an *instruction*, not an
  enforcement — exactly the gap this change closes for `--surfaces`. The `$SKILL_*` family plausibly
  has the same surviving-trigger shape, but auditing and hardening it is **still out of scope here**;
  file it separately rather than widening this change.
- **The Step-0 preamble rewrite and the `eval` retirement** — 0068 and 0072, both merged.
- **The `github` surface and `github-mirror.sh`.** Same caller pattern, but that surface is
  best-effort by design; this change scopes to the `inline` write decision.
- **Retrofitting stale boards.** Any board left stale by this bug self-heals at the next correctly
  wired Board pass.

## Risks

- **The exit-2-on-empty reversal could break an unfound caller.** Audited on `main` 2026-07-14: the
  callers of `board-refresh.sh` are (a) `docket-status.sh`, via `board_pass`, and (b) the 8 skill-prose
  sites across 6 files, which this change deletes. No third caller exists. The build must **re-derive
  this list by grep** rather than trusting this spec — that discipline is exactly what caught 0072's
  unfulfilled prediction.
- **Sequencing — RESOLVED.** This change had to land *after* 0072 or the two would rewrite the same
  skill prose with different designs. 0072 merged (PR #79) on 2026-07-14, so the hazard is discharged
  and `depends_on: [72]` is satisfied. The prose 0072 left behind is the prose this change now edits.
- **`docket-status.sh`'s guard is the reference implementation, not a bystander.** `board_pass`
  (`scripts/docket-status.sh:46`) still reads `[ -n "$surfaces" ] || { echo "board off"; return 0; }` —
  i.e. it *also* treats unresolved-config as "disabled". Part 1's `none` arm must flip this to an
  assertion, or the orchestrator that part 2 routes every caller through keeps the very bug this
  change removes from everyone else.
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
