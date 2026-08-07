---
id: 151
slug: vacuous-docket-bash-path-asserts-sit-in-eval-free-blocks-out
title: Vacuous DOCKET_BASH_PATH asserts sit in eval-free blocks, out of the poison-prelude guard's reach
status: killed
priority: medium
type: fix
created: 2026-07-28
updated: 2026-08-07
depends_on: []
related: [148, 149, 150]
discovered_from: [126]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable: false
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

`tests/test_docket_config.sh` contains asserts of the form `[ -z "$DOCKET_BASH_PATH" ]` (near the
0132 runtime-resolution section) inside blocks that contain **no `eval` at all**. They can never
fail regardless of what the variable actually holds — they are vacuous.

Change 0126's poison-prelude guard cannot see them. Its whole mechanism is keyed on `eval` sites, so
a correspondence guard of that shape has no reach into fixtures that never eval anything. This is a
distinct defect class from the stale-value hazard 0126 addressed, and was explicitly left out of its
scope.

## What changes

- Identify every assert in `tests/test_docket_config.sh` that reads a resolver-exported variable
  inside a block that performs no resolver `eval`.
- Decide per site: give the block a real resolver invocation, or delete the assert as meaningless.
- Consider whether the 0126 guard (or a sibling) can be widened to detect the class, rather than
  fixing only today's instances.

## Out of scope

- The poison-prelude guard's need-window / cleared-window asymmetry (documented in-file by 0126).
- The exempt-ceiling drift question (parked separately in 0126's results).

## Open questions

- Is a general "assert reads an exported resolver key in a block with no eval" check feasible, or
  does it produce too many false positives against helper-driven fixtures?

## Auto-groom blocked

**2026-07-28** — autonomous grooming abstained. Not because the design is hard, but because there is
**no design left to do**: change **0148**, groomed to build-ready in this same drain, discharges
every part of this stub. What remains is a verdict on the backlog's composition, and kill is never
autonomous.

### The undecidable decision

0151 asks for three things. All three are now answered:

1. **"Identify every assert that reads a resolver-exported variable inside a block that performs no
   resolver `eval`."** Swept: `tests/test_docket_config.sh` contains exactly **two** such sites
   (`DOCKET_BASH_PATH=""` on the 0132 runtime-invalid loop and the runtime-absent block). Verified in
   all three clear-spellings — `VAR=""`, `VAR=''`, and bare `VAR=` — with `/usr/bin/grep`; the other
   two spellings return nothing. The population is two, and it is closed.
2. **"Decide per site: give the block a real resolver invocation, or delete."** Decided in 0148:
   **delete**. On these fail-closed paths the adjacent `export is empty` assert is the sole channel
   and fully implies the per-variable claim, so a per-variable restatement is implied rather than
   additive. Inserting an `eval` of a provably-empty export would be a no-op added solely to satisfy
   0126's site-detection heuristic — guard-gaming, not coverage.
3. **"Consider whether the 0126 guard (or a sibling) can be widened to detect the class."** Answered
   **no**, in 0148's assumption 4: against two known instances, both of which 0148 removes, such a
   checker is over-fitting. It would need an allowlist for every legitimately eval-free assert (the
   `rc != 0`, `-z "$out"`, and stderr-diagnostic asserts around every fail-closed fixture read no
   exported key but sit in the same blocks), and an allowlist answers "is this expected?" rather than
   "does this exist?". Revisit if a third instance appears — three is a pattern, two is a pair.

So after 0148 lands, this stub's residual is empty. Whether an emptied stub should be killed, or
kept as the tracking record for the guard-widening question, is a composition call for a human.

### One correction worth carrying

This stub's framing — that the asserts are "out of the poison-prelude guard's reach" because its
mechanism is keyed on eval sites — is **wrong**, and it was verified wrong empirically. The guard's
need-windows *tile the file*: a site's window runs to the **next** eval site, not to the end of its
own block, so the `require_pr_approval` fixture's window sweeps in both asserts. That fixture's
`DOCKET_BASH_PATH=__poison__` clause exists **solely** to satisfy them, and deleting it today
reddens the guard.

What 0126 genuinely could not do was *detect* the vacuity: it proves each site clears the variables
its window's asserts read, and a `VAR=""` clear satisfies that — a limitation the file already names
in-comment. The defect is the seed forcing the asserted value, not the absence of an eval.

### What a human should supply

The verdict: kill 0151 as discharged by 0148, or keep it as the tracking record for a future
guard-widening should a third instance ever appear.

### Recommendation

**Kill**, once 0148 has merged. Its concrete half is built there and its open question is answered
there with the evidence attached. Killing before 0148 merges would lose the tracking thread if 0148
is reworked, so the kill should follow the merge rather than precede it.


## Why killed

Killed at the 2026-08-07 backlog triage: discharged. Change 0148 (merged) removed both vacuous DOCKET_BASH_PATH asserts with in-file attribution (test_docket_config.sh:2095,:2116), and the guard-widening ask was answered no in 0148's assumption 4. Nothing remains.
