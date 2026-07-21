# Complete the `finalize.test_command` cross-layer masking matrix — design

Change: 0112 — *Pin the reverse cross-layer masking for the committed-over-local rung pair*
Date: 2026-07-20 · Author: `docket-auto-groom` (autonomous; assumptions gated by `docket-auto-groom-critic`)

## Problem

`scripts/docket-config.sh` resolves `finalize.test_command` through a flat three-rung `:-` chain
(`:195`) — local `.docket.local.yml` → committed `.docket.yml` → global
`${XDG_CONFIG_HOME}/docket/config.yml` — and collapses the `auto` sentinel to `""` **after** the
chain (`:202`). That placement is the behavior: a **higher** rung's `auto` masks a **lower** rung's
real command, while a **lower** rung's `auto` must never wipe a **higher** rung's real command.

Change 0106 pinned three of the six ordered rung pairs. Writing each pair as *(rung holding `auto`
→ rung holding the real command)*, with local > committed > global:

| pair | direction | fixture |
|---|---|---|
| local `auto` → committed real | forward (higher `auto` masks lower real) | `s4` ✅ |
| committed `auto` → global real | forward | `s5` ✅ |
| local `auto` → global real | forward, skip-rung | **gap** → `s9` |
| global `auto` → committed real | reverse (lower `auto` must not wipe higher real) | `s6` ✅ |
| committed `auto` → local real | reverse | **gap** → `s7` |
| global `auto` → local real | reverse, skip-rung | **gap** → `s8` |

Three cells are unpinned. The one the stub names — `s7`, committed `auto` under a local real
command — is the dangerous one: a real repo whose `.docket.local.yml` sets
`test_command: make local-test` over a committed `test_command: auto` would silently lose its
local command, and finalize would fall back to auto-detection with the whole suite green. The
concrete refactor that produces exactly that, while leaving all five of 0106's asserts green, is a
committed-rung-specific clear appended after the chain:

```sh
[ "$(yaml_get "$CFG" test_command)" = auto ] && FINALIZE_TEST_COMMAND=""
```

That mutation is the entire case for `s7`, and it reaches **only** `s7`. 0106 declined the
skip-rung ordering (`s9`) on the grounds that it "reuses the two helpers the forward cases already
cover and adds no distinct code path" — and on discriminating power **0106 was right**: `s9`'s
only witness is M1 below, the same mutation that already reddens `s4` and `s5`. The case for `s7`
therefore rests on precedence and rebuts nothing 0106 decided; `s8` and `s9` are added on the
separate and weaker ground stated in A1/A2 — matrix completeness, at the cost of two fixtures, with
no claim of unique discriminating power.

## Decision

Complete the matrix. Add **three** fixtures to section S of `tests/test_docket_config.sh`,
following the established `s4`/`s5`/`s6` shape, so all six ordered rung pairs are pinned:

- **`s7` — reverse, committed `auto` under local real.** Committed `.docket.yml` sets
  `test_command: auto`; `.docket.local.yml` sets `test_command: make local-test`. Assert the
  resolved export is `make local-test`.
- **`s8` — reverse, skip-rung: global `auto` under local real.** Global `config.yml` sets
  `test_command: auto`; the committed rung leaves the key absent; `.docket.local.yml` sets
  `test_command: make local-test`. Assert `make local-test`.
- **`s9` — forward, skip-rung: local `auto` over global real.** Global `config.yml` sets
  `test_command: make global`; the committed rung leaves the key absent; `.docket.local.yml` sets
  `test_command: auto`. Assert the resolved export is empty, preceded by a control assert that the
  global command resolves before the masking layer is added.

"Committed rung absent" means the **key** is absent from `.docket.yml`; the file itself still
exists and still pins `metadata_branch: main` / `integration_branch: main`, exactly as `s5`'s first
phase does (`tests/test_docket_config.sh:1066-1074`). To be precise about *why*, since a fixture
comment must not encode a false reason: dropping the file entirely also resolves the key correctly
— it simply routes the fixture through `BOOTSTRAP=CREATE_ORPHAN` / docket-mode instead. The file is
kept for consistency with the main-mode shape every other section-S fixture uses, and to keep a
`test_command` fixture off the bootstrap path, **not** because omitting it would break resolution.

This is a **test-only** change plus the section's comment header. No resolver edit, no doc edit, no
ADR: the change pins current behavior, as 0106 did.

### Mutation protocol (the completion bar)

Each new assert must be shown to redden under at least one mutation of `scripts/docket-config.sh`,
per the repo's `guards-are-code` rule. Three mutations, run against the real resolver, reading the
`ok` count and the named lines — not merely the suite's PASS/FAIL:

| mutation | s4 | s5 | s6 | **s7** | **s8** | **s9** |
|---|---|---|---|---|---|---|
| **M1** collapse `auto` per-layer, before the chain | RED | RED | green | green | green | **RED** |
| **M2** blanket "any rung says `auto` ⇒ unset" | green | green | RED | **RED** | **RED** | green |
| **M3** committed-rung-specific clear (shown above) | green | green | green | **RED** | green | green |

M1 and M2 are 0106's own two mutations, re-pointed at the new asserts; **M3 is new to this change**
and is the one that uniquely isolates `s7` — under M3 every one of 0106's five asserts stays green,
which is precisely why `s7` has to exist. The mirror-image global-rung-specific clear is already
recorded against `s6` in 0106's results file and is **not** re-run here.

