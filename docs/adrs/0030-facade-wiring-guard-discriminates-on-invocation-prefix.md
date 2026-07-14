---
id: 30
slug: facade-wiring-guard-discriminates-on-invocation-prefix
title: The facade-wiring guard discriminates on the invocation prefix, not the bare presence of a `.sh` token
status: Accepted
date: 2026-07-14
supersedes: []
reverses: []
relates_to: [29]
change: 72
---

## Context

Change 0072 rewires the seven operating skills and the convention's Step-0
preamble to invoke docket helpers only through the `docket.sh` facade (change
0068), and adds `tests/test_skill_facade_wiring.sh` to lock the retirement in.

The spec's tokenizer rule (§4) was internally ambiguous. Its opening clause read
"any `*.sh` token inside a code span is a violation" — a broad reading that would
forbid every script noun — while the same rule also said the human-initiated tier
is "permitted in prose position only, never as a code-span invocation" — a narrow
reading that targets invocations. The two readings diverge by roughly 60 sites and
decide whether `skills/docket-convention/references/agent-layer.md` — a reference
doc whose SUBJECT is `sync-agents.sh`, full of descriptive `sync-agents.sh` /
`link-skills.sh` code spans — must be heavily rewritten. A future maintainer could
"tighten" the guard the wrong way without this record.

## Decision

The Layer-1 absence sweep guards non-facade **invocations**, not every `.sh`
token.

- The discriminator is the invocation prefix `${DOCKET_SCRIPTS_DIR` — every
  retired helper invocation, the eval preamble, and the disable-worktree-hooks
  call carry it — plus the retired shapes `eval "$(`, `fetch origin`, and
  `pull --rebase` in code units, after stripping the two byte-exact canonical
  forms: `…/docket.sh <op>` and the single `…/docket-config.sh --bootstrap`
  carve-out.
- Descriptive **noun** mentions of scripts that lack that prefix
  (`board-refresh.sh`, `render-board.sh`, `sync-agents.sh`, `scripts/<name>.sh`)
  are PERMITTED. Describing mechanics is allowed; only instructing a non-facade
  invocation is retired.
- The code-unit extractor tokenizes fenced blocks (including indented, list-item
  fences) and inline code spans.

### Rejected alternative — forbid every `.sh` token in a code span (the broad reading)

Rejected because it over-scopes into `agent-layer.md` and other reference prose
that legitimately NAMES internal / human-tier scripts, contradicting the change's
own Out-of-scope boundary; and because spec §3 itself states `agent-layer.md` is
"rewired only if the build-time grep finds old shapes in it" — a statement that is
only consistent if descriptive nouns are NOT violations (otherwise that file would
always need rewiring). The narrow reading is also the sounder, more
mutation-testable guard (guards-are-code): a single prefix + shape discriminator
over full code-unit extraction, with no fragile per-noun allowlist.

## Consequences

- Skill prose may NAME docket's internal / setup scripts descriptively, but may
  INVOKE a docket helper only via the canonical `docket.sh <op>` facade spelling
  (plus the one convention-only `docket-config.sh --bootstrap` CREATE_ORPHAN
  carve-out).
- The guard's soundness rests on two things a maintainer must preserve: the
  `${DOCKET_SCRIPTS_DIR`-prefix discriminator, and full code-unit extraction —
  the fence regex MUST match indented fences (`/^[[:space:]]*```/`), or invocation
  blocks inside list items are silently dropped. This exact vacuity was caught and
  fixed at review.
- A future maintainer must NOT tighten the guard to forbid all `.sh` tokens; doing
  so reopens the over-scope this decision rejected.
- This narrows / extends the facade-routing decision recorded in ADR-0029 (change
  0068) onto the consuming (skill-prose) side.

## Update — 2026-07-14 (change 0074): the `docket-config.sh --bootstrap` carve-out is retired

Change 0074 added a `bootstrap` verb to the `docket.sh` facade and rewired the
convention's Step-0 `CREATE_ORPHAN` clause to invoke `docket.sh bootstrap`,
removing the last direct-helper invocation from skill prose. Consequently the
wiring guard (`tests/test_skill_facade_wiring.sh`) changed as follows, WITHOUT
altering this ADR's decision:

- `strip_canonical` now strips only the single canonical facade form
  `…/docket.sh <op>`; the `…/docket-config.sh --bootstrap` strip clause is
  deleted. A prefixed `docket-config.sh` invocation reappearing in skill prose
  therefore survives into the haystack and trips the Layer-1 `${DOCKET_SCRIPTS_DIR`
  sweep.
- The former "carve-out occurs exactly once" assertion is replaced by an explicit
  `== 0` assertion that counts occurrences of the prefixed invocation form
  `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh` across skill
  scope and asserts zero.

The **discriminator is unchanged**: the guard still keys on the invocation
prefix `${DOCKET_SCRIPTS_DIR`, never the bare `docket-config.sh --bootstrap`
string — prose may still NAME `docket-config.sh`/`--bootstrap` descriptively
(the convention's Config section and this ADR both do), and the rejected
"forbid every `.sh` token" alternative stays rejected. The Consequences
sentence above that lists "the one convention-only `docket-config.sh
--bootstrap` CREATE_ORPHAN carve-out" as a permitted invocation no longer holds
after 0074: there is no permitted direct-helper invocation left; `docket.sh
bootstrap` is the sanctioned spelling.
