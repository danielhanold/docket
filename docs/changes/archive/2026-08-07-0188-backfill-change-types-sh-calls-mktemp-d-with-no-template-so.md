---
id: 188
slug: backfill-change-types-sh-calls-mktemp-d-with-no-template-so
title: backfill-change-types.sh calls mktemp -d with no template, so TMPDIR is ignored on macOS and uchg fixtures leak undeletable dirs
status: killed
priority: medium
type: fix
created: 2026-08-01
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [186]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

`scripts/backfill-change-types.sh` creates its staging dir with `stage="$(mktemp -d)"` — **no
template**. On macOS, `mktemp -d` with no template **ignores `TMPDIR` entirely**. Measured
2026-08-01:

```
TMPDIR=/var/folders/…/T/tmp.uNenpW7cL3 mktemp -d
  -> /var/folders/…/T/tmp.0gWNidPx3a      # NOT under the requested TMPDIR
```

Two consequences.

**1. A documented test affordance is silently a no-op.** `tests/test_backfill_change_types.sh`
passes `TMPDIR="$drb/tmpdir"` in both rollback fixtures, and the committed comment explains it as
"TMPDIR is redirected under the fixture so the script's own scratch dir (whose rollback copies
inherit the immutable flag) is cleaned up with it." On macOS that redirect does nothing — the stage
dir lands outside the fixture, where no trap in the test file can reach it.

**2. It leaks undeletable directories, permanently.** The rollback scenarios use `chflags uchg` to
force an install failure. `cp -p` preserves BSD file flags, so the script's own
`.backup/0002-b.md` inherits `uchg`; the script's `trap 'rm -rf "$stage"' EXIT` then fails with
`Operation not permitted` and the directory survives forever. Measured rate: **+1 per test run on
`origin/main`, +2 on change 0186's branch** (which adds a second `uchg` fixture by the identical
mechanism). Roughly 11,000 such directories had accumulated on the development machine before
change 0186 swept 238 of them by signature.

Found while building change 0186. Deliberately **not** fixed there: 0186's scope is the `mv -f`
install fix and its guards, and this is a distinct defect in a different line of the same script
that deserves its own coverage rather than being smuggled into an unrelated diff. Change 0186's
plan records the corrected measurement so the false "count does not grow" expectation is not
inherited.

## What changes

- Give `mktemp` a template so `TMPDIR` is honored, e.g.
  `mktemp -d "${TMPDIR:-/tmp}/backfill-change-types.XXXXXXXX"`. This immediately makes the existing
  `TMPDIR=` redirects in both test fixtures work, taking the leak to zero.
- Add coverage pinning the property: with `TMPDIR` set, the script's scratch dir is created **under
  it**. Without that assert the fix is invisible to the suite.
- Audit the other `mktemp -d` call sites in `scripts/` for the same no-template shape, and decide
  whether the rule belongs in `AGENTS.md` (a bare `mktemp -d` is not `TMPDIR`-respecting on BSD).
- Consider a one-off sweep of the accumulated `*/tmp.*/.backup/*` debris, matched by signature and
  age-gated so a concurrent suite run cannot be raced.

## Out of scope

- Any change to how the rollback fixtures force their install failure — `chflags uchg` is the right
  mechanism and is what change 0186's pty guard depends on.

## Why killed

Consolidated into #0254 at the 2026-08-07 backlog triage: same 0186 origin and same BSD-default root class as #0189; one sweep, one AGENTS.md rule promotion.
