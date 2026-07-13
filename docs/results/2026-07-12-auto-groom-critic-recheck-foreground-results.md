# Auto-groom critic re-check foreground — never-yield rule — results
Change: #66 · Branch: feat/auto-groom-critic-recheck-foreground · PR: <set at PR open> · Plan: docs/superpowers/plans/2026-07-12-auto-groom-critic-recheck-foreground.md · ADRs: 24 (Update note)

## Verify (human)

Automated sentinels + the whole-branch review are the primary receipt; the items below are the
observational checks that prose cannot hermetically prove.

- [ ] **Behavioral contract (observational, not a code test).** The fix is a prose rule an
      autonomous agent must follow — its true confirmation is a *future* `docket-auto-groom` run
      whose critic re-check runs foreground (the designer blocks on the critic's return) rather
      than backgrounding it and yielding. There is no runtime code to exercise; watch the next
      auto-groom that hits a "wrong-but-fixable" verdict.
- [ ] **Suite failures are pre-existing/environmental — confirm on your machine.** In the build
      sandbox 5 tests failed *identically on this branch and on unmodified `origin/main`*
      (byte-for-byte same failing set + exit codes), so they are environment-bound, not
      regressions from this change: `test_docket_config.sh`, `test_ensure_claude_settings.sh`,
      `test_docket_status.sh` (all fail on `cannot resolve origin/HEAD` — the proxied git remote
      cannot `set-head`), `test_board_refresh.sh` (`BOARD.md` mode `644` — sandbox umask), and
      `test_sync_agents.sh` (times out). The 30 other tests pass, including every test that reads
      the three files this change edits (`test_composition_wiring.sh`, `test_auto_groom.sh`, and
      the convention-readers). Expect all 35 green in a normal CI/dev environment.

## Findings

- **ADR-0024 gained a dated `## Update` note** (metadata on `docket`, published to `main` at
  close-out via `adrs: [24]`): the fork-exclusion "no channel to the human" principle extends to
  the **task-notification** channel — a fork awaiting a notification yields rather than being
  resumed. This is an elaboration of an Accepted decision, deliberately recorded as an Update
  note, not a new ADR (spec D4).
- **The never-yield rule is stated once, at the contract source** (`docket-convention`
  *Composition*), so it binds auto-groom's re-check, both single-shot dispatchers, and any future
  multi-round dispatch with no per-skill duplication (spec D1). Only `docket-auto-groom` §3 — the
  one actually-under-qualified round — got a concrete wording fix.

## Follow-ups

- **Hard caller-side guard (deferred — spec D2).** The caller-side reading ("a bare `completed`
  is not proof; never adopt a child's uncommitted files") is **advisory**, leaning on the existing
  git-state dispatch contract. If premature-return incidents recur *despite* the never-yield rule,
  a hard, enforced completion validator is a clean follow-up change.
- **Build ran under the Skill-layer `auto` fallback.** `superpowers:writing-plans`,
  `subagent-driven-development`, and `requesting-code-review` were not invocable skills in the
  build session, so plan/build/review degraded to the convention's documented `auto` fallback
  (plan authored directly; plan executed with TDD inline; whole-branch review done inline). The
  artifacts (plan file, executed diff, this review) are unaffected; noted per the Missing-skill
  rule and also flagged in the PR body.
