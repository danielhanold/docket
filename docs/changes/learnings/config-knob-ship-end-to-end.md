---
slug: config-knob-ship-end-to-end
hook: "A new config knob is not done when it merely works — ship the sample config, the README, and the now-relaxed prose in the same change."
topics: [config, docs, ux]
changes: [49]
created: 2026-07-09
updated: 2026-07-09
promotion_state: retained
promoted_to:
---

## Apply
A new config knob is not done when it merely *works* — ship it end-to-end in the same change: add it
(commented, with every option) to the sample `.docket.yml`, document it in README, and update any prose
that stated the now-relaxed requirement as absolute.

## War story
- 2026-07-09 (#49, PR #58) — A change that added a new user-facing config knob (the role-keyed
  `skills:` map) shipped its resolution logic and skill-body wiring but NOT its surfacing: the
  commented sample `.docket.yml` never gained the new keys, README still framed superpowers as a
  hard requirement rather than a configurable default, and the option went undocumented — all caught
  by the human at the merge gate, not the build.
