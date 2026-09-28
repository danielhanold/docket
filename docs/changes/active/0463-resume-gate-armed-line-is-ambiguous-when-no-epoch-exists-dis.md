---
id: 463
slug: 'resume-gate-armed-line-is-ambiguous-when-no-epoch-exists-dis'
title: 'Resume gate-armed line is ambiguous when no epoch exists — dispatch context gets passed as --run-epoch'
status: 'implemented'
priority: 'high'
type: 'fix'
created: '2026-09-27'
updated: '2026-09-28'
depends_on: []
stacked_on:
related: [345, 375, 359, 441, 435]
discovered_from: [382]
adrs: [111, 118, 128]
spec: 'docs/superpowers/specs/2026-09-27-resume-gate-armed-line-is-ambiguous-when-no-epoch-exists-dis-design.md'
plan: 'docs/superpowers/plans/2026-09-27-resume-gate-armed-line-is-ambiguous-when-no-epoch-exists-dis.md'
results: 'docs/results/2026-09-27-resume-gate-armed-line-is-ambiguous-when-no-epoch-exists-dis-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/resume-gate-armed-line-is-ambiguous-when-no-epoch-exists-dis'
pr: 'https://github.com/danielhanold/docket/pull/345'
blocked_by:
reconciled: true
claimed_at: '2026-09-27T21:29:12Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-27-resume-gate-armed-line-is-ambiguous-when-no-epoch-exists-dis-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-27-resume-gate-armed-line-is-ambiguous-when-no-epoch-exists-dis-design.md) |
| Plan | [2026-09-27-resume-gate-armed-line-is-ambiguous-when-no-epoch-exists-dis.md](https://github.com/danielhanold/docket/blob/fix/resume-gate-armed-line-is-ambiguous-when-no-epoch-exists-dis/docs/superpowers/plans/2026-09-27-resume-gate-armed-line-is-ambiguous-when-no-epoch-exists-dis.md) |
| Results | [2026-09-27-resume-gate-armed-line-is-ambiguous-when-no-epoch-exists-dis-results.md](https://github.com/danielhanold/docket/blob/fix/resume-gate-armed-line-is-ambiguous-when-no-epoch-exists-dis/docs/results/2026-09-27-resume-gate-armed-line-is-ambiguous-when-no-epoch-exists-dis-results.md) |
| ADRs | [ADR-0111](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0111-run-gate-attribution-binds-a-dispatch-to-its-successful-clai.md), [ADR-0118](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0118-worktree-wide-gate-admission-and-explicit-human-cancellation.md), [ADR-0128](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0128-resume-arms-mint-an-arm-time-epoch-that-run-cancel-can-cance.md) |
<!-- docket:artifacts:end -->

## Why

During change 0382's resumed implement-next run (2026-09-27), the build suite gate refused `docket gate drive start --run-epoch <token>` with a bare `{"result":"invalid-input","reason":"invalid-request"}`. The child dropped the flag and the same call without it was `applied`, so the run finished (`gate-done … run-complete 382`, PR #342), but it ran with no epoch linkage and the child reported the flag itself as broken. It is not: `gate.drive.start` does accept `--run-epoch`.

**Root cause (traced and confirmed):**

1. **Unarmed first dispatch → no epoch ever bound.** The coordinator dispatched the first 0382 run without running `run.gate-before`. That run halted at Task 8. Nothing ever minted a run epoch for change 382.
2. **Resume arm falls through to the epochless path.** `run.gate-before implement-next --resume 382` calls `FindEpochByChange`, finds none, and takes the "no prior epoch (a legacy/pre-epoch resume, or an unclaimed run that never bound one)" branch in `RunGateBefore` (`internal/app/rungate_before.go`). That branch arms a gate and a recovery scope but mints no epoch, so `RunGateBeforeResult.Epoch` is empty.
3. **The human-readable line becomes ambiguous.** `RunGateBeforeResult.HumanText` prints `gate-armed <key>` + (`<epoch>` only if non-empty) + `<dispatch-context>`. With no epoch it printed `gate-armed implement-next-20260927t175007z-1115-e516 0790b760e26444866ef2e156ba383326` — two tokens, and the second is the **dispatch context**, not an epoch. AGENTS.md's run-gate rule documents the line as always `gate-armed <key> <epoch> <dispatch-context>`, and both values are 32-hex strings, so the parent reads the second token as the epoch. Proof: `sha256(0790b760…)` equals the stored record's `child_context_hash` (`.git/docket/rungate/<key>/record.json`), and the record carries no epoch.
4. **Misrouted token, opaque refusal.** The parent put that token in the dispatch prompt labelled as the run epoch and told the child that no dispatch context was printed. The child passed a child capability as `--run-epoch`, and the dispatch context was never used as `--gate-context`. The driver rejected the unknown epoch, and `mapDriveFailure` (`internal/app/gate_drive.go`) turned the unclassified error into the generic `invalid-request`. With no stable reason token, neither agent could tell a bad epoch from a malformed request.

**Why it matters:**

- **Silent loss of linkage.** Run-epoch linkage (change 0375) is what lets `run.cancel` fence and tear down a run's drives and what stops an omitted or stale epoch from detaching a workflow-owned worktree. Here the whole resumed run went without it, with no warning.
- **Wrong blame.** The refusal was reported as "the `--run-epoch` flag is refused", which blames the wrong layer.
- **Credential in the wrong channel.** The dispatch context is a credential-shaped capability that is meant for the child only. Passing it where a locator (`--run-epoch`) is expected is exactly the confusion the gate's token design is meant to prevent.
- **Likely to recur.** Any resume of a change whose earlier run was unarmed, pre-epoch, or claimed through a slash command (see 0345) hits the same two-token line.

## What changes

When a resume has no prior run epoch, make the arm always mint one, so the `gate-armed` line always has exactly three tokens and a misused epoch fails loudly and specifically.

- **Every armed gate carries an epoch.** When `run.gate-before --resume <id>` finds no prior epoch for the change, it now mints one, as a fresh arm does. The epoch is bound to the change id and to the verified feature worktree. This makes the documented `gate-armed <key> <epoch> <dispatch-context>` line true on every armed path and restores `run.cancel`/fence coverage for resumed runs. A mint or bind failure fails closed as `gate-unarmed mint-failed`.
- **The human line is always three tokens.** The optional-epoch branch in `HumanText` is removed. Stale code comments and the `gate-before` CLI help text, which describe the old two-token form, are corrected. The parent-facing prose (AGENTS.md, cursor run-gate rule) already documents the three-token form and needs no edit; the build confirms this with a repo-wide grep. The JSON shape and the positional contract are unchanged.
- **Unknown epochs get a named refusal.** A `--run-epoch` naming no known epoch now refuses with `unknown-run-epoch` and a short next-action hint, instead of the catch-all `invalid-request`. This applies to `gate drive start`, `gate drive prepare-scope`, and `agent.enter`. Other epoch-registry faults surface their own kind.
- **Regression tests, mutation-checked:** epochless-resume arm shape, the human-line positional invariant, the repeat resume refused as active, the unknown-epoch token, and an end-to-end repro of the 0382 sequence.

## Out of scope

- **Unarmed first dispatches.** The fact that the first 0382 run was dispatched unarmed was a coordinator mistake against an existing rule, not a docket defect. Enforcing arm-before-dispatch for slash-command or agent launches is change 0345's territory.
- **Epochs for completed runs.** Retroactively minting epochs for already-completed runs, or repairing gate records from earlier runs.
- **Retiring the positional line.** Moving parents to `run gate-before --json` in place of the positional `gate-armed` line: unnecessary once the line is always three tokens.
- **Credential detection.** Detecting a dispatch-context or credential hash presented as `--run-epoch`: the parse ambiguity that caused it is removed instead.
- **A per-change resume lock.** Two concurrent epochless resumes of one change can each mint an epoch. The result fails safe (later resumes refuse as `resume-epoch-unreadable`), so no new lock is added.
- **Other catch-all refusals.** Redesigning the gate-drive refusal vocabulary beyond the epoch-related cases above.
- **Agent spelling bug in the same run.** The resumed run's first `gate drive start` also omitted `--change-id`/`--phase`/`--gate-context`. That is the child's call-site spelling, not the refusal cause.

## Reconcile log

### 2026-09-27

2026-09-27 — Reconciled against origin/main 86f14149. Traced internal/app/rungate_before.go: the epochless resume branch is still present (step 6a mints only when resumeID == 0; HumanText still omits an empty epoch). Recent 0446/0459 commits touched resume replacement and scope-transfer paths but not the epochless branch or mapDriveFailure EpochError handling. Scope and spec unchanged.

