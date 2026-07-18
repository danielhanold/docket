---
slug: relax-the-policy-before-building-the-workaround
hook: "Before building machinery to route around a blocking wall, ask who controls the wall — a policy the maintainer can relax beats any amount of in-repo automation."
topics: [process, design, automation]
changes: [95]
created: 2026-07-18
updated: 2026-07-18
promotion_state: candidate
promoted_to:
---

## Apply
When automation hits a wall — a required approval, a permission denial, a protected branch, a quota
— the reflex is to design a mechanism that satisfies the wall. Ask one question first: **who
controls this wall, and is it configuration?**

Walls come in two kinds, and they are not equally hard:

- **Policy walls** are settings some human owns — branch protection, required reviewers, a CI
  gate's own config, an org permission. Changing one is a single console toggle, ships instantly,
  adds no code, and cannot rot.
- **Mechanism walls** are properties of a system you do not control — a vendor's classifier, an
  API's rate limit, a protocol.

Only a mechanism wall justifies building machinery. A policy wall wants a *conversation*, and it is
the maintainer's call, not yours to make silently — but you must surface the option. State it
plainly: "this is your branch-protection setting; relaxing it to 0 required approvals removes the
need for everything below." The user often does not realize the blocker is theirs, because from
inside the automation it is indistinguishable from physics.

The failure mode is quiet and expensive: each workaround makes the *next* wall look like it also
needs a mechanism, so the policy question never gets asked. Watch for the smell — **a second change
whose purpose is to work around the same wall**. One workaround is a judgment call; two is a signal
you never asked who owns the wall. At that point stop building and ask.

Corollary: a policy relaxation is also strictly more honest. Machinery that satisfies an approval
gate without a human reading the diff has not obtained review — it has faked the signal. Turning
the requirement off says the true thing out loud, and leaves the option of turning it back on. See
[[harness-behavior-is-mode-and-version-scoped]]: machinery built against a vendor guard is doubly
fragile, because the guard moves under you.

## War story
- 2026-07-18 (#95, PR #101) — **Three changes spent working around one console setting.** Changes
  0015, 0021, and 0062 all chased the same goal: let `docket-finalize-change` gate, merge, and
  close out without a human wall. `main`'s branch protection required an approving review, and a
  solo maintainer cannot approve their own PR — so the merge needed `--admin`, and 0062's answer
  was to *build a GitHub Actions workflow that bot-approves the PR* (ADR-0042), plus a
  `setup-auto-approve.sh` installer, a setup doc, a template, a `finalize.auto_approve` config
  knob, a resolver field, a facade op, a whole step 6 in the finalize gate, and three test files.
  It never worked: Claude Code's interactive classifier soft-denies the `gh workflow run` dispatch
  the gate must issue, so the bot chain's first step is blocked and the promised capability does
  not exist on the harness as run. The actual fix, found empirically at the 0088 finalize: set
  branch protection to **require a PR but require zero approvals**. A plain `gh pr merge --rebase`
  then satisfies protection with no `--admin`, no bot, and nothing for a classifier to deny. That
  is one setting in the GitHub UI, and it made 0088's finalize the first end-to-end run that
  worked. This change deleted the entire subsystem (−732 lines) and reversed ADR-0042 with
  ADR-0043. The maintainer's own reaction on being shown the one-setting fix was surprise that it
  had never been suggested across three changes of workarounds — nobody had asked who owned the
  wall.
