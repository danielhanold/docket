---
id: 45
slug: auto-capture-is-best-effort
title: Auto-capture is best-effort — a failed stub mint never aborts the change being built
status: Accepted
date: 2026-07-18
supersedes: []
reverses: []
relates_to: [12]
change: 91
---

## Context

Change 0091 adds `auto_capture`: when enabled, docket's autonomous skills —
`docket-implement-next`'s reconcile and review passes, and the
`docket-finalize-change` / `docket-status` close-out harvest — mint a
`proposed` needs-brainstorm stub for follow-up work they discover mid-run, via
the deterministic `scripts/mint-stub.sh`, reached as `docket.sh mint-stub`.
The script returns `0` (minted), `3` (duplicate skipped), `4` (per-invocation
cap reached), or `1` (a real error — push failure, malformed body, refused
reset, retry exhaustion).

docket's established house posture for facade-op exit codes is the opposite
of best-effort: the terminal close-out reference and `docket-implement-next`
step 4 both say to trust each exit code, and a non-zero exit **aborts** the
operation and is surfaced (abort-and-report). Under that default, an
implementer reading only the skill prose would treat a non-zero mint as
fatal.

Exit `1` is reachable in ordinary operation, not just adversarially: (a)
contention — another agent holding an uncommitted tracked edit in the shared
`.docket` metadata worktree causes the CAS retry's clean-tree gate to refuse;
(b) `main`-mode, where the `.docket/<changes_dir>` path the invocation names
does not exist, so every mint fails; (c) a body file that does not begin with
`## Why`, the script's enforced precondition.

## Decision

Auto-capture is explicitly **best-effort**: every mint outcome — success,
dedup skip, cap overflow, and hard error — is **surfaced in the run report**,
and **none of them aborts the change being built**. Capture is a courtesy;
the change under construction is the job. This is a deliberate, narrow
exception to docket's abort-and-report exit-code posture, and it is stated in
the convention's `### Auto-capture (shared definition)` section, which is the
single source the three mint sites reference.

Relates to ADR-0012 (the script-vs-model boundary): `mint-stub.sh` performs
the deterministic mint mechanics; the calling skill (model) judges what
discovered work is material enough to capture. This decision governs only
how the model treats the script's exit code once that judgment call has
already been made to invoke it.

## Consequences

**Enables:** a capture failure can never cost a build, so enabling
`auto_capture` carries no availability risk for the change being built —
important because the failure modes above are contention-shaped and
therefore most likely to occur exactly when the system is busiest.

**Costs:** discovered work can be silently *not captured* when a mint fails,
so the run report becomes the only evidence that a capture was attempted and
lost — which is why surfacing every outcome (not only failures) is part of
the decision, not an afterthought. It also means the exception must be
stated where implementers actually read it (the shared convention section),
or the house abort-and-report default silently reasserts itself at each new
call site.

**Consistency note:** this mirrors the existing posture for the learnings
harvest — a failed learnings harvest is likewise best-effort — so
auto-capture aligns with the other close-out courtesy, not with the
terminal-transition operations (archive, terminal-publish, cleanup, board),
which remain must-land / abort-and-report.
