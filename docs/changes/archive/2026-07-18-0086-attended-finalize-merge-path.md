---
id: 86
slug: attended-finalize-merge-path
title: Attended finalize has no merge path under auto_approve — scope the --admin ban to autonomous runs
status: killed
priority: high
created: 2026-07-17
updated: 2026-07-18
depends_on: []
related: [62, 87]
adrs: [42, 11]
spec:
plan:
results:
trivial: false
auto_groomable: false
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| ADRs | [ADR-0042](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0042-auto-approve-consent-model.md), [ADR-0011](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0011-finalize-consent-model.md) |
<!-- docket:artifacts:end -->

## Why

A repo with `finalize.auto_approve: true` has **no legal merge path in an attended run**. Both
doors are shut at once:

- ADR-0042 Decision #2 forbids `--admin` on any auto-approve failure, and
  `docket-finalize-change` restates it as "never used under `auto_approve`".
- The Claude Code auto-mode classifier soft-denies `gh workflow run docket-approve.yml` in an
  interactive session, so the bot approval that would satisfy branch protection cannot be
  obtained. Allow-rules do not clear a classifier soft-deny (ADR-0042's own Context says so of
  the sibling `gh pr merge` arm), and conversational-intent retry did not clear it either.

Observed live on 2026-07-17 finalizing change 0085 (PR #95): the rebase-retest gate ran green
(49/49), and the run then had nowhere to go — the human had to hand-dispatch the workflow with a
`!` prefix or authorize `--admin` in deliberate deviation from an Accepted ADR.

This is a **regression against the pre-0062 attended path**, which merged fine via explicit-id +
`--admin` + human approval. 0062 closed that door and — because the headless driver it depends on
was deliberately punted (see #87) — opened nothing in its place. Net effect today: zero
hands-off finalizes gained, one working attended path lost.

Decision #2's stated rationale is that a fallback would **silently** reinstate the two-party-review
bypass. That rationale is about the unattended driver, where nobody is watching the downgrade. It
does not bite when a human is present and explicitly authorizing the merge in-session.

## What changes

- Scope ADR-0042 Decision #2's `--admin` prohibition to **autonomous** finalize runs, leaving the
  pre-existing attended explicit-id `--admin` path intact behind explicit human authorization.
- Record the scoping as a new ADR (Accepted ADRs are immutable except `status:`; a non-reversing
  clarification is an `## Update` note, a semantic narrowing is a new ADR — the brainstorm settles
  which shape applies).
- Update `docket-finalize-change`'s gate step 6/7 prose and the abort-and-report set so the
  attended path is stated explicitly rather than implied by omission.
- Decide the attended UX when the dispatch is denied: prompt for the `!` hand-dispatch, prompt for
  `--admin`, or offer both. Autonomous runs keep today's abort-and-report, unchanged.

## Out of scope

- The headless driver (#87).
- The rebase-retest gate, `require_pr_approval` mechanics, terminal-publish.
- `docket-approve.yml` itself and the `setup-auto-approve` flow — the workflow works; the spike
  and this run agree the GitHub-side mechanism is sound.

## Open questions

- New ADR vs `## Update` on ADR-0042 — depends whether narrowing Decision #2's scope reads as a
  clarification or a reversal of a decided rule.
- Should attended `--admin` require a fresh per-merge authorization, or does the explicit id
  already carry it? (ADR-0011 says an explicit id IS the merge decision; ADR-0042 did not revisit
  that.)
- Is there a legitimate way to make the classifier pass the dispatch attended? Worth one cheap
  probe before designing around it — but note the allow-rule route is believed dead, and
  spawning a headless subprocess to dodge an interactive denial is explicitly not an option.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

## Why killed

Obsolete — the premise was retired by change 0095 (PR #101, ADR-0043) on 2026-07-18.

0086 existed because a repo with `finalize.auto_approve: true` had no legal attended merge
path: ADR-0042 Decision #2 banned `--admin` on any auto-approve failure, while the Claude Code
auto-mode classifier soft-denied the `gh workflow run docket-approve.yml` dispatch that would
have satisfied branch protection. Both halves of that trap are gone:

- **ADR-0042 is `Reversed by ADR-0043`** — Decision #2 no longer binds anything.
- **The `auto_approve` subsystem was deleted in full** by 0095 — the knob, the resolver field,
  the workflow and its template, the setup script, and the finalize gate's step 6. There is no
  dispatch left to be denied.
- **Branch protection now requires a PR with zero approvals**, so a plain `gh pr merge --rebase`
  lands attended with no `--admin` and nothing for a classifier to fire on.

All four of 0086's work items are moot, and the one that was real work is already shipped: the
finalize gate's step 6 on `main` now states the attended path explicitly — "`--admin` remains
available only on the pre-existing explicit-id / attended paths, where a sole maintainer chooses
to force past an otherwise-unsatisfiable required review."

Live proof: the 2026-07-18 attended, explicit-id finalize of change 0095 itself merged with no
`--admin`, no dispatch, and no wall — the exact scenario 0086 was filed to unblock.

Not carried forward: open question 2 (whether attended `--admin` needs fresh per-merge
authorization). ADR-0011 already answers it — an explicit id IS the merge decision — and with
0-approvals protection `--admin` is a near-never path. Reviewed and accepted by the maintainer
2026-07-18. #87 (headless finalize driver) is unaffected and stands.
