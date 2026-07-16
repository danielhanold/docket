---
slug: pipefail
hook: "Never producer | early-exiting-consumer under set -o pipefail — capture into a variable first."
topics: [shell, pipefail, testing]
changes: [11, 16, 46]
created: 2026-06-16
updated: 2026-07-16
promotion_state: promoted
promoted_to: AGENTS.md
---

## Apply
Never `producer | early-exiting-consumer` (`grep -q`, `head`, `head -n1`, or any reader that may stop
before EOF) under `set -o pipefail` — capture into a variable first, then grep/`head <<<"$var"`.

## War story
- 2026-06-16 / 2026-07-08 (#11 PR #11; #16 PR #30; #46 PR #56 — merged, one pipefail family) — A test
  piped a live-producing script straight into `grep -q`; grep exits on first match, the still-writing
  producer takes SIGPIPE, and `pipefail` turned that 141 into an intermittent failure — review later
  found the same shape with `head`, and #46 hit it again in production code (`printf … | section_body`,
  whose consumer `exit`s early; guarded with `|| true`).
