# Explain repository check and status findings in plain language — design

Change 0553 — Explain repository check and status findings in plain language

Change #553, groomed 2026-10-15 through an interactive brainstorm with Daniel.

## Problem

`docket repository check` and `docket status` report record-level problems in vocabulary only a docket developer can decode. A real report from a consuming repository:

```
repository check: no-op (healthy)
- [warning] drop-final-claimed-at (docs/changes/archive/2026-05-28-0001-wisdom-ai-bastion.md)
  claimed_at on a final archived change; remove the stale claim lease
  remedy: Run `docket repository repair` to preview and apply this repair, or edit the record frontmatter by hand.
```

The reader cannot tell what is wrong, whether it matters, or why it happened. "no-op" is envelope vocabulary; "healthy" next to a warning (and exit 1) reads as a contradiction; "final" and "claim lease" are internal terms.

Three families are cryptic today. The git/worktree topology findings in `internal/reposetup/health.go` are already full sentences and are not touched.

1. **Record validation.** `corpusFindings` (`internal/app/repository_check.go`) folds every error-severity validator finding into a `frontmatter-manual-review` finding whose message is `"metadata corpus finding: " + code`. It drops the finding's `Detail`, `Field`, and `Related` data and attaches one generic remedy: "Edit the record frontmatter manually; this shape is outside the mechanical repair roster." `docket status` renders the same codes through `findingMessage` (`internal/app/status.go`) as the bare code plus `key=value` details, with no remedy.
2. **Frontmatter repair.** The messages built by `PlanRepairs` (`internal/reposetup/repair.go`) — for example "unsafe scalar shape with ambiguous decoded value; needs manual review".
3. **Generated views.** The board, ADR index, artifact-links, and artifact-backlink messages built in `internal/app/repository_check.go` — for example "the inline board differs from the canonical render of the current metadata." Their repairable remedy, built by `DerivedFinding.Finding` (`internal/reposetup/derived.go`), says "or edit the file by hand", which the convention forbids for BOARD.md, the ADR index, and the generated blocks.

## Goal and success criteria

Every finding these surfaces show answers three questions in plain words: **what is wrong**, **whether it matters**, and **what to do**. Daniel can act on any finding without opening the source. Agents relaying findings get the same text through JSON.

Constraints settled in the brainstorm:

- Human text only. Finding codes stay stable, with one deliberate exception (Section 5). `RepairDigest`, the retired-vocabulary guard, and JSON consumers keep their codes.
- No new blocking gate. Severities, exit codes, detection, and repair behavior are unchanged.
- One level of verbosity. There is no `--explain` flag: each finding always carries its "whether it matters" sentence.

## 1. Output shape

### The `repository check` header

`RepositoryCheckResult.HumanText` stops printing the envelope result (`no-op`). It names the classified **setup** state in plain words, then counts findings by severity, errors first:

```
repository check: setup healthy, no problems found
repository check: setup healthy, 1 warning (read-only, nothing was changed)
repository check: old single-branch layout, 1 error, 2 warnings (read-only, nothing was changed)
```

| `repository_state` | Header phrase |
|---|---|
| `healthy` | setup healthy |
| `fresh` | not set up yet |
| `legacy` | old single-branch layout |
| `partial` | migration interrupted |
| `needs-review` | setup waiting for your review |
| `conflict` | setup conflict |
| `unknown` | setup state unknown |

The phrase lookup is keyed on the `reposetup.State` constants. A test fails if a state has no phrase. The gather-failure header built by `checkGatherFailure` (`repository check: <result>: <error>`) is unchanged. The JSON envelope (`result`, `repository_state`) is unchanged.

### The finding block

`appendFindingBlock` (`internal/app/config_diagnostics.go`) renders up to four lines, so the refusal blocks that share it get the same shape:

```
- [warning] drop-final-claimed-at (docs/changes/archive/2026-05-28-0001-wisdom-ai-bastion.md)
  This finished change still has a `claimed_at` timestamp from when it was being built.
  It's harmless: nothing reads it once a change is closed. Older docket versions left it behind.
  remedy: Run `docket repository repair` to remove it (it shows the edit first), or delete the `claimed_at:` line by hand.
```

Line 2 is `message`, line 3 is `impact`, and line 4 is `remedy`. An empty part prints nothing, so a finding without an `impact` keeps today's three-line shape.

`docket status` keeps its grouped layout (`writeFinding`, `internal/app/status_human.go`). The `impact` sentence prints as a four-space continuation line under the `code locator — message` row, before the existing `remedy:` block.

### JSON

`reposetup.Finding` and `StatusFinding` each gain an optional `impact` string (`json:"impact,omitempty"`). `message` carries only the what-is-wrong sentence. The `schema` operation's descriptors gain the field, and the schema fidelity tests cover it.

