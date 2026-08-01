# Codex support for profile-routed Docket builds — results
Change: #169 · Branch: feat/codex-profile-routed-build-support · PR: https://github.com/danielhanold/docket/pull/143 · Plan: docs/superpowers/plans/2026-07-31-codex-profile-routed-build-support.md · ADRs: 36, 37, 38, 63, 64

**Codex certification pending.** Tier 2 of the design is a human-in-a-Codex-session checklist that an
autonomous build cannot execute, and this change is **not certified** until all three named dispatches
below are observed. The determination was made against the installed CLI, not assumed:

- `codex exec --help` exposes **no agent-selection flag**. A named profile dispatch can only be
  induced by a real, non-deterministic multi-task build inside a Codex session; it cannot be invoked
  directly.
- `codex debug` offers `models`, `app-server`, and `prompt-input` only. There is **no agent-registry
  command**, so there is no way to observe which model a named agent resolved to short of the session
  reporting it.

The evidence the spec demands per profile — the controller's routing line, the observed named
agent/model indicator, the structured worker outcome, the focused verification, and the task commit —
is session-observation evidence. A green Tier 1 suite (73/73 on this branch) is **not** certification
and must not be presented as such.

## Automated evidence (recorded 2026-07-31)

Installed CLI:

```
codex-cli 0.146.0
```

Catalog-reported reasoning efforts for the three shipped slugs (`codex debug models`):

```
gpt-5.6-luna ['low', 'medium', 'high', 'xhigh', 'max']
gpt-5.6-terra ['low', 'medium', 'high', 'xhigh', 'max', 'ultra']
gpt-5.6-sol ['low', 'medium', 'high', 'xhigh', 'max', 'ultra']
```

Generated `.codex/agents/docket-*.toml` for all twelve wrappers, from a sandbox run of
`sync-agents.sh` with `agent_harnesses: [claude, codex]`:

```
docket-adr: model = "gpt-5.6-terra" model_reasoning_effort = "xhigh"
docket-auto-groom-critic: model = "gpt-5.6-sol" model_reasoning_effort = "medium"
docket-auto-groom: model = "gpt-5.6-sol" model_reasoning_effort = "low"
docket-brainstorm-consultant: model = "gpt-5.6-sol" model_reasoning_effort = "medium"
docket-build-economy: model = "gpt-5.6-luna" model_reasoning_effort = "xhigh"
docket-build-premium: model = "gpt-5.6-sol" model_reasoning_effort = "medium"
docket-build-standard: model = "gpt-5.6-terra" model_reasoning_effort = "high"
docket-finalize-change: model = "gpt-5.6-terra" model_reasoning_effort = "high"
docket-implement-next: model = "gpt-5.6-sol" model_reasoning_effort = "medium"
docket-integration-repair: model = "gpt-5.6-sol" model_reasoning_effort = "high"
docket-rebase-resolver: model = "gpt-5.6-sol" model_reasoning_effort = "high"
docket-status: model = "gpt-5.6-luna" model_reasoning_effort = "xhigh"
```

Every row matches the settled table in the plan's Global Constraints, and every effort token is one
the installed catalog reports as supported for its slug.

## Verify (human)

Run `bash sync-agents.sh` and then **restart Codex** — Codex registers agent definitions at process
start, so a wrapper written mid-session is invisible to it. Then drive a real, multi-task
`docket-build` inside a Codex session; do not mock the dispatch.

- [ ] An explicit `economy` task dispatches to a named agent observed on `gpt-5.6-luna` / `xhigh`
- [ ] An explicit `standard` task dispatches to a named agent observed on `gpt-5.6-terra` / `high`
- [ ] An explicit `premium` task dispatches to a named agent observed on `gpt-5.6-sol` / `medium`

Record for each: the controller's routing line, the observed named agent and model indicator, the
structured worker outcome, the focused verification, and the task commit. **A merely completed child
is not success evidence** — the controller must observe the worker's structured outcome.

**Re-probe before certifying.** Query the catalog again immediately before you certify
(`codex --version`, `codex debug models`) and record the version at that moment. The design forbids
substituting a model if a slug has become unavailable — that is a stop-and-surface, not an
in-implementation choice.

### Recorded waiver

