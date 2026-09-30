<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0472 — Rename change version to revision (--version → --revision)](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0472-rename-change-version-to-revision-version-revision.md)**
<!-- docket:backlink:end -->
# Rename change version to revision (--version → --revision) — Results

**Human action:** Needed at landing. Merge only when no dispatched implement-next or finalize run is in flight, then rebuild the binary right away, restart open coordinator sessions, and re-run `docket install` in every consumer repo. The old `--version` flag and `version` request key are hard-cut, so an older loaded skill fails loudly until it is refreshed.

## Outcome

Every docket operation pins the exact record it read, so that a concurrent edit is refused rather than overwritten. That pin used to be spelled `--version` / `version`, which read like a software version and clashed with `docket version`. It is now **revision** everywhere, as ADR-0129 family (b) requires:

- **Flags.** The 14 mutating operations take `--revision`, and `change repair-identity` takes `--expect-revision`. The old flags are unknown flags.
- **Keys.**
  - Request and read `version` is now `revision`. This includes `status --records` and `context.finalize`'s `pr.revision`.
  - `spec_version` is now `spec_revision`, and `pr_version` is now `pr_revision`.
  - The ADR requests' `target.version` and `change.version` are now `.revision`.
  - Old keys are refused as unknown fields.
- **Codes.** The ten refusal codes that said "version" now say "revision".
- **Glossary.** The glossary has one Revision entry: a record revision is a blob id and a PR revision is a snapshot hash. The existing commit-id `*_revision` keys are listed in the same entry and were not renamed.
- **Everything else that names these terms moved with them:** skills (and their embedded copies), docs, Go identifiers and tests.
- **Kept on purpose:**
  - software and format versions, `docket version`, and `capabilities` `binary.version`;
  - `resolver_budget_version`;
  - the release tools' `--version`;
  - the claim digest payload's `version` JSON key. A test pins the digest values, so existing `Docket-Request-Digest` trailers stay valid.
- **Seal.** The retired-vocabulary seal now carries the retired spellings:
  - compound tokens;
  - bound `--version` in markdown or shell, next to a change, finalize or workspace op reference;
  - `Flags().…("version")` in `internal/cli`;
  - a schema-registry walk that refuses any `version` / `*_version` key outside an exact kept set.

  Each class was mutation-tested.

Two stale skill shapes were fixed along the way. `docket-implement-next` gave `change.reconcile` `--id` / `--version` flags, but that operation takes only `--input`. `docket-finalize-change` spelled the retarget child list with Go field names instead of wire keys.

## Human actions and testing

### Important — Landing procedure

This matters because skills already loaded in a running session still send `--version`, and the new binary refuses it. If you skip a step, the result is a loud unknown-flag or unknown-field failure, not silent corruption. To recover, re-dispatch the run with the new binary and skills.

1. Confirm that no implement-next or finalize run is in flight, then merge the PR.
   Expected: the PR lands on `main`.
2. Run the post-merge binary rebuild from CLAUDE.md: `repository.sync-integration`, then `development.install --source /Users/homer/dev/docket`.
   Expected: `docket version` reports the merged `main` HEAD.
3. Restart open coordinator sessions and re-run `docket install` in each consumer repo. On other machines, switch binaries only when no run is in flight there.
   Expected: `docket capabilities --json | jq '.commands[]|select(.id=="change.claim")'` shows `--revision`.
4. Update saved agent memory notes that name `workspace prepare --version` or `finalize clear-block --version` so they say `--revision`.

## Verification performed

- Each plan task ran its focused tests through the gate driver:
  - claim-digest stability, with a mutation;
  - schema key and code goldens;
  - refusal of the retired request keys;
  - the CLI hard-cut and wiring tests, with mutations;
  - the embedded-copy and skill-budget guards;
  - the seal's seven subtests and its five mutations.
- The integration-tagged `internal/app` checks ran with a `-run` filter and `-timeout 25m`. The whole package under the integration tag exceeds go test's default 10-minute timeout.
- The full suite runs as the build gate over the final head. Its evidence is in the PR body.

## Known issues and follow-ups

### Integration-tagged `internal/app` exceeds the default go test timeout

`go test -tags integration ./internal/app/` with no `-timeout` flag panicked at 10 minutes during this build. The configured suite command is unaffected, but an ad hoc focused run must pass `-timeout` or use `-run`. This is confirmed and predates this change. The suggested next action is to triage whether the suite partition should cap this package's runtime.
