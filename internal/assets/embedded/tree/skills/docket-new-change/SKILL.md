---
name: docket-new-change
description: Use when capturing a new unit of planned work (a change, roughly one PR) into the docket backlog — turning an idea into a tracked, build-ready change through up-front design brainstorming, or (opt-in) scanning a project for candidate work into proposed stubs. Interactive; the entry point a human runs to propose work before it is implemented. Writes markdown only — never branches, worktrees, or code.
---

# docket-new-change — the producer (interactive)

## Overview

`docket-new-change` is where the human takes part. It turns an idea into a build-ready change by brainstorming the design up front with the human before any implementation begins. It only ever mints new `proposed` ids — scanning the max existing id and incrementing — so it structurally cannot collide with the autonomous implementer. It writes markdown only: a change file, an optional spec, and a refreshed `BOARD.md`. It never branches, creates worktrees, or touches code.

## When to use

- You have a new idea, feature request, or known gap you want to track and eventually build.
- You want to brainstorm and spec out a change before handing it to `docket-implement-next`.
- You want to quickly stub several `proposed` candidates without brainstorming yet (scan mode — opt-in).
- A trivial mechanical change needs to be tracked but has no real design questions (trivial path).

## Recommended model/effort (advisory)

This skill brainstorms with a human, so it cannot be a fire-and-forget subagent and cannot force the session model. **Recommended: `claude-sonnet-5`, effort: model default** (wide variance from a trivial stub to a full brainstorm). Set `/model claude-sonnet-5` to match; this is advisory only — the human owns the session.

## Convention (load first — blocking)

Invoke the `docket-convention` skill via the Skill tool first — unless already invoked this session — and run its *startup check* (load the convention; run the capability bootstrap; run the `repository.prepare` operation with `--repo-dir <dir> --json` as its own Bash call; validate the protocol-v1 envelope and carry its typed context values forward as literals; act on the disposition). Everything below uses its vocabulary without redefinition. All writes — the change, spec, and refreshed `BOARD.md` — land in the metadata worktree (`metadata_worktree_path` in the `repository.prepare` context) on the metadata branch, pushed to the metadata remote (`metadata_remote`) immediately so the backlog is reviewable and visible to the autonomous implementer.

## Brainstorm mode (default)

The default path for any non-trivial new change. Five steps:

1. **Allocate** — id allocation is owned by the `change.create` operation transaction (step 4): it reads the next id from fresh origin state, derives the slug from the title, serializes the canonical proposed record, and lands it under an exact-lease push — so there is **no** hand-scan of `active/` + `archive/`, no `max + 1` by hand, and no manual rename-on-collision. Supply a stable `request_id` on the create request so a lost-response retry converges by idempotent replay rather than minting a duplicate. The startup-check `repository.prepare` operation already synced the metadata working tree.

2. **Brainstorm** — invoke `superpowers:brainstorming` WITH THE HUMAN. This is the decision point. When `superpowers:brainstorming` cannot be invoked, run the brainstorm inline with the human, warning prominently; the artifact is unchanged: a spec, then stop. If the human asks for a consultant-written spec, invoke `docket-brainstorm` for this run instead — human steering of an interactive session always wins (see the README's consultant-brainstorm section). STOP AT THE SPEC — do NOT continue to `writing-plans` (that is build-time). The settled design becomes the **spec's Markdown body**, authored and held for step 5 — do NOT hand-write the spec file, hand-set `spec:`, or hand-render the `## Artifacts`/`docket:backlink` blocks: the step-5 `change.groom` operation writes the spec file under `docs/superpowers/specs/…` on the metadata branch, sets `spec:`, stamps the reciprocal `docket:backlink` block, and re-renders the `## Artifacts` block atomically. A typed refusal (malformed markers, spec-path-taken) leaves the files untouched — surface it, do not hand-edit.

3. **Scan related context** — scan neighbouring changes (`active/` + recent `archive/`) and the ADR index to pre-fill `related`, `depends_on`, `adrs`, and — when the human names the change(s) whose work surfaced this one (or scan mode infers an origin) — `discovered_from` (informational, like `related:`; empty for deliberately planned work). In practice, do this quick read just *before* step 2 so the brainstorm is informed by neighbouring work; record the resulting `related`/`depends_on`/`adrs`/`discovered_from` after the design settles.

4. **Create the proposed change** — submit the record through one atomic transaction: the `change.create` operation (resolve argv from the capability catalog) with `--repo-dir <metadata_worktree_path> --request <request-file>`. Its closed JSON request carries `title`, `type` (one configured `change_type` — `create` refuses an unknown or empty type, so no created change is ever left `untyped`, and there is no template comment to replace), `priority` (default `medium`), the PM-altitude `why`/`what_changes`/`out_of_scope` body distilled from the brainstorm (design detail lives in the linked spec, NOT here), the resolved `depends_on`/`related`/`adrs`/`discovered_from` from step 3, `stacked_on` when the work builds on another change's **unmerged** branch (set it and **read [stacked-changes.md](../docket-convention/references/stacked-changes.md) now (blocking)** first — stacking changes how it is built, merged, and closed out), and the stable `request_id` from step 1. The transaction allocates the id, derives the slug, serializes the canonical `status: proposed` record (`created`/`updated` = the commit's UTC date), renders its `## Artifacts` block, and re-renders the inline board — one metadata commit under an exact-lease push, returning the new `id`/`slug`/`path`.

   **Two draft-time scalars ride in the same request** — `auto_groomable` and `branch_prefix`. When the human says the change may be designed without them, send `auto_groomable: true` (so `docket-auto-groom` carries it to build-ready; `false` opts it out). When they name a branch prefix ("use the `hotfix/` prefix"), send it as typed in `branch_prefix` — the operation normalizes it and refuses an unusable value with `invalid-branch_prefix` before anything is written: show the human that finding and ask for a new value. Omit `auto_groomable` to leave the change not auto-groomable; omit `branch_prefix` to use `<type>`.

