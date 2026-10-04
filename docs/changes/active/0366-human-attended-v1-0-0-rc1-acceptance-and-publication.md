---
id: 366
slug: 'human-attended-v1-0-0-rc1-acceptance-and-publication'
title: 'v1.0.0-alpha.1 acceptance and publication (Claude Code)'
status: 'proposed'
priority: 'critical'
type: 'chore'
created: '2026-08-29'
updated: '2026-10-04'
depends_on: [502, 511]
stacked_on:
related: [317, 318, 322, 326, 352, 361, 363, 369, 370, 371, 372, 374, 377, 384, 392, 393, 394, 399, 401, 412, 433, 510, 512, 513]
discovered_from: [318]
adrs: [95, 96, 99, 100, 102, 103, 104]
spec: 'docs/superpowers/specs/2026-09-04-human-attended-v1-0-0-rc1-acceptance-and-publication-design.md'
plan:
results:
trivial: false
auto_groomable: false
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-04-human-attended-v1-0-0-rc1-acceptance-and-publication-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-04-human-attended-v1-0-0-rc1-acceptance-and-publication-design.md) |
| ADRs | [ADR-0095](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0095-native-supervisor-delivers-a-real-session-and-an-exact-terminal-record.md), [ADR-0096](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0096-legacy-reproduction-uses-a-frozen-embedded-floor.md), [ADR-0099](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0099-one-metadata-topology-for-go-v1.md), [ADR-0100](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0100-native-host-dispatch-is-authoritative-for-registered-docket.md), [ADR-0102](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0102-build-and-finalize-own-independent-gate-and-test-command-con.md), [ADR-0103](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0103-enter-codex-coordinator-roles-through-app-server-root-thread.md), [ADR-0104](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0104-the-capability-catalog-is-the-authoritative-executable-cli-s.md) |
<!-- docket:artifacts:end -->

## Why

The Go-only source cutover is complete and many changes have merged since, but no Go binary has ever been published. The latest tag is still `v0.9.3`, a Bash-era release with no assets.

This change takes one reviewed commit of `main` to the first public pre-release, **`v1.0.0-alpha.1`**, with a human present at each irreversible step. It is aimed at the few known Bash-era users, and they upgrade with change 0511's guide.

On 2026-10-04 the human re-scoped the release:
- It is renamed from `v1.0.0-beta1`.
- Claude Code is the only human-tested harness. Cursor follows in alpha.2 (0512) and OpenCode in alpha.3 (0513); Codex stays paused.
- The fresh macOS user, the rollback rehearsal, the recorded smoke evidence, the separate upgrade probes, and the backlog audit are dropped.

The linked spec was rewritten to match. The three older protocol sections below `## Out of scope` (*Human-attended protocol to preserve*, *Required evidence*, *Failure and retry boundary*) predate both grooms. **The linked spec supersedes them.**

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

## Human-attended protocol to preserve

This stub deliberately has no linked spec and is not build-ready. When a human opens the release
session, grooming must preserve the following protocol rather than collapsing it into a normal
single-PR implementation:

1. **Establish the release source.** Verify changes 0318, 0369, 0371, 0372, and 0370 are merged, the integration branch is clean,
   the Go-only source gate passed on the exact merge commit, and no later source commit is being
   silently substituted.
2. **Close the migration ledger.** Read every active backlog item rather than bulk-editing by
   filename or keyword. Kill Bash-mechanism-only work, link surviving product invariants to their
   landed Go owners, leave deferred Go work deferred, preserve independent post-Go work, and stop
   for human disposition on any ambiguous proposal. Perform mutations and board refreshes through
   the installed Go product. Record migration learnings manually through the Go learning workflow.
3. **Package once.** Dispatch change 0317's candidate workflow for `v1.0.0-rc1` at the exact 0370
   merge commit. Preserve the workflow run identity, toolchain identity, archive checksums, and one
   immutable downloaded copy. A source change invalidates the candidate and returns the work to a
   new reviewed code change.
