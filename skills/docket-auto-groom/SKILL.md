---
name: docket-auto-groom
description: Use when individual stubs opted into autonomous grooming (their `auto_groomable` is `true`) and you want the auto-groomable needs-grooming queue drained with no human — selecting each auto-groomable stub deterministically and designing it via a default-biased self-brainstorm gated by an adversarial critic, exiting each stub with a linked spec, a trivial verdict, or an abstain back to the human queue. Kill and defer are never autonomous. Writes markdown only — never branches, worktrees, or code.
context: fork
agent: docket-auto-groom
---

# docket-auto-groom — the autonomous groomer (drain)

## Overview

`docket-auto-groom` is `docket-groom-next`'s autonomous sibling. Same queue vocabulary, same exits where safe — but no human, and **drain semantics**: nobody is waiting between stubs, so one invocation keeps going until no auto-groomable stub remains, then reports. It keeps superpowers' brainstorming *reasoning* — enumerate the decision points, weigh approaches, commit to the conservative default — and replaces the *waiting-for-a-human protocol* with an audit trail (the spec's `## Assumptions` block) plus an adversarial critic that gates every build-ready exit. It writes markdown only: change files, specs, `BOARD.md` — never branches, worktrees, or code.

## When to use

- Needs-grooming stubs carrying `auto_groomable: true` are piling up.
- You want the backlog groomed to build-ready overnight / from a routine, with abstains waiting for you in the morning.
- Do NOT use for interactive design — that is `docket-groom-next`; the human there is the point.
- Do NOT use to capture new ideas (`docket-new-change` mints ids) or to re-groom a change that already has a spec (build-time reconcile owns drift).

## Convention (load first — blocking)

Invoke the `docket-convention` skill via the Skill tool first — unless already invoked this session — and run its *startup check* (load the convention; run the capability bootstrap; run the `repository.prepare` operation with `--repo-dir <dir> --json` as its own Bash call; validate the protocol-v1 envelope and carry its typed context values forward as literals; act on the disposition). Everything below uses its vocabulary (needs-grooming, auto-groomable, the abstain rule, …) without redefinition. All reads and writes land in the metadata worktree (`metadata_worktree_path` in the `repository.prepare` context) on the metadata branch, pushed to the metadata remote (`metadata_remote`) immediately.

## Procedure — the drain

Repeat steps 1–5 until no auto-groomable stub remains; then step 6.

### Step 1 — Select

Sync the metadata working tree (the startup-check `repository.prepare` operation). Rank every **auto-groomable** stub (per the convention: needs-grooming AND `auto_groomable: true`; unsatisfied `depends_on` does NOT exclude — design ahead, note the dependency state in the assumptions) by the deterministic selection order. Pick the top. None left → step 6. Read the selected stub's exact record `path` + `revision` (blob object id) from the `status` operation (with `--json`) — the Step-4 groom transaction pins the record with those.

### Step 2 — Designer pass

Read the stub body, its `related`/`depends_on` neighbours (active + recently archived), the ADR index, and the relevant code. Read the learnings index `<changes_dir>/learnings/README.md` and pull any findings whose hook bears on the stub, so the self-brainstorm is informed by past lessons (skipped entirely when `learnings.enabled` is `false`). Enumerate the decision points an interactive brainstorm would raise. For each, weigh 2–3 approaches and COMMIT to the conservative / recommended default — do NOT invoke `superpowers:brainstorming` with a simulated human answerer (a subagent picking "the recommended option" is the model agreeing with itself while faking an approval gate; rejected at design time). Draft the spec in session scratch space — never inside the metadata worktree — as the Markdown body that Step 4 sends as `spec_markdown` in the `change.groom` request file, with an `## Assumptions` block: every decision, the chosen default, the rejected alternatives, and why — the human's deferred audit trail. In a private repository the spec ships with the PR, so its body follows the private-repository writing rule: no `Change #N, groomed …` line, no change or ADR numbers, no backlog references. If the stub is genuinely mechanical (no real design questions), the draft verdict is *trivial* instead of a spec, with the reasoning written for the critic.


### Step 3 — Critic pass

Dispatch the dedicated **`docket-auto-groom-critic`** subagent (foreground, at the model/effort its wrapper resolves) — a fresh subagent (never the designer reviewing itself), isolated in its own context, loading only `docket-convention` and never this designer skill — to adversarially attack the draft — specs and trivial verdicts alike. Per assumption, one verdict: **sound** (stands) · **wrong but fixable from available context** (designer revises; ONE bounded revision round; the critic re-checks only the revised items — this re-check is dispatched foreground exactly like the first pass, per the convention's *Composition* never-yield rule) · **needs human context** (⇒ the whole groom abstains — a spec must only be emitted when every decision in it is safe to auto-commit, because emission = build-ready = the autonomous builder will build it). If no dispatch mechanism resolves per the convention's *Dispatch-capability resolution* — never from a tool name — the `docket-auto-groom-critic` dispatch's fallback is **`abstain`**: the groom **abstains** for that stub (→ Step 4's **Abstain** exit) rather than self-reviewing — an author cannot be their own adversarial gate.

**Receiving the verdict.** The verdict is read from the critic's **return** — its final report, which the groom is actively blocking on; the groom never backgrounds the critic. The groom never waits for a message, a notification, or any other out-of-band delivery: nothing is registered to deliver one, so that wait never ends.

**No-verdict posture (bounded — two steps, then out).** If the dispatch returns no legible verdict — a malformed return, pre-yield prose, or a backgrounded child's bare completion — make **one collect attempt** (read the child's completed final report where the harness surfaces it), and failing that **one fresh foreground re-dispatch** of the critic over the same draft, issued through whatever mechanism makes the parent block on the return — if none does, that leg would only repeat the first, so skip it straight to `abstain`. Still no verdict ⇒ treat it as a failed dispatch attempt under the convention's *Dispatch-capability resolution*: **`abstain`**, so the groom **abstains** for this stub (→ Step 4's **Abstain** exit in full, the `auto_groomable: false` flip included — left armed, the stub stays auto-groomable and the drain re-selects it, forfeiting *Termination & concurrency*), recording the return-channel diagnostic in the `blocked_note`, the human's re-enable cue. Never a third dispatch; never an indefinite wait. Re-dispatching a critic is safe where a build worker is not — it is read-only over prose, holds no worktree, and writes no git state, so `yielded-worker-return-closes-every-door`'s closed-doors analysis does not bind here.

