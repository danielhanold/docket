<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0135 — Generated Cursor wrappers violate Cursor's subagent contract, disabling skills and model effort](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0135-cursor-agent-wrapper-contract.md)**
<!-- docket:backlink:end -->

# Cursor agent wrapper contract — design

Change: #0135 · Type: fix · Groomed: 2026-07-26

## Problem

Docket generates Cursor subagent wrappers through the **generic Claude-shaped emitter**. In
`sync-agents.sh`, `emit_for_harness()` routes `codex` to its own emitter and passes *everything
else* — including `cursor` — through `emit()`, which preserves the source wrapper's Claude
frontmatter verbatim:

```yaml
model: claude-opus-4-8
effort: xhigh
skills: [docket-implement-next, docket-convention]
```

Cursor's [subagent contract](https://cursor.com/docs/subagents) documents exactly five frontmatter
fields — `name`, `description`, `model`, `readonly`, `is_background` — and encodes reasoning effort
*inside* the model value (`claude-opus-5[effort=high]`). Neither a standalone `effort:` field nor a
`skills:` preload exists. Docket therefore emits two fields Cursor ignores and one pin Cursor cannot
read, while reporting all three as honored.

The observed consequence: a live `docket-implement-next` run under Cursor produced a plausible PR in
which `plan`, `build`, `review`, and `finish` had all silently degraded to their inline `auto`
fallbacks. The child had no docket instructions and no reason to load them, so
`superpowers:subagent-driven-development`'s fresh per-task implementers, TDD gates, and independent
review never ran. This is the third instance of the `skill-fallback-degrades-discipline` learning
(#0066, re-hit #0136): complete-looking artifacts concealing an unrun discipline.

## What this change is really for

ADR-0059 (from change #0137) decided the harness-neutral rule: dispatch capability is **resolved,
never inferred from a tool name**, and unavailability is **tiered** — with `skills.build` and
`skills.review` on the strictest tier, **authorized-or-halt** (an explicitly configured `auto` is the
human's authorization to run inline; any other configured value that cannot dispatch is
abort-and-report).

That rule currently **cannot reach a Cursor child at all**, because docket delivers the convention
through `skills:` — the one field Cursor ignores. #0137 fixed the Claude Code twin and delivered its
fix as docket-convention prose, through the very channel Cursor is broken on. **Repairing wrapper
delivery here is the prerequisite for any convention-level rule reaching Cursor.** The two changes
share the ADR, not the mechanism, and stay `related:` rather than `depends_on:` — neither gates the
other.

## Grooming findings that changed the stub's premises

Two of the stub's open questions were answered from Cursor's live documentation and from
`superpowers:subagent-driven-development`'s own flowchart, and one of them **invalidates a stated
fear**. Both are recorded here because the design depends on them.

### Finding 1 — the nesting fear was unfounded

The stub asked: *"What is Cursor's actual Task nesting limit? Docket's tree runs three deep (wrapper
→ SDD implementer → task reviewer). If the limit is below three, SDD genuinely cannot run on
Cursor."*

Cursor documents a limit of **two**: *"The main agent and its direct subagents can launch subagents,
but a subagent launched by another subagent can't launch further ones."*

But docket's tree does **not** run three deep. `superpowers:subagent-driven-development`'s topology
is **flat**: the orchestrator dispatches implementers, task reviewers, fix subagents, *and* the final
whole-branch reviewer. The implementer never dispatches — it implements, tests, commits,
self-reviews, and returns a status, after which the *orchestrator* writes the diff file and
dispatches the task reviewer. So the real tree is:

```
main chat  →  docket-implement-next wrapper   →  SDD implementer / task reviewer / fix / final reviewer
 (depth 0)      (depth 1, a direct subagent)      (depth 2, siblings — none of them dispatch)
```

Depth 2 is exactly what Cursor permits. **SDD is genuinely reachable on Cursor**; no halt posture is
required for nesting. The residual risks are tool-policy denial and mode restrictions ("Nested
launches also need Task tool access in the current mode, and hooks or tool policies can block
spawning"), which ADR-0059's resolve-then-attempt rule and Tier C already govern.

*Evidence scope: cursor.com/docs/subagents as of 2026-07-26, and superpowers 6.1.1's
`subagent-driven-development/SKILL.md`. Re-check both if either moves.*

### Finding 2 — skills are discoverable; only the preload field is fake

Cursor auto-discovers skills from `.cursor/skills/` and `~/.cursor/skills/` (among others), and
`link-skills.sh` **already symlinks every docket skill into `~/.cursor/skills/`**. Cursor documents no
frontmatter for attaching skills to an agent, but the skills are present and invocable.

So `skills: [...]` is not failing because the skills are absent. It is failing because it is an
ignored field with **no body instruction behind it** — the child is never told to load anything.
This is what makes the fix cheap: an instruction, not an installation.

## Design

### 1 — `emit_cursor_md()`: a named Cursor emitter

A new emitter in `sync-agents.sh`, structurally parallel to the existing `emit_codex_toml()` (same
extraction of `name` / `description` / `model` / `effort` / `skills` / body from the source wrapper,
same preamble-before-body assembly).

**Frontmatter.** Only Cursor's documented fields, and only three of the five:

```yaml
name: docket-implement-next
description: <verbatim from the source wrapper>
model: claude-opus-4-8[effort=xhigh]
```

`readonly` and `is_background` are **not emitted**. Their defaults are already correct for every
docket agent: the agents commit and push (so not readonly), and every docket dispatch is foreground
(so not background). Emitting them would assert a policy docket does not have.

**Model/effort encoding — verbatim passthrough, no validation.** Per ADR-0015 (harness-portable model
IDs are opaque passthrough) and ADR-0059's rejection of per-harness name tables as a class:

| resolved model | resolved effort | emitted |
|---|---|---|
| `claude-opus-4-8` | `xhigh` | `model: claude-opus-4-8[effort=xhigh]` |
| `claude-opus-4-8` | unset or `auto` | `model: claude-opus-4-8` |
| unset or `inherit` | unset or `auto` | *(no `model:` line)* |
| unset or `inherit` | `xhigh` | *(no `model:` line)* **+ generation-time WARN** |

Docket keeps **no allowlist of Cursor model IDs and no allowlist of effort tokens**. Cursor's own
documented compatible-model fallback handles anything it does not recognize. A committed table of a
vendor's internals is the exact artifact ADR-0059 rejected: it goes stale silently, and a stale entry
produces a *false negative* that reads as a successful degrade.

**The `inherit` + effort edge case is deliberately loud.** Effort has nowhere to attach without a
model, so the pin is dropped — and a dropped pin must never be silent, since silently-dropped pins
are the defect this whole change exists to fix. `sync-agents.sh` logs:

```
WARN cursor/docket-<name>: effort '<e>' dropped — Cursor encodes effort inside the model value,
     and the resolved model is 'inherit'. Set an explicit model to pin effort on Cursor.
```

**Body.** The skills preamble, then the source wrapper's body verbatim. Preamble emitted only when
the source has a non-empty `skills:` list, mirroring `emit_codex_toml`'s conditional:

```
Before acting, load these docket skills from your Cursor skills directory:
docket-implement-next, docket-convention.
```

Rejected alternative: inlining the full skill text into each wrapper. It makes wrappers thousands of
lines, drifts against the skill sources on every edit, and duplicates what `link-skills.sh` already
installs.

### 2 — `emit_for_harness()` gains named branches

```sh
case "$2" in
  codex)  emit_codex_toml "$1" "$3" "$4";;
  cursor) emit_cursor_md  "$1" "$3" "$4";;
  claude) emit            "$1" "$3" "$4";;
  *)      emit            "$1" "$3" "$4";;   # generic Claude-shaped wrapper — see note
esac
```

The `*)` branch keeps working exactly as today, but is documented in a comment as *"the Claude-shaped
generic wrapper. A harness reaching this branch has no verified contract mapping — its wrapper is a
best guess, not a supported shape."* That converts today's silent inheritance (which is how Cursor
got here) into a stated gap the next harness rollout has to confront.

### 3 — Dispatch rule rewording

`cursor-rules/dispatch.head.md` and the nine `cursor-rules/dispatch/docket-*.md` fragments currently
instruct `Launch a **Task** with subagent_type: ...` and show `Task(subagent_type: ..., ...)` call
snippets.

Reword the **instruction** to capability language — "dispatch to the subagent `docket-<name>`,
foreground, using this mode's subagent-launch mechanism; do not run it inline" — while keeping the
concrete call snippet as a clearly-labelled *illustration*. ADR-0059 §2 permits a tool name to appear
as an observed internal or in a diagnostic; it forbids one as a **decision input**. These fragments
tell a parent chat how to act, and are Cursor-scoped, so they were correctly left alone by #0137's
narrower prose fix — but standardizing them on capability language removes the last place a reader
could infer that docket depends on the name.

### 4 — `runners/cursor.sh`: a Cursor runner adapter

Mirrors `scripts/runners/codex.sh` (change 0079's framework), registered as
`REGISTERED_RUNNERS="codex cursor"`:

- Prompt assembled from `agents/docket-<name>.md` (skills list + body), as codex.sh does.
- Executes `cursor-agent -p --output-format text --workspace "$DOCKET_REPO_ROOT"`, foreground, and
  relays the final message on stdout.
- `--model` passed verbatim (ADR-0015). `--effort`, having no `cursor-agent` flag, rides inside the
  model value using the same `<model>[effort=<e>]` encoding as the wrapper emitter; when no model is
  resolved the effort is dropped with the same WARN.
- Mock seam `CURSOR_BIN`, matching `CODEX_BIN`. Contract doc at `scripts/runners/cursor.md`.
- Config block `runners.cursor` for sandbox/force flags, matching `runners.codex`.

The existing rule that `runner:` is reserved for a **claude parent** is unchanged and correct: the
runner names the *child* harness, so a `cursor` runner is a Claude wrapper delegating to
`cursor-agent`.

**Recorded risk.** `cursor-agent` is known (from prior hands-on testing) to be **unreliable and to lag
the Cursor IDE in features**. This adapter therefore rests on a shakier foundation than
`runners/codex.sh`. Its failure posture is pinned accordingly: a `cursor-agent` failure, timeout, or
missing-feature error is a **loud abort-and-report**, never a silent fall-back to running the agent
inline in the parent. The adapter must not become a new silent-degradation path — which would
reproduce this change's own root cause in a new location.

**Separable scope.** This item is the one piece of the change that is a *new capability* rather than
a repair of the wrapper-contract defect. If the PR proves unreviewable at full scope, this is the
clean carve-out point: items 1, 2, 3, 5, 6 form a coherent standalone fix, and the runner adapter can
be spun into its own change without weakening any of them.

### 5 — Documentation and ADR

- **`skills/docket-convention/references/agent-layer.md`** gains a per-harness wrapper-shape table, so
  the reference stops implying one uniform shape:

  | harness | file | model | effort | skills |
  |---|---|---|---|---|
  | claude | `.md` | `model:` | `effort:` | `skills:` frontmatter |
  | cursor | `.md` | `model: <id>[effort=<e>]` | *(inside model)* | body preamble |
  | codex | `.toml` | `model =` | `model_reasoning_effort =` | `developer_instructions` preamble |

- Any config/README prose asserting a uniform wrapper shape is corrected alongside it.

- **A new ADR** records the general rule this change generalizes: *a generated wrapper conforms to
  its target harness's own documented contract; the generic emitter is Claude's shape, not a default
  other harnesses may silently inherit, and a harness without a dedicated emitter is a known gap
  rather than a supported mapping.* `relates_to: [8, 15, 17, 59]`, `supersedes: []`, `reverses: []` —
  it **refines** ADR-0008 and ADR-0015's harness mappings without superseding either, and cites
  ADR-0059 as the governing decision for the dispatch/tiering question it does not re-open.

- **`adrs:` on this change** becomes `[8, 15, 24, 59, <new>]`. (Note: #0137's scope included adding
  59 to this change's `adrs:`; that had not landed as of grooming, so it is folded in here.)

## Verification — three tiers, with asymmetric weight on the CLI

### Tier 1 — hermetic contract tests (gating)

`tests/test_sync_agents.sh` currently asserts Cursor wrappers are byte-identical to Claude wrappers
and carry a standalone `effort:` field. Those assertions **encode the defect** and are replaced with
contract tests:

- a generated Cursor wrapper emits **no `effort:`** key;
- a generated Cursor wrapper emits **no `skills:`** key;
- the `model:` line uses bracket encoding when an effort is resolved, and bare when it is not;
- no `model:` line is emitted when the resolved model is `inherit`/empty;
- the `inherit` + effort case emits the WARN;
- the body preamble names **every** skill from the source wrapper's `skills:` list;
- Codex and Claude wrapper generation are unchanged (regression guard on the emitter split).

These gate the suite. They prove **shape**, and nothing else — which is precisely why Tiers 2 and 3
exist.

### Tier 2 — `cursor-agent -p` spike (best-effort, **non-gating**)

A scripted probe via `cursor-agent -p --output-format json` that dispatches a docket agent and reports
the effective model, whether the docket skills loaded, and whether a nested dispatch succeeds.
Findings recorded verbatim in the results file with the `cursor-agent` version.

**Evidence rule — this is the load-bearing part of Tier 2, not a caveat.** The CLI is known to be
unreliable and to lack features the IDE has. Therefore:

> **A negative or absent result from `cursor-agent` is never evidence that the wrapper contract is
> wrong.** It is recorded as a CLI limitation observation and nothing more. Only a *positive* result
> carries weight, and it proves only that the contract works on the CLI surface.

Treating an unreliable probe's silence as capability absence is the exact false-negative shape
ADR-0059 exists to prevent — an absence observed in the wrong surface, promoted to a verdict. This
rule is stated in the spec, in the test file, and in the results file so that a future implementer
cannot quietly re-promote this spike to a gate.

### Tier 3 — Cursor IDE validation checklist (human-executed, **required before merge**)

Modelled on change #0078's Codex CLI validation runbook, which is this repo's house pattern: a phased
checklist Daniel executes interactively, findings recorded verbatim in the results file, every gap
becoming a follow-up stub.

Phases, each with a definitive observable outcome:

1. **Generated artifacts** — run `./sync-agents.sh`; assert `.cursor/agents/docket-*.md` contain no
   `effort:`/`skills:` keys, carry bracket-encoded models, and carry the skills preamble.
2. **Agent visible** — the docket agents are listed and selectable in the Cursor IDE.
3. **Dispatch honored** — asking for a docket agent dispatches to the subagent rather than running
   the skill inline in the parent chat.
4. **Pin honored** — the child reports the pinned model *and* the pinned reasoning effort.
5. **Skills loaded** — the child confirms `docket-convention` and its own docket skill loaded, and can
   state a convention rule it could only know from having loaded them.
6. **SDD reachable at depth 2** — a real dispatch from inside the child succeeds, confirming Finding
   1 live rather than from documentation.

**Passes** when phases 1–3 and 5 are green and phases 4 and 6 have definitive observed answers.

### The merge-gate obligation

Tier 3 necessarily runs *after* the PR opens. So the change carries an explicit obligation: **the PR
body must state that Cursor IDE validation is pending**, naming the checklist, so the human merge
gate is not cleared on a green hermetic suite alone.

Without this, the change would become the fourth instance of `skill-fallback-degrades-discipline` —
green artifacts concealing an unrun verification — inside the very change written to end that
pattern.

## Out of scope

- Changing the Superpowers SDD or TDD skills themselves.
- Retrofitting or reopening the consuming `cet-devops` change 6 implementation or its PR.
- Completing change #0044's broader configurable per-role build-model design, beyond keeping its
  eventual configuration compatible with the corrected Cursor emitter.
- Redesigning Claude Code or Codex wrapper generation beyond the emitter split needed to name them.
- Re-deciding ADR-0059's capability-resolution rule or its tiered unavailability posture. This change
  **implements and delivers** that decision for Cursor; it does not reopen it.
- Other unvalidated harness tokens (`kiro`, `windsurf`) — they remain on the documented `*)` gap.

## Risks

- **Cursor's contract may move again.** The mitigation is structural, not a version pin: docket holds
  no table of Cursor internals, so a contract move breaks at most the emitter's field mapping, which
  Tier 1 tests localize, and Tier 3 re-detects on demand.
- **The CLI/IDE gap could mislead the build.** Addressed by the Tier 2 evidence rule and by making
  Tier 3 the certifying tier.
- **Scope.** Six items in one change. The runner adapter is the pre-agreed carve-out point.
