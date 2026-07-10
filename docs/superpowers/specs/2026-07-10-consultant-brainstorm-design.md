# Consultant-authored brainstorm — design

**Status:** design (brainstormed 2026-07-10 via `docket-new-change`; revised same day —
single-dispatch, no convention injection, README deliverable)
**Change:** 0056

## 1. Problem

The brainstorming phase is the only load-bearing design work in docket that runs at
whatever model the session happens to be on. ADR-0008 (change 0016) settled that the
two interactive skills (`docket-new-change`, `docket-groom-next`) stay inline with an
**advisory** model recommendation only, because (a) a brainstorm is live dialogue with
the human and a subagent was fire-and-forget, and (b) a skill cannot force the session
model. The often-recalled "reloading the convention in a subagent is token-inefficient"
argument appears in no ADR or spec and was never the load-bearing rationale — ADR-0009's
critic wrapper injects `docket-convention` into a fresh subagent as routine practice.

Two problems survive that decision:

- **Model mismatch.** Design thinking — critically, the spec that feeds
  `docket-implement-next` — deserves a high tier, but the advisory nudge is ignorable:
  on a cheap session the build-ready artifact is cheap-model prose.
- **No cheap-parent story.** The user cannot run day-to-day sessions on a fast model
  and have docket fan out to intelligence only where it pays.

The ADR-0006 boundary is unchanged and respected: no simulated human — the dialogue
stays with the real human, inline.

## 2. Decision summary

An **opt-in consultant-author pattern**, off by default. Two new artifacts; zero edits
to `docket-new-change` / `docket-groom-next` beyond a one-line verbal-trigger note; one
README deliverable.

| Artifact | What it is |
|---|---|
| `skills/docket-brainstorm` | A docket-owned brainstorm skill implementing the single-dispatch consultant flow (§3). Bindable via `skills: brainstorm:` (0049 passthrough — no new machinery). |
| `agents/docket-brainstorm-consultant.md` | A generated wrapper that wraps **no skill and injects no convention** (§5 — a documented deviation from the ADR-0009 critic pattern): default pin **opus/xhigh**, config key `brainstorm-consultant`, auto-discovered by the `sync-agents.sh` glob. |

The built-in default for the brainstorm role **stays `superpowers:brainstorming`** —
no repo's behavior shifts.

## 3. The consultant flow (`docket-brainstorm`) — single dispatch

The parent (whatever model the session runs) conducts the brainstorm; the pinned
consultant fires **once**, at the end, as author-and-audit. An earlier two-call design
(a pre-dialogue analysis call returning approaches + questions, continued via
`SendMessage` for authorship) was revised out: `SendMessage` agent continuation is a
Claude Code-only construct, the second dispatch doubled cost, and the analysis call
dragged the heaviest context handoff. Single-dispatch is fully portable — one fresh
agent, no continuation anywhere.

1. **Dialogue — inline, real.** The parent explores the idea with the human directly,
   one question at a time, generating approaches and trade-offs itself at the session
   model. No relay, no auto-answerer (ADR-0006 boundary).
2. **Dispatch — author or critique.** Once the design is settled, the parent dispatches
   `docket-brainstorm-consultant` with the settled design, the stub/idea, neighbouring
   changes, relevant ADRs, `LEARNINGS.md` excerpts, and the compact brief (§5). The
   consultant returns **in-context** (the finalize gate-agent contract) exactly one of:
   - **an authored spec** (markdown, ready to write), or
   - **critique concerns** — the settled design has a hole the human must see. The
     parent takes the concerns back to the human, resolves them in dialogue, and
     re-dispatches. This gate is what keeps the pinned tier load-bearing even though
     approach generation ran at the session model: nothing becomes build-ready without
     pinned-tier sign-off.
3. **Present + write.** Parent shows the authored spec to the human; change requests
   loop back as further dispatch rounds. On approval the parent writes the spec file to
   the configured spec path and **stops at the spec** — the 0049 role contract's
   artifact/stop-point is unchanged.

The spec that feeds the autonomous builder is therefore always pinned-tier prose, even
from a Haiku session; the conversational glue and option generation run at the session
model, audited by the consultant's author-or-critique gate.

## 4. Activation — off by default, two channels

- **Per-invocation (verbal).** The human mentions it when running the interactive
  skills — e.g. `/docket-new-change "… have a consultant write the spec"` — and the
  skill invokes `docket-brainstorm` for that run regardless of the resolved
  `$SKILL_BRAINSTORM`. Human steering of an interactive session always wins; a
  one-line note in each interactive skill's brainstorm step makes it discoverable.
