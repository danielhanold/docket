# Autonomous finalize merge authorization — clear the auto-mode "Merge Without Review" soft-deny

- **Change:** 0062
- **Status:** design (build-ready on approval)
- **Date:** 2026-07-11
- **Related:** change 0061 (context:fork parity — established the fork-exclusion principle finalize relies on)
- **ADRs:** extends ADR-0011 (finalize consent model)

## Problem

`docket-finalize-change` cannot run headless on Claude Code. Under `permissions.defaultMode: auto`, a built-in auto-mode classifier rule — **"Merge Without Review"** — issues a `soft_deny` on `gh pr merge` for any PR no human has approved. A `soft_deny` is cleared only by explicit human merge intent, which a forked/dispatched/`/loop` autonomous run has no way to express. So autonomous finalize hard-stops at the merge, and change 0061 had to explicitly **exclude** finalize from its `context: fork` parity work.

This is sharpest for docket's primary use case, a **single maintainer**: GitHub structurally forbids self-approving your own PR, so the "just satisfy the classifier via `require_pr_approval`" path (ADR-0011) is not merely inconvenient — it is **unavailable**. The bypass is the only route to headless finalize for a solo-human repo.

A prior fix attempt concluded "the allow-rule doesn't work." It used `permissions.allow` (`Bash(gh pr merge:*)`), which is evaluated **before** the classifier and structurally cannot clear a `soft_deny`. The untried, correct lever is **`autoMode.allow`** — a separate field read *inside* the classifier that overrides matching `soft_deny` rules.

Granting standing permission to merge unreviewed code is a real safety-policy decision, not a mechanical config tweak — hence its own change.

## Confirmed harness mechanics (Claude Code docs, verified 2026-07-11)

These four facts shaped every design decision:

1. **Subagents cannot carry a permission exception the parent lacks.** A dispatched subagent's actions "go through the classifier with the same rules as the parent session, and any `permissionMode` in the subagent's frontmatter is ignored." There is **no per-subagent `autoMode` scoping**. → the merge/publish sub-agent idea buys nothing and is dropped.
2. **`autoMode.allow` overrides `soft_deny`.** Precedence inside the classifier: `allow` rules override matching `soft_deny` rules as exceptions.
3. **`autoMode` is read from** `~/.claude/settings.json`, project `.claude/settings.local.json`, managed settings, and `--settings` — **not** committed project `.claude/settings.json`. A checked-in repo cannot inject its own `autoMode` rules.
4. **`autoMode.allow` entries are natural-language prose, not command patterns.** You cannot scope a rule to `gh pr merge`; it reads like *"Merging pull requests without review is allowed: …"* and clears the merge-without-review *class*. Because the rule cannot be command-scoped, **where the rule physically lives is the only thing that bounds its blast radius.**

## Decision

Enable an opt-in `autoMode.allow` bypass, materialized into the repo's machine-local `.claude/settings.local.json`, gated behind a deliberate per-machine act. Everything else about finalize is unchanged.

### What this is NOT

- **No subagents** — fact (1): they inherit parent settings, so they cannot scope the bypass.
- **No skill split** — the merge is not a clean tail: only the rebase-retest gate precedes it, and *everything after it* (archive filename stamped with the merge commit's `mergedAt`, the terminal-publish "done" record, feature-branch deletion, the `done` board state) hard-depends on the merge having landed. A two-user-facing-skill split would have skill A call skill B mid-flow — no cleaner than the single flow, more surface.
- **No `context: fork` on `docket-finalize-change`** — forking would regress the interactive single-human path 0061 deliberately protected: a fork cannot take the human's merge intent (so on a non-opted-in machine it would abort-and-report instead of today's inline state-intent-and-retry) and cannot answer finalize's `>1`-candidate ambiguity prompt. finalize stays inline.

### The bypass

