---
id: 222
slug: raise-docket-s-minimum-bash-from-4-to-4-4
title: Raise docket's minimum Bash from 4+ to 4.4
status: 'killed'
priority: medium
type: chore
created: 2026-08-05
updated: '2026-09-03'
depends_on: [211]
related: [150, 117, 200, 227]
discovered_from: [211]
adrs: []
spec: docs/superpowers/specs/2026-08-07-raise-docket-s-minimum-bash-from-4-to-4-4-design.md
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-07-raise-docket-s-minimum-bash-from-4-to-4-4-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-raise-docket-s-minimum-bash-from-4-to-4-4-design.md) |
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
  every macOS user installs a newer Bash and lands well above the floor.
- There is **no CI** (no `.github/workflows` at all); the only exercised runtime is the
  maintainer's Bash 5.3.
- Bash 4.0–4.3 cannot be executed on the development machine, which is why change 0211's
  mutation M shipped with a **bash-4.0–4.3 arm that is unexecutable here** — recorded as an open
  item in that change's results record.

A repo whose stated discipline is refusing tests that prove less than they claim is currently
carrying a standing set of compatibility claims that nothing can falsify.

**Ruled 2026-08-07 (Daniel, triage):** raise the floor to 4.4. The competing resolution — 0200's
former item 10, rewriting `test_grep_portability.sh`'s `mapfile -d` to stay 4.0-compatible — is
dropped from 0200's scope; `mapfile -d` becomes legal. Breakage posture: **hard error at the
validators** (a 4.0–4.3 install gets a clear floor diagnostic, not warn-and-degrade). Note the
guarded-expansion population has grown since filing — now five sites, adding `run-tests.sh:270,
:278` (0227) and `docket-status.sh:407,:938` to the table above; the stale change-0064
cross-references at `docket-status.sh:909,:926` go with them.

## What changes

Raise the enforced runtime floor from Bash 4+ to **Bash 4.4** — the exact version that fixes the
defect being worked around, and no higher. Design settled in the linked spec; headline shape:

- The comparison is implemented three times, not five: the shared lib validator
  (`docket_runtime_validate_bash`), docket.sh's inline pre-exec check, and — groom-discovered —
  `run-tests.sh`'s prologue self-check, whose own threshold (4.3, for `wait -n`) must bump to 4.4
  or a 4.3 host bash crashes on this change's now-unguarded expansions. All go major+minor >= 4.4
  with matching remedy strings; the reason token `old-major` renames to `old-version`.
- Delete the guarded-expansion idioms `${ARR[@]+"${ARR[@]}"}` and their 4.0–4.3 comments (five
  sites). The two empty-array *count gates* (board-checks leg C, docket-status bases) STAY —
  they encode "ahead of nothing" semantics on >= 4.4, not back-compat.
- Retire fixture I1 in `tests/test_board_checks.sh`, the `test_comment_anchor_style.sh`
  empty-population skip, and mutation M's dead 4.0–4.3 arm (change 0211).
- Cross-reference fix is `docket-status.sh` ~926 ONLY (real precedent: change 0117 /
  `0695b921`); the ~909 change-0064 citation is correct provenance for a version-independent
  scalar guard and stays.
- Update `tests/test_bash_runtime_routing.sh` (a named maintenance obligation: fixtures, remedy
  asserts, REAL_BASH picker, new 4.3 reject fixture) and `test_docket_runtime_lib.sh`.
- Update user-facing docs and remedy text (`scripts/docket.md`, `scripts/docket-config.md`,
  `scripts/ensure-docket-env.md`, `scripts/ensure-global-config.md`, README install guidance,
  profiler prologue wording).

**4.4 rather than 5.0.** An earlier draft of this stub proposed 5.0, to collapse docket's two
de-facto floors into one — `scripts/profile-asserts.sh` and `scripts/profile-one-test.sh` already
hard-require Bash 5+ for `EPOCHREALTIME`. That argument does not survive inspection:
`EPOCHREALTIME` appears **only** in those two profilers and their contracts, and they are optional
diagnostic tooling, not the runtime path. It is normal for a dev-only tool to demand more than the
thing it profiles — and requiring every user to install 5.0 so two profiling scripts can keep a
convenience charges users for tooling they may never run.

5.0 also carries a real cost with no matching benefit: **RHEL 8 ships Bash 4.4 and is supported
until May 2029**. A 5.0 floor would exclude a currently-supported enterprise platform while buying
nothing syntactically — essentially every feature that changes how shell is written arrived in 4.x
(associative arrays, `mapfile`, `declare -g`, namerefs), all already available under today's floor.

**Deliberately still Bash 3.2/POSIX:** `scripts/ensure-global-config.sh` and
`scripts/lib/docket-runtime.sh` run *before* a configured runtime exists and must keep bootstrapping
under whatever Bash the host has. Raising the floor changes what they *validate*, never the syntax
they are *written in*.

## Out of scope

- Adopting newer Bash syntax anywhere. This change removes back-compat code; it does not spend the
  new floor. (Separately worth considering: the scripts carry ~599 `[ ]` tests against 10 `[[ ]]`,
  and 212 external `sed`/`awk`/`grep` calls — a simplification available under *today's* floor, and
  the lever that would actually retire the recurring BSD-vs-GNU tool divergence of changes
  0130/0178.)
- Changing the profiler scripts' Bash 5+ requirement. They stay as they are: optional tooling with
  a higher, self-enforced floor.
- Adding CI or a multi-version test matrix — worth doing, but a separate change.
- Revisiting the `DOCKET_BASH_PATH` / `runtime.bash` resolution design.

## Open questions

All resolved by the 2026-08-07 groom (details in the spec's Assumptions):

- **Minor-version comparison**: net simplification confirmed (~6 added lines vs. deleting five
  guarded idioms with comments, fixture I1, the anchor-style skip, and mutation M's dead arm);
  the comparison stays in the three existing implementations, no new shared helper.
- **Failure posture**: hard error (`die`) at all validators — ruled by Daniel, triage 2026-08-07.
- **Sequencing vs 0150**: independent, no ordering coupling; 0150's check merely gets simpler to
  state if 0222 lands first. Not folded in.
- **CI follow-up**: worth filing as a separate change (auto-groom does not mint stubs); this
  change stands regardless — it deletes unfalsifiable claims rather than needing new ones.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

## Why killed

Backlog review 2026-09-02 (Bash→Go migration): superseded by the Go migration — all five Bash floor-enforcement sites are deleted; the only remaining shell entrypoint is POSIX install.sh.
