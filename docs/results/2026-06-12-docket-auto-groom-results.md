# docket-auto-groom — autonomous grooming drain — results
Change: #14 · Branch: feat/docket-auto-groom · PR: (set on open) · Plan: docs/superpowers/plans/2026-06-12-docket-auto-groom.md · ADRs: 4, 6

## Verify (human)

- [ ] Read `skills/docket-auto-groom/SKILL.md` end-to-end — this is the first skill that can
  make changes build-ready (= auto-buildable) with no human; confirm the bounds match your
  intent (critic gates spec AND trivial exits; kill/defer surface only as abstain
  recommendations).
- [ ] Skim the convention diff (`skills/docket-convention/SKILL.md`) — the *Autonomous
  grooming (shared definition)* section is the contract every other skill now references.
- [ ] After merge: run `bash link-skills.sh` once so the new skill symlinks into your
  harnesses (it globs `skills/*/` — no script edit was needed).

## Findings

- **Re-arm had to delete the abstain section** (whole-branch review): the board's
  "auto-groom blocked — needs you" cell and groom-next's first selection band key off the
  `## Auto-groom blocked` section's *presence*, so a re-armed stub with a stale section
  would be mislabeled. Re-arm is now: supply context, flip `auto_groomable: true`, delete
  the section (git history keeps it). Spec amended same-day.
- **Verdict-authority bounds recorded as ADR-0006** (critic gates every build-ready exit;
  kill/defer never autonomous), relating to ADR-0004's no-claim stance, which this skill
  adopts in an autonomous variant (single-commit-per-stub CAS, discard-and-loop on a lost
  race instead of stop-and-report).
- **README drift from 0012**: the skill table lacked `docket-groom-next` and said "six
  skills"; fixed here while adding the eighth row.
- **Line-number order assertions can't order two phrases in one paragraph** — the
  groom-next band-order sentinel needed byte offsets (`grep -ob`), not `grep -n`.

## Follow-ups

- Chaining (auto-groom → implement-next in one autonomous run) — future change, with/after
  0008 (parallel backlog drain); deliberately out of 0014's scope.
- Abstain → 0009 (human-escalation-loop) upgrade path: the blocked section is
  forward-compatible with structured questions-for-you + notification delivery.
- The two open questions in the change file (persisting critic refutations; per-drain
  token/runtime cap) were left open by design — revisit after first real drains.
