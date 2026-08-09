<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0276 — Dummy mode — persona-calibrated human-facing language simplification](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0276-dummy-mode.md)**
<!-- docket:backlink:end -->

# Dummy mode — human-facing language simplification — design

**Date:** 2026-08-09
**Status:** Approved (brainstorm with Daniel, 2026-08-08/09)
**Change:** 0276

## Problem

Docket's generated prose frequently asks the human questions they cannot truly answer, for
two compounding reasons: the voice density is too high (terms the reader does not know) and
the reader's expertise in the repo's programming language may not be deep enough to evaluate
the question. The human's current workaround — "please simplify that question" — works every
time, which is exactly the signal it should be productized.

Dummy mode makes that simplification a configured behavior. It is **for the human only,
never the agent**: agent-facing artifacts keep full technical density, because dumbing them
down would degrade the build loop itself.

## Decision summary

Benchmarked two calibration models:

- **Persona string** — one free-text description of the reader.
- **On/off + numeric intensity** — a boolean plus a "how much" dial.

Persona won: an intensity level only means something relative to a house scale each skill
would re-interpret (and drift on), while a persona says *what* to simplify — the reader's
actual gaps — capturing both of the problem's axes (unknown vocabulary AND language
expertise). Intensity's only real edge (cheap per-surface variation) is recovered by a
per-surface on/off list. The numeric dial is dropped.

The shape merges the two: a top-level `dummy_mode:` key with nested `enabled:` (boolean) and
`persona:` (string), plus an optional `surfaces:` narrowing list.

## Config

```yaml
# .docket.yml (primarily per-repo; all three layers resolve per-field as usual)
dummy_mode:
  enabled: false          # default off
  persona: ""             # who the reader is — optional; blank falls back to the default persona
  surfaces: all           # `all` (default) or a subset list of the surface tokens
```

- **Layer classification: global-able** (not coordination-fenced). It changes prose tone,
  not shared non-re-derivable state. Docs frame it as *primarily per-repo*: the persona
  describes this repo's reader — a user may be an SME in one repo's domain and a novice in
  another's — so the repo-committed file is the natural home, with the user-level and
  machine-local layers available per normal per-field resolution.
- **Validation:** `enabled: true` with a blank/absent `persona:` falls back to the
  **default persona** (below) with a one-line notice — never a warning, never disabled.
  An unknown `surfaces:` token is
  warned-and-ignored (typo must never abort a run); an empty list is equivalent to
  `enabled: false` for eligibility purposes.
- **Resolution:** `docket-config.sh --export` emits `DUMMY_MODE_ENABLED`,
  `DUMMY_MODE_PERSONA`, `DUMMY_MODE_SURFACES` (space-separated tokens, `all` expanded or
  passed literally — implementation's choice, stated in the script contract). Skills read
  the exports, never re-parse YAML — the same pattern as `skills:`. When `persona:` is
  blank, `DUMMY_MODE_PERSONA` carries the default persona string — skills never
  special-case an empty persona.

## Default persona

The shipped fallback when `persona:` is unset, and the calibration ad-hoc enablement uses
when the config defines none. Lives in exactly one place — `docket-config.sh` (and its
contract) — and is quoted in the docs:

> A mid-level software engineer: solid grasp of software architecture and general
> engineering concepts — APIs, testing, CI, version control — but only working-level
> fluency in any specific programming language, so avoid language-specific idioms unless
> glossed. Assume no familiarity with this project's internal vocabulary; introduce each
> project-specific term with a one-clause explanation.

Rationale: a defensible median reader for a technical repo — strips expert idioms without
going ELI5 — and it bakes in the universal half of the problem: project-internal jargon is
unknown to every newcomer at any skill level, so the default always glosses it.

## Ad-hoc session enablement

A human may enable dummy mode **on demand** — "enable dummy mode" in any natural phrasing,
when initiating an interactive skill (a groom, a new-change brainstorm) or mid-session.
Semantics, owned by the shared definition in `docket-convention`:

- **Effect:** the session treats `dummy_mode` as enabled for the eligible surfaces that
  session authors — `dialogue` immediately, plus the `reports` and `change-sections` of any
  skill run inline in that same session — regardless of the resolved `enabled`/`surfaces`
  values.
- **Persona:** the config's resolved `persona:` (which is the default persona when none is
  configured). Ad-hoc enablement never defines its own.
- **Duration:** the rest of the session. The reverse request ("disable dummy mode")
  disables it the same way, also session-scoped.
- **No writes:** no config edit, no env change — a prose rule: a human's in-session request
  overrides the resolved config for the session's duration, the same precedence principle
  the convention already applies to human steering of interactive sessions.
- **Boundary:** session-scoped means *this* session only — a subagent dispatched with its
  own context is not the session; the dispatching prose must carry the enablement forward
  explicitly if its output is a covered surface.

## Surface tokens (v1)

Five tokens; the shared definition in `docket-convention` owns this table.

| Token | Covers | Mode |
|---|---|---|
| `dialogue` | brainstorm/groom conversation (questions, approach proposals, design presentation, spec walkthrough), any question a docket skill asks the human, finalize's human-present prompts | **replace** |
| `reports` | end-of-run reports of the autonomous skills (`docket-implement-next`, `docket-finalize-change`, `docket-status`, `docket-auto-groom`) | **replace** |
| `results` | the close-out results artifact | **additive** |
| `change-sections` | `## Auto-groom blocked`, `## Finalize blocked`, `## Run halted`, `## Why deferred`, `## Why killed` | **additive** |
| `pr` | the PR body | **additive** |

**Replace** = the prose itself is written calibrated to the persona; no separate technical
copy. The underlying decisions and artifacts stay fully technical — only the human-facing
wording changes.

