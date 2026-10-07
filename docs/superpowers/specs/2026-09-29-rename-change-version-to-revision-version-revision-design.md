<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0472 — Rename change version to revision (--version → --revision)](../../changes/archive/2026-09-30-0472-rename-change-version-to-revision-version-revision.md)**
<!-- docket:backlink:end -->

# Revision rename (ADR-0129 family (b)) — design

**Change:** 0472 · **Date:** 2026-09-29 · **Status:** groomed · **Decision record:** ADR-0129 (family (b) rows amended in place 2026-09-29 at this grooming)

## Problem

Every mutating docket operation pins the exact record it read, so a concurrent edit is refused (compare-and-swap) instead of silently overwritten. The pin is spelled `--version <blob id>` as a flag and `version` in request files and reads. That reads like a software version and clashes with `docket version`. ADR-0129 family (b) renames it to **revision**.

Grooming traced the code (2026-09-29, `main` at `32fd8adea`) and found:

- **Rows 40–44 were incomplete.** Row 40 listed 22 flag operations, but the flag is on 14. Seven more take the revision as a request-file key, and `workspace.inspect` takes none (its shared request struct's `version` is ignored). Five sites were not listed at all: `change repair-identity --expect-version`, `status --records` `records[].version`, `context.finalize` `candidates[].pr.version`, the `adr.record` producing-change key `change.version`, and ten refusal codes that carry "version". Row 43 also named `adr.record`, which carries no target. ADR-0129 was amended in place at this grooming: rows 40, 41, 43 and 44 were corrected, and rows 40a, 41a, 41b, 43a and 44a were added.
- **"revision" already names commit ids** in about a dozen keys: `committed_revision` on every mutation result, `metadata_revision`, `*_branch_revision`, and the run tracker's `revision` / `bound_revision`. Settled: the word keeps one sense, "the exact id of a pinned state", and the glossary defines it once (Decision 2).
- **The claim idempotency digest payload** (`claimDigestPayload` in `internal/app/change_claim.go`) hashes a `version` key, and its digests are committed as `Docket-Request-Digest` trailers on `docket`. ADR-0129 already keeps that payload unchanged.
- **No local store persists a record revision.** The only persisted `*_version` key outside the app layer is `resolver_budget_version` (row 45, kept). So, unlike 0471, there is no storage reset.
- **Request files are decoded strictly** (`DisallowUnknownFields`), so an old `version` key is refused as an unknown field, never silently ignored.
- **Other `--version` flags name software versions and stay:** `cmd/releasepkg --version`, the release downloader's `--version` (exercised by `tests/test_release_downloader_refusals.sh`), and the `docket version` command.

## Decisions (settled at grooming)

1. **Scope is ADR-0129 family (b) as amended: rows 39–44, 40a, 41a, 41b, 43a and 44a, as one hard cut with no aliases.** Row 45 stays. The ADR is the naming authority.
2. **"revision" keeps one meaning across docket: the exact id of a pinned state.** A record revision is a git blob id. A PR revision is a hash over the PR's mutable snapshot. The existing `*_revision` keys are commit ids and are not renamed. Considered and rejected: `etag` (collision-free, but web jargon and a deviation from the ADR); renaming the commit keys to `*_commit` (grows the family by about a dozen keys the ADR never assigned); `blob` (wrong for the PR snapshot hash).
3. **Every gap the trace found is renamed in this change.** ADR-0129 records them through an in-place amendment, authorized by the human at grooming, rather than an Update note.
4. **Kept:**
   - software and format versions (`protocol_version`, `schema_version`, `capability_version`, `format_version`, `go_version`, `product_version`);
   - the `version` operation, `docket version`, and the `capabilities` result's `binary.version`;
   - the claim digest payload's `version` key;
   - `resolver_budget_version`;
   - the release tools' `--version`;
   - the commit-id `*_revision` keys.
5. **No storage reset and no migration.** Nothing local persists a record revision.

## Design

### 1. Wire renames

Apply the rows to every wire surface. Old spellings are refused outright, and nothing accepts both.

| Row | Old | New |
|---|---|---|
| 40 | `--version` on `change.{attach-plan, attach-results, claim, halt, mark-implemented, reclaim, refresh-claim, resume-halted}`, `finalize.{block, clear-block, merge, rebase, retarget-children}`, `workspace.prepare` | `--revision` |
| 40a | `change repair-identity --expect-version` | `--expect-revision` |
| 41 | request `version` on `change.{block, defer, groom, kill, reconcile, revive, unblock}`, `learning.update` and the flag-built requests; `status` `changes[].version`; `context.implementation` `change.version` / `spec.version`; `context.finalize` `candidates[].version` | `revision` |
| 41a | `status --records` `records[].version` | `revision` |
| 41b | `context.finalize` `candidates[].pr.version` | `pr.revision` |
| 42 | `spec_version` (`change.groom`) | `spec_revision` |
| 43 | `target.version` (`adr.supersede`, `adr.reverse`) | `target.revision` |
| 43a | `change.version` (`adr.record`; `successor.change.version` on supersede/reverse) | `change.revision` |
| 44 | `pr_version` (`finalize.merge` result `merge.pr_version`; `finalize.retarget-children` authorized children) | `pr_revision` |
| 44a | `version-mismatch`, `version-drift`, `spec-version-mismatch`, `reclaim-version-missing`, `empty-version`, `empty-change-version`, `empty-target-version`, `empty-spec_version`, `invalid-spec_version`, `empty-child_pr_version` | the same spellings with `revision` in place of `version` |

The `docket status` remedy string for `change repair-identity` (`internal/app/status.go`) moves to `--expect-revision`. Flag help text ("exact record blob object `id` …") moves to the concept name.

**Go identifiers follow their row's term**, derived by a whole-repo grep and never from a hand list. For example:

- request and result `Version` fields → `Revision`; `SpecVersion` → `SpecRevision`; `PRVersion` → `PRRevision`;
- `FCEmptySpecVersion` → `FCEmptySpecRevision`; `ReasonImplementedVersionMismatch` → `ReasonImplementedRevisionMismatch`;
- in `internal/githubcli`: `PullRequest.Version` / `ExpectedVersion` / `computeVersion` → `Revision` / `ExpectedRevision` / `computeRevision`.

Software-version identifiers (`buildinfo.Version`, `release.Version`, `VersionResult`, the `*SchemaVersion` family, …) keep their names. The claim digest payload's Go field may be renamed, but its JSON tag stays `version`.

No struct today carries both a record `version` field and a `Revision` field. A renamed local variable can still meet an existing commit-meaning `revision` in the same function. Where it does, the commit one becomes `committedRevision`. Watch for inner-scope shadowing, which compiles silently.

**No file renames.** Every file named `version*.go` concerns software versions.

### 2. Prose, skills and docs

Sites known at grooming. This list is a starting point for the plan's grep, not the population:

- **Skills:**
  - `docket-groom-next`, `docket-auto-groom`, `docket-new-change`;
  - `docket-implement-next` and its `references/edge-paths.md`;
  - `docket-finalize-change` and its `references/gate-failure.md`;
  - `docket-convention/references/terminal-close-out.md`;
  - the embedded copies under `internal/assets/embedded/tree/`, which are regenerated, never hand-edited.
- **Docs:**
  - `docs/reference/glossary.md` and `docs/guide/keeping-the-backlog-honest.md`;
  - `docs/reference/harness/validation.md` and `validation-runbook.md`;
  - the `docs/reference/harness/fixtures/nested-launch/` pages (sort out the point-in-time records first).
- **Agent instructions:** CLAUDE.md and AGENTS.md carry no record-revision spelling today. Re-check at build.

**Glossary.** The "Change version (`--version`)" and "Entity version" entries merge into one **Revision** entry that defines the word once (Decision 2). It covers:

- the record revision: a blob id, spelled `--revision`, `revision`, `spec_revision`, `target.revision` or `change.revision`;
- the PR revision: a snapshot hash, spelled `pr_revision` or `pr.revision`;
- the commit-id keys `committed_revision`, `*_branch_revision` and `metadata_revision`.

Update inbound links and the alphabetical index.

### 3. Retired-vocabulary seal

Append rows to the retired-vocabulary table in `internal/repoguard/retired_vocabulary_test.go` (ADR-0129 Decision 10), over the surfaces it already scans. Every failure names the replacement.

- **Compound tokens** match as tokens at executable sites, in Go string literals, and in struct tags: `--expect-version` / `expect-version`, `spec_version`, `pr_version`, and the ten row-44a codes. No kept name shares these spellings.
- **The bare `--version` flag (row 40) is retired only when it is bound to a `change`, `finalize` or `workspace` command.**
  - In markdown and shell, the binding is the nearest docket command noun or catalog op id in the same block.
  - In Go, it is a flag definition or lookup (`Flags().String("version"`, `GetString("version")`, `MarkFlagRequired("version")`) in those command builders in `internal/cli`.

  This binding is the inverse of row 12's: here the bound use is the retired one. It must not depend on the `Kept` / `scanKeptRows` machinery, which change 0477 removes. If 0477 has landed, reuse any generic helper that survived; otherwise add a small predicate of 0472's own.
- **The bare `version` JSON key (rows 41–43a) is sealed by walking the schema registry** (`internal/app/schema_registry.go`, the wire contract), not by grepping struct tags. No request or result key may be named `version` or end in `_version`, except a bounded kept set: the software and format versions of Decision 4, the `version` operation's own result key, and `capabilities` `binary.version`. The claim digest payload is not a registered shape, so it falls outside the walk. A failure names the key path and its replacement.

**Shape boundaries.** The seal must accept each of these, and each is a negative control:

- `docket version`;
- `capabilities` `binary.version`;
- `cmd/releasepkg --version`;
- the release downloader's `--version` and its test;
- the commit-id `*_revision` keys;
- `resolver_budget_version`;
- the claim digest payload's `version` key.

### 4. Landing

Landing is a human procedure. The results file's `**Human action:**` and the PR body say:

1. **Merge only when no dispatched implement-next or finalize run is in flight** (drain or cancel first). Their loaded skills send `--version`, which the new binary refuses.
2. **Run the post-merge binary rebuild immediately** (the AGENTS.md rule).
3. **Restart open coordinator sessions.** Their loaded skills name `--version`.
4. **Re-run `docket install` in every consumer repo.**
5. **On other machines, switch binaries only when no run is in flight there.** No storage cleanup is needed.

A skipped step fails loudly: an old flag is an unknown flag, and an old request key is an unknown field. Recovery is to re-dispatch with the new binary and skills.

Building 0472 itself is safe. Its build and finalize run on the old installed binary and skills, the suite runs from source, and no shell test calls the installed binary with a record `--version`.

Notes outside the repo (the human's saved agent memory names `workspace prepare --version` and `finalize clear-block --version`) are updated by the human's session after landing, not by the build.

## Testing

- **The full suite** through `build.test_command`. Read the budget report even when it is green.
- **Seal mutation tests.** The shape classes are: compound token, bound `--version` in markdown or shell, bound `--version` in Go, and schema-walk key. For each class, reintroduce one retired spelling and watch the seal go red naming the replacement, then restore it and watch the seal go green. Include a non-vacuity floor per class and every negative control from section 3.
- **Claim digest stability.** A fixed `(id, revision, run-context hash)` still hashes to its pre-rename digest value. Mutation: change the payload's JSON tag, and the test goes red. This extends `internal/app/claim_receipt_key_test.go`.
- **Old spellings are refused.** `--version` on a renamed command fails as an unknown flag. A request file carrying `version`, `spec_version`, `target.version`, `change.version` or `pr_version` fails as an unknown field.
- **Golden output.** `capabilities --json` and `schema --json` show only the new names, including `--expect-revision` and the row-44a codes wherever the schema's vocabularies list them.
- **Whole-repo grep.** Every remaining hit for a retired spelling is a kept name (Decision 4), a point-in-time record, or ADR-0129.

## Out of scope

- Aliases, a deprecation window or dual-spelling support (ADR-0129 Decision 2).
- Renaming the commit-id `*_revision` keys, software or format versions, `resolver_budget_version`, or the claim digest payload's `version` key.
- Renaming config keys, agent names or frontmatter fields.
- Editing point-in-time records: archived changes, results, specs and plans. ADR-0129 was amended in place at grooming. The build edits it only to record a newly found name, with the human's explicit authorization, as 0471 did for row 28a.
- Rows owned by changes 0473, 0474 and 0477, or by change 0469.

## Coordination

- **0477** (high priority, build-ready) and **0474** also append rows to the retired-vocabulary table and edit ADR-0129. Whichever lands later rebases an append-only table. 0477 removes the seal's `Kept` machinery, so 0472's row-40 binding must stand on its own (section 3).
- Neither blocks 0472. `depends_on` stays `[468]`, which is done.
