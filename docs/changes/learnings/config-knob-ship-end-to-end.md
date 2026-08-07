---
slug: config-knob-ship-end-to-end
hook: "A new config knob is not done when it merely works — ship the sample config, the README, and the now-relaxed prose in the same change, written for someone who has never heard of the change that introduced it."
topics: [config, docs, ux]
changes: [49, 190]
created: 2026-07-09
updated: 2026-08-07
promotion_state: retained
promoted_to:
---

## Apply
A new config knob is not done when it merely *works* — ship it end-to-end in the same change: add it
(commented, with every option) to the sample `.docket.yml`, document it in README, and update any prose
that stated the now-relaxed requirement as absolute.

**Then check the register of what you wrote.** The audience for a config comment is a user deciding
whether to set the key — not a reviewer of the change that added it. A comment that opens by naming
its change number, its internal mechanism, or the machinery it arms is written from the author's
vantage and is unreadable from the user's, even when every sentence is true. Lead with what the key
does *for the reader* and why the default is what it is; keep the mechanism only where it changes
their decision (typically the safety condition on turning it on). The tell: if the first sentence
cannot be understood without having read the change, rewrite it.

## War story
- 2026-07-09 (#49, PR #58) — A change that added a new user-facing config knob (the role-keyed
  `skills:` map) shipped its resolution logic and skill-body wiring but NOT its surfacing: the
  commented sample `.docket.yml` never gained the new keys, README still framed superpowers as a
  hard requirement rather than a configurable default, and the option went undocumented — all caught
  by the human at the merge gate, not the build.
- 2026-08-07 (#190, PR #173) — The successor failure, one level up: change 0190's
  `finalize.skip_results_only_delta` key *did* ship every surface — `.docket.example.yml` entry,
  scope tag, guard test, README — and the build's own tests enforced their presence. What no test
  could check was legibility. The comment opened "arms the SECOND limb of the gate's post-rebase
  suite-skip," then leaned on changes 0170/0190, `head_sha`, and "strict ancestor of HEAD" — every
  claim accurate, and the whole block impenetrable to anyone who had not read both changes. The
  maintainer's verdict at the merge gate was "way too complex even for me." Rewritten to lead with
  the payoff (skip a redundant pre-merge test run), state the cause in terms of the results doc
  rather than a SHA, and keep the one thing that governs the decision — don't arm it if a test reads
  that directory. Same length, same facts, opposite readability. Presence is testable; register is
  not, so it needs a deliberate second pass.
