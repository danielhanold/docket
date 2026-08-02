<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0192 — opencode support for profile-routed Docket builds](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0192-opencode-profile-routed-build-support.md)**
<!-- docket:backlink:end -->

# opencode support for profile-routed Docket builds

Design for making Docket's complete agent set natively generated and pinned under opencode,
including the four profile-routed build workers and three review rungs, without adding an
opencode-specific controller path. Unlike change 0169 (Codex), which only shipped defaults into an
existing substrate, this change builds the substrate too: harness registration, a native emitter,
and dispatch wiring, plus the shipped sixteen-agent default block and live certification.

## Context

Change 0167 shipped Docket's profile-routed `docket-build` controller; change 0168 moved every
shipped model/effort default into the harness-indexed `agents/harness-defaults.yml` sidecar
(ADR-0064: sparse by whole harness, complete within every shipped harness); change 0169 completed
the Codex harness. Claude, Cursor, and Codex each cover all sixteen wrapper sources today.

opencode (<https://opencode.ai>) is a terminal agent harness with native subagent support:

- Agents are markdown files with YAML frontmatter in `.opencode/agents/` (per-project) or
  `~/.config/opencode/agents/` (global); the filename is the agent identifier.
- Frontmatter fields include `description`, `mode` (`primary` | `subagent` | `all`), `model`
  (`provider/model-id` syntax), `temperature`, `permission`, and `hidden`. Unrecognized options
  are passed through directly to the provider as model options (the documented example is
  `reasoningEffort: "high"`), so a real per-agent reasoning effort is expressible even though
  opencode has no first-class effort token.
- opencode reads the same committed project-root `AGENTS.md` that Codex reads (with `CLAUDE.md` as
  a fallback), and primary agents invoke subagents natively by name — automatically or via
  `@`-mention.
- Models are used through OpenRouter here, so resolved IDs are double-prefixed
  (`openrouter/<provider>/<model>`); the sidecar's bare-scalar rule already tolerates slashes.

opencode has none of Docket's existing substrate: no known-harness token, no emitter in
`sync-agents.sh`'s registry, no dispatch wiring, no sidecar block.

## Goals

- Register opencode as a known and shipped harness; ship complete native defaults for every
  generated Docket agent.
- Generate valid opencode agent definitions (`.opencode/agents/docket-*.md`) carrying the
  resolved model and a per-agent reasoning-effort passthrough.
- Route dispatch through the existing committed `AGENTS.md` managed block, shared with Codex.
- Preserve the harness-neutral routing rubric, outcome protocol, escalation graph, TDD contract,
  full-suite gate, and field-level user overrides above the shipped sidecar.
- Promote `.docket.example.yml`'s coverage with a singly commented exact-mirror opencode block.
- Certify the economy, standard, and premium build-profile dispatches in a live opencode session.

## Non-goals

- A Claude-to-opencode whole-run runner. `REGISTERED_RUNNERS` is unchanged; runner delegation for
  opencode is a possible follow-up change, not part of this one.
- Adding an opencode-specific branch to `docket-build` or changing the shared task-worker
  contract.
- Changing Claude, Cursor, or Codex mappings.
- A vendor model registry, runtime model allowlist, or automatic fallback for unavailable IDs.
- Live certification of the max build profile, the review rungs, automatic classification, or the
  single-escalation path (waivers recorded explicitly in the results artifact; see Verification).

## Shipped opencode defaults

Add a complete `opencode:` block to `agents/harness-defaults.yml` and add `opencode` to
`HD_SHIPPED_HARNESSES`, so the existing validator requires the block's key set to equal the
`agents/docket-*.md` source set in both directions.

Model selection was driven by a cost/intelligence frontier analysis (2026-08-02) with the explicit
goal of very cheap, high-intelligence defaults anchored on heavy DeepSeek V4 Flash use. The
frontier collapsed the table to three models: DeepSeek V4 Flash 0731 (the workhorse — ~82% of
frontier intelligence at ~1% of frontier cost), Kimi K3 (the judgment tier — beats Claude Sonnet 5
on both axes), and GPT-5.6 Luna (one deliberate diversity row). Estimated cost per full change
lifecycle under this table is on the order of $1–2, with a worst-case (max-rung) change around $2.

| Agent | Model | Effort |
|---|---|---|
| `status` | `openrouter/deepseek/deepseek-v4-flash-0731` | `low` |
| `auto-groom` | `openrouter/deepseek/deepseek-v4-flash-0731` | `medium` |
| `auto-groom-critic` | `openrouter/openai/gpt-5.6-luna` | `high` |
| `brainstorm-consultant` | `openrouter/moonshotai/kimi-k3` | `medium` |
| `adr` | `openrouter/moonshotai/kimi-k3` | `medium` |
| `implement-next` | `openrouter/deepseek/deepseek-v4-flash-0731` | `high` |
| `finalize-change` | `openrouter/deepseek/deepseek-v4-flash-0731` | `high` |
| `rebase-resolver` | `openrouter/moonshotai/kimi-k3` | `high` |
| `integration-repair` | `openrouter/moonshotai/kimi-k3` | `high` |
| `build-economy` | `openrouter/deepseek/deepseek-v4-flash-0731` | `medium` |
| `build-standard` | `openrouter/deepseek/deepseek-v4-flash-0731` | `high` |
| `build-premium` | `openrouter/moonshotai/kimi-k3` | `medium` |
| `build-max` | `openrouter/moonshotai/kimi-k3` | `high` |
| `review-lean` | `openrouter/deepseek/deepseek-v4-flash-0731` | `high` |
| `review-standard` | `openrouter/moonshotai/kimi-k3` | `medium` |
| `review-deep` | `openrouter/moonshotai/kimi-k3` | `high` |

Selection rationale, in the sidecar's house terms (the pair is the role, not the model):

- Flash carries every volume row: the mechanical sweep, grooming, orchestration, the economy and
  standard build rungs, and the lean review rung (a fumbling reviewer's findings are discountable,
  the same reasoning as the claude block's review-lean note).
- Kimi K3 carries the judgment rows — design prose, ADRs, merge-intent reconstruction, red-suite
  repair — and the ladder top, appearing at two efforts exactly as Sol does in the codex block:
  `medium` for premium/review-standard, `high` for max/review-deep.
- `review-deep` equals the `build-max` pin, preserving the house invariant that the cap rung never
  reviews below the strength the riskiest build work was built with.
- `auto-groom-critic` deliberately uses a different model family (Luna) from the Flash-drafted
  specs it attacks: an adversarial gate from the drafter's own family shares its blind spots, and
  Luna is the cheapest frontier-adjacent way to buy that diversity.
- Efforts are capped at `high`: reasoning-effort passthrough vocabularies are model-specific and
  `xhigh`-class tokens are not assumed portable. Raising a row later is a one-line sidecar edit.

The model IDs above are design-time selections whose exact OpenRouter spellings were NOT yet
verified against a live catalog. Both the IDs and the effort tokens must be verified against the
installed `opencode models openrouter` output at build-time reconcile and again immediately before
live certification (see Failure behavior).

## Harness registration and native generation

`opencode` joins the known-harness token set (`is_valid_harness` and `HD_KNOWN_HARNESSES` — the
two readers deliberately match) and `HD_SHIPPED_HARNESSES`. Repos opt in the ordinary way:
`agent_harnesses: [claude, opencode]` (or any list including `opencode`).

A new emitter joins the `emit_for_harness` registry — opencode gets its own named emitter rather
than inheriting the Claude-shaped default, which is exactly how the Cursor defect (change 0135)
shipped. It writes one `.opencode/agents/docket-<name>.md` per wrapper source:

- `description:` carried over from the source wrapper.
- `mode: subagent` — every Docket agent is dispatched, never a primary.
- `model:` the FINAL resolved model verbatim (shipped sidecar ⊕ user layers). An unresolved model
  and the `inherit`/`auto` sentinels normalize to "no pin" (the field is omitted), matching the
  Cursor/Codex emitters' treatment rather than Claude's verbatim passthrough.
- The resolved effort is emitted as a reasoning-effort passthrough option in the frontmatter. The
  exact option key and value shape accepted by opencode's OpenRouter provider is a build-time
  verification item: the plan must prove, against a real opencode installation, which spelling
  reaches the provider (the docs document `reasoningEffort`), and the emitter uses that verified
  spelling. If per-agent effort passthrough turns out not to reach the provider, that is design
  drift surfaced to a human — never a silent drop to unpinned effort.
- The wrapper body (the skill preload / worker contract text) is carried into the markdown body as
  the agent's system prompt, following the source wrapper's existing structure.

Generation is opt-in per repo exactly like the other harnesses, and the sidecar validator runs
before any wrapper write, so a malformed block fails generation without a half-regenerated
directory.

## Dispatch

opencode joins `AGENTS_MD_DISPATCH_HARNESSES`, today `codex` only. The managed committed
`AGENTS.md` dispatch block (ADR-0036: committed, machine-neutral) becomes harness-neutral in its
prose — it addresses "the hosting harness's native named-agent dispatch" rather than Codex
specifically — and serves both harnesses from the single committed block. The write-vs-strip
logic, `--check` staleness leg, and the machine-neutrality invariant carry over unchanged; a repo
targeting either harness gets the block, a repo targeting both gets it once.

`docket-build` continues to dispatch `docket-build-economy` / `-standard` / `-premium` /
`-max` by name, foreground and sequentially; opencode resolves the name to the generated agent
definition. The controller does not detect the hosting harness or select model IDs. Because
opencode registers agent definitions at session start, setup and certification instructions retain
the required restart after regenerating wrappers.

## Example and documentation surfaces

`.docket.example.yml` gains a singly commented `opencode:` block that exactly mirrors the shipped
sidecar block, described uniformly with the other three shipped harnesses. The mirror guard
derives its harness population from `HD_SHIPPED_HARNESSES` (per the 0169 design), so it extends
without a new literal list; the resolver round-trip gains opencode evidence proving the example
resolves through the real generator into an opencode agent definition.

A whole-repository grep derives the maintained documentation update set. At minimum: the README's
harness sections, the agent-layer reference, the convention's harness mentions, and a new
`docs/opencode/setup.md` modeled on the Codex setup doc (install, `agent_harnesses:` opt-in,
OpenRouter auth, regeneration + restart, override examples). Point-in-time records (archived
changes, Accepted ADRs, prior specs/plans/results) retain their original wording.

Whether the AGENTS.md-block generalization needs a new ADR or an `## Update` note on ADR-0036 is
decided at build time by docket-adr's own rules; the sidecar extension itself is the planned
consumer path of ADR-0064 and needs no new ADR.

## Failure behavior

The sidecar validator's existing failure modes apply unchanged (missing row, phantom agent,
duplicate, partial entry, forbidden `runner:`, malformed scalar ⇒ generation fails atomically).

Docket does not silently replace a model whose slug is unavailable. At build-time reconcile and
again immediately before live certification, query the installed opencode catalog
(`opencode models openrouter`), record the opencode version, and verify every selected model ID
spelling and the effort-passthrough behavior. Any unavailable slug, unresolvable spelling, or
non-functional effort passthrough stops the build and surfaces design drift for a human; no
substitute is chosen inside implementation.

User configuration remains the operational escape hatch: any repo may override either field of any
row (e.g. pinning `build-max` back to a frontier model) without touching shipped defaults, and the
target harness owns the diagnostic for an invalid opaque value.

## Verification

### Tier 1: hermetic and gating

Generator and contract tests prove, at minimum:

1. `agents/harness-defaults.yml` validates with opencode listed as shipped; the opencode key set
   and the source-wrapper set are equal in both directions.
2. Every generated opencode definition carries the sidecar's exact resolved model and the verified
   effort-passthrough option; the four build workers and three review rungs resolve exactly to the
   table above, and `review-deep` equals `build-max`.
3. The `inherit`/`auto` sentinels and a genuinely unresolved field emit no pin.
4. Machine-local, committed, and global user values override shipped opencode values
   field-by-field.
5. The managed AGENTS.md block is written for an opencode-targeting repo, stripped when no
   AGENTS.md-dispatch harness is targeted, flagged stale by `--check` when its premise changes,
   and stays machine-neutral.
6. The singly commented opencode example mirrors every sidecar row and resolves through the real
   generator.
7. Existing controller tests remain green; no cross-harness leakage into Claude/Cursor/Codex
   outputs (extending the existing absence guards to the new harness in both directions).

Mutation evidence accompanies the guards (remove a row, add a phantom, change an example value,
defeat the review-deep=build-max assertion, break the AGENTS.md marker balance; each mutation must
redden its guard). The whole repository suite runs at the build gate.

### Tier 2: live opencode certification

Use the real `docket-build` controller in a real opencode session after regeneration and restart,
with a small fixture plan routing one safe task to each certified profile. Record in the results
artifact:

- opencode version and the catalog/effort verification evidence;
- the generated definitions for all sixteen wrappers;
- the controller's routing line, observed named agent/model indicator, structured worker outcome,
  focused verification, and task commit for each dispatched profile;
- economy observed on Flash/`medium`, standard on Flash/`high`, premium on Kimi/`medium`.

The support claim is not certified until those three named dispatches are observed. The max rung,
the review rungs, automatic classification, and the single-escalation path are deliberately not
repeated live: their harness-neutral tests and prior Claude/Codex evidence are accepted by human
decision, and the results artifact records each waiver explicitly so it can be reopened if
opencode behavior later diverges.
