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

## Convention (load first — blocking)

Before anything else in this skill, invoke the `docket-convention` skill via the Skill tool — unless it was already invoked earlier in this session and its content is in context. Everything below uses its vocabulary (needs-brainstorm, build-ready, metadata working tree, the bootstrap probes, …) without redefinition; no step below is executable without the convention loaded.

## Where everything is read and written

All reads and writes happen in the **metadata working tree** on `metadata_branch`, pushed to its remote immediately so the backlog stays reviewable on GitHub and visible to the autonomous implementer. In `docket`-mode that tree is the persistent `.docket/` worktree parked on `docket` — ensure it (state-specific create per the convention's Branch model, idempotent) and **sync it to `origin/docket` before any read** (`git -C .docket fetch origin docket && git -C .docket pull --rebase origin docket`). In single-branch/`main`-mode this degrades to the primary working tree on the integration branch (no `.docket/`): `git pull --rebase` and push on `origin/<metadata_branch>` (which equals `origin/<integration_branch>` there). The steps below say "`.docket/`" / "`origin/docket`" for the common (`docket`-mode) case; read those as the metadata working tree / `origin/<metadata_branch>` in `main`-mode.

## Procedure

### Step 1 — Select

Sync the metadata working tree, then rank every needs-brainstorm change in `active/` — `status: proposed`, no `spec:`, not `trivial: true` — by the convention's deterministic selection order (the same ranking `docket-implement-next` uses). Pick the top, or accept an explicit id from the caller; an explicit id that is not needs-brainstorm is an error to report, never a silent re-pick. Empty queue → report that nothing needs grooming and stop.

When autonomous grooming is in play (see the convention's *Autonomous grooming* shared definition), rank in **selection bands** — the human's attention goes first to stubs that need a human: (1) abstained stubs (a `## Auto-groom blocked` section is present — they are literally waiting on you), then (2) effective `auto_groomable: false` stubs, then (3) effective auto-groomable stubs, each flagged "#NNNN is auto-groomable — docket-auto-groom will handle it unless you want it now." Within each band, the deterministic order applies unchanged. Every needs-brainstorm stub stays selectable — bands reorder, they never exclude; an explicit id still overrides everything.

Unsatisfied `depends_on` does NOT exclude a stub — designing ahead of dependencies is expected (that is what specs are for, and the implementer's reconcile pass re-validates every spec against current reality at build time). Instead, state each dependency and its current status as part of the Step 3 recap, so the human designs with eyes open.

No claim is taken — see *Concurrency — no claim* below.

### Step 2 — Scan related context

Read the neighbouring `active/` changes, recently archived changes, the ADR index, and `<changes_dir>/LEARNINGS.md` BEFORE the brainstorm, so the conversation is informed by adjacent work and past lessons. Record the resulting `related:`/`depends_on:`/`adrs:` updates after the design settles.

### Step 3 — Recap, then groom with the human

Open with a **recap of the selected stub**, written for a reader with no prior context — grooming is routinely invoked from a phone or a fresh session, long after the stub was captured, and a cold-start human cannot answer design questions about a change they have not been reminded of. The recap covers:

- What was selected and why: id, title, priority — and whether it was the deterministic pick or an explicitly requested id.
- A PM-altitude summary of the stub: its `## Why` and `## What changes` distilled into a few sentences.
- Each `depends_on` entry and its current status (the statement Step 1 requires).
- The stub's `## Open questions`, framed as the agenda the brainstorm will work through.

The recap is an introduction, not a confirmation gate — flow directly into the brainstorm; the human redirects there, not at a pre-brainstorm prompt.

Then run `superpowers:brainstorming` WITH THE HUMAN, seeded with the stub's body and its `## Open questions` — the open questions are the session's starting agenda. STOP AT THE SPEC — do NOT continue to `superpowers:writing-plans` (planning is build-time, owned by `docket-implement-next`).

### Step 4 — Exit (one of four; the human confirms which)

All four exits reuse existing transitions — this skill introduces no new lifecycle status:

1. **Spec** (the normal exit): write the design doc natively to `.docket/docs/superpowers/specs/<UTC date>-<slug>-design.md` (on `metadata_branch`); set `spec:`; refresh the change body to the settled design (keep it at proposal altitude — design detail lives in the spec); remove resolved `## Open questions` entries; set `updated: <UTC today>`. The change is now build-ready.
2. **Trivial verdict**: the brainstorm concludes there is no real design question — set `trivial: true`, tighten the body, no spec, set `updated:`. Also build-ready.
3. **Kill**: the stub is obsolete, a duplicate, or decided against — follow the proposed-kill sub-path in `docket-new-change` (it owns the kill mechanics; do not restate them here).
4. **Defer**: right idea, wrong time — set `status: deferred`, add `## Why deferred`, set `updated:`.

### Step 5 — Commit, push, board

Commit the change-file edit + spec together in the metadata working tree and push to `origin/docket`. On a non-fast-forward rejection: `pull --rebase` and retry; if the rebase brought in commits touching the groomed change's file, RE-READ it first — if it is no longer needs-brainstorm (someone else groomed, killed, or claimed it), STOP and report rather than overwrite. Then refresh `BOARD.md` via `docket-status`'s Board pass as a separate, must-land commit — retry until it lands, in contrast to `docket-implement-next`'s best-effort mid-build refresh (same pattern as `docket-new-change`'s step 5) — the readiness cell flips from needs-brainstorm, or the row leaves the Proposed section on a kill or defer. STOP — grooming never implements.

## Concurrency — no claim

Grooming is human-attended and minutes-long, so concurrent-groomer collisions are improbable; the step-5 push discipline (rebase-retry plus the mandatory re-read when the groomed file was touched) is the compare-and-swap that protects the write. A `grooming:` marker field and a status-based claim were considered and rejected — both add machinery (new field or new status, plus stale-state cleanup) for a race that the final-push CAS already resolves safely.
