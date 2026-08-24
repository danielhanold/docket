---
id: 344
slug: finalize-pr-prober-cannot-parse-the-full-url-pr-form
title: 'Finalize PR prober cannot parse the full-URL pr: form'
status: proposed
priority: high
type: fix
created: 2026-08-24
updated: 2026-08-24
depends_on: []
stacked_on:
related: []
discovered_from: [341]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

`docket context finalize` returns `pr-unknown` for any change whose `pr:` frontmatter is written
in the full-URL form (`https://github.com/owner/repo/pull/N`), making that change un-finalizable
through the binary — the merge gate refuses before it ever contacts GitHub.

The finalize PR prober resolves a PR number from the `pr:` field via `parsePRNumber` and
`prNumberToken` in `internal/app/finalize_context.go`, both of which parse only the
`owner/repo#N` shorthand (`strings.LastIndex(ref, "#")`). A full URL has no `#`, so parsing fails,
the failure is folded into unknown PR facts, and the selector reports `pr-unknown`.

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

Teach the finalize PR prober to also accept the full-URL `pr:` form: extract the trailing PR
integer from a `.../pull/N` URL in addition to the existing `owner/repo#N` shorthand, in both
`parsePRNumber` and `prNumberToken` (keep the two representations reconciled). Cover the new form
with tests, including the existing shorthand so it keeps working.

## Out of scope

- Changing the required `pr:` representation itself (it must stay a full URL for the board).
- Any board-rendering work (that is a separate concern; see change 0343's family).
- Broader URL-parsing refactors — this is a targeted parser fix in the finalize prober.

## Open questions

- Which exact URL shapes to accept: canonical `.../pull/N`, plus a trailing slash or query/fragment
  suffix? Reject the `.../pull/N/files` sub-page form, or tolerate it?
- Should both `parsePRNumber` and `prNumberToken` share one internal extractor to guarantee they
  never diverge on which forms they accept?
- Is there a reusable URL helper already introduced by change 341 (`githubWebURL` /
  `linkContextOf` in `internal/app/link_context.go`) worth routing through, or is a small local
  extractor cleaner given the different parse direction (URL → number, vs remote → web URL)?