### Step 4 — Exit (one of three)

1. **Spec** — every assumption survived: apply one atomic transaction — the `change.groom` operation with `--repo-dir <metadata_worktree_path> --request <request-file>` — carrying the change id, the pinned `path` + `revision` from Step 1, `outcome: spec`, the drafted `spec_markdown` (the settled design plus its `## Assumptions` block), the owned proposal-section rewrites (proposal altitude, resolved `## Open questions` removed), and the desired `depends_on`/`related`/`adrs`/`discovered_from`/`stacked_on`. In one metadata commit it writes the spec file, sets `spec:` + `updated:`, stamps the spec's `docket:backlink` block, and re-renders the `## Artifacts` block and inline board; a typed refusal (not-groomable, spec-path-taken, malformed markers, revision mismatch) writes nothing. Build-ready.
2. **Trivial** — the critic confirmed no hidden design decisions: apply the `change.groom` operation with `--repo-dir <metadata_worktree_path> --request <request-file>` with `outcome: trivial`, the pinned `path` + `revision`, and an owned-section edit carrying the tightened body and its reasoning as the trivial rationale. The transaction sets `trivial: true` + `updated:` and re-renders the `## Artifacts` block and inline board atomically. Build-ready, no spec.
3. **Abstain** — any needs-human-context verdict, or Step 3's exhausted no-verdict posture: emit NO spec; apply the `change.groom` operation with `--repo-dir <metadata_worktree_path> --request <request-file>` with `outcome: abstain`, the pinned `path` + `revision`, and a `blocked_note` — the undecidable decision(s), what context is missing, what a human should supply, and any recommendation (including "this should probably be killed/deferred because …"), with subsections at `###` or deeper. The transaction sets `auto_groomable: false` + `updated:`, appends a dated entry to the `## Auto-groom blocked` section, and re-renders the inline board — the row flips to **auto-groom blocked — needs you** in that same commit. It accepts no section, spec, or relationship edits. The stub stays needs-grooming, first in `docket-groom-next`'s queue.

**Kill and defer are NEVER autonomous.** Verdict authority over the backlog's composition stays human; the strongest the drain may say is an abstain-with-recommendation.

### Step 5 — The outcome lands (no separate board pass)

Every exit's Step-4 `change.groom` operation is the whole write — it re-checks the pinned `revision` and commits the record, any spec, the `## Artifacts` block, and the inline board in one metadata commit pushed under an exact-lease push, so there is **no separate Board pass** and no hand-staged commit. On a `contended` refusal it writes nothing: re-sync (re-run the `repository.prepare` operation), re-read the stub's `path` + `revision` from the `status` operation, and if it is no longer auto-groomable (groomed, killed, claimed, or opted out) DISCARD this iteration's draft (discard the draft from scratch space) and move on; otherwise re-author and retry. Return to step 1.

### Step 6 — Report

Summarize the drain: groomed N (specs), trivial M, abstained K — each abstain with its one-line reason — plus anything skipped to a lost race. STOP. Grooming never implements; the build-ready output is `docket-implement-next`'s queue.

## Termination & concurrency

Every exit shrinks the queue (spec/trivial ⇒ no longer needs-grooming; abstain ⇒ no longer auto-groomable), so the drain visits each stub at most once and provably terminates. No claim is taken — ADR-0004's conflict-checked final-push stance, adopted for the autonomous case: its human-attended rationale does not apply here, but the load-bearing half does — each stub's writes land in a single final commit, so a late collision wastes minutes, not hours, and the post-rebase re-read is the arbiter.