Automatic task classification and the `NEEDS_ESCALATION` → single-retry path are deliberately **not**
repeated live under Codex. They are defined in the harness-neutral `docket-build-task` contract and
`docket-build`'s loop, loaded identically by all three profile workers; what is harness-specific is
*dispatch*, which is exactly what the three checks above certify. Both paths have prior Claude
evidence and hermetic test coverage. This is a deliberate waiver, not an omission: if profile routing
under Codex ever misbehaves on an escalation, this is the first gap to re-open.

## Findings

**Two plan mutations were inert and were substituted with effective ones.**

1. Mutating a *sidecar* value is self-cancelling for the generated-TOML asserts, because both sides of
   the comparison read the same sidecar. The real defect class is a *generator* mutation: hard-coding
   a Claude ID into `emit_codex_toml` reddens 14 asserts.
2. Dropping `codex` from `HD_SHIPPED_HARNESSES` alone is inert for the warning path — that variable is
   consumed only by `hd_validate`'s completeness loop. The effective form is de-ship **and** strip the
   sidecar block.

Both substitutions were run and reddened their intended guards.

**`grep` on this machine is ugrep, and it prints paths without the `./` prefix.** Exclusion filters
written as `grep -viE "^\./docs/..."` therefore match nothing and silently become no-ops. The plan's
own derived-site greps in Steps 1 and 6 had this bug; they behave correctly under `/usr/bin/grep`.
Any future derived-site grep in this repo needs the same care — and `/usr/bin/grep` remains the
portability target.

**Two pre-existing bash 3.2 failures, not caused by this change.** Under bash 3.2,
`tests/test_docket_example_yml.sh` fails with a syntax error (unmatched backtick, around line 1564)
and `tests/test_grep_portability.sh` fails with `ALL_FILES[@]: unbound variable` (around line 109).
Both were reproduced against the base commit `9d41fa6b`, and `test_grep_portability.sh` is untouched
by this branch.

**Plan deviation: `tests/test_docket_build.sh` was not in the plan's File Structure.** Task 4's README
prose edit falsified two 0168-era guards in it. Task 5's whole-suite gate caught them and re-pointed
them — asserting the retired claim **absent** rather than deleting the asserts. This is the "tests
grep the copy, not the source" hazard in its usual form.

**TDD exception on Task 4 (documentation).** No guard asserts the new wording, so a manufactured
failing test would only have encoded the prose about to be written. Substituted verification:
green-before/green-after across the six neighbouring guards that grep the copy, plus the derived
stale-promise grep. Residual risk: nothing mechanically ties README and `docs/codex/setup.md` prose
to `agents/harness-defaults.yml`.

## Independent whole-branch review (2026-07-31)

Zero Critical, six Important, six Minor. All six Important are fixed on this branch; the Minor ones
that were cheap and in files already being touched went in with them. **Two findings were proven by
mutation before being fixed, and both were asserts that passed while detecting nothing** — the same
self-cancelling class the build had already caught twice in the plan's own mutation matrix. Four
instances on one change is the defining hazard here, and worth naming for the next reader.

