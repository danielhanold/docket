# runners/opencode.sh — the opencode runner adapter

## Purpose

The third per-runner adapter of the cross-harness runner delegation framework (change 0079):
delegates one docket agent's **whole run** to opencode via its non-interactive `opencode run`.
Owns everything child-specific — permission gating, preflight, prompt assembly, flag mapping,
foreground execution, relay. Invoked only by `runner-dispatch.sh` (behind
`docket.sh runner-dispatch`), never directly by skills or shims.

The shim's own frontmatter `model:`/`effort:` pin the PARENT-side relay agent, not this child — they
come from `runners.<name>.shim_model` / `shim_effort` (defaults `inherit` / `low`) and must name
something the parent harness can resolve.

The motivating use is cost asymmetry: opencode reaches OpenRouter models, so docket's four
build profile workers can be delegated to cheap models while the review rungs stay native on the
parent's own subscription. Because build and review are already separate wrappered agents
(ADR-0063), that split needs no new mechanism — just `runner:` on the rows you want to leave.

Delegating a build worker therefore turns on **where the child starts**. `runner-dispatch.sh` sets
`DOCKET_REPO_ROOT` to the run anchor — the main worktree by default, or the tree named by
`--worktree` (both cwd-independent by design, ADR-0034) — and this adapter passes it to `--dir`.
The default suits the metadata-scoped agents delegation first shipped for; a build worker's
contract requires the feature worktree on its branch, so the facade **requires** `--worktree` for a
`build-*` agent and aborts loudly when it names none (change 0206).

## Usage

```
bash scripts/runners/opencode.sh --agent <name> [--model <m>] [--effort <e>] [--brief-file <path>] [--] [<args…>]
```

- `--agent <name>` (required) — the built-in agent to delegate; its wrapper source
  `agents/docket-<name>.md` supplies the skills list and body for the prompt. That source is
  behavior-only — model and effort arrive as the flags below, resolved by the caller from the
  user's config layers. A **shipped** default is never forwarded: only a user-configured value
  becomes a flag, and since change 0205 a `runner:`-bearing agent with no user-configured model
  is a generation-time error rather than a model-less dispatch.
- `--model <m>` (optional here, required in practice by the generation-time rule) — passed to
  `opencode run --model` **verbatim** (ADR-0015 opaque passthrough). OpenRouter IDs are
  double-prefixed (`openrouter/<vendor>/<model>`); opencode splits that itself. Docket's own
  `inherit` no-pin sentinel is normalized to "no flag" and never reaches opencode;
  `runner-dispatch.sh` owns that normalization for every adapter and this adapter keeps a
  defensive twin for the hand path above.
- `--effort <e>` (optional) — mapped to `opencode run --variant`, opencode's provider-specific
  reasoning-effort knob. Values pass through **verbatim, with no mapping table**: `--variant`
  accepts docket's `max` natively, unlike codex where `max` becomes `xhigh`. `auto` and an unset
  value both emit no flag (the provider's own default applies). With no model resolved the effort
  has nothing to attach to and is dropped with a WARN.
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
| `DOCKET_REPO_ROOT` | absolute run anchor — the main worktree unless the caller named a feature worktree; becomes `opencode run --dir` | required |
| `DOCKET_RUNNER_CFG_PERMISSIONS` | `runners.opencode.permissions` — `ask` \| `auto-approve`. Resolved by the facade from the **main worktree's** config layers, independently of the anchor above, so a `--worktree` delegation still sees a machine-local grant | `ask` |

Mock seam: `OPENCODE_BIN` (default `opencode`).

## The `permissions` knob

opencode has no sandbox *levels*. Where codex takes `--sandbox workspace-write |
danger-full-access`, opencode has a permission system that prompts for approval before editing a
file or running a shell command; `--auto` auto-approves everything **not explicitly denied** in
opencode's own config, and its own help text marks it `(dangerous!)`.

- **`ask`** (the default) — names what actually happens with no flag. A delegated run under `ask`
  **fails at adapter preflight, before any child process is invoked**, because a delegated run has
  no human channel and would otherwise block on the first approval until something times out. The
  default value describes reality rather than serving as a placeholder.
- **`auto-approve`** — bakes `--auto`. Self-describing at the config site; a reader needs no
  knowledge of opencode's CLI. Pair it with opencode's own deny rules — `--auto` approves what is
  not explicitly denied, so the deny list is *intended* to be the real boundary. **Unverified:**
  read from one line of `opencode run --help`; the interaction has not been tested. Confirm it
  against your own opencode version before relying on it.
- Any other value is a loud refusal, not a silent fall-back to `ask`: explicit config is never
  silently ignored, and a typo must not be indistinguishable from a deliberate refusal.

An enum rather than a boolean, structurally parallel to `runners.codex.sandbox`, leaving room for
a future `deny-list` value without a boolean→enum migration.

## Behavior

1. **Permission gate** — resolve `permissions`; refuse on `ask` (default) or an unknown value.
   Evaluated **before** preflight so the refusal is identical with or without the binary present.
2. **Preflight** — `opencode` (or `$OPENCODE_BIN`) resolvable on PATH. Failure is a loud
   abort-and-report — **never** a silent degrade to a native run, because `runner:` was explicit
   human config. Authentication is **not** probed; see *Prerequisites*.
3. **Prompt assembly** — from `agents/docket-<agent>.md`: "invoke skill `<s>`" for each entry of
   the wrapper's `skills:` frontmatter list (docket skills are linked into `~/.agents/skills` by
   `link-skills.sh`, which opencode reads), then the wrapper body verbatim (which carries the
   abort-and-report rule), then any passthrough args. The assembled prompt is opencode's
   **positional** `message` argument.
