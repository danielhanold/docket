<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0205 — opencode runner adapter — delegate build workers to OpenRouter models](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0205-opencode-runner-adapter.md)**
<!-- docket:backlink:end -->

# opencode runner adapter — design

Change 0205. Spec date 2026-08-04.

## Context

Docket's cross-harness runner delegation framework (change 0079, ADR-0037, ADR-0038) lets an
autonomous agent's **whole run** be delegated from the parent harness hosting the session to a
child harness's non-interactive exec primitive. Two adapters exist: `scripts/runners/codex.sh`
(93 lines) and `scripts/runners/cursor.sh` (102 lines), behind the `runner-dispatch.sh` facade.

Change 0192 registered `opencode` as a fully shipped docket **harness** — registration, a native
`.opencode/agents/` emitter, AGENTS.md dispatch wiring, and a complete sixteen-agent default block
on OpenRouter model IDs. It explicitly scoped out the runner: *"A Claude-to-opencode whole-run
runner (`REGISTERED_RUNNERS` unchanged); possible follow-up."* This change is that follow-up.

The motivation is cost asymmetry. Through OpenRouter, opencode reaches DeepSeek-tier models at
roughly 3¢ per task against Claude Opus at $2.40. The human wants build work delegated to those
models while review stays on the existing Claude subscription.

### The rejected alternative, and why

A direct OpenRouter API wrapper script — bypassing opencode and calling the model endpoint from
shell — was proposed and rejected. The premise was that the harness stands between docket and the
cheap model. It does not: OpenRouter is opencode's *model backend*, opencode is the *agent
runtime*, and docket already reaches OpenRouter through it.

A docket runner delegates a whole autonomous agent run — planning, branching, editing files,
running the suite, committing, opening a PR. A raw API call returns one completion. Closing that
gap means hand-building a tool-use loop, file and bash tools, sandboxing, permission handling,
context and session management, retries, and cost accounting: rebuilding opencode, in shell,
unmaintained. It would also require docket to own model semantics, which ADR-0015's opaque
passthrough exists to prevent, and it fits no existing seam — ADR-0038 defines the runner seam as
one foreground call to a child harness's exec primitive, and a direct-API caller has none.

The adapter, by contrast, is a ~100-line sibling of `codex.sh`.

A direct API call remains the better tool for a single-shot, non-agentic role (a classification, a
critique verdict). No delegatable docket agent is one, so it does not apply here.

## Delegation granularity

ADR-0037's rule is that delegation is all-or-nothing **for a single agent's run**, and a delegated
agent's sub-dispatches run child-natively. It is not "the whole change lifecycle goes to one
harness." `runner:` is a per-entry key in the `agents:` block, and `emit_wrapper` applies it to
whatever agent carries it — there is no eligibility allowlist in code. The "only autonomous
wrappers are delegatable" constraint enforces itself structurally: the two interactive skills have
no generated wrapper, so there is nothing to put a `runner:` on.

Because docket's build and review roles are already separate wrappered agents (ADR-0063), the
motivating split needs no new mechanism:

```yaml
agents:
  claude:
    build-economy:  { runner: opencode, model: openrouter/deepseek/deepseek-v4-flash-0731 }
    build-standard: { runner: opencode, model: openrouter/deepseek/deepseek-v4-flash-0731 }
    build-premium:  { runner: opencode, model: openrouter/x-ai/grok-4.5 }
    build-max:      { runner: opencode, model: openrouter/moonshotai/kimi-k3 }
    # review-lean / review-standard / review-deep: no runner: → native Claude Code
```

`docket-implement-next` stays native and orchestrates as usual; at step 5 it dispatches
`docket-build`, which routes each plan task to a profile worker, and those shim out to opencode.
Step 6's review dispatch carries no runner, so it runs on the Claude subscription.

**The load-bearing rule: delegate leaves, not orchestrators.** Delegating an orchestrator drags
everything beneath it into the child, because a delegated run's sub-dispatches are child-native.
Delegate `implement-next` and review goes to opencode too. Delegating the four profile workers
rather than the `docket-build` controller is deliberate for the same reason: delegating the
controller would move the routing decision into the child as well.

## Model selection — explicit user config (option A)

Change 0168 established that **only a user-configured value is ever baked into a child-runner
flag**; a shipped `agents/harness-defaults.yml` entry is never forwarded, because a shipped default
is a default *for this harness* and forwarding it could send a Claude model ID to a Codex process.

