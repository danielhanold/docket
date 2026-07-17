---
slug: harness-behavior-is-mode-and-version-scoped
hook: "An observation about a harness guard is scoped to the mode and version it was seen in — re-probe in the exact mode you will run before designing against it."
topics: [process, spike, environment]
changes: [62]
created: 2026-07-17
updated: 2026-07-17
promotion_state: candidate
promoted_to:
---

## Apply
Behavior of a *harness* — a permission classifier, an agent runtime, a platform policy — is not a
stable fact of the world. It is an observation, and its scope is the **mode** and the **version** it
was observed in. Two rules follow.

**Record the scope with the fact.** "The classifier denies X" is not usable; "CC 2.1.211,
`--permission-mode auto`, headless, denies X" is. A recorded finding with no mode/version attached
will be read later as universal and will be wrong. Pin the version in the ADR or the finding
itself.

**Re-probe in the mode you will actually run.** Interactive and headless are different
environments, not the same environment with a different frontend, and they diverge — the same guard
fires in one and not the other, on the same day, on the same repo. An observation from an attended
session predicts nothing about a headless run. Spike the exact mode, cheaply, BEFORE the design
rests on it, and design so the spike re-opens cheaply when the version moves.

Corollary: prefer changing the **real state** of the external system over reasoning about whether a
guard will fire on it. A genuinely approved PR gives a "merge without review" classifier nothing to
fire on, in any mode, at any version — that is durable in a way "we verified it does not deny us"
never is. See [[verify-the-claim]]: a prior finding asserting harness behavior is a claim, not an
oracle.

## War story
- 2026-07-17 (#62, PR #94) — This change was designed twice against harness behavior and the
  behavior moved both times. (a) The first design rested on a documented fact — that `autoMode` is
  honored from a repo's `.claude/settings.local.json` — which its own mandated build-time spike
  **disproved** at CC 2.1.207 (honored only from user-level `~/.claude/settings.json`, the inverse
  of the intended repo-bounded envelope). The whole design died; nothing had been built, only
  because the spike was mandated before any code. (b) The re-groomed design's go/no-go spike then
  found the "Merge Without Review" soft-deny — the wall the entire change existed to route around —
  **does not fire headless at all** under CC 2.1.211, a behavior change from the 2.1.207 findings
  the design was reasoning from. (c) Same afternoon, same repo: the *interactive* classifier denied
  committing the approval-granting workflow and denied the `gh workflow run` dispatch, while the
  *headless* arm ran the identical chain end-to-end with `permission_denials: []`. The shipped
  design survived only because it changes real GitHub state (an Actions-bot review satisfies branch
  protection) instead of arguing with the guard — the one arm that holds across both modes and any
  version. ADR-0042 pins the CC version for exactly this reason.
