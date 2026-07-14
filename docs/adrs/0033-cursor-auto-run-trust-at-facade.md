---
id: 33
slug: cursor-auto-run-trust-at-facade
title: Cursor auto-run trust is granted at the facade, not per operation
status: Accepted
date: 2026-07-14
supersedes: []
reverses: []
relates_to: [29, 20, 27]
change: 73
---

## Context

Cursor's auto-run classifier demotes an entire command program to the sandbox
when any single leaf is not allowlisted (observed live in Cursor 3.11.19,
2026-07-14; recorded in change 0073's spec appendix). docket's facade
(`docket.sh`, change 0068) was built precisely to collapse the agent-facing
runtime to one program behind one canonical invocation, making the permission
surface finite. When publishing a copyable Cursor `permissions.json` fragment
for docket, there are two ways to model the trust: allowlist the facade
invocation, or enumerate per-operation helper entries.

## Decision

Grant trust at the facade. The published `terminalAllowlist` fragment
allowlists the observed `docket.sh` invocation spellings — the canonical
guarded expansion `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh`,
the two short `:?}` guarded quote styles agents emit in practice
(`"${DOCKET_SCRIPTS_DIR:?}"/docket.sh` and `"${DOCKET_SCRIPTS_DIR:?}/docket.sh"`),
and the resolved-absolute path — rather than ~13 per-operation helper entries.
The facade is the trust boundary (ADR-0029); re-litigating it as per-op entries
would produce a fragment that drifts with every new operation and buys control
the facade was designed to make unnecessary.

## Consequences

The fragment stays a small, stable set as operations come and go, and the trust
boundary matches the one the code already draws. The price: the single
allowlist entry authorizes docket's destructive and external-writing operations
UNPROMPTED — `docket-status`'s guarded sweep (archives changes, publishes
terminal records onto the integration branch, deletes merged feature branches
and worktrees), `terminal-publish`'s direct push, `github-mirror`'s external
GitHub writes, and `cleanup-feature-branch`'s deletions. The guide states this
price plainly rather than hiding behind granularity (the guarded/provenance-checked
nature of each op is a mitigation, not a reason to omit the statement). Cursor
prefix-matches the literal command string and the short guarded forms are not
prefixes of the canonical one, so all four spellings must be listed. docket
never edits the user's Cursor config (ADR-0020) — the fragment is copy-paste,
human-applied.