5. **Groom to build-ready & land** — read the created record's `path` + exact **record revision** (blob object id) from the `status` operation (with `--json`), then apply the spec atomically: the `change.groom` operation with `--repo-dir <metadata_worktree_path> --request <request-file>` carrying the change id, the pinned `path` + `revision`, `outcome: spec`, the `spec_markdown` from step 2, and any owned proposal-section rewrites. In one metadata commit the transaction writes the spec file, sets `spec:` + `updated:`, stamps the spec's `docket:backlink` block, re-renders the `## Artifacts` block, and re-renders the inline board (pushed under an exact-lease push) — **no separate Board pass**. A revision-mismatch / `contended` refusal writes nothing: re-sync (re-run the `repository.prepare` operation), re-read `path` + `revision`, retry. STOP. Never implements. To adjust a just-landed spec or its owned sections afterwards, run `docket-groom-next <id>` — its explicit-id path routes an already-groomed change to `change.groom` with `outcome: revise`; never re-run this skill or hand-edit the spec file.

## Trivial path

For a small mechanical change with no real design questions: skip the brainstorm — no spec, still build-ready. It follows Brainstorm mode's steps 1 (Allocate), 3 (Scan related context), and 4 (Create the proposed change) unchanged, then step 5 grooms with `outcome: trivial` instead of `spec`: the `change.groom` operation with `--repo-dir <metadata_worktree_path> --request <request-file>` with the pinned `path` + `revision` and an owned-section edit carrying the tightened body as the trivial rationale — the transaction sets `trivial: true` + `updated:`, re-renders the `## Artifacts` block, and re-renders the inline board atomically (no spec file, no separate Board pass). Only step 2 (Brainstorm) is skipped.

## Scan mode (opt-in)

Survey TODOs, deferred changes, known gaps, and the ADR backlog; emit several lightweight `proposed` STUBS in one pass — WITHOUT specs. Scan-stubs are NOT build-ready (no spec, not trivial) — the board calls this state `needs-grooming`; they form the "ideas to brainstorm" backlog `docket-groom-next` turns build-ready. Scan-stubs still carry a `type:` — every creation path writes one, so the untyped set only ever shrinks. Scan-stubs leave `auto_groomable` unset, so none is auto-groomable until a human sets `auto_groomable: true` on it. Kept opt-in so routine runs don't generate speculative noise. Emit each stub through its own `change.create` operation transaction (resolve argv from the capability catalog) with `--repo-dir <metadata_worktree_path> --request <request-file>` (title/type/priority + why/what/scope + a stable `request_id`; no spec). Each transaction allocates the id, renders the stub's `## Artifacts` block, and re-renders the inline board atomically, pushing to the metadata remote under an exact-lease push — so there is **no separate Board pass**.

## Proposed-kill sub-path

When a `proposed` change is abandoned (obsolete, decided against, a duplicate) the producer drives it to the `killed` final status — this is one of the two kill origins the shared close-out serves (the other is the implementer's reconcile-kill from `in-progress`).

Follow `../docket-convention/references/close-out.md`'s **kill path** — one atomic `change.kill` operation transaction that archives the record on `docket`, re-renders its `## Artifacts` block, retargets the linked spec's back-link, and re-renders the inline board in one metadata commit (mechanics and ordering live there — do not restate them here). Trust the typed outcome; a refusal aborts the kill and is surfaced, writing nothing. A `proposed` change never had a feature branch or open PR — the reference's cleanup step is a no-op here — and usually no plan/results. The kill copies nothing onto the integration branch: the archived change file, plus its `spec:`/`adrs:` if set, stays on `docket`. The archive date matches `origin/<integration_branch>`.

Because the `change.kill` operation renders the inline board **atomically inside its own metadata commit**, the killed change leaves the board as part of that same commit — there is **no separate Board pass**, and the skill never hand-renders or double-commits the board. `BOARD.md` is the live planning view and is never published to the integration branch.
