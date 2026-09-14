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

Invoke the `docket-convention` skill via the Skill tool first — unless already invoked this session — and run its *Step-0 preamble* (load the convention; run the capability bootstrap; run the `repository.prepare` operation with `--repo-dir <dir> --json` as its own Bash call; validate the protocol-v1 envelope and carry its typed context values forward as literals; act on the disposition). Everything below uses its vocabulary (needs-brainstorm, effective auto-groomable, autonomous-eligible, the abstain rule, …) without redefinition. All reads and writes land in the metadata working tree on `metadata_branch`, pushed to its remote immediately.

## Procedure — the drain loop

Repeat steps 1–5 until no autonomous-eligible stub remains; then step 6.

### Step 1 — Select

Sync the metadata working tree (the Step-0 `repository.prepare` operation). Rank every **autonomous-eligible** stub (per the convention: needs-brainstorm AND effective auto-groomable; unsatisfied `depends_on` does NOT exclude — design ahead, note the dependency state in the assumptions) by the deterministic selection order. Pick the top. None left → step 6. Read the selected stub's exact record `path` + `version` (blob object id) from the `status` operation (with `--json`) — the Step-4 groom transaction pins the record with those.

### Step 2 — Designer pass

Read the stub body, its `related`/`depends_on` neighbours (active + recently archived), the ADR index, and the relevant code. Read the learnings index `<changes_dir>/learnings/README.md` and pull any findings whose hook bears on the stub, so the self-brainstorm is informed by past lessons (skipped entirely when `learnings.enabled` is `false`). Enumerate the decision points an interactive brainstorm would raise. For each, weigh 2–3 approaches and COMMIT to the conservative / recommended default — do NOT invoke `superpowers:brainstorming` with a simulated human answerer (a subagent picking "the recommended option" is the model agreeing with itself while faking an approval gate; rejected at design time). Draft the spec to `.docket/docs/superpowers/specs/<UTC date>-<slug>-design.md` with an `## Assumptions` block: every decision, the chosen default, the rejected alternatives, and why — the human's deferred audit trail. If the stub is genuinely mechanical (no real design questions), the draft verdict is *trivial* instead of a spec, with the reasoning written for the critic.


### Step 3 — Critic pass

Dispatch the dedicated **`docket-auto-groom-critic`** subagent (foreground, at the model/effort its wrapper resolves) — a fresh subagent (never the designer reviewing itself), isolated in its own context, loading only `docket-convention` and never this designer skill — to adversarially attack the draft — specs and trivial verdicts alike. Per assumption, one verdict: **sound** (stands) · **wrong but fixable from available context** (designer revises; ONE bounded revision round; the critic re-checks only the revised items — this re-check is dispatched foreground exactly like the first pass, per the convention's *Composition* never-yield rule) · **needs human context** (⇒ the whole groom abstains — a spec must only be emitted when every decision in it is safe to auto-commit, because emission = build-ready = the autonomous builder will build it). If no dispatch mechanism resolves per the convention's *Dispatch-capability resolution* — never from a tool name — the `docket-auto-groom-critic` dispatch is **Tier B**: the groom **abstains** for that stub (→ Step 4's **Abstain** exit) rather than self-reviewing — an author cannot be their own adversarial gate.

**Receiving the verdict.** The verdict is read from the critic's **return** — its final report, which the groom is actively blocking on; the groom never backgrounds the critic. The groom never waits for a message, a notification, or any other out-of-band delivery: nothing is registered to deliver one, so that wait never ends.

**No-verdict posture (bounded — two steps, then out).** If the dispatch returns no legible verdict — a malformed return, pre-yield prose, or a backgrounded child's bare completion — make **one collect attempt** (read the child's completed final report where the harness surfaces it), and failing that **one fresh foreground re-dispatch** of the critic over the same draft, issued through whatever mechanism makes the parent block on the return — if none does, that leg would only repeat the first, so skip it straight to Tier B. Still no verdict ⇒ treat it as a failed dispatch attempt under the convention's *Dispatch-capability resolution*: **Tier B**, so the groom **abstains** for this stub (→ Step 4's **Abstain** exit in full, the `auto_groomable: false` flip included — left armed, the stub stays autonomous-eligible and the drain re-selects it, forfeiting *Termination & concurrency*), recording the return-channel diagnostic in the `## Auto-groom blocked` section, the human's re-arm cue. Never a third dispatch; never an indefinite wait. Re-dispatching a critic is safe where a build worker is not — it is read-only over prose, holds no worktree, and writes no git state, so `yielded-worker-return-closes-every-door`'s closed-doors analysis does not bind here.

