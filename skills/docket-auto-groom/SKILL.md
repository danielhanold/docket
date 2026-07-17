---
name: docket-auto-groom
description: Use when a repo (or individual stubs) opted into autonomous grooming and you want the auto-groomable needs-brainstorm queue drained with no human — selecting each autonomous-eligible stub deterministically and designing it via a default-biased self-brainstorm gated by an adversarial critic, exiting each stub with a linked spec, a trivial verdict, or an abstain back to the human queue. Kill and defer are never autonomous. Writes markdown only — never branches, worktrees, or code.
context: fork
agent: docket-auto-groom
---

# docket-auto-groom — the autonomous groomer (drain)

## Overview

`docket-auto-groom` is `docket-groom-next`'s autonomous sibling. Same queue vocabulary, same exits where safe — but no human, and **drain semantics**: nobody is waiting between stubs, so one invocation loops until no autonomous-eligible stub remains, then reports. It keeps superpowers' brainstorming *reasoning* — enumerate the decision points, weigh approaches, commit to the conservative default — and replaces the *waiting-for-a-human protocol* with an audit trail (the spec's `## Assumptions` block) plus an adversarial critic that gates every build-ready exit. It writes markdown only: change files, specs, `BOARD.md` — never branches, worktrees, or code.

## When to use

- The repo sets `auto_groom: true` (or stubs carry `auto_groomable: true`) and needs-brainstorm stubs are piling up.
- You want the backlog groomed to build-ready overnight / from a routine, with abstains waiting for you in the morning.
- Do NOT use for interactive design — that is `docket-groom-next`; the human there is the point.
- Do NOT use to capture new ideas (`docket-new-change` mints ids) or to re-groom a change that already has a spec (build-time reconcile owns drift).

## Convention (load first — blocking)

Before anything else in this skill, invoke the `docket-convention` skill via the Skill tool — unless it was already invoked earlier in this session and its content is in context. Everything below uses its vocabulary (needs-brainstorm, **effective auto-groomable**, **autonomous-eligible**, the abstain rule, metadata working tree, …) without redefinition; no step below is executable without the convention loaded.

## Step 0

Run the convention's *Step-0 preamble*: load the convention, then run `docket.sh preflight` as its own Bash call and read the printed `KEY=value` block off stdout (it resolves config, enforces the bootstrap verdict fail-closed, and ensures + syncs the metadata working tree). All reads and writes land in that tree on `metadata_branch`, pushed to its remote immediately — `.docket/` on `origin/docket` in `docket`-mode; the primary working tree on `origin/<integration_branch>` in `main`-mode.

## Procedure — the drain loop

Repeat steps 1–5 until no autonomous-eligible stub remains; then step 6.

### Step 1 — Select

Sync the metadata working tree. Rank every **autonomous-eligible** stub (per the convention: needs-brainstorm AND effective auto-groomable; unsatisfied `depends_on` does NOT exclude — design ahead, note the dependency state in the assumptions) by the deterministic selection order. Pick the top. None left → step 6.

### Step 2 — Designer pass

Read the stub body, its `related`/`depends_on` neighbours (active + recently archived), the ADR index, and the relevant code. Read the learnings index `<changes_dir>/learnings/README.md` and pull any findings whose hook bears on the stub, so the self-brainstorm is informed by past lessons (skipped entirely when `learnings.enabled` is `false`). Enumerate the decision points an interactive brainstorm would raise. For each, weigh 2–3 approaches and COMMIT to the conservative / recommended default — do NOT invoke `superpowers:brainstorming` with a simulated human answerer (a subagent picking "the recommended option" is the model agreeing with itself while faking an approval gate; rejected at design time). Draft the spec to `.docket/docs/superpowers/specs/<UTC date>-<slug>-design.md` with an `## Assumptions` block: every decision, the chosen default, the rejected alternatives, and why — the human's deferred audit trail. If the stub is genuinely mechanical (no real design questions), the draft verdict is *trivial* instead of a spec, with the reasoning written for the critic.


### Step 3 — Critic pass

Dispatch the dedicated **`docket-auto-groom-critic`** subagent (foreground, at the model/effort its wrapper resolves) — a fresh subagent (never the designer reviewing itself), isolated in its own context, loading only `docket-convention` and never this designer skill — to adversarially attack the draft — specs and trivial verdicts alike. Per assumption, one verdict: **sound** (stands) · **wrong but fixable from available context** (designer revises; ONE bounded revision round; the critic re-checks only the revised items — this re-check is dispatched foreground exactly like the first pass: the designer blocks on the critic's return and never backgrounds it to await a notification, per the convention's *Composition* never-yield rule) · **needs human context** (⇒ the whole groom abstains — a spec must only be emitted when every decision in it is safe to auto-commit, because emission = build-ready = the autonomous builder will build it).

### Step 4 — Exit (one of three)

1. **Spec** — every assumption survived: set `spec:`, refresh the body to the settled design (proposal altitude), resolve `## Open questions`, set `updated: <UTC today>`. After writing `spec:`, regenerate the change's `## Artifacts` block: `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh render-change-links --change-file .docket/<changes_dir>/active/<id>-<slug>.md --adrs-dir .docket/<adrs_dir>` (the block edit rides with the spec-write commit; the renderer is the sole writer of the block). Build-ready.
2. **Trivial** — the critic confirmed no hidden design decisions: set `trivial: true`, tighten the body, log the reasoning in the body, set `updated:`. Build-ready, no spec.
3. **Abstain** — any needs-human-context verdict: emit NO spec; flip `auto_groomable: false` and append a dated `## Auto-groom blocked` section (the undecidable decision(s), what context is missing, what a human should supply, and any recommendation — including "this should probably be killed/deferred because …"). The stub stays needs-brainstorm, first in `docket-groom-next`'s queue.

**Kill and defer are NEVER autonomous.** Verdict authority over the backlog's composition stays human; the strongest the drain may say is an abstain-with-recommendation.

### Step 5 — Commit, push, board

Commit the stub's outcome (change-file edit + spec when emitted) in the metadata working tree; push `origin/docket`. On a non-fast-forward rejection: re-run `docket.sh preflight`, and if the rebase brought in commits touching this stub's file, RE-READ it — no longer autonomous-eligible (groomed, killed, claimed, or opted out elsewhere) ⇒ DISCARD this iteration's writes for it (`git -C .docket restore -- <changed paths>` for the change-file edit, and delete the just-drafted spec file — it is this iteration's own uncommitted artifact) and loop. Run the must-land Board pass: `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh docket-status --board-only --must-land` — a non-zero exit means the board did not land; STOP and surface it (abort-and-report). Loop to step 1.

### Step 6 — Report

Summarize the drain: groomed N (specs), trivial M, abstained K — each abstain with its one-line reason — plus anything skipped to a lost race. STOP. Grooming never implements; the build-ready output is `docket-implement-next`'s queue.

## Termination & concurrency

Every exit shrinks the queue (spec/trivial ⇒ no longer needs-brainstorm; abstain ⇒ no longer effective auto-groomable), so the drain visits each stub at most once and provably terminates. No claim is taken — ADR-0004's final-push CAS stance, adopted for the autonomous case: its human-attended rationale does not apply here, but the load-bearing half does — each stub's writes land in a single final commit, so a late collision wastes minutes, not hours, and the post-rebase re-read is the arbiter.
