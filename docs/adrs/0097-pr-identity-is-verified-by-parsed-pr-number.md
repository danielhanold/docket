---
id: 97
slug: pr-identity-is-verified-by-parsed-pr-number
title: "Manifest pr: stores the canonical URL; PR identity is verified by parsed number"
status: Accepted
date: 2026-08-24
supersedes: []
reverses: []
relates_to: []
change: 344
---

## Context

Two requirements collided over the change manifest's `pr:` field.

The board needs the **full canonical URL** (`https://github.com/owner/repo/pull/N`). The older
`owner/repo#N` shorthand renders as bare text, and in this repo — whose name is `docket` — the
board's link renderer mangles it into `[#docket#N](…/pull/docket#N)`: a broken link on the primary
human surface.

Finalize, meanwhile, verifies PR identity: it re-discovers the live PR for a branch and asserts it
is the one the manifest records. That assertion was written as **full-string equality** against the
shorthand, so the field's two consumers pulled in opposite directions — widening the format for the
board would break every identity comparison written against the old spelling. Change 0344 first
widened the finalize *reader* to accept both forms; this decision closes the loop on the *writer*
and on every comparison site.

The question is what the comparison should be *about*. A repo-qualified string compares two facts at
once — which repository, and which PR in it — but the repository half is already established
upstream of every one of these comparisons: `DiscoverRepository` resolves `ghRepo`, and
`FindOpenPullRequestsByHead(ghRepo, branch)` returns only PRs belonging to it. By the time the
comparison runs, the `owner/repo` prefix is redundant with an already-verified fact, and carrying it
into the comparison is what couples identity to a spelling.

## Decision

The manifest `pr:` field stores the **canonical PR URL** (`https://github.com/owner/repo/pull/N`),
and PR identity is verified by **parsed PR number**, never by full-string equality on the recorded
text.

- Parsing is centralized in the `parsePRRef` extractor in `internal/app/finalize_context.go`, which
  is **tolerant of both forms** — the new canonical URL and the legacy `owner/repo#N` shorthand — so
  manifests written before this change still verify.
- Comparing by number rather than by repo-qualified string is sound precisely because the repository
  is pinned upstream: `DiscoverRepository` plus `FindOpenPullRequestsByHead(ghRepo, branch)` have
  already established that the live PR belongs to `ghRepo`, which makes the PR number a **complete
  discriminator** at that point.
- Sites migrated onto parsed-number comparison:
  - the two live-recompute identity conjuncts, in `change_implemented.go` and `run_verify.go`;
  - **and — noted explicitly because it is the non-obvious one — the mark-implemented
    replay/idempotency guard** in `change_implemented.go` (`c.PR().Value == req.PR`). It was made
    form-tolerant via a new `samePRRef` helper, so a legitimate retry that passes a shorthand
    `--pr` still reads as a **no-op** rather than as `contended`. Left as string equality, the
    format widening would have turned benign retries into spurious contention.
- The `--pr` CLI flag's semantics are **unchanged**: it remains an identity assertion, now accepting
  either form and checked by number.
- Surviving `fmt.Sprintf("%s#%d")` uses are **display-only** — protocol `Reference:` result fields.
  They are not identity and were deliberately left alone; the shorthand is fine as something a human
  reads, and only bad as something a machine compares.

## Consequences

- The board renders a working PR link for every change, including in a repo whose name collides with
  the shorthand's separator.
- Identity checks no longer break when the recorded spelling changes. Format and identity are
  decoupled: a future third form only needs a `parsePRRef` case, not an audit of every comparison.
- Legacy manifests keep verifying with no migration pass, and retries against either form stay
  idempotent.
- The cost is that identity is now narrower than the text it is parsed from: a number alone would
  not distinguish PRs across repositories. That is safe **only** under the upstream pinning above,
  so any future comparison site that is not downstream of `DiscoverRepository` +
  `FindOpenPullRequestsByHead` must not adopt this pattern without re-establishing the repository
  first.
- Two spellings now circulate — canonical in the manifest, shorthand still emitted in display
  `Reference:` fields — so a reader must not infer from a displayed shorthand that identity is
  string-compared anywhere.
