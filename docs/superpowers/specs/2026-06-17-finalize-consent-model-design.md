# Finalize consent model — ambiguity-only prompt + `require_pr_approval` policy gate

**Date:** 2026-06-17
**Change:** 0021
**Status:** Design (brainstorm complete, awaiting build)
**Related:** change 0015 (rebase-retest merge gate), ADR-0010 (finalize merge gate), change 0019 (finalize ci/both functional test — sibling, not overlapping)

## 1. Context

`docket-finalize-change`'s Selection rule prompts before merging a mergeable-but-unmerged PR under
the no-arg (auto-detect) path — "merging is a deliberate act." The prompt's real job is guarding the
**bulk-merge blast radius**: a no-arg invocation can match several `implemented` changes at once, and
it shouldn't blanket-merge everything it finds.

But in the common case — **one** obvious target the human clearly meant to finalize — that prompt is
pure friction. The friction is sharpest on docket's **primary use case, a single human**, where the
author pushes their own PRs and **cannot approve them on GitHub at all**, so the PR is always
unapproved. The human invoking finalize *is* the decision; being asked to re-confirm it interrupts
the flow.

## 2. Decision

Two changes, both to `docket-finalize-change`:

1. **Prompt only when ambiguous.** The no-arg path stops asking when there is exactly one thing to
   merge; it prompts only when auto-detect would merge **more than one** change.
2. **A repo-level approval policy knob, `finalize.require_pr_approval` (default `false`).** It decides
   whether docket will merge an *unapproved* PR on the auto-detect path — defaulting permissive for
   the single-human case, opt-in strict for teams.

## 3. Config — `finalize.require_pr_approval`

Nested in the existing `finalize:` block (alongside `gate:` / `test_command:`), because it is the
same *kind* of thing as the rebase-retest `gate`: a repo-level safety gate on docket's merge action.
`gate` validates **correctness** (rebase + tests); `require_pr_approval` validates **human sign-off**.

```yaml
finalize:
  gate: local                  # existing — correctness gate (local | ci | both | off)
  # test_command:              # existing — optional suite override
  require_pr_approval: false   # NEW — default false. true ⇒ the auto-detect path refuses to merge
                               #       an unapproved PR (reviewDecision != APPROVED), surfacing instead.
```

Default **`false`**: approval is never a selection-time blocker (CI is still enforced by `gate` when
`gate: ci|both`). Documented in `docket-finalize-change`'s *rebase-retest merge gate* section, the
existing home of `finalize:` config (the convention's `.docket.yml` example does not enumerate
`finalize:` — finalize owns its config doc; this follows `gate`/`test_command`'s precedent).

## 4. Selection behavior (the full matrix)

### 4.1 No-arg (auto-detect)

| Situation | Behavior |
|---|---|
| Already-merged PRs | Archive silently (unchanged) |
| Candidate not git-mergeable (`CLOSED`, `DRAFT`, GitHub-reported conflict the gate can't act on) | **Surface, do not merge** |
| `require_pr_approval: true` AND candidate unapproved (`reviewDecision != APPROVED`) | **Surface, do not merge** (policy gate) — report it so the human knows docket saw it and why it was skipped |
| Exactly **one** eligible candidate (git-mergeable; approved-or-policy-off) | **Run the full flow — gate + merge + finalize — NO prompt** |
| **More than one** eligible candidate | **Prompt**: list them, confirm the batch (the blast-radius guard) |

"Eligible" = git-mergeable AND (`require_pr_approval: false` OR approved). The ambiguity count is over
*eligible* candidates: under `true`, an unapproved PR is surfaced-not-merged and does **not** count
toward the prompt.

Git-conflict handling is **delegated to the rebase-retest gate** (it rebases onto base and dispatches
`docket-rebase-resolver` on conflict, aborting-and-reporting on an unresolvable one). Selection's
"surface, don't merge" therefore covers states the gate can't act on (draft/closed/flatly
un-mergeable), not the rebaseable-conflict case the gate already owns.

### 4.2 Explicit id (`docket-finalize-change <id>`)

**Unchanged by default; the explicit id is itself the human authorization.**

- Never prompts (an explicit id is unambiguous — true today, true after).
- The rebase-retest **correctness gate still runs** (as today).
- `require_pr_approval` does **not** block it: passing an explicit id *is* the sign-off the approval
  gate asks for, so it proceeds even on an unapproved PR under `require_pr_approval: true`. The
  approval policy governs only the auto-detect path; merging an unapproved PR simply requires being
  explicit about it (a deliberate, logged act the author can't reach by a bare no-arg run).

So: with the default `false`, explicit-id behavior is **byte-for-byte unchanged**. The only explicit-id
difference exists under `true`, and it is "explicit id overrides the approval requirement."

## 5. Principle (the one-liner this all reduces to)

`require_pr_approval` ensures a **human authorized** the merge. On the auto-detect path that proof is
a GitHub approval; an **explicit id is that proof by another means**. Correctness (rebase-retest gate)
is checked regardless of which proof was used.

## 6. Scope

- Edit `skills/docket-finalize-change/SKILL.md`:
  - **Selection** section → encode the §4.1 matrix (single eligible → no prompt; >1 → prompt;
    surface-don't-merge for un-mergeable and, under the policy, unapproved).
  - **The rebase-retest merge gate** config block → document `require_pr_approval` and its default.
  - Note the explicit-id override (§4.2) where Selection/step-1 describes the explicit-id path.
- Add a commented `require_pr_approval: false` to this repo's `.docket.yml` (`finalize:` block) for
  discoverability — a default-branch edit.
- Extend `tests/test_finalize_gate.sh` (the finalize structural test) to assert: the knob is
  documented with default `false`; the Selection prose encodes ambiguity-only prompting; the
  explicit-id-overrides rule is stated. Non-vacuous (mutation-tested), grep files not piped producers.
- **Likely a small ADR** at build time (the finalize consent/approval model), relating to ADR-0010 —
  recorded by `docket-implement-next` per its non-obvious-decision rule.

## 7. Out of scope

- Changing the rebase-retest `gate` behavior or its CI logic (the gate owns correctness + CI).
- CI-state selection logic — CI is the gate's concern, not selection's.
- A `--yes`/`all` bypass flag (the ambiguity-only model removes the need for the single-target case;
  multi-target deliberately still confirms).
- Any change to the kill paths or terminal-publish.

## 8. Open questions

None — the consent model (ambiguity-only prompt), the config knob (name `require_pr_approval`,
placement under `finalize:`, default `false`), the readiness bar (git-mergeability; approval gated by
the knob), and the explicit-id override were all settled in the 2026-06-17 brainstorm.
