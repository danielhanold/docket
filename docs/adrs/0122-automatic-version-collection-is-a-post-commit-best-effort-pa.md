---
id: 122
slug: 'automatic-version-collection-is-a-post-commit-best-effort-pa'
title: 'Automatic version collection is a post-commit best-effort pass that never reclassifies its install'
status: 'Accepted'
date: '2026-09-16'
supersedes: []
reverses: []
relates_to: []
change: 323
---

## Context

Every install-family transaction — a release install, a development-install candidate, an uninstall — can leave behind version trees that nothing references any more. The natural place to reclaim them is inside the transaction that made them unreferenced, but that placement conflates two different kinds of work. Publishing the new installed state is the operation the caller asked for and the thing whose success or failure the caller must act on; reclaiming disk from superseded trees is maintenance whose failure changes nothing about whether the install is usable. Collection is also the operation most likely to fail for reasons outside the install's control: a tree can be held open, a permission can be denied, a path can become unresolvable. If that failure were allowed to propagate into the transaction's result, a fully successful install that published its state correctly would be reported to the caller — and to any script or agent reading its exit code — as a failed install, inviting a retry of the thing that already succeeded.

## Decision

Automatic version collection runs ONLY as a best-effort pass AFTER a successful (or no-op) release install, development-install candidate, or uninstall transaction has finalized, under the same held installation lock. It is never part of the transaction's commit.

The consequence is the rule a maintainer must not break: a failure of the automatic pass surfaces as a `collection-pending` WARNING carrying the pending paths and the retry command `docket install collect`, and it NEVER reclassifies the primary operation's success. The primary result's Err and Reason are left untouched, and the automatic path exits 0.

The explicit `docket install collect` command inverts this, because there the reclamation IS the operation the caller asked for: unresolved candidates are its primary result, and it exits non-zero.

## Consequences

An install that published its state is reported as the success it is, and cannot be turned into a failure by a later reclamation that needs a retry. The pending work is not lost: the warning names the exact paths and the exact command, so a human or a follow-up run can finish it deterministically, and running it under the same held lock means the pass sees the state the transaction just committed rather than racing a concurrent installer.

The cost is that disk can stay held silently if nobody reads warnings — the automatic path is deliberately quiet in its exit code, so a monitoring setup keyed only on exit status will never learn that collection is pending. It also gives collection two different contracts in one codebase, and maintainers must keep them straight: a change that makes the automatic pass propagate its error into the primary result, or that makes `docket install collect` swallow unresolved candidates into a zero exit, breaks the decision in one direction or the other. Both surfaces share the derivation described in the reference-set ADR; only their disposition of failure differs.

## Alternatives considered

Collect inside the install transaction, failing the install on a collection error. Rejected: it reports a published, working install as failed and invites a retry of work that already succeeded.

Collect inside the transaction but swallow all collection errors. Rejected: the pending paths and the remedy disappear entirely, so held disk is never recoverable except by accident.

Run collection as a detached background job after the transaction. Rejected: it would run outside the held installation lock, racing a concurrent installer over exactly the trees whose reference status is being decided.

Give the explicit command the same non-fatal posture as the automatic pass. Rejected: for an explicitly requested reclamation the unresolved candidates ARE the result, and a zero exit would report a no-op as a completed cleanup.

Defer all collection to a separate scheduled maintenance command with no automatic pass. Rejected: ordinary upgrades would accumulate superseded trees indefinitely for anyone who never runs the maintenance command.
