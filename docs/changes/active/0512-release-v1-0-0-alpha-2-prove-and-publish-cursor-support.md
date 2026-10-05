---
id: 512
slug: 'release-v1-0-0-alpha-2-prove-and-publish-cursor-support'
title: 'Release v1.0.0-alpha.2: prove and publish Cursor support'
status: 'proposed'
priority: 'high'
type: 'chore'
created: '2026-10-04'
updated: '2026-10-05'
depends_on: [366]
stacked_on:
related: [366, 511]
discovered_from: []
adrs: []
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
<!-- docket:artifacts:end -->

## Why

The human decided to grow the Go pre-releases one harness at a time:

- `v1.0.0-alpha.1` (change 0366) is human-tested on Claude Code only.
- Cursor comes next, in alpha.2.
- OpenCode follows in alpha.3.

Until alpha.2 ships, Cursor users have no tested release and no upgrade steps for their Cursor files.

## What changes

- **Upgrade guide.** Add the Cursor section to the Bash upgrade guide from change 0511, and add the Cursor assertions to its test. The saved v0.9.2 and v0.9.3 cases already hold the Cursor files.
- **Human test.** Run the full lifecycle in a fresh Cursor process (IDE), as 0366 does for Claude Code.
- **Release.** Package, verify and publish `v1.0.0-alpha.2` with a lighter version of 0366's protocol: the new harness row plus the release basics. Settle the exact trimmed protocol at grooming.
- **Fold in the protocol gaps alpha.1 hit (change 0366).** The trimmed protocol should state each of these up front instead of discovering them mid-run:
  - *Installing the candidate before publication.* `sh <copy>/install.sh` always downloads from the release URL, which doesn't exist yet. Install from the read-only copy through `DOCKET_RELEASE_BASE_URL=file://<mirror>`, where `<mirror>/<version>` points at the copy.
  - *Test-home Git credentials.* With `HOME` pointed at the test home, `git push` over HTTPS prompts for credentials. Run `gh auth login` and then `gh auth setup-git` inside the test home. The operator's SSH keys are not there.
  - *Fixture setup.* `gh repo create --clone` gave a `master` branch with nothing pushed. Rename it to `main` and push before `repository init`. `init` wrote `gate: off` and `configure-tests` would not change it, so the local gates were set by hand (see the change filed for that).
  - *The kill window.* The record's `plan:` field is set well after the plan commit lands, so polling for `plan:` missed the window and the kill came during review. Key the window on a signal that appears in time, such as the plan commit on the feature branch or the record going `in-progress`.
  - *Verdict lines.* The printed `run-*` lines from the kill and resume weren't captured, and the session transcripts don't contain them. Say who copies them, and when.
  - *Closeout with no code.* `change.mark-implemented` refuses without a reconciled record (`not-reconciled`) and a linked plan (`plan-unlinked`). `change.attach-plan` needs a plan-only commit carrying a `Docket-Plan-Path:` trailer. Plan for a reconcile and a pointer plan from the start.
  - *PR evidence after implemented.* After `mark-implemented`, `pr.publish` returns `contended`, so the PR body kept stale evidence until `evidence recertify` refreshed it. Order the closeout so the final head is gated and published before marking implemented.
  - *Private fixture and finalize.* Finalize's merge refused on the private fixture (branch-rules API), and the operator merged by hand. Use a fixture where finalize can merge, or track the fix (see the finalize change filed from 0366).

## Out of scope

- OpenCode (alpha.3).
- Codex (paused).
- Re-proving Claude Code beyond what the shared protocol already re-runs.
