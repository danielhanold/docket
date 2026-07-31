<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0168 — Cursor support for profile-routed Docket builds](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-07-31-0168-cursor-profile-routed-build-support.md)**
<!-- docket:backlink:end -->

# Cursor support for profile-routed Docket builds — results
Change: #168 · Branch: feat/cursor-profile-routed-build-support · PR: https://github.com/danielhanold/docket/pull/140 · Plan: docs/superpowers/plans/2026-07-31-cursor-profile-routed-build-support.md · ADRs: 64 (supersedes 48)

**Cursor IDE certification pending.** Tier 2 of the design is a human-at-the-IDE checklist that an
autonomous build cannot execute. Until all five checks below pass, the support claim is unproven and
this change should not merge. `cursor-agent` is explicitly **not** an accepted substitute — it can
lag the IDE and return false negatives.

## Verify (human)

Run these in the Cursor IDE against the real generated agents and the real `docket-build`
controller — not a mock dispatch. Record the Cursor build, account/plan context, and the generated
wrapper model lines alongside each result. The controller must observe the worker's structured
outcome before continuing; a merely completed child is not success evidence.

- [ ] An explicit `economy` task dispatches to `cursor-grok-4.5-medium`
- [ ] An explicit `standard` task dispatches to `cursor-grok-4.5-high`
- [x] An explicit `premium` task dispatches to `claude-opus-5-high`
- [x] At least one automatically classified task whose routing line and model indicator agree
      — **accepted on reasoning, not observed under Cursor** (see the certification note below)
- [x] One task deliberately started at `standard` whose revealed named risk causes
      `NEEDS_ESCALATION`, followed by exactly **one** foreground premium retry
      — **accepted on reasoning, not observed under Cursor** (see the certification note below)

Before the above, regenerate and eyeball the wrappers — `bash sync-agents.sh`, then inspect
`.cursor/agents/`:

- [ ] The three `docket-build-*` wrappers carry their Cursor IDs with **no** `[effort=…]` suffix
- [ ] **All twelve** Cursor wrappers carry a `model:` line, each matching its `cursor:` row in
      `agents/harness-defaults.yml`, none with an `[effort=…]` suffix. A **missing** `model:` line
      is a defect — the shipped cursor block is complete.
      The one Claude-namespace ID that is correct here is `docket-build-premium`'s
      `claude-opus-5-high`: that is Cursor's own name for the model, selected through Cursor, not a
      leaked Claude Code pin. Any *other* `claude-*` ID in a Cursor wrapper is the cross-harness
      leak this design removed.

> This item was amended after the first certification attempt. It originally read "the other nine
> Cursor wrappers carry **no** `model:` line at all", which was the shipped design until the review
> of PR #140 completed the cursor block. If you are working from a printed or cached copy, the
> twelve-pinned form above is the current one.

### Certification outcome (2026-07-31, maintainer, Cursor IDE)

**Certified.** Observed directly in a live Cursor build of change 0009: all three profiles
dispatched to their own agents (`Docket Build Economy` / `Standard` / `Premium`), economy and
standard on Grok and premium on Opus, with routing that varied by task rather than being uniform —
Zero Trust Access gating and a least-privilege deploy token to premium, scaffolding and gate
validation to economy. Regenerated `.cursor/agents/` wrappers were separately confirmed to carry
the correct model IDs.

**Two checklist items were accepted on reasoning rather than observed**, at the maintainer's
decision: automatic task classification, and the `NEEDS_ESCALATION` → single premium retry path.
The rationale — escalation and classification are defined in the `docket-build-task` contract and
`docket-build`'s loop, both harness-neutral prose loaded identically by all three profile workers;
what is harness-specific is *dispatch*, and dispatch was certified above across all three profiles
including two independent premium dispatches. Both paths have been observed working under Claude
Code. Recorded here as a deliberate waiver, not an omission: if profile routing under Cursor ever
misbehaves on an escalation, this is the gap to re-open first.

**ADR-0064** records the decision — shipped agent defaults live in a sparse, harness-indexed sidecar
(`agents/harness-defaults.yml`); behavioral wrapper templates carry no cross-harness model floor. It
**supersedes ADR-0048**, whose first invariant fixed the example file's mirror target at
`agents/docket-*.md` wrapper frontmatter. All three of ADR-0048's invariants are restated in
ADR-0064 with the mirror re-pointed at the sidecar, so the two unaffected ones are carried forward
rather than orphaned. ADR-0048's status change was published to `main` directly; **ADR-0064 rides
this change's `adrs:` at terminal publish** — do not publish it standalone, or the change is left
with no route to `done`.

**The reconcile pass found the ADR-0048 collision, which the design spec had missed** — the spec's
architecture-decision section named ADR-0015/0016/0060/0063 and claimed the new ADR superseded
nothing. Both the spec and the change body were corrected before planning.