A consequence deserves stating plainly: **0192's sixteen-row opencode table is not reachable from
the runner path.** That block is indexed under harness `opencode`, meaning "when generating
`.opencode/agents/` files for a natively-hosted opencode session." The runner path resolves with
harness `claude`, reads the `claude:` block, and would not forward a sidecar value regardless.

Three options were weighed. **Cross-indexing the sidecar** — when an agent resolves
`runner: <name>` and `<name>` is itself a shipped harness, resolve model and effort from that
harness's block — is architecturally attractive: single source of truth, full profile ladder, free
extension to future runners. It was **rejected** on two grounds:

1. **The native table answers a different question.** The `opencode:` block answers "if opencode
   ran this whole project, what should each role cost?" Delegation asks "which rows do I want to
   leave my Claude subscription, and which do I deliberately keep?" The build-delegated /
   review-native split is exactly that asymmetry. A table tuned for the first question is not
   automatically correct for the second.
2. **It couples two unrelated intents to one dial.** Retuning the native block (change 0195) would
   silently change what delegated Claude Code builds run on, with the coupling invisible at the
   config site.

A per-runner single default model (`runners.opencode.model`) was also rejected: one model for all
delegated agents discards the economy/standard/premium/max ladder, which is most of the value.

**Decision: explicit user config.** The model is written per agent alongside `runner:`, in the
user's config layers. Explicit, greppable, reviewable, revertible — the same principle ADR-0037
already chose over model-ID sniffing. The drift risk (hand-maintaining a table that resembles one
already in-repo) is addressed at the **documentation** layer: the config recipe ships a
copy-pasteable block a human chooses to adopt, rather than machinery that adopts it silently.

## Required model — a runner-wide rule change

Today a model-less delegation is legal on every adapter: codex's contract documents *"Omitted ⇒
the child's own default model."* Under OpenRouter that is a pay-per-token default of unknown
identity and cost, and the failure is silent — the run succeeds on the wrong model and surfaces on
the bill.

