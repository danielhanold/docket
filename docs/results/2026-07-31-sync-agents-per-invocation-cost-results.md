<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0175 — sync-agents.sh costs ~5.5s per invocation and dominates the test suite](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0175-sync-agents-per-invocation-cost.md)**
<!-- docket:backlink:end -->

# sync-agents.sh per-invocation cost — results
Change: #0175 · Branch: feat/sync-agents-per-invocation-cost · PR: pending · Plan: docs/superpowers/plans/2026-07-31-sync-agents-per-invocation-cost.md · ADRs: none

## Verify (human)

No manual verification is required. The retained parser-command guard, focused harness suites, and
the complete configured Bash suite cover the change.

## Findings

### 1. Parser subprocesses, not fixture construction, dominated each generation pass

The same real no-argument generation fixture measured **1.82s median on `origin/main`** and
**0.29s median on the optimized branch**, a roughly **6.3x improvement**. The retained counting
guard now observes **38** `awk`/`grep`/`sed`/`head` calls (`awk` 22, `grep` 16, `sed` 0, `head` 0),
comfortably below its `<400` ceiling.

The base trace used during implementation counted 788 calls through the same four-command shim.
The plan's 2,427 estimate came from an earlier, broader trace and should not be compared directly
to the retained fixture. The guard's mutation test is the important invariant: bypassing the cache
raised the retained fixture to **597 calls** and reddened the ceiling.

### 2. The first cache pass was correct but not sufficient

The initial layer-body cache left **644–667** counted parser calls because `hd_validate` still
reparsed every live config for every validation query. A second optimization added one synchronous
validation sidecar per file on Bash 4+, while retaining the existing Bash 3.2 path. That brought
the count to 38 without changing the accepted/rejected YAML subset.

Focused test times improved on the same machine:

| suite | before | after | reduction |
|---|---:|---:|---:|
| `tests/test_sync_agents.sh` | 197.8s | 103.43s | ~48% |
| `tests/test_sync_agents_codex.sh` | 66.8s | 27.72s | ~59% |
| `tests/test_sync_agents_cursor.sh` | 14.3s | 5.87s | ~59% |

All three focused suites pass under the configured Bash; the main suite also passes under Bash
3.2 with the same 543 assertions. The full repository suite passed after the final review repairs.

### 3. Review caught three fail-open or parity weaknesses in the new evidence

Whole-branch review found that the optimized validator stripped a top-level header comment before
checking it, while the Bash 3.2 fallback checked the raw header; that the new argument tests looked
for writes under the sandbox instead of the configured harness root; and that the performance
guard did not independently assert the generator's exit status. All three were repaired and
mutation-tested: bypassing either argument return reddens the correct-root no-write assertion, and
a forced generator exit 42 reddens the exit-status assertion even though its zero call count still
satisfies the ceiling.

The reviewer initially proposed adding the quote leg from ADR-0065 to only the optimized validator.
That finding was withdrawn on follow-up: `hd_validate` itself still has that known gap, and changing
only Bash 4+ would create version-dependent validation. The uniform fix is already tracked by
change **#0180**.

## Follow-ups

- **#0180** already tracks applying ADR-0065's quote-aware bare-scalar validation uniformly to the
  shared validator and every runtime path; no duplicate change was created here.
- **#0176** covers the analogous per-invocation cost in `docket-config.sh`.
- **#0179** covers extraction of the shared config parser rather than duplicating future parser
  changes across scripts.
