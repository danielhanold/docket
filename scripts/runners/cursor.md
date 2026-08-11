# runners/cursor.sh — the cursor runner adapter

## Purpose

The per-runner adapter that delegates one docket agent's **whole run** to Cursor's CLI via its
non-interactive print primitive, `cursor-agent -p`. Owns everything child-specific — preflight,
prompt assembly, flag mapping, foreground execution, final-message relay. Invoked only by
`runner-dispatch.sh` (behind `docket.sh runner-dispatch`), never directly by skills or shims.

## Usage

```
bash scripts/runners/cursor.sh --agent <name> [--model <m>] [--effort <e>] [--brief-file <path>] [--] [<args…>]
```

- `--agent <name>` (required) — the built-in agent to delegate; its wrapper source
  `agents/docket-<name>.md` supplies the skills list and body for the prompt. That source is
  behavior-only — model and effort arrive as the flags below, resolved by the caller from the
  user's config layers over `agents/harness-defaults.yml`. A **shipped** default is never
  forwarded: only a user-configured value becomes a flag.
- `--model <m>` (optional **at this CLI**, required in practice) — passed to `cursor-agent --model`
  **verbatim** (ADR-0015 opaque passthrough; docket never validates or rewrites model IDs). Omitted
  here ⇒ the child's own default — but since change 0205 a `runner:`-bearing agent with no
  user-configured model is a **generation-time error** (ADR-0067), so a generated shim never omits
  it. The model-less case is reachable only by invoking this adapter by hand. Docket's own
  `inherit` no-pin sentinel is normalized to "no flag" and never reaches `cursor-agent`;
  `runner-dispatch.sh` owns that normalization for every adapter and this adapter keeps a
  defensive twin for the hand path above. Because Cursor encodes effort inside the model value,
  the sentinel behaves exactly like no model at all — the effort-dropped WARN below fires. The
  shim's own frontmatter `model:`/`effort:` pin the PARENT-side relay agent, not this child — they
  come from `runners.<name>.shim_model` / `shim_effort` (defaults `inherit` / `low`) and must name
  something the parent harness can resolve.
- `--effort <e>` (optional) — Cursor has **no effort flag**. Reasoning effort is a model parameter
  encoded inside the model value, so the adapter passes `--model <model>[effort=<effort>]` — the
  same encoding the wrapper emitter uses. With **no model resolved** the effort has nowhere to
  attach and is **dropped with a WARN** on stderr. `auto` means "no pin" and is never encoded.
  In config, note that **omitting `effort:` is not the same as opting out**: it defers to lower user
  config layers, whose value *is* forwarded. `effort: auto` is the explicit no-pin.
- `--brief-file <path>` (optional, change 0277) — the caller's task brief, read from a file and
  appended to the prompt **verbatim** under the `Additional caller arguments / task context:`
  heading. Preferred over trailing argv: the caller writes the file with a quoted-delimiter
  heredoc, so nothing about the brief is shell-quoted by a model and nothing is joined or
  reflowed. The file must exist, be readable, and carry actual content — emptiness is measured the same
  way the payload is (`$(cat …)`, trailing newlines stripped), so a file holding only
  whitespace is refused as loudly as a zero-byte one. **A brief file and trailing
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
| `DOCKET_REPO_ROOT` | absolute run anchor — the main worktree unless the caller named a feature worktree; the run's repo anchor | required |

`runners.cursor` has no configuration keys today. Mock seam: `CURSOR_BIN` (default `cursor-agent`).

## Behavior

1. **Preflight** — `cursor-agent` (or `$CURSOR_BIN`) resolvable on PATH. Failing is a loud
   abort-and-report — **never** a silent degrade to a native run, because `runner:` was explicit
   human config.
2. **Prompt assembly** — from `agents/docket-<agent>.md`: "invoke skill `<s>`" for each entry of
   the wrapper's `skills:` frontmatter list (docket skills are linked into `~/.cursor/skills` by
   `link-skills.sh`), then the wrapper body verbatim (which carries the abort-and-report rule),
   then any passthrough args.
3. **Flag mapping** — `-p --output-format text`, plus `--model <model>` (with the effort bracket
   appended when both are supplied) when a model is resolved.
4. **Execution + relay** — runs `cursor-agent -p` **foreground**, blocking until exit; the child's
   final message is the adapter's stdout, relayed verbatim.

## Exit codes

- `0` — child ran and exited 0; stdout carries its final message.
- `1` — precondition abort (bad args, missing agent source, missing binary, missing
  `DOCKET_REPO_ROOT`).
- any other — the child's own nonzero exit, propagated.

## Invariants

- Model IDs are never validated or rewritten (ADR-0015); there is no allowlist of Cursor model IDs
  or effort tokens.
- Exactly one `cursor-agent` invocation per adapter run; always foreground, never backgrounded,
  never retried.
- **Never degrades to running the agent natively.** A `cursor-agent` failure, timeout, or
  missing-feature error is a loud abort-and-report; the adapter has no fall-back path and never
  suggests running the agent inline in the parent instead.

## Recorded risk

`cursor-agent` is known from hands-on testing to be **unreliable and to lag the Cursor IDE in
features**. This adapter therefore rests on a shakier foundation than `runners/codex.md` — which is
exactly why its failure posture admits no fall-back. A silent degrade here would reproduce change
0135's own root cause (a silently-dropped configuration) in a new location.

**Stdout purity is an ASSUMPTION, not established behavior**, pending first-real-run verification:
Behavior §4 relays the child's stdout verbatim as the agent's result on the strength of
`--output-format text` emitting the final message and nothing else — unlike `runners/codex.md`,
which guarantees purity mechanically (child stream redirected to stderr, final message relayed from
`--output-last-message`). If the real CLI ever writes a banner or a progress line to stdout, that
line becomes part of the agent's reported result; confirm against a real `cursor-agent` run before
treating §4's relay as proven.

## Prerequisites (documented, not automated)

- Cursor CLI installed (`cursor-agent` on PATH) and authenticated.
- docket skills linked into `~/.cursor/skills` (`link-skills.sh`, automatic on install).
