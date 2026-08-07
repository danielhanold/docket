<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0220 — clear the unfixed review findings from change 0207](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-07-0220-clear-the-unfixed-review-findings-from-change-0207.md)**
<!-- docket:backlink:end -->

# Clear the unfixed review findings from change 0207 — design

Change: 0220 · Scope: `sync-agents.sh` and its test suite. No behavior change to the
generation contract 0207 settled; these are defects in its execution plus two test-coverage holes.

## Context

0207 introduced **Gate 3** (`validate_runner_config`) — every `runner:` rule checked across every
candidate triple *before the first wrapper write*, so a bad `runner:` fails the run with an
accumulated summary instead of aborting mid-loop and leaving a zero-length wrapper. `docket-review-deep`
returned 7 findings on PR #159; none were remediated (only blockers auto-fix). This change works all 7.

Relevant code (all in `sync-agents.sh` on `main`):

- `validate_runner_config` — user-level leg over `$USER_TARGETS`, project-level leg over `$HARNESSES`
  guarded by `per_repo_opted_in`.
- `project_level_pass` — writes per-repo wrappers, guarded by `per_repo_opted_in`.
- `check_project_level` — legs (a)/(b) CI-meaningful, leg (c) advisory drift, plus a dispatch-rule
  drift loop. Everything below the early return is guarded by `gitignore_block_wanted`, which is
  strictly weaker: it also returns true for a `.docket.local.yml`, a `docket` branch, or an existing
  `.gitignore` block.
- `emit_wrapper` — keeps its own copy of the provenance filter (`RES_MODEL_FROM_USER` over positional
  `$2`) rather than calling `user_flag_model`.
- `runner_config_error` — the single source of both rules, their diagnostics, and their order.

## Decisions

### D1 — Close the leg (c) gap by tightening leg (c), not by widening the gate (findings 1, 7)

The gap is real: with a global `agent_harnesses:` list that omits `claude`, `$USER_TARGETS` has no
`claude`, so the gate's user-level leg never sees `agents.claude.*`; in a repo that is **not** opted in
the gate's project-level leg `continue`s; but leg (c) still iterates `$HARNESSES` (which includes
`claude`) and calls `emit_wrapper`, which dies on the can't-happen assertion — raw `ERROR` + `exit 1`,
skipping the remaining `--check` legs and leaking leg (c)'s `mktemp -d`.

**Chosen:** guard leg (c)'s wrapper-drift loop and the dispatch-rule drift loop with the same
predicate their writer uses — `per_repo_opted_in` — extracted as one named predicate
(`project_wrappers_generated`) used by `project_level_pass`, the gate's project-level leg, and
`check_project_level`'s two drift loops. Legs (a) and (b) keep `gitignore_block_wanted` (they are
about the `.gitignore` block and tracked leftovers, not wrappers).

*Rejected — widen the gate to `gitignore_block_wanted`:* it makes the gate a superset of the writer,
so a repo that generates **no** project wrappers (not opted in, but carrying a `docket` branch) would
be hard-failed on the real run over project-layer config that never reaches disk. Trading a
diagnostic-quality bug for a false rejection is a worse trade.

*Rejected — leave it and only fix the comment:* the in-source comment's claim that no gap exists would
become accurate only by admitting the gap; the leaked temp dir and truncated `--check` output remain.

**Consequence — a second bug fixed as a side effect.** Today, in a `gitignore_block_wanted`-but-not-
opted-in repo, `project_level_pass` writes nothing while leg (c) reports "not generated on this
machine" for every agent — a false advisory. Tightening removes it. No existing fixture encodes it:
every leg-(c) advisory assertion in `tests/test_sync_agents.sh` writes `agents:` or `agent_harnesses:`
into `.docket.yml`, so all are opted in, and the codex suite has no leg-(c) advisory assertions at all.

**Accepted cost — one narrow advisory genuinely goes silent.** A repo that *was* opted in, generated
`.claude/agents/docket-*.md`, then dropped its `agents:`/`agent_harnesses:` key keeps those wrappers
and, after this change, stops having them diffed. Accepted because those leftovers are already
un-prunable — `prune_orphans`' legs are themselves `per_repo_opted_in`-gated — so leg (c) was the last
survivor of a boundary drawn everywhere else; making it consistent is the point of the shared predicate.

