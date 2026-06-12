# Groom-next skill — results
Change: #12 · Branch: feat/groom-next-skill · PR: (set at open) · Plan: docs/superpowers/plans/2026-06-12-groom-next-skill.md · ADRs: 4

## Verify (human)

- [ ] After merge, run `bash link-skills.sh` from the repo root so `docket-groom-next` gets
  symlinked into your harness skill dirs (the script only creates missing links; existing skills
  are untouched). Until then the new skill exists in the repo but is not invocable.
- [ ] Smoke-test a real groom session: invoke `docket-groom-next` with no argument and confirm it
  selects change 0006 (learnings-ledger — the highest-priority needs-brainstorm stub), states its
  dependencies (none), and opens the brainstorm seeded with the stub's open questions.

## Findings

- `link-skills.sh` needs no code change for new skills — it globs `skills/*/`. Caught at
  reconcile; the spec's original touch-up list was corrected before planning.
- The no-claim concurrency decision became ADR-0004 (grooming relies on final-push CAS plus a
  mandatory re-read after rebase; claim-up-front remains the rule for autonomous long-running
  writers). Future multi-agent work — change 0008 (parallel-backlog-drain) in particular — should
  read ADR-0004 before assuming every skill claims.
- One quality-review finding was a false positive (a sentence the reviewer attributed to the new
  skill that exists only in `docket-new-change`); caught by byte-diffing the file against the
  plan's canonical content before acting on review feedback.

## Follow-ups

- Change 0012's "What changes" mentions the suite's count in prose in several historical
  documents (specs/results of older changes say "five operating skills"); those are immutable
  point-in-time records and were deliberately left untouched.