- **Durable (config).** A repo (or the global config) sets
  `skills: brainstorm: docket-brainstorm` and every brainstorm uses the consultant.
  Costs nothing: the 0049 passthrough already accepts any skill name.

**README deliverable:** the off-by-default status and both opt-in channels are
documented **prominently** in the repo README (not only in skill bodies) — a top-level
feature section, not a buried footnote.

**Whole-brainstorm model control (documented guidance, not a mechanism).** The
consultant path pins **authorship**; the dialogue and option generation still run at
the session model. When the human wants *all* portions of a brainstorm at a specific
model, the documented pattern is **capture-then-groom**: capture the idea as a stub
with `docket-new-change` in whatever session it strikes (no brainstorm — the stub is
needs-brainstorm), then run `docket-groom-next` from a session set to the desired
model. This is a README/docs note riding this change — it needs no new machinery
(stubs and grooming already work this way) and composes with the consultant path
(a strong-model groom session can still opt into consultant authorship, though it
gains little there).

## 5. Context economics — no convention reload anywhere

- **Parent side:** `docket-brainstorm` is only ever invoked from `docket-new-change` /
  `docket-groom-next`, whose blocking Step 0 already loaded `docket-convention` this
  session; the load rule ("unless already in context") means no reload. The skill body
  itself uses convention vocabulary without redefinition (ADR-0003 posture).
- **Consultant side:** the wrapper injects **nothing** — no skill, no
  `docket-convention`. The dispatch prompt carries a **compact brief** instead (well
  under a page): the spec path and expected format, the PM-altitude boundary (design
  detail belongs in the spec, intent/scope in the change), and the requirement for an
  explicit assumptions section. This deviates from the ADR-0009 critic deliberately
  and safely: the critic judges build-readiness and docket semantics, so it needs the
  convention; the consultant **authors prose and performs zero docket operations** —
  no git, no status writes, no board. The deviation is recorded in the build-time ADR.
  (Change 0053's convention slimming is adjacent but this design does not depend on it.)

## 6. Degrade rule (ADR-0018 posture)

If the consultant cannot be dispatched (agents not synced, harness without dispatch),
`docket-brainstorm` degrades to running the whole flow inline at the session model
**with a prominent warning** — exactly today's behavior, so the failure mode is "no
worse than now." Never a hard abort: skill/agent availability is a per-machine
property, not a repo-state error.

## 7. ADR

Build time records one ADR (relates_to 8, 9, 18): the consultant-author pattern and
the corrected rationale — ADR-0008's "interactive skills stay inline, advisory only"
is **refined, not reversed** (the interactive skills do stay inline; the brainstorm
*role* they invoke may now fan out its authorship), the fire-and-forget premise fell
when agent continuation arrived (though the final design needs no continuation at
all), token cost was never the blocker, and the consultant wrapper's
no-skill/no-convention shape is a documented deviation from ADR-0009.

## 8. Out of scope

- `docket-auto-groom` — its designer is already wrapper-pinned and its critic covers
  the adversarial side.
- The plan/build/review/finish roles.
- Any relay/ping-pong machinery, `SendMessage`/continuation dependence, pre-dialogue
  consultant analysis calls, and simulated-human answering (ADR-0006 boundary).
- Any change to the advisory mechanism (it stays — the parent still runs the
  dialogue, so session model still affects conversational quality).
- Flipping the built-in brainstorm default to `docket-brainstorm` — a possible later
  change once the pattern has mileage.

## 9. Decisions resolved at brainstorm

- Consultant is **author**, not advisor-only — otherwise the build-ready artifact
  remains session-model prose and the model-mismatch problem survives in the
  deliverable.
- New skill via the 0049 seam, **not** consultant calls woven into the interactive
  skill bodies (would fight `superpowers:brainstorming`'s own approach/authoring
  steps) and **not** conditional-on-default dispatch (breaks ADR-0018 passthrough
  opacity).
- Off by default; verbal + config activation (a repo that always wants it shouldn't
  have to ask every time); README documents this prominently.
- **Single dispatch** (revision): the pre-dialogue analysis call was dropped —
  portability (no Claude-only continuation), half the dispatch cost, lighter context.
  The author-or-critique gate preserves pinned-tier audit of every design.
- **No convention injection** (revision): compact brief in the dispatch prompt; the
  consultant performs no docket operations, so the vocabulary risk is nil.
- In-context return contract for the consultant call.
- Default pin opus/xhigh, matching the critic's tier rationale: design judgment at or
  above the tier of what it feeds.
