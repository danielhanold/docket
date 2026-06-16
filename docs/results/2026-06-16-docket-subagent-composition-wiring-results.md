# docket subagent composition — nested status/adr/critic dispatch — results
Change: #17 · Branch: feat/docket-subagent-composition-wiring · PR: <set on open> · Plan: docs/superpowers/plans/2026-06-16-docket-subagent-composition-wiring.md · ADRs: 8 (update), 9

## Verify (human)

- [ ] After merge, re-run `bash sync-agents.sh` so the new `agents/docket-auto-groom-critic.md`
      wrapper is generated into your harness `…/agents/` dirs (and any committed project-level
      copies). The generator runs on demand, not at session start — the source wrapper lands on
      `main` with this PR, but the generated copies do not refresh until you run it. CI guards
      drift via `sync-agents.sh --check`.

## Findings

- **Two ADRs produced** (both ride this change's terminal publish to `main` on merge, via
  `adrs: [8, 9]`):
  - **ADR-0009 (new)** — *Auto-groom critic isolation*: the critic is a dedicated wrapper that
    loads only `docket-convention`, never the `docket-auto-groom` designer body, pinned opus/xhigh,
    so the adversarial gate is real and not self-agreement theater.
  - **ADR-0008 (`## Update`)** — records that the composition it deferred has landed (foreground +
    git-state-as-contract dispatch). Its `Decision` is unchanged; immutable-body discipline kept.
- **"fresh subagent" phrase kept deliberately** in `auto-groom` Step 3. `tests/test_auto_groom.sh`
  already asserts that literal phrase and a designer→critic→exit order; naming the critic
  (`docket-auto-groom-critic`) while keeping "fresh subagent" is honest (the named critic genuinely
  IS a fresh, isolated subagent) — not a contortion to pass the test (LEARNINGS #14).
- **No `sync-agents.sh` edit** — the `agents/docket-*.md` glob + `short_name` auto-discover the
  critic (config key `auto-groom-critic`); verified at reconcile and guarded by a per-repo
  override + `--check` test (LEARNINGS #12: check whether plumbing auto-discovers before editing it).
- **Wrapper count is "five skills, six wrappers"** — the convention's "Five *skills* get a wrapper"
  line stays correct; the critic is a sixth *wrapper* that wraps no skill. Repo swept for stale
  count prose (LEARNINGS #5/#14); none found outside the immutable 0016 archive/ADR.

## Follow-ups

- **Board-pass → `docket-status` subagent wiring** (spec §2, out of scope here): every
  status-writing skill refreshes the inline board at the *caller's* model rather than
  `docket-status`'s sonnet/medium. A real but debatable optimization (a near-mechanical board
  regen may not pay for a subagent round-trip). Capture as its own change via `docket-new-change`
  — already noted in this change's `## Out of scope`.
