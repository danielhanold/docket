---
name: docket-new-change
description: Use when capturing a new unit of planned work (a change, roughly one PR) into the docket backlog — turning an idea into a tracked, build-ready change through up-front design brainstorming, or (opt-in) scanning a project for candidate work into proposed stubs. Interactive; the entry point a human runs to propose work before it is implemented. Writes markdown only — never branches, worktrees, or code.
---

# docket-new-change — the producer (interactive)

## Overview

`docket-new-change` is where the human is in the loop. It turns an idea into a build-ready change by brainstorming the design up front with the human before any implementation begins. It only ever mints new `proposed` ids — scanning the max existing id and incrementing — so it structurally cannot collide with the autonomous implementer. It writes markdown only: a change file, an optional spec, and a refreshed `BOARD.md`. It never branches, creates worktrees, or touches code.

## When to use

- You have a new idea, feature request, or known gap you want to track and eventually build.
- You want to brainstorm and spec out a change before handing it to `docket-implement-next`.
- You want to quickly stub several `proposed` candidates without brainstorming yet (scan mode — opt-in).
- A trivial mechanical change needs to be tracked but has no real design questions (trivial path).

## Recommended model/effort (advisory)

This skill brainstorms with a human, so it cannot be a fire-and-forget subagent and cannot force the session model. **Recommended: `claude-sonnet-5`, effort: model default** (wide variance from a trivial stub to a full brainstorm). Set `/model claude-sonnet-5` to match; this is advisory only — the human owns the session.

## Convention (load first — blocking)

Before anything else in this skill, invoke the `docket-convention` skill via the Skill tool — unless it was already invoked earlier in this session and its content is in context. Everything below uses its vocabulary (build-ready, metadata working tree, terminal-publish, the `DOCKET`/`LIVE` bootstrap probes, …) without redefinition; no step below is executable without the convention loaded.

## Step 0

Run the convention's *Step-0 preamble*: load the convention, resolve config + the bootstrap verdict (`eval "$("${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh --export)"`, fail-closed; act on `BOOTSTRAP`), then ensure + sync the metadata working tree. All reads and writes land in that tree on `metadata_branch`, pushed to its remote immediately so the backlog is reviewable on GitHub and visible to the autonomous implementer — `.docket/` on `origin/docket` in `docket`-mode; the primary working tree on `origin/<integration_branch>` in `main`-mode. The change, spec, and refreshed `BOARD.md` are committed there.

## Brainstorm mode (default)

The default path for any non-trivial new change. Five steps:

1. **Allocate** — sync the metadata working tree (`git -C .docket pull --rebase origin docket`); scan the `id:` frontmatter of EVERY change in `active/` + `archive/` (archive filenames are date-prefixed, so frontmatter is the reliable id source); next id = max + 1; derive slug from title. The id is finalized at the step-5 push (compare-and-swap): if that push to `origin/docket` is rejected because another `docket-new-change` minted the same id first, re-pull → re-read max id → re-allocate, RENAME `active/<id>-<slug>.md` and fix any id-bearing links, then re-push.

2. **Brainstorm** — run the **resolved brainstorm skill** — `$SKILL_BRAINSTORM` from the Step-0 config export (default `superpowers:brainstorming`) — WITH THE HUMAN. This is the decision point. If it resolves to `auto` or cannot be invoked, apply the brainstorm auto-fallback per the convention's *Skill layer* (design inline with the human, warning on unavailability); the artifact is unchanged: a spec, then stop. If the human asks for a consultant-written spec, invoke `docket-brainstorm` for this run regardless of `$SKILL_BRAINSTORM` — human steering of an interactive session always wins (see the README's consultant-brainstorm section). STOP AT THE SPEC — do NOT continue to `writing-plans` (that is build-time). The spec is written natively to `.docket/docs/superpowers/specs/…` (on `docket`) and committed to `metadata_branch`; record its path in `spec:`. After writing `spec:`, regenerate the change's `## Artifacts` block: `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/render-change-links.sh --change-file .docket/<changes_dir>/active/<id>-<slug>.md --adrs-dir .docket/<adrs_dir>` (the template already seeds the empty marker block; the block edit rides with this same spec-write commit; the renderer is the sole writer of the block).

3. **Scan related context** — scan neighbouring changes (`active/` + recent `archive/`) and the ADR index to pre-fill `related`, `depends_on`, `adrs`. In practice, do this quick read just *before* step 2 so the brainstorm is informed by neighbouring work; record the resulting `related`/`depends_on`/`adrs` after the design settles.

4. **Draft the change** — write the thin `active/<id>-<slug>.md` from `change-template.md`: frontmatter (`status: proposed`, `spec:`, `created`/`updated` = UTC today (the UTC date of the commit), priority default `medium`) + a PM-altitude why/what/scope body distilled from the brainstorm. Design detail lives in the linked spec, NOT here. When the human provided rich initial context and says the change may be designed without them, set `auto_groomable: true` at draft time — `docket-auto-groom` will carry it to build-ready. Otherwise leave the field unset (it inherits the repo's `auto_groom` default).

5. **Board, commit & push** — refresh `BOARD.md` (via `docket-status`'s Board pass), commit the change + spec, and PUSH to `origin/docket` (immediately reviewable on GitHub; visible to the autonomous implementer). STOP. Never implements.

## Trivial path

For a small mechanical change with no real design questions: skip the brainstorm, set `trivial: true`, write the change body directly — no spec, still build-ready. It still follows Brainstorm mode's steps 1 (Allocate), 3 (Scan related context), 4 (Draft — but omit `spec:`), and 5 (Board, commit & push) — only step 2 (Brainstorm) is skipped.

## Scan mode (opt-in)

Survey TODOs, deferred changes, known gaps, and the ADR backlog; emit several lightweight `proposed` STUBS in one pass — WITHOUT specs. Scan-stubs are NOT build-ready (no spec, not trivial) — the board calls this state `needs-brainstorm`. They form an "ideas to brainstorm" backlog that `docket-groom-next` — the later brainstorm pass — turns build-ready. Scan-stubs leave `auto_groomable` unset — they inherit the repo default; in an `auto_groom: true` repo that makes the whole scan harvest autonomously groomable, which is the point. Kept opt-in so routine runs don't generate speculative noise. Once all stubs are written, commit them together with a refreshed `BOARD.md` and push to `origin/docket` (same push pattern as Brainstorm mode's step 5, but no spec).

## Proposed-kill sub-path

When a `proposed` change is abandoned (obsolete, decided against, a duplicate) the producer drives it to the `killed` terminal state — this is one of the two kill origins the shared terminal-publish serves (the other is the implementer's reconcile-kill from `in-progress`).

Follow `references/terminal-close-out.md`'s sequence with `--outcome killed`: archive → re-render `## Artifacts` → `terminal-publish` (mechanics, ordering, and `main`-mode degradation live there — do not restate them here). Trust each exit code; a failure aborts the kill and is surfaced. A `proposed` change never had a feature branch or open PR, so there is nothing to clean up — the reference's cleanup step is a no-op here — and usually no plan/results, so the kill publishes only what is on `docket`: the change file, plus its `spec:`/`adrs:` if set. `<UTC kill date>` matches `origin/<integration_branch>`.

Unlike the reference's best-effort default, after the kill is archived this skill runs a **must-land Board pass**: refresh `BOARD.md` as a separate commit, retrying until it lands, so the killed change leaves the board — terminal-publish never touches `BOARD.md`.
