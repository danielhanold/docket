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

This skill brainstorms with a human, so it cannot be a fire-and-forget subagent and cannot force the session model. **Recommended: `sonnet`, effort: model default** (wide variance from a trivial stub to a full brainstorm). Set `/model sonnet` to match; this is advisory only — the human owns the session.

## Convention (load first — blocking)

Before anything else in this skill, invoke the `docket-convention` skill via the Skill tool — unless it was already invoked earlier in this session and its content is in context. Everything below uses its vocabulary (build-ready, metadata working tree, terminal-publish, the `DOCKET`/`LIVE` bootstrap probes, …) without redefinition; no step below is executable without the convention loaded.

## Where everything is read and written

Resolve config + the bootstrap verdict deterministically: `eval "$(scripts/docket-config.sh --export)"` (fail-closed; read-only). Act on `BOOTSTRAP` — `PROCEED` to continue; `STOP_MIGRATE` to refuse-and-point at `migrate-to-docket.sh`; `CREATE_ORPHAN` to opt into `scripts/docket-config.sh --bootstrap` (fresh repo only).

All reads and writes happen in the **metadata working tree** on `metadata_branch`, pushed to its remote immediately so the backlog is reviewable on GitHub and visible to the autonomous implementer. In `docket`-mode that tree is the persistent `.docket/` worktree parked on `docket` — ensure it (state-specific create per the convention's Branch model, idempotent) and **sync it to `origin/docket` before any read** (`git -C .docket fetch origin docket && git -C .docket pull --rebase origin docket`); the change, spec, and refreshed `BOARD.md` are committed in `.docket/` and pushed to `origin/docket`. In single-branch/`main`-mode this degrades to the primary working tree on the integration branch (no `.docket/`): `git pull --rebase` and push on `origin/<metadata_branch>` (which equals `origin/<integration_branch>` there). The steps below say "`.docket/`" / "`origin/docket`" for the common (`docket`-mode) case; read those as the metadata working tree / `origin/<metadata_branch>` in `main`-mode.

## Brainstorm mode (default)

The default path for any non-trivial new change. Five steps:

1. **Allocate** — sync the metadata working tree (`git -C .docket pull --rebase origin docket`); scan the `id:` frontmatter of EVERY change in `active/` + `archive/` (archive filenames are date-prefixed, so frontmatter is the reliable id source); next id = max + 1; derive slug from title. The id is finalized at the step-5 push (compare-and-swap): if that push to `origin/docket` is rejected because another `docket-new-change` minted the same id first, re-pull → re-read max id → re-allocate, RENAME `active/<id>-<slug>.md` and fix any id-bearing links, then re-push.

2. **Brainstorm** — run `superpowers:brainstorming` WITH THE HUMAN. This is the decision point. STOP AT THE SPEC — do NOT continue to `writing-plans` (that is build-time). The spec is written natively to `.docket/docs/superpowers/specs/…` (on `docket`) and committed to `metadata_branch`; record its path in `spec:`. After writing `spec:`, regenerate the change's `## Artifacts` block: `scripts/render-change-links.sh --change-file .docket/<changes_dir>/active/<id>-<slug>.md --adrs-dir .docket/<adrs_dir>` (the template already seeds the empty marker block; the block edit rides with this same spec-write commit; the renderer is the sole writer of the block).

3. **Scan related context** — scan neighbouring changes (`active/` + recent `archive/`) and the ADR index to pre-fill `related`, `depends_on`, `adrs`. In practice, do this quick read just *before* step 2 so the brainstorm is informed by neighbouring work; record the resulting `related`/`depends_on`/`adrs` after the design settles.

4. **Draft the change** — write the thin `active/<id>-<slug>.md` from `change-template.md`: frontmatter (`status: proposed`, `spec:`, `created`/`updated` = UTC today (the UTC date of the commit), priority default `medium`) + a PM-altitude why/what/scope body distilled from the brainstorm. Design detail lives in the linked spec, NOT here. When the human provided rich initial context and says the change may be designed without them, set `auto_groomable: true` at draft time — `docket-auto-groom` will carry it to build-ready. Otherwise leave the field unset (it inherits the repo's `auto_groom` default).

5. **Board, commit & push** — refresh `BOARD.md` (via `docket-status`'s Board pass), commit the change + spec, and PUSH to `origin/docket` (immediately reviewable on GitHub; visible to the autonomous implementer). STOP. Never implements.

## Trivial path

For a small mechanical change with no real design questions: skip the brainstorm, set `trivial: true`, write the change body directly — no spec, still build-ready. It still follows Brainstorm mode's steps 1 (Allocate), 3 (Scan related context), 4 (Draft — but omit `spec:`), and 5 (Board, commit & push) — only step 2 (Brainstorm) is skipped.

## Scan mode (opt-in)

Survey TODOs, deferred changes, known gaps, and the ADR backlog; emit several lightweight `proposed` STUBS in one pass — WITHOUT specs. Scan-stubs are NOT build-ready (no spec, not trivial) — the board calls this state `needs-brainstorm`. They form an "ideas to brainstorm" backlog that `docket-groom-next` — the later brainstorm pass — turns build-ready. Scan-stubs leave `auto_groomable` unset — they inherit the repo default; in an `auto_groom: true` repo that makes the whole scan harvest autonomously groomable, which is the point. Kept opt-in so routine runs don't generate speculative noise. Once all stubs are written, commit them together with a refreshed `BOARD.md` and push to `origin/docket` (same push pattern as Brainstorm mode's step 5, but no spec).

## Proposed-kill sub-path

When a `proposed` change is abandoned (obsolete, decided against, a duplicate) the producer drives it to the `killed` terminal state — this is one of the two kill origins the shared terminal-publish serves (the other is the implementer's reconcile-kill from `in-progress`).

In `docket`-mode: the kill is driven by the same two-script sequence finalize uses (it is the single source — *Terminal publish (docket-mode)* in `docket-finalize-change`):

- **Archive:** `scripts/archive-change.sh --changes-dir .docket/<changes_dir> --id <id> --outcome killed --date <UTC kill date> --reason "<why killed text>" --message "<msg>"` — performs the `active/ → archive/<UTC kill date>-<id>-<slug>.md` move, inserts the `## Why killed` section, sets `status: killed` / `updated: <UTC kill date>`, commits the **change file only**, and pushes `origin/docket`. Trust the exit code.
- **Publish:** after archiving, `scripts/terminal-publish.sh --id <id> --outcome killed --integration-branch <integration_branch> --metadata-branch docket --changes-dir <changes_dir> --adrs-dir <adrs_dir> --message "<msg>"` — copies the terminal records (archived change file, spec if set, `Accepted` ADRs) from `origin/docket` onto the integration branch. Trust the exit code.

In `main`-mode (no `docket` branch / no terminal-publish): `scripts/archive-change.sh --outcome killed …` runs against the primary working tree (the integration branch), performing the archive move + `## Why killed` insertion + push directly there. `scripts/terminal-publish.sh` is a no-op in `main`-mode (its own mode-guard fires on `metadata_branch == integration_branch`). The `<UTC kill date>` is the same date used for the `archive/<date>-…` filename prefix.

A `proposed` change never had a feature branch or open PR, so there is nothing to clean up — and usually no plan/results, so the kill publishes only what is on `docket`: the change file, plus its `spec:`/`adrs:` if set. This skill still writes markdown only — the terminal-publish copy touches no code.

In both modes, after the kill is archived, refresh `BOARD.md` via the **must-land Board pass** (a separate commit, same as the create path's step 5) so the killed change leaves the board. terminal-publish copies records to the integration branch but never touches `BOARD.md`, so the board refresh is this skill's responsibility.
