---
id: 511
slug: 'upgrade-guide-from-bash-docket-to-the-go-binary-proven-on-sa'
title: 'Upgrade guide from Bash docket to the Go binary, proven on saved v0.9.2 and v0.9.3 installs'
status: 'in-progress'
priority: 'critical'
type: 'docs'
created: '2026-10-04'
updated: '2026-10-04'
depends_on: []
stacked_on:
related: [366, 502, 464, 392, 322, 363, 377, 409, 323]
discovered_from: [366]
adrs: [96, 99]
spec: 'docs/superpowers/specs/2026-10-04-upgrade-guide-from-bash-docket-to-the-go-binary-proven-on-sa-design.md'
plan:
results:
trivial: false
auto_groomable: false
branch_prefix:
branch: 'docs/upgrade-guide-from-bash-docket-to-the-go-binary-proven-on-sa'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-04T17:59:48Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-04-upgrade-guide-from-bash-docket-to-the-go-binary-proven-on-sa-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-04-upgrade-guide-from-bash-docket-to-the-go-binary-proven-on-sa-design.md) |
| ADRs | [ADR-0096](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0096-legacy-reproduction-uses-a-frozen-embedded-floor.md), [ADR-0099](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0099-one-metadata-topology-for-go-v1.md) |
<!-- docket:artifacts:end -->

## Why

A few known users still run Bash docket, installed from the v0.9.2 or v0.9.3 tags, with repositories on the `docket` branch. Nothing tells them how to move to the Go binary, and nothing proves that the move works:

- The Go installer only takes over a v0.9.2 install automatically. A v0.9.3 install will probably stop with an ownership conflict, and there is no `--force`.
- No document describes the repository path for a repo already on the `docket` branch. `repository migrate` only handles the old single-branch layout.
- Some old `.docket.yml` settings now block writes.
- No test has ever run a real Bash-made repository through the Go binary. The `v0.9.x` fixtures in `testdata/repositories/` hold config files only.

The first public Go build, `v1.0.0-alpha.1` (change 0366), is aimed at exactly these users. 0366's spec left the install and upgrade docs to a separate change, and this is that change. 0366 will depend on it.

## What changes

- **Saved Bash state.** Build it once from the real v0.9.2 and v0.9.3 tags, in a sandbox: a throwaway home folder and a local stand-in for GitHub. For each tag, save:
  - a small repository on the `docket` branch as a git bundle, so its exact history is kept;
  - the harness files that tag's install left in the home folder, for Claude Code, Cursor and OpenCode, so alpha.2 and alpha.3 can reuse them;
  - a provenance note saying how it was made.
- **Two configs on purpose.** The v0.9.2 repository keeps the default config and proves the common path. The v0.9.3 repository sets every then-valid setting that Go now refuses or warns about, and proves the clean-up path.
- **A proof test.** An integration-tagged Go test restores each saved case and runs the guide's own commands against it. It must end with:
  - a clean install check, a clean repository check and zero status errors;
  - one successful write;
  - every pre-existing record intact.
  For v0.9.3 it first sees the ownership conflict, then proves the guide's hand remedy clears it. Only Claude Code is asserted for now.
- **The guide,** at `docs/release/upgrading-from-bash.md` (the one docs folder where the living-docs guard allows naming the refused settings). It covers:
  - before you start: finish or park in-flight changes;
  - installing the Go binary, from the release download or a checkout (the checkout route needs Go);
  - taking over the old Claude Code install, including the ownership-conflict remedy;
  - upgrading each repository;
  - a table of `.docket.yml` settings that changed;
  - leftovers you can delete;
  - restarting Claude Code;
  - one factual line that the Bash tags v0.9.2 and v0.9.3 remain available (no rollback instructions, because rollback is not tested).
  `README.md` and `docs/install/install.md` link to it.
- **Temporary by design.** The saved cases and the test are retired when stable v1.0.0 ships. The guide then stays as a frozen document.
- **Code bugs stay out.** If the test exposes something the binary does wrong, as opposed to something the guide can explain, the build halts. That bug gets its own fix change, and this change waits on it.

## Out of scope

- Repositories on the old single-branch layout (the `repository migrate` path).
- Guide sections and test assertions for Cursor and OpenCode. Those come with alpha.2 and alpha.3; their files are only saved here. Codex is paused.
- Teaching the installer to recognize v0.9.3 installs (widening ADR-0096's frozen floor). The guide's hand remedy covers it.
- Users who tracked `main` between v0.9.3 and the Go cutover.
- The alpha.1 release notes, which belong to 0366.
- Rollback instructions or a rollback test.
- Any product code change. A defect gets its own change.

## Reconcile log

### 2026-10-04

2026-10-04 — Reconciled against main @ 7ddda0471 (81 commits past the spec's traced 6908393f3). No commit since then touches internal/install, internal/reposetup, internal/config, install.sh or docs/install/install.md; README.md and docs/release/ gained only unrelated lines (four-harness-acceptance.md). Tags v0.9.2 and v0.9.3 exist locally. Scope unchanged; the spec's traced facts stand, and the build still re-verifies each and halts on a binary defect per spec section 4.
