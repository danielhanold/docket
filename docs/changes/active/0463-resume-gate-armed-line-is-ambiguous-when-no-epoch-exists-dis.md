---
id: 463
slug: 'resume-gate-armed-line-is-ambiguous-when-no-epoch-exists-dis'
title: 'Resume gate-armed line is ambiguous when no epoch exists — dispatch context gets passed as --run-epoch'
status: 'proposed'
priority: 'high'
type: 'fix'
created: '2026-09-27'
updated: '2026-09-27'
depends_on: []
stacked_on:
related: [345, 375, 359, 441]
discovered_from: [382]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
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

Make the resume arm's output unambiguous and make the misuse fail loudly and specifically. Candidate scope, to be settled at grooming:

- **Unambiguous human line.** When no epoch exists, `run.gate-before` must never print a positional line that can be read as `<key> <epoch> <dispatch-context>`. Options:
  - an explicit placeholder token (e.g. `gate-armed <key> no-epoch <dispatch-context>`)
  - a distinct disposition (e.g. `gate-armed-epochless`)
  - labelled fields
  - always minting a fresh epoch on resume when none exists, so the three-token contract always holds (preferred if safe, since it also restores `run.cancel`/fence coverage for the resumed run)
- **Consistent contract text.** Align the AGENTS.md / CLAUDE.md run-gate rule, `docket-implement-next`, and the `run gate-before` CLI help (`internal/cli/run.go`'s `Short` still says `gate-armed <key> <dispatch-context>`) with whatever the line becomes. Any prose that documents the 3-token form must also say how to recognise an epochless arm.
- **Specific refusal token.** When `gate drive start` / `gate drive prepare-scope` / `agent.enter` get a `--run-epoch` that names no known epoch, or is malformed, return a stable reason token such as `unknown-run-epoch` instead of the catch-all `invalid-request` from `mapDriveFailure`. Consider also detecting a value whose hash matches a known dispatch-context / child capability and refusing with a token that says so, without echoing the value.
- **Regression tests (mutation-tested per repo rules).** Cover: resume-with-no-prior-epoch output shape; the unknown-epoch refusal token; and that the epochless arm cannot be mis-parsed by the documented 3-token reader.

**Open questions for grooming:**

- Is minting a fresh epoch on a no-prior-epoch resume safe with respect to change 0375/0435's single-live-run and replacement-reservation invariants? Or must the epochless branch stay epochless and only its output change?
- Should `run.gate-before` gain a `--json` shape that the documented parent procedure reads instead of positional text, retiring the positional contract entirely?
- Should the driver distinguish "unknown epoch" from "epoch not owned by this worktree" in its refusal tokens?

## Out of scope

- **Unarmed first dispatches.** The fact that the first 0382 run was dispatched unarmed was a coordinator mistake against an existing rule, not a docket defect. Enforcing arm-before-dispatch for slash-command or agent launches is change 0345's territory.
- **Epochs for completed runs.** Retroactively minting epochs for already-completed runs, or repairing gate records from earlier runs.
- **Other catch-all refusals.** Redesigning the gate-drive refusal vocabulary beyond the epoch-related cases above.
- **Agent spelling bug in the same run.** The resumed run's first `gate drive start` also omitted `--change-id`/`--phase`/`--gate-context`. That is the child's call-site spelling, not the refusal cause; the identical call without `--run-epoch` was applied.
