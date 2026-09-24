<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0449 — Unrelated invalid change records must not block a named change's metadata writes or board](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0449-unrelated-invalid-change-records-must-not-block-a-named-chan.md)**
<!-- docket:backlink:end -->
# Unrelated invalid change records must not block a named change's metadata writes or board — Results

**Human action:** Review the PR diff before merging; this change relaxes a repository-wide safety check, so one Important walkthrough below is recommended. No other action is required.

## Outcome

Before this change, one broken change record anywhere in the repository (for example a file with unclosed frontmatter) stopped every metadata write for every other change: claim, reconcile, plan/results attachment, lifecycle writes, mark-implemented, halt/resume, finalize block/clear-block, final archive/closeout, and ADR writes all refused. The board renderer aborted on one bad record, and named reads fetched live branch facts for every stacked change, so one bad branch name elsewhere broke them too.

Now a named operation on change B validates a bounded set of subjects: B itself, the records B structurally needs (dependencies, stack ancestors and descendants), and every record the operation writes. Errors in those subjects still refuse. Errors in unrelated records are tolerated only when they are exactly the same pre-existing findings on byte-identical files; any new or changed error anywhere still refuses. Operations with no subject contract keep the old strict whole-repository validation, and an empty or unresolvable subject set falls back to strict. Tolerated unrelated errors are still returned in the operation's findings, while the result stays `applied`.

B's own defects are also checked before the two external effects a named workflow performs: finalize's GitHub merge and PR publication now refuse with `record-invalid` before touching GitHub, while unrelated records cannot veto them.

The board and the ADR index now render every usable record and list unrenderable ones (including unparseable files, which used to vanish silently) in a repair notice. `repository check` accepts a notice-bearing board or index as current. Named reads and mutations probe live branch facts only for B's stack ancestors, which is what B's base resolution uses. Read-only health checks still report the whole repository.

The decision and its departure from change 0309's error-free-corpus rule are recorded as ADR-0127.

## Human actions and testing

### Important — Confirm a broken unrelated record no longer blocks a real claim

The automated tests use isolated repositories. This walkthrough checks the behavior against a real docket metadata branch with the installed binary, which the suite cannot do. If skipped, the remaining uncertainty is only whether the installed binary is built from this branch.

Prerequisites: a scratch clone of any docket-managed repository with a build-ready change you can claim (or a throwaway repository set up with `docket repository init`), and `docket` built from this branch (`go run ./cmd/docket development install --source <this worktree>`).

1. In the scratch clone's `.docket` worktree, create `docs/changes/active/0999-broken.md` containing only `---` followed by `id: 999` (no closing `---`), commit it and push to `docket`.
   Expected: `docket status --json` still runs and lists a finding for `0999-broken.md`.
2. Claim a different build-ready change by id: `docket change claim --id <id> --version <version from docket status --json> --json`.
   Expected: `"result":"applied"`, and the findings list includes the `0999-broken.md` error.
3. Open `BOARD.md` on the `docket` branch.
   Expected: the claimed change shows `in-progress`, and a repair notice names `docs/changes/active/0999-broken.md`.

Cleanup: delete the scratch clone (or remove `0999-broken.md` and reclaim/revert the claimed change with `docket change reclaim`).

## Verification performed

Every plan task was built test-first by a separate worker with focused runs through the gate driver, and each task mutation-tested its own guard. A final mutation sweep reverted each key mechanism in turn (before-gate, after-gate, board abort, whole-corpus branch probing, relevance check, finding-equality key, empty-scope handling); every probe turned tests red. End-to-end tests drive B by id through the implementation flow (context, claim, reconcile, workspace, attach plan/results, publish, mark-implemented) and the finalize flow (block, clear-block, merge including the already-landed path, closeout, cleanup) with a broken record A present throughout, checking after every step that A's bytes are unchanged and that B's board row and the repair notice are present. Production-loader progress rows cover the spec's other unrelated-defect shapes: a duplicate-id pair, a dependency cycle, an active record with status `done`, an invalid ADR, and unrelated records that depend on the unparseable A.

The first full-suite run failed on one test-infrastructure check (the new end-to-end tests had no shard runner); a repair added the shard and the second run passed. A whole-branch review returned seven findings (three important, four minor). All seven were fixed in-branch:

| Finding | Disposition |
|---|---|
| Unrelated dangling `depends_on`/`stacked_on` blocked every named operation | fixed (2c590273): an absent id resolves to no records; ambiguous ids resolve to every carrier; malformed refs still fail closed |
| B's own defects were not checked before merge/PR publication | fixed (c87bb811) |
| Tolerated unrelated errors were dropped from applied/no-op results | fixed (30a7165b) |
| Branch-fact probe wider than B's base needs | fixed (870372ba) |
| Absent/ambiguous explicit id still probed every branch | fixed (870372ba) |
| Unparseable ADR silently dropped from the ADR index | fixed (870372ba) |
| Most acceptance shapes not run through the production loader | fixed (870372ba) |

The fix loop's full-suite gate result is recorded in the PR's build-evidence block.

## Known issues and follow-ups

### Whole-repository status still fails on an unrelated invalid branch name

When some unrelated change records an invalid branch name, `docket status` (the whole-repository read) fails with an external error, because its branch-fact probe still covers every change. Named operations are no longer affected. Confirmed in tests; outside this change's scope because the spec keeps status whole-repository. Suggested next action: a follow-up change making status report a per-record finding instead of failing the whole read.

### A defective B merged out of band now reports record-invalid at merge

If B has its own validation error and its PR was already merged outside docket, finalize's merge step now reports `record-invalid` instead of `already-merged`. Closeout would refuse the same defect anyway, so this only surfaces it earlier. Workaround: fix B's record, then re-run finalize.

### Finalize end-to-end test fakes the gate and GitHub

The finalize flow test enters the merge through faked gate and GitHub seams and does not start a real finalize gate. The real finalize gate is exercised elsewhere in the suite, but not together with a broken unrelated record.

### Finalize skill prose does not list the new record-invalid reason

`docket-finalize-change`'s reason list is illustrative and does not mention `record-invalid`. Suggested next action: mention it in a later skill-prose pass.
