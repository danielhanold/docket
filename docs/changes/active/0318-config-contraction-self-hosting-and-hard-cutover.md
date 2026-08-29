---
id: 318
slug: config-contraction-self-hosting-and-hard-cutover
title: 'Remaining configuration cleanup, self-hosting, and hard cutover'
status: 'in-progress'
priority: critical
type: refactor
created: 2026-08-12
updated: '2026-08-29'
depends_on: [317, 352, 363]
stacked_on:
related: [322, 326, 361]
discovered_from: [303]
adrs: []
spec: docs/superpowers/specs/2026-08-28-config-contraction-self-hosting-and-hard-cutover-design.md
plan:
results:
trivial: false
auto_groomable:
branch: 'refactor/config-contraction-self-hosting-and-hard-cutover'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-08-29T12:29:14Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-28-config-contraction-self-hosting-and-hard-cutover-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-28-config-contraction-self-hosting-and-hard-cutover-design.md) |
<!-- docket:artifacts:end -->

## Why

Docket must prove that the installed Go product can manage its own complete real lifecycle before
the production Bash implementation is removed. The final cutover must preserve settled Go v1
compatibility and rollback boundaries while publishing the first public Go release candidate.

## What changes

- Require 0352's native repository operation family and 0363's removal of unused main-mode
  compatibility before cutover.
- Rehearse and verify full self-hosting from the exact installed candidate through Claude, Codex,
  Cursor, and OpenCode.
- Finish remaining active configuration and migration-ledger cleanup, and capture migration
  learnings manually through Go.
- Remove production Bash and implementation-only Bash tests while preserving retained product
  coverage and the two approved POSIX bootstrap/downloader surfaces.
- Replace active documentation and publish the exact accepted artifacts as `v1.0.0-rc1`.

## Out of scope

Implementing 0352 or 0363, reintroducing deferred capabilities, repeating 0322's
bootstrap/adoption or 0326's early configuration contraction, retaining a Bash fallback, changing
the existing-repository compatibility contract inside 0318, Homebrew, or stable `v1.0.0`
promotion.

## Design decisions

The public tag is `v1.0.0-rc1`. The exact post-merge candidate is packaged once, accepted through
the four target tuples and four fresh host sessions, rehearsed against an isolated `v0.9.2`
rollback copy, then published without rebuilding. Historical records and the frozen rollback corpus
are not rewritten during active-surface cleanup.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

## Run halted

### 2026-08-29

Halted by an autonomous docket-implement-next run on 2026-08-29 during Step 3 (reconcile), after claim. The change is legitimately build-ready (proposed, spec linked, deps 317/352/363 all done) and was claimed cleanly. It is halted, not built, because change 0318's defined deliverable and acceptance structurally exceed what a single autonomous implement-next run (one feature-branch code PR, stopping at the human merge gate) can produce. This is a stop-and-escalate for human-driven, staged execution — not a defect in the change or its spec.

Why this cannot be driven to a single autonomous PR:

1. Whole-backlog metadata-ledger closing is part of the pre-merge deliverable, not feature-branch code. The spec's 'Close the migration ledger' step and reviewable-result item 4 require reading every active backlog item and giving each an explicit disposition (kill Bash-mechanism-only items, retain-and-link product invariants in their Go owners, leave deferred work deferred, flag ambiguous items for human disposition) through the installed Go transaction on the metadata branch, plus manual Go learning records. implement-next's feature branch adds only plan+results+code and never mutates docket metadata, so there is no place in this workflow to perform live backlog dispositions. An ambiguous item left proposed explicitly blocks 0318 acceptance and requires human judgment.

2. Acceptance requires human-attended, irreversible, post-merge release publication. The spec makes tag/release creation of v1.0.0-rc1 'an explicit, human-attended irreversible boundary': build the candidate once from the exact merged commit, dispatch the 0317 candidate workflow, publish the GitHub Release, verify remote tag/assets/checksums, then clean-install from the public URL. This is post-merge and human-attended; it is beyond implement-next's stop-at-PR and cannot be performed autonomously.

3. Acceptance requires human-verified, fresh-session self-host proofs the spec designates as human-verified truth. The exact accepted bytes must drive one complete retained lifecycle (init/health, groom, claim, reconcile, build, evidence, PR, finalize, archive, plus restart/resume recovery) through four genuinely fresh native host processes (Claude, Codex, Cursor, OpenCode), each with its own disposable clone and isolated HOME/XDG/remote, plus an isolated v0.9.2 rollback rehearsal. The spec states 'External host behavior, GitHub publication, and process-start asset loading remain human-verified truth. Missing or ambiguous evidence is a failed gate, not a documentation follow-up.' A forked autonomous agent cannot produce this evidence.

4. The scope is bound into one non-stageable change. The spec removes the Bash facade, runner/helper tree, and implementation-only tests 'in the same change' and its acceptance boundary requires all six reviewable-result items plus the four-tuple smokes and four-harness scenarios together. So I cannot autonomously narrow to just the source cutover without a human re-brainstorm (which this skill cannot perform).

Scale confirming the above (current source, 2026-08-29): 232 .sh files under scripts/ + tests/; ~127 active (non-archive/non-.docket) DOCKET_SCRIPTS_DIR reference sites across .md/.sh/.go; finalize.test_command still rooted at scripts/run-tests.sh (Bash); all docket-* skills, native agent definitions, dispatch blocks, CI gates, and active docs still reference the Bash facade. The Go CLI is feature-complete for the lifecycle (change/workspace/pr/finalize/evidence/gate/repository/learning/maintenance/install/adr/artifact/context/status/run/development all present), so the Go replacement targets exist — feasibility of the source-cutover *code* is not the blocker; the blocker is that the change's deliverable + acceptance are a human-attended, post-merge, multi-phase release/self-host protocol, not a single autonomous code PR.

Recommended human path (staged, human-driven):
- Consider splitting 0318 via re-brainstorm into (a) the Go-only source/config/docs/test cutover as an ordinary reviewable code change, and (b) a separate human-attended release-and-self-host acceptance change, if the program owner is willing to relax the spec's single-change binding; OR
- Keep 0318 whole and execute it as a human-attended run: build the source cutover on refactor/config-contraction-self-hosting-and-hard-cutover, drive the whole-backlog ledger dispositions through the installed Go product, then perform the human-attended v1.0.0-rc1 publication, four fresh-session self-host proofs, and v0.9.2 rollback rehearsal per the spec's self-host-and-release protocol.

State left clean: change 318 is in-progress on branch refactor/config-contraction-self-hosting-and-hard-cutover (no feature branch cut, no workspace prepared, no plan, no code). Resume for a genuine human-driven continuation via docket change resume-halted --id 318 --acknowledge-quiescent, or reclaim if abandoning the claim.
