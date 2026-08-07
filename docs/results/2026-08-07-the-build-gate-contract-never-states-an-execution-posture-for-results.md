<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0223 — The build gate contract never states an execution posture for a suite that outgrows a single foreground call](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-07-0223-the-build-gate-contract-never-states-an-execution-posture-for.md)**
<!-- docket:backlink:end -->

# Gate execution posture — results

Change: #0223 · Branch: feat/the-build-gate-contract-never-states-an-execution-posture-for · PR: (see manifest) · Plan: docs/superpowers/plans/2026-08-07-the-build-gate-contract-never-states-an-execution-posture-for.md · ADRs: none

## Verify (human)

- [ ] **Re-probe any harness verdict you intend to rely on.** All four rows in
      `skills/docket-build/references/gate-execution.md` were measured on this machine on 2026-08-07
      (macOS 26.6.1 arm64) at codex-cli 0.146.1, cursor-agent 2026.08.04-aaa8809, opencode 1.18.14,
      claude 2.1.223. Verdicts are version-scoped by the file's own rule; nothing in the suite can
      re-verify them, because the oracle is outside the repo.
- [ ] **The `claude` verdict's unmeasured mode.** Its evidence is an interactive session across two
      foreground calls. The stricter variant — a non-interactive child observed from outside — was
      **not obtainable on this machine**: the permission classifier denied granting the child Bash
      access under both `--allowedTools Bash` and the bypass flag. The row now carries that scope
      explicitly rather than claiming the mode it did not measure. If you want the forked-mode
      verdict, that grant is the run that settles it.
- [ ] **`setsid` is absent on this machine.** Every probe used a `fork` + `POSIX::setsid` substitute.
      If you re-run the probes anywhere else, confirm the detach mechanism before reading a failure
      as a harness verdict — a launcher that is a process-group leader gets EPERM and the run is
      *inconclusive*, not `incompatible`. Two of grooming's Codex runs failed exactly this way.

## Findings

**The change's own thesis was demonstrated four times during its build.** The suite (80 files,
~10.5 min) sits past a single foreground call's ceiling. Four workers hit it; three stalled by
backgrounding the suite and yielding to await a completion event a subagent has no channel to
receive. One had to be discarded and its task rebuilt. By the end of the run the two-way split was
itself insufficient and the second half needed splitting again. This is the gap the change closes,
observed live rather than argued.

**Review findings — all nine fixed in-branch; see the PR body's disposition table for SHAs.** Three
are worth reading beyond their fix:

- **The blocker was real and this run proved it.** The drafted posture permitted the agent to yield
  while the gate ran, unqualified. But `docket-build` is invoked inline by `docket-implement-next`,
  which is itself forked — so the permission licensed exactly the stall that happened three times.
  The fix scopes the yield by *the observing agent's own dispatch posture*: available only to a
  top-level session agent that can receive a resumption signal; a dispatched or forked build role
  observes by **blocking**. The finite budget still bounds both branches, so fail-closed is
  unchanged, and ADR-0024 is not relaxed.
- **Two verdict rows claimed more than the probe measured.** `supported` was defined against six
  capabilities while the probe established three; capability 5's four-state distinction was never
  produced at all (the stand-in gate always succeeds), and capability 6 was not probed. Fixed by
  narrowing the token's definition where it is defined, with per-row scopes only on the two rows
  narrower than the shared bound.
- **One inherited spec claim was probed and not reproduced.** The design recorded that an attached
  stream *blocks* the initiating call on cursor; the measured call returned in 20s, not 180s.
  Capability 2 now rests on durability alone and records the non-reproduction. The claim was removed
  rather than shipped unverified.

**Two guard defects surfaced only under mutation, not under reading.** A negation-to-`yield` window
was satisfied by the sentence's own "has **no** such channel" sitting 24 characters away, so a full
semantic inversion stayed green until the window was tightened to 12. And `grep -qi "interactive"`
matched inside `non-interactive`, so deleting the measured mode's name passed. Both are the
[[assert-detects-removal-not-replacement]] shape.

**A plan-authored extractor was dead in both directions.** The plan's
`grep -oE 'GATE_OBSERVATION_BUDGET:-[0-9]+' | sed 's/.*://'` yields `-30`, not `30` — the greedy
`.*:` stops at the parameter-expansion colon. The cross-surface agreement comparison it fed could
never have equalled any surface, mutated or not. Caught by a worker, not by the suite.
[[plan-supplied-test-code-is-unverified]].

**Two workers raced one worktree.** A stalled worker, presumed dead and discarded, woke and wrote to
the same two files as its replacement, committed a doubled assert group, then amended. Net state was
correct; nothing made that likely. Minted as change 0231.

## Follow-ups

- **#0231** (`fix`) — a presumed-dead build worker can wake and race its own replacement in one
  worktree. No liveness check before discard, no worktree write lease, and a resumed worker may
  amend over a rival's work.
- **#0232** (`docs`) — the gate execution posture never reaches the build *workers*, who also run
  the full suite in their own verification. The rule now exists in the controller's contract, which
  is the one place a dispatched worker does not read.

**Plan deviations worth noting.** The plan's final checklist asserts the diff shows exactly ten
files; it shows twelve. `tests/test_skill_size_budgets.sh` auto-discovers `skills/**/*.md` and
rejects any file without a budget row, so creating `gate-execution.md` necessarily reddens it — no
task in this plan could have left the suite green otherwise. That checklist line is wrong, not the
diff. Four budget rows were raised across the change, each with the in-diff justification that file
requires; one of them (`docket-finalize-change/SKILL.md`) had been silently absorbed down to 13
words of headroom and was re-set as review finding 7.

**No ADR was written.** The spec called for one recording the ADR-0024 boundary. The blocker fix
made that boundary a *scoping rule inside the contract itself* — the yield is scoped by the
observing agent's dispatch posture — rather than an unexplained exception needing a ledger entry to
justify it. If you disagree, the ADR is still worth minting; the reasoning is in
`skills/docket-build/SKILL.md`'s "does not relax" paragraph and would transplant directly.