## 2. The shared validation-text table

A new file in `internal/app` holds one entry per validator code. Given a `domain.Finding`, it returns `{message, impact, remedy}`, filled from what the finding carries:

- the entity (kind, id, slug, path)
- `Field`
- `Related` (rendered as change or ADR ids)
- `Detail`

Both callers use it:

- `validationFinding` (`internal/app/status.go`) takes all three parts from the table.
- `repository check` takes the same three parts for its validator-backed findings (Section 5).

**Coverage set.** The table covers every `Code`-prefixed string constant declared in the non-test files of `internal/domain` and `internal/repository`, except those in `internal/repository/evolution.go`. Evolution codes come only from `ValidateEvolution`, which only the transaction engine calls, so they never reach `status` or `check`. The transaction subpackage's scope codes sit in their own package and are outside the set. That leaves 37 codes today.

**Fallback.** A code with no entry renders today's text (code plus sorted `key=value` details, as `findingMessage` does now), never an empty message. The coverage guard (Section 6) keeps that path from being reached in practice.

**Missing data.** Where a finding does not carry the value its message needs to name the specific problem, the build adds a `Detail` key at that finding's single producer. The code stays the same, and `Detail` is additive. The results file lists every key added.

## 3. Writing rules

Every message, impact, and remedy follows these rules:

1. **Message: one sentence naming the specific record and value.** For example: "Change 0263 lists #172 under `depends_on`, but no change 172 exists." Field names go in backticks.
2. **Impact: one sentence on what goes wrong if the problem is left alone.** When nothing goes wrong, say "It's harmless" outright. Add how the problem usually arises when that is known.
3. **Remedy: something the reader can do as written.**
   - A command must be valid in the exact state that produced the finding (learning `printed-remedy-state-validity`).
   - A hand edit must name the file and field. "Resolve manually" alone is not a remedy.
   - Never suggest hand-editing a generated file (BOARD.md, the ADR index, the `## Artifacts` block, a `docket:backlink` block).
   - Where a docket command can make the edit, name it.
   - Where the only fix is a hand edit of a record, say to run `docket repository repair` afterwards, so the board and link blocks catch up.
4. **Real locations.** A hand-edit remedy names the record's path inside the actual metadata checkout: `.docket/…` in a shared repository, the outside-the-clone checkout in a private one. The path comes from the resolved repository layout, never a hard-coded `.docket`.
5. **No internal vocabulary.** Avoid words such as "canonical render", "roster", "scalar", "corpus", "lease", "snapshot", "postcondition", and "final" (say "done or killed"). Use the words a reader already has: change, ADR, learning, record, field, board, branch, spec, plan, results.

## 4. Frontmatter-repair and generated-view findings

These messages stay at their producers and are rewritten in place under the Section 3 rules. Their `impact` and remedy move to the same three-part shape, and they all appear in the catalog file (Section 6). Target wording:

| Code | Message | Impact | Remedy |
|---|---|---|---|
| `drop-final-claimed-at` | This finished change still has a `claimed_at` timestamp from when it was being built. | It's harmless: nothing reads it once a change is closed. Older docket versions left it behind. | Run `docket repository repair` to remove it (it shows the edit first), or delete the `claimed_at:` line by hand. |
| `scalar-to-list` | `<field>: <value>` is written as plain text, not a list. | docket can't read it as a list, so it ignores these ids. | Run `docket repository repair` to rewrite it as `[<ids>]`, or edit it by hand. |
| `quote-unsafe-scalar` | `<field>: <value>` is unquoted, and YAML can misread values shaped like this. | docket reads it correctly today; quoting is safe and does not change the value. | Run `docket repository repair` to add the quotes, or quote it by hand. |
| `board-stale` | BOARD.md doesn't match the current change records. | The board shows out-of-date statuses. This usually means a record was edited with plain git instead of a docket command. | Run `docket repository repair` to regenerate it (it shows the change first). |
| `artifact-backlink-stale` | The link at the top of `<artifact>` back to its change points to the wrong place. | Following it from the artifact leads to a missing file. This usually happens after the change was archived. | Run `docket repository repair`. |
| `artifact-backlink-shared` | `<artifact>` is linked from more than one change (`<ids>`). | docket can't tell which change its back-link should point to, so it leaves the link alone. | Remove the link from the change that doesn't own the file, then run `docket repository repair`. |
| `corpus-unreadable` | docket couldn't read the records on the docket branch: `<error>`. | None of the record checks ran, so this result says nothing about the records. | Fix the read error, then run `docket repository check` again. |

The remaining codes in these families follow the same pattern:

