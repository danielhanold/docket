<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0283 — Slim AGENTS.md to an effective, lean always-in-context file](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0283-slim-agents-md-to-an-effective-claude-md.md)**
<!-- docket:backlink:end -->
# Slim AGENTS.md to an effective, lean always-in-context file — Results

## Outcome

The unmanaged authored prefix of root `AGENTS.md` (everything above the managed
`docket:dispatch` block) was rewritten to a leaner form that preserves every operational
obligation while removing three obsolete pointers: the automated-learnings-harvest framing,
`mint-stub.sh`, and `tests/test_comment_anchor_style.sh` (the last retargeted to the current
Go symbol `TestCommentAnchorStyle` in `internal/repoguard/anchors_test.go`).

- Managed `docket:dispatch` block is byte-for-byte identical to the base commit (`cmp`-confirmed
  pre- and post-edit); it remains the last section, immediately after the final unmanaged line.
- `CLAUDE.md` is unchanged and still a symlink to `AGENTS.md`.
- The branch diff is `AGENTS.md` plus the plan artifact only — no Go, test, or config files changed.
- Measurements: full file 144 lines / 1603 words → 129 / 1376; unmanaged prefix 107 / 1192 → 92 / 965
  (~19% word reduction). The spec's ~499-word draft target predated the current, richer rebuild
  section (post-0346 sync/ancestry/identity contract + the 0392 schema-tolerance bullet), which is
  pinned verbatim by `TestFinalizeRebuildAgentsRule`; per the spec, meaning outranks the count and
  no obligation was dropped to hit a number.

## Human testing

### Confirm the slimmed rules still convey every obligation to a fresh reader

Automated guards pin only a subset of AGENTS.md's prose — the eight rebuild-section clauses
(`TestFinalizeRebuildAgentsRule`), the managed-block markers/budget (`TestDispatchBlockBudget`),
the capability-surface ban (`TestCapabilitySurface`), and the filename-plus-line-number
cross-reference form (`TestCommentAnchorStyle`). The shell rules, the frontmatter scalar-quoting
trigger list, the guard-discipline rules, and the three budget-report token meanings rest on
authoring discipline and human review, not on a test.

1. Read the final `AGENTS.md` prefix top to bottom against the spec's rule-by-rule disposition table.
   Expected: each of the five shell rules (with its operand example and the `git mv` / beside-destination
   `mktemp` exceptions), the full scalar-quoting trigger list plus the scalar-vs-flow-collection
   distinction, the guard mutation/shape/whole-repo-grep rules, the whole-suite gate reading both
   config keys independently, and all three budget tokens with their distinct meanings each read as
   the same obligation the base file carried — only rationale/incident narrative is shorter.

## Verification performed

- Full configured build gate (`go run ./cmd/docket development test`) driven through the native gate
  driver from the feature checkout: green at head `75b0c234` (`SUITE files=43 passed=43 failed=0`).
- Targeted `internal/repoguard` guards run green on both the unedited base tree and the edited tree
  (`TestFinalizeRebuildAgentsRule`, `TestFinalizeRebuildContract`, `TestDispatchBlockBudget`,
  `TestCapabilitySurface`, `TestCommentAnchorStyle`); the full `internal/repoguard` package is green.
- Mutation probe: temporarily altering the pinned clause `binary rebuild incomplete` reddened
  `TestFinalizeRebuildAgentsRule` with the intended binding error, and restoring it returned the
  guard to green — the guard still bites the rewritten text.
- Managed-block byte identity and the `CLAUDE.md` symlink were confirmed unchanged.
- Independent whole-branch deep review: 0 findings; every operational obligation confirmed preserved
  and all three obsolete pointers confirmed removed.

## Findings and limitations

### Parallel-load flake in an unrelated gate-driver integration test

The first full-suite run went red solely on `TestIntegrationTerminalConsumedFromFreshProcess`
(`internal/gatedrive/integration_takeover_test.go`) with `first-process terminal HALTED (cause
"observation-unreadable"), want PASSED`. The test passes in isolation (`ok 0.706s`) and did not recur
on the certifying full-suite run. The HALT originates at `internal/gatedrive/driver.go` when a
per-slice on-disk observation (`internal/process/observe.go`) is transiently unreadable under heavy
parallel `-race` I/O — a fail-closed path, not a code defect. This branch changed no Go code, so the
flake is latent in `main`, independent of this change.

## Follow-ups

### Harden the gate-driver integration test against parallel-load observation flakiness

`TestIntegrationTerminalConsumedFromFreshProcess` can HALT with `observation-unreadable` under the
full parallel `-race` suite when a transient `Observe` read fails, rather than reaching its expected
terminal. A bounded retry of a transient observation read before HALT (or reduced parallel pressure
for this real-process integration test) would remove the flake. This is pre-existing behavior in
`main`, outside this docs-only change; no existing change is known to cover it — left for human triage.
