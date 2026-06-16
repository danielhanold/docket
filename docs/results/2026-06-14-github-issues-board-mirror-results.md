# GitHub board mirror — results
Change: #11 · Branch: feat/github-issues-board-mirror · PR: #11 · Plan: docs/superpowers/plans/2026-06-14-github-issues-board-mirror.md · ADRs: 7

## Verify (human)

The automated suite only exercises `scripts/github-mirror.sh` command construction against a
mocked `gh` in `--dry-run` (no live GitHub — the suite runs against the integration-branch
checkout). Live behavior must be checked at the merge gate with a real `gh` token.

**Live run — 2026-06-15, scratch repo `danielhanold/docket-mirror-test` + auto-created board
`users/danielhanold/projects/1` (both since torn down).** Ran the bare script against the real
`.docket/docs/changes` with `--auto-create-project` and a `project`-scoped token. Confirmed:

- [x] Issues minted one-per-change with the one-way banner and `docket:`-namespaced labels,
      including the computed readiness/waiting labels (`needs-brainstorm`, `build-ready`,
      `waiting/not-yet-built`). (`issue:` write-back is the `docket-status` Board pass's job, not
      the bare script's — see below; persistence itself unchanged from prior passes.)
- [x] `done` changes closed as **completed** in the **same first pass** (close-on-first-mint fix),
      and no `Closes #N` appears anywhere.
- [x] Projects auto-create: a **private** board minted under the repo owner; the **"Docket Status"**
      single-select field carried the five active statuses with **no collision** with the built-in
      "Status" field; every issue added as an item; active items set to the right Docket Status
      (proposed×4, implemented×1) and terminal items correctly left with no Docket Status value.
- [x] `project-minted <owner> <number>` emitted for the `.docket.yml` write-back.

Still to verify (out of scope for that run):

- [ ] **Idempotency re-run** — confirm a second pass **edits** (no duplicate issues/items) once
      `issue:`/`github_project` are persisted. Covered by the mock suite (create-vs-update keyed on
      `issue:`); not re-run live this round.
- [ ] **`killed` → not planned** — no `killed` change exists in the backlog, so the not-planned
      close reason is mock-verified only.
- [ ] **`.docket.yml` write-back path** — the `docket-status` Board pass persisting `project-minted`
      into `.docket.yml` on the default branch (the bare script does no git writes by contract).
- [ ] **Degradation** — token lacking `project` scope skips Projects, keeps Issues (seen
      incidentally before the scope was added; worth an explicit pass).
- [ ] Set `board_surfaces: []` and confirm the Board pass is a clean no-op (no `BOARD.md`, no
      mirror) and nothing authoritative is lost.

**Known board wart (decision: keep separate field).** An auto-created ProjectV2 keeps its built-in
"Status" field (Todo/In Progress/Done), and the project's **default view groups by it** — so our
"Docket Status" column isn't what a human sees first until they switch the view's group-by. We
keep the separate, collision-proof "Docket Status" field deliberately; repurposing the built-in
field was rejected (fragile, re-introduces the collision). Switching the default view is a
one-time human step.

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