- `artifact-links-stale`, `artifact-links-missing`, `adr-index-stale`
- the `*-malformed` marker findings: their message turns the parser error into plain words, naming the file and saying which marker is unbalanced
- the four non-repairable frontmatter shapes: an ambiguous unquoted value, a non-id list, `claimed_at` on an archived record that is not done or killed, and an undecodable frontmatter block

For the frontmatter shapes, `DerivedFinding` and `RepairFinding` gain an `Impact` (and, where it is per-finding, a `Remedy`) so the producers own their text. `frontmatterFinding` and `DerivedFinding.Finding` lift those parts into `reposetup.Finding`. Messages that name other changes, such as the shared-backlink ids, compute them at the producer.

**Generated-view remedy fix.** A repairable generated-view finding's remedy names only `docket repository repair`; it no longer offers a hand edit. A non-repairable (`*-malformed`) one says how to restore the markers. `TestRepairableFamilyRemediesNameRepairNotMigrate` keeps passing unchanged.

## 5. `repository check` reports validator findings under their own code

The one code change. A validator finding surfaced by `repository check` is reported under **its own code** (for example `change-reference-dangling`), the same code `docket status` shows, instead of `frontmatter-manual-review`. It keeps error severity, its `Ref` path, and a non-nil `Repairable` of `false`, so it stays in the manual-review population. Its message, impact, and remedy come from the Section 2 table.

`frontmatter-manual-review` remains the code for the four genuine frontmatter-shape problems that `PlanRepairs` reports as non-repairable.

`docket repository repair`'s list of what it can't fix (`planRepositoryRepair`, `internal/app/repository_repair.go`) passes the real code through instead of the hard-coded `frontmatter-manual-review`.

Unchanged: the per-path suppression in `corpusFindings` (a path already named by `PlanRepairs` is not reported twice), and the rule that only error-severity validator findings reach `check`.

## 6. Testing and guards

- **Coverage guard.** A test parses the coverage set's constants directly from source with `go/parser`. It never uses a hand-written list. It fails when a code has no table entry or an entry has an empty message, impact, or remedy. Mutation probe: deleting one entry turns it red.
- **Catalog file.** A test renders every message from a representative fixture finding into one file under `internal/app/testdata/` and compares it, using an update flag (the pattern the harness tests use). The file covers:
  - every validator code
  - every frontmatter-repair and generated-view finding
  - the check header for every setup state

  All the wording lives in one file a human reads top to bottom, and every later wording change shows up in the PR diff.
- **Header guard.** Each `reposetup.State` constant has a phrase. A healthy state with findings renders the counted form, and one without renders "no problems found".
- **Generated-view remedy guard.** A repairable generated-view remedy must not offer a hand edit. Mutation probe: restoring "or edit the file by hand" turns it red.
- **Code-switch guard.** A check over a corpus with a dangling reference reports the validator's code, not `frontmatter-manual-review`. Mutation probe: restoring the wrapper code turns it red. `TestRepairableFamilyRemediesNameRepairNotMigrate`'s fixture gains one validator finding so its population covers the switch.
- **Whole suite at the build gate**, as always.

## 7. Known sites that pin today's text

- `internal/bashupgrade/registry_test.go` expects `repository check: no-op (healthy)`.
- The header tests in `internal/app/config_diagnostics_test.go` and `internal/app/configrefusal_integration_test.go`. The refusal header form `repository check: <result>: …` is unchanged; only the healthy-path header moves.
- Tests that assert the old message strings (for example `repository_check_derived_test.go`, `repository_repair_test.go`, `repocheck_integration_test.go`, `derived_test.go`).
- The docket-status skill's findings paragraph (`skills/docket-status/SKILL.md`) and its embedded copy, if its wording relies on the old check behavior. Regenerate with `go generate ./internal/assets/`.

No guide or README quotes this output verbatim, so no user docs change.

## Out of scope

- Topology findings (`internal/reposetup/health.go`), config diagnostics, `artifact-missing`, and `branch-malformed` — already plain sentences.
- Evolution and transaction-scope codes. They never reach `status` or `check`.
- What is detected, severities, exit codes, and repair behavior.
- Any new blocking gate.
- A `--explain` flag or other verbosity switch.
- Naming the artifact-backlink codes in the docket-status skill (change 539).

## Decisions from the brainstorm

- Each finding states what is wrong **and** whether it matters, always on: option A over a one-line message or an `--explain` flag.
- `repository check` reports validator findings under their own code rather than `frontmatter-manual-review`.
- The validation text lives in one app-layer table that both commands call. The domain layer stays text-free, and the frontmatter and generated-view producers keep their own rewritten text.
- Whether "validation text lives in one app-layer table" warrants its own ADR is left to the build.
