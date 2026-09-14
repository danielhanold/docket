---
name: docket-groom-next
description: Use when stubs are sitting at needs-brainstorm on the docket board and you want the next one designed — selecting the next needs-brainstorm change (proposed, no spec, not trivial) deterministically and grooming it to build-ready through an interactive brainstorm with the human, exiting with a linked spec, a trivial verdict, a kill, or a defer. Selection is autonomous; the design conversation is not. Writes markdown only — never branches, worktrees, or code.
---

# docket-groom-next — the groomer (interactive)

## Overview

`docket-groom-next` drains the needs-brainstorm queue: `docket-new-change`'s scan mode captures ideas as lightweight stubs; this skill is the later brainstorm pass that turns them build-ready through an interactive design conversation with the human. One stub per invocation; loop by re-invoking. It writes markdown only: the change file, a spec, and a refreshed `BOARD.md` — never branches, worktrees, or code.

## When to use

- Stubs show as needs-brainstorm on the board and you want to design the next one, or a specific one (pass its id explicitly to skip selection).
- Do NOT use to capture a brand-new idea — that is `docket-new-change`'s job; this skill never mints ids.
- Do NOT use to re-groom a change that already has a spec — drift against current reality is the reconcile pass's job in `docket-implement-next`. A human who wants to redo a design can clear `spec:` by hand first.

## Recommended model/effort (advisory)

This skill grooms interactively with a human, so it cannot be a fire-and-forget subagent and cannot force the session model. **Recommended: `claude-sonnet-5` / `high`** (the cold-start recap is genuine synthesis). Set `/model claude-sonnet-5` and `/effort high` to match; this is advisory only — the human owns the session.

## Convention (load first — blocking)

Invoke the `docket-convention` skill via the Skill tool first — unless already invoked this session — and run its *Step-0 preamble* (load the convention; run the capability bootstrap; run the `repository.prepare` operation with `--repo-dir <dir> --json` as its own Bash call; validate the protocol-v1 envelope and carry its typed context values forward as literals; act on the disposition). Everything below uses its vocabulary without redefinition. All reads and writes land in the metadata working tree on `metadata_branch`, pushed to its remote immediately so the backlog stays reviewable on GitHub and visible to the autonomous implementer; grooming never touches `origin/<integration_branch>` — markdown only, no branches or code.

## Procedure

### Step 1 — Select

Sync the metadata working tree (the Step-0 `repository.prepare` operation), then rank every needs-brainstorm change in `active/` — `status: proposed`, no `spec:`, not `trivial: true` — by the convention's deterministic selection order (the same ranking `docket-implement-next` uses). Pick the top, or accept an explicit id from the caller; an explicit id that is not needs-brainstorm is an error to report, never a silent re-pick. Empty queue → report that nothing needs grooming and stop. Read the selected stub's exact **record path** and opaque **entity version** (the blob object id) from the `status` operation (with `--json`) — the Step-4 groom/defer transactions pin the record with those, and re-reading them after a mid-run re-sync is a fresh `status` read.

