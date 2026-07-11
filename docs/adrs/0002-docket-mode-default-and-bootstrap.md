---
id: 2
slug: docket-mode-default-and-bootstrap
title: docket-mode is the default; refuse-and-migrate bootstrap; terminal-publish single-sourced in finalize
status: Accepted
date: 2026-06-03
supersedes: []
reverses: []
relates_to: [1]
change: 2
---

## Context

Making docket-mode real (ADR-0001) raised three decisions that the branch model alone doesn't settle: (a) is docket-mode the default or an opt-in? (b) what happens to the many existing single-branch repos — including docket itself — the moment the default flips? (c) the terminal-publish procedure is invoked by four different skills (finalize on `done`; producer and implementer on `killed`; `docket-adr` for standalone ADRs) — where does it physically live so it isn't duplicated or allowed to drift?

## Decision

- **docket-mode is the literal default.** Absent `.docket.yml` ⇒ `metadata_branch: docket`. `main`-mode is a pinned opt-out that reproduces single-branch behavior exactly. A new **`integration_branch`** knob (`auto`→`origin/HEAD`, fallback `main` | `main` | `develop`) decouples *where code lands* from *where metadata lands*, supporting trunk and GitFlow. `.docket.yml` lives on the repo's **default branch (`origin/HEAD`)** — not the integration branch — so it is findable with zero prior config (it *declares* `integration_branch`, so it can't be located *by* it).
- **First run refuses; it never auto-migrates.** A four-state bootstrap guard (over `DOCKET` = the branch exists, and `LIVE` = the live planning surface still on the integration branch) STOPs an un-migrated repo and points to a one-shot, idempotent **`migrate-to-docket.sh`**; it also detects a half-migrated (interrupted) state. No skill silently restructures a repo's branches.
- **The terminal-publish procedure is single-sourced in `docket-finalize-change`.** The other terminal-transition drivers reference it ("the *Terminal publish* procedure in `docket-finalize-change`") rather than restating the git sequence. It is an operational *procedure*, so it stays **out** of the byte-identical convention block (which would 5×-duplicate ~25 lines of git); the short bootstrap-guard *rule* does live in the convention, because that is a cross-agent contract, not a procedure.

## Consequences

- **Enables:** clean code history out of the box; one procedure to maintain with four references (the test suite asserts both the single source and the references); GitFlow support.
- **Costs / given up:** every existing single-branch repo (including docket itself) must either run `migrate-to-docket.sh` or pin `metadata_branch: main` before the default flip reaches it. docket therefore **dogfoods the pin** — it ships a `.docket.yml` pinning `main`-mode and defers its own migration to a separate follow-up — so the default flip doesn't break its own tooling on merge.
- **Trade:** refuse-and-migrate costs a one-time manual step on legacy repos in exchange for zero risk of silently stranding planning metadata on the wrong branch.

## Update — 2026-06-19 (change 0025)

**Decision 3 still stands.** Change 0025 extracted the *mechanics* of the terminal-transition close-out into three deterministic, fail-closed scripts — `scripts/archive-change.sh`, `scripts/terminal-publish.sh`, and `scripts/cleanup-feature-branch.sh`. This does NOT reverse or supersede Decision 3: `docket-finalize-change` remains the single documented **source and owner** of the procedure. It still owns *when* to run each step and authors the human-legible commit messages. What changed is that the four call sites now **invoke the named scripts** rather than restating the git sequence in prose — the same doc-owner-keeps-*when* / script-owns-*how* split that `docket-status` ↔ `render-board.sh` already has (ADR-0007).

The **ADR-only publish path** (`T = adr-<NN>`, driven by `docket-adr` for standalone ADRs) is **not** covered by `scripts/terminal-publish.sh` (which is keyed on `--id` for a change) and remains documented as an in-procedure step inside `docket-finalize-change`'s terminal-publish procedure. No change there.
