---
id: 120
slug: 'historical-gate-drive-schemas-are-assessed-never-executed'
title: 'Historical gate-drive schemas are assessed, never executed'
status: 'Accepted'
date: '2026-09-14'
supersedes: []
reverses: []
relates_to: [87, 95, 118]
change: 428
---

## Context

Change 0375's worktree-admission inventory, running on first admission, met pre-0375 schema-2 gate-drive history in a real consumer repository. The executable drive reader accepts only schemas 3 and 4 and fails closed (ErrUnknownSchema) on schema 2. The inventory loaded every record through that executable reader, and it resolved the record's historical worktree path before recognizing a terminal outcome. A single unreadable or removed-worktree legacy record therefore refused an otherwise-unrelated admission, with the misleading message "a prior execution in this worktree is unresolved". (Change 0428.)

## Decision

Introduce a separate HISTORICAL reader (internal/gatedrive loadHistoricalDrive) and a shared legacy-history classifier that understands schema 2 for assessment ONLY. Historical records are never loaded into the executable state machine, never migrated, and never rewritten. The classifier recognizes trustworthy completed outcomes (PASSED/FAILED) BEFORE resolving a possibly-removed worktree path — this terminal-before-path ordering is load-bearing. For HALTED history it reuses the EXISTING single process recovery predicate (process.Service.ClassifyRun) rather than adding a second liveness implementation, and it treats a probe error or missing evidence as retained, never recovered. The executable reader's schema policy (v3/v4 only) is UNCHANGED. The same classifier backs a new idempotent `docket gate history cleanup` command.

## Consequences

Legacy completed history no longer blocks unrelated first admissions, and refusals now carry an exact inventory-legacy-drive-<id> locator plus a bounded recovery summary. Each FUTURE drive-schema bump must decide whether the retired version joins the historical-assessment range: there is a deliberate boundary between the executable schema range and the historical-assessment range, and that boundary is a decision, not a default. No drive-retirement lifecycle and no retirement receipts are introduced. Historical assessment reads a bounded view only — never command, environment, credentials, or generations.

## Alternatives considered

Widen the executable reader to accept schema 2: rejected — it would admit records the state machine cannot faithfully execute, trading a false refusal for a real correctness hazard. Migrate or rewrite legacy records in place: rejected — history is evidence, and rewriting it destroys the audit value while adding a write path to a read-only assessment. Add a second liveness probe for HALTED history: rejected in favor of reusing process.Service.ClassifyRun, so there is exactly one recovery predicate. Introduce a drive-retirement lifecycle with receipts: rejected as scope beyond the observed defect.
