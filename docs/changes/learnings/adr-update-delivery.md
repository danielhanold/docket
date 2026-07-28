---
slug: adr-update-delivery
hook: "Deliver an ADR body update atomically by listing that ADR id in the producing change's adrs: — never a standalone push."
topics: [adr, publishing, git]
changes: [17, 74, 18]
created: 2026-06-17
updated: 2026-07-28
promotion_state: retained
promoted_to:
---

## Apply
To deliver an ADR body update onto the integration branch atomically, list that ADR id in the producing
change's `adrs:` so terminal-publish re-copies it on merge — never a standalone push that races the
cross-referenced ADR's own publish. An Accepted ADR is immutable except its `status:` line, so a detail
the world has since dated is appended to as a dated `## Update`, never rewritten as a Decision edit.

## War story
- 2026-06-17 / 2026-07-14 (#17 PR #31; #74 PR #82 — merged, one ADR-update-delivery family) — An
  `## Update` to an already-published, immutable ADR (0008) had to reach the integration branch
  alongside a NEW ADR (0009) it cross-references, without a premature direct-to-`main` push (which
  would dangle the `[[0009]]` link until the new ADR merged). #74 hit the same shape without the
  cross-reference: narrowing the facade wiring guard dated a *supporting detail* of ADR-0030's
  Decision while leaving the decision itself intact.
- 2026-07-28 (#18, PR #137 — merged) — Same rule for a **new** ADR that is the change's whole
  deliverable. ADR-0062 was authored on `docket`, so the feature branch carried only a plan and a
  results file: an empty-looking PR diff is the expected shape for a metadata-only change, not a
  broken build. The route to `main` is the change's own `adrs: [57, 58, 62]` at terminal publish —
  running `terminal-publish --adr 62` standalone would have landed the ADR while leaving the change
  with no route to `done`. Say so in the results file so the reviewer does not "fix" either.
