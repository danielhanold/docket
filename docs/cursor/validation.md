# Validating docket's Cursor harness — the CLI probe and the IDE checklist

`sync-agents.sh` writes docket's Cursor artifacts against **Cursor's own documented subagent
contract**: `.cursor/agents/docket-*.md` wrappers carrying `name`/`description`/`model` with the
reasoning effort bracket-encoded inside the model value and the skills list as a body preamble, plus
the `docket-dispatch.mdc` rule. The hermetic suite (`tests/test_sync_agents_cursor.sh`) proves docket
*emits* that shape. It cannot prove Cursor *honors* it — that takes a live harness.

This runbook carries the two tiers that cannot be automated. Tier 1 is the hermetic suite and is not
described here.

> **Provenance.** Cursor's subagent contract and its nesting limit are read from Cursor's published
> docs; the CLI observations below are empirical and version-scoped. Record the versions you observed
> alongside any finding, and re-verify if your Cursor differs.

## Tier 2 — cursor-agent probe (best-effort, non-gating)

A cheap CLI probe. Run it if `cursor-agent` is on your PATH; skip it if not.

```sh
cursor-agent --version                      # record this alongside any finding

cursor-agent -p --output-format json \
  "Dispatch the docket-status subagent. Have it report, as JSON:
   (1) the model it is running on and its reasoning effort,
   (2) whether the docket-convention skill and the docket-status skill loaded,
   (3) the result of dispatching one further subagent from inside itself."
```

What a *positive* run tells you: the wrapper resolved, the pinned model and bracket-encoded effort
reached the child, the linked skills loaded, and a nested dispatch is reachable on the CLI surface.

### The evidence rule

> **A negative or absent result from `cursor-agent` is never evidence that the wrapper contract is wrong.**
> It is recorded as a CLI limitation observation and nothing more. Only a *positive* result carries
> weight, and it proves only that the contract works on the CLI surface.

Why this rule exists, rather than the obvious reading: `cursor-agent` is an unreliable, feature-lagging
surface. Treating its silence as capability absence is the exact false-negative shape **ADR-0059**
exists to prevent — an absence observed in the wrong surface, promoted to a verdict. A CLI that cannot
dispatch does not tell you Cursor cannot dispatch; it tells you the CLI could not, today, at that
version.

A future implementer **must not re-promote this spike to a gate.** If Tier 2 keeps returning nothing
useful, the correct move is to delete the probe or leave it non-gating — never to convert its silence
into a failing check, and never to change the emitted wrapper shape on the strength of a negative CLI
result. The shape is settled by Cursor's published contract and certified by Tier 3.

## Tier 3 — Cursor IDE validation checklist (human-executed, required before merge)

The certifying tier. A human runs this in the Cursor IDE, in a repo opted in with
`agent_harnesses: [claude, cursor]`.

**Pass condition: passes when phases 1–3 and 5 are green and phases 4 and 6 have definitive observed
answers.** A phase that is merely "seemed fine" is not an answer. Every gap found becomes a follow-up
stub, not a silent note.

### Phase 1 — Generated artifacts

Run `./sync-agents.sh`. Open `.cursor/agents/docket-*.md`.

Observable outcome: every file's frontmatter carries `name`, `description`, `model` and **no**
`effort:` key and **no** `skills:` key; the `model` value carries the bracket encoding
(`<id>[effort=<e>]`) whenever an effort is pinned; the skills the agent needs appear as a preamble in
the **body**. `.cursor/rules/docket-dispatch.mdc` exists.

### Phase 2 — Agent visible

Open the Cursor agent picker.

Observable outcome: the docket agents are listed by name and selectable. An agent that generates but
does not appear is a wrapper Cursor rejected — capture the exact filename and frontmatter.

### Phase 3 — Dispatch honored

In a fresh Cursor session, ask for a docket agent's work by name (the path the `docket-dispatch.mdc`
rule governs).

Observable outcome: the request **dispatches to the subagent** — you can see the child run — rather
than the skill running inline in the parent chat. Inline execution here means the pin is defeated,
which is the whole reason the rule exists.

### Phase 4 — Pin honored

Ask the child, in its own run, to report the model and reasoning effort it is running at.

Observable outcome: the child reports **both** the pinned model **and** the pinned reasoning effort.
A child that reports the model but cannot report effort is a definitive *partial* answer — record it
as such; it is the bracket encoding's open question.

### Phase 5 — Skills loaded

Ask the child to confirm which skills it loaded, and to state one docket convention rule.

Observable outcome: the child names `docket-convention` and its own docket skill, **and** states a
rule it could only know from having loaded them (e.g. a manifest field's lifecycle semantics). A bare
"yes, loaded" is not evidence.

### Phase 6 — SDD reachable at depth 2

From inside the child, trigger a real dispatch of one further subagent.

Observable outcome: the nested dispatch runs and returns. Cursor documents a nesting limit of **two**,
and docket's SDD topology is **flat** — the orchestrator dispatches implementers, task reviewers, fix
subagents and the final reviewer as siblings, and the implementer never dispatches — so docket needs
exactly depth 2, which Cursor permits. This phase confirms live that the documented limit and docket's
actual need line up. A failure here is a definitive answer too, and a blocking one for SDD under
Cursor.

## The merge-gate obligation

Tier 3 necessarily runs **after** the PR opens — it needs the branch's generated artifacts in a live
IDE. A green hermetic suite therefore does not clear the human merge gate on its own.

The PR body for any change touching the Cursor wrapper contract **must state that Cursor IDE
validation is pending and name this checklist** (`docs/cursor/validation.md`), so the human at the
merge gate knows what has not been verified yet. Merging on a green suite alone is exactly how the
wrapper defect this runbook exists to prevent shipped in the first place.
