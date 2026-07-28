<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0135 — Generated Cursor wrappers violate Cursor's subagent contract, disabling skills and model effort](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-07-28-0135-cursor-agent-wrapper-contract.md)**
<!-- docket:backlink:end -->

# Cursor agent wrapper contract — results

Change: #0135 · Branch: `feat/cursor-agent-wrapper-contract` · Plan: `docs/superpowers/plans/2026-07-27-cursor-agent-wrapper-contract.md` · ADRs: 0060 (new), citing 0008/0015/0017/0059

## ⚠️ Cursor IDE validation is PENDING — read before merging

**A green suite does not clear this change's merge gate.** The hermetic suite proves docket *emits*
Cursor's documented wrapper shape. **Nothing in this repo proves Cursor *reads* it.** That docket
holds no vendor tables is a deliberate design property (ADR-0015, ADR-0059), and its cost is that
the contract is only confirmable against the real product.

The certifying tier is the human-executed checklist at **`docs/cursor/validation.md` § Tier 3** —
six phases, each with a definitive observable outcome. It passes when phases 1–3 and 5 are green
and phases 4 and 6 have definitive observed answers.

Specifically unproven by anything on this branch:

- that Cursor parses `model: <id>[effort=<e>]` and actually honors the effort;
- that Cursor silently ignores unknown frontmatter keys;
- that `readonly`/`is_background` default to the values docket relies on by omitting them;
- that a Cursor child loads the docket skills from the body preamble;
- that a nested dispatch at depth 2 succeeds (Finding 1 of the spec, confirmed from documentation
  but not live).

Merging on the suite alone would make this change the fourth instance of the
`skill-fallback-degrades-discipline` learning — green artifacts concealing an unrun verification —
*inside the change written to end that pattern.*

## What shipped

23 files, +2084/−99, seven commits.

| Item | Where |
|---|---|
| `emit_cursor_md()` — Cursor's documented frontmatter only, `<model>[effort=<e>]`, body preamble replacing the inert `skills:` | `sync-agents.sh` |
| Named `codex)`/`cursor)`/`claude)` branches; `*)` documented as an unverified gap, not a supported mapping | `sync-agents.sh` |
| Dispatch rule reworded to capability language (head + all nine fragments); call snippet demoted to a labelled illustration | `cursor-rules/` |
| `cursor` runner adapter + contract, registered in `REGISTERED_RUNNERS` | `scripts/runners/cursor.{sh,md}` |
| Per-harness wrapper-shape table; uniform-shape prose corrected | `skills/docket-convention/references/agent-layer.md` |
| Tier 2 probe + Tier 3 six-phase IDE checklist + the merge-gate obligation | `docs/cursor/validation.md` |
| ADR-0060 | on `docket` (metadata branch) |

New guards: `test_sync_agents_cursor.sh` (24), `test_runner_cursor.sh` (38),
`test_cursor_contract_docs.sh` (16), `test_cursor_dispatch_rule.sh` (7 assert calls, 34 executed —
the fragment block loops over a glob-derived population).

**Whole suite: every file in `tests/`, 0 failing, 0 `NOT OK`** — run twice, before and after the
final fix wave.

## Tier 2 — `cursor-agent` probe: ABSENT (non-gating, and this is not a defect signal)

**Version:** `2026.01.23-916f423` · **Date (UTC):** 2026-07-27 · **Result:** no data.

`cursor-agent -p --output-format text` returned
`Error: Authentication required. Please run 'agent login' first, or set CURSOR_API_KEY`. The probe
never reached a model, so it reports nothing about the wrapper contract in either direction.

Per the evidence rule stated in the spec, in `docs/cursor/validation.md`, and again here:

> **A negative or absent result from `cursor-agent` is never evidence that the wrapper contract is
> wrong.** It is recorded as a CLI limitation observation and nothing more. Only a *positive* result
> carries weight, and it proves only that the contract works on the CLI surface.

Treating an unreliable probe's silence as capability absence is the exact false-negative shape
ADR-0059 exists to prevent — an absence observed in the wrong surface, promoted to a verdict. This
run is the rule's first live exercise and it behaved correctly: **absent, therefore uninformative.**
A future implementer must not re-promote this spike to a gate.

Note the version scoping (learnings: `harness-behavior-is-mode-and-version-scoped`): the observation
above is scoped to this `cursor-agent` build, unauthenticated, headless.

## Findings worth recording

### The change's own thesis turned on itself, in new code (caught at final review)

`scripts/runners/cursor.sh` did not normalize docket's `inherit` model sentinel, while
`emit_cursor_md` does. `emit_shim` bakes `--model $2` whenever the resolved override is non-empty,
so `runner: cursor` plus an explicit `model: inherit` sent `--model inherit[effort=xhigh]` to
`cursor-agent` — a non-existent model ID handed to a CLI with a documented compatible-model
fallback, the effort pin destroyed along with it, and the adapter's own WARN unreachable because it
keys on `-z "$MODEL"`. docket would have pinned one thing in the wrapper and something else when
delegating, silently.

That is precisely the defect class #0135 exists to end, reintroduced in the change fixing it. Fixed
(commit `21f6292`) by normalizing the sentinel before the flag mapping, which routes into the
existing correct WARN. Four asserts, mutation-proven. **The `inherit` gap's twin is still live in
the Codex adapter and upstream in `emit_shim`** — minted as **#0140**.