**Additive** = the artifact keeps its full technical content and gains an authored
`### In plain terms` sub-section, written **at the same moment the parent is authored** so
it rides the same commit and respects the frozen-artifact rule — never retro-added to a
merged results file. Plain heading, not marker-bounded: it is authored prose, not a rendered
view, so no script owns it.

### Explicitly not eligible

- **Agent-facing artifacts — never:** plans, the spec **file**, learnings findings, build
  evidence, script contracts. The spec's ambiguity (human-approved, agent-consumed) is
  resolved by simplifying the *walkthrough dialogue* only: the human approves via
  plain-language conversation; the file stays dense.
- **Script-generated views:** `BOARD.md`, GitHub mirror issue bodies, `## Artifacts` /
  backlink blocks, index READMEs, `## Reclaim log` / `## Publish deferred` entries —
  deterministic renderer output, out of scope.
- **Change body (`## Why` etc.)** — already the PM-altitude plain layer.
- **ADRs** — immutable once Accepted and agent-read at plan time.

### Agent-safety rule

Agents are instructed that a `### In plain terms` block is for the human and is **never a
decision input** — reconcile, review, and planning read the technical content only. This
line lives in the shared definition so every reader inherits it.

## Implementation shape

Convention-prose feature; no new scripts.

1. **`docket-config.sh`** — parse `dummy_mode.{enabled,persona,surfaces}` across the three
   layers, validate per above, emit the three exports with the default-persona fallback.
   Update the layer-classification table and export list in `scripts/docket-config.md`.
2. **`docket-convention`** — a new *Dummy mode (shared definition)* section owning the
   token table, replace/additive semantics, the default-persona fallback, the ad-hoc
   session-enablement rule, the agent-safety rule, and the not-eligible list.
3. **Eligible skill bodies** — one-line pointers (read the exports, apply the shared
   definition to the surfaces that skill owns): `docket-new-change`, `docket-groom-next`
   (`dialogue`), `docket-implement-next` (`pr`, `reports`, `change-sections` it writes),
   `docket-finalize-change` (`dialogue`, `reports`, `change-sections`), `docket-status`
   (`reports`), `docket-auto-groom` (`reports`, `change-sections`).
4. **Docs** (README + config docs) — the persona-example gallery below, verbatim or
   near-verbatim.
5. **Tests** — config-resolution coverage (defaults, per-layer override, blank-persona
   warn-and-disable, unknown-token warn-and-ignore, export names/order per the config
   suite's conventions) plus guard tests keeping the skill-body pointers present.

## Persona example gallery (ships in docs)

Requirement: 3–5 fully worked examples spanning different application types and languages —
each shows the config as written and what it changes in practice.

**1. Shell/CLI tooling repo (docket itself) — PM-technical reader**

```yaml
dummy_mode:
  enabled: true
  persona: >
    Comfortable with git, GitHub PRs, and YAML. Cannot read bash or awk — explain
    script behavior by outcome, never by code. Does not know docket's internal
    vocabulary (worktree, CAS push, claim lease, orphan branch) — use each term only
    with a one-clause gloss.
```

Effect: a brainstorm question like "should the CAS retry re-run preflight before the
re-push?" becomes "when two sessions save at the same time, one loses — should the loser
automatically re-sync and retry, or stop and ask you?"

**2. Python data-pipeline repo — analyst reader**

```yaml
dummy_mode:
  enabled: true
  persona: >
    Data analyst: fluent in SQL and pandas, reads simple Python. No infrastructure
    background — Docker, Airflow scheduling internals, and IAM are unknown terms.
    Frame trade-offs in terms of data freshness, correctness, and cost.
  surfaces: [dialogue, reports]
```

**3. TypeScript/React web app — designer/founder reader**

```yaml
dummy_mode:
  enabled: true
  persona: >
    Non-engineer founder: thinks in user flows and screens, not components or state.
    Knows what an API is, not what REST vs GraphQL implies. Avoid all TypeScript
    jargon; describe changes by what the user sees and what could break for them.
```

**4. Terraform/infrastructure repo — application-developer reader**

```yaml
dummy_mode:
  enabled: true
  persona: >
    Backend application developer (Go, Postgres) new to infrastructure-as-code: plan
    vs apply, state files, and drift are new concepts. Comfortable with networking
    basics. Always spell out blast radius: what a change destroys or recreates.
  surfaces: [dialogue, results, pr]
```

**5. iOS/Swift app — backend-engineer reader**

```yaml
dummy_mode:
  enabled: true
  persona: >
    Backend engineer fluent in Java and REST APIs, new to mobile: SwiftUI, the app
    lifecycle, and App Store review constraints are unfamiliar. Map iOS concepts to
    server-side analogies where one exists; flag where the analogy breaks.
```

The gallery's axes, stated in the docs so users can compose their own: **subject-matter
gaps** (knows CI/CD, new to release engineering), **language gaps** (reads Python, not
bash), **tooling/vocabulary gaps** (docket's own terms), and **framing preferences** (what
dimensions trade-offs should be expressed in).

## Out of scope

- Numeric intensity levels (decided against — see Decision summary).
- Per-surface persona overrides (YAGNI; one persona per resolution).
- Simplifying script-rendered views (would require renderer redesign).
- Translating to non-English languages (persona could ask for it, but it is untested and
  unsupported in v1).
- Any agent-behavior change: selection, grooming, building, reviewing are untouched.

## Testing

- Config suite: new-key defaults, layer precedence, blank-persona → default persona
  exported (with notice), unknown token → warned + ignored, export presence/order.
- Guard tests: convention section exists; each eligible skill body carries its pointer;
  the agent-safety line present in the shared definition.
- No runtime prose assertions — the simplification itself is LLM-authored and is exercised
  by use, not by the suite.
