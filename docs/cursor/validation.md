# Validating docket's Cursor harness — the CLI probe and the IDE checklist

`sync-agents.sh` writes docket's Cursor artifacts against **Cursor's own documented subagent
contract**: `.cursor/agents/docket-*.md` wrappers carrying `name`/`description` (plus `model`
wherever one resolves, with the reasoning effort bracket-encoded inside the model value) and the
skills list as a body preamble, plus the `docket-dispatch.mdc` rule. The hermetic suite
(`tests/test_sync_agents_cursor.sh`) proves docket *emits* that shape. It cannot prove Cursor *honors* it — that takes a live harness.

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

> `agent_harnesses` is the explicit repository opt-in (change 0351). At install time
> `docket development install` (reached through `install.sh`) reconciles Cursor's wrappers and the
> `docket-dispatch.mdc` rule in Go; an *absent* key keeps the shipped Claude-only default, a
> *non-empty* list reconciles exactly the harnesses named, and an *explicit empty* list
> (`agent_harnesses: []`) retires every docket-owned repository surface. Running `./sync-agents.sh`
> directly, as Phase 1 does, still regenerates the same artifacts for inspection. Start a fresh
> Cursor session after any change to a wrapper or the dispatch rule — Cursor registers agents at
> process start, so clearing a conversation is not enough.

**Pass condition: passes when phases 1–3 and 5 are green and phases 4, 6, and 7 have definitive
observed answers.** Phase 7 applies only to a repo running `skills.build: docket-build`. A phase that
is merely "seemed fine" is not an answer. Every gap found becomes a follow-up stub, not a silent
note.

### Phase 1 — Generated artifacts

Run `./sync-agents.sh`. Open `.cursor/agents/docket-*.md`.

Observable outcome: every file's frontmatter carries `name` and `description`, and **no** `effort:`
key and **no** `skills:` key; the skills the agent needs appear as a preamble in the **body**.
`.cursor/rules/docket-dispatch.mdc` exists.

The `model:` key is present only where a model resolves. Docket's shipped
`agents/harness-defaults.yml` maps Cursor for **all thirteen wrappers**, so with no Cursor `agents:`
config of your own, every file carries a `model:` line holding the Cursor ID that harness-defaults
ships for it.

A **Claude** model ID appearing in a Cursor wrapper is the cross-harness leak this design removed;
treat it as a defect, not a default. The one deliberate exception is `docket-build-max`, whose
shipped Cursor ID *is* `claude-opus-5-high` — that is Cursor's own name for the model, selected
through Cursor, not a leaked Claude Code pin. A **missing** `model:` line is also a defect now:
before this change nine wrappers shipped unpinned, and that is no longer the design.

Where an effort *is* pinned by your own config, the `model` value carries the bracket encoding
(`<id>[effort=<e>]`). Every shipped Cursor ID already encodes its variant, so they all ship at
`effort: auto` and carry no bracket.

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

### Phase 7 — Profile-routed build under Cursor (required when `skills.build: docket-build`)

Docket ships Cursor model IDs for every wrapper, the four build profiles among them, so a Cursor
repo can run a profile-routed build with no configuration. That routing is what these checks certify; none of them
can be run by an autonomous build, and `cursor-agent` is not an accepted substitute.

Run a real `docket-build` on a plan with at least four tasks, in the Cursor IDE:

1. **Explicit routing, all four profiles.** A task carrying `**Build profile:** economy` lands on
   `docket-build-economy`; likewise `standard`, `premium`, and `max` on their own workers. Observable
   outcome: four dispatches, four distinct agent names, each child reporting the Cursor model its
   wrapper resolved — not the session model, and not a Claude ID.
2. **One auto-classified task.** A task with no `**Build profile:**` line is routed by the
   classifier. Observable outcome: the controller names the profile it chose and why, and the child
   that runs is that profile's agent.
3. **One bounded escalation.** A task that a worker returns `NEEDS_ESCALATION` on retries exactly
   once, one tier up, and never climbs twice. Observable outcome: two dispatches for that task, the
   second at the next tier, and a halt (not a third dispatch) if the second also fails.

Record the Cursor version and each observed model ID. Anything short of a definitive observed answer
is a gap, and becomes a follow-up stub.

## The merge-gate obligation

Tier 3 necessarily runs **after** the PR opens — it needs the branch's generated artifacts in a live
IDE. A green hermetic suite therefore does not clear the human merge gate on its own.

The PR body for any change touching the Cursor wrapper contract **must state that Cursor IDE
validation is pending and name this checklist** (`docs/cursor/validation.md`), so the human at the
merge gate knows what has not been verified yet. Merging on a green suite alone is exactly how the
wrapper defect this runbook exists to prevent shipped in the first place.