### Five false greens in one change — a pattern, not bad luck

Every one was caught by a reviewer's own mutation, never by the suite:

1. A `perl -0pi` substitution that silently did not match, so a mutation test "passed" without the
   mutation ever landing.
2. `preflight: diagnostic names cursor-agent` — satisfied by the **worktree path**, which contains
   the string `cursor-agent`. It passed during the red run, with no adapter on disk at all.
3. Two asserts claiming to prove a reverse-direction derivation while both grepped the same file for
   the same table row.
4. A "reverse-direction" emitter guard whose extraction pattern **hardcoded** `(claude|cursor|codex)`
   — it could only ever emit the names the forward loop already asserted. A planted
   `windsurf) emit_windsurf_md` arm, the guard's exact target failure, left the suite green.
5. A Tier 3 guard that stayed green when its heading was downgraded to "optional, best-effort",
   because its whole-file OR-grep still matched `certifying tier` in body prose.

The unifying shape, worth carrying forward: **every one of these asserts was written to CONFIRM the
wording or behavior just introduced, rather than to DETECT the state just removed.** For a prose or
shape guard, write the negative assert first and mutate the original defect back in. A positive
assert on replacement text proves only that the replacement is present.

Corollary that bit twice: **a mutation that "passes" is evidence only if you confirmed the mutation
actually landed.** Both surviving implementers ended up asserting `grep -c` before and after every
mutation.

### Per-fragment variance is real

The nine dispatch fragments looked interchangeable and were not.
`docket-brainstorm-consultant.md` carries no `Do NOT` line at all (it states "It performs zero
docket operations" instead), and the nine split into two structural families. Applying one template
literally would have deleted a behavioural constraint while looking like a docs edit — the
`consolidation-flattens-caller-variance` learning, live. Only each fragment's dispatch-instruction
sentence was changed; a `--word-diff` over `cursor-rules/` shows **zero word deletions** in any of
the nine.

### A change-0137 guard was legitimately loosened

`tests/test_dispatch_capability.sh`'s floor went 2 → 1 because the rewording this change *ordered*
removed one of the two live Cursor-scoped matches, and it was that file's only one. Reviewed
specifically: the removed mention was the exact sentence the plan required reworded, and the test's
own maintainer note (confirm the sentence still lives, re-derive from the scan, never zero) was
followed. The side effect — that lowering the floor stranded the only live coverage of one third of
`$SHAPE` — was caught by mutation and fixed with a **hermetic positive-control record**, so the
coverage no longer depends on how live prose happens to be worded.

## Deviations from the plan

- **`runners.cursor` config keys not implemented.** The spec named a sandbox/force block. The
  adapter has no adapter-specific knobs today and `scripts/runners/cursor.md` says so;
  `runner-dispatch.sh` resolves and exports `DOCKET_RUNNER_CFG_*` generically, so adding one later
  costs nothing. Deliberate, disclosed, not deferred work.
- **The runner adapter stayed in.** It was the pre-agreed carve-out point, and it is the one item
  that is new capability rather than defect repair. It rests on a CLI the spec itself records as
  unreliable, and Tier 2 could not exercise it — so its real-world behavior is unproven by anything
  here. Its posture is right for that: every failure is a loud abort-and-report, never a fall-back.
- **`agent-layer.md` lost one historical aside** (why identical-on-every-clone pinning was retired)
  to fit its size budget without raising it. No normative claim was lost; the *why* is gone.

## Follow-ups minted

| # | Type | What |
|---|---|---|
| 0140 | fix | Normalize the `inherit` sentinel **once** for every runner adapter — the Codex twin of I-1, rooted in `emit_shim`. Folds in the `cursor.md` `--model` bullet, which does not yet record the normalization. |
| 0141 | refactor | Factor the shared wrapper-source parse out of `emit_cursor_md`/`emit_codex_toml` (~90% identical). ADR-0060 makes a third emitter likely, and that is the point to stop copying. |
| 0142 | fix | Make the unmapped-harness gap loud at generation time. `agents`, `kiro`, `windsurf` are accepted tokens that still get an unverified Claude-shaped wrapper with no runtime WARN — 0135's defect, still live for three tokens. |

## Parked, deliberately not fixed

- `scripts/runners/cursor.md`'s `--model` bullet does not record the `inherit` normalization the
  final fix wave added. Real contract/code drift in a repo where the co-located `.md` **is** the
  contract, but one line and not load-bearing — folded into #0140 rather than spending a second fix
  wave.
- Each dispatch fragment's demoted illustration still shows a Claude Code tool symbol inside a
  Cursor rule. The label demotes the snippet's *authority*, not its *priming*. Plan-mandated; a
  follow-up decision, not a defect.
- `tests/test_dispatch_capability.sh`'s cursor-rules exclusion rationale ("where the literal tool
  name is correct as written") is now stale — correct only inside a labelled illustration. The
  exclusion itself still holds; comment-only.
- The nine per-fragment "names no dispatch tool literal" asserts are mutation-proven live but never
  went red-to-green: the fragments never carried the literal in prose. The plan's expectation was
  wrong about where the violation lived — it was in the head's `description:` frontmatter and
  `sync-agents.sh`'s auto-block.
- `test_cursor_contract_docs.sh`'s six-phase assert counts `^### Phase [1-6]` occurrences, so a
  duplicated Phase 3 with a missing Phase 5 would pass.
