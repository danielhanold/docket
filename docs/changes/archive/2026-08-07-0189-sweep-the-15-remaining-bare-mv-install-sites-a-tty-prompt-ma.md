---
id: 189
slug: sweep-the-15-remaining-bare-mv-install-sites-a-tty-prompt-ma
title: Sweep the 15 remaining bare-mv install sites — a tty prompt makes their || die guards unreachable
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

Change 0186 fixed one bare `mv` that prompts on a tty, and its spec explicitly deferred a repo-wide
sweep — *"a sweep is its own change if the finding justifies one."* Reviewing 0186 produced the
evidence that justifies one.

**The population is 15, not 1.** `/usr/bin/grep -rn -E '(^|[^-[:alnum:]])mv "' scripts/*.sh` finds
15 bare-`mv` install sites (verified 2026-08-01, excluding 0186's now-fixed one):

```
scripts/board-refresh.sh            -> docs/changes/BOARD.md
scripts/ensure-global-config.sh     -> the user's global docket config
scripts/ensure-claude-settings.sh   -> the user's Claude settings.json
scripts/ensure-docket-env.sh        -> the user's shell profile, and settings.json
scripts/render-change-links.sh      -> a change file
scripts/render-artifact-backlink.sh -> an artifact file
scripts/mark-publish-deferred.sh    -> a change file (x2)
scripts/mint-stub.sh                -> a change file (x2)
scripts/archive-change.sh           -> a change file
scripts/reclaim-claims.sh           -> a change file
scripts/docket-status.sh            -> learnings/README.md
```

**The existing `|| die` guards do not protect these.** That is the load-bearing point. The failure
mode 0186 diagnosed is not a non-zero exit — it is that BSD `mv`, given an unwritable destination
and a terminal on stdin, prints `override …? (y/n [n])`, self-answers `n` at EOF, and **exits 0**.
So `mv "$tmp" "$DEST" || die "cannot atomically replace $DEST"` never fires: the tool reports
success having written nothing. Every one of these sites is the final atomic-replace step of a
read-modify-write, so a silent no-op means the user's edit is discarded while the command prints
success.

These are worse targets than the one 0186 fixed. `backfill-change-types.sh` is a one-time
maintenance helper; `ensure-docket-env.sh`, `ensure-claude-settings.sh`, and
`ensure-global-config.sh` write **user-facing config on the interactive install path** — precisely
where a human has a terminal attached. The same shape reaches `BOARD.md` and every change-file
writer, i.e. docket's own metadata.

The reason none of this has been observed in the wild is the same reason 0186's bug survived since
2026-07-23: the destinations are normally writable, and agent shells have no tty. A read-only
destination (a restrictive umask, a root-owned file, an `uchg` flag, a synced/locked directory) plus
a human terminal is all it takes.

## What changes

- Sweep the 15 sites to a non-interactive move. Decide the house form once — `mv -f` is what 0186
  used and is the minimal edit — and apply it uniformly rather than case by case.
- Pin the rule mechanically so it cannot regress: a shape-keyed guard over `scripts/*.sh` asserting
  no bare `mv "` install site survives. Key it on **shape**, never an enumerated file list
  (`AGENTS.md`), and mutation-test it.
- Audit the `cp`/`rm` twins for the same exposure. `cp` prompts only under `-i` and `rm` only under
  `-i`/`-I`, so the expected outcome is "no change, recorded" — the same audit 0186 did for its own
  file — but it should be checked rather than assumed.
- Consider promoting the rule to `AGENTS.md`: *a non-interactive flag on a tool that can prompt is
  load-bearing, not style — an unwritable destination becomes either a hang or a zero-exit no-op.*

## Out of scope

- Re-litigating change 0186's site, which is already fixed and guarded.
- A general BSD-vs-GNU audit of every tool in the tree. This change is scoped to the
  prompt-on-unwritable-destination class in `scripts/`.

## Why killed

Consolidated into #0254 at the 2026-08-07 backlog triage: the 15 bare-mv sites land with the mktemp templating sweep; git mv carve-out recorded there.
