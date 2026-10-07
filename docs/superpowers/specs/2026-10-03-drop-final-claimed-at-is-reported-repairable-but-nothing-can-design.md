<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0496 — Add `docket repository repair` and stop flagging empty claimed_at](../../changes/archive/2026-10-03-0496-drop-final-claimed-at-is-reported-repairable-but-nothing-can.md)**
<!-- docket:backlink:end -->

# `docket repository repair` and the empty-claimed_at false positive — design

## Problem

`docket repository check` on this repository reports 269 `drop-final-claimed-at` warnings, each
`repairable: true` with the remedy "Apply the previewed mechanical repair, or edit the record
frontmatter manually." Two defects sit behind that line, plus a sibling gap found while tracing it.

1. **False positive.** All 269 archived records carry an *empty* `claimed_at:` key; none carries a
   timestamp (verified 2026-10-03 over `docs/changes/archive/`: 269 with the key, 269 empty, 0
   non-empty). An empty key is docket's deliberate cleared form: `MarkDone` / `killedResult` call
   `clearClaimedAt` ("drops the lease stamp entirely"), and the record writer emits a cleared field as
   the bare null form (`lifecycleFieldValue` → `document.Null()`), exactly as it does for `branch:`,
   `pr:`, and the other optional fields. The snapshot validator already agrees: it raises
   `CodeChangeFinalClaimStamp` only when `c.ClaimedAt().State == domain.FieldPresent`. The one
   disagreeing reader is `planClaimedAt` (`internal/reposetup/repair.go`), which keys on the mere
   presence of the `claimed_at` key. Every closeout therefore adds a finding (0491 made 268, 0492
   made 269).
2. **Remedy names a repair nothing runs.** Frontmatter repairs (`ApplyRepairs`) are applied only on
   `migrate`'s legacy path (`gatherMigrationRepairs`). On a migrated repository
   `migratePhaseDispatch` routes `phaseAlreadyMigrated` to `migrateHealthyRepair`, which repairs only
   derived views. `migrate --repair-frontmatter --yes` prints `repository already migrated` and
   changes nothing for these findings.
3. **Sibling gap: derived-view repair lives on the wrong command, and its remedies do not work as
   typed.** Change 0377 placed derived-view repair (inline board, `## Artifacts` blocks, ADR index)
   on `migrate`'s already-migrated path, gated on `--repair-frontmatter`. The remedies that point at
   it say "Run `docket repository migrate`" (`reposetup/derived.go` `DerivedFinding` remedy;
   `app/status.go` `branchMalformedCheck`; the docket-status and docket-adr skills; the
   docket-convention *Board refresh on status writes* paragraph). On a migrated repository a plain
   `migrate` is a no-op (`migrateHealthyRepair` returns `migrateNoOp` unless `o.RepairAuthorized`),
   so none of those remedies works as written, and the flag that does unlock it is named for
   frontmatter.

Because `CheckExit` returns 1 whenever a healthy classification carries any finding, defect 1 alone
means `repository check` can never exit 0 on this repository.

## Decision

- **Repair on a migrated repository is `docket repository repair`.** It owns every mechanically
  repairable finding `repository check` reports: the frontmatter roster and the derived views.
- **`migrate` only migrates.** Its already-migrated path stops repairing; `--repair-frontmatter`
  keeps its one remaining meaning — authorizing the frontmatter roster during a legacy migration.
- **An empty `claimed_at:` is not a claim stamp**, everywhere — `planClaimedAt` aligns with the
  validator. Closeout keeps the uniform bare-null cleared form; it is not special-cased to delete the
  key.

Record this as an ADR at build time (docket-adr, implement-next step 6): it reverses the placement
0377's spec chose ("`docket repository migrate` extends its existing preview/authorization model to
repair the mechanically repairable findings even when topology is already healthy"), and no
existing ADR records that placement, so this is a new ADR, not a supersession.

## Design

### 1. `planClaimedAt` ignores an empty value

