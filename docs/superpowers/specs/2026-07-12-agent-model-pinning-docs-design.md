# Design — #0065 Document the two invocation paths and per-agent model pinning; ADR the `context: fork` findings

- **Change:** 0065 · `agent-model-pinning-docs`
- **Date:** 2026-07-12
- **Author:** `docket-auto-groom` (autonomous designer pass; every decision below is a defaulted assumption, audited in *Assumptions*)
- **Related:** #16 (agent layer), #45/#46 (multi-harness / per-harness models), #61 (`context: fork` dispatch), #62 (finalize merge authorization) · ADR-0008, ADR-0017, ADR-0020, ADR-0024

## Problem

Change 0061 shipped `context: fork` + `agent: docket-<name>` on the four headless-safe autonomous skills, so a *direct* invocation (`/docket-status`) forks into the pinned wrapper instead of running inline at the session model. It works. Two things about it are undocumented, and both bit a user on 2026-07-12:

1. **The fork is invisible.** A forked skill returns to the parent as a Skill tool result (`completed (forked execution)`) with no expandable box in the TUI. The natural conclusion is that the fork silently failed and the skill ran inline at the session model. It did not — the run happened, at the pin, and its full transcript exists on disk. Nothing says so, and nothing says the *other* invocation path (dispatching the wrapper agent, `@docket-status`) yields the identical pinned run **plus** a drillable subagent.
2. **0061's open question was never answered in the ledger.** "Does `context: fork` compose with the wrapper's `skills:` preload?" was settled empirically on 2026-07-12 (four probe skills + one live in-session invocation, Claude Code 2.1.207) — it does, safely — but the evidence lives in a chat log, not the ADR ledger.

Separately, the feature these mechanics serve — **per-agent model/effort pinning** — is docket's most load-bearing and least explained capability. It is what lets a board refresh run on haiku while a build runs on opus/xhigh, in one session, with no model choice by the human. The README documents *how to configure* it (`## Tuning agent models & effort`) and never says *why it matters*.

This change is **documentation plus one ADR**. It touches no script, no schema, no skill behavior.

## The verified findings (inputs, not decisions)

Established empirically on Claude Code 2.1.207 and taken as given by this design:

- `context: fork` **is honored**, including when the skill is reached through the Skill tool — a real subagent is spawned, with its own `subagents/agent-<id>.jsonl` and `agentType` metadata.
- The wrapper's **`model`/`effort` pin is honored inside the fork** (haiku-pinned wrappers ran at `claude-haiku-4-5` under an opus/sonnet parent).
- The **self-preloading cycle is safe**: an agent whose `skills:` preloads the very skill that forks into it neither recurses nor degrades to inline. This closes 0061's open question — and confirms ADR-0024's *stated but untested* no-recursion argument (preload is startup content injection; fork fires on invocation).
- A forked run is **not drillable in the TUI**, unlike an Agent-tool dispatch. Its transcript is reachable only on disk, at `~/.claude/projects/<project-slug>/<session-id>/subagents/agent-<id>.jsonl`.
- **Skills and agents are registered at process start.** A session that predates a frontmatter or wrapper change runs the *old* definition — the failure mode that made the fork look broken.

## Design

### 1. ADR-0026 — accept fork opacity; two invocation paths, no tooling

