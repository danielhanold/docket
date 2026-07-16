---
name: docket-groom-next
description: Use when stubs are sitting at needs-brainstorm on the docket board and you want the next one designed — selecting the next needs-brainstorm change (proposed, no spec, not trivial) deterministically and grooming it to build-ready through an interactive brainstorm with the human, exiting with a linked spec, a trivial verdict, a kill, or a defer. Selection is autonomous; the design conversation is not. Writes markdown only — never branches, worktrees, or code.
---

# docket-groom-next — the groomer (interactive)

## Overview

`docket-groom-next` drains the needs-brainstorm queue. `docket-new-change`'s scan mode captures ideas on the go as lightweight stubs; this skill is the later brainstorm pass that turns them build-ready. It mirrors `docket-implement-next`'s shape — a "next" skill over a queue — but the queue is needs-brainstorm stubs, the work is an interactive design conversation with the human, and the exit is a build-ready `proposed` change, not an open PR. One stub per invocation; loop by re-invoking. It writes markdown only: the change file, a spec, and a refreshed `BOARD.md` — never branches, worktrees, or code.

## When to use

- Stubs show as needs-brainstorm on the board and you want to design the next one.
- You want to groom a specific stub now (pass its id explicitly to skip selection).
- Do NOT use to capture a brand-new idea — that is `docket-new-change`'s job; this skill never mints ids.
- Do NOT use to re-groom a change that already has a spec — drift against current reality is the reconcile pass's job in `docket-implement-next`. A human who wants to redo a design can clear `spec:` by hand first.

## Recommended model/effort (advisory)

This skill grooms interactively with a human, so it cannot be a fire-and-forget subagent and cannot force the session model. **Recommended: `claude-sonnet-5` / `high`** (the cold-start recap is genuine synthesis). Set `/model claude-sonnet-5` and `/effort high` to match; this is advisory only — the human owns the session.

## Convention (load first — blocking)

Before anything else in this skill, invoke the `docket-convention` skill via the Skill tool — unless it was already invoked earlier in this session and its content is in context. Everything below uses its vocabulary (needs-brainstorm, build-ready, metadata working tree, the bootstrap probes, …) without redefinition; no step below is executable without the convention loaded.

## Step 0

Run the convention's *Step-0 preamble*: load the convention, then run `docket.sh preflight` as its own Bash call and read the printed `KEY=value` block off stdout (it resolves config, enforces the bootstrap verdict fail-closed, and ensures + syncs the metadata working tree). All reads and writes land in that tree on `metadata_branch`, pushed to its remote immediately so the backlog stays reviewable on GitHub and visible to the autonomous implementer — `.docket/` on `origin/docket` in `docket`-mode; the primary working tree on `origin/<integration_branch>` in `main`-mode.

## Procedure

### Step 1 — Select

Sync the metadata working tree, then rank every needs-brainstorm change in `active/` — `status: proposed`, no `spec:`, not `trivial: true` — by the convention's deterministic selection order (the same ranking `docket-implement-next` uses). Pick the top, or accept an explicit id from the caller; an explicit id that is not needs-brainstorm is an error to report, never a silent re-pick. Empty queue → report that nothing needs grooming and stop.

