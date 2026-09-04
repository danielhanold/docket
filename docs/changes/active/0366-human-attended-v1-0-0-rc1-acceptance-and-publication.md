---
id: 366
slug: 'human-attended-v1-0-0-rc1-acceptance-and-publication'
title: 'Human-attended v1.0.0-rc1 acceptance and publication'
status: 'proposed'
priority: 'critical'
type: 'chore'
created: '2026-08-29'
updated: '2026-09-04'
depends_on: [370]
stacked_on:
related: [317, 318, 322, 326, 352, 361, 363, 369, 370, 371, 372, 374, 377, 384, 392, 393, 394, 399, 401]
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

The Go-only source cutover is complete — 0318, 0369, 0371, 0372, and 0370 are all done, and
fourteen further changes have merged since — but source acceptance is not release acceptance. No
Go binary has been published: the latest tag is still `v0.9.3`, a Bash-era release with no assets.
The exact candidate commit still needs human-attended proof across the four native targets and
four genuinely fresh harness processes, a Bash-install upgrade probe, an isolated `v0.9.2` rollback
rehearsal, explicit migration-ledger dispositions, and then irreversible publication as
`v1.0.0-beta1` — the first public build of the Go product, released as a beta to gather feedback
from the existing user base before stable `v1.0.0`.

The three protocol sections that follow `## Out of scope` below predate this groom (2026-09-03)
and still name `v1.0.0-rc1` and the exact 0370 merge commit; the linked spec supersedes them,
re-anchored on the current tip of `main` and on `v1.0.0-beta1`.

## What changes

- Decide the pre-cut agenda with the human (0393, 0401, 0392, held PRs), then cut one candidate
  from the current tip of `origin/main` and hold a quiescence window until closeout.
- Close the migration ledger item by item through the installed Go product, using the program
  map's five disposition rules, and record migration learnings manually.
- Package once by dispatching the non-publishing candidate workflow for `v1.0.0-beta1` at the
  candidate commit; keep one immutable, checksum-identified copy that every later gate reads.
- Take the workflow's four native tuple smokes as tuple evidence, plus an operator-machine run.
- In a fresh host, install the accepted bytes and drive one complete retained mutating lifecycle —
  create, groom, implement with a restart/resume interruption, finalize, archive — through fresh
  Claude, Codex, Cursor, and OpenCode processes, each against its own disposable private remote.
- Probe upgrades from `v0.9.2` and `v0.9.3` Bash installs; rehearse an isolated `v0.9.2` rollback
  and a read-only cross-compatibility check, without any runtime fallback.
- At an explicit human boundary, create the annotated tag, the draft pre-release, the six verified
  assets, publish, and verify a clean public installation from the release URL.
- Collate the evidence bundle under `docs/release/v1.0.0-beta1/` and close out through the
  normal PR, finalize, and sweep path.

Design decisions (detail in the spec): the candidate is cut from the current tip of `main`, not
from 0370's merge commit, because the repository's own configuration and the skills' Step-0 verbs
already depend on changes merged after it. The pre-release is `v1.0.0-beta1` (dotted, as the
packager's version grammar requires); `rc` is reserved for a feature-frozen candidate of stable
`v1.0.0`. Publication stays human-typed `gh` at an explicit boundary — no workflow carries a write
token. Fresh-host proof runs in a fresh macOS user account so the Cursor IDE and the CLI harnesses
alike load only the candidate's assets, with one disposable private GitHub repository per harness
and one shared terminal predicate. The ledger audit table in the spec is the agenda; every rule-5
item waits for the human. `v0.9.2` remains the documented rollback artifact per the program map,
with `v0.9.3` recorded as the last Bash-era tag.

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

Source changes of any kind — a defect found during acceptance returns to a separate reviewed
change and invalidates the candidate; changes to the accepted candidate after packaging;
rebuilding or substituting bytes mid-protocol; a Bash fallback or compatibility launcher; stable
`v1.0.0` promotion; a tag-triggered publishing workflow; public-install documentation in the README
or `docs/guide/` (a separate docs change); widening ADR-0096's frozen corpus to `v0.9.3`;
Homebrew, Windows, signing/notarization, SBOM or provenance signing; uninstall or version-tree
collection (0323); and any redesign of storage, the JSON protocol, harness topology, or the
Git/GitHub adapters.
