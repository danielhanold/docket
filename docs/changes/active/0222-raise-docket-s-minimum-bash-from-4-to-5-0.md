---
id: 222
slug: raise-docket-s-minimum-bash-from-4-to-5-0
title: Raise docket's minimum Bash from 4+ to 5.0
status: proposed
priority: medium
type: chore
created: 2026-08-05
updated: 2026-08-05
depends_on: [211]
related: [150, 117]
discovered_from: [211]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

docket's runtime floor is **Bash 4+**, enforced in five places (`ensure-docket-env.sh`,
`docket.sh`, `docket-config.sh`, `lib/docket-runtime.sh`, `ensure-global-config.sh`). The
4.0–4.3 slice of that range carries one upstream defect — expanding a declared-but-empty array
under `set -u` raises `unbound variable`, fixed in Bash 4.4 — and supporting it has cost real
code and real test surface:

| Site | Shape |
|---|---|
| `scripts/board-checks.sh:506` | `${ADR_FILES[@]+"${ADR_FILES[@]}"}` guarded-expansion idiom |
| `scripts/board-checks.sh:506` | the `ar_bases` count gate added by change 0211 |
| `scripts/docket-status.sh:721` | guarded `adr_args` expansion |
| `tests/test_comment_anchor_style.sh:102` | loop skipped entirely when the population is empty |
| `tests/test_board_checks.sh:2403,2421` | fixture I1 exists solely to pin this crash shape |

This is at least the second time it has been paid: commit `0695b921` (**change 0117**) is
titled "guard adr_args empty-array expansion against bash 4.0-4.3 unbound-variable crash".

The decisive argument is not the line count — it is that **this support is unverifiable by
construction**:

- macOS ships Bash 3.2, which already cannot run docket at all (`mapfile`, `declare -g`), so
  every macOS user installs a newer Bash and lands on 5.x.
- `scripts/profile-asserts.sh` and `scripts/profile-one-test.sh` **already hard-require Bash 5+**
  for `EPOCHREALTIME` and exit 1 below it — so the repo effectively has two floors today.
- There is **no CI** (no `.github/workflows` at all); the only exercised runtime is the
  maintainer's Bash 5.3.
- Bash 4.0–4.3 cannot be executed on the development machine, which is why change 0211's
  mutation M shipped with a **bash-4.0–4.3 arm that is unexecutable here** — recorded as an open
  item in that change's results record.

A repo whose stated discipline is refusing tests that prove less than they claim is currently
carrying a standing set of compatibility claims that nothing can falsify.

## What changes

Raise the enforced floor from Bash 4+ to **Bash 5.0**, and collapse the two de-facto floors into
one.

- Flip the five validators to require major version >= 5, with matching remedy strings.
- Delete the guarded-expansion idioms and the explanatory comments that exist only for 4.0–4.3.
- Retire fixture I1 in `tests/test_board_checks.sh` and the `test_comment_anchor_style.sh`
  empty-population skip.
- Drop mutation M's dead 4.0–4.3 arm in the leg-C tests (change 0211).
- Update the user-facing docs and remedy text (`scripts/docket.md`, `scripts/docket-config.md`,
  `scripts/ensure-docket-env.md`, `scripts/ensure-global-config.md`, README install guidance).

**5.0 rather than 4.4.** 4.4 would close the unbound-variable hazard but leave the profiler
scripts demanding 5+, preserving the two-floor split. 5.0 gives the repo a single number.

**Deliberately still Bash 3.2/POSIX:** `scripts/ensure-global-config.sh` and
`scripts/lib/docket-runtime.sh` run *before* a configured runtime exists and must keep bootstrapping
under whatever `/bin/sh`-era Bash the host has. Raising the floor changes what they *validate*,
never the syntax they are *written in*.

## Out of scope

- Adopting Bash 5-only syntax anywhere. This change removes back-compat code; it does not spend
  the new floor.
- Adding CI or a multi-version test matrix — worth doing, but a separate change.
- Revisiting the `DOCKET_BASH_PATH` / `runtime.bash` resolution design.

## Open questions

- **Is anyone actually on 4.x?** The realistic 4.0–4.3 population is RHEL 7 (Bash 4.2, EOL June
  2024). Worth one explicit decision that docket does not support it, rather than inferring it.
- **Should the floor be asserted at all, or reported?** Change 0150 (pin or report the resolved
  shell toolchain across the test suite) overlaps here: a single floor makes that check simpler to
  state, so the two want sequencing — decide whether 0150 lands first, after, or is folded in.
- **Failure posture for an existing install on 4.x** — hard error at preflight, or a warn-and-degrade
  window? The five validators currently `die`.
- **Does the `## Why`'s unverifiability argument imply a CI follow-up** should be filed at the same
  time, so the new floor is actually exercised somewhere?

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