Every new assert is witnessed: `s7` by M2 and M3, `s8` by M2, `s9` by M1. None of the three is
decoration.

## Assumptions

**A1 — Comprehensive matrix over the single named fixture.** *Chosen:* add `s7`, `s8`, and `s9`,
closing all three open cells. *Rejected:* (a) `s7` alone, as the stub literally scopes it — leaves
the matrix asymmetric and re-opens the same "which orderings are pinned?" question the next time
this key is touched; (b) `s7` + a comment explaining why the skip-rung cells are unnecessary — a
comment is a claim, not a guard, which is the exact failure mode 0106 was written to end. *Why:*
the standing house preference is the comprehensive fix over the narrow patch, and the cost here is
two additional `mkrepo` fixtures in a suite that already builds dozens. The bar that made this
safe rather than reflexive is the mutation table: `s8` and `s9` each redden under one of 0106's
canonical mutations, so completing the matrix does not smuggle in decoration.

**A2 — `s8`/`s9` earn their place on shared, not unique, mutation witnesses.** *Chosen:* accept a
mutation witness that also reddens an existing assert. *Rejected:* requiring every new fixture to
have a mutation that reddens it and nothing else. *Why:* `guards-are-code` requires that a guard
redden when the thing it guards is broken, not that it be the sole detector. Demanding uniqueness
would force contrived mutations (e.g. "clear when global is `auto` **and** committed is unset")
that model no plausible refactor. Stated openly so a reviewer can weigh it: `s7` is the fixture
with unique discriminating power; `s8`/`s9` are configuration-completeness witnesses.

**A3 — Keep the established per-fixture shape; do not refactor `s4`–`s6`.** *Chosen:* copy the
existing pattern — own `$tmp/<n>` repo, own `$tmp/<n>.xdg` global root via `rung`, a
`FINALIZE_TEST_COMMAND=__poison__` line before every `eval`. *Rejected:* a table-driven loop over
the six cells. *Why:* the cells are not uniform (two-phase writes, control asserts on some and not
others, one committed-file rewrite in `s5`), so a table would need escape hatches; and rewriting
0106's just-merged fixtures inflates the diff of a change whose entire value is a few new asserts.

**A4 — Control asserts only where the expectation is empty.** *Chosen:* `s9` gets a control
(assert the global command resolves, *then* add the masking local rung); `s7` and `s8` get none.
*Why:* an empty export is also what an absent key yields, so an empty-expecting assert can pass for
the wrong reason — that is why `s4`/`s5` have controls. `s7`/`s8` assert a distinctive non-empty
string, which no misconfiguration produces by accident; `s6`, the existing reverse case, has no
control for the same reason.

**A5 — Distinct command strings per rung, and hermetic global roots.** *Chosen:* `make local-test`
(local), `make test` (committed), `make global` (global), matching the strings already in use; each
fixture roots the global layer at its own `$tmp/<n>.xdg` through the `rung` helper. *Why:* distinct
strings mean a cross-fixture leak cannot read as a pass, and the per-fixture root keeps the suite
from ever reading — or writing — the developer's real `~/.config/docket/config.yml`
(`config-layer-write-and-read-hazards`). The `.docket.local.yml` files are written into the fixture
clone and never `git add`ed.

**A6 — Section header updated; 0106's archived record left untouched.** *Chosen:* retitle the
section comment from `(S4/S5/S6)` to cover `S4`–`S9`, and in it credit **`s7` alone** on precedence
grounds (its unique witness is M3), labelling `s8`/`s9` as completeness witnesses that share `s6`'s
and `s4`/`s5`'s mutations respectively. *Rejected:* (a) writing that 0106's skip-rung exclusion was
wrong — this change's own mutation table shows it was right about `s9`, and shipping the opposite
as a code comment is exactly the unfalsifiable claim `verify-the-claim` exists to stop; (b) editing
0106's archived change file or its spec. *Why:* archived records are history, and a comment must
state only what the mutation runs actually establish.

**A7 — No resolver, doc, or ADR change.** *Chosen:* test-only. *Why:* the change pins existing
behavior. If a mutation run reveals the resolver does **not** behave as the matrix predicts, that
is a genuine defect discovery, not a licence to edit the resolver under this change — record it and
stop, because the correct fix is a behavior change needing its own review.

**A8 — Dependency state.** `depends_on` is empty and `discovered_from: [106]`, which is `done` and
merged, so section S is present on `origin/main` in the shape this design assumes. The implementer's
reconcile pass re-verifies the `:195`/`:202` anchors and the `s4`–`s6` fixtures before building —
concurrent work on this file is the only realistic drift.

*Reconcile (2026-07-21):* that drift materialized and was absorbed. Change 0102 landed after this
spec was authored, inserting `require_pr_approval` resolution into `scripts/docket-config.sh` and
shifting both anchors down one line — the chain is now `:195`, the collapse `:202` (this section
and the Problem statement above are corrected accordingly). Section S itself and the `s4`–`s6`
fixtures are byte-unchanged in the assumed shape, and 0102's own `43b1aca` already re-anchored the
section-S header comment to `:202`/`:195`, so the header the implementer edits is already correct
and must not be "fixed" back.

## Out of scope

- Any change to the sentinel's semantics or the placement of the collapse.
- Re-running 0106's own two mutation runs as recorded in its results file; M1/M2 are re-pointed at
  the new asserts only.
- Extending the `auto` sentinel to any other key (`github_project` remains unwired — change 0103).
- A table-driven rewrite of section S (A3).