A **new** ADR (next free id: **0026** — the highest on the metadata branch is `0025`; `main`'s ledger gaps are unpublished ADRs, not free ids), `relates_to: [8, 17, 20, 24]`, `change: 65`. It **supersedes and reverses nothing** — ADR-0024's decision (fork the four headless-safe skills via native frontmatter) stands unchanged.

`relates_to` **keeps ADR-0017 and adds ADR-0020.** ADR-0020 supersedes only 0017's *committed-generation* decision (that generated wrapper files are committed) and explicitly **keeps** the rest — including the always-full-set rationale and the by-construction dispatch-target guarantee (ADR-0020, "This ADR supersedes ADR-0017's committed-generation model … while explicitly keeping"). So the **Cursor dispatch rule** — the very mechanism ADR-0026 is the Claude-Code counterpart to — is still **0017's** decision, merely preserved by 0020. ADR-0024 sets the precedent: dated two days *after* 0020 landed, it deliberately cites `relates_to: [8, 17]` and names 0017 as "the Cursor dispatch rule / full agent set that this is the Claude-Code-native counterpart to." Dropping 0017 would lose the pointer to the ADR that actually defines the contrasted mechanism.

Its **decision** is not "we ran an experiment" (an ADR records a decision, not a test report). It is the decision the findings force:

> **Fork-dispatch's opacity is accepted, not fixed.** A forked run is unobservable in the TUI by design of the harness; docket will not add tooling to compensate (no log-tailer, no wrapper-side progress protocol). Instead docket **documents two first-class invocation paths** into the same pinned wrapper — skill-invoke (`/docket-status`) and agent-dispatch (`@docket-status`) — and names the on-disk transcript path as the escape hatch. Observability is a *choice the caller makes at invocation time*, not a property docket engineers.

`## Context` carries the five verified findings above (version-stamped: Claude Code 2.1.207, 2026-07-12), including the closure of 0061's open question. `## Consequences` carries:

- Both paths produce an identical pinned run; they differ only in **observability** (agent-dispatch is drillable) and **cost** (agent-dispatch spends a dispatch turn; the fork does not).
- The self-preload cycle is verified safe → ADR-0024's no-recursion argument is now evidence-backed, and the composition wiring (implement-next → status/adr) needs no guard.
- The on-disk transcript path is an **observed Claude Code internal, not a contract** — version-stamped, may move, and docket depends on it for nothing (it appears in prose only).
- **Process-start registration**: after `sync-agents.sh` or any skill-frontmatter edit, an existing session keeps running the old definition — restart the harness process. This is the caveat that makes a working fork *look* broken.
- No `sync-agents.sh` change, no new script, no new generated file.

**ADR-0024 gets a dated `## Update` note pointing forward to ADR-0026.** The index renderer (`scripts/render-adr-index.sh`, `row()`) renders only each ADR's **own forward** fields — it derives no back-links (ADR-0008's row lists "relates to ADR-0001, ADR-0003" and never mentions the nine later ADRs that relate *to* it). So 0026's `relates_to: [24]` buys a reader of **0024** nothing: they would land on the fork decision and never learn that its open question was closed or that its opacity was subsequently accepted. The convention's escape hatch is exactly this case — an Accepted ADR is immutable *but for its `status:` line*, with a non-reversing context change appended as a **dated `## Update` note, never an edit to the decision**. One note, two sentences, zero words of 0024's `## Context`/`## Decision`/`## Consequences` touched:

> `## Update — 2026-07-12` — the composition question this ADR argued but did not test is now verified, and the fork's TUI opacity is accepted as a documented trade rather than tooled around. See ADR-0026.

**Build note.** Per the known auto-mode classifier block on publishing an ADR onto `main` mid-run, the `docket-adr` dispatch is scoped to `origin/docket` only; the ADR ids go in the change's `adrs:` and finalize publishes them to `main` at merge.

**`adrs: [24, 26]` is load-bearing, not bookkeeping — do not "tidy" 24 out of it.** Terminal close-out publishes *the `Accepted` ADRs listed in `adrs:`* onto the integration branch. ADR-0024 stays `Accepted`, so **listing 24 is the only thing that carries its new `## Update` note to `main`** — drop it and the note lives on `origin/docket` forever while `main`'s copy of 0024 silently lacks the forward pointer this design depends on.

### 2. README — the two invocation paths

**Follow the section's existing idiom: a `**bold-lead**` paragraph block, NOT a new `###` heading.** `## Tuning agent models & effort` (L385–431) today contains **zero `###` subsections** — it is a flat run of a numbered how-to (*1. Edit a config layer* → *2. Refresh the generated wrappers* → *3. Guard drift in CI*) interleaved with bold-lead paragraphs (*Always the full set…*, *Two mechanisms for one inline quirk.*, *The clone-identical guarantee is retired.*). Introducing an `###` mid-section would silently swallow every paragraph below it — including the unrelated *clone-identical guarantee* closer — into the new subsection. Match the idiom instead; the section stays flat.

So: a **`**The two invocation paths.**`** block inserted **between** the existing *Two mechanisms for one inline quirk.* paragraph (which it continues — that one explains how the *pin* survives a direct invocation; this one explains what the caller *sees*) and the *clone-identical guarantee* paragraph, which stays a top-level sibling exactly where it is. Content:

| Path | How | You get | You give up |
|---|---|---|---|
| **Skill-invoke** | `/docket-status` (or the model auto-invoking the skill) | The pinned run, forked; cheapest — no dispatch turn | Opaque: returns as `completed (forked execution)`; no drill-down in the TUI |
| **Agent-dispatch** | `@docket-status` / a `Task` dispatch at the wrapper | The **identical** pinned run, drillable live in the TUI | One dispatch turn of overhead |

Plus, in prose: the fork's transcript path on disk (`~/.claude/projects/<project-slug>/<session-id>/subagents/agent-<id>.jsonl`), explicitly flagged as an observed Claude Code internal rather than a stable interface; a "reach for agent-dispatch when you want to watch a long run (a build), skill-invoke for everything else" rule of thumb; and the **restart-your-session** caveat (skills and agents are registered at process start — after `sync-agents.sh` or a skill edit, the running session still holds the old definitions, which is what makes a healthy fork look like a silent failure).

Cursor's path is unaffected and gets one clause: its generated dispatch rule already routes a direct invocation through a real `Task` dispatch, so Cursor users are always on the drillable path.

### 3. README — why pin models per agent

A **`**Why pin a model per agent.**`** paragraph block opening `## Tuning agent models & effort`, sitting between the h2 and the section's existing lead paragraph — again **no `###`**, for the reason in §2 (a heading at the top of the section would nest the entire 1/2/3 how-to under "motivation"). The section keeps one flat outline: motivation, then the existing lead, then the numbered how-to.

Teaching altitude, written for a reader who has never considered that one session can span several models:

- The default mental model — *one session, one model* — makes you pay opus prices to regenerate a board and think at haiku depth while designing a build. Both are the same mistake: the model was matched to the **session**, not to the **task**.
- docket's unit of work is the **skill**, and every autonomous skill declares its own tier. A `docket-status` sweep is mechanical file bookkeeping (cheap tier, low effort); a `docket-implement-next` build is deep reasoning (top tier, high effort). They run in the **same session**, minutes apart, at different models, with no human choosing.
- `agents:` in `.docket.yml` (or the global / machine-local layer) is how you express that; the generated wrapper is how it is enforced; `context: fork` and the Cursor dispatch rule are how it survives a direct invocation.
- Short worked illustration of a single loop spanning tiers (groom → build → sweep), naming the tiers only as *cheap/mid/top* rather than literal model ids — literal tiers live in the config, and restating them in prose is exactly how docs drift from an override.

Also: one bullet added to the README's top-of-file **What you get** list, linking to the section, so the idea is discoverable from the first screen rather than 385 lines down.

**No new file.** README stays the single prose home (see *Assumptions* A3).

### 4. `references/agent-layer.md` — the mechanics propagate

`skills/docket-convention/references/agent-layer.md` is the canonical agent-layer reference and a *blocking read* for anyone configuring the layer; it already carries the two-mechanism story (0061 added it there after a review caught exactly this drift). Add a short paragraph in `## Always-full-set generation + the Cursor dispatch rule` stating that **both invocation paths land on the same pinned wrapper** — a forked skill-invoke and an explicit agent dispatch are equivalent as far as the pin is concerned, differing only in observability — and the process-start registration caveat. Mechanics only, ~6 lines; the teaching and the table stay in README, the decision stays in the ADR. Three homes, three altitudes, no duplication.

`skills/docket-convention/SKILL.md` is **not** touched: its *Agent layer* section already states the dispatch story at contract altitude, and invocation-path observability is mechanics, not contract.

## Scope

**Files touched**

| File | Change |
|---|---|
| `docs/adrs/0026-<slug>.md` | New ADR (via `docket-adr`; id 0026) |
| `docs/adrs/0024-claude-context-fork-skill-dispatch.md` | Dated `## Update` note only — decision text untouched |
| `docs/adrs/README.md` | Regenerated index (script-owned) |
| `README.md` | Two bold-lead paragraph blocks inside `## Tuning agent models & effort` (no new headings); one bullet in *What you get* |
| `skills/docket-convention/references/agent-layer.md` | ~6-line mechanics paragraph |
| `tests/test_skill_fork_dispatch.sh` | Doc sentinels (positive anchors) for the new README + reference prose |
| change `0065` frontmatter | `adrs: [24, 26]` |

