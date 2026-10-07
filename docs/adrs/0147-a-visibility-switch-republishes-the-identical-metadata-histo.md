---
id: 147
slug: 'a-visibility-switch-republishes-the-identical-metadata-histo'
title: 'A visibility switch republishes the identical metadata history under the target branch name'
status: 'Accepted'
date: '2026-10-07'
supersedes: []
reverses: []
relates_to: [1, 99]
change: 533
---

## Context

Private mode (ADR-0099, one metadata topology; ADR-0001, the metadata branch model) made visibility a fact of repository state chosen at init: a shared repository keeps its metadata on the `docket` branch on origin, a private one on the `dckt` branch of a local bare remote outside the clone. Editing configuration never moves a repository between the two, so a dedicated, resumable, preview-then-`--yes` command is the one supported move. That command must not lose records, receipts, or idempotency replays, and must never silently merge or discard a backlog.

## Decision

`docket repository set-visibility <shared|private>` preserves every record by publishing the identical metadata history under the target branch name: `docket` on origin for shared, `dckt` on the local bare remote for private. It never re-seeds, squashes, or rewrites records. Commit receipts (`Docket-Result` trailers) do not encode the branch name and links between metadata files are relative, so claim receipts and idempotency replays survive the move.

- Pushes use create-only or fast-forward leases.
- Origin's `docket` branch is deleted only with `--delete-shared-branch`, and only after verifying the bare remote holds that exact tip.
- Going shared refuses when origin already holds unrelated or diverged `docket` history; two backlogs are never merged.
- The bare remote is kept as a backup.
- Integration-branch edits are committed locally (never pushed) under two fixed subjects, "Remove docket configurations from repository" / "Add docket configurations to repository", which never contain shared, public, private, or visibility.

## Consequences

A switch is lossless and reversible: history, receipts, and replays carry over unchanged, and a switch back reuses the same history. The cost is strictness: a repository whose origin already holds a different or diverged `docket` history cannot go shared until a human resolves it, and the old shared branch persists on origin unless explicitly deleted. Keeping the bare remote as a backup leaves a redundant copy on disk. The locally committed integration-branch edits are left for the human to push, so the switch never publishes anything to the integration branch on its own, and their fixed subjects reveal nothing about visibility.

## Alternatives considered

- An option on `repository migrate`: rejected, because commands are named for their job.
- Re-seeding a fresh metadata branch from a file snapshot: rejected, because it loses commit-trailer receipts and idempotency replays.
- Rewriting records to change links: unnecessary, since links between metadata files are relative.
- Merging two backlogs when origin already holds one: out of scope and refused.
