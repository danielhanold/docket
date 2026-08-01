<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0176 — docket-config.sh costs ~0.87s per invocation and dominates test_docket_config.sh](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-01-0176-docket-config-sh-costs-0-87s-per-invocation-and-dominates-te.md)**
<!-- docket:backlink:end -->

# docket-config.sh per-invocation cost — results

Change: #176 · Branch: feat/docket-config-sh-costs-0-87s-per-invocation-and-dominates-te · Plan: docs/superpowers/plans/2026-08-01-docket-config-per-invocation-cost.md · ADRs: none

## Findings

- Same hermetic local-origin fixture, 20 warmed runs: `origin/main` median midpoint was 0.485s
  (10th/11th samples 0.48s/0.49s); the feature branch median was 0.170s
  (0.17s/0.17s), a 2.85× reduction that exceeds the 2× acceptance target.
- The retained runtime-derived PATH-shim guard measured 13 spawned commands, under its 120-command
  ceiling. Restoring repeated external scans raised the count to 173 and reddened the guard;
  removing the nested-block boundary also reddened its corresponding characterization assertion.
- `bash tests/test_docket_config.sh` passed. The full aggregate equivalent
  `for t in tests/test_*.sh; do bash "$t"; done` passed all 73 test files.
- Independent whole-branch review found no Critical or Important issues. It noted that the planned
  `scripts/test-all.sh` entry point does not exist; the aggregate test command above was used
  instead.