When autonomous grooming is in play (see the convention's *Autonomous grooming* shared definition), rank in **selection bands** — the human's attention goes first to stubs that need a human: (1) abstained stubs (a `## Auto-groom blocked` section is present — they are literally waiting on you), then (2) effective `auto_groomable: false` stubs, then (3) effective auto-groomable stubs, each flagged "#NNNN is auto-groomable — docket-auto-groom will handle it unless you want it now." Within each band, the deterministic order applies unchanged. Every needs-brainstorm stub stays selectable — bands reorder, they never exclude; an explicit id still overrides everything.

Unsatisfied `depends_on` does NOT exclude a stub — designing ahead of dependencies is expected (that is what specs are for, and the implementer's reconcile pass re-validates every spec against current reality at build time). Instead, state each dependency and its current status as part of the Step 3 recap, so the human designs with eyes open.

No claim is taken — see *Concurrency — no claim* below.

### Step 2 — Scan related context

Read the neighbouring `active/` changes, recently archived changes, and the ADR index BEFORE the brainstorm, so the conversation is informed by adjacent work. Read the learnings index `<changes_dir>/learnings/README.md` BEFORE the brainstorm and pull any findings whose hook bears on the stub, so the conversation is informed by adjacent work and past lessons (skipped entirely when `learnings.enabled` is `false`). Record the resulting `related:`/`depends_on:`/`adrs:` updates after the design settles.

### Step 3 — Recap, then groom with the human

Open with a **recap of the selected stub**, written for a reader with no prior context — grooming is routinely invoked from a phone or a fresh session, long after the stub was captured, and a cold-start human cannot answer design questions about a change they have not been reminded of. The recap covers:

- What was selected and why: id, title, priority — and whether it was the deterministic pick or an explicitly requested id.
- A PM-altitude summary of the stub: its `## Why` and `## What changes` distilled into a few sentences.
- Each `depends_on` entry and its current status (the statement Step 1 requires).
- The stub's `## Open questions`, framed as the agenda the brainstorm will work through.

The recap is an introduction, not a confirmation gate — flow directly into the brainstorm; the human redirects there, not at a pre-brainstorm prompt.

Then run the **resolved brainstorm skill** — `$SKILL_BRAINSTORM` from the Step-0 config export (default `superpowers:brainstorming`) — WITH THE HUMAN, seeded with the stub's body and its `## Open questions` — the open questions are the session's starting agenda. If it resolves to `auto` or cannot be invoked, apply the brainstorm auto-fallback per the convention's *Skill layer* (design inline with the human, warning prominently on unavailability) — the artifact is unchanged: a spec, then stop. If the human asks for a consultant-written spec, invoke `docket-brainstorm` for this run regardless of `$SKILL_BRAINSTORM` — human steering of an interactive session always wins (see the README's consultant-brainstorm section). STOP AT THE SPEC — do NOT continue to `superpowers:writing-plans` (planning is build-time, owned by `docket-implement-next`).

### Step 4 — Exit (one of four; the human confirms which)

All four exits reuse existing transitions — this skill introduces no new lifecycle status:

1. **Spec** (the normal exit): write the design doc natively to `.docket/docs/superpowers/specs/<UTC date>-<slug>-design.md` (on `metadata_branch`); set `spec:`; refresh the change body to the settled design (keep it at proposal altitude — design detail lives in the spec); remove resolved `## Open questions` entries; set `updated: <UTC today>`. After writing `spec:`, regenerate the change's `## Artifacts` block: `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh render-change-links --change-file .docket/<changes_dir>/active/<id>-<slug>.md --adrs-dir .docket/<adrs_dir>` (trivial verdict has no spec — block stays empty until build; the block edit rides with this spec-write commit; the renderer is the sole writer of the block). The change is now build-ready.
2. **Trivial verdict**: the brainstorm concludes there is no real design question — set `trivial: true`, tighten the body, no spec, set `updated:`. Also build-ready.
3. **Kill**: the stub is obsolete, a duplicate, or decided against — follow the proposed-kill sub-path in `docket-new-change` (it owns the kill mechanics; do not restate them here).
4. **Defer**: right idea, wrong time — set `status: deferred`, add `## Why deferred`, set `updated:`.

### Step 5 — Commit, push, board

Commit the change-file edit + spec together in the metadata working tree and push to `origin/docket`. On a non-fast-forward rejection: re-run `docket.sh preflight` and retry; if the rebase brought in commits touching the groomed change's file, RE-READ it first — if it is no longer needs-brainstorm (someone else groomed, killed, or claimed it), STOP and report rather than overwrite. Then invoke `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh docket-status --board-only` — the single Board-pass entry point; it renders, commits, and pushes `BOARD.md` itself (a separate commit, only if the board changed). **Must-land:** key on the stdout report line, never the exit code — `board inline changed push-failed` is the only retryable line; every other report line (`board inline changed pushed`, `board inline clean`, `board off`, `board github ok`, `board github failed`) is terminal. On `board inline changed push-failed`, re-run `docket.sh preflight` and invoke it again, bounded to 3 attempts total; if it still reports `board inline changed push-failed`, STOP and surface the failure — in contrast to `docket-implement-next`'s best-effort mid-build refresh (same pattern as `docket-new-change`'s step 5) — the readiness cell flips from needs-brainstorm, or the row leaves the Proposed section on a kill or defer. STOP — grooming never implements.

## Concurrency — no claim

Grooming is human-attended and minutes-long, so concurrent-groomer collisions are improbable; the step-5 push discipline (rebase-retry plus the mandatory re-read when the groomed file was touched) is the compare-and-swap that protects the write. A `grooming:` marker field and a status-based claim were considered and rejected — both add machinery (new field or new status, plus stale-state cleanup) for a race that the final-push CAS already resolves safely.
