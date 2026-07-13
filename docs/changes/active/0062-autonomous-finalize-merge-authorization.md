---
id: 62
slug: autonomous-finalize-merge-authorization
title: Autonomous finalize merge — clear the auto-mode Merge-Without-Review soft-deny
status: proposed
priority: low
created: 2026-07-11
updated: 2026-07-13
depends_on: []
related: [61]
adrs: [11]
spec:
plan:
results:
trivial: false
auto_groomable: false
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| ADRs | [ADR-0011](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0011-finalize-consent-model.md) |
<!-- docket:artifacts:end -->

## Why

`docket-finalize-change` can't run headless on Claude Code because its `gh pr merge` is soft-denied by the auto-mode **"Merge Without Review"** classifier (active under `permissions.defaultMode: auto`). A `soft_deny` clears only on explicit human merge intent, which an autonomous run (dispatched wrapper, or finalize inside a `/loop`) has no way to express — so it hard-stops. This is why change 0061 had to **exclude** finalize from `context: fork`.

It bites hardest for docket's primary use case, a **single maintainer**: GitHub forbids self-approving your own PR, so the "just require a GitHub approval" path (ADR-0011) is *structurally unavailable*. The bypass is the only route to headless finalize for a solo repo.

The prior "the allow-rule doesn't work" attempt used `permissions.allow`, which runs **before** the classifier and can't clear a `soft_deny`. That diagnosis is **confirmed** — the spike reproduced it directly (`Bash(gh pr merge:*)` was present and the merge was still soft-denied).

The proposed lever was **`autoMode.allow`** — a field read *inside* the classifier that does override matching `soft_deny` rules. **The lever is real; the *placement* was not.** The 2026-07-13 spike proved Claude Code 2.1.207 does not honor `autoMode` from a project-level `.claude/settings.local.json`, only from user-level `~/.claude/settings.json` — so the "opt-in, repo-local, per-machine" envelope that made this change acceptable is **not achievable by the route it assumed**. Granting standing permission to merge unreviewed code remains a real safety decision; the open problem is finding a scope that is genuinely repo-bounded. See *What changes*.

## What changes

**TO BE REDESIGNED — needs-brainstorm.** The original design was **disproven by its own mandated build-time spike** (2026-07-13) before any code was written, and the change has been **unlinked from its spec** and returned to the grooming queue. It was never claimed, branched, or built. The old spec is preserved with a `⛔ DISPROVEN` banner: [`docs/superpowers/specs/2026-07-11-autonomous-finalize-merge-authorization-design.md`](../../superpowers/specs/2026-07-11-autonomous-finalize-merge-authorization-design.md) — read it as history, not instructions.

**What was disproven.** The design put an opt-in `autoMode.allow` rule in the repo's gitignored `.claude/settings.local.json`. **Claude Code 2.1.207 does not honor `autoMode` from that file.** Clean A/B — identical `autoMode.hard_deny` rule, identical command (`gh pr merge 999999`, a nonexistent PR), identical restart-then-fire discipline, only the file differed:

| Rule placement | Result |
|---|---|
| `<repo>/.claude/settings.local.json` | no effect — the *default* `Merge Without Review` soft-deny fired instead |
| `~/.claude/settings.json` (user-level) | **enforced** — `[Docket Canary Probe]`, unconditional |

That kills the design, not merely a detail of it: the whole safety envelope was the *placement* — "repo-local **and** machine-only, never global." The only placement that works is **machine-global**, which would grant merge-without-review to agents in *every repo on this machine* — the inverse of the intended property, not a tuning of the cost the spec had already accepted. (The docs' scope table claims `settings.local.json` is read; runtime disagrees. If that's an upstream bug and it gets fixed, the original design becomes buildable *exactly as written* — worth a `/feedback` report.)

**Redesign direction (chosen 2026-07-13 — UNVERIFIED).** The docs list **`--settings <file>`** as an `autoMode` scope. An autonomous finalize would be *launched* as `claude --settings <fragment> …`, making the bypass **per-invocation** instead of standing — it would never leak into interactive sessions, which is a *better* safety envelope than the original. **This is untested, and the same docs table already proved wrong once here.** The redesign's first task is to verify it; it must not become the next false premise.