Then correct the two in-source comments that claim the three `emit_wrapper` call sites are all gated
(`emit_wrapper`'s can't-happen block, and the gate's header): after this change the claim is true
*because* the predicate is shared, and the comment should say that, not assert it as a coincidence.

**Finding 7 folds in here.** The vacuous `--check wrote no wrappers` assert (already true pre-0207,
since leg (c) redirects into a `mktemp -d`) is replaced by the regression test for this decision.
The assertion is **inverted** from the obvious shape, because in the gap fixture the gate itself stays
silent (`USER_TARGETS` has no `claude`, so `runner_config_error` returns 0 on the user leg; the project
leg `continue`s) — and because on the `--check` path a failing gate `exit 1`s before
`check_project_level` runs at all, so "still runs its remaining legs" describes no reachable path.
Post-change, under a global `agent_harnesses:` without `claude` plus a bad `agents.claude.*` `runner:`
in a repo that is *not* opted in, `--check` must show: (a) **no** runner `ERROR` diagnostic, (b) **no**
leg-(c) advisory for `.claude/agents/*` (the false advisory), and (c) the legs *after* leg (c) still
emitting — proof of no mid-leg abort. Pre-write the `.gitignore` docket block in the fixture so leg (a)
passes and `rc = 0` is a meaningful assert rather than a vacuous non-zero. Un-sharing the predicate
reverts to raw `ERROR` + `exit 1` and reddens all three.

### D2 — Fixture the user-level gate leg via the global config layer (finding 2)

Every existing `runner:` fixture writes `.docket.yml` (project layer), so the whole
`for harness in $USER_TARGETS` block — including the `set -u` resolution added for `--check` — is
mutation-survivable.

**Chosen:** add a fixture that writes `runner:` into `$SBX/.config/docket/config.yml` with **no**
`.docket.yml` present, asserting both the real run and `--check` fail with the user-level diagnostic;
add it to the suite that already owns the `runner:` fixtures (`tests/test_sync_agents.sh` unless the
suite split places runner cases elsewhere — the build follows the existing fixture's home, it does not
create a new suite file). Mutation-test it: deleting the `for harness in $USER_TARGETS` block must
redden it (per learning `assert-detects-removal-not-replacement`).

*Rejected — assert on `$USER_TARGETS` internals directly:* a white-box assert on a shell variable
pins the implementation, not the protected behavior (`~/.claude/agents` never generated from bad config).

### D3 — Make "spelled once" true, and document the contract (finding 3)

**Chosen — document the contract and *enforce* it; do not reroute one of the two uses.**
`emit_wrapper` keeps `flag_model="$2"`, its header states that `$2` must be the `RES_MODEL` just
resolved for this `(harness, agent)` by `resolve_agent_layers`, and the assertion block gains the
matching check: `[ "$2" = "${RES_MODEL:-}" ]` or `log ERROR` + `exit 1`. It sits **above** the
`[ -z "$runner" ]` short-circuit, not beside the existing `runner_config_error` assertion inside the
runner branch — the header states the contract for every call, so enforcing it only on the delegated
path would leave the documented rule unenforced on the native one.

*Rejected — `flag_model="$(user_flag_model)"`:* `$2` is **also** passed straight to `emit_shim` as the
frontmatter pin. Sourcing the baked `--model` flag from `$RES_MODEL` while the frontmatter pin stays on
`$2` splits `emit_wrapper` against itself — and against `flag_effort`, which stays on `$3`. The exact
future call site this decision exists for (one passing a post-processed model) would then emit a wrapper
whose frontmatter and baked flag disagree, silently, instead of dying loudly. A loud abort traded for a
wrong artifact is the worse half of the trade 0207 exists to prevent.

*Also rejected — drop `$2`/`$3` from the signature and read `RES_MODEL`/`RES_EFFORT` directly for both
the frontmatter and the flags:* coherent, and the right shape eventually, but it changes the signature of
the function three call sites and 0141's in-flight refactor both touch. Out of proportion to a minor
finding; the assertion buys the same guarantee at a fraction of the blast radius.

`flag_effort` is deliberately **left as-is** — no `user_flag_effort` is introduced. Effort has no
counterpart in `runner_config_error`, so there is no second spelling to drift from; adding a helper
for symmetry alone widens the diff without closing a gap.

*Rejected — document only:* it leaves two spellings of a rule whose divergence is exactly the failure
mode 0207 exists to prevent (learning `guard-keyed-on-presence-not-provenance`).

### D4 — Make the ordering assert discriminating (finding 4)

The `unregistered offender reports the registration rule` assert matches the runner name, which appears
in both diagnostics, so swapping the two `if` blocks in `runner_config_error` leaves it green.

**Chosen:** extract the **offending agent's own diagnostic line** from the accumulated output first,
then assert on it: it carries `is not a registered runner` and does **not** carry the required-model
wording. The line extraction is load-bearing — that fixture also configures a second, legitimately
model-less agent, so the required-model wording is present in the accumulated `$err` and a
whole-output negative assert would fail on a correct implementation. Ordering itself stays pinned by
the existing ORDERING FENCE fixture; this assert is only made able to tell the two rules apart.

### D5 — Scope the "changes nothing on disk" comment (finding 5)

`migrate_legacy_global` runs above Gate 3, so the atomicity claim is overbroad as written — and by more
than the finding says: besides `mv`-ing the legacy `agents.yaml` to `.migrated`, it **appends an indented
`agents:` block to the user's live global `config.yml`** when that file carries none — and appends a
trailing newline to a pre-existing `config.yml` that lacks one, even before the block goes in.

**Chosen:** narrow the wording to wrappers — "a run regenerates every **wrapper** or changes no wrapper
on disk" — and name **both** migration effects (the rename and the write into the user's global config)
as the stated exception in the same comment, so the next reader does not re-derive them. Nothing else on
the failure path writes: the `.gitignore` write, `migrate_tracked_wrappers`, and `prune_orphans` all sit
below a passing gate. Comment-only; no behavior change.

### D6 — Report each distinct diagnostic once (finding 6)

A bad `runner:` in the **global** layer visible to both legs is reported twice, verbatim identically,
against a README promising every offender "in one pass".

**Chosen:** de-duplicate on the **exact diagnostic string** inside `validate_runner_config` — accumulate
seen diagnostics in a newline-delimited string and `grep -F -x -q` before logging (bash-3.2-safe; no
associative arrays). Deduping on the *rendered diagnostic* rather than on the `(harness, agent)` triple
keeps two genuinely different offenders reported separately even when they share a harness/agent, and
keeps accumulation (never short-circuit) intact.

**Required companion fixture — an over-dedupe guard.** Add a fixture where the two legs yield
**distinct** diagnostics (project config sets an unregistered runner while global sets a
registered-but-model-less one) and assert **both** survive the `grep -F -x -q` filter. That is the
failure mode the new code introduces: over-suppression. It is explicitly *not* justified as covering the
project-level leg — the gate's project leg is already well mutation-covered suite-wide (deleting
`for harness in $HARNESSES` still reddens 0207's "no wrapper was written" and "every pre-existing wrapper
survives" asserts); within the dedup fixture alone the second copy stops being observable, which is why
the guard is a distinct-diagnostic assert rather than a duplicate-count one.

*Rejected — dedupe by skipping the project-level leg when it resolves to the same layer as the
user-level leg:* that is layer-provenance reasoning inside a loop that deliberately does not do
provenance, and it would suppress a project diagnostic that merely *happens* to read identically today.

## Assumptions

Every decision below was defaulted autonomously; this is the deferred audit trail.

1. **The findings are accurate as written.** Each was re-verified against the running code on `main`
   (the gate's `per_repo_opted_in` guard vs leg (c)'s `gitignore_block_wanted`; `emit_wrapper`'s own
   provenance test; the two comment claims) rather than trusted from PR #159's prose — learning
   `verify-the-claim`. All 7 reproduce by inspection.
2. **D1 tightens leg (c) rather than widening the gate.** Conservative default: never turn a
   diagnostic-quality bug into a false rejection of a run that would otherwise succeed. Supported by
   learning `opt-in-signal-not-file-presence` — `gitignore_block_wanted` keys on file/branch presence,
   `per_repo_opted_in` on an explicit key, and wrapper generation is the output-generating behavior.
3. **No existing fixture depends on leg (c) running without opt-in** — checked, not deferred: every
   leg-(c) advisory assertion in `tests/test_sync_agents.sh` writes `agents:` or `agent_harnesses:` into
   `.docket.yml`; `tests/test_sync_agents_codex.sh` has no leg-(c) advisory assertions at all (its
   `--check` cases cover legs (a)/(b) and AGENTS.md). The one coverage genuinely lost (a repo that
   dropped its opt-in after generating) is accepted in D1 for consistency with `prune_orphans`.
4. **D3 leaves `flag_model` on `$2` and asserts the contract instead of rerouting it.** All three call
   sites pass `$RES_MODEL` today; the assertion is what stops that being a coincidence, without splitting
   the baked flag's source from the frontmatter pin's.
5. **No new suite file, no new script.** Fixtures land in the suite that already owns `runner:` cases.
6. **Scope stays inside 0207's settled design** — the atomicity invariant and the all-or-nothing
   strictness trade are not reopened (0220's own *Out of scope*), and Gate 2's pre-migration blind spot
   stays out.
7. **Dependency state:** `depends_on` is empty and 0207 is `done` (merged, PR #159), so this is buildable
   now. Four active changes touch the same file in adjacent regions (0082 `per_repo_opted_in`/global
   harness reach, 0140 `emit_shim` model sentinel, 0141 the wrapper-source parse, 0207 the parent) —
   recorded in `related:`, not `depends_on:`: none must land first, but whichever lands second rebases
   over the other in `sync-agents.sh`.
8. **`/usr/bin/grep` portability** — new fixture greps use BSD-safe expressions; the local `grep` is
   ugrep and masks portability bugs (repo learning `shell-portability`).

## Out of scope

- Re-litigating 0207's design (atomicity invariant, all-or-nothing strictness).
- Gate 2's pre-migration blind spot.
- Any `runner:`-scope change (e.g. extending `runner:` beyond the `claude` harness).
- A `trap`-based temp-dir cleanup for `check_project_level` — with D1 the leak's trigger is gone; a
  general cleanup hardening is separate work.
