# Consultant-authored brainstorm — design

**Status:** design (brainstormed 2026-07-10 via `docket-new-change`)
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

- **Model mismatch.** Design thinking — approach generation and, critically, the spec
  that feeds `docket-implement-next` — deserves a high tier, but the advisory nudge is
  ignorable: on a cheap session the build-ready artifact is cheap-model prose.
- **No cheap-parent story.** The user cannot run day-to-day sessions on a fast model
  and have docket fan out to intelligence only where it pays.

**What changed since 0016:** the harness now supports continuing a spawned agent
(`SendMessage`, context intact), so "fire-and-forget" is no longer the ceiling. The
ADR-0006 boundary is unchanged and respected: no simulated human — the dialogue stays
with the real human.

## 2. Decision summary

An **opt-in consultant-author pattern**, off by default. Two new artifacts; zero edits
to `docket-new-change` / `docket-groom-next` beyond a one-line verbal-trigger note.

| Artifact | What it is |
|---|---|
| `skills/docket-brainstorm` | A docket-owned brainstorm skill implementing the consultant flow (§3). Bindable via `skills: brainstorm:` (0049 passthrough — no new machinery). |
| `agents/docket-brainstorm-consultant.md` | ADR-0009-pattern wrapper: wraps **no skill**, injects only `docket-convention`, default pin **opus/xhigh**, config key `brainstorm-consultant`, auto-discovered by the `sync-agents.sh` glob. |

The built-in default for the brainstorm role **stays `superpowers:brainstorming`** —
no repo's behavior shifts.

## 3. The consultant flow (`docket-brainstorm`)

The parent (whatever model the session runs) is the conversational front-end; the
pinned consultant does the load-bearing thinking at both ends.

1. **Call 1 — analysis.** Dispatch `docket-brainstorm-consultant` with the stub/idea,
   neighbouring changes (`active/` + recent `archive/`), relevant ADRs, and
   `LEARNINGS.md`. It returns **in-context** (the finalize gate-agent contract, not a
   git-state contract): 2–3 approaches with trade-offs, a recommendation, and the key
   questions to put to the human.
2. **Dialogue — inline, real.** The parent runs the conversation with the human
   directly, one question at a time, using the consultant's material. No relay, no
   auto-answerer (ADR-0006 boundary).
3. **Call 2 — authorship.** The dialogue outcome goes back to the consultant, which
   **authors the spec** and returns the markdown. Mechanism: prefer `SendMessage`
   continuation of the call-1 agent (context intact, no reload); portable fallback is
   a fresh dispatch carrying a full recap (stub + call-1 analysis + the human's
   decisions). The fallback is mandatory to specify because agent continuation is
   harness-specific.
4. **Present + write.** Parent shows the authored spec to the human; change requests
   loop back as further consultant rounds. On approval the parent writes the spec file
   to the configured spec path and **stops at the spec** — the 0049 role contract's
   artifact/stop-point is unchanged.

The spec that feeds the autonomous builder is therefore always pinned-tier prose, even
from a Haiku session; only the conversational glue runs at the session model.

## 4. Activation — off by default, two channels

- **Per-invocation (verbal).** The human mentions it when running the interactive
  skills — e.g. `/docket-new-change "… have a consultant write the spec"` — and the
  skill invokes `docket-brainstorm` for that run regardless of the resolved
  `$SKILL_BRAINSTORM`. Human steering of an interactive session always wins; a
  one-line note in each interactive skill's brainstorm step makes it discoverable.
- **Durable (config).** A repo (or the global config) sets
  `skills: brainstorm: docket-brainstorm` and every brainstorm uses the consultant.
  Costs nothing: the 0049 passthrough already accepts any skill name.

## 5. Degrade rule (ADR-0018 posture)

If the consultant cannot be dispatched (agents not synced, harness without dispatch),
`docket-brainstorm` degrades to running the whole flow inline at the session model
**with a prominent warning** — exactly today's behavior, so the failure mode is "no
worse than now." Never a hard abort: skill/agent availability is a per-machine
property, not a repo-state error.

## 6. ADR

Build time records one ADR (relates_to 8, 9, 18): the consultant-author pattern and
the corrected rationale — ADR-0008's "interactive skills stay inline, advisory only"
is **refined, not reversed** (the interactive skills do stay inline; the brainstorm
*role* they invoke may now fan out its thinking), the fire-and-forget premise fell
when agent continuation arrived, and token cost was never the blocker.

## 7. Out of scope

- `docket-auto-groom` — its designer is already wrapper-pinned and its critic covers
  the adversarial side.
- The plan/build/review/finish roles.
- Any relay/ping-pong machinery (the parent never proxies the dialogue itself) and
  any change to the advisory mechanism (it stays — the parent still runs the
  dialogue, so session model still affects conversational quality).
- Flipping the built-in brainstorm default to `docket-brainstorm` — a possible later
  change once the pattern has mileage.

## 8. Decisions resolved at brainstorm

- Consultant is **author**, not advisor-only — otherwise the build-ready artifact
  remains session-model prose and the model-mismatch problem survives in the
  deliverable.
- New skill via the 0049 seam, **not** consultant calls woven into the interactive
  skill bodies (would fight `superpowers:brainstorming`'s own approach/authoring
  steps) and **not** conditional-on-default dispatch (breaks ADR-0018 passthrough
  opacity).
- Off by default; verbal + config activation (a repo that always wants it shouldn't
  have to ask every time).
- In-context return contract for both consultant calls; continuation preferred,
  recap-dispatch fallback.
- Default pin opus/xhigh, matching the critic's tier rationale: design judgment at or
  above the tier of what it feeds.