**Out of scope** (inherited from the stub, unchanged): replacing `context: fork` with a thin-dispatcher SKILL.md or any change to how skills are invoked; any change to `sync-agents.sh`, wrapper generation, or the `agents:` schema; a helper script to tail a running fork's log; changing which skills are forked (0061's fork-exclusion principle stands).

**Tests.** **Extend `tests/test_skill_fork_dispatch.sh`** — the suite that already owns this change's subject matter — with doc sentinels in the repo's established positive-anchor form (see A6):

- **README** carries the two-invocation-paths framing and the on-disk transcript path (anchor on the *meaningful framing*, e.g. the agent-dispatch/drillable contrast and the `subagents/` path fragment — not on incidental wording).
- **README** carries the process-start / restart-your-session caveat.
- **`skills/docket-convention/references/agent-layer.md`** carries the "both paths land on the same pinned wrapper" statement — the file whose staleness 0061's review caught, so the anchor is the guard against a repeat.

Precedent for asserting authored English in the root README: `tests/test_consultant_brainstorm.sh:26-29`, `tests/test_docket_metadata_branch.sh:98-101`, `tests/test_ensure_claude_settings.sh:109-110`, `tests/test_results_artifact.sh:45`.

The 4/3 fork invariant remains guarded by the same suite's existing assertions. Note that `tests/test_adr_checks.sh` and `tests/test_render_adr_index.sh` are **fixture-only** (they synthesize ADRs into `mktemp -d` and never read `docs/adrs/`), so they give ADR-0026 no coverage — the ADR is reviewed by a human at the merge gate, which is the right gate for a decision record; no ledger test is added.

**Dependencies.** None. #62 (autonomous finalize merge authorization) is referenced as the reason `docket-finalize-change` stays unforked, but is not a dependency — the README already carries that clause and this change only cross-references it.

## Assumptions

Every decision below was defaulted autonomously. Each records the chosen default, the rejected alternatives, and why.

**A1 — The findings get a NEW ADR (0026) that EXTENDS ADR-0024, plus a dated `## Update` note on 0024 pointing at it.**
*Chosen:* both — a new parallel ADR carrying the decision, and a two-sentence forward pointer appended to 0024. *Rejected:* (a) the `## Update` note **alone** — cheaper, but it buries a real decision (accept the opacity, add no tooling) inside another ADR's context, leaving nothing citable; (b) the new ADR **alone**, on the theory that the index renders a back-link — **it does not**: `scripts/render-adr-index.sh`'s `row()` emits only each ADR's own *forward* fields, so a reader of 0024 would never learn its open question was closed (ADR-0008's row proves it — nine ADRs relate to it, none appear); (c) superseding 0024 — wrong on the facts: its decision is *confirmed*, not replaced, and the stub itself leans "extend."
*Why:* the two rejected singles fail in opposite directions (no citable decision / no discoverability); together they cost one extra note. The `## Update` is the convention's own sanctioned move for a non-reversing context change on an Accepted ADR, and it touches no decision text — 0024 stays immutable where immutability means something.

**A2 — The ADR's decision is framed as "accept the opacity, document two paths, add no tooling" — not as a test report.**
*Chosen:* a decision-shaped ADR whose `## Context` carries the five findings. *Rejected:* an ADR whose decision is "we verified that `context: fork` works," which is a finding, not a choice — an ADR ledger of experiments erodes into a changelog. *Why:* the convention's ADR shape (Context = forces, Decision = the rule a reader needs) only admits the former, and there **is** a real choice here — docket could have built a log-tailer or a progress protocol, and is deciding not to. Risk: if a reviewer wanted a pure findings record, this reframes their intent — but the findings survive verbatim in `## Context`, so nothing is lost.

