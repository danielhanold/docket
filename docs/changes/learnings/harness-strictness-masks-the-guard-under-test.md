---
slug: harness-strictness-masks-the-guard-under-test
hook: "A fixture stricter than production can enforce the very property your new guard adds — delete the guard, the assert stays green, and the vacuity hides inside the stricter shell."
topics: [testing, guards, mutation, shell-portability]
changes: [286]
created: 2026-08-10
updated: 2026-08-10
promotion_state: candidate
promoted_to:
---

## Apply
A mutation test answers one question: *with the guard removed, does the assert go red?* That answer
is only trustworthy if nothing **else** in the test environment enforces the same property. A
fixture harness is usually written stricter than the code it exercises — `set -euo pipefail` is the
house default — and each of those strictness flags is itself a guard. Where the flag and the new
code guard overlap, the flag silently satisfies the assert and the mutation test scores green on a
guard that does nothing.

The concrete instance: adding `${VAR:?message}` to a bare `$VAR` read. Under `set -u`, bash already
aborts on the unset name, so the fixture reddens identically with and without the `:?` — the new
assert is vacuous, and looks proven. The fix is to make the fixture reproduce the **real** caller's
laxity: drop `-u` from the harness for that fixture specifically, and note beside it why.

Generalize it as a checklist question at every mutation test: **what in this harness, other than the
code under test, could make this assert pass?** Shell flags (`-u`, `-e`, `-o pipefail`), a `trap`, a
strict linter in the runner, a fixture directory whose name contains the expected string, a
pre-seeded env export. If any of them subsumes the property, the mutation test is measuring the
harness.

The mirror hazard is why the guard mattered at all: bash arithmetic turns an unset name into `0`,
and where `0` is a **legal** configured value the missing export is indistinguishable from a valid
config. An unguarded read that degrades into a legal setting is worse than one that crashes.

## War story
- 2026-08-10 (#286, PR #192) — the canonical `gate-run --observe` poll loop read
  `GATE_OBSERVATION_BUDGET` bare while guarding its sibling with `${DOCKET_SCRIPTS_DIR:?…}`. A
  missing export therefore read as a budget of `0` — legal, buying exactly one observation — and
  would have halted a healthy build 30 minutes early. The repair added `${GATE_OBSERVATION_BUDGET:?…}`;
  the first fixture written for it passed with the `:?` deleted, because the harness ran under
  `set -u` and bash aborted on the unset name by itself. The fixture now deliberately drops `-u`.
