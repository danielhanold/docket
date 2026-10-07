<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0500 — committed-ignore-invalid remedies print the paste-ready managed block](../../changes/archive/2026-10-04-0500-committed-ignore-invalid-hint-for-an-absent-gitignore-names.md)**
<!-- docket:backlink:end -->

# committed-ignore-invalid remedies print the paste-ready managed block — design

## Problem

`docket repository check` reports `committed-ignore-invalid` when the committed `.gitignore` on the integration branch lacks docket's managed ignore block. `committedIgnoreFinding` (`internal/reposetup/health.go`) renders one remedy per `IgnoreDefect`. Two things are wrong with those remedies:

1. **The missing-file remedy names a command that does nothing.** For `IgnoreDefectFileAbsent` it says "Restore the managed block (e.g. re-run `docket repository migrate`, or add it by hand from the canonical block)…". Since change 0496, `migrate` is a no-op on a migrated repository and points at `docket repository repair`, which does not write `.gitignore` either.
2. **No by-hand remedy shows what to write.** Every variant tells the human to write "the managed block" or "the canonical representation", but none prints it. The start marker has an exact spelling (`# docket:start (managed by docket — do not hand-edit)`, em dash included) that nobody can guess. It is documented only in docket's own source and ADR-0020.

The 0496 results file lists problem 1 as a follow-up. Problem 2 is the sibling gap the groom bundled.

## What the trace found

1. **Only the `IgnoreDefectFileAbsent` remedy names `migrate`.** The five sibling cases (`BlockAbsent`, `LegacyOnly`, `MalformedMarkers`, `MissingEntries`, `NonCanonical`) and the no-detail `default` case already use the hand path: fix it, then review, commit, push.
2. **No command writes the block on a migrated repository.** `migrate` is a no-op there (0496). `repository init` re-runs converge and rewrite the block through `ensureManagedGitignore`, but `publishOrAdoptMetadataRoot` refuses a migration-seeded lineage (`initEquivalent`). `repository repair` writes only the metadata branch, and `.gitignore` lives in the integration working tree, which `init` deliberately leaves unstaged for human review. Printing the block is the cheapest remedy that works.
3. **The by-hand remedy holds in every state the finding fires in** (healthy, needs-review, partial). On a half-migrated repository, `migrate`'s prune also merges the block through `mergedGitignore` → `EnsureGitignoreBlock`, which returns `changed=false` over a block that is already canonical. So a hand-added block and a later `migrate` resume never conflict, and the remedy needs no branching on state (learning `printed-remedy-state-validity`).
4. **0496's guard never saw this finding.** `TestRepairableFamilyRemediesNameRepairNotMigrate` (`internal/app/repair_remedy_family_test.go`) keys its population on `Finding.Repairable != nil`. `committed-ignore-invalid` carries a nil `Repairable`, so it was outside the guard by construction.
5. **No test pins any `committed-ignore-invalid` remedy text today**, so the rewrite breaks no existing assertion.
6. **How the remedy renders:**
   - `repository check`'s human output goes through `appendFindingBlock` (`internal/app/config_diagnostics.go`), which writes `\n  remedy: <remedy>` verbatim. Lines after the first land at column 0. That is the behavior a paste-ready block needs: in a `.gitignore`, leading whitespace is part of the pattern, so an indented line would never match.
   - `status`'s `writeFinding` indents continuation lines four spaces. `status` lifts repository findings only on the legacy refusal (`refusalFindings`), where the only remedy is `migrate`, which writes the block itself.
   - JSON carries the remedy string as is.
7. **The canonical bytes already have one source.** `GitignoreBlock()` (`internal/reposetup/gitignore.go`) returns a fresh copy of the block `init` writes and `check` validates against, and its byte-parity with the Bash emitter is already guarded.

## Decision

### Code — `internal/reposetup/health.go`

- Add one unexported helper that appends the canonical block to a remedy instruction: the instruction, a newline, then `GitignoreBlock()` with its trailing newline trimmed. The block bytes come **only** from `GitignoreBlock()`, never from a second literal.
- Every remedy `committedIgnoreFinding` builds goes through that helper. Each case keeps its own instruction, ending in a colon that introduces the block:

| Defect | Instruction before the block (wording may be polished) |
|---|---|
| `IgnoreDefectFileAbsent` | Add a .gitignore containing exactly these lines, then review, commit, and push it: |
| `IgnoreDefectBlockAbsent` | Append exactly these lines to the .gitignore, then review, commit, and push it: |
| `IgnoreDefectLegacyOnly` | Replace the legacy managed block with exactly these lines, then review, commit, and push the corrected .gitignore: |
| `IgnoreDefectMalformedMarkers` | Inspect and correct the reported marker structure by hand first; the finished managed block must be exactly these lines. Then review, commit, and push the corrected .gitignore: |
| `IgnoreDefectMissingEntries` | Restore the missing entries (`<list>`) so the managed block is exactly these lines, then review, commit, and push the corrected .gitignore: |
| `IgnoreDefectNonCanonical` | Rewrite the managed block to exactly these lines, then review, commit, and push the corrected .gitignore: |
| `default` (no preserved detail) | Restore the managed block as exactly these lines, then review, commit, and push the corrected .gitignore: |

  These constraints are binding: no remedy names `repository migrate`, and the block starts on its own line after the instruction. The `MissingEntries` remedy keeps its explicit list of missing entries.
- **Unchanged:**
  - The finding messages.
  - The `committed-ignore-unverified` finding: the blob was unreadable, so there is nothing to tell the human to write.
  - `migrate`, `init`, `repair`, and both renderers.
  - The other `migrate` remedies in `health.go`, which stay valid.

### Tests

1. **Variant guard** (`internal/reposetup/health_test.go`). For every `IgnoreDefect` value `committedIgnoreFinding` can receive, including the zero value that reaches `default`, the finding's remedy must:
   - (a) end with `GitignoreBlock()`, trailing newline trimmed;
   - (b) carry `"\n"` immediately before the block's start marker;
   - (c) never contain `repository migrate`.

   Derive the population; don't hand-list it. Add a terminal unexported sentinel to the `IgnoreDefect` enum and iterate up to it, so a defect added later is covered automatically. Add a population floor (at least the eight values that exist today, `IgnoreDefectNone` through `IgnoreDefectUnreadable`) so a broken loop cannot pass vacuously.
2. **Render guard** (`internal/app`). `RepositoryCheckResult.HumanText` for a result carrying a real `committed-ignore-invalid` finding (built through `reposetup.EvaluateHealth` from facts carrying an `IgnoreDefectFileAbsent` detail, not a hand-built `Finding`) contains `"\n"` followed by the trimmed `GitignoreBlock()` verbatim. That proves every block line prints at column 0.
3. **Mutation probes.** Each must make a test fail:
   - restore the old `IgnoreDefectFileAbsent` remedy text;
   - drop the helper call from any one case;
   - make the helper indent the block lines;
   - make the helper join the instruction and block with a space instead of a newline.

### Verification

Run the whole suite at the build gate through `build.test_command`.

## Out of scope

- Teaching `repository repair` or `repository init` to write `.gitignore` on a migrated repository.
- The other `migrate` remedies in `health.go` (`local-metadata-missing`, `docket-worktree-missing`, the legacy and half-migrated findings). They remain valid.
- `committed-ignore-unverified`.
- `status`'s four-space continuation indent on the legacy refusal path.
- Documenting the managed block in user-facing docs.