4. **Flag mapping** — `run --dir $DOCKET_REPO_ROOT`, `--model <model>` and `--variant <effort>`
   when supplied, `--auto` when `permissions` is `auto-approve`, then `--` and the prompt. The
   `--` ends option parsing so the prompt is always the positional `message`: a wrapper body that
   opened with a markdown bullet or a `--flag` example would otherwise be partly consumed by
   opencode's parser. Verified against 1.18.11 — `opencode run --version` prints the version, while
   `opencode run -- --version` sends `--version` as the message.
5. **Execution + relay** — runs `opencode run` **foreground**, blocking until exit; the child's
   stdout is the adapter's stdout, verbatim.

## Exit codes

- `0` — child ran and exited 0; stdout carries its output.
- `1` — precondition abort (bad args, missing agent source, missing binary, missing
  `DOCKET_REPO_ROOT`, or a `permissions` value that refuses).
- any other — the child's own nonzero exit, propagated.

## Invariants

- The `ask` refusal happens **before** any child process is invoked — never after.
- Model IDs and effort tokens are never validated or rewritten (ADR-0015); `max` is **not**
  remapped.
- Exactly one `opencode run` invocation per adapter run; always foreground, never backgrounded.
- Never degrades to running the agent natively.

## Relay shape — why default, not `--format json`

`opencode run` offers `--format default` (formatted) and `--format json` (raw JSON events). It has
no `--output-last-message` analogue, so there is no flag that yields the final message alone.
This adapter relays the **default** formatted stdout verbatim, matching `runners/cursor.sh`.
Parsing `--format json` would bind docket to an unversioned event schema where a wrong or drifted
parse silently **truncates** the relay; decoration inside a faithful relay is the smaller failure,
and it is visible rather than silent. If real-world output proves unusable, `--format json` plus a
documented extractor is the recorded escape hatch — a deliberate, reversible follow-up.

## Prerequisites (documented, not automated)

- opencode installed (`opencode` on PATH); verified against **1.18.11** (`run --model`,
  `--variant`, `--dir`, `--auto`).
- A provider authenticated — `opencode auth login` (alias `opencode providers`). OpenRouter for
  the double-prefixed IDs in the recipe. **Not probed by the adapter:** `opencode auth list`'s
  exit code on a machine with zero credentials is unverified, and a probe with unknown failure
  semantics would convert an unusual-but-working setup into a hard abort.
- Docket skills linked into `~/.agents/skills` (`link-skills.sh`, automatic on install).
- `runners.opencode.permissions: auto-approve` in a config layer read at the **main worktree** —
  its `.docket.local.yml` or `.docket.yml`, or the global
  `${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml`; never inside a feature worktree, because
  the facade resolves its config layers at the main worktree regardless of `--worktree`, so a grant
  written inside a feature worktree is not read wherever it is written — and the machine-local
  layer, being gitignored, could not travel to a feature worktree even in principle. Without the
  grant every delegated run refuses by design.
