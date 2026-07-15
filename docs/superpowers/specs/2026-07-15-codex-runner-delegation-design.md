# Codex runner delegation — design

**Date:** 2026-07-15
**Change:** #0077
**Status:** Approved (brainstormed with Daniel; whole-run topology, explicit `runner:` field, shim-wrapper mechanism)

## Problem

Every docket agent dispatch under the Claude Code harness runs as a Claude Code subagent, billed
against the Claude subscription and limited to Claude models. Daniel also holds an OpenAI Codex
subscription (ChatGPT plan) with its own usage capacity, and Codex CLI is a capable agentic
harness — superpowers is installed there and works, including `subagent-driven-development` via
Codex's native multi-agent support. There is no way to say "run this docket agent on Codex."

Two candidate bridges were researched:

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

## Decisions (settled in brainstorm)

1. **Topology: whole-run delegation only.** An agent whose config says Codex runs *entirely* in
   Codex via one `codex exec` call. A delegated orchestrator's own sub-dispatches (e.g. SDD's
   implementer/reviewer fan-out) happen Codex-natively via `spawn_agent`/`wait_agent`
   (`[features] multi_agent = true` in `~/.codex/config.toml` — a documented prerequisite).
   A *mixed* topology (Claude Code orchestrator routing individual SDD build leaves to
   `codex exec`, i.e. a Codex-backed `build.implementer`/`build.reviewer` from #0044) is
   **deferred** to a possible follow-up change — it is a second mechanism with its own prompt
   marshalling, and whole-run delegation subsumes its main win.
2. **Activation: explicit `runner:` field, never model-ID sniffing.** Model-prefix detection
   (`gpt-*` ⇒ delegate) was considered and rejected: ADR-0015 makes agent `model:` values opaque
   passthrough strings docket never validates or interprets, and a prefix list misfires both ways
   (`o4-mini`, aliases; and it can't express intent explicitly). `runner:` keeps `model:` opaque —
   it is handed verbatim to `codex exec --model`.
3. **Mechanism: shim wrapper + deterministic dispatch script.** `sync-agents.sh` still generates
   the normal per-agent wrapper file; when the agent resolves `runner: codex` the generated *body*
   is a shim that makes one foreground Bash call to a new `codex-dispatch.sh` (via the `docket.sh`
   facade). Every existing invocation path — `context: fork` self-dispatch, `@docket-<name>`
   dispatch, composition dispatches from other skills — already lands on the wrapper, so all of
   them inherit delegation unchanged and unknowingly. Mechanics live in a script with a co-located
   contract (ADR-0012 script-vs-model boundary). Skill-body branching at every dispatch site was
   rejected (prose duplication across seven skills, drift risk, breaks "wrappers own the pin").
4. **Skill availability: `link-skills.sh` grows a codex leg.** Docket's skills get linked into
   Codex's skill-discovery location at install time (mirroring how superpowers installs there), so
   the dispatch prompt can just say "invoke skill `docket-<x>`". Inline prompt injection of skill
   bodies was rejected (huge prompts; on-demand `references/*.md` reads would break).
5. **Sandbox: configurable, default safe.** Default posture `--sandbox workspace-write` with
   network access enabled and approvals `never` (git push and `gh` must work); overridable via a
   new `runner_codex:` config block per repo or machine. `danger-full-access` is available as an
   explicit opt-in, never the default.

## Design

### 1. Config surface

- `agents.<harness>.<agent>` entries gain an optional **`runner:`** key. Values: `native`
  (default; today's behavior) | `codex`. Honored for the **`claude`** harness only in this change;
  a `runner:` under any other harness key is warned-and-ignored (same posture as unknown harness
  tokens).
- Resolution rides the existing field-by-field four-layer merge (repo-local > repo-committed >
  global > built-in). `runner` is **global-able** — a machine preference in the same class as
  `model`/`effort`, not a coordination key (it writes no shared state; the same change built via
  Codex or Claude lands identically on git).
- `effort:` maps to Codex's reasoning-effort setting (`model_reasoning_effort`); `effort: auto`
  and omitted keep their existing semantics (auto ⇒ don't pass a value; omitted ⇒ built-in effort,
  passed through the mapping).
- New optional **`runner_codex:`** block (any layer; not coordination-fenced):

  ```yaml
  runner_codex:
    sandbox: workspace-write   # workspace-write (default) | danger-full-access
    network: true              # default true — pushes and gh need it
    # approvals are always `never` — codex exec is non-interactive by definition
  ```

### 2. Generation (`sync-agents.sh`)

- When an agent resolves `runner: codex`, the generated `docket-<name>.md` wrapper body is a
  **shim**: it instructs exactly one foreground Bash call —
  `"${DOCKET_SCRIPTS_DIR:?…}"/docket.sh codex-dispatch --agent <name> --model <m> [--effort <e>] [-- <args…>]`
  — with a maxed timeout, then relays the dispatch report and verifies the child's contract
  exactly as a native caller would (git state on `origin/docket` for state-contract agents;
  the relayed report for in-context-report agents).
- Wrapper frontmatter stays as today (discoverable, dispatchable; the `model:` line remains for
  bookkeeping — the effective pin is enforced by the script arguments, not by Claude Code).
- `sync-agents.sh --check` covers shim staleness identically to any generated file. Pruning,
  gitignore-block maintenance, and always-full-set generation are unchanged.

### 3. Dispatch script (`scripts/codex-dispatch.sh` + `scripts/codex-dispatch.md`)

Deterministic mechanics, in order:

1. **Preflight:** `codex` binary on PATH and authenticated (cheapest reliable auth probe chosen at
   build). Failure is **loud abort-and-report** — never a silent degrade to a native run, because
   `runner: codex` was explicit human config.
2. **Prompt assembly:** "Invoke skill `docket-<x>`" + passthrough args + the abort-and-report rule
   (a Codex run has no human channel; unmet preconditions are surfaced and stopped on, never
   prompted).
3. **Flags:** `--model` verbatim from config; effort via the reasoning-effort mapping; sandbox +
   network from `runner_codex:`; approvals `never`; repo root as working directory.
4. **Execution:** run `codex exec` **foreground**, blocking until exit. Capture the final message
   (`--output-last-message` / JSON mode — exact flag pinned at build against the installed Codex
   version) and relay it on stdout; exit nonzero with stderr diagnostics on any Codex failure.

### 4. Skill availability in Codex (`link-skills.sh` codex leg)

- Link docket's skills (including `docket-convention` and its references) into Codex's
  skill-discovery location; exact path verified at build against the superpowers-for-Codex install.
- **Documented prerequisites (not automated):** Codex CLI installed and authenticated; superpowers
  installed in Codex; `[features] multi_agent = true` (required so a delegated orchestrator's SDD
  fan-out works). `codex-dispatch.sh`'s preflight checks what it cheaply can; the rest is README.

### 5. Eligibility & composition

- Only **autonomous** wrappers are delegatable — the nine generated wrappers. The two interactive
  skills (`docket-new-change`, `docket-groom-next`) stay inline with the human; the fork-exclusion
  principle extends verbatim: `codex exec` has no human channel.
- **Per-agent granularity.** A natively-running parent that dispatches a `runner: codex` child
  simply invokes the shim wrapper: in-context-report leaves (auto-groom-critic, rebase-resolver,
  integration-repair, brainstorm-consultant) return via the shim's stdout relay; git-state-contract
  children (status, adr) verify exactly as today.
- **Inside a fully delegated orchestrator,** child dispatches resolve Codex-side (`spawn_agent` +
  the linked skills). **Known limitation:** per-child model pins from the `agents:` block do not
  carry into Codex — Codex-side children run at Codex's own defaults. Accepted for this change;
  revisit only if it bites.

### 6. Failure posture & testing

- Every failure is abort-and-report (autonomous context, no human to prompt).
- **Hermetic tests:** a fake `codex` binary on PATH asserting assembled flags and prompt,
  simulating missing auth, nonzero exit, and final-message capture; sync-agents tests asserting
  shim-body generation, `--check` staleness, and warned-ignored `runner:` under non-claude
  harnesses; config tests for `runner`/`runner_codex` layer resolution.
- **Live verification at build:** one real `codex exec` smoke dispatch of the cheapest agent
  (status) end-to-end.
- Likely **two ADRs** at build time: (a) the explicit `runner:` field as the delegation switch and
  its relationship to ADR-0015 passthrough; (b) the shim-wrapper mechanism (all invocation paths
  inherit delegation through the generated wrapper).

## Out of scope

- The mixed topology: routing individual SDD `build.implementer`/`build.reviewer` dispatches from a
  Claude Code-hosted orchestrator to `codex exec` (possible follow-up change).
- Runners other than Codex (`gemini-cli`, …) — the `runner:` enum is deliberately extensible but
  only `codex` is implemented.
- `runner:` support under non-`claude` harness keys (Cursor etc.) — warned-and-ignored.
- Automating Codex install/auth/superpowers setup — documented prerequisites.
- Carrying per-child model pins into Codex-side sub-dispatches.
- Any read-back of Codex session state (transfer/resume features of the plugin).

## Open questions (resolve at build)

1. Exact Codex skill-discovery path for the `link-skills.sh` codex leg.
2. Exact final-message capture flag on the installed Codex version.
3. Whether delegating `docket-finalize-change` to Codex sidesteps Claude Code's merge-without-review
   classifier — interacts with #0062's authorization design; **policy question, flagged not
   decided** (delegation must not become a classifier bypass without an explicit authorization
   story).
4. Whether model aliases like `spark` need mapping (lean: no — pure passthrough; the user writes
   the real ID).
