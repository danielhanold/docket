# runners/codex.sh — the codex runner adapter

## Purpose

The first per-runner adapter of the cross-harness runner delegation framework (change 0079):
delegates one docket agent's **whole run** to OpenAI Codex CLI via its non-interactive exec
primitive, `codex exec`. Owns everything child-specific — preflight, prompt assembly, flag
mapping, foreground execution, final-message relay. Invoked only by `runner-dispatch.sh`
(behind `docket.sh runner-dispatch`), never directly by skills or shims.

## Usage

```
bash scripts/runners/codex.sh --agent <name> [--model <m>] [--effort <e>] [--brief-file <path>] [--] [<args…>]
```

- `--agent <name>` (required) — the built-in agent to delegate; its wrapper source
  `agents/docket-<name>.md` supplies the skills list and body for the prompt. That source is
  behavior-only — model and effort arrive as the flags below, resolved by the caller from the
  user's config layers over `agents/harness-defaults.yml`. A **shipped** default is never
  forwarded: only a user-configured value becomes a flag.
- `--model <m>` (optional **at this CLI**, required in practice) — passed to `codex exec -m`
  **verbatim** (ADR-0015 opaque passthrough; docket never validates model IDs). Omitted here ⇒ the
  child's own default model — but since change 0205 a `runner:`-bearing agent with no
  user-configured model is a **generation-time error** (ADR-0067), so a generated shim never omits
  it. The model-less case is reachable only by invoking this adapter by hand. Docket's own
  `inherit` no-pin sentinel is normalized to "no flag" and never reaches `codex exec`;
  `runner-dispatch.sh` owns that normalization for every adapter and this adapter keeps a
  defensive twin for the hand path above. Unlike Cursor, **effort survives** the model-less case
  here, because `model_reasoning_effort` is a separate flag rather than an encoding inside the
  model value. The shim's own frontmatter `model:`/`effort:` pin the PARENT-side relay agent, not
  this child — they come from `runners.<name>.shim_model` / `shim_effort` (defaults `inherit` /
  `low`) and must name something the parent harness can resolve.
- `--effort <e>` (optional) — mapped to Codex's `model_reasoning_effort` config override.
  Values pass through verbatim except docket's `max`, which maps to codex's `xhigh` (the top
  of Codex's vocabulary). Omitted ⇒ no override (child default). Effort stays genuinely optional,
  but note that **omitting `effort:` in config is not the same as opting out**: it defers to lower
  user config layers, whose value *is* forwarded. `effort: auto` is the explicit no-pin that
  suppresses the flag outright.
- `--brief-file <path>` (optional, change 0277) — the caller's task brief, read from a file and
  appended to the prompt **verbatim** under the `Additional caller arguments / task context:`
  heading. Preferred over trailing argv: the caller writes the file with a quoted-delimiter
  heredoc, so nothing about the brief is shell-quoted by a model and nothing is joined or
  reflowed. The file must exist, be readable, and be non-empty. **A brief file and trailing
  arguments together are refused** — passing both would silently drop or duplicate the child's
  only input, so this adapter dies rather than picking one. `runner-dispatch.sh` refuses the same
  shape first; this is the defensive twin for the hand invocation documented here.
- `-- <args…>` — the legacy payload channel, still supported and no longer lossy: the arguments
  are appended to the prompt as caller task context, joined on a **newline** in order (they were
  previously interpolated with `$*`, which joins on the first character of `IFS` and flattened a
  multi-line brief onto one line).

Environment (set by the facade):

| Var | Meaning | Default |
|---|---|---|
| `DOCKET_REPO_ROOT` | absolute run anchor — the main worktree unless the caller named a feature worktree; becomes `codex exec -C` | required |
| `DOCKET_RUNNER_CFG_SANDBOX` | `runners.codex.sandbox` — `workspace-write` \| `danger-full-access` | `workspace-write` |
| `DOCKET_RUNNER_CFG_NETWORK` | `runners.codex.network` — network access inside workspace-write | `true` |

Approvals are always `never` — `codex exec` is non-interactive by definition; there is no
approvals knob to configure. Mock seam: `CODEX_BIN` (default `codex`).

## Behavior

1. **Preflight** — `codex` (or `$CODEX_BIN`) resolvable on PATH, then `codex login status`
   exits 0. Either failing is a loud abort-and-report — **never** a silent degrade to a native
   run, because `runner:` was explicit human config.
2. **Prompt assembly** — from `agents/docket-<agent>.md`: "invoke skill `<s>`" for each entry
   of the wrapper's `skills:` frontmatter list (docket skills are linked into
   `~/.codex/skills` by `link-skills.sh`), then the wrapper body verbatim (which carries the
   abort-and-report rule), then any passthrough args.
3. **Flag mapping** — `-C $DOCKET_REPO_ROOT`, `--sandbox <sandbox>`, `--color never`,
   `-c sandbox_workspace_write.network_access=true` (only when sandbox is `workspace-write`
   and network is `true`), `-m <model>` / `-c model_reasoning_effort=<effort>` when supplied.
4. **Execution + relay** — runs `codex exec` **foreground**, blocking until exit; codex's own
   event stream is redirected to stderr; the child's final message (captured via
   `--output-last-message`) is the adapter's **entire stdout**.

## Exit codes

- `0` — child ran and exited 0; stdout carries its final message.
- `1` — precondition abort (bad args, missing agent source, missing binary, unauthenticated,
  missing `DOCKET_REPO_ROOT`).
- any other — the child's own nonzero exit, propagated (the final message, if captured, is
  still relayed before exiting).

## Invariants

- stdout is the child's final message ONLY — event noise never leaks there.
- Model IDs are never validated or rewritten (ADR-0015).
- Exactly one `codex exec` invocation per adapter run; always foreground, never backgrounded.
- Never degrades to running the agent natively.

## Prerequisites (documented, not automated)

- Codex CLI installed (`codex` on PATH) and authenticated (`codex login`); verified ≥ 0.144.4
  (`--output-last-message`, `login status`).
- superpowers installed in Codex; docket skills linked into `~/.codex/skills`
  (`link-skills.sh`, automatic on install).
- `[features] multi_agent = true` in `~/.codex/config.toml` — required only when delegating an
  **orchestrator** whose run fans out child-side (e.g. SDD); leaf agents run without it.
