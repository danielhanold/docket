<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0222 — Raise docket's minimum Bash from 4+ to 4.4](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-03-0222-raise-docket-s-minimum-bash-from-4-to-4-4.md)**
<!-- docket:backlink:end -->

# Raise docket's minimum Bash from 4+ to 4.4 — design

Change 0222. Raise the enforced runtime floor to GNU Bash 4.4 — the exact version that fixes the
one upstream defect docket pays for (empty-array expansion under `set -u`) — then delete every
piece of code, comment, and test surface that exists only to survive 4.0–4.3. Breakage posture
is ruled: hard error at the validators (Daniel, triage 2026-08-07).

## The three comparison implementations (not five)

The stub counts five validators; the *diagnostic sites* are five, but the version comparison is
implemented three times, and all must change coherently:

1. **`scripts/lib/docket-runtime.sh` — `docket_runtime_validate_bash()`** (~line 162). The shared
   validator, sourced by `docket-config.sh`, `ensure-global-config.sh`, and (via them)
   `ensure-docket-env.sh`. Written in Bash-3.2/POSIX-safe syntax because it runs before a
   configured runtime exists — that constraint is unchanged.
2. **`scripts/docket.sh` — the inline pre-exec check** (~lines 52–83). The facade duplicates the
   check before re-execing through `$DOCKET_BASH_PATH`; it cannot rely on the lib being loadable
   under the host shell. Same 3.2-safe constraint.
3. **`scripts/run-tests.sh` — the prologue self-check** (~lines 41–44). Requires
   `BASH_VERSINFO` >= 4.**3** (for `wait -n`) and re-execs under the configured runtime only when
   below that; `TEST_BASH` (~line 50) otherwise falls back to PATH bash. Bump this threshold to
   4.4 and its "needs GNU Bash 4.3+ (wait -n)" string — WITHOUT this, a host bash of exactly 4.3
   passes the prologue, never re-execs, and crashes on this change's now-unguarded `PAR`/`SER`
   expansions; the bump is also what makes the test-surface deletions below safe under the suite.

