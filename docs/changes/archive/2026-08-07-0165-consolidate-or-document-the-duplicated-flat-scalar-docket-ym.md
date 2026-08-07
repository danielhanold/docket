---
id: 165
slug: consolidate-or-document-the-duplicated-flat-scalar-docket-ym
title: Consolidate or document the duplicated flat-scalar .docket.yml reader in migrate-to-docket.sh
status: killed
priority: medium
type: refactor
created: 2026-07-28
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [18]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

`scripts/docket-config.sh` documents that `migrate-to-docket.sh` carries an **identical duplicate copy**
of the flat-scalar `.docket.yml` reader, explicitly "left as-is" as out of scope at the time.

ADR-0062 (change 0018) has now made the in-repo-shell-reader stance a permanent, recorded decision —
docket will not adopt an external YAML parser, so these hand-rolled readers are the long-term
implementation, not a stopgap. That raises the cost of the duplication: a fix or a hardening applied to
one copy will not reach the other, and the divergence is silent. The frontmatter readers were already
consolidated into `scripts/lib/docket-frontmatter.sh` for exactly this reason; this copy was skipped.

`migrate-to-docket.sh` has a real constraint that may justify keeping the copy: it runs standalone,
before docket is installed, so it may not be able to source a `scripts/lib/` helper the way the
installed scripts do.

## What changes

Decide between the two honest outcomes and execute it:

- **Consolidate** — factor the flat-scalar `.docket.yml` reader into a shared helper both call, if
  `migrate-to-docket.sh`'s pre-install standalone constraint permits sourcing it.
- **Document the duplication as intentional** — if it does not, state the constraint at both copies and
  add a maintenance obligation note (the pattern `scripts/lib/docket-runtime.sh` already uses for the
  permanent-by-design duplication it shares with `scripts/docket.sh`'s POSIX prologue), plus a guard
  that reddens when the two copies diverge.

## Out of scope

- Adopting `yq` or any external YAML parser — foreclosed by ADR-0062.
- Changing the reader's parsing behavior or the documented block-style subset it accepts.

## Open questions

- Can `migrate-to-docket.sh` source `scripts/lib/*` at the point it reads `.docket.yml`, or does it run
  before that path is guaranteed to exist? This decides which of the two outcomes applies.

## Why killed

Consolidated into #0256 at the 2026-08-07 backlog triage: premise was stale (the readers are no longer identical and the cited comment is gone) — #0256 restates current shapes and folds this family into the same one-owner-or-ADR ruling.
