---
slug: environment
hook: "A RED suite in a build sandbox or an installed dev shell is a hypothesis, not a verdict — re-run it on the unmodified base."
topics: [testing, environment, ci]
changes: [34, 66, 311]
created: 2026-06-21
updated: 2026-08-14
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
- 2026-08-14 (#311, PR #207) — The mirror case: the merge gate's detached runner ran RED where the
  environmental difference was the *revealer*, not the cause. A umask-077 runner exposed a genuine
  installer defect (documented 0755/0644 targets landing 0700/0600 because the create-time mode
  argument is umask-masked), alongside one genuinely test-side failure. So the differential re-run
  cuts both ways: it separates false RED from real defect, and "it only fails in the sandbox" is
  never on its own grounds to wave a failure through.
