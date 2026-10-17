---
id: 151
slug: 'a-change-s-deliverable-decides-how-it-reaches-done-done-no-l'
title: 'A change''s deliverable decides how it reaches done; done no longer implies a merge'
status: 'Accepted'
date: '2026-10-17'
supersedes: []
reverses: []
relates_to: [129, 141, 148]
change: 559
---

## Context

Every change used to end through a PR: done meant merged. Research and investigation changes produce a results artifact, not code, so they had no legal ending. Operators either killed them (which reads as abandoned), hand-edited them to done, or left them open. Change 559 (spec section 8, docs/superpowers/specs/2026-10-17-close-a-change-to-done-without-a-pr-design.md) adds a supported path.

## Decision

A change declares its deliverable: `pr` (the default) or `results-artifact`. A results-artifact change reaches `done` through the new `change.complete` operation with no PR, so `done` no longer implies a merge. `change.mark-implemented` accepts only `pr` changes, so each deliverable has exactly one legal ending. A results-artifact change takes no part in stacking (neither parent nor child). A completed change's Plan and Results artifact rows are never treated as legacy and link to the metadata branch; this refines ADR-0141's legacy-row rule. Every no-op replay result names what already happened (`already-<what happened>`, for example already-completed, already-closed-out, already-recorded, already-retargeted), never a bare `already`.

## Consequences

Finished research changes close to the happy final status with their results artifact as the record of work, and the board stops showing them as stuck or abandoned. Readers can no longer infer a merged PR from `done`; they must consult the deliverable. Stacking logic and mark-implemented gain a deliverable check. Replay dispositions across operations are renamed to specific already-* values, which callers matching a bare `already` must update.

## Alternatives considered

Killing finished research changes: rejected because killed reads as abandoned and misreports successful work. Hand-editing the record to done: rejected because it bypasses typed operations, the board render, and the archive transaction. A separate final status (for example `completed`): rejected because it adds a third final outcome every reader, board section, and selector must learn, while `done` plus a deliverable field carries the same meaning.