When autonomous grooming is in play (see the convention's *Autonomous grooming* shared definition), rank in **selection bands** — the human's attention goes first to stubs that need a human: (1) abstained stubs (a `## Auto-groom blocked` section is present — they are literally waiting on you), then (2) effective `auto_groomable: false` stubs, then (3) effective auto-groomable stubs, each flagged "#NNNN is auto-groomable — docket-auto-groom will handle it unless you want it now." Within each band, the deterministic order applies unchanged. Every needs-brainstorm stub stays selectable — bands reorder, they never exclude; an explicit id still overrides everything.

Unsatisfied `depends_on` does NOT exclude a stub — designing ahead of dependencies is expected (that is what specs are for, and the implementer's reconcile pass re-validates every spec against current reality at build time). Instead, state each dependency and its current status as part of the Step 3 recap, so the human designs with eyes open.

No claim is taken — see *Concurrency — no claim* below.

### Step 2 — Scan related context

BEFORE the brainstorm, read the neighbouring `active/` changes, recently archived changes, and the ADR index, plus the learnings index `<changes_dir>/learnings/README.md` and any findings whose hook bears on the stub (skipped entirely when `learnings.enabled` is `false`) — so the conversation is informed by adjacent work and past lessons. Record the resulting `related:`/`depends_on:`/`adrs:` updates after the design settles.

### Step 3 — Recap, then groom with the human

Open with a **recap of the selected stub**, written for a reader with no prior context — grooming is routinely invoked from a phone or a fresh session, long after the stub was captured, and a cold-start human cannot answer design questions about a change they have not been reminded of. The recap covers:

- What was selected and why: id, title, priority — and whether it was the deterministic pick or an explicitly requested id.
- A PM-altitude summary of the stub: its `## Why` and `## What changes` distilled into a few sentences.
- Each `depends_on` entry and its current status (the statement Step 1 requires).
- The stub's `## Open questions`, framed as the agenda the brainstorm will work through.

**Dummy mode:** when `DUMMY_MODE_ENABLED` is `true` (Step-0 export) — or the human asks for it in-session — write this step's `dialogue` (recap, questions, and design presentation) calibrated to `DUMMY_MODE_PERSONA`, per the convention's *Dummy mode* shared definition. The spec file itself is never simplified.

When the design settles on building this stub atop another change's **unmerged** branch, set `stacked_on: <parent id>` at the spec exit and **read [`../docket-convention/references/stacked-changes.md`](../docket-convention/references/stacked-changes.md) now (blocking)** first — stacking changes how the change is built, merged, and closed out.

The recap is an introduction, not a confirmation gate — flow directly into the brainstorm; the human redirects there, not at a pre-brainstorm prompt.

Then run the **resolved brainstorm skill** — `$SKILL_BRAINSTORM` from the Step-0 config export (default `superpowers:brainstorming`) — WITH THE HUMAN, seeded with the stub's body and its `## Open questions` — the open questions are the session's starting agenda. If it resolves to `auto` or cannot be invoked, apply the brainstorm auto-fallback per the convention's *Skill layer* (design inline with the human, warning prominently on unavailability) — the artifact is unchanged: a spec, then stop. If the human asks for a consultant-written spec, invoke `docket-brainstorm` for this run regardless of `$SKILL_BRAINSTORM` — human steering of an interactive session always wins (see the README's consultant-brainstorm section). STOP AT THE SPEC — do NOT continue to `superpowers:writing-plans` (planning is build-time, owned by `docket-implement-next`).

### Step 4 — Exit (one of four; the human confirms which)

All four exits reuse existing transitions — this skill introduces no new lifecycle status:

1. **Spec** (the normal exit): **author** the settled design as the spec's Markdown body, and any owned proposal-section rewrites (proposal altitude, resolved `## Open questions` removed). Apply it with one atomic transaction — the `change.groom` operation with `--repo-dir .docket --request <request-file>` — carrying the change id, the pinned record `path` + `version` from Step 1, `outcome: spec`, the `spec_markdown`, the owned-section edits, and the desired `depends_on`/`related`/`adrs`/`discovered_from`/`stacked_on` (authored Markdown travels in the request file, never a shell-escaped flag). In one metadata commit it writes the spec file, sets `spec:` + `updated:`, stamps the spec's `docket:backlink` block, and re-renders the `## Artifacts` block and inline board — so there is no separate render, back-link, or Board pass. A typed refusal (not-groomable, spec-path-taken, malformed markers, version mismatch) writes nothing — surface it. The change is now build-ready.
2. **Trivial verdict**: the brainstorm concludes there is no real design question — apply the `change.groom` operation with `--repo-dir .docket --request <request-file>` with `outcome: trivial`, the pinned `path` + `version`, and an owned-section edit carrying the tightened body as the trivial rationale. The transaction sets `trivial: true` + `updated:`, re-renders the `## Artifacts` block, and re-renders the inline board atomically — no spec file, no separate Board pass. Also build-ready.
3. **Kill**: the stub is obsolete, a duplicate, or decided against — follow the proposed-kill sub-path in `docket-new-change` (it owns the kill mechanics; do not restate them here).
4. **Defer**: right idea, wrong time — apply the `change.defer` operation with `--repo-dir .docket --request <request-file>` with the pinned `path` + `version` and the authored `why_deferred` section body. The transaction sets `status: deferred`, splices the `## Why deferred` section, sets `updated:`, and re-renders the inline board atomically.

### Step 5 — The transaction lands (no separate board pass)

The Step-4 typed op is the whole write: it re-checks the pinned exact `version`, commits the change record, the spec, the `## Artifacts` block, and the inline board in one metadata commit pushed to `origin/docket` under an exact-lease push — so there is no separate hand-staged commit and **no separate Board pass** (the readiness cell flips from needs-brainstorm, or the row leaves the Proposed section on a kill or defer, in that same commit). On a `contended` refusal the op writes nothing: re-sync (re-run the `repository.prepare` operation), re-read the record's `path` + `version` from the `status` operation, and — if it is no longer needs-brainstorm (someone else groomed, killed, or claimed it) — STOP and report rather than overwrite; otherwise re-author and retry. STOP — grooming never implements.

## Concurrency — no claim

Grooming is human-attended and minutes-long, so concurrent-groomer collisions are improbable; the Step-4 transaction's exact-version check plus exact-lease push (and the mandatory re-read when a `contended` refusal shows the record moved) is the compare-and-swap that protects the write. A `grooming:` marker field and a status-based claim were considered and rejected — both add machinery (new field or new status, plus stale-state cleanup) for a race that the transaction CAS already resolves safely.
