---
id: 366
slug: 'human-attended-v1-0-0-rc1-acceptance-and-publication'
title: 'v1.0.0-alpha.1 acceptance and publication (Claude Code)'
status: 'implemented'
priority: 'critical'
type: 'chore'
created: '2026-08-29'
updated: '2026-10-05'
depends_on: [502, 511]
stacked_on:
related: [317, 318, 322, 326, 352, 361, 363, 369, 370, 371, 372, 374, 377, 384, 392, 393, 394, 399, 401, 412, 433, 510, 512, 513]
discovered_from: [318]
adrs: [95, 96, 99, 100, 102, 103, 104]
spec: 'docs/superpowers/specs/2026-09-04-human-attended-v1-0-0-rc1-acceptance-and-publication-design.md'
plan: 'docs/superpowers/plans/2026-10-04-human-attended-v1-0-0-alpha-1-acceptance-and-publication-plan.md'
results: 'docs/results/2026-10-04-human-attended-v1-0-0-rc1-acceptance-and-publication-results.md'
trivial: false
auto_groomable: false
branch_prefix:
branch: 'chore/human-attended-v1-0-0-rc1-acceptance-and-publication'
pr: 'https://github.com/danielhanold/docket/pull/390'
blocked_by:
reconciled: true
claimed_at: '2026-10-05T01:15:14Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-04-human-attended-v1-0-0-rc1-acceptance-and-publication-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-04-human-attended-v1-0-0-rc1-acceptance-and-publication-design.md) |
| Plan | [2026-10-04-human-attended-v1-0-0-alpha-1-acceptance-and-publication-plan.md](https://github.com/danielhanold/docket/blob/chore/human-attended-v1-0-0-rc1-acceptance-and-publication/docs/superpowers/plans/2026-10-04-human-attended-v1-0-0-alpha-1-acceptance-and-publication-plan.md) |
| Results | [2026-10-04-human-attended-v1-0-0-rc1-acceptance-and-publication-results.md](https://github.com/danielhanold/docket/blob/chore/human-attended-v1-0-0-rc1-acceptance-and-publication/docs/results/2026-10-04-human-attended-v1-0-0-rc1-acceptance-and-publication-results.md) |
| ADRs | [ADR-0095](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0095-native-supervisor-delivers-a-real-session-and-an-exact-terminal-record.md), [ADR-0096](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0096-legacy-reproduction-uses-a-frozen-embedded-floor.md), [ADR-0099](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0099-one-metadata-topology-for-go-v1.md), [ADR-0100](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0100-native-host-dispatch-is-authoritative-for-registered-docket.md), [ADR-0102](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0102-build-and-finalize-own-independent-gate-and-test-command-con.md), [ADR-0103](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0103-enter-codex-coordinator-roles-through-app-server-root-thread.md), [ADR-0104](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0104-the-capability-catalog-is-the-authoritative-executable-cli-s.md) |
<!-- docket:artifacts:end -->

## Why


The Go-only source cutover is complete and many changes have merged since, but no Go binary has ever been published. The latest tag is still `v0.9.3`, a Bash-era release with no assets.

This change takes one reviewed commit of `main` to the first public pre-release, **`v1.0.0-alpha.1`**, with a human present at each irreversible step. It is aimed at the few known Bash-era users, and they upgrade with change 0511's guide.

On 2026-10-04 the human re-scoped the release:
- It is renamed from `v1.0.0-beta1`.
- Claude Code is the only human-tested harness. Cursor follows in alpha.2 (0512) and OpenCode in alpha.3 (0513); Codex stays paused.
- The fresh macOS user, the rollback rehearsal, the recorded smoke evidence, the separate upgrade probes, and the backlog audit are dropped.

The linked spec was rewritten to match and is the authoritative release protocol.

## What changes

- **Before the cut:** 0502 and 0511 are merged; 0510 is finished or held; no loops or other sessions are running; 0412 is recorded as a known gap.
- **Cut:** take one candidate from the tip of `main` and freeze `main` until closeout.
- **Package once:** run the release workflow for `v1.0.0-alpha.1` and require it all green. Its whole-suite gate includes 0511's Bash upgrade test. Keep one read-only, checksum-verified copy of the bundle that every later step uses.
- **Claude Code test:** in a throwaway home folder on the operator's own account, install the candidate and drive one full lifecycle on a disposable private repository: create, groom, implement with a kill and resume, finalize.
- **Publish:** at the human's explicit go-ahead, tag, draft the pre-release (not marked latest), upload the six verified files, and publish.
- **Public install check:** install from the real release URL with the checksum verified first.
- **Closeout:** collect the evidence under `docs/release/v1.0.0-alpha.1/` and close out through the normal PR and finalize path.

The release notes say:
- Claude Code is the only tested harness.
- The upgrade guide is the upgrade path; the notes link it.
- Known gaps: 0412 with its recovery steps, Cursor and OpenCode untested, Codex paused.
- The Bash tags `v0.9.2` and `v0.9.3` remain available.

## Out of scope

- Source changes of any kind. A defect goes to its own reviewed change and invalidates the candidate.
- Changing or substituting accepted bytes; a Bash fallback.
- Stable `v1.0.0`; alpha.2 and alpha.3.
- Cursor, OpenCode and Codex testing or support claims.
- A fresh macOS user account; a rollback rehearsal; four-platform smoke evidence; Bash-install upgrade probes; a backlog or migration-ledger audit.
- Homebrew, Windows, signing/notarization, SBOM.
- A publishing workflow.
- Any redesign of storage, the JSON protocol, harness topology, or the Git/GitHub adapters.

## Reconcile log

### 2026-10-05

2026-10-04 — Human-attended run per the spec. Scope unchanged at claim: 0502, 0510 and 0511 merged, no open PRs, no other in-progress change. During the run the human dropped 0412 and the private-repo finalize merge block from the release notes known gaps (recorded in decisions.md). No plan artifact: the spec says there is no code to plan or build.
