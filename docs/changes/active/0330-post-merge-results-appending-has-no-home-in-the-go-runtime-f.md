---
id: 330
slug: 'post-merge-results-appending-has-no-home-in-the-go-runtime-f'
title: 'Optional closeout notes preserve post-merge verification without rewriting frozen results'
status: 'implemented'
priority: 'medium'
type: 'feat'
created: '2026-08-19'
updated: '2026-08-21'
depends_on: [316]
stacked_on:
related: [316, 331]
discovered_from: [316]
adrs: []
spec: 'docs/superpowers/specs/2026-08-21-terminal-closeout-notes-design.md'
plan: 'docs/superpowers/plans/2026-08-21-terminal-closeout-notes.md'
results:
trivial: false
auto_groomable:
branch: 'feat/post-merge-results-appending-has-no-home-in-the-go-runtime-f'
pr: 'github.com/danielhanold/docket#225'
blocked_by:
reconciled: true
claimed_at: '2026-08-21T02:43:01Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | `docs/superpowers/specs/2026-08-21-terminal-closeout-notes-design.md` |
| Plan | `docs/superpowers/plans/2026-08-21-terminal-closeout-notes.md` |
<!-- docket:artifacts:end -->

## Why

The Bash finalize workflow once told the agent to append interactive-verification outcomes and late
findings to `results:` after merge. Change 0316's Go rewrite dropped that instruction, exposing that
the information had no typed owner. Restoring the instruction literally would now be wrong: the
repository's later frozen-artifact rule makes a merged results file a point-in-time build record that
authored closeout prose must not rewrite.

Finalize still needs a safe place for verification outcomes or late findings already supplied when
the human invokes closeout. Without one, that context is discarded or invites an unowned edit to a
merged artifact. The terminal change record is the durable home: it can distinguish what the build
knew from what closeout learned while preserving both.

## What changes

Extend `docket finalize closeout` with an optional structured request containing exactly two lists:
`verification_outcomes` and `late_findings`. Go renders non-empty lists under `## Closeout notes`
with `### Verification` and `### Late findings`, then lands those notes in the same transaction as
the explicit change's terminal closeout. Empty input preserves today's closeout byte-for-byte.

Keep finalize single-step. The finalize skill teaches callers to include already-known notes in the
invocation and routes supplied context into the request; it never pauses after merge or adds a human
checkpoint. The request participates in closeout's idempotency receipt so a response-loss retry cannot
duplicate notes and a later request cannot rewrite the frozen terminal record.

Document the terminal section in the convention, leave the merged-results freeze rule unchanged,
replace the obsolete skipped append assertion with semantic Go coverage plus a mutation-proven skill
handoff guard, and regenerate the embedded skill assets mechanically.

## Out of scope

Editing or redesigning `results:`, changing `attach-results`, adding free-form closeout Markdown or a
third category, adding a post-merge pause or lifecycle state, automatically creating follow-ups or
harvesting learnings, and the capabilities 0316 deliberately deferred: terminal publishing,
CI/combined gates, results-only skips, skill rebinding, and Bash fallback.

## Reconcile log

### 2026-08-21

Reconciled against current main. Verified the spec's premises still hold: internal/cli/finalize.go registers a `finalize closeout` subcommand carrying `--id`/`--repo-dir` only (no `--input`); internal/app/finalize_closeout.go exposes `FinalizeCloseout(ctx, deps, repoDir, id)` with a scalar id and no request/notes payload in its CloseoutResult; render.ApplySectionEdits is H2-section-granular and marker/fence-aware (so a `## Closeout notes` body with `### Verification`/`### Late findings` subsections is authored as one section body, exactly as the spec anticipates); tests/test_results_artifact.sh still carries the skipped Bash-era post-merge-append assertion naming change 0330; and skills/docket-finalize-change/SKILL.md's `### 9. Closeout` invokes `docket finalize closeout --id <id>` with no notes ingress. Dependency 316 is done; sibling 331 (related) is already merged/done and addresses the evidence/gate-launch re-mint path, not closeout notes, so there is no scope overlap. Frozen-artifact rule (AGENTS.md / convention) unchanged and compatible with the design. No scope adjustment needed; relations left as-is.

## Finalize blocked

### 2026-08-21 — attempt 20260821T151615Z-11d14e56b224

<!-- attempt:20260821T151615Z-11d14e56b224 -->

- Reason: merge-denied
- Head: dbf6b6516a7a7192aad29d05ba467b30dedb6e20
- PR: #225
- Comment: https://github.com/danielhanold/docket/pull/225#issuecomment-5371802440

Remedy: A human must land the PR with an allowed method, then re-run finalize to close it out. Either: (1) `gh pr merge 225 --repo danielhanold/docket --rebase --match-head-commit dbf6b6516a7a7192aad29d05ba467b30dedb6e20`, then re-run `docket-finalize-change` naming id 330 so it archives the already-merged PR (merged-recovery); or (2) fix docket's merge method to use rebase/squash for repos where merge commits are disabled (internal/githubcli/merge.go hardcodes `--merge`), rebuild the binary, and re-run finalize naming id 330. Enabling allow_merge_commit on the repo would also unblock the current binary but conflicts with the repo's rebase-only convention.
