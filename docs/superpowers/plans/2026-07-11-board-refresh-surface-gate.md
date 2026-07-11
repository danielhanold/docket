# Board-refresh surface gate implementation plan

**Change:** 0059 — board-refresh honors `board_surfaces`

**Goal:** Make the inline board write decision deterministic and surface-aware so disabled or
GitHub-only configurations never create, truncate, or rewrite `BOARD.md`, while preserving the
existing renderer and each caller's must-land/best-effort git discipline.

**Architecture:** Add `scripts/board-refresh.sh` as the only file-writing entry point for the
inline surface. It accepts the already-resolved surface tokens, delegates rendering to
`render-board.sh` only when `inline` is present, and otherwise leaves the filesystem untouched.
Skill prose calls this helper and commits only an actual `BOARD.md` diff.

**Tech stack:** Bash, Markdown skill/contracts, shell test suite (`tests/test_*.sh`).

**Spec:** `docs/superpowers/specs/2026-07-10-board-refresh-surface-gate-design.md` on the `docket`
metadata branch.

## Constraints

- Keep `render-board.sh`'s stdout-only contract and rendering behavior unchanged.
- Treat `--surfaces ""` as a valid explicit value; a missing `--surfaces` flag is an error.
- Unknown surface tokens warn and do not abort; only the exact `inline` token enables a write.
- A disabled run never creates, writes, truncates, or deletes `BOARD.md`.
- The helper performs no git operations.
- Preserve each caller's existing must-land or best-effort push posture.
- Update the entire grep-discovered call-site/sentinel inventory, not only the spec's examples.

### Task 1: Specify and implement the gated helper

**Files:**

- Create `tests/test_board_refresh.sh`
- Create `scripts/board-refresh.sh`
- Create `scripts/board-refresh.md`

1. Write a hermetic test fixture with `active/` and `archive/` directories and a minimal proposed
   change. Cover inline output byte-for-byte against `render-board.sh`, empty surfaces, GitHub-only,
   inline+GitHub, the pre-existing-file truncation trap, unknown-token warning, missing surfaces,
   and missing/invalid changes directories.
2. Run `bash tests/test_board_refresh.sh`; confirm it fails because the helper does not exist.
3. Implement strict argument parsing while tracking whether `--surfaces` was supplied separately
   from whether its value is empty. Validate the changes directory, warn for unknown tokens, and
   return without touching the board when `inline` is absent.
4. When enabled, render to a temporary file in the changes directory and move it onto `BOARD.md`
   only after `render-board.sh` succeeds, preventing a renderer failure from truncating the prior
   board. Clean the temporary file on exit. Forward `--repo` when present.
5. Write the co-located script contract, including usage, behavior matrix, diagnostics, exit codes,
   atomic-write behavior, no-git invariant, and stale-board no-op decision.
6. Run `bash tests/test_board_refresh.sh` and
   `bash tests/test_script_contracts_coverage.sh`; confirm both pass.
7. Commit: `feat(0059): add gated inline board refresh helper`.

### Task 2: Compose the helper into docket-status's orchestrator (rescoped after 0058 merged)

**Files:**

- Modify `scripts/docket-status.sh`
- Modify `tests/test_docket_status.sh`
- Modify `tests/test_render_board.sh`

> **Rescope note.** The original Task 2 edited `skills/docket-status/SKILL.md`'s raw
> `render-board.sh > BOARD.md` redirect. Change 0058 merged first: it moved the Board pass out of
> that SKILL into `scripts/docket-status.sh` (`board_pass` / `board_pass_inline`), which already
> gates on the `inline` token and renders truncation-safely. So `docket-status/SKILL.md` is **left
> at main** (no edit), and the helper is composed into the orchestrator instead.

1. Add a wiring sentinel to `tests/test_docket_status.sh`: `board_pass_inline` must route through
   `/board-refresh.sh` and must **not** call `/render-board.sh` directly (so render-board.sh is
   reached only via the gated helper). Run it; confirm the two assertions fail.
2. Refactor `scripts/docket-status.sh` `board_pass_inline`: replace both `render-board.sh … > tmp`
   sites (initial render + the rebase-conflict regenerate loop) with
   `board-refresh.sh --changes-dir … --surfaces inline [--repo …]`. `board_pass` already gates on
   `inline`, so pass it verbatim; `board-refresh.sh` owns the atomic write. Detect "changed" via
   `git -C "$mw" status --porcelain -- "$CHANGES_DIR/BOARD.md"` (the git-add pathspec form verified
   to work under `git -C`; a full `$mw/…/BOARD.md` pathspec fatals). Preserve the commit/push +
   bounded rebase-retry loop and "never 3-way merge" behavior exactly.
3. In `tests/test_render_board.sh`, **leave** the docket-status SKILL sentinel at main
   (`grep -qF "/render-board.sh" "$SKILL"` — the SKILL still describes render-board.sh) and **keep**
   the general negative-redirect scan + positive control; do not add SKILL-coupled board-refresh.sh
   assertions (that edit is dropped).
4. Run `bash tests/test_docket_status.sh`, `bash tests/test_render_board.sh`, and
   `bash tests/test_board_refresh.sh`; confirm all pass (including the behavioral board_pass +
   conflict-regenerate cases, which validate the composition against real git).
5. Commit: `docs(0059): route docket-status through board-refresh`.

### Task 3: Make every status-writing Board-pass caller explicit

**Files:**

- Modify `skills/docket-new-change/SKILL.md`
- Modify `skills/docket-groom-next/SKILL.md`
- Modify `skills/docket-auto-groom/SKILL.md`
- Modify `skills/docket-finalize-change/SKILL.md`
- Modify `skills/docket-implement-next/SKILL.md`
- Modify `skills/docket-convention/SKILL.md`
- Modify `skills/docket-convention/references/terminal-close-out.md`
- Modify `tests/test_board_refresh_on_transition.sh`

1. Extend transition sentinels to require every listed caller to name the gated Board pass and the
   diff-only rule while preserving existing must-land/best-effort assertions.
2. Run `bash tests/test_board_refresh_on_transition.sh`; confirm the new assertions fail.
3. Reword each caller's Board step to invoke `board-refresh.sh` with resolved surfaces, and commit
   only if `BOARD.md` changed. Keep must-land retry language for interactive/groom/finalize paths
   and bounded best-effort language for implement-next.
4. Update the convention's status-write rule and derived-view family to identify
   `board-refresh.sh` as the inline entry point and `render-board.sh` as its internal renderer.
   Update the shared terminal close-out Board step consistently.
5. Exhaustively grep `skills/`, `scripts/`, and `tests/` for stale direct redirects and for Board
   pass wording that could still instruct an unconditional board commit.
6. Run `bash tests/test_board_refresh_on_transition.sh`,
   `bash tests/test_convention_extraction.sh`, and `bash tests/test_closeout.sh`; confirm they pass.
7. Commit: `docs(0059): gate every board refresh call site`.

### Task 4: Whole-branch verification

1. Run `bash -n scripts/board-refresh.sh tests/test_board_refresh.sh`.
2. Run `shellcheck scripts/board-refresh.sh tests/test_board_refresh.sh` if `shellcheck` is
   installed; fix actionable findings without weakening tests.
3. Run every test with `for t in tests/test_*.sh; do bash "$t" || exit 1; done`.
4. Run the grep gate proving no skill contains a direct `render-board.sh` to `BOARD.md` redirect.
5. Inspect `git diff origin/main...HEAD`, verify only planned files changed, and confirm metadata
   files are absent from the feature branch.