`planClaimedAt` returns no finding when the located `claimed_at` value is empty — decide on the
parsed value (the decode layer's `FieldEmpty` notion, as `optionalTime` produces it), not on a byte
spelling, so every spelling the decode layer reads as empty (`claimed_at:`, and any of `''`, `null`,
`~` it also maps to `FieldEmpty`) is "no stamp". Any non-empty value — well-formed (`FieldPresent`)
or malformed (`FieldMalformed`) — continues to the existing logic unchanged: final archived →
repairable `drop-final-claimed-at`; non-final archived → manual-review. The roster entry, `buildRepair`'s `RepairDropClaimedAt` branch,
`ApplyRepairs`, and `RepairDigest` are otherwise unchanged.

Effect: the 269 current findings disappear with no record rewrite; future closeouts add none. No
backlog-clearing write is needed or performed.

### 2. New command `docket repository repair`

A new `repository` subcommand and capability-catalog operation `repository.repair`
(effects: `metadata-write`; signature `[--repo-dir <dir>] [--yes]` plus whatever the existing
confirm flow needs to carry the pinned revision, mirroring `migrate`'s `ExpectedSource` handshake).
Register it everywhere `repository migrate` is registered: the cobra command group, the capability
catalog, and the `internal/cli/install.go` known-command set.

**Preconditions.** The remote docket metadata branch is present and the topology routes to the
already-migrated state (the same routing `migrateRoute` uses to reach `phaseAlreadyMigrated`). Any
other state is a typed refusal whose remedy names `docket repository migrate` (legacy, partial,
half-migrated — ADR-0099: migrate is the only legacy exit). Fact gathering failures map the same way
`migrate`'s do (`migrateExternalFailure` analogue), never to a false clean.

**What it repairs.** Exactly the repairable set `repository check` reports from the same corpus read
(`readCheckCorpus` at the pinned metadata tip):

- frontmatter roster findings — `reposetup.PlanRepairs` per change record (archived flag as `check`
  passes it), repairable entries only;
- derived-view findings — `derivedViewFindings` repairable entries (board, artifact-links blocks,
  ADR index).

Non-repairable findings of either kind are listed in the preview as manual review and never touched
(malformed/unbalanced managed markers keep their existing manual-review posture).

**Composition order.** Apply the frontmatter repairs to the in-memory corpus first
(`reposetup.ApplyRepairs` per record, which re-derives and re-proves each patch), then build the
snapshot and compute derived-view findings over the *repaired* corpus, so a record that needs both a
frontmatter fix and an `## Artifacts` re-render — or a `scalar-to-list` fix that changes what the
board renders — comes out canonical in one pass. The written file set is the union of
frontmatter-repaired records and derived-view-repaired files; each written blob is the final
composed bytes.

**Two-pass authorization.** Without `--yes`: return the preview — repository, remote, the exact
pinned metadata revision, each repair as `[code] path` (frontmatter entries also show their patch
preview), and the manual-review list — as a confirmation-required result, writing nothing. With
`--yes`: re-prove the pinned revision (a moved tip is `contended`, never an overwrite), compose the
tree from the pinned tip with `BuildTree`/`CommitTree`, publish one descendant commit under an exact
`PushLease` keyed on the pinned tip, then re-read the remote tip byte-exactly. This is
`executeDerivedRepair`'s existing machinery, moved and extended — reuse it, do not fork a second
publisher. Nothing repairable → idempotent `no-op`. The success result names the new metadata tip,
the prior tip, the repaired file set, and the pending-local remedy (re-run `docket repository
prepare` to fast-forward `.docket`), as `derivedRepairApplied` does today.

**Commit subject.** The repair descendant's subject is `docket: repository repair` (replacing
`migrateRepairSubject`'s "docket: repair derived views").

**Result shape.** A `RepositoryRepairResult` (protocol-v1 envelope; `result` from the shared
vocabulary: `applied` | `no-op` | `contended` | `invalid-state` | `external-failed` |
`internal-error`) carrying the pinned/new metadata revisions, the repaired file set, and, on a
preview, the planned repairs. Add its `schema` descriptor alongside the other operations'.

### 3. `migrate` on a migrated repository

`migratePhaseDispatch`'s `phaseAlreadyMigrated` branch no longer calls a repair path; it returns the
idempotent no-op. Its human output names the new command (`repository already migrated; for
mechanical repairs run docket repository repair`) — including, explicitly, when
`--repair-frontmatter` was passed, so a migrated-repo run never again prints a bare "already
migrated" and silently ignores the flag. `migrate` never writes on this path. The legacy path's
frontmatter-repair behavior (`gatherMigrationRepairs`, `decideMigrateAuthorization`,
`migrateRepairAuthorizationRequired`) is unchanged.

### 4. Remedies that name a working command

Every remedy, skill instruction, or doc line that sends a reader to `migrate` to repair a derived view
or a frontmatter roster finding on a migrated repository is retargeted to `docket repository repair`
(agent-executed skill surfaces name the `repository.repair` catalog operation per ADR-0104, not a
hard-coded spelling). Derive the site list from a whole-repo grep for `repository migrate`,
`repair-frontmatter`, and `previewed mechanical repair`, then sort prose vs executable (AGENTS.md
*Guards and tests*). Known starting points, not an exhaustive list:

- `reposetup.frontmatterFinding` repairable remedy → "Run `docket repository repair` to preview and
  apply this repair, or edit the record frontmatter by hand." (`check` reads the corpus only when the
  remote docket branch is present, so this one text is valid in every state that produces it.)
- `reposetup` `DerivedFinding` remedy (`derived.go`) → `docket repository repair`.
- `app/status.go` `branchMalformedCheck` hand-edit remedy → "… then run: docket repository repair to
  re-render the board".
- `skills/docket-adr/SKILL.md` (ADR-index drift repair block), `skills/docket-status/SKILL.md`
  (`artifact-links-stale` self-heal sentence), `skills/docket-convention/SKILL.md` (*Board refresh
  on status writes*: "repaired by an authorized, human-typed `docket repository migrate`").

Remedies that legitimately name `migrate` — legacy, half-migrated, local-attachment recovery — stay.
The `internal/repoguard/capability_surface_test.go` exemption count for `docket repository migrate`
drops by however many skill sites move to the catalog id; adjust it to the measured value (never add
a `docket repository repair` hard-coded exemption for an agent-executed surface).

## Error handling

- Corpus read failure → error result naming the failed read; never a clean no-op.
- `ApplyRepairs` rejection (tampered/stale finding, failed postcondition) → `internal-error` naming
  the record; nothing is written.
- Renderer failure composing a derived file → `internal-error`; nothing is written.
- Lease lost / remote moved after push → `contended` naming both revisions.
- Legacy / partial / unknown topology → typed `invalid-state` refusal naming `docket repository
  migrate`.

## Testing

- `planClaimedAt`: an archived final record with an empty `claimed_at` (each empty spelling the decode
  layer maps to `FieldEmpty`) yields no finding; a timestamp, and a malformed non-empty value, still
  yield `drop-final-claimed-at` with an unchanged patch preview; a non-final archived record with a
  timestamp still yields manual review.
  Mutation-test: restoring key-presence semantics must redden the empty-value case.
- `repository check` over a corpus whose only `claimed_at` keys are empty yields no frontmatter
  findings (healthy + no other findings → exit 0).
- `repository repair`: preview without `--yes` writes nothing and lists both kinds; `--yes` produces
  exactly one descendant commit containing exactly the listed files; a moved tip between preview and
  apply → `contended`; legacy repository → refusal naming `migrate`; nothing repairable → `no-op`; a
  record needing both a frontmatter fix and an `## Artifacts` re-render ends canonical (a following
  `check` reports neither); a malformed managed marker → listed as manual review, file untouched.
- `migrate` on a migrated repository — with and without `--repair-frontmatter`, with and without
  `--yes` — writes nothing and names `docket repository repair` in its output.
- Existing `migrateHealthyRepair` tests move to the repair command rather than being deleted.
- Real-Git / remote tests go in the established integration partition (no test or shard near the
  60-second budget); pure planning/composition stays in unit tests.
- A repo-wide assertion that no `check`/`status` remedy for a derived-view or frontmatter-roster
  finding names `docket repository migrate` — keyed on the finding family, mutation-tested.

## Out of scope

- Other frontmatter repair codes beyond moving them to the new command — the roster's membership and
  rules are unchanged.
- Any new blocking gate: frontmatter and derived-view findings stay visibility-only warnings.
- Changing closeout's cleared-field form, or rewriting the 269 records.
- Agent-autonomy posture for running `repair --yes`: it carries over unchanged from today's
  `migrate --repair-frontmatter` posture.
- Broader docs alignment (README / guide) beyond the remedy sites above — tracked by change 0464.
