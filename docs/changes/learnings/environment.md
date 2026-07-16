---
slug: environment
hook: "A RED suite in a build sandbox or an installed dev shell is a hypothesis, not a verdict — re-run it on the unmodified base."
topics: [testing, environment, ci]
changes: [34, 66]
created: 2026-06-21
updated: 2026-07-13
promotion_state: retained
promoted_to:
---

## Apply
A RED suite in a build sandbox, or in a dev shell that has the feature installed, is a hypothesis,
not a verdict — before calling it a regression OR waving it through, re-run the identical suite on the
unmodified base (or under `env -u VAR`), byte-compare the failing sets, record the differential in the
results file, and let the merge gate's clean-env run confirm. Author fail-loud tests to `env -u VAR`
their own sub-shells so an installed shell can't false-RED them.

## War story
- 2026-06-21 / 2026-07-13 (#34 PR #45; #66 PR #73 — merged, one environment family) — Twice a suite
  ran RED where the failure was NOT a regression: (a) an ambient `DOCKET_SCRIPTS_DIR` export in the
  dev shell (written there by that very change's `install.sh`) was inherited by the test's sub-shells
  and masked their `${VAR:?}` fail-loud assertions; (b) a build sandbox failed 5 tests on environment
  facts (`origin/HEAD` unresolvable behind a proxied remote, a umask-dependent file mode, a timeout).
  Both were proven environment-bound by re-running the identical suite against unmodified `origin/main`
  and byte-comparing the failing sets.
