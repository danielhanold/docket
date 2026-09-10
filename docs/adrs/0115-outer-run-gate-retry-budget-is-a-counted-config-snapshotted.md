---
id: 115
slug: 'outer-run-gate-retry-budget-is-a-counted-config-snapshotted'
title: 'Outer run gate retry budget is a counted, config-snapshotted allowance (GateRecord schema v4)'
status: 'Accepted'
date: '2026-09-10'
supersedes: []
reverses: []
relates_to: [74, 75, 107, 111]
change: 421
---

## Context

Before change 0421 the outer implement-next run gate granted exactly one retry for an attributed, quiescent run-incomplete, encoded as a single binary `retry-consumed` marker file created with O_CREATE|O_EXCL (ConsumeGateRetry in internal/app/rungate_store.go). The allowance was therefore a constant of the implementation: there was no way for a repository to grant more than one retry, or none at all, and the marker carried no record of how many attempts a run had actually spent. Change 0421 makes the allowance configurable via `run.max_attempts` (built-in default 2, a positive integer, counting the initial attempt; the value 1 disables retries), which requires the durable gate record to represent a counted budget rather than a single consumed bit, without weakening the exactly-once CAS that keeps two concurrent observers of the same completed attempt from both granting a retry.

## Decision

Generalize the single binary retry marker into a counted per-attempt compare-and-swap. Per-attempt markers `retry-consumed-<n>` are created with O_CREATE|O_EXCL; the current attempt is derived from the persisted marker count (`attempt = 1 + GateRetryUsage`), and a grant is authorized iff `attempt < AttemptLimit`. `AttemptLimit` is snapshotted into the GateRecord at mint from the authoritative resolved config and is never re-resolved afterward, so a config edit after mint does not change an owned record's limit. The durable GateRecord schema version is bumped 3 -> 4; a v3-or-older record fails closed on load with the existing schema-mismatch diagnostic, whose recovery is a `gate-before --resume` re-arm — a silent v3 -> v4 migration is deliberately rejected. A bare legacy `retry-consumed` marker counts as the attempt-1 consumed marker, so an older already-consumed permit can never be re-granted. The `gate-retry-once` / `gate-stop` report tokens are unchanged: each `gate-retry-once` still authorizes exactly one next dispatch, and usage/limit are surfaced only as additive JSON fields. Consumption stays ordered after the takeover/continuation check and after run-halted precedence; run-waiting, tracked-drive recovery, and routine observation consume no attempt; concurrent observers of the same completed attempt race on the marker so exactly one grants.

## Consequences

The default limit of 2 preserves the prior single-retry behavior exactly, so no repository changes behavior by upgrading. Higher limits grant the corresponding number of eligible retries, and a limit of 1 grants none. The hard cap always holds: at most `run.max_attempts - 1` markers can exist over a record's life. Snapshotting the limit at mint costs the ability to widen an in-flight run's budget by editing config, which is the intended trade — an owned record's authorization must not shift under it. Failing closed on a v3 record costs a re-arm after upgrade, in exchange for never silently reinterpreting an old record's consumed state. Known bounded, safe-erring limitation, deferred to a follow-up: because the attempt is derived from the raw marker count rather than a per-dispatch epoch, sequential repeat diagnostic `gate-verdict` observations of the SAME unchanged quiescent incomplete at a limit of 3 or more each grant a successive retry until the budget is spent. This never exceeds the hard cap and the default limit of 2 is immune, but a proper epoch-bound fix touches the deferred ship-once attribution/concurrency model and is out of scope here.

## Alternatives considered

Keep the single binary marker and multiply it by an out-of-band counter file: rejected because two files can disagree, and the exactly-once property depends on a single O_EXCL create being the authorization. Re-resolve `run.max_attempts` from config at each verdict instead of snapshotting at mint: rejected because a mid-run config edit would then retroactively change an owned record's authorization, and gate decisions must be reproducible from the record. Silently migrate a v3 record to v4 by treating a present `retry-consumed` marker as usage 1: rejected as the load-path default because a silent rewrite of durable authorization state is exactly the class of change that should fail closed and be re-armed deliberately — the legacy marker is still honored as the attempt-1 consumed marker where a v4 record is in play, which gets the safety without the silent migration. Introduce a new report token (for example `gate-retry-remaining <n>`): rejected because every caller's contract keys on the existing tokens, and each grant still authorizes exactly one dispatch; the count belongs in additive JSON fields, not in the parsed report line. Bind the attempt to a per-dispatch epoch now, closing the repeat-observation gap: rejected for this change because it reaches into the deferred ship-once attribution model; the gap is bounded by the hard cap and invisible at the default.