**The cursor block was completed after the first certification attempt.** As shipped by the build,
cursor carried only the three build-profile workers and the other nine generated unpinned, with
docket's intended tiering for them sitting in a commented example block. A commented default is
inert, so in practice those nine had no docket default at all. Maintainer review of PR #140 called
this out; the cursor block now covers all twelve wrappers, and `hd_validate` enforces the same
completeness for cursor that it already enforced for claude, keyed off a new
`HD_SHIPPED_HARNESSES`. Sparseness is now a property of *which harnesses* appear, never of how much
of one appears — a thirteenth wrapper cannot land pinned on one shipped harness and unpinned on
another.

**Completing the block exposed a latent bug in the build's own warning.**
`warn_fallback_model` suppressed its foreign-ID warning when the sidecar *held an entry* for a
harness/agent pair, rather than when the sidecar *supplied the resolved value*. Those differ: a
user's `agents.default` line outranks the sidecar, so the wrapper was emitted carrying the foreign
ID while the guard stayed silent, reporting a shipped default that never applied. Verified
concretely — `agents.default.status.model: claude-opus-4-8` put `model: claude-opus-4-8` in the
*Cursor* wrapper with no warning. Now keyed on a `RES_MODEL_FROM_SIDECAR` provenance flag and
pinned by a test asserting both the warning and that the wrapper really carries the foreign ID, so
the guard cannot pass on a false alarm. Latent in the original 0168 build because it was reachable
only through cursor's three build workers; general once a harness ships a complete block.

**The safety property was verified by generation, not by prose.** The independent review extracted
`origin/main` and `HEAD` into separate trees and ran `sync-agents.sh` from each into identical
sandboxes: all twelve Claude wrappers resolve to byte-identical model/effort pairs, differing only
in frontmatter field order. No silently-unpinned Claude agent.

**One place where "Claude keeps its exact prior resolution" was not literally true, now closed.**
The rewritten `emit()` initially normalized `model: inherit` to an omitted line on *every* harness.
On Claude that is a real semantic change — `inherit` is a Claude Code frontmatter value meaning "use
the parent conversation's model", while omitting the key selects Claude Code's own subagent default.
Verbatim `inherit` emission was restored on the Claude path; Cursor and Codex, which have no such
value and normalized it before this change, keep doing so. The asymmetry is now commented,
documented in the README, and pinned by tests across all three harnesses.

**Plan defects the workers caught and corrected** (recorded because the plan was written without
executing its own code):
- The plan's `fm_has` awk helper was broken — `exit 0` inside a rule still runs `END`, whose
  `exit 1` overrode it, so the helper returned 1 unconditionally. As written it would have made
  every `! fm_has` assert permanently green: a decoration guard on the change's central negative
  invariant.
- The plan's test snippets called sandbox helpers (`mk_repo_cfg`, `run_sync`) that do not exist in
  `tests/test_sync_agents.sh`, with a wrong generated-path shape.
- The plan told Task 6 to leave the Codex TOML value asserts unchanged. They could not stay: Codex
  has no sidecar block, so all twelve Codex wrappers now emit no pin. They were re-pointed at
  absence. **Change 0169 must flip them back to value asserts when it lands the Codex mapping.**

**Review found six Important issues, all fixed on the branch** (commit `177c00db`): an unguarded new
Cursor mirror in `.docket.example.yml`; the `inherit` semantic change above; two *generated*
documents still claiming every wrapper is model/effort-pinned (`cursor-rules/dispatch.head.md` and
the committed Codex `AGENTS.md` dispatch block); a provably dead duplicate-harness guard in the
validator; and a value-class defect where a model ID containing `/` or `:` was silently truncated
yet still validated. No Critical issues.

**Consumer-repo note:** repos that list `codex` in `agent_harnesses` will see `sync-agents.sh --check`
flag their committed `AGENTS.md` dispatch block as stale until they re-run sync and commit. That is
the normal path for any managed-block text change. This repo itself carries no such block.

## Follow-ups

- **#0173** (auto-captured) — `field_of()` in `sync-agents.sh` shares the value-class defect fixed in
  the new sidecar reader: a user-configured `model: anthropic/claude-opus-5` in any of the three user
  config layers silently resolves to `anthropic` and bakes a wrong pin. Pre-existing and identical on
  `origin/main`, so deliberately left out of this change's scope — but arguably the worse of the two,
  since user config is where provider-prefixed IDs actually get typed.
- **#0169** owns the Codex block in the same sidecar, and must restore the two Codex TOML value
  asserts noted above.
- **#0142** owns making the unmapped-harness wrapper gap loud at generation time. This change only
  kept `warn_fallback_model()` honest under the new resolution order.
- `tests/test_docket_example_yml.sh` hits a bash parse error when BSD grep precedes ugrep on `PATH`,
  truncating it to roughly half its asserts. **Pre-existing on `origin/main`**, reproduced on the
  unmodified base — not introduced here, and not fixed here.
