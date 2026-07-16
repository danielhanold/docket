---
id: 84
slug: terminal-publish-opt-in-default
title: Flip terminal_publish default to false — publishing to the integration branch becomes opt-in
status: in-progress
priority: medium
created: 2026-07-16
updated: 2026-07-16
depends_on: []
related: [62, 64, 83]
adrs: [27]
spec: docs/superpowers/specs/2026-07-16-terminal-publish-opt-in-default-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/terminal-publish-opt-in-default
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-16-terminal-publish-opt-in-default-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-16-terminal-publish-opt-in-default-design.md) |
| ADRs | [ADR-0027](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0027-terminal-publish-repo-scoped-script-gated.md) |
<!-- docket:artifacts:end -->

## Why

`terminal_publish` defaults to `true`, so every repo that never set the key gets direct
machine commits onto its integration branch at close-out. That default keeps causing real
friction: direct pushes trip branch protection on protected/PR-only branches, auto-mode
permission classifiers deny the push mid-run and hard-stop autonomous loops, and a failed
publish can gap silently (#0083 / #0043 — a terminal record missing from `main` for eight
days with nothing noticing). Writing to the code line should be a conscious, per-repo
opt-in, not a fail-open default.

## What changes

- The built-in default flips to `false` at every layer that encodes it: the config resolver
  (`docket-config.sh`), the `terminal-publish.sh` `--enabled` flag fallback, and the
  `docket-status.sh` sweep fallback — no code path defaults to publishing.
- `terminal-publish.sh` invoked with **no** `--enabled` flag no-ops loudly (prominent stderr
  warning that nothing was published, exit 0); an explicit `--enabled false` stays a silent,
  intentional no-op.
- Docs invert the framing to opt-in and call out the risks of `true`: README (rewritten
  `terminal_publish` section + config sample), `docket-config.md`, `terminal-publish.md`,
  the convention SKILL + terminal-close-out reference, and default-mentions in the adr /
  finalize / status skills.
- This repo pins `terminal_publish: true` in its committed `.docket.yml` — its
  archive-parity practice is unchanged, now explicit.
- A new ADR records the flip (`relates_to: [27]`; ADR-0027 stays Accepted — it decided the
  fencing and gating, not the default value).
- The four affected test suites are updated; fixtures that relied on the implicit default
  pin `true` so the publish path stays covered, plus new coverage that unset ⇒ no publish.

## Out of scope

- #0083's gap-detection health check (separate change; this narrows its blast radius but
  does not replace it).
- Any PR-based publish mechanism for repos that want records on the integration branch
  without direct pushes.
- Retroactive behavior — the knob remains never-retroactive (ADR-0027).

## Open questions

None — design settled in the linked spec (brainstormed 2026-07-16).

## Reconcile log
