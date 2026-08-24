---
id: 344
slug: finalize-pr-prober-cannot-parse-the-full-url-pr-form
title: 'Finalize PR prober cannot parse the full-URL pr: form'
status: 'in-progress'
priority: high
type: fix
created: 2026-08-24
updated: '2026-08-24'
depends_on: []
stacked_on:
related: []
discovered_from: [341]
adrs: []
spec: docs/superpowers/specs/2026-08-24-finalize-pr-prober-cannot-parse-the-full-url-pr-form-design.md
plan:
results:
trivial: false
auto_groomable: true
branch: 'feat/finalize-pr-prober-cannot-parse-the-full-url-pr-form'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-08-24T18:48:27Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | `docs/superpowers/specs/2026-08-24-finalize-pr-prober-cannot-parse-the-full-url-pr-form-design.md` |
<!-- docket:artifacts:end -->

## Why

`docket context finalize` returns `pr-unknown` for any change whose `pr:` frontmatter is written
in the full-URL form (`https://github.com/owner/repo/pull/N`), making that change un-finalizable
through the binary — the merge gate refuses before it ever contacts GitHub.

The finalize PR prober resolves a PR number from the `pr:` field via `parsePRNumber` and
`prNumberToken` in `internal/app/finalize_context.go` — the only two `pr:`-number parsers in the
tree — both of which parse only the `owner/repo#N` shorthand (`strings.LastIndex(ref, "#")`). A full
URL has no `#`, so parsing fails, the failure is folded into unknown PR facts, and the selector
reports `pr-unknown`. `parsePRNumber` also feeds the cleanup, closeout, and merge paths, so a
URL-form `pr:` is mis-handled there too, not only on the probe path.

This directly collides with a standing requirement: `pr:` **must** be a full URL for the board to
render it as a proper GitHub link (the shorthand renders as plain text / mangles on the board).
So the two representations the codebase requires are mutually exclusive across subsystems — the
board needs the URL form, the finalize prober only understands the shorthand.

This is a **pre-existing** bug: `parsePRNumber`/`prNumberToken` were introduced by change 0316
(`feat(0316): authoritative finalize context and selection`) and have only ever parsed the
shorthand; the full-URL `pr:` form predates 341. It was **not** introduced by change 341 — it was
merely first hit live while finalizing 341 (PR #235), whose `pr:` is the required URL form.

**This blocks finalizing change 341** (and every other change written with a full-URL `pr:`).

## What changes

Teach the finalize PR prober to also accept the full-URL `pr:` form. Introduce one shared internal
extractor (`parsePRRef`) that accepts **both** representations — the trailing integer of a
`.../pull/N` URL and the existing `owner/repo#N` shorthand — requiring a positive number, and route
both `parsePRNumber` and `prNumberToken` through it so they can never diverge on which forms they
accept. Because the widening lands in `parsePRNumber` itself, all five of its call sites (probe,
cleanup, closeout ×2, merge) accept the URL form for free. Accept the canonical `.../pull/N` plus a
trailing slash, `?query`, `#fragment`, and sub-page suffix (the number after `/pull/` is
unambiguous), checking `/pull/` before `#` so a URL fragment is not mistaken for the number. Cover
the new form and the retained shorthand with tests. Full design, alternatives, and the assumptions
ledger are in the linked spec.

## Out of scope

- Changing the required `pr:` representation itself (it must stay a full URL for the board).
- Any board-rendering work (that is a separate concern; see change 0343's family).
- Broader URL-parsing refactors — this is a targeted parser fix in the finalize prober.

## Open questions

Resolved at design time; see the spec's `## Assumptions` block for the full audit trail.

- **URL shapes** — accept canonical `.../pull/N` and tolerate a trailing slash, `?query`,
  `#fragment`, and sub-page suffix (the number after `/pull/` is always unambiguous); rejecting
  benign suffixes would only recreate the un-finalizable failure.
- **Shared extractor** — yes: one `parsePRRef` both functions delegate to, so they cannot re-diverge
  (they already differ on non-positive acceptance today).
- **Reuse 0341's helper** — no: 0341's `link_context.go` helpers are on its unmerged branch, 0344
  carries no `depends_on: [341]`, and they parse the opposite direction (remote → URL). A small
  local extractor keeps 0344 independent and mergeable on its own.

## Reconcile log

### 2026-08-24

2026-08-24 — Reconciled against current `origin/main`. The spec's code-proven root cause holds unchanged: `internal/app/finalize_context.go` still defines the only two `pr:`-number parsers, `parsePRNumber` (5 call sites — probe `finalize_context.go:609`, cleanup `finalize_cleanup.go:233`, closeout `finalize_closeout.go:286` + `:604`, merge `finalize_merge.go:425`) and `prNumberToken` (1 call site, `finalize_context.go:514`), and both key solely on `strings.LastIndex(ref, "#")`, so a full-URL `pr:` (no `#`) fails to parse. The two still diverge on non-positive acceptance (`parsePRNumber` requires `n > 0`; `prNumberToken` runs bare `strconv.Atoi`), exactly the latent divergence the shared-`parsePRRef` consolidation closes. Change 341 remains `implemented` (not `done`); `depends_on: []` and `discovered_from: [341]` are correct and retained. No scope change; proceeding to plan and build as specified.