**A delegated agent with no user-configured model becomes a loud generation-time error**, applied
**runner-wide** rather than opencode-only. Uniformity is the point: "is a model required?" should
not be an adapter-by-adapter fact a user learns twice, and the framework already prefers loud
failure over silent degradation (ADR-0037: "explicit config is never silently ignored and never
silently degraded"). The guard is milder in value for a subscription-billed child but costs nothing
there.

This is a **behavior change** for any existing model-less codex or cursor configuration, which is
why it carries an ADR. The error is raised in `sync-agents.sh` at generation time — the same place
an unregistered runner name already fails loudly — so the failure lands when config is written, not
mid-dispatch.

**Effort remains optional.** Absence *is* the auto setting: omitting `--variant` lets the provider's
own default apply, matching codex's documented "omitted ⇒ no override (child default)." No sentinel
value is invented.

## The adapter

`scripts/runners/opencode.sh` plus its co-located `scripts/runners/opencode.md` contract, following
`codex.sh` exactly: preflight, prompt assembly from `agents/docket-<name>.md`, flag mapping,
foreground execution, final-message relay, abort-and-report on nonzero. Invoked only by
`runner-dispatch.sh`, never directly by skills or shims.

Flag mapping, from `opencode run --help` (opencode 1.18.11):

| docket | opencode | Notes |
|---|---|---|
| `--model <m>` | `-m, --model` | verbatim; ADR-0015 opaque passthrough |
| `--effort <e>` | `--variant` | `--variant` is "provider-specific reasoning effort, e.g. high, max, minimal" |
| repo root | `--dir` | analogue of codex's `-C` |
| permission posture | `--auto` | gated on the knob below |

**`--variant` takes docket's `max` natively**, so unlike codex — which maps `max` → `xhigh` — the
effort vocabulary passes straight through with no mapping table.

A note on why this needed checking: 0192 verified that `reasoningEffort:` in **agent frontmatter**
resolves to `options.reasoningEffort`. That is the native emitter surface. The runner path is the
`opencode run` CLI, a different surface with a different spelling. The 0192 finding does not
transfer.

Mock seam: `OPENCODE_BIN` (default `opencode`), matching `CODEX_BIN`.

## Permission posture

opencode has no sandbox *levels*. Where codex takes `--sandbox workspace-write |
danger-full-access`, opencode has a permission system: by default it prompts for approval before
editing a file or running a shell command. `--auto` auto-approves everything **not explicitly
denied** in opencode's own config; its help text marks it `(dangerous!)`.

A delegated run cannot answer a prompt, so without `--auto` it hangs on the first approval until it
times out. But a build worker with `--auto` and no deny rules can run any shell command in the
repository unwatched.

The posture is therefore **explicit config, never an adapter default**:

```yaml
runners:
  opencode:
    permissions: ask          # default — the child prompts for approval
                              # auto-approve — approve everything not explicitly denied (--auto)
```

- **`ask`** (default) — names what actually happens with no flag. A delegated run under `ask` fails
  loudly at adapter preflight, stating that delegation requires a posture. The default value
  describes reality rather than serving as a placeholder.
- **`auto-approve`** — bakes `--auto`. Self-describing at the config site; a reader needs no
  knowledge of opencode's CLI.

An enum rather than a boolean, structurally parallel to `runners.codex.sandbox`, leaving room for a
future `deny-list` value without a boolean→enum migration. The knob is machine-preference class
(coordination-fence exempt), like the other `runners.<name>:` blocks.

Nobody receives blanket auto-approval as a side effect of typing `runner: opencode`; the risk is
accepted at a visible line in config.

## Git identity — explicitly a non-issue

Recorded because it was raised and dismissed, so it is not re-litigated at build time. `codex.sh`
runs `codex exec -C "$DOCKET_REPO_ROOT"` and touches git identity nowhere. The child is the same OS
user on the same machine in the same working tree, so `git config user.name` / `user.email` resolve
through the same global and repo config a native run uses. opencode's `--dir` is the direct
analogue. This becomes real only for a containerized or remote runner, which this is not.

## Documentation

A config recipe in `docs/opencode/setup.md` (the file 0192 created), carrying:

- The build-delegated / review-native block above, with **verified** model IDs so it is
  copy-paste-usable.
- The "delegate leaves, not orchestrators" rule and why.
- The `permissions` knob, what `auto-approve` actually grants, and the recommendation to pair it
  with opencode deny rules.
- A pointer that model selection is explicit by design, with the reasoning compressed to a sentence.

## Testing

Following the existing adapter tests:

- Flag mapping: model verbatim, `--variant` from effort, `--dir` from repo root, `max` passing
  through unmapped.
- Effort omitted ⇒ no `--variant` flag emitted.
- `permissions: ask` (and unset) ⇒ preflight failure with a diagnostic naming the knob; nonzero
  exit; **no child process invoked**.
- `permissions: auto-approve` ⇒ `--auto` present.
- Registry parity: `REGISTERED_RUNNERS` ↔ `scripts/runners/*.sh` in both directions (the existing
  assertion, extended).
- Runner-wide required-model rule: generation-time error for a `runner:`-bearing agent with no
  user-configured model, asserted on **codex and cursor as well as opencode**, since it is a
  framework rule and not an adapter behavior.
- Mutation evidence for the required-model guard and the `ask` refusal — both are the kind of
  negative assertion that passes vacuously when miswired.

## Out of scope

- **Model retuning.** Change 0195 owns the opencode default table. Its open questions (Luna vs Grok
  at `review-standard`; whether `openrouter/x-ai/grok-4.5` is a real ID) stay there. Under option A
  they do not gate this change, which is a further argument for option A.
- **Sidecar cross-indexing** (option C), and any change to 0168's provenance rule beyond the
  required-model error.
- **Codex work.** Change 0078's validation runbook is being deferred as built on outdated logic.
- **New delegatable-agent restrictions.** The framework's existing rule is unchanged; this ships
  capability, and which agents are pinned is user config.
- **Parent harnesses other than `claude`.** `runner:` under another harness key stays reserved and
  warned-and-ignored.

## Open questions

All require a live opencode session and are verify items, not design gaps.

1. **Confirm the model IDs against `opencode models`** before the recipe ships. Docket keeps no
   vendor allowlist and every mirror assertion compares generated output against the sidecar that
   generated it, so **no in-repo test can detect a wrong ID**. 0192 imposed the same requirement.
2. **Confirm `--variant` omission** yields the provider default rather than an error or a silent
   substitution. 0192's results explicitly flagged that its own model-less/effort case was never
   probed, so this is not assumed.
3. **Confirm `--auto` semantics and the deny-list interaction.** Both are read from a single line of
   `opencode run --help`; the deny-list behavior is inferred, not tested. If a workable deny-list
   spelling exists, evaluate whether the recipe should recommend a concrete starter set — but do not
   let it grow into a third `permissions` value in this change.
4. **Confirm the relay surface.** `--format json` emits raw JSON events; whether the adapter should
   use it or the default formatted output for final-message extraction is a build-time call against
   real output, matching how `codex.sh` handles relay.
5. **Live-certify one delegated dispatch end to end** — a real `build-economy` task on a real
   branch — before this is called done. 0192 certified economy, standard, and premium natively;
   this is the same bar for the runner path.
