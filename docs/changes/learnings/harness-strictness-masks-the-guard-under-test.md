---
slug: harness-strictness-masks-the-guard-under-test
hook: "A fixture stricter than production can enforce the very property your new guard adds — delete the guard, the assert stays green, and the vacuity hides inside the stricter shell."
topics: [testing, guards, mutation, shell-portability]
changes: [286, 270]
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
- 2026-08-10 (#270, PR #193) — the same vacuity from an **ambient config layer** rather than a shell
  flag, and it exposes an asymmetry the checklist above needs. A new fixture fenced "the runner
  config grant is read at the main worktree, not at `--worktree`". It `unset XDG_CONFIG_HOME` at the
  top of the file but left `DOCKET_HARNESS_ROOT` unpinned, and the facade resolves
  `GLOBAL_CFG="${XDG_CONFIG_HOME:-${DOCKET_HARNESS_ROOT:-$HOME}/.config}/docket/config.yml"` — so the
  sandbox inherited the developer's **real** global config. On any machine whose global layer sets
  the documented `runners.codex.sandbox: danger-full-access` knob, spelled as the fixture spells it,
  that layer alone satisfies the grant assert: no main-worktree read happens, the mutation probe
  stays green, and the fence is decoration that appears to have survived its own probe. It was
  load-bearing here only by accident — this machine's global config carries a *different* runner key.
  Review caught it and the fix worker confirmed it empirically, building a fake `HOME` carrying the
  grant and showing the probe green before the fix and red after.
  **The asymmetry:** the suite runner sandboxes `HOME` per job, so the gate ran hermetic and green —
  the hole was observable only under the *direct* `bash tests/test_foo.sh` invocation that mutation
  probing itself uses. A guard whose non-vacuity depends on which mode you run it in is not yet a
  guard, and the mode where it is weakest is the one you verify it in. Add to the checklist question:
  **which env vars does this fixture unset, and which sibling fallbacks in the same parameter
  expansion did it leave unpinned?** Unsetting one name in a `${A:-${B:-$C}}` chain just moves the
  read one link down. The idiom three sibling sections in the same file already used was to pin
  `DOCKET_HARNESS_ROOT="$SBX"`; the new section had simply not copied it.