- The bypass is one **`autoMode.allow` prose rule** covering **both** classifier soft-denies an autonomous finalize can hit:
  1. `gh pr merge` — the "Merge Without Review" soft-deny (known).
  2. the **terminal-publish push onto the integration branch** — a *suspected* second soft-deny (docket's own "ADR main-publish classifier block" experience). Note this is distinct from the plain-permission grant `ensure-claude-settings.sh` already writes to `permissions.allow` for that push; the classifier layer is separate and may need its own `autoMode.allow` coverage. The spike confirms whether rule (2) is needed and words it accordingly.
- Rule wording is behavioral/justificatory (per fact 4), e.g.: *"Merging docket pull requests and publishing terminal records onto the integration branch without a GitHub review is allowed: docket's rebase-retest gate re-runs the full suite on the merged result before every merge, and a solo maintainer cannot self-approve their own PRs on GitHub."* Final wording is fixed by the spike.

### Placement & opt-in (blast-radius decision)

- The rule lives in the repo's **`.claude/settings.local.json`** — gitignored, per-user, machine-local, repo-scoped. This is the placement the maintainer chose, accepting its cost: because the rule cannot be command-scoped, **every** session in *this* repo on *this* machine (interactive included) will skip the merge soft-deny while the rule is present — not just autonomous runs. That cost is bounded to one repo, one machine, and is the price of a simple, docket-owned, persistent opt-in.
- **Opt-in is a deliberate imperative act**, not a declarative config knob:
  - `ensure-claude-settings.sh --enable-autonomous-merge` adds the `autoMode.allow` rule; `--disable-autonomous-merge` removes it. No positional-arg / no-flag invocation keeps today's behavior (writes only the existing `permissions.allow` push grant), so the dangerous rule is never added implicitly.
  - **Why a flag, not a `.docket.local.yml` knob:** a standing permission to merge unreviewed code should require a conscious per-machine act, not a config line a `sync` step silently re-applies or a cloner inherits. The flag path is also strictly simpler — no `docket-config.sh` parsing change and no new config-fence class. Crucially it is inherently **repo-local + machine-only and never auto-spreads to collaborators**, because it is not read from committed config: each person must run it themselves, deliberately, on their own machine.
  - A `.docket.local.yml` knob with a new "machine-local-only" fence class (honored only in `.docket.local.yml`, warned-and-ignored in committed `.docket.yml` and user-global config) was considered and rejected as more machinery for a low-priority change; the flag achieves the same safety envelope with less surface.

### Finalize behavior (unchanged, stated for clarity)

- **Interactive finalize:** untouched. The human is present and clears the soft-deny by stating merge intent, exactly as today. (If the maintainer has opted in on this machine, the soft-deny simply won't fire — an accepted consequence of the persistent placement.)
- **Autonomous finalize** (dispatched wrapper, or finalize running inside an autonomous `/loop` session in the repo): with the rule present, `gh pr merge` (and the terminal-publish push) proceed without the soft-deny — because a dispatched subagent inherits the parent session's `.claude/settings.local.json` (fact 1). With the rule **absent**, autonomous finalize hits the soft-deny and **abort-and-reports exactly as today** — no regression, no silent behavior change.
- No edit to `docket-finalize-change`'s logic is required; the bypass changes behavior transparently through settings. A short documentation note in finalize's SKILL.md (and the convention) points at the opt-in and its blast-radius cost.

## Components

| Unit | Change |
|---|---|
| `ensure-claude-settings.sh` | Add `--enable-autonomous-merge` / `--disable-autonomous-merge` flags that idempotently add/remove the `autoMode.allow` rule (new `.autoMode.allow[]` array, created if absent), preserving all existing content and its corrupt-JSON-refusal invariant. Default (no flag) behavior unchanged. |
| `scripts/ensure-claude-settings.md` | Document the new flags, the `autoMode.allow` vs `permissions.allow` distinction, the exact rule text, and the blast-radius note. |
| `docket-finalize-change/SKILL.md` | Documentation note: the opt-in exists, what it grants, its repo+machine scope and interactive-session cost; no logic change. |
| `docket-convention` (finalize/consent prose) | One reference note tying the bypass to ADR-0011's consent model. |
| New ADR | Extend ADR-0011: a third authorization proof (standing machine-local bypass) beside GitHub-approval and explicit-id; record the solo-maintainer self-approval wall and the `permissions.allow`-can't-clear-`soft_deny` finding. |
| Tests | Assert the flag adds exactly one `autoMode.allow` rule; `--disable` removes it; default invocation adds no `autoMode` rule; idempotency; corrupt-JSON refusal preserved; existing `permissions.allow` grant untouched. |

## Build-time spike (do first, before code)

1. Reproduce the "Merge Without Review" `soft_deny` on `gh pr merge` under `defaultMode: auto`.
2. Confirm a well-worded `autoMode.allow` rule in `.claude/settings.local.json` clears it. Validate with `claude auto-mode critique` / `claude auto-mode config`.
3. Determine whether terminal-publish's push onto the integration branch is **independently** soft-denied by the classifier (beyond the existing `permissions.allow` grant). If yes, fold coverage into the same rule; if no, drop rule (2) and word for merge only.
4. Fix the final prose wording from the above; it becomes the literal string the script writes.

If the spike disproves fact (2) for this specific soft-deny (rule doesn't clear it), stop and reconvene — the whole approach rests on it.

## Out of scope

- The autonomous **dispatcher/loop** that would actually drive finalize headless — this change enables the *capability*; the driver is separate work.
- The context:fork parity fix itself (change 0061).
- Any harness other than Claude Code (Cursor has no such classifier; the flag is a no-op there).
- A declarative `.docket.local.yml` knob and its config-fence class (considered, rejected above).

## Open questions — resolved in this design

- *Is autonomous unreviewed auto-merge acceptable at all?* → Yes, as a deliberate, opt-in, repo-local-machine-only bypass; the "satisfy the classifier" path stays available but is structurally unavailable to a solo maintainer.
- *Opt-in per-repo vs machine-wide?* → Per-repo **and** per-machine, via an imperative flag; never global, never committed, never auto-spread.
- *Empirically confirm `autoMode.allow` clears the soft-deny?* → Moved to the build-time spike (first task), which also settles the terminal-publish second-soft-deny question.
