---
id: 90
slug: publish-deferred-covers-any-handled-post-archive-failure
title: "`## Publish deferred` marks any handled post-archive failure that abandons an expected publish, not only a failed publisher"
status: Accepted
date: 2026-08-11
supersedes: []
reverses: []
relates_to: [51]
change: 118
---

## Context

ADR-0051 introduced the `## Publish deferred` marker for a terminal publish that was *expected but
did not complete*, and change 0083 wired the merge sweep to write it on the one path where
`terminal-publish.sh` actually ran and exited non-zero. The sweep has a sibling path: the step-2
`## Artifacts` re-render fails, the driver skips the publish, and the publisher is **never
attempted**. Nothing marked there.

The original stub premise was that this second gap is self-correcting — "the next sweep retries the
re-render, so the window is one pass." That is factually wrong. By the time the re-render runs the
change has already been archived, so it has left `active/`, and the sweep scans `active/` only. No
later pass ever resumes it.

## Decision

The skip-publish path marks too, and the rule generalizes: **every close-out driver owes a
`## Publish deferred` mark on any HANDLED post-archive failure that abandons an expected publish** —
stated normatively in `skills/docket-convention/references/terminal-close-out.md`.

The reasoning that settles it:

- **Permanence, not frequency, is the deciding property.** An archived change is never re-scanned,
  so the gap is permanent until a human acts — byte-for-byte the state ADR-0051 exists to surface.
- **"Nothing published means nothing was deferred yet" does not survive the code.** *Deferred*,
  *blocked*, and *never reached* are distinctions about **cause**, not visibility, and cause travels
  in the marker's dated `--detail` line. One heading keeps one reader, one removal path, one health
  finding.
- **The transient-noise objection collapses on permanence, not determinism.** The renderer genuinely
  can fail transiently (it resolves config through `docket-config.sh --export`, which fetches), but
  flapping is structurally impossible: only `terminal-publish.sh`'s success path strips the marker,
  and nothing retries an archived change.
- **The rule reaches HANDLED paths only.** A hard crash between archive and publish writes nothing
  by definition; that residual stays accepted per ADR-0051. The rule must not claim coverage it
  cannot enforce.
- **Scoped per leg, because the drivers diverge.** A failed step-2 **re-render** abandons the
  publish for every driver, so every driver owes a mark (the sweep by code; the three skill-driven
  drivers by contract text, with no executable enforcement). A failed step-2 **commit/push** skips
  the publish only in the skill-driven drivers — the sweep deliberately continues to publish there
  (change 0075 §5) — so it owes no mark on that leg, and the pre-existing "(all callers)" claim in
  the skip-publish guard is carved out.
- **Never mark under suppression stays absolute.** Under `terminal_publish: false` or in main-mode a
  skipped publish is success. The new leg therefore needs an explicit
  `TERMINAL_PUBLISH=true && DOCKET_MODE=docket` gate — the one structural difference from the
  change-0083 leg, where both publisher suppressions are exit-0 no-ops so the leg is unreachable
  under suppression, whereas a renderer failure fires regardless of the knob.

This extends ADR-0051; it neither supersedes nor reverses it.

## Consequences

- The marker's fixed body prose is generalized ("The archive landed on the metadata branch; the
  terminal-publish step … did **not** complete"), because the old wording asserted the `## Artifacts`
  re-render had landed — factually false on exactly the new path.
- Both sweep marks share one writer that is **best-effort toward the report contract but
  transactional toward the shared metadata worktree**: a failed `add`/`commit` restores the archived
  path to HEAD, a failed `push` retains the local commit for self-healing, and a mid-rebase/merge
  tree skips the mark entirely (committing into one writes onto its detached HEAD, which would also
  make the restore corrupt the file it repairs).
- The clean-path precondition is scoped to the new leg only, never to the shared helper. On the
  change-0083 leg a dirty archived path is that run's own wake — including the window
  `terminal-publish.sh` documents as covered by "the driver's defer path re-marks" — so refusing to
  mark there would have re-opened a documented recovery window.
- Remediation now differs by leg: a `skipped-publish` gap must be **re-rendered before it is
  published**, or the publish strips the marker and ships the stale `## Artifacts` block the guard
  exists to stop.
- Cost: the generalized rule is enforced in code only for the sweep; the three skill-driven drivers
  carry it as contract text.