**A3 — The model-pinning explainer lands in README, not a new `docs/` guide.** *(The stub's second open question — no lean given.)*
*Chosen:* README, as bold-lead paragraph blocks inside the existing `## Tuning agent models & effort` (no new headings — see §2). *Rejected:* (a) a new `docs/guides/model-pinning.md` linked from README, on README-length grounds (484 lines today, ~540 after); (b) putting the teaching in `references/agent-layer.md`. *Why:* README is the repo's **only** prose document on `main` (`docs/` holds only changes, adrs, results, superpowers — there is no guides tree, so (a) would create the precedent as a side-effect of a doc change). README already carries teaching-altitude sections (*Why docket*, *The reconcile superpower*), so this is the established home for exactly this register. And the project's own build-loop memory is emphatic that agent-layer prose living in several homes is how it drifts — 0061's review caught precisely that. Keeping every agent-model word inside one h2, with a discoverability bullet up top, is the lower-drift option. **~540 lines is the accepted cost.** *Reversal is cheap:* if the section proves unwieldy, extracting it to `docs/` later is a pure move.

**A4 — The on-disk transcript path is documented, but explicitly as an unstable Claude Code internal.**
*Chosen:* name `~/.claude/projects/<project-slug>/<session-id>/subagents/agent-<id>.jsonl` in prose, version-stamped (2.1.207), flagged as observed-not-contracted. *Rejected:* (a) omitting it — but "the log exists and here is where" is the single fact that would have saved the user's 2026-07-12 confusion, so omitting it defeats the change; (b) documenting it as a stable interface, or shipping a helper that reads it — the stub puts a tail helper explicitly out of scope, and any code depending on the path would break silently on a harness upgrade. *Why:* prose can carry a caveat that code cannot; a stale sentence misleads, a stale script breaks.

**A5 — The mechanics also propagate to `references/agent-layer.md`; `docket-convention/SKILL.md` is left alone.**
*Chosen:* a short mechanics paragraph in the reference, nothing in the contract. *Rejected:* (a) README only — but that is the exact drift 0061's whole-branch review flagged (the reference framed the inline-defeat as Cursor-only long after the fix), and LEARNINGS carries the rule to grep for a stale framing when correcting one; (b) also updating `SKILL.md` — its *Agent layer* section is contract altitude ("the pin holds either way; mechanics live in the reference") and is already correct, so an edit would duplicate, not clarify. *Why:* one fact, three altitudes — decision (ADR), teaching (README), mechanics (reference) — with the contract already pointing at the mechanics.

**A6 — Doc sentinels, in the repo's positive-anchor form, added to `tests/test_skill_fork_dispatch.sh`.**
*Chosen:* extend the suite that already owns the fork subject matter with a handful of positive-anchor greps over the README and `references/agent-layer.md`. *Rejected:* (a) **no test at all** — the earlier default here, and it was wrong on both of its premises: the repo *does* assert authored README English in at least four suites (`test_consultant_brainstorm.sh:26-29` is the near-exact precedent — change 0056's docs surface, guarded by README sentinels), and ADR-0026 would get **zero** coverage, because `test_adr_checks.sh` / `test_render_adr_index.sh` are fixture-only (`mktemp -d` + a synthesized `mkadr`; they never read `docs/adrs/`); (b) a **new** test file — the fork story already has a home, and a second one splits the invariant; (c) a **negative/brittle** grep pinning incidental wording.
*Why:* the whole change exists because a doc gap cost a user an afternoon, and 0061's own review caught `references/agent-layer.md` sitting stale for exactly this reason — an untested doc claim is the failure mode this change is *about*. LEARNINGS does not forbid doc sentinels; it prescribes their form (assert doc intent with a **positive anchor on the meaningful framing**), which is what these are. The prose stays freely rewritable; only the framing is pinned.
*(The rejected default also mis-cited ADR-0012 — that boundary governs skill-pass work, script vs. model; it says nothing about doc tests.)*

**A7 — Teaching prose names tiers as cheap/mid/top, never literal model ids.**
*Chosen:* abstract tiers in the worked illustration. *Rejected:* concrete ids (`haiku` for status, `opus/xhigh` for implement-next) — more vivid, and the ADR *does* use one concrete id as evidence of the pin firing. *Why:* the convention's own rule — literal tiers are never restated in prose, so an override can never drift from the docs (an `agents:` retune, like #42, would otherwise silently falsify the README). The ADR is exempt because it records a dated observation, not current configuration.

**A8 — No `depends_on`; the change is groomed ahead of nothing.**
*Chosen:* leave `depends_on: []`. *Rejected:* depending on #62. *Why:* #62 decides whether `docket-finalize-change` becomes forkable; this change only *cross-references* the current exclusion, wording already present in the README. If #62 lands first, the fork set changes and this change's reconcile pass picks that up — the ordinary build-time reconcile, not a hard dependency.

## Open questions

None outstanding — both of the stub's open questions are resolved above (A1 for the ADR relationship, A3 for the explainer's home) and are recorded as defaulted assumptions for a human to audit.
