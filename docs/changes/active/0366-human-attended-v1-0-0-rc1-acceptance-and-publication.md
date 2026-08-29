---
id: 366
slug: 'human-attended-v1-0-0-rc1-acceptance-and-publication'
title: 'Human-attended v1.0.0-rc1 acceptance and publication'
status: 'proposed'
priority: 'critical'
type: 'chore'
created: '2026-08-29'
updated: '2026-08-29'
depends_on: [370]
stacked_on:
related: [317, 318, 322, 326, 352, 361, 363, 369, 370]
discovered_from: [318]
adrs: [95, 96, 99]
spec:
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
| ADRs | [ADR-0095](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0095-native-supervisor-delivers-a-real-session-and-an-exact-terminal-record.md), [ADR-0096](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0096-legacy-reproduction-uses-a-frozen-embedded-floor.md), [ADR-0099](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0099-one-metadata-topology-for-go-v1.md) |
<!-- docket:artifacts:end -->

## Why

Changes 0318, 0369, and 0370 will make the reviewed source Go-only through three independently
mergeable stages, but source acceptance is not release acceptance. The exact merged 0370 candidate
still needs human-attended proof across native targets and genuinely fresh harness processes, an
isolated rollback rehearsal, explicit migration-ledger dispositions, and irreversible publication.

## What changes

- Audit and disposition the complete active migration backlog through the installed Go product, preserving independent post-Go work and recording manual migration learnings.
- Package one candidate from the exact merged 0370 commit and use the same checksum-identified bytes for all native tuple, self-host, rollback, publication, and public-install gates.
- Run Darwin/Linux on amd64/arm64 native smokes, then complete the retained mutating lifecycle plus restart/resume recovery in genuinely fresh Claude, Codex, Cursor, and OpenCode sessions.
- Rehearse an isolated v0.9.2 rollback without introducing a runtime fallback.
- At an explicit human irreversible boundary, create and verify the v1.0.0-rc1 tag, GitHub Release, assets, checksums, and clean public installation.
- Collate durable evidence and close the migration/release metadata only after every gate passes.

## Human-attended protocol to preserve

This stub deliberately has no linked spec and is not build-ready. When a human opens the release
session, grooming must preserve the following protocol rather than collapsing it into a normal
single-PR implementation:

1. **Establish the release source.** Verify changes 0318, 0369, and 0370 are merged, the integration branch is clean,
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

## Open questions

- What exact operator checklist and quiescence proof will bracket the post-0370 release window?
- Which disposable Git hosting arrangement and transcript location will supply durable evidence
  without leaking credentials, private paths, or unrelated user configuration?
- What is the exact lifecycle fixture and success predicate shared by all four harnesses while
  still proving each harness's native loading and dispatch boundary?
- Which active backlog items are expected migration-ledger candidates when the audit begins, and
  what human decision rule settles items that mix a retired mechanism with an independent product
  invariant?
- What are the exact retry rules after a matching tag exists but the Release or asset set is only
  partially present?
- How are the three source changes' terminal states and this successor's metadata sequenced so no normal sweep closes
  the source change before its release evidence is durably attached?
- What final release-note language distinguishes the first Go candidate, the four supported native
  targets and harnesses, the two retained POSIX products, and the separate `v0.9.2` rollback path?

## Out of scope

Source-cutover implementation owned by 0318, 0369, and 0370; changes to the accepted candidate after
packaging; rebuilding or substituting bytes mid-protocol; a Bash fallback or compatibility launcher;
stable v1.0.0 promotion; Homebrew, Windows, signing/notarization, SBOM or provenance signing;
uninstall or version-tree garbage collection; and redesign of Docket storage, JSON protocol, harness
topology, or Git/GitHub adapters. A source defect found during acceptance returns to a separate
reviewed code change and invalidates the candidate rather than being repaired inline.