The first two currently extract only the major. Change: also extract the minor
(`sed -n 's/^GNU bash, version \([0-9][0-9]*\)\.\([0-9][0-9]*\).*/…/p'` shape, two captures or two
`sed` passes — match the file's existing style) and require
`major > 4 || (major == 4 && minor >= 4)`. An unparseable minor fails the check, same as an
unparseable major does today.

## Reason token

`docket_runtime_validate_bash` prints a machine-readable reason token; the too-old case is
`old-major`. Rename it to **`old-version`** and let it cover: unparseable major, unparseable
minor, major < 4, and 4.0–4.3. Update the three in-repo consumers' case arms
(`docket-config.sh` ~391, `ensure-docket-env.sh` ~52, plus the lib's own doc comment) and the
token assertions in `tests/test_docket_runtime_lib.sh` (~170–173). The token is consumed only
in-repo, so the rename is safe; keeping a token named `old-major` that now fires on a minor
mismatch would be a standing lie.

## Diagnostic and remedy strings

Every site that says "Bash 4 or newer" / "Bash 4+" says "Bash 4.4 or newer" / "Bash 4.4+":

- `scripts/docket.sh` `_docket_runtime_remedy` + the version-failure printf
- `scripts/docket-config.sh` `_runtime_remedy` + the `old-version` die
- `scripts/ensure-docket-env.sh` `old-version` die
- `scripts/ensure-global-config.sh` `REMEDY` + both die strings (~136, ~142)
- Docs: `scripts/docket.md`, `scripts/docket-config.md`, `scripts/ensure-docket-env.md`,
  `scripts/ensure-global-config.md`, `README.md` (~124, install guidance)
- Stale-wording sweep: `scripts/profile-asserts.sh` ~29 and `scripts/profile-one-test.sh` ~30 both
  say "one major above docket's own 4+ runtime floor" — reword for the 4.4 floor (the profilers'
  own Bash 5+ requirement is untouched)

## Deletions (the payoff)

Reference sites by pattern, not line — numbers have drifted since filing:

- Guarded-expansion idiom `${ARR[@]+"${ARR[@]}"}` → plain `"${ARR[@]}"`, plus the explanatory
  4.0–4.3 comments beside each: `scripts/board-checks.sh` (`ADR_FILES`, ~700),
  `scripts/docket-status.sh` (`filt` ~407, `adr_args` ~938), `scripts/run-tests.sh`
  (`PAR` ~270, `SER` ~278).
- **Keep** `scripts/board-checks.sh` leg C's `ar_bases` count gate (~567) and
  `scripts/docket-status.sh`'s `bases` count gate (~667): both comments say bash >= 4.4 expands
  the empty array "happily" into a *forbidden reading* ("ahead of nothing") — the gate encodes
  semantics, not a 4.0–4.3 workaround. Trim only the sentences describing pre-4.4 crash behavior.
- `tests/test_comment_anchor_style.sh` (~101): drop the empty-population skip, loop plainly.
- `tests/test_board_checks.sh` fixture **I1** (~3494–3517): retire the fixture and its two
  asserts — its sole purpose is pinning the 4.0–4.3 crash shape. (The second assert, "other
  checks still fire when --adrs-dir is empty", is covered elsewhere; if inspection during build
  shows it is not, keep that one assert on a slimmed fixture.)
- `tests/test_board_checks.sh` mutation M (~2638–2667): drop the dead 4.0–4.3 arm (the branch
  keyed on "this bash errors on an empty array"); keep the >= 4.4 arm, which measures the count
  gate's real job (misfire = "ahead of nothing").
- Stale cross-reference — **~926 only**: `scripts/docket-status.sh` ~926 ("the same change-0064
  crash shape, one line lower") lives inside the guarded-expansion comment being deleted; delete
  it with that block (the real array-crash precedent is change 0117 / commit `0695b921`). The
  ~909 comment is NOT stale, despite the stub's triage note: it cites 0064 for the
  `${TERMINAL_PUBLISH:-false}` **scalar** guard, 0064 is the change that introduced
  `terminal_publish`, and an unset-scalar crash under `set -u` is version-independent — that
  guard and its citation stay untouched.
- `scripts/board-checks.sh` `PATH_STACK` pop comment "(bash-4.0-safe; no unset arr[-1])" (~818):
  drop the parenthetical only — `unset 'arr[-1]'` needs 4.3+ and buys nothing; the slice-pop
  stays.

## New floor-validator test surface

- `tests/test_docket_runtime_lib.sh`: a fake `GNU bash, version 4.3.48(1)-release` binary must be
  rejected `old-version`; `4.4.x` and `5.x` accepted; token-rename asserts (~170–173) updated.
- `tests/test_bash_runtime_routing.sh` — a NAMED maintenance obligation in docket.sh's prologue,
  not optional: its equivalence scope statement (~183, "banner shape and major-version floor")
  becomes major+minor; fixture `v4` (`4.0.0`, built with `mk_accept`, ~210) flips to a reject
  fixture (add a `4.4.0` accept fixture); the remedy-string asserts (~156–170) pinning
  "Bash 4+"/"Bash 4 or newer" move to the 4.4 strings; the `REAL_BASH` picker (~13–14) must
  require >= 4.4, not any major >= 4; add a `4.3.x` `mk_reject` fixture to the equivalence loop
  (critic-caught: without it, a minor-threshold drift in docket.sh's prologue agrees with the
  library on every other fixture and the guard stays green — 4.3 is exactly the crash-critical
  boundary); the "major floor" assert label (~223) rewords to major+minor. All in the same
  commit as the docket.sh change or the suite reddens/lies.

## Assumptions

1. **depends_on 0211 state: done** — archived at
   `docs/changes/archive/2026-08-05-0211-aborted-run-is-blind-to-a-run-that-stops-after-the-build-com.md`
   (status: done, PR #160 merged). Its mutation-M 4.0–4.3 arm and results-record open item are
   real and current; no design-ahead risk. Rejected: treating the dependency as unsatisfied —
   it is satisfied.
2. **Floor = 4.4, not 5.0; hard error, not warn-and-degrade.** Both ruled by Daniel in triage
   2026-08-07 (recorded in the stub). Not re-opened here.
3. **Comparison stays in the three existing implementations; no new shared helper.** The three
   are the shared lib validator, docket.sh's inline pre-exec check, and run-tests.sh's prologue
   self-check (whose threshold moves 4.3 → 4.4 as part of this change — critic-caught: leaving it
   at 4.3 would reintroduce the crash this change deletes). Alternative: extract a shared
   major.minor comparator. Rejected — docket.sh's check is deliberately inline (it runs before it
   can trust sourcing anything under the host shell), divergence is already pinned by the
   test_bash_runtime_routing.sh equivalence guard, and a pre-runtime-safe include re-opens the
   bootstrap layering for no payoff.
4. **Rename the reason token `old-major` → `old-version`.** Alternative: keep `old-major`
   covering minors too (zero caller churn). Rejected — every consumer is in-repo and already
   being edited for remedy strings; a token whose name misstates what it detects is exactly the
   stale-comment class this change is deleting. Contract note in `docket_runtime_validate_bash`'s
   header comment updated to match.
5. **The two empty-array *count gates* stay; only the guarded-expansion *idioms* go.** The gates
   (board-checks leg C, docket-status bases) prevent a semantic misfire on >= 4.4, per their own
   comments and mutation M's >= 4.4 arm. Rejected: deleting them as "4.0–4.3 support" — that
   would re-introduce the forbidden "ahead of nothing" reading mutation M pins.
6. **`ensure-global-config.sh` and `lib/docket-runtime.sh` stay 3.2/POSIX-syntax.** Unchanged
   constraint from the stub: raising the floor changes what they validate, never what they are
   written in. No new syntax anywhere (stub's out-of-scope holds).
7. **Sequencing vs 0150 (pin/report shell toolchain): no ordering coupling.** 0150 is still
   proposed, spec-less, independent; 0222 neither needs it nor blocks it — if 0222 lands first
   0150's check merely gets simpler to state. Rejected: folding 0150 in (scope creep into a
   deliberately mechanical floor-raise) or adding a depends_on either way. `related: [150]`
   already records the association.
8. **CI follow-up: recommended, not filed here.** The stub's open question — without CI a 4.4
   floor is no better *exercised* than 4.0 — is true but does not weaken this change: the point
   is deleting unfalsifiable claims, not adding falsifiable ones. A CI/multi-version-matrix
   change should be proposed separately (auto-groom may not mint stubs; reported in the groom
   report). Note the stub's own out-of-scope already says this.
9. **`mapfile -d` needs no work here.** `tests/test_grep_portability.sh` already uses it; 0200
   has already dropped its former item 10 (rewrite for 4.0 compat) per Daniel's same ruling. This
   change only legalizes the status quo; no edit to that test.
10. **profiler scripts untouched.** `profile-asserts.sh` / `profile-one-test.sh` keep their
    self-enforced Bash 5+ (`EPOCHREALTIME`) requirement, per the stub.

## Open questions resolved from the stub

- *Minor-version comparison net-simplification*: yes — ~6 added lines (two `sed` minor extracts +
  two compares + token rename) against deleting 5 guarded idioms with multi-line comments,
  fixture I1, the anchor-style skip, and mutation M's dead arm. Net negative diff.
- *Failure posture*: ruled — hard error (die) at all validators, matching current behavior.
- *0150 sequencing*: independent; no coupling (Assumption 7).
- *CI follow-up*: recommend filing separately (Assumption 8).
