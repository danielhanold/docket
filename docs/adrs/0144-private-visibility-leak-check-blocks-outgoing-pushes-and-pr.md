---
id: 144
slug: 'private-visibility-leak-check-blocks-outgoing-pushes-and-pr'
title: 'Private-visibility leak check blocks outgoing pushes and PR edits carrying docket fingerprints'
status: 'Accepted'
date: '2026-10-06'
supersedes: []
reverses: []
relates_to: [142]
change: 532
---

## Context

Docket's default is that new checks are report-only, never blocking. In a private-visibility repository, however, a docket or dckt fingerprint pushed to a shared origin is irreversible and outward-facing: rebase-merge carries it into main, and history cannot be recalled once shared. The check cannot affect shared repositories, because they never invoke the scanner, so making it blocking is a deliberately scoped exception to the visibility-only default rather than a change to it.

## Decision

In private-visibility repositories, a leak check blocks every feature-branch push (workspace.publish, finalize.publish) and every PR create/edit (pr.publish) when outgoing content carries a docket or dckt fingerprint. The refusal is typed `leak-detected` and carries per-hit {source, commit, file, line, matched text, rule}; the implement-next run halts via change.halt.

Scope of the scan: every outgoing non-merge commit's message and added lines and paths (per commit, plus the merge-base diff), the feature branch name, and the PR title and body.

Patterns: `dckt`; `docket` (switchable via `leak_check.match_word`, default true); `docket:`/`dckt:` markers; `Docket-` trailers; `.docket` and `.git/dckt` paths; zero-padded change refs to backlog ids.

Shared repositories never invoke the scanner. An unreadable outgoing range fails closed: nothing is pushed.

Related postures stay unchanged: metadata-on-shared-remote findings for docket/dckt branches and refs/docket/ refs on origin remain report-only, and pushes refuse any ref outside refs/heads/.

## Consequences

Private repositories cannot leak docket vocabulary into PRs, commits, or shipped files through docket's own publish paths; a hit stops the run for a human instead of shipping. The cost is a blocking gate (the only one so far outside the visibility-only default), possible false positives on the bare word `docket` (mitigated by `leak_check.match_word`), and per-commit scanning work on every publish. Shared repositories are unaffected.

## Alternatives considered

Report-only finding: rejected, because a pushed leak is irreversible and a report arrives too late. Scanning only the net base...head diff: rejected in review, because a fingerprint added then removed across commits still ships in history. Scanning shared repositories too: rejected, because docket vocabulary is legitimate there.