- **`tests/test_sync_agents.sh` — a pure-negative assert that could not redden for its own stated
  condition.** `0169: a complete codex block silences the whole harness` (`! grep -qF "WARN codex/"`)
  was commented as "what would redden if the codex block were dropped or left partial". It does not:
  with a partial or missing block `hd_validate` aborts generation *before* any wrapper is written, so
  no `WARN codex/` line is ever emitted and the negative passes. It would also have passed on any
  unrelated `sync-agents.sh` failure. Confirmed by deleting the codex `status` row — the assert
  stayed green. Now paired with a positive companion (run exited 0 **and** the generated TOML carries
  the sidecar's model), and the comment states what the pair actually guards.
- **`tests/test_docket_example_yml.sh` — the four codex round-trip asserts were self-cancelling.**
  Deleting all twelve codex rows from `.docket.example.yml` left
  `round-trip: codex status model came from the example block` **green**, because the resolver falls
  back to the sidecar and both sides of the comparison move together. The assert names claimed
  something they could not detect, and spec Tier-1 property 9's second clause ("resolves through the real
  generator into Codex TOML") was established by nothing. Fixed with a sentinel: one example value is
  rewritten to `gpt-5.6-probe` — verified absent repo-wide, and guarded by an assert that catches it
  if ever adopted as a real ID — and the sentinel must reach the generated TOML. The pre-existing
  cursor leg had the same misnomer and got the same treatment (`cursor-grok-4.5-probe`).
- **Three maintained-prose sites still carried claims this change falsified** — `README.md`'s
  "unvalidated illustrations, marked as such in the file" (pointing at a marking Task 3 deleted),
  `skills/docket-convention/references/agent-layer.md`'s "claude complete, cursor build-profiles only"
  table row (false since 0168, and a file agents load as ground truth), and
  `tests/test_sync_agents_cursor.sh`'s cross-reference claiming codex ships no block. **Root cause was
  a defect in the plan, not the build:** the spec said to remove all "unvalidated" wording, but the
  plan's derivation grep vocabulary never included `unvalidated` or `illustration`, so the derived
  site list structurally could not find them. Re-run with those terms added, it surfaced nothing else
  in maintained source.
- **A backward-compatibility hazard, now documented and guarded.** A user who had pinned only
  `model` for a Codex agent previously got no effort line and now silently inherits docket's shipped
  effort beside a model they chose. Cursor had no such hazard at 0168 because every Cursor effort is
  `auto`; Codex's are real tokens, so this is new and specific to this change. Spec Tier-1 property 6
  ("user values still override field-by-field") also had **no Codex-specific guard** — the example
  round-trip could not substitute, being the same self-cancelling shape. Both closed: a sandbox test
  with a partial `agents.codex` override asserting user-model + shipped-effort, and an upgrade
  warning in `docs/codex/setup.md`.

The review also verified the **change 0173 interaction end to end** (it merged to `main` mid-build,
touching `sync-agents.sh` and `tests/test_sync_agents.sh`): `git merge-tree` merges cleanly, and
0173's widened `field_of` value class and this change's `hd_field` are independent — the sidecar
keeps its own `hd_validate` leg, and all twelve new Codex values are bare, unquoted, space-free, so
both validators accept them. ADR-0065's missing quote leg in `hd_validate` is real and is already
change **0180**; the duplicated-extractor question is change **0179**. Both correctly left alone.

**Suite after the fixes: 73/73 green** (a full re-run, since test files changed after the first gate).

## Post-merge live certification (2026-08-01)

The maintainer completed a real multi-task Codex build while implementing change 0176 and observed
the three named profile workers using their shipped model/effort pairs:

- economy — `gpt-5.6-luna` / `xhigh`
- standard — `gpt-5.6-terra` / `high`
- premium — `gpt-5.6-sol` / `medium`

The build's testing completed successfully. This supplies the previously pending Tier 2
session-observation evidence: all three native Codex profile dispatches resolve to the intended
classes in a real Codex session.

## Follow-ups

- **Change 0183** was minted from this review for the `cursor-rules/dispatch.head.md` item below.
- **`cursor-rules/dispatch.head.md` carries a stale claim.** It still says docket ships validated
  Cursor model IDs "for the three build-profile workers only — … Every other wrapper is generated
  **unpinned**". Change 0168 completed the Cursor block (twelve of twelve pinned), which made that
  false *and* silently retired the guard over it: `tests/test_cursor_dispatch_rule.sh` gates its head
  asserts on `if [ "$n_cursor_pinned" -lt "$n_src" ]`, and `12 < 12` is false. That head is catted
  verbatim into every consumer repo's `.cursor/rules/docket-dispatch.mdc`, so the false claim ships.
  The fix is the same shape as this change's Task 2: correct the prose and give the guard an `else`
  arm. Out of scope here — it belongs to change 0168's lineage.
- **A line-number cross-reference in maintained source.** `scripts/lib/harness-defaults.sh`'s
  completeness comment says the reverse direction "is already enforced per-entry above (line 131)".
  AGENTS.md requires a symbol name or a verbatim-quoted clause; the prose "line N" form is
  unenforceable by `tests/test_comment_anchor_style.sh` and rots exactly where code moves most.
- **A bash 3.2 compatibility pass** on `tests/test_docket_example_yml.sh` and
  `tests/test_grep_portability.sh` (see Findings) is separate work.
