# Dummy mode — mechanics (change 0276)

Dummy mode calibrates docket's **human-facing** prose to a reader the repo describes in
`DUMMY_MODE_PERSONA`. It is loaded on demand — the convention's *Dummy mode (shared definition)*
section carries what every agent needs unprompted, and this file carries the rest. Nothing here
changes what an agent decides or what an agent-facing artifact says: only the wording a human reads
changes, and only on the surfaces below.

## Surface tokens (v1)

`DUMMY_MODE_SURFACES` is the literal `all` (every token, including ones added later) or a
space-separated subset of these five. An unknown token is warned-and-ignored by the resolver; an
empty value means no surface is eligible, which is equivalent to being off.

| Token | Covers | Mode |
|---|---|---|
| `dialogue` | brainstorm/groom conversation — questions, approach proposals, design presentation, spec walkthrough — any question a docket skill asks the human, and finalize's human-present prompts | **replace** |
| `reports` | end-of-run reports of the autonomous skills (`docket-implement-next`, `docket-finalize-change`, `docket-status`, `docket-auto-groom`) | **replace** |
| `results` | the close-out results artifact | **additive** |
| `change-sections` | `## Auto-groom blocked`, `## Finalize blocked`, `## Run halted`, `## Why deferred`, `## Why killed` | **additive** |
| `pr` | the PR body | **additive** |

## Replace

The prose itself is written calibrated to the persona. There is no second, technical copy and no
parallel block — the reader gets one version, written for them. The underlying decisions and
artifacts stay fully technical: a simplified question is still the same question, and a simplified
report still reports the same run.

## Additive

The artifact keeps its full technical content **unchanged** and gains an authored
`### In plain terms` sub-section, written at the same moment the parent is authored so that it
rides the same commit and respects the frozen-artifact rule — never retro-added to a merged results
file or an already-published artifact.

Plain heading, not marker-bounded: this is authored prose, not a rendered view, so no script owns
it, regenerates it, or strips it. It sits inside the parent artifact, after the technical content
it explains.

## Ad-hoc session enablement

A human may enable dummy mode on demand — "enable dummy mode", in any natural phrasing — when
starting an interactive skill or mid-session.

- **Effect:** treat `dummy_mode` as enabled for the eligible surfaces this session authors —
  `dialogue` immediately, plus the `reports` and `change-sections` of any skill run inline in the
  same session — regardless of the resolved `DUMMY_MODE_ENABLED` and `DUMMY_MODE_SURFACES`.
- **Persona:** always the resolved `DUMMY_MODE_PERSONA`, which carries the shipped default when the
  repo configures none. Ad-hoc enablement never defines its own persona.
- **Duration:** the rest of the session, and it writes nothing — no config edit, no env change, no
  file touched. The reverse request ("disable dummy mode") turns it off the same way, also
  session-scoped.
- **Boundary:** a dispatched subagent has its own context and is **not** this session. If a child's
  output is a covered surface, the dispatching prose must carry the enablement and the persona
  forward explicitly, or the child writes untranslated prose.

## Not eligible

- **Agent-facing artifacts — never eligible:** plans, the spec **file**, learnings findings, build
  evidence, and script contracts keep full technical density. A spec is human-approved but
  agent-consumed, and the resolution is to simplify the *walkthrough dialogue* only: the human
  approves in plain-language conversation while the file stays dense.
- **Script-generated views:** `BOARD.md`, GitHub mirror issue bodies, `## Artifacts` and backlink
  blocks, index READMEs, `## Reclaim log` and `## Publish deferred` entries. Deterministic renderer
  output, out of scope.
- **The change body's `## Why`** — already the PM-altitude plain layer.
- **ADRs** — immutable once Accepted, and read by agents at plan time.

## Authoring guidance

- Gloss every project-internal term on its first use in the piece. Project vocabulary is unknown to
  every newcomer at any skill level, so this holds even for an expert persona.
- Frame trade-offs in the dimensions the persona names — data freshness and cost, blast radius,
  what the user sees — rather than in the repo's own.
- Never drop a decision, a caveat, or an option to make the prose simpler. Simplification is about
  **vocabulary and framing, never about content**: a question the human cannot answer is a failure,
  and so is a question that hides the thing they needed to weigh.
- Prefer a concrete consequence over an abstraction ("the second save loses its work" beats "a CAS
  push races"), and keep the technical term alongside its gloss the first time, so the human can
  still search for it.
