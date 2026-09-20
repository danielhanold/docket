<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0418 — Surface every unmet repository health postcondition](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0418-surface-every-unmet-repository-health-postcondition.md)**
<!-- docket:backlink:end -->
# Surface every unmet repository health postcondition — Results

## Outcome

`docket repository check` now reports every applicable unmet repository-health condition in one
run, instead of collapsing them behind the generic `postconditions-unmet` fallback or masking
independent failures behind an earlier specific diagnosis. Classification precedence, reason
tokens, state names, exit codes (0/1/2), and read-only behavior are unchanged — the supplemental
findings never alter the selected state or exit.

The implementation follows the spec's design:

- **One evaluation of the healthy conjunction.** `internal/reposetup/healthconditions.go` adds
  `UnmetHealthConditions(Facts) []HealthCondition` (17 `Cond*` identifiers in fixed order).
  `Classify` now selects healthy exactly when that helper returns empty, replacing its inline
  17-conjunct predicate, so the diagnostic list can never drift from the healthy decision.
- **Ignore detail preserved at the read boundary.** `gitignore.go` adds pure explanatory helpers
  (`IgnoreDetail`/`IgnoreDefect*`, `GitignoreEntries`, `ExplainGitignoreBlock`,
  `CommittedIgnoreOutcome`) that classify why a committed `.gitignore` managed block was rejected
  (file/block absent, legacy-only, malformed markers, missing canonical entries in canonical
  order, non-canonical, unreadable). `ValidGitignoreBlock` remains the sole acceptance authority —
  any input it accepts explains as `None`, so the helpers can never tighten validity.
- **Supplemental per-condition findings.** `EvaluateHealth` supplements the existing reason-based
  findings with per-condition findings, gated by applicability (only when remote metadata is proven
  present), prerequisite grouping (a dependent condition behind an unresolved prerequisite is
  represented by the prerequisite's own finding, not a fabricated cascade), and dedup by the
  condition explained. Unknown evidence is a `warning` ("unverified"), a proven wrong state is an
  `error`; an unknown ownership proof is never called foreign, an unknown hook setting never called
  enabled, and an unreadable committed blob never called a missing file.
- **Service wiring.** `committedIgnorePresence` returns `(Presence, IgnoreDetail)` and
  `augmentCheckFacts` stores the detail; the findings ride the existing JSON serialization and
  shared human renderer with no envelope change.

The originally reported real-world regression — a committed `.gitignore` missing the
`.opencode/agents/docket-*.md` entry surfacing only as `postconditions-unmet` — is now named
explicitly (path, exact missing entry, and a review/commit/push remedy) and covered end to end by
an integration regression.

## Verification performed

- Full source-tree build gate (`go run ./cmd/docket development test`) driven through the native
  gate driver: **green**, 52/52 suite files passed (423 asserts). Build evidence recorded and
  re-established for the final head.
- TDD throughout: each of the five plan tasks and the one review fix landed RED-first, then GREEN;
  the equivalence guard (`TestClassifyHealthyIffNoUnmetConditions`), the supplemental-findings
  population steps, and the committed missing-entry integration regression were mutation-tested
  (population step removed → assert reddens → restored → green).
- Integration regression `TestIntegrationRepoCheckMissingIgnoreEntryNamed` deletes only the
  `.opencode` entry from the canonical committed block, commits/pushes it, leaves the working tree
  matching, and asserts human and JSON output name `.gitignore`, the exact entry, and its remedy
  while state and exit are unchanged; proven non-vacuous by reverting the wiring and observing the
  failure.
- Healthy baseline preserved: `TestIntegrationRepoCheckHealthyBaselineHasNoSupplementalFindings`
  confirms no supplemental finding leaks into a healthy report (state healthy, 0 findings, exit 0).
- Whole-branch deep review confirmed the classifier is mirrored 1:1, dedup/prerequisite grouping is
  correct, unknown is never rendered as proven-wrong, `ValidGitignoreBlock` acceptance is unchanged,
  and the generic finding never stands alone.

## Findings and limitations

### Live-surface unverified remedy alignment (review finding, fixed)

The deep review's one finding (minor): the `live-surface-unverified` remedy printed a generic
"ensure the remote is reachable" message, less precise than its sibling committed-tree unverified
remedies even though LiveSurface is proven from the same integration commit tree. Fixed in-branch
(commit `4e9ece9c`) by aligning the wording to the committed-tree phrasing ("Restore readable
committed evidence (fetch the integration objects), then re-run `docket repository check`."), with a
focused test pinning the corrected remedy. No blocker or important findings.

### Build-suite budget watch (screening only)

The build gate reported `BUDGET WATCH:` lines for several integration/race/toolchain suite files
(parallel-overrun streak 1/5 under `-j11`); no `SERIAL CONFIRMED OVER BUDGET:` breach was reported.
Per `tests/README.md`, a watch line at streak 1/5 is a machine-dependent screening finding, not an
authoritative breach, and requires no action here.
