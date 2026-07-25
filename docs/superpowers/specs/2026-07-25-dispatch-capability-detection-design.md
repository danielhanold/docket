<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0137 — Claude Code dispatch-capability detection: name-based probing silently drops SDD build and review discipline](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0137-forked-claude-code-skills-assume-absent-task-dispatch.md)**
<!-- docket:backlink:end -->

# Dispatch-capability detection and the tiered unavailability posture

Design for change #0137. Establishes how a docket agent determines whether subagent
dispatch is available to it, and what an autonomous run does when it genuinely is not.

## Context

A `docket-implement-next` run on change #0136 reported that its runtime had **no
subagent-dispatch (`Task`) tool**. `superpowers:subagent-driven-development` could therefore
not dispatch fresh per-task implementers, and `superpowers:requesting-code-review` could not
dispatch a reviewer; both degraded to their inline `auto` fallbacks per the Skill layer's
missing-skill rule. PR #124 was sound and the degradation was disclosed in the results file
and the PR body — the honest-degradation posture worked — but the SDD isolation and the
independent review that docket's wrapper advertises did not run.

### What is actually broken

Three findings, established by probe on 2026-07-25 (Claude Code, session model
`claude-opus-5`), correct the stub's premise:

1. **There is no tool named `Task` in current Claude Code.** The subagent-dispatch tool is
   named `Agent`. A `ToolSearch` for `select:Task` returns nothing.
2. **Dispatch and nesting work.** An `Agent`-dispatched subagent has the `Agent` tool, and
   dispatched a sub-subagent successfully (returned `NESTED_OK`). That child also had the
   `Skill` tool, with `superpowers:subagent-driven-development`,
   `superpowers:requesting-code-review`, `superpowers:writing-plans`, and `docket-convention`
   all present — which is a *different* situation from change #0066's
   skill-not-invocable-in-subagent trigger.
3. **Subagent tool sets are partially deferred.** Only a core set loads up front; the rest sit
   behind `ToolSearch` and are invisible until searched for.

`AskUserQuestion` is genuinely absent from a subagent, so ADR-0024's fork-exclusion principle
stands unchanged.

The failure is therefore a **false-negative capability probe**, not a harness capability gap.
Docket's own prose primes an agent to look for a tool named `Task`; a partially-deferred tool
surface means absence is easy to observe without ever having resolved anything. The
missing-skill rule then fires *correctly, on a false premise*, and the run's artifacts still
look complete.

