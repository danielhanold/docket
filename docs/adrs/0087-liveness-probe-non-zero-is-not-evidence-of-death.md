---
id: 87
slug: liveness-probe-non-zero-is-not-evidence-of-death
title: "A liveness probe's non-zero answer is not evidence of death — only a failed kill -0 is"
status: Accepted
date: 2026-08-11
supersedes: []
reverses: []
relates_to: []
change: 284
---

## Context

`scripts/lib/docket-liveness.sh` (new in change 0284) is the single identity-checked liveness
predicate, shared by `scripts/gate-run.sh` and `scripts/runner-dispatch.sh`. It returns non-zero on
several distinct legs: the recorded pgid is absent, non-numeric, `0` or `1`; `kill -0 -<pgid>`
fails; the recorded identity token is empty; the live token is unreadable; the two tokens differ.

The predicate was written fail-closed on **every** leg, and the justification imported from
`gate-run.sh` was: *"a false `dead` costs one wasted observation, while a false `alive` costs the
caller its ENTIRE budget."* That asymmetry is true **for gate-run.sh**, where a false `dead` costs
exactly one bounded relaunch of an idempotent suite.

It is **not** true for the new `runner-dispatch --observe` consumer. There a false `dead` writes a
terminal `killed` marker, returns a terminal exit code, and ends the caller's polling loop —
permanently. And because git decides the code on that leg, it can return **`0`** ("the work
landed") for a child that is **still running**, handing a driver a green result so it draws the next
change while the live run keeps writing. The cost of a false `dead` is not one wasted observation;
it is an unrecoverable wrong verdict plus a concurrent-run hazard.

The review that caught this also established that the failure is **reachable, not theoretical**:
`docket_identity_of` renders through `ps -o lstart=`, whose output is `TZ`/`LC_TIME`-dependent —
`TZ=UTC`, `TZ=Asia/Tokyo` and `TZ=America/New_York` produce three different tokens for the same
pid. A `--launch` and an `--observe` invoked under different environments therefore make a healthy
long-running child unprovable, and under a uniformly fail-closed predicate that means *dead*.

Crucially, `runner-dispatch.sh` already owned the right machinery for exactly this case:
`note_unenforceable` counts consecutive passes on which the budget could not be enforced, bounds
them at 3, and then terminates with an honest cause. The uniformly-fail-closed predicate bypassed
it.

## Decision

Separate the two answers a liveness probe can give, and route them differently:

- **`gone`** — `kill -0 -<pgid>` failed. This is *positive evidence* that the recorded group is not
  there. It alone may reach a terminal disposition (`dispose_vanished_child`).
- **`unprovable`** — everything else: an absent/unusable pgid, an empty recorded token, an
  unreadable live token, a token that cannot be compared. Nothing is known about the child. This
  routes through the existing `note_unenforceable` counter, which is already bounded at 3
  consecutive passes and already terminates with an honest cause.

The lib exposes this as a reason **class** (`DOCKET_LIVENESS_CLASS`) alongside the existing
printable reason (`DOCKET_LIVENESS_WHY`). The class is **additive**: `gate-run.sh` continues to
treat any non-zero as "not alive", which remains correct for it, and its behaviour is unchanged
(verified — `tests/test_gate_run.sh` and `tests/test_gate_run_stop.sh` pass unchanged, and a
mutation reclassifying the mismatch leg reddens the runner-dispatch shard while leaving gate-run
entirely green).

Diagnostic wording follows the class: "died without writing a sentinel" only on `gone`, "can no
longer be proven alive" on `unprovable` — because asserting a death that was never established is
the same fabricated-verdict failure the same code refuses to commit about an exit code it never
read.

Additionally, `docket_identity_of` now reads `ps` under a pinned locale with `TZ` unset, so the
caller's environment cannot move the rendering. (`unset TZ` rather than `TZ=UTC`: an existing assert
in `tests/test_gate_run.sh` compares the recorded token against a hand-rolled *ambient* `ps`
rendering, and that file is change 0284's own behaviour-preservation safety net and may not be
edited.)

## Consequences

- Enables: an unprovable liveness answer can no longer manufacture a terminal verdict, and can no
  longer produce a false green for a live run.
- Costs: a genuinely recycled pgid whose child's work *did* land in git now reports unavailable
  after three bounded passes rather than `0` on the first, because the bounded terminal path does
  not consult git. Deliberate — fail-closed toward "we cannot say" rather than toward a wrong
  verdict.
- Costs: two reason channels instead of one, and consumers must know which they care about.
  Mitigated by the class being additive and by `gate-run.sh` correctly ignoring it.
- Generalizes: the rule is not specific to this predicate. Wherever a fail-closed predicate is
  reused by a second consumer, the **cost asymmetry that justified failing closed must be re-derived
  for that consumer** — it does not travel with the code. A shared predicate may report *why* it
  failed, but the disposition belongs to the caller.
- A residual, recorded rather than fixed here: `tests/test_gate_run.sh` hand-rolls its own
  `ps -o lstart=` rendering instead of reading it through the shared helper, which leaves that one
  assert locale-fragile under a non-English `LC_TIME`. Out of 0284's scope (the file is its safety
  net); noted for a follow-up.