4. **Run native tuple smokes.** Prove the same archives on Darwin and Linux, each on amd64 and
   arm64. Per-target rebuilding is not equivalent evidence.
5. **Run four fresh-host self-host scenarios.** Install the accepted bytes into isolated roots and
   start genuinely fresh native Claude, Codex, Cursor, and OpenCode processes. Each gets its own
   disposable clone, isolated HOME/XDG state, isolated remote, and newly loaded generated assets.
   Drive one complete retained mutating lifecycle—including repository init/check, groom, claim,
   reconcile, build, evidence, PR, finalize, archive, and restart/resume recovery—and record the
   observed named-agent child and terminal state.
6. **Rehearse rollback.** In a separate isolated copy, install and exercise frozen `v0.9.2` using
   its embedded compatibility floor. This proves an independent rollback procedure, never an
   in-process fallback or compatibility launcher in the Go candidate.
7. **Publish at the human boundary.** Probe GitHub before every effect. Create the immutable
   `v1.0.0-rc1` tag at the accepted commit, create the GitHub Release, upload the already accepted
   archives, checksum manifest, and downloader, and verify the remote objects exactly match the
   recorded identities. Conflicting existing objects are a stop, not an overwrite opportunity.
8. **Verify public installation.** Download through the public release path, verify checksums and
   build identity, complete a clean installation, and run the native install check. A public URL
   test verifies exposure of accepted bytes; it does not authorize a rebuild.
9. **Close out durably.** Store sanitized evidence references, complete the release notes and
   metadata closeout through the installed Go product, then allow the normal maintenance sweep.

## Required evidence

| Gate | Minimum durable evidence |
|---|---|
| Source identity | Exact merged 0370 commit, Go/toolchain identity, canonical suite command and outcome, inventory result, budget findings |
| Candidate identity | Workflow run, one SHA-256 per archive/manifest/downloader, proof every later gate used those bytes |
| Native targets | Darwin/Linux × amd64/arm64 result rows with installed binary identity |
| Fresh harnesses | Claude/Codex/Cursor/OpenCode version and mode, isolated paths and remote, child-agent proof, lifecycle terminal state, restart/resume outcome, sanitized transcript location |
| Migration ledger | Every active-item disposition, successor links where applicable, manual Go learning records, no ambiguous proposed item left silently unresolved |
| Rollback | Isolated `v0.9.2` source and checksum identity, installation and recovery outcome, proof no candidate fallback was introduced |
| Publication | Remote tag target, GitHub Release identity, asset list and checksums, public downloader result, clean-install and `docket install check` outcome |

External host behavior, process-start asset loading, GitHub publication, and subjective backlog
dispositions remain human-verified truth. Generated files, status summaries, or a child agent's
claim cannot substitute for direct observation. Missing or ambiguous evidence fails the gate.

## Failure and retry boundary

- Stop before publication on any failed source, candidate, tuple, harness, lifecycle, rollback,
  checksum, ledger, or evidence gate.
- Resume only from authoritative Git/GitHub probes and the recorded candidate checksums. Local
  files, elapsed time, or a previously attempted command do not establish success.
- Never automatically compensate a published external effect. After partial publication, probe
  each remote object and continue only when its identity exactly matches the accepted candidate.
- Never repair source inline during acceptance. Any required source change gets its own reviewed
  change, invalidates all candidate evidence, and restarts packaging from the new merged commit.
- Do not run a maintenance sweep during the bounded interval between merging 0370 and completing
  this release metadata closeout.

## Out of scope

- Source changes of any kind. A defect goes to its own reviewed change and invalidates the candidate.
- Changing or substituting accepted bytes; a Bash fallback.
- Stable `v1.0.0`; alpha.2 and alpha.3.
- Cursor, OpenCode and Codex testing or support claims.
- A fresh macOS user account; a rollback rehearsal; four-platform smoke evidence; Bash-install upgrade probes; a backlog or migration-ledger audit.
- Homebrew, Windows, signing/notarization, SBOM.
- A publishing workflow.
- Any redesign of storage, the JSON protocol, harness topology, or the Git/GitHub adapters.
