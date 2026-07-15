---
id: 37
slug: runner-delegation-explicit-runner-field
title: Cross-harness runner delegation is switched by an explicit runner field, never model-ID sniffing
status: Accepted
date: 2026-07-15
supersedes: []
reverses: []
relates_to: [15, 12]
change: 79
---

## Context

Change 0079 lets a docket agent's whole run be delegated from the parent harness hosting
the session (Claude Code) to a child harness with its own subscription, models, and skills
(first pair: OpenAI Codex via `codex exec`). Something has to decide WHICH agents delegate.
Two candidate switches were considered: infer delegation from the agent's `model:` value
(a `gpt-*` prefix means "this is a Codex model, so delegate"), or an explicit new per-agent
config key. Model-prefix inference initially looks attractive — zero new config surface.

But ADR-0015 makes agent `model:` values opaque passthrough strings that docket never
validates or interprets, and that opacity is load-bearing: it is exactly what lets docket
drive arbitrary harnesses without a model registry. A prefix list also misfires in both
directions — `o4-mini` carries no `gpt-` prefix, aliases and future IDs break the list, and
a user who wants a Claude-compatible ID executed on a different harness cannot express that
intent at all.

## Decision

Delegation is activated ONLY by an explicit **`runner: <name>`** key on an `agents:` entry,
naming a **registered runner** (registered = a `scripts/runners/<name>.sh` adapter exists;
change 0079 registers exactly `codex`). `model:` stays an opaque passthrough handed to the
child verbatim (for codex, `codex exec -m`); docket never sniffs, validates, or maps model
IDs to make a delegation decision.

The framework's seams are keyed by the runner name: the config surface (`runner:` on agent
entries + a per-runner `runners.<name>:` knob block, both global-able — machine preferences
in the same class as `model`/`effort`, coordination-fence exempt), the generation seam (a
runner registry in `sync-agents.sh`), and the dispatch seam (the `runner-dispatch.sh`
facade + per-runner adapters). The harness key an entry sits under names the PARENT; only
`claude` is implemented, and `runner:` under any other harness key is reserved and
warned-and-ignored. An unregistered runner name is a loud generation-time error, and a
delegated run that fails preflight aborts loudly — explicit config is never silently
ignored and never silently degraded to a native run. Whole-run delegation only: a delegated
orchestrator's own sub-dispatches run child-natively; only autonomous wrappers are
delegatable (an exec primitive has no human channel). Delegation must never be used as a
policy bypass (e.g. delegating finalize to sidestep merge-approval gates — change 0062).

## Consequences

- Delegation intent is always visible in config — greppable, reviewable, revertible; no
  behavior change can ride in on a model-ID rename.
- ADR-0015's passthrough stays intact end-to-end: new child harnesses need no model
  registry, and unknown IDs fail in the child with the child's own diagnostics.
- Costs one more config key to learn, and a user CAN misconfigure a Claude model onto the
  codex runner — the child then errors loudly at dispatch time rather than docket
  second-guessing the pairing.
- Future pairs (other children like `gemini-cli`, other parents like Cursor) slot into the
  reserved seams without schema change.
