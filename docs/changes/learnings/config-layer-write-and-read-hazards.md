---
slug: config-layer-write-and-read-hazards
hook: "A new config layer brings two hazards — a shared-location write turns read-leaking tests into data-loss, and a lower-precedence read can be shadowed by a generated artifact."
topics: [config, testing, precedence]
changes: [50]
created: 2026-07-09
updated: 2026-07-09
promotion_state: retained
promoted_to:
---

## Apply
When a change adds a write path to a shared user-level location, audit every test that can
transitively reach it and pin the relevant env (XDG/HOME) hermetically — tests that merely
read-leaked before the change become data-loss hazards after it. And when adding a LOWER-precedence
config layer, enumerate every higher-precedence **generated artifact** that can shadow it — not just
the direct readers — then live-test the new layer in a repo where those artifacts exist: a value can
resolve correctly and still never take effect.

## War story
- 2026-07-09 (#50, PR #59 — two hazards from one new config layer) — (a) **The write path.** Adding a
  write to a shared per-user location (`sync-agents.sh`'s auto-migration writes
  `~/.config/docket/config.yml`) upgraded every non-hermetic test that reaches it from read-leak to
  write hazard: `tests/test_install.sh` inherited `XDG_CONFIG_HOME`, so on machines exporting it (common
  on Linux) the suite would have **rewritten the developer's real global config** as a test side effect
  — caught by the final whole-branch review, fixed pre-merge. (b) **The read path.** The new layer
  passed every unit fixture, yet in live use its `agents:` values were fully shadowed in any repo
  opted into per-repo generation: the committed full wrapper set (change 0048) resolves from
  `.docket.yml` + built-ins only and takes harness precedence over the user-level wrappers carrying the
  global values. Found only by live-testing a real repo *after* the build.
