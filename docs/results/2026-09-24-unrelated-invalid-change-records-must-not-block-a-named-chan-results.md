<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0449 — Unrelated invalid change records must not block a named change's metadata writes or board](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0449-unrelated-invalid-change-records-must-not-block-a-named-chan.md)**
<!-- docket:backlink:end -->
# Unrelated invalid change records must not block a named change's metadata writes or board — Results

**Human action:** Assessment pending — review and gate results are not yet recorded.

## Outcome

Before this change, one broken change record anywhere in the repository (for example a file with unclosed frontmatter) stopped every metadata write for every other change: claim, reconcile, plan/results attachment, lifecycle writes, mark-implemented, halt/resume, finalize block/clear-block, final archive/closeout, and ADR writes all refused. The board renderer aborted on one bad record, and named reads fetched live branch facts for every stacked change, so one bad branch name elsewhere broke them too.

Now a named operation on change B validates a bounded set of subjects: B itself, the records B structurally needs (dependencies, stack ancestors and descendants), and every record the operation writes. Errors in those subjects still refuse. Errors in unrelated records are tolerated only when they are exactly the same pre-existing findings on byte-identical files; any new or changed error anywhere still refuses. Operations with no subject contract keep the old strict whole-repository validation, and an empty or unresolvable subject set falls back to strict.

The board now renders every usable record and lists unrenderable ones (including unparseable files, which used to silently vanish) in a repair notice. Named reads and mutations probe live branch facts only for B's own base and stack. Read-only health checks still report the whole repository.

## Verification performed

Every plan task was built test-first by a separate worker with focused runs through the gate driver, and each task mutation-tested its own guard. A final mutation sweep reverted each key mechanism in turn (before-gate, after-gate, board abort, whole-corpus branch probing, relevance check, finding-equality key, empty-scope handling). Every probe turned tests red. The one weak spot the sweep found (Task 5 refusal rows stayed green when relevance was deleted) was closed with a new test where B depends on a defective record the claim never writes. End-to-end tests drive B by id through the implementation flow (context, claim, reconcile, workspace, attach plan/results, publish, mark-implemented) and the finalize flow (block, clear-block, merge already landed, closeout, cleanup), with a broken record A present throughout. After every step they check that A's bytes are unchanged and that B's board row and the repair notice are present.

## Known issues and follow-ups

### Whole-repository status still fails on an unrelated invalid branch name

When some unrelated change records an invalid branch name, `docket status` (the whole-repository read) fails with an external error, because its branch-fact probe still covers every change. Named operations are no longer affected. This is confirmed from tests and is outside this change's scope, since the spec keeps status whole-repository. Suggested next action: a follow-up change making status report a per-record finding instead of failing the whole read.

### Finalize end-to-end test fakes the gate and GitHub

The finalize flow test enters the merge through the already-merged path with faked gate and GitHub seams. It does not start a real finalize gate. The real finalize gate is exercised elsewhere in the suite, but not together with a broken unrelated record.