### Step 4 — Exit (one of three)

1. **Spec** — every assumption survived: apply one atomic transaction — the `change.groom` operation with `--repo-dir .docket --request <request-file>` — carrying the change id, the pinned `path` + `version` from Step 1, `outcome: spec`, the `spec_markdown` (the settled design plus its `## Assumptions` block), the owned proposal-section rewrites (proposal altitude, resolved `## Open questions` removed), and the desired `depends_on`/`related`/`adrs`/`discovered_from`/`stacked_on`. In one metadata commit it writes the spec file, sets `spec:` + `updated:`, stamps the spec's `docket:backlink` block, and re-renders the `## Artifacts` block and inline board; a typed refusal (not-groomable, spec-path-taken, malformed markers, version mismatch) writes nothing. Build-ready.
2. **Trivial** — the critic confirmed no hidden design decisions: apply the `change.groom` operation with `--repo-dir .docket --request <request-file>` with `outcome: trivial`, the pinned `path` + `version`, and an owned-section edit carrying the tightened body and its reasoning as the trivial rationale. The transaction sets `trivial: true` + `updated:` and re-renders the `## Artifacts` block and inline board atomically. Build-ready, no spec.
3. **Abstain** — any needs-human-context verdict, or Step 3's exhausted no-verdict posture: emit NO spec; there is no typed groom for this outcome, so write it directly on the metadata tree — flip `auto_groomable: false` and append a dated `## Auto-groom blocked` section (the undecidable decision(s), what context is missing, what a human should supply, and any recommendation — including "this should probably be killed/deferred because …") — then commit that change-file edit with plain git plumbing (Step 5). The abstain changes no board-visible cell, so no board render is needed; the stub stays needs-brainstorm, first in `docket-groom-next`'s queue.

**Kill and defer are NEVER autonomous.** Verdict authority over the backlog's composition stays human; the strongest the drain may say is an abstain-with-recommendation.

### Step 5 — The outcome lands (no separate board pass)

For a **spec** or **trivial** exit the Step-4 `change.groom` operation is the whole write — it re-checks the pinned `version` and commits the record, the spec, the `## Artifacts` block, and the inline board in one metadata commit pushed under an exact-lease push, so there is **no separate Board pass**. On a `contended` refusal it writes nothing: re-sync (re-run the `repository.prepare` operation), re-read the stub's `path` + `version` from the `status` operation, and if it is no longer autonomous-eligible (groomed, killed, claimed, or opted out) DISCARD this iteration's draft (delete the just-drafted spec markdown) and loop; otherwise re-author and retry.

For an **abstain** exit, commit the change-file edit (`auto_groomable: false` + the `## Auto-groom blocked` section) with plain git plumbing in the metadata working tree; push `origin/docket`. **Stage by explicit path** — that tree is shared, so a bare `add -A` commits another agent's staged work under your message. On a non-fast-forward rejection: re-sync (re-run the `repository.prepare` operation), and if the rebase brought in commits touching this stub's file, RE-READ it — no longer autonomous-eligible ⇒ DISCARD this iteration's writes for it (`git -C .docket restore -- <changed paths>`) and loop. Loop to step 1.

### Step 6 — Report

Summarize the drain: groomed N (specs), trivial M, abstained K — each abstain with its one-line reason — plus anything skipped to a lost race. STOP. Grooming never implements; the build-ready output is `docket-implement-next`'s queue.

**Dummy mode:** when `DUMMY_MODE_ENABLED` is `true` (Step-0 export), write this drain's `reports` calibrated to `DUMMY_MODE_PERSONA`, and give any `change-sections` it writes (`## Auto-groom blocked`) an authored `### In plain terms` block alongside the full technical content, per the convention's *Dummy mode* shared definition.

## Termination & concurrency

Every exit shrinks the queue (spec/trivial ⇒ no longer needs-brainstorm; abstain ⇒ no longer effective auto-groomable), so the drain visits each stub at most once and provably terminates. No claim is taken — ADR-0004's final-push CAS stance, adopted for the autonomous case: its human-attended rationale does not apply here, but the load-bearing half does — each stub's writes land in a single final commit, so a late collision wastes minutes, not hours, and the post-rebase re-read is the arbiter.