Supporting evidence that this is variance rather than a structural wall: SDD dispatched
successfully on 2026-07-14 (`facade-skill-rewiring` results — "per-task implementer subagents
on `opus`/`sonnet` + controller review + mutation-verification") and partly on 2026-07-17. The
no-dispatch reports are exactly two runs: **#0127** (2026-07-22) and **#0136** (2026-07-24).
This is the `harness-behavior-is-mode-and-version-scoped` learning: an observation about a
harness is scoped to the mode and version it was seen in.

### The naming surface is two sites, not six

A blanket rename would introduce new errors. Four `Task` mentions are **Cursor-scoped and
correct** — Cursor genuinely documents a Task tool:

- `docs/adrs/0017-cursor-dispatch-rule-full-agent-set.md:30`
- `docs/adrs/0026-fork-dispatch-opacity-two-invocation-paths.md:77`
- `skills/docket-convention/references/agent-layer.md:115`
- `README.md:622`

Two live sites are Claude-Code-scoped and wrong:

- `skills/docket-convention/references/agent-layer.md:131`
- `README.md:620`

One more is wrong but **immutable**: `docs/adrs/0024-claude-context-fork-skill-dispatch.md:16`
("that pin holds **only when the skill is reached via a `Task` dispatch**"). An `Accepted` ADR
changes only its `status:` line, so this correction lands as an appended `## Update`, never an
edit.

## Decision

### 1. Capability-resolution rule (docket-convention; normative, harness-neutral)

Before declaring a dispatch-dependent role or composition step unavailable, an agent MUST:

1. Attempt to **resolve** a subagent-dispatch mechanism, **including searching deferred or
   lazily-loaded tool surfaces**.
2. If resolution is inconclusive, **attempt one trivial dispatch**.

Only a failed attempt, or an explicit policy denial, establishes unavailability. **The absence
of a specifically-named tool is never sufficient evidence.**

The rule is stated by capability, never by tool name. The failure diagnostic MAY name what was
searched for (e.g. "no dispatch mechanism resolved; searched `Agent`, `Task`") — that name is
an **observed internal that docket depends on for nothing**, the same posture README already
takes toward the fork transcript path. A tool name is a diagnostic string and never a decision
input.

Rationale for capability-over-name, given that a per-harness name table is the more
deterministic-looking option:

- A stale name produces a **silent false negative**: the check says "degrade to inline," the
  run completes, and the disclosure reads as boilerplate. An attempt-based probe fails only
  when dispatch is genuinely absent.
- The name went stale between 2026-07-17 and 2026-07-22 with **no signal**. A committed name
  table makes a vendor internal load-bearing.
- Attempting is **strictly more accurate** than name-matching, because it also catches the
  reverse error — name present but denied by policy, present but deferred-and-unsearched, or
  capped by nesting depth. The `Explore` and `Plan` agent types are denied dispatch outright;
  a name-presence check would call them capable and then fail.
- A name gate cannot be tested honestly: a sentinel asserting the prose says `Agent` is
  `specified-but-unreachable`, and a fixture omitting the tool routes every test through the
  degrade path and still goes green (`green-suite-untested-branch`).

### 2. Tiered unavailability posture

The dispatch kinds are not equivalent, so a single blanket posture would be wrong in one
direction or the other.

| Tier | Dispatches | When dispatch is genuinely unavailable |
|---|---|---|
| **A — deterministic** | `docket-status` (§0), `docket-adr` (§6) | Run **inline**. Their contract is git state, not an in-context return, so inline execution of the same deterministic orchestrator satisfies it **fully**. Reclassify as a first-class equivalent path — not degradation, not a warning. |
| **B — adversarial** | `docket-auto-groom-critic` | **Abstain.** Self-critique by the agent that authored the draft is not an adversarial gate. `docket-auto-groom` already owns this exact path: flip `auto_groomable: false`, append a dated `## Auto-groom blocked` section, route to the human queue. |
| **C — discipline** | `skills.build`, `skills.review` | **Authorized-or-halt.** An explicitly configured `auto` is the human's authorization to run inline. Any other configured value that cannot dispatch ⇒ **abort-and-report**. |

Tier A obligations are unchanged by running inline: re-sync before reading, derive state from
fresh origin, and never adopt or commit another agent's uncommitted working-tree files.

Tier C's halt uses existing state only: leave the change `in-progress` with `claimed_at`
refreshed and a dated note recording the halt reason. The existing reclaim lease
(`reclaim.lease_ttl`) self-heals an abandoned claim. No new status, no new field.

Tier B's placement is the one refinement beyond a literal two-way split. Putting the critic in
Tier A would let `docket-auto-groom` mark its own homework; halting is strictly worse than the
abstain it already owns.

### 3. Record

- **A new ADR**: capability-gate-not-name detection plus the tiered posture. Written
  **harness-neutral by construction**, and naming the wrapper-delivery problem explicitly as
  change #0135's to solve, so a Claude-Code-first framing does not bake in assumptions #0135
  then has to fight. #0135 cites it — that is the consistent cross-harness posture the stub
  asks for.
- **A dated `## Update` on ADR-0024**, extending its fork-exclusion reasoning from the human
  channel to the dispatch channel and pointing at the new ADR. Additive; supersedes and
  reverses nothing.

Per the `adr-update-delivery` learning, the ADR-0024 body update is delivered **atomically** by
listing both ids in this change's `adrs:` — never a standalone push.

### 4. Relationship to change #0135 (Cursor)

The two are failures at **different layers** with an identical symptom, and neither fix reaches
the other harness:

| | Claude Code (#0137) | Cursor (#0135) |
|---|---|---|
| Broken layer | capability **detection** | wrapper **contract / delivery** |
| Dispatch tool | exists (`Agent`), works | exists (`Task`), documented |
| Skill invocation | works — all four skills present to the probe | **absent** — no Skill tool, `skills:` preload not a Cursor field |
| Fix | a normative rule in docket-convention | a Cursor-specific wrapper emitter |

This change **does not fix Cursor**, and says so. Its fix is delivered as prose in
docket-convention — through the one channel Cursor is broken on, since `skills:` preload is
exactly the field Cursor ignores. #0135 must repair delivery before any convention-level rule,
this one included, has effect there. Symmetrically, #0135 alone does not fix Claude Code: a
conforming Cursor emitter does nothing about a false-negative probe on the fork path.

They stay **`related:`, never `depends_on:`** — neither gates the other, and #0137 is
`critical` while #0135 is `high`.

Cursor's documented **nesting limit** on Task launches is a live question: docket's tree runs
three deep (wrapper → SDD implementer → task reviewer). If the limit is below three, SDD
genuinely cannot run on Cursor, the capability-resolution rule would correctly detect it, and
Tier C would halt — an honest outcome, not a bug. The limit is **not verified here** and
belongs to #0135's open questions.

## Verification

A standing runtime smoke test is **not available**: docket's suite is hermetic bash and cannot
dispatch a subagent. Verification is therefore split.

1. **Structural test** over the capability-resolution rule and the tiered posture, anchored on
   the **consuming** skill sections rather than an allowlist (`correspondence-guard-runs-one-way`
   — a guard over a correspondence proves only the direction it iterates), and asserting a
   population floor so the guard cannot validate zero markers
   (`marker-scoped-guard-needs-a-population-floor`).
2. **Negative guard**: no docket prose gates on a literal tool name. Scoped to exclude the four
   legitimately Cursor-scoped mentions listed above, so the guard does not demand their removal —
   **and scoped to live prose only** (`skills/`, `README.md`, `docs/adrs/`). Archived change files
   and results files are immutable historical records that mention `Task` both as the old tool name
   and as plan-task labels ("Task 10", "Task-1b"); a guard that swept them would either be
   permanently red or force edits to the ledger. See the reconcile addendum.
3. **Live spike at build time**, findings recorded **verbatim in the results file** with the
   Claude Code version (`metadata-branch-invisible-to-suite`; and per
   `harness-behavior-is-mode-and-version-scoped` the findings are version-scoped and must say
   so). It probes **both** invocation paths ADR-0026 names first-class:
   - a real `context: fork` child (skill-invoke), and
   - an agent-dispatched child (`@docket-implement-next`),

   recording for each: whether a dispatch mechanism resolves, whether a trivial nested dispatch
   succeeds, whether `Skill` is present, and which tool name was found. This **answers the
   change's open question (a) as a build task** rather than deferring it.
4. **The `skill-fallback-degrades-discipline` learning** currently records #0136's cause as "the
   run's runtime exposed no subagent-dispatch (Task) tool at all." That is now known to be
   likely a false negative and needs correcting. The correction rides the **close-out harvest** —
   the harvest is the ledger's only writer.

### Gating branch on the spike

The spike is a **gating task**, not a reporting one. If it finds that a real `context: fork`
child categorically lacks dispatch, Tier C would halt **every** forked build — bricking
`/docket-implement-next`, the path ADR-0024 names first-class — unless `auto` is configured.

In that case the change **stops and reports back to the human** rather than shipping a posture
that breaks the primary path. The human then chooses between steering users toward
agent-dispatch and relaxing Tier C. Do not resolve this silently.

## Reconcile addendum — 2026-07-25 (build claim)

Verified against current `main` + `docket` at claim time. The design stands unchanged; four
build-facing constraints the groom pass could not see:

1. **Every cited line number is still accurate.** The two wrong sites
   (`agent-layer.md:131`, `README.md:620`), the immutable `ADR-0024:16`, and all four
   Cursor-scoped mentions (`agent-layer.md:115`, `README.md:622`, `ADR-0017:30`, `ADR-0026:77`)
   are byte-for-byte where the spec says. No re-survey needed.

2. **The size-budget guard has ~zero headroom, so the fix must raise it in-diff.**
   `tests/test_skill_size_budgets.sh` caps `skills/docket-convention/SKILL.md` at 354 lines /
   5850 words; it currently sits at **347 / 5848 — two words of headroom**.
   `skills/docket-implement-next/SKILL.md` sits at 135 / 3307 against 147 / 3315 — **eight
   words**. Adding the capability-resolution rule and the Tier C posture therefore *requires*
   raising those budget rows in the same diff. That is the guard's own sanctioned escape hatch
   ("a change that bloats a skill must slim elsewhere or consciously RAISE the budget in this
   table"), with precedent in changes 0127 and 0102 — but it is a deliberate, reviewable edit,
   never a surprise, and the raise must be justified in the table's comment block.
   `agent-layer.md` (152/168, 1671/1839) and `docket-auto-groom/SKILL.md` (60/66, 1159/1237)
   have real headroom.

3. **Tier C must be drawn against the existing missing-skill rule, not on top of it.** The
   convention's *Skill layer* already says an un-invocable skill degrades to `auto` + warn. That
   stays true and is a **different** condition: #0136's `subagent-driven-development` *was*
   invoked and then could not dispatch from inside. The build must state the boundary explicitly
   — **skill not invocable ⇒ missing-skill rule (degrade + warn); skill invocable but dispatch
   unresolvable ⇒ Tier C (authorized-or-halt)** — or the two rules will read as contradicting
   each other on the same symptom.

4. **The #0135 stub amendment is already done.** Grooming landed it on 2026-07-25 (`0135`'s body
   carries the dated "failure is skill-delivery, not dispatch" note, and its `related:` already
   lists `137`). The only residue for this build is adding the new ADR's id to #0135's `adrs:`
   once the number is assigned. Dropped from the change's scope accordingly.

Also confirmed: **#0136 is `done`** (PR #124 merged 2026-07-24T14:32:40Z), so the
"no retrofit of PR #124" non-goal is now about merged history. #0135 and #0113 are both still
`proposed` / needs-brainstorm — `related:` only, nothing gates this build. Because this repo runs
`terminal_publish: true`, `docs/adrs/` exists on **both** branches; the ADR-0024 `## Update` is a
`docket-adr` write on `docket` that publishes onto `main` at acceptance — **never** a
feature-branch edit.

## Out of scope

- The Cursor instance of the symptom — change #0135, per §4 above.
- Changing the Superpowers SDD, TDD, or code-review skills themselves.
- Reworking `sync-agents.sh` wrapper generation beyond what this fix requires.
- Retrofitting the already-open PR #124 from the #0136 run.
- Renaming the four Cursor-scoped `Task` mentions, which are correct.
- Verifying Cursor's Task nesting limit (#0135).
