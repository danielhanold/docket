---
slug: harness-behavior-is-mode-and-version-scoped
hook: "An observation about a harness guard is scoped to the mode and version it was seen in — re-probe in the exact mode you will run before designing against it."
topics: [process, spike, environment]
changes: [62, 85, 95]
created: 2026-07-17
updated: 2026-07-18
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

**Scope the RULE to the mode you validated, not just the observation.** The mode-scoping error
recurs one level up: a design validated in one mode writes an *unscoped prohibition*, and strands
the mode it never tested. Before writing "never do X," ask which mode the reasoning came from and
whether the other mode has a path left. A rule whose rationale is "silently" does not apply where a
human is watching — say so in the rule, or the next reader applies it everywhere.

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
- 2026-07-17 (#85, PR #95) — The bill for (c) came due the same day, at the very next finalize. An
  **attended** `docket-finalize-change` on a repo with `auto_approve: true` ran its rebase-retest
  gate green (49/49) and then had **no legal way to merge**: the interactive classifier soft-denied
  the `gh workflow run docket-approve.yml` dispatch (twice — conversational-intent retry did not
  clear it, and per ADR-0042's own Context an allow-rule cannot clear a soft-deny), while ADR-0042
  Decision #2's "any auto-approve failure is abort-and-report; **NEVER** `--admin`" closed the
  pre-0062 attended path. 0062 validated headless and wrote its prohibition unscoped, so the mode it
  never tested lost the working path it already had — a regression, not a gap. Decision #2's own
  rationale is that a fallback would **silently** reinstate the bypass; with a human present
  explicitly authorizing, nothing is silent and the rationale does not bite. Filed as #86 (scope the
  ban to autonomous runs) and #87 (ship the driver, whose absence means the capability has no
  consumer and the mode-scoped rule protects nothing yet). Note the diagnostic trap: the denial
  looks like proof the mechanism failed, and is not — the spike ran that exact dispatch clean
  headless. Same command, same repo, same day, different mode.
- 2026-07-18 (#95, PR #101) — **The verdict, one day later: the whole subsystem was retired.** The
  0085 entry above filed #86/#87 to scope the prohibition and ship the driver — i.e. to keep
  building on the mechanism. Instead the mechanism was deleted. What settled it was the corollary
  in *Apply*, applied one level further out: rather than change the real state of GitHub *via a
  bot*, change the real state of the **policy** — branch protection set to require a PR with zero
  approvals. A plain `gh pr merge --rebase` then needs no approval to satisfy, so there is nothing
  for any classifier, in any mode, at any version, to deny. ADR-0042 was reversed by ADR-0043 and
  −732 lines came out. Read this as the load-bearing lesson of the whole family: a design resting
  on "we verified the guard does not fire on us" was fragile enough that it never once ran green
  attended, and its replacement is a setting. When a finding says harness behavior is
  mode-and-version-scoped, the strongest response is not a better probe — it is to stop needing
  the probe. See [[relax-the-policy-before-building-the-workaround]], minted from this change.
