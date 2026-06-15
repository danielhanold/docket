# GitHub board mirror — results
Change: #11 · Branch: feat/github-issues-board-mirror · PR: #11 · Plan: docs/superpowers/plans/2026-06-14-github-issues-board-mirror.md · ADRs: 7

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
- [ ] With a `project`-scoped token, run `docket-status` with `github_project` unset; confirm the
      private Projects v2 board auto-creates (via `--auto-create-project`), `project-minted` is
      recorded as `github_project` in `.docket.yml` on the default branch, the **"Docket Status"**
      single-select field carries the five active statuses, and each issue is added as an item with
      the right Status. Re-run with `github_project` set; confirm it links items (no second board).
      With a token lacking `project` scope, confirm Projects is skipped and Issues still mirror.
      **Live-only checks** (the mock suite can't reach these): that "Docket Status" doesn't collide
      with a project's default field, and that `gh project create` lands the board private.
- [ ] Set `board_surfaces: []` and confirm the Board pass is a clean no-op (no `BOARD.md`, no
      mirror) and nothing authoritative is lost.

## Findings

- The one-way boundary + the deterministic-script execution model became **ADR-0007** — both set
  precedents (source-of-truth direction; when docket may use a script vs prose), so they were
  worth freezing.
- Dry-run traces had to go to **stderr**, not stdout: command substitution (`$(run_gh …)` for the
  created issue number) and `… >/dev/null` on real calls would otherwise swallow the trace,
  making the surface untestable. The mock-`gh` test caught this on the first run.
- **Standalone re-mint (post-merge field report).** Running the script by hand twice re-minted
  issues. Two compounding causes, both addressed: (1) it was pointed at the *integration-branch*
  `docs/changes`, where `active/` is pruned — so it only ever saw the archived changes (none with
  an `issue:`) and re-created them. Added a **wrong-tree guard**: an empty `active/` beside a
  populated `archive/` warns loudly (best-effort, never aborts). (2) Idempotency is keyed on the
  change-file `issue:` field, and the script does **no** git writes by contract — so it is
  idempotent only once the Board pass persists the `issue-minted` numbers. Bare back-to-back runs
  with nothing committing `issue:` will re-create; the orchestrated `docket-status` loop closes
  that. We kept the write-back design (no GitHub-side reconciliation lookup) deliberately.
- **Close-on-first-mint (field report).** A change already terminal on its *first* sync was
  created **open** and only closed on a later pass, because close-state keyed on the pre-existing
  `issue:` field, which is empty on a fresh mint. Surfaced when first-syncing a backlog with 9
  `done` changes (every issue stayed open). Fixed: close now keys on the *effective* number (the
  existing `issue:` or the one just minted), so an already-`done`/`killed` change mints **and**
  closes in the same pass.
- **Projects v2 auto-create** (previously a stub) is now built on native `gh project` subcommands
  (`create` / `field-create` / `item-add` / `item-edit`) rather than hand-rolled GraphQL — far
  more robust, and the spec (§4.4) explicitly permits it. Auto-create is **opt-in**
  (`--auto-create-project`) so a bare/ad-hoc run never silently mints a board — same footgun
  instinct as the wrong-tree guard. The field is named **"Docket Status"** (not "Status") to avoid
  colliding with a project's default field, matching the `docket:` label-namespace philosophy.

## Follow-ups

- **Cross-branch config write (residual, flagged in spec §8).** The first-sync `github_project`
  write-back lands on the *default/integration* branch (where `.docket.yml` lives), not `docket` —
  the one mirror write outside the metadata branch. The script deliberately does **no** git writes
  (it emits `issue-minted` lines); the Board pass owns persistence. Confirm the commit mechanism
  (fetch → edit → commit → push, race-tolerant) when the live Projects path is first exercised.
- **Projects field-schema drift.** The sync re-resolves the "Docket Status" field + option ids by
  **name** every pass (`gh project field-list … --jq`), so a human renaming an *option* silently
  drops that status's column value until renamed back; renaming the *field* away from "Docket
  Status" makes the sync re-create it. Re-resolve-by-name is the chosen posture (no cached ids to
  go stale); revisit only if it proves noisy on a live board.
- **Projects option-resolution is O(statuses) field-list calls.** Each pass resolves the field id
  once and each option id with its own `field-list` round-trip (≤6/pass). Fine at this scale;
  collapse into a single cached parse if Projects sync ever gets hot.
- **Token-scope detection.** Distinguishing "no `project` scope" (skip Projects, keep Issues) from
  a transient GraphQL failure (retry next pass) is currently best-effort; tighten the signal once
  tested against a live token.
- **Body `plan:`/`results:` hrefs** resolve on the integration branch only after merge; pre-merge
  they 404. Acceptable (links appear as the change advances), but worth a note in the banner if it
  confuses readers.
