# GitHub board mirror — results
Change: #11 · Branch: feat/github-issues-board-mirror · PR: <pending> · Plan: docs/superpowers/plans/2026-06-14-github-issues-board-mirror.md · ADRs: 7

## Verify (human)

The automated suite only exercises `scripts/github-mirror.sh` command construction against a
mocked `gh` in `--dry-run` (no live GitHub — the suite runs against the integration-branch
checkout). Live behavior must be checked at the merge gate with a real `gh` token:

- [ ] On a test repo, set `board_surfaces: [inline, github]` and run `docket-status`; confirm one
      issue is minted per change with the one-way banner, `docket:`-namespaced labels, and the
      artifact hrefs, and that the change file's `issue:` is recorded on `docket`.
- [ ] Re-run `docket-status`; confirm it **edits** (no duplicate issues) — idempotency holds.
- [ ] Drive a change to `done` and one to `killed`; confirm the issues close as **completed** and
      **not planned** respectively, and that no `Closes #N` ever appeared in a PR body.
- [ ] With a `project`-scoped token, confirm the private Projects v2 board auto-creates, the
      `github_project` ref is written back into `.docket.yml` on the default branch, and items
      carry the right Status; with a token lacking `project` scope, confirm Projects is skipped
      and Issues still mirror (best-effort degradation).
- [ ] Set `board_surfaces: []` and confirm the Board pass is a clean no-op (no `BOARD.md`, no
      mirror) and nothing authoritative is lost.

## Findings

- The one-way boundary + the deterministic-script execution model became **ADR-0007** — both set
  precedents (source-of-truth direction; when docket may use a script vs prose), so they were
  worth freezing.
- Dry-run traces had to go to **stderr**, not stdout: command substitution (`$(run_gh …)` for the
  created issue number) and `… >/dev/null` on real calls would otherwise swallow the trace,
  making the surface untestable. The mock-`gh` test caught this on the first run.

## Follow-ups

- **Cross-branch config write (residual, flagged in spec §8).** The first-sync `github_project`
  write-back lands on the *default/integration* branch (where `.docket.yml` lives), not `docket` —
  the one mirror write outside the metadata branch. The script deliberately does **no** git writes
  (it emits `issue-minted` lines); the Board pass owns persistence. Confirm the commit mechanism
  (fetch → edit → commit → push, race-tolerant) when the live Projects path is first exercised.
- **Projects field-schema drift.** If a human renames/reorders the auto-created Status options,
  cached option ids go stale. Decide repair posture (re-resolve by option name each pass vs.
  cache-and-heal) when Projects is wired against a live board.
- **Token-scope detection.** Distinguishing "no `project` scope" (skip Projects, keep Issues) from
  a transient GraphQL failure (retry next pass) is currently best-effort; tighten the signal once
  tested against a live token.
- **Body `plan:`/`results:` hrefs** resolve on the integration branch only after merge; pre-merge
  they 404. Acceptable (links appear as the change advances), but worth a note in the banner if it
  confuses readers.
