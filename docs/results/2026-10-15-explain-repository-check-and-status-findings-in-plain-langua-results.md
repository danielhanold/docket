<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0553 — Explain repository check and status findings in plain language](../changes/active/0553-explain-repository-check-and-status-findings-in-plain-langua.md)**
<!-- docket:backlink:end -->

# Explain repository check and status findings in plain language — Results

**Human action:** Needed. The run halted because the machine's disk filled up (100% used, 1.4 GiB free) during the post-fix full-suite gate. Free some disk space, then resume the change.

## Outcome

Build complete at a3ee5f9b7 (10 plan tasks, all green on the first full-suite run). Every record-level finding from `docket repository check` and `docket status` now carries a plain-language message, an "is it harmful" impact line, and a remedy. All 10 review findings were fixed in-branch (17da9b7e9, 43a703d44, 9ca351f34, 84439f2f3), but the post-fix suite gate could not certify head 84439f2f3: its run failed with "no space left on device" across many test jobs. That is an environment failure, not a code regression, so the fixes were not reverted.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) green at a3ee5f9b7 on the first attempt. Budget report showed BUDGET WATCH and PARALLEL-SENSITIVE screening lines only, with no SERIAL CONFIRMED OVER BUDGET.
- Whole-branch review (deep tier, bumped from standard for a 3019-line diff): 10 findings (0 blocker, 3 important, 7 minor). Dispositions: finding 1 fixed in 17da9b7e9, 2 in 43a703d44, 3 in 9ca351f34, and 4-10 in 84439f2f3 (one batched commit). The post-fix gate at 84439f2f3 (drive 9c84c4ac) ended FAILED, and every failure in its log is "no space left on device". Findings returned:
  1. important: `docket status` shows document-parse failures in raw form, with no impact and no remedy (`parseFinding`).
  2. important: `findingsToStatus` attaches hand-edit remedies to findings from mutation refusals, which describe records that were never written.
  3. important: record remedies in check name `.docket` and `docket repository repair` even when the setup state is not healthy and repair would refuse.
  4. minor: `HandEditThenRepair` glues "in `<file>`" onto the end of the instruction; the empty-rel case leaves "it" with nothing to refer to.
  5. minor: `PlainDocumentError` stacks two parentheticals and repeats the name.
  6. minor: the type-invalid text describes the slug rule, not the type rule.
  7. minor: placeholder values are quoted as if they were stored (`missing` status, `id: 0`).
  8. minor: the priority-unknown remedy doesn't name the groom revise priority edit for proposed changes.
  9. minor: a family-test fixture comment is misplaced.
  10. minor: the empty-slug catalog fixture shows an unrealistic path, `0263-.md`.
