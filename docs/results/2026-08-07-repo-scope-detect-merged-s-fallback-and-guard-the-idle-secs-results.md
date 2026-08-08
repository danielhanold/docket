<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0250 — Repo-scope detect-merged's fallback and guard the idle-secs duplication](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-08-0250-repo-scope-detect-merged-s-fallback-and-guard-the-idle-secs.md)**
<!-- docket:backlink:end -->

# Repo-scope detect_merged's fallback and guard the idle-secs duplication — results

Change: #0250 · Branch: feat/repo-scope-detect-merged-s-fallback-and-guard-the-idle-secs · PR: (opened at close of this run) · Plan: docs/superpowers/plans/2026-08-07-repo-scope-detect-merged-fallback-and-guard-idle-secs.md · ADRs: 0072 (cited, not modified)

## Verify (human)

No genuinely manual checks. Both parts are covered by the suite: Part 1 by the argv-recording `gh`
stub (a real `gh` is never invoked), Part 2 by pure textual extraction over two files on disk. The
green suite plus the PR diff is the whole verification.

## Findings

**The same grep-engine hazard appeared twice in one change, at two altitudes.** The PATH `grep` on
this machine is ugrep 7.5.0, and it treats a mid-pattern `$` as an end-of-line anchor where GNU grep
and `/usr/bin/grep` treat it as a literal. Both the ADR-0072 guard's constants and its mutation
witness embed `$(( ... ))` in their match text, so this bites directly:

1. The **plan's own hand-run verification command** (`grep -c '^ABORTED_RUN_IDLE_SECS=$(( 5 \* 3600 ))'`)
   returned `0` against a file where the mutation had genuinely landed. The build worker caught it,
   re-confirmed with `grep -c -F`, and only then believed the red reading — the
   confirm-the-mutation-landed discipline is what kept a false negative from reading as a robust
   guard.
2. The **committed guard carried the same pattern shape**, and that survived the build (the suite was
   green) because the worker's own copy of the file happened to resolve a grep that matched. Review
   caught it by running the committed pattern under all three engines. Fixed in `b72a5931` by
   dropping to `grep -cF`.

The generalizable part: the repo's standing portability rule is written as "ugrep is *more*
permissive than `/usr/bin/grep`, so bugs hide behind local greens." This change is the mirror case —
ugrep being *stricter* on a metacharacter — and the existing rule's phrasing does not prompt anyone
to look for it. A pattern whose match text contains shell arithmetic (`$((`), a literal `$`, or any
regex metacharacter should use `-F` rather than being escaped, because escaping is what varies
between engines.

**A correspondence guard over two constants needs a consumption floor.** As first written, all four
of the guard's asserts were satisfied by a state where both constants are assigned once, agree by
value, and neither is read by anything: inlining a literal at the predicate site while leaving the
assignment behind would keep the guard green while the agreement it exists to protect was gone.
Closed in `b72a5931` with one `>= 2` occurrence assert per file — the scope-selection population
floor that a value-equality guard does not imply.

**Review outcome:** 3 findings, 0 blocker / 0 important / 3 minor, all fixed in-branch in one
batched `economy` commit (`b72a5931`). Full disposition table is in the PR body.

## Follow-ups

None. Nothing surfaced that clears the auto-capture bar — every finding was about this branch's own
diff, and both spec-level narrowings (no shared-helper refactor, no predicate-shape guard) remain
deliberate triage decisions rather than deferred work.
