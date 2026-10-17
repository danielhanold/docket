<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0556 — A typed /docket-implement-next in Claude Code starts an untracked run](../changes/archive/2026-10-17-0556-a-typed-docket-implement-next-in-claude-code-starts-an-untra.md)**
<!-- docket:backlink:end -->

# A typed /docket-implement-next in Claude Code starts an untracked run — Results

**Human action:** None required to merge. One optional read-through of the five edited pages is suggested if you want to check the wording yourself.

## Outcome

Docket's run tracker only tracks an implement run when the session takes a turn first and runs `docket run start implement-next`. In Claude Code, typing `/docket-implement-next` skips that turn, because the skill runs forked and goes straight to the implementer. That run still builds to a PR, but if it stops partway there is no automatic retry, resume, or cancel, and its outcome can only be read with `docket run verdict --unattributed`.

The maintained docs now name the tracked ways to start implement-next: a plain request in the session (for example "implement 412"), or `/loop docket-implement-next`. They also state, scoped to Claude Code, that a typed `/docket-implement-next` runs untracked. The pages changed are `README.md` (quick start), `docs/install/models-and-effort.md` (the two invocation paths), `docs/reference/glossary.md`, `docs/guide/building-without-supervision.md`, and `docs/concepts/run-tracker.md`. The run-tracker page also says that a later `docket run start implement-next --resume <id>` starts a tracked resume of a change that an untracked run left behind.

Nothing else changed: no skill, agent, harness, or run-tracker code, and the typed command keeps working and stays in the `/` menu. New prose-contract rows in `internal/repoguard/prose_contracts_test.go` (sentinel `change_0556_tracked_start`) pin the sentence "A typed `/docket-implement-next` runs untracked." on each of the five pages, and they pin the absence of the old README drain line and the old glossary example.

## Human actions and testing

### Optional — read the edited pages

The wording of the five pages was checked only by the automated reviewer and the prose-contract guard.

1. Run `git diff origin/main...fix/a-typed-docket-implement-next-in-claude-code-starts-an-untra -- README.md docs/`.
   Expected: each page names a plain request or `/loop` as the way to start, and limits the untracked claim to Claude Code. No page carries a change number.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) through the gate driver, green at the final head. The budget report had screening-only `BUDGET WATCH`/`PARALLEL-SENSITIVE` lines for Go integration test files this change does not touch, and no `SERIAL CONFIRMED OVER BUDGET` line.
- Each new prose-contract row was mutation-checked. Removing the pinned sentence from any of the five pages, or restoring the old README or glossary line, turned `TestProseContracts` red for that file only.
- A whole-repository grep for `/docket-implement-next` and `skill-invoke` in maintained docs found no other page that presents the typed command as the way to start. A second grep found no change-number leakage.
- Whole-branch review (standard tier): 3 findings (1 important, 2 minor), all fixed in-branch. The full table is in the PR body.
- Plan deviation: the plan put the new test rows in the wrong table (`alignmentContracts`), which would have made the guard vacuous. The builder placed them in `proseContracts`, where they are live.
