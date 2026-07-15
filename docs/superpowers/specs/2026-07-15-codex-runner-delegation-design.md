# Cross-harness runner delegation — design (first runner: OpenAI Codex)

**Date:** 2026-07-15
**Change:** #0079
**Status:** Approved (brainstormed with Daniel; whole-run topology, explicit `runner:` field, shim-wrapper mechanism; repositioned 2026-07-15 as an extensible framework — any parent harness delegating to any child harness — with claude→codex as the first shipped pair)

## Problem

Every docket agent dispatch runs on the harness hosting the session — under Claude Code, as a
Claude Code subagent, billed to the Claude subscription and limited to Claude models. Daniel also
holds an OpenAI Codex subscription (ChatGPT plan) with its own usage capacity, and Codex CLI is a
capable agentic harness — superpowers is installed there and works, including
`subagent-driven-development` via Codex's native multi-agent support. There is no way to say "run
this docket agent on Codex."

The general shape of the gap: a **parent harness** (where the docket session runs) cannot delegate
an agent's whole run to a **child harness** (a different agentic CLI with its own subscription,
models, and skills). Codex-from-Claude-Code is the motivating pair, but the same seam should later
admit other children (e.g. `gemini-cli`) and other parents (e.g. Cursor) without redesign. This
change builds the framework and ships **one** pair: parent `claude`, child `codex`.

Two candidate bridges for the codex child were researched:

- **`openai/codex-plugin-cc`** — OpenAI's official Claude Code plugin. It is a *human-facing
  convenience layer*: slash commands (`/codex:review`, `/codex:rescue`, `/codex:transfer`) plus
  background-job management (`/codex:status`, `/codex:result`, `/codex:cancel`). Its delegation
  primitive takes free-text prompts; it knows nothing about skills, wrappers, or docket's dispatch
  contract, and its job model is background-oriented — the opposite of docket's foreground-blocking
  dispatch rule. **Rejected as the mechanism** (also adds a Node plugin dependency for what is one
  CLI call).
- **`codex exec`** — the non-interactive CLI primitive. Synchronous, scriptable, takes a prompt +
  `--model`, honors the ChatGPT-subscription auth, supports sandbox flags and final-message capture.
  This maps directly onto docket's "parent actively blocks on the child" contract. **Chosen.**
  The framework generalizes this: a child harness qualifies as a runner exactly when it has such a
  non-interactive, foreground, scriptable exec primitive.

## Decisions (settled in brainstorm; framework framing added after)