**Findings any redesign must carry** (independently verified during the spike):

1. **The premise is sound.** `Merge Without Review` genuinely blocks a solo, unprotected repo. And `permissions.allow` really cannot clear a `soft_deny` — `Bash(gh pr merge:*)` was present and the merge was still soft-denied.
2. **`allow` overrides `soft_deny`** as an exception (documented precedence, unchanged).
3. **The terminal-publish push IS independently exposed.** `integration_branch: main` *is* the repo default branch, so the push hits the **ported-provenance arm (b)** of `Git Push to Default Branch` (records are copied from `docket`, not authored in the pushing session). The original spike question 3 = **yes**; any bypass must cover it.
4. **`$defaults` is mandatory.** Writing `autoMode.allow = ["<rule>"]` without the sentinel silently drops all 16 built-in allow rules.
5. **`Self-Approval` survives any bypass** — as does the sensitive-content arm (a) of `Git Push to Default Branch`. A bypass can grant *merging without review*; it can neither manufacture a review nor push a secret. A materially better bound than the old spec assumed, and worth stating in whatever ADR lands.
6. **The rule cannot be a literal string.** `ensure-claude-settings.sh` is generic (any docket-adopting repo), so it must be a **template** interpolating the `owner/repo` slug, `$INTEGRATION_BRANCH`, and the configured dirs.
7. **terminal-publish does NOT publish `BOARD.md`** — the copy-set is the archived change manifest, its spec, and its `Accepted` ADRs. Any rule text naming BOARD.md is wrong.

`auto_groomable: false` is deliberate: granting standing permission to merge unreviewed code is a **safety-policy decision**, so this stub must be groomed by a human, never autonomously.

## Out of scope

- The autonomous **dispatcher/loop** that would drive finalize headless — this enables the capability; the driver is separate work.
- The context:fork parity fix itself (change 0061).
- Any harness other than Claude Code (Cursor has no such classifier; the flag is a no-op there).
- A declarative `.docket.local.yml` knob + config-fence class (considered, rejected for a low-priority change in favor of the simpler flag).

Note: the "Out of scope" list above is inherited from the disproven design. The `--settings` direction may **pull the dispatcher back into scope** — if the bypass can only ride on a launch flag, then it is a property of *how finalize is launched*, and the capability may no longer be separable from its driver. Settle this during the re-brainstorm.

## Open questions

Resolve these in the re-brainstorm (all opened by the 2026-07-13 spike):

- **Does `--settings <file>` actually carry `autoMode` into a live session?** Untested. The docs list it in the *same* scope table that wrongly listed `settings.local.json`, so it must be **verified before it is designed around**. Probe: pass a `hard_deny` canary via `--settings` to a headless `claude -p` and have it attempt `gh pr merge 999999`; if the denial cites the canary rather than `Merge Without Review`, the scope is real. (Deliberately not run yet — the human stopped testing here.)
- **If `--settings` works, is 0062 still a standalone change?** A launch-flag bypass is inseparable from the launcher, which may merge this change into the autonomous-dispatcher work rather than leaving it a prerequisite capability.
- **If `--settings` does *not* work either**, the only proven lever is machine-global `~/.claude/settings.json`. Is *any* form of standing, machine-wide merge-without-review acceptable — or is headless finalize simply not available on Claude Code until upstream fixes project-scope resolution? (Kill/defer is a legitimate outcome.)
- **Should this instead be blocked on an upstream fix?** The docs promise `settings.local.json` works. A `/feedback` report may make the original design buildable as written, at zero design cost.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

- **2026-07-13** — Ran the spec's mandated build-time spike *before* any code. It **disproved fact (3)** (project-level `.claude/settings.local.json` is not honored for `autoMode`), which the whole design rested on. Followed the spec's own instruction (*"stop and reconvene"*): cleared `spec:`, set `auto_groomable: false`, returned the change to **needs-brainstorm**. Nothing was claimed, branched, or built; no code exists. Old spec retained with a `⛔ DISPROVEN` banner. Full probe transcript and the two false conclusions that preceded the real one are recorded in that banner and in the spec body.
