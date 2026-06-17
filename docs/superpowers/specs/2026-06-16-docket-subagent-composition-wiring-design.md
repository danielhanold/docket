# Design: docket subagent composition — nested status/adr/critic dispatch

**Status:** design (brainstormed 2026-06-16 via `docket-groom-next`, with the human)
**Change:** 0017
**Depends on:** change 0016 (`done`, PR #30 — the agent-layer foundation: wrappers, `sync-agents.sh`, layered config, ADR-0008)
**Related:** change 0015 (finalize rebase/retest — also composes subagents, independent), change 0016 (foundation), `docket-convention` (gains the present-tense composition contract)

## 1. Context / problem

Change 0016 gave each autonomous docket skill a thin **subagent wrapper** that pins its model + effort (`implement-next`/`auto-groom` → opus/xhigh; `finalize-change`/`status`/`adr` → sonnet/medium). So *standalone* invocation of each skill now runs at its own tier. But 0016 deliberately stopped at the foundation: a skill's **internal sub-invocations still run inline at the parent's model**. ADR-0008's final consequence states it plainly — "Composition is deferred to change 0017 … nested sub-invocations (`implement-next → status`/`adr`, `auto-groom → critic`) still run inline at the parent's model until 0017 rewires them."

Three concrete costs of the inline status quo:

- `implement-next` (opus/xhigh) runs the *entire* `docket-status` skill inline at step 0 (merge-sweep + health + board) and the *entire* `docket-adr` skill inline at step 6 — both on Opus, though each is pinned sonnet/medium standalone.
- `auto-groom` (opus/xhigh) dispatches its adversarial critic as an *unnamed* fresh subagent that inherits the parent's context/model rather than running in a genuinely isolated, independently-pinned context.

Nested subagents are confirmed (Claude Code ≥ v2.1.172, foreground, any depth — verified in 0016's spec §2 and exercised by 0016). 0017 is the **composition** half: rewire those sub-invocations to dispatch *named* subagents so each runs at its own configured model, and give the critic real adversarial isolation.

## 2. Scope

**In scope — exactly three call sites** (the "whole-skill mid-flight invocation" set ADR-0008 named):

1. `implement-next` step 0 → dispatch the `docket-status` subagent (sonnet/medium).
2. `implement-next` step 6 → dispatch the `docket-adr` subagent (sonnet/medium), once per non-obvious decision.
3. `auto-groom` step 3 → dispatch a dedicated `docket-auto-groom-critic` subagent (opus/xhigh) for the adversarial gate.

**Out of scope:**

- **The board-refresh "Board pass" wiring** — every status-writing skill (`new-change`, `groom-next`, `finalize`, `auto-groom`, `implement-next`'s own claim/reconcile/implemented writes) refreshes the board inline, so it renders at the caller's model rather than `docket-status`'s sonnet/medium. That inconsistency is real but it is a separate, debatable optimization (a near-mechanical board regen may not pay for a full subagent round-trip). Split to its **own change** (captured via `docket-new-change`), mirroring the 0015/0016/0017 split.
- **The TDD build's model** — `superpowers:subagent-driven-development`'s own config, where most token spend lands. Pinning `implement-next` to Opus governs its reconcile/escalation, not the build. Untouched (same boundary as 0016 §6).
- **The other inline sub-references** (e.g. `status`'s sweep invoking the harvest by reference) — small, in-skill, not whole-skill invocations; left inline.

## 3. Dispatch mechanics (the shared contract for all three sites)

All three rewirings follow one contract:

- **Foreground / blocking.** The parent suspends until the subagent returns. This is what preserves ordering (§3a) and makes shared-worktree access collision-free within the chain (§3c). **No background dispatch anywhere** in 0017.
- **Unconditional, baked into the skill body.** The rewiring lives in the skill *body*, so the sub-call gets its own model whether the parent was invoked as its wrapper subagent or as a plain inline skill. (If we only dispatched "when the parent is itself a subagent," a human running `docket-implement-next` inline on an Opus session would run status on Opus — defeating the point.)
- **The contract is git state, not in-context return.** The sub-skills' effects are commits on `origin/docket` (and, for adr, a published ADR on the integration branch); the parent re-reads them after re-syncing, never relying on shared memory.

### 3a. Ordering — `implement-next → status` (step 0)

Today `docket-status`'s merge-sweep archives merged `implemented` changes and refreshes `BOARD.md` (commits to `origin/docket`) **before** `implement-next` selects. As a foreground subagent it does the identical git work on the **same shared `.docket/` worktree**, returns, and the parent — which already re-syncs `.docket/` (`pull --rebase`) before any read, i.e. before selection — picks up the swept state. Ordering holds because of **blocking + re-sync**, not shared memory.

### 3b. Return value — `implement-next → adr` (step 6)

`docket-adr` assigns the ADR number, commits the ADR on `origin/docket`, publishes it onto the integration branch on acceptance, and **returns the number**. The parent appends that number to the change's `adrs:` in the metadata working tree (after re-syncing `.docket/`), then commits that edit itself — exactly as today, only the body of the work now runs at sonnet/medium. The dispatch may recur (one per non-obvious decision).

### 3c. Depth & isolation

Every added dispatch is depth ≤ 2 from the skill (`implement-next → status`/`adr`; `auto-groom → critic`), all foreground. The only deep chain is the TDD build (`subagent-driven-development`), which is out of scope and which 0017 does **not** deepen — status/adr are *siblings* of the build, not nested under it. The depth-5 cap is never threatened, so no path needs a background subagent. Shared-worktree safety within a chain is free: parent and nested subagent never touch `.docket/`'s index concurrently because the parent is suspended while the child runs. (Cross-*agent* concurrency — a separate groomer alongside an implementer — is the existing CAS-protected situation, unchanged here.)

## 4. The critic wrapper — `agents/docket-auto-groom-critic.md` (new)

The one genuinely new artifact. Unlike the other five wrappers, **it wraps no skill**: there is no "critic skill," and injecting the `docket-auto-groom` designer body would re-inject the designer's "commit to the conservative default" bias into the adversary — exactly wrong. So:

```yaml
---
name: docket-auto-groom-critic
description: <one-line; an adversarial reviewer of an auto-groom draft spec/verdict>
model: opus
effort: xhigh
skills: [docket-convention]      # convention vocabulary only — NOT the auto-groom skill
---
You are an adversarial critic of the draft handed to you in your prompt. Attack it;
do not defend or improve it. Return exactly one verdict per the dispatching skill's
protocol. You run autonomously: never prompt — if you cannot reach a verdict from the
context provided, that IS the "needs human context" verdict (the groom abstains).
```

- **Model/effort: opus/xhigh** — the adversarial gate must be ≥ the designer or it is theater (0016 §4 table's `(critic)` row, now materialized).
- **Behavioral contract stays in `auto-groom` Step 3.** The verdict set (`sound` / `wrong-but-fixable, one bounded revision round` / `needs-human ⇒ abstain`) and *what* to attack ride in the dispatch prompt. The wrapper only adds the pinned tier + convention + the adversarial stance + abort-and-report (its `needs-human` verdict is auto-groom's abstain — consistent with ADR-0008 sub-decision #4). `auto-groom` stays the single source of the critic's behavior.
- **Auto-discovered by the generator — no `sync-agents.sh` edit.** ADR-0008 sub-decision #1: "the generator globs [`agents/docket-*.md`], so adding a wrapper needs no script edit." `sync-agents.sh`'s `user_level_pass` iterates `"$AGENTS_SRC"/docket-*.md`; `short_name` strips `docket-`/`.md`, so this file's config key is **`auto-groom-critic`** — overridable for free via the `.docket.yml agents:` block, and project-level `--check` covers it. No collision with `auto-groom` (distinct short-name).

## 5. The three rewirings (skill-body edits)

| Skill / step | Today (inline) | After 0017 |
|---|---|---|
| `implement-next` step 0 | "…invoke `docket-status` … before selection" | Dispatch the `docket-status` subagent (foreground); on return, re-sync `.docket/`, then select |
| `implement-next` step 6 | "…invoke `docket-adr` to record it … append the returned number to `adrs:`" | Dispatch the `docket-adr` subagent (foreground); append its returned number to `adrs:` in the metadata tree (per decision) |
| `auto-groom` step 3 | "Dispatch a **fresh subagent** … to adversarially attack the draft" | Dispatch the named **`docket-auto-groom-critic`** subagent (foreground); the verdict protocol stays in this step (the dispatch prompt) |

Edits are surgical — they re-point existing invocations, leaving each skill's surrounding procedure (the sync discipline, the `adrs:` write-back, the three-verdict logic) intact.

## 6. `docket-convention` change

Convert the forward-pointer at the convention's "Composition" paragraph (currently *"built in change 0017 … `will spawn` … Until 0017 lands those sub-invocations still run inline"*) to the **present-tense contract**: the three nested-foreground dispatch sites, the git-state-as-contract + re-sync rule, and the dedicated `docket-auto-groom-critic` wrapper (the sixth generated file, wrapping no skill) with its `auto-groom-critic` config key. The "Agent layer" line "Five skills get a wrapper" stays accurate (five *skills* do; the critic is an additional wrapper attached to `auto-groom`) — the composition paragraph introduces the sixth wrapper so no count goes stale.

## 7. Testing strategy

- **`tests/test_sync_agents.sh` — count bump + critic assertions.** The suite hardcodes `"exactly 5 built-in wrappers"` (line ~17) and `"all 5 wrappers land in .claude/agents"` (line ~61), both asserting `= 5`. The sixth wrapper flips these red the instant it lands → **update both to 6**, and add: the critic wrapper exists with `model: opus` / `effort: xhigh`; `skills:` includes `docket-convention` and **excludes** `docket-auto-groom`; a `.docket.yml agents:` override for `auto-groom-critic` produces a project-level file (precedence path) and `--check` flags drift. This is the LEARNINGS #6 stale-count trap — caught up front.
- **Skill-body rewiring assertions.** Sentinel checks that step 0 dispatches `docket-status` as a subagent, step 6 dispatches `docket-adr`, and `auto-groom` step 3 names `docket-auto-groom-critic`. Per LEARNINGS, sentinels are sampling not parsing — **pair them with the whole-branch review**; if asserting *order* of two anchors that can share a line, use byte offsets (`grep -ob`), not `grep -n`.

## 8. ADR

0017's composition realizes what **ADR-0008** already framed and deferred, so it does not reverse it. Two options, **decided at build** (same call 0016 made for its own ADR question):

- A dated **`## Update`** appended to ADR-0008 recording that composition landed and how (foreground + git-as-contract + critic isolation); or
- A **new ADR** if the critic-isolation decision (a critic that loads only the convention, never the designer skill, to prevent self-agreement) reads as a distinct decision with its own consequences.

Lean: the critic-isolation rationale is the one genuinely new, non-obvious decision and may warrant its own short ADR; the rest is an Update to 0008. Recorded via `docket-adr` at build step 6 if warranted; append the number to `adrs:`.

## 9. Reconcile notes (grounded in 0016's shipped reality, 2026-06-16)

Verified against `origin/main` (`f9cbd1f`) after 0016 merged (PR #30):

- **`agents/`** ships exactly the 5 wrappers (`docket-implement-next`, `docket-auto-groom`, `docket-finalize-change`, `docket-status`, `docket-adr`) — **no critic** (0017 adds the 6th).
- **`sync-agents.sh`** globs `agents/docket-*.md` and derives keys via `short_name` ⇒ **no generator edit** for the critic; the override key is `auto-groom-critic`.
- **`.docket.yml`** has the `agents:` block commented out (example only) ⇒ built-in defaults in effect; the critic's opus/xhigh lives in its wrapper frontmatter.
- **`auto-groom` Step 3** already dispatches a fresh (unnamed) subagent ⇒ the edit is to *name* it (`docket-auto-groom-critic`), giving it the pinned tier + isolated context; the three-verdict protocol is unchanged.
- **ADR-0008** is `Accepted` (immutable except status) ⇒ 0017 touches it only via a dated `## Update`, never a body edit.

The build-time reconcile pass re-validates all of the above against whatever is current then.

## 10. Decisions (resolved at brainstorm, with the human)

- **Critic form:** a **dedicated committed wrapper file** (`agents/docket-auto-groom-critic.md`), not an inline-spawned variant — the only portable way to pin *effort* (not just model), config-overridable via the `agents:` block, clone-identical, and consistent with the other five. (Rejected: inline spawn — can't pin effort portably, hard-codes the tier into the skill body, not config-overridable; reuse the `auto-groom` wrapper — injects designer bias into the adversary.)
- **Critic loads `docket-convention` only**, never the `docket-auto-groom` skill body.
- **All three dispatches foreground + unconditional + git-state-as-contract**; no background dispatch; depth ≤ 2; depth-5 cap untouched.
- **Scope stays narrow** (the 3 whole-skill sites); the board-pass→`docket-status`-subagent wiring is a **separate change** to be captured via `docket-new-change`.
- **No `sync-agents.sh` edit** (glob auto-discovers the 6th wrapper); the one mechanical must-do is bumping `test_sync_agents.sh`'s 5→6 counts.
- **ADR:** Update ADR-0008 vs new ADR — decided at build.
