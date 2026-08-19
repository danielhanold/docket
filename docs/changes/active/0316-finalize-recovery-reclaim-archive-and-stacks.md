---
id: 316
slug: finalize-recovery-reclaim-archive-and-stacks
title: 'Finalize, recovery, reclaim, archive, and stacks'
status: 'in-progress'
priority: critical
type: feat
created: 2026-08-12
updated: '2026-08-19'
depends_on: [315, 322, 326]
stacked_on:
related: [298, 318, 322, 326]
discovered_from: [303]
adrs: [10, 11, 35, 43, 59, 66, 74, 83, 86, 92, 95]
spec: docs/superpowers/specs/2026-08-18-finalize-recovery-reclaim-archive-and-stacks-design.md
plan:
results:
trivial: false
auto_groomable:
branch: 'feat/finalize-recovery-reclaim-archive-and-stacks'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-08-19T00:42:22Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | `docs/superpowers/specs/2026-08-18-finalize-recovery-reclaim-archive-and-stacks-design.md` |
| ADRs | `docs/adrs/0010-finalize-merge-gate-split-agents.md`, `docs/adrs/0011-finalize-consent-model.md`, `docs/adrs/0035-cleanup-teardown-fail-closed.md`, `docs/adrs/0043-retire-bot-auto-approval-zero-approvals-branch-protection.md`, `docs/adrs/0059-dispatch-capability-resolved-not-inferred-from-tool-name.md`, `docs/adrs/0066-docket-owns-the-review-role-suite-runs-in-the-build-gate.md`, `docs/adrs/0074-build-gate-verdict-is-tri-state-runner-defined-non-failure-exit-is-a-halt.md`, `docs/adrs/0083-agent-worktree-scope-is-a-declared-frontmatter-fact.md`, `docs/adrs/0086-in-context-gating-dispatch-carved-out-of-the-tier-taxonomy.md`, `docs/adrs/0092-a-stacked-changes-base-is-its-parents-merge-destination.md`, `docs/adrs/0095-native-supervisor-delivers-a-real-session-and-an-exact-terminal-record.md` |
<!-- docket:artifacts:end -->

## Why

The migration is not functionally complete until merged and interrupted work converges safely to
the correct terminal state in both repository modes.

## What changes

Add authoritative finalize context and resumable Go operations for local rebase/retest, rewritten
head publication, merge verification, atomic terminal archive and stack close-out, explicit and
policy-driven reclaim, durable halted/finalize-blocked recovery, merged-PR maintenance, generated
terminal-link repair, and ownership-safe run/workspace/branch cleanup.

## Implementation launch prerequisites

Changes 0322 and 0326 are explicit dependencies, not behavior to implement here. Change 0322 makes
the reviewed source bootstrap install and adopt a permanent Go development runtime; change 0326
contracts Docket's active deferred configuration before the first Go metadata mutation. The
migration owner then runs the source bootstrap from reviewed merged `origin/main`, verifies
`docket install check` and `docket diagnostic config --for-mutation`, and restarts the host before
dispatching this change.

The transient `go run` bootstrap may perform only `development install`. It is never used for
`context`, `change`, `workspace`, `evidence`, `pr`, `run`, or `finalize` against shared Docket
state. Change 0316 starts only through the installed, verified binary after both dependencies are
done; a failed install/config check remains an abort-and-report precondition failure.

## Out of scope

Behavior owned by changes 0305 through 0315; release packaging and four-harness acceptance from
0317; remaining self-hosting, Bash removal, and hard cutover from 0318; and
deferred CI/combined gates, results-only skips, terminal publishing, automatic learning harvest,
capture/groom automation, cross-harness routing, skill rebinding, or Bash fallback behavior. This
change also does not implement the source bootstrap or legacy adoption from 0322, contract
configuration from 0326, add Go verbs to `docket.sh`, authorize a source-built binary to mutate
Docket's live metadata, or bypass the unsupported-capability fence.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

## Run halted — resolved 2026-08-18

2026-08-18 — An autonomous docket-implement-next run scoped to change 0316 halted at its
implementation launch precondition, before Step 2 (claim) and before any claim landed. No claim,
branch, worktree, plan, or code was created. **This blocker has since been cleared; the change is
dispatchable again.** The record is kept, with its one factual error corrected, because the
correction matters more than the halt.

An earlier halt on this change recorded a malformed-frontmatter blocker (ADR-0024's unquoted
`title:` failed the strict Go YAML parser, which validates the whole metadata repository before any
mutation). That was repaired separately; `docket status` reports 0 errors.

**The real blocker was a missing installation record**, not an unbuilt release binary:

- `docket install check` → `install.check: invalid-state`, `reason: installation-required`,
  `message: install: no installation record`. Nothing at
  `/Users/homer/.local/share/docket/state/install.json`.
- `docket diagnostic config --for-mutation --repo-dir <repo>` reported `configuration: valid` /
  `mutation: allowed` throughout — the config contraction from 0326 was never implicated.

**Correction.** The superseded text of this record claimed the fix required a binary whose
`docket version` reports "a real installed identity (not `development`)". That is wrong in a way
that sends a reader in a circle. This change's own prerequisites call for "a permanent Go
**development** runtime" (0322), and `internal/buildinfo/buildinfo.go` states that release
packaging belongs to change 0317 — which is `waiting-dependency` on **this change**. Requiring a
release-stamped binary to build 0316 is therefore circular and unsatisfiable. A `development`
version string is expected and correct. What "the installed, verified binary" means here is the
binary installed *through docket's own journaled install transaction*, not a release build.

**Resolution.** `./install.sh` (resolving to `docket development install --source <checkout>`) run
from a clean checkout at merged `origin/main`. Its first attempt refused with
`reason: ownership-conflict` — 18 targets "not provably docket's", left by the legacy Bash
installer, which 0322 does not adopt automatically. After moving those aside (17 generated
`~/.claude/agents/docket-*.md`, plus a `~/.agents/skills/docket-convention` symlink that had been
dangling into the deleted `.worktrees/convention-extraction-skill/` since 2026-06-10), the install
completed. Post-state: `install.check: no-op`, `mutation: allowed`, 17 agents regenerated, and the
convention symlink repointed into the content-addressed version store.
