---
id: 165
slug: 'unattended-private-ticket-updates-post-only-on-a-clean-final'
title: 'Unattended private ticket updates post only on a clean final leak check'
status: 'Accepted'
date: '2026-11-27'
supersedes: []
reverses: []
relates_to: [144, 163, 162, 44]
change: 588
---

## Context

Automatic ticket updates (ADR-0163) previously posted nothing in a private repository, because the private delivery path (ADR-0162) relied on a person reviewing the cleanup's before and after. Change 588 lets the automatic path post in a private repository, where nobody is present to review the export cleanup, so something other than a human has to decide whether the outbound comment and attachments are safe to post. The publish operations already enforce the private-repository writing rule with a Go leak matcher (ADR-0144), but only on what ships through the feature branch and PR, and only in private repositories.

## Decision

An unattended private ticket update skips human review of the cleanup. The direction to `docket-export` asks nobody (a call-site direction, ADR-0044, not a new mode), no before/after is shown, and the post goes ahead without anyone's go.

It is posted only when the final leak check finds nothing. After the comment is written to a file (exact posted words, real line breaks, never an escaped structured body) and the attachments are final, `docket-update-ticket` runs the new read-only `leak.check` operation (`docket leak check --file ...`) once over the comment file and every attachment file. `leak.check` uses exactly the publish leak check's patterns and the same options, built through the same `leakOptions` helper: the effective `leak_check.match_word` and every change id in the metadata snapshot.

- `applied` and `clean: true`: post, attachments first and comment last.
- `applied` with any hit, or a `docket-export` word-check hit: post nothing and report one line naming where it hit and the exported files' paths.
- Any other result (the check did not run): never treated as clean; post nothing and report one line.

There is no retry and no fix-and-recheck: a hit posts nothing.

What the patterns deliberately miss (docket ADR numbers, unpadded change ids such as `#612`, bare numbers, and the words "backlog" and "board") relies on the cleanup alone, which always rewrites docket's change and ADR numbers. An ADR number that slips through is accepted.

Unknown visibility (a value other than `shared` or `private`, or no `context.ticket` result) is treated as private and, since nobody can be asked, posts nothing and reports one line.

`leak.check` scans in any visibility, as a pure read. The publish operations still run their check in private repositories only, so ADR-0144's "shared repositories never invoke the scanner" keeps holding for publishes.

## Consequences

Automatic ticket updates now work in private repositories without a human present, gated by the same Go rules that guard publishes, so the ticket gate and the publish gate cannot drift apart. The cost: a single hit, or any failure to run the check, drops that update entirely; the person gets one report line and the exported paths and can inspect them or request the update on request. Leaks outside the matcher's patterns rest on the cleanup's rewrite alone. Docket gains one new read-only operation, `leak.check`, usable on arbitrary outbound files in any visibility.

## Alternatives considered

- A review step before posting: rejected, nobody is there on the automatic path.
- An agent-run pattern search instead of the Go matcher: rejected, it would drift from the Go rules, lack docket's change-id list, and the agent that did the cleanup would be grading its own work.
- Fixing a flagged line and checking again: rejected, a hit posts nothing, as the earlier design fixed.
- A new `docket-export` mode for unattended use: rejected, the existing file-request path with a call-site direction (ADR-0044) already decides the outcome.