1. **Topology: whole-run delegation only.** An agent whose config names a runner runs *entirely*
   on the child harness via one exec-primitive call. A delegated orchestrator's own sub-dispatches
   (e.g. SDD's implementer/reviewer fan-out) happen child-natively — for Codex, via
   `spawn_agent`/`wait_agent` (`[features] multi_agent = true` in `~/.codex/config.toml`, a
   documented prerequisite). A *mixed* topology (parent-hosted orchestrator routing individual SDD
   build leaves to the child, i.e. a Codex-backed `build.implementer`/`build.reviewer` from #0044)
   is **deferred to #0044's redesign pass** (folded there 2026-07-15; #0044 is blocked pending it)
   — it is a second mechanism with its own prompt marshalling, and whole-run delegation subsumes
   its main win.
2. **Activation: explicit `runner:` field, never model-ID sniffing.** Model-prefix detection
   (`gpt-*` ⇒ delegate) was considered and rejected: ADR-0015 makes agent `model:` values opaque
   passthrough strings docket never validates or interprets, and a prefix list misfires both ways
   (`o4-mini`, aliases; and it can't express intent explicitly). `runner:` names a **registered
   runner**; `model:` stays opaque — handed verbatim to the child (for codex,
   `codex exec --model`).
3. **Mechanism: shim wrapper + deterministic dispatch script.** `sync-agents.sh` still generates
   the normal per-agent wrapper file for the parent harness; when the agent resolves a `runner:`
   the generated *body* is a shim that makes one foreground Bash call to the runner dispatch
   facade. Every existing invocation path — `context: fork` self-dispatch, `@docket-<name>`
   dispatch, composition dispatches from other skills — already lands on the wrapper, so all of
   them inherit delegation unchanged and unknowingly. Mechanics live in scripts with co-located
   contracts (ADR-0012 script-vs-model boundary). Skill-body branching at every dispatch site was
   rejected (prose duplication across seven skills, drift risk, breaks "wrappers own the pin").
4. **Skill availability on the child: already in place for codex.** `link-skills.sh` already links
   docket's skills into `~/.codex/skills` (established by change #0077, codex-harness-toml-agents),
   so the dispatch prompt can just say "invoke skill `docket-<x>`". This change only verifies
   discovery works from a `codex exec` run. Inline prompt injection of skill bodies was rejected
   during brainstorm (huge prompts; on-demand `references/*.md` reads would break). A future runner
   must state its own skill-availability story in its adapter contract.
5. **Sandbox: configurable, default safe.** Default posture for codex:
   `--sandbox workspace-write` with network access enabled and approvals `never` (git push and `gh`
   must work); overridable via per-runner config. `danger-full-access` is available as an explicit
   opt-in, never the default. Sandbox knobs are per-runner (each child CLI has its own sandbox
   vocabulary).

## Design

The framework has three extension seams, each keyed by the runner name: the **config surface**
(`runner:` + `runners.<name>:`), the **shim generation** (a runner registry in `sync-agents.sh`),
and the **dispatch adapter** (a per-runner script behind one facade verb). Adding a future pair
touches only those seams — no skill-body changes, no new dispatch semantics.

### 1. Config surface

- `agents.<harness>.<agent>` entries gain an optional **`runner:`** key. Value: the name of a
  **registered runner** (this change registers exactly one: `codex`); unset = native dispatch on
  the parent harness (today's behavior). An unregistered runner name is a loud generation-time
  error (explicit config, never silently ignored).
- The harness key the entry sits under is the **parent**: `agents.claude.<agent>.runner: codex`
  means "when Claude Code is the running harness, delegate this agent to codex." This change
  implements and verifies the `claude` parent only; a `runner:` under any other harness key is
  warned-and-ignored (not an error — the framework reserves the meaning; e.g. a future
  `agents.cursor.<agent>.runner: codex` slots in without schema change).
- Resolution rides the existing field-by-field four-layer merge (repo-local > repo-committed >
  global > built-in). `runner` is **global-able** — a machine preference in the same class as
  `model`/`effort`, not a coordination key (it writes no shared state; the same change built via
  any harness lands identically on git).
- `effort:` maps through the runner adapter to the child's reasoning-effort setting (for codex,
  `model_reasoning_effort`); `effort: auto` and omitted keep their existing semantics (auto ⇒
  don't pass a value; omitted ⇒ built-in effort, passed through the mapping).
- New optional **`runners:`** config block (any layer; not coordination-fenced), namespaced
  per runner — each runner defines its own keys in its adapter contract:

  ```yaml
  runners:
    codex:
      sandbox: workspace-write   # workspace-write (default) | danger-full-access
      network: true              # default true — pushes and gh need it
      # approvals are always `never` — the exec primitive is non-interactive by definition
  ```

### 2. Generation (`sync-agents.sh`)

- A **runner registry** in `sync-agents.sh`, parallel in shape to #0077's per-harness emitter
  registry: runner name → shim emitter. When an agent resolves `runner: <name>`, the generated
  parent-harness wrapper body is that runner's **shim**: it instructs exactly one foreground Bash
  call —
  `"${DOCKET_SCRIPTS_DIR:?…}"/docket.sh runner-dispatch --runner <name> --agent <agent> --model <m> [--effort <e>] [-- <args…>]`
  — with a maxed timeout, then relays the dispatch report and verifies the child's contract
  exactly as a native caller would (git state on `origin/docket` for state-contract agents; the
  relayed report for in-context-report agents). The shim body is runner-parameterized, not
  runner-specific prose — one template, the registry fills the name.
- Wrapper frontmatter stays as today (discoverable, dispatchable; the `model:` line remains for
  bookkeeping — the effective pin is enforced by the script arguments, not by the parent harness).
- `sync-agents.sh --check` covers shim staleness identically to any generated file. Pruning,
  gitignore-block maintenance, and always-full-set generation are unchanged.

### 3. Dispatch facade + per-runner adapters

- **`scripts/runner-dispatch.sh`** (+ contract `.md`) — the runner-neutral facade behind
  `docket.sh runner-dispatch`. It validates arguments, resolves the `runners.<name>:` config, and
  hands off to the named adapter. Unknown runner ⇒ loud nonzero exit (abort-and-report).
- **Per-runner adapter** — `scripts/runners/<name>.sh` (+ contract `.md`) owns everything
  child-specific. The adapter contract (what every runner must implement):
  1. **Preflight:** child binary on PATH and authenticated (cheapest reliable probe). Failure is
     **loud abort-and-report** — never a silent degrade to a native run, because `runner:` was
     explicit human config.
  2. **Prompt assembly:** "Invoke skill `docket-<x>`" + passthrough args + the abort-and-report
     rule (a delegated run has no human channel; unmet preconditions are surfaced and stopped on,
     never prompted).
  3. **Flag mapping:** `--model` verbatim (ADR-0015 passthrough); effort via the child's
     reasoning-effort vocabulary; sandbox/network from `runners.<name>:`; approvals `never`; repo
     root as working directory.
  4. **Execution:** run the child's exec primitive **foreground**, blocking until exit. Capture
     the final message and relay it on stdout; exit nonzero with stderr diagnostics on any child
     failure.
- **`scripts/runners/codex.sh`** — the first adapter: `codex exec`, final-message capture via
  `--output-last-message`/JSON mode (exact flag pinned at build against the installed Codex
  version), sandbox flags per §1.

### 4. Skill availability on the child harness

- For codex: docket's skills (including `docket-convention` and its references) are already linked
  into `~/.codex/skills` by `link-skills.sh` (change #0077); verify at build that a `codex exec`
  run discovers them.
- **Documented prerequisites (not automated):** Codex CLI installed and authenticated; superpowers
  installed in Codex; `[features] multi_agent = true` (required so a delegated orchestrator's SDD
  fan-out works). The adapter's preflight checks what it cheaply can; the rest is README.
- Framework rule: every future adapter documents its own skill-availability story and
  prerequisites in its contract; the facade assumes nothing child-specific.

### 5. Eligibility & composition

- Only **autonomous** wrappers are delegatable — the nine generated wrappers. The two interactive
  skills (`docket-new-change`, `docket-groom-next`) stay inline with the human; the fork-exclusion
  principle extends verbatim: an exec primitive has no human channel. This is a framework rule,
  not a codex rule.
- **Per-agent granularity.** A natively-running parent that dispatches a runner-delegated child
  simply invokes the shim wrapper: in-context-report leaves (auto-groom-critic, rebase-resolver,
  integration-repair, brainstorm-consultant) return via the shim's stdout relay; git-state-contract
  children (status, adr) verify exactly as today.
- **Inside a fully delegated orchestrator,** child dispatches resolve on the child harness (for
  codex: `spawn_agent` + the linked skills). **Known limitation:** per-child model pins from the
  `agents:` block do not carry into the child harness — its own defaults apply there. Accepted for
  this change; revisit only if it bites.

### 6. Documentation

- README: a new **`### Runner delegation — running docket agents on another harness`** subsection
  under **`## Customization`** — the framework (parent/child, `runner:` + `runners.<name>:`,
  whole-run semantics, autonomous-only eligibility), then the codex runner specifics (sandbox
  block, prerequisites: Codex CLI + auth, superpowers in Codex, `multi_agent = true`).
- README: **`## Tuning agent models & effort`** gains a short pointer + link to that subsection
  (the `runner:` key appears in the same `agents:` entries that section documents, so readers
  tuning models discover delegation there).
- The convention's *Agent layer* reference (`references/agent-layer.md`) documents the `runner:`
  key alongside `model:`/`effort:` resolution.

### 7. Failure posture & testing

- Every failure is abort-and-report (autonomous context, no human to prompt).
- **Hermetic tests:** a fake `codex` binary on PATH asserting assembled flags and prompt,
  simulating missing auth, nonzero exit, and final-message capture; facade tests for unknown-runner
  rejection and adapter handoff; sync-agents tests asserting shim-body generation from the runner
  registry, `--check` staleness, unregistered-`runner:` generation error, and warned-ignored
  `runner:` under non-claude harnesses; config tests for `runner`/`runners.<name>` layer
  resolution.
- **Live verification at build:** one real `codex exec` smoke dispatch of the cheapest agent
  (status) end-to-end.
- Likely **two ADRs** at build time: (a) the runner-delegation framework — explicit `runner:`
  field as the switch (relationship to ADR-0015 passthrough), parent/child seams, whole-run
  semantics; (b) the shim-wrapper mechanism (all invocation paths inherit delegation through the
  generated wrapper).

## Out of scope

- The mixed topology: routing individual SDD `build.implementer`/`build.reviewer` dispatches from a
  parent-hosted orchestrator to a child harness — folded into #0044's redesign
  (`build.<role>.runner: codex`).
- Additional runner adapters (`gemini-cli`, …) and additional parent harnesses (Cursor, …) — the
  seams ship; only the claude→codex pair is implemented and verified.
- Automating Codex install/auth/superpowers setup — documented prerequisites.
- Carrying per-child model pins into child-harness sub-dispatches.
- Any read-back of child session state (transfer/resume features of the plugin).

## Open questions (resolve at build)

1. Exact final-message capture flag on the installed Codex version.
2. Whether delegating `docket-finalize-change` to Codex sidesteps Claude Code's merge-without-review
   classifier — interacts with #0062's authorization design; **policy question, flagged not
   decided** (delegation must not become a classifier bypass without an explicit authorization
   story).
3. Whether model aliases like `spark` need mapping (lean: no — pure passthrough; the user writes
   the real ID).
4. Whether #0077's TOML agents (`.codex/agents/docket-*.toml`) give a delegated orchestrator's
   Codex-side children resolvable model pins, softening the accepted pin-loss limitation in §5.
