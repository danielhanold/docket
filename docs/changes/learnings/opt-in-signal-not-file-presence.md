---
slug: opt-in-signal-not-file-presence
hook: "Gate output-generating behavior on an explicit opt-in key, never on the mere presence of the config file."
topics: [config, adoption, compat]
changes: [48]
created: 2026-07-09
updated: 2026-07-09
promotion_state: retained
promoted_to:
---

## Apply
When adding output-generating behavior to a tool that has a minimal "tracking-only" adoption mode, gate
it on an explicit opt-in signal (a dedicated config key), never on the mere presence of the config file —
and add a regression test asserting the minimal adopter generates zero files and keeps `--check` a no-op.

## War story
- 2026-07-09 (#48, PR #57) — A new per-repo generation behavior (committed agent wrappers + a Cursor
  dispatch rule) was gated on merely `.docket.yml` being *present*, which silently broke tracking-only
  adopters: `install.sh`'s `sync-agents.sh` run littered 8 untracked `.claude/agents/docket-*.md` into
  any change-tracking-only repo and flipped its `sync-agents.sh --check` from a no-op to failing — a
  backward-incompatible break caught only by the whole-branch review, not planning.
