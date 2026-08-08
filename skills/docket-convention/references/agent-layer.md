# Agent layer — configuring model/effort-pinned subagents

> On-demand detail for the convention's *Agent layer*. Read this before configuring
> `agents:` / `agent_harnesses:` in any config layer, or running/debugging `sync-agents.sh`.
> The runtime contract (which skills get wrappers, dispatch semantics, abort-and-report)
> stays in `SKILL.md`'s *Agent layer* stub; this file is the full configuration mechanics.

Contents: [Layered config](#layered-config) · [Harness-first agents: blocks](#harness-first-agents-blocks) · [Generation scope: agent_harnesses](#generation-scope-agent_harnesses) · [Harness-portable model IDs](#harness-portable-model-ids) · [Always-full-set generation + the Cursor dispatch rule](#always-full-set-generation--the-cursor-dispatch-rule) · [sync-agents.sh runs + the --check gate](#sync-agentssh-runs--the---check-gate)

## Layered config

**Layered config (precedence: repo-local > repo-committed > global > built-in).** Frontmatter is static, so configurability is a **generator** — `sync-agents.sh` — that resolves layers and writes agent files (generated copies it owns and overwrites, unlike `link-skills.sh`'s symlinks):

| Layer | Source | Generates |
|---|---|---|
| Built-in | `agents/harness-defaults.yml` shipped in docket (harness-indexed; claude, cursor, codex, and opencode each complete) | — |
| Global | the `agents:` block in `~/.config/docket/config.yml` (optional, XDG; legacy `agents.yaml` auto-migrated) | user-level `~/.claude/agents/docket-*.md` |
| Repo-committed | `.docket.yml` `agents:` block (committed, every clone) | project-level `<repo>/.claude/agents/docket-*.md` (gitignored, machine-local — see below) |
| Repo-local | `.docket.local.yml` `agents:` block (gitignored, this machine only) | same project-level files, highest precedence |

## Harness-first agents: blocks

Every one of `config.yml`'s, the repo's committed `.docket.yml`'s, and the repo's `.docket.local.yml`'s `agents:`
blocks are **harness-first**: a reserved `default:` key holds the harness-neutral fallback, and any harness name
(e.g. `cursor`) can override just the fields that differ for that harness — the harness key is just a map key.
All three resolve the same harness-first way (`~/.claude/agents`, `~/.cursor/agents`, …):

```yaml
agents:                                 # harness-first: reserved `default:` + harness-name keys
  default:                              # neutral fallback for any harness without its own entry
    implement-next: { model: claude-opus-5, effort: medium }
    status:         { model: claude-haiku-4-5-20251001 }
  cursor:                               # per-harness override — only what differs
    implement-next: { model: gpt-5.1, effort: high }
    status:         { model: gpt-5.5-medium-fast }
    # CONFIG shape, identical across harnesses; the GENERATED Cursor wrapper carries the effort
    # inside the model value instead of an `effort:` key. See the wrapper-shape table below.
  claude:                               # runner: delegates the whole run to a child harness
    status: { model: gpt-5.1-codex, runner: codex }   # (change 0079; see below)
  # Write model/effort values unquoted and space-free; `#` cannot appear inside the `{…}` flow map
  # — docket strips comments before parsing, so an in-map `#` truncates the value. Both validators
  # refuse it rather than shipping a clipped pin.
  # Resolution is field-by-field, first non-empty wins: agents.<harness>.<agent> -> agents.default.<agent> -> that harness's shipped built-in (agents/harness-defaults.yml).
  # effort: auto explicitly drops the effort line (inherit the model default); omitting the
  # effort: key instead keeps the built-in effort — auto and omitted are NOT equivalent.
  # The global ~/.config/docket/config.yml uses the SAME agents: wrapper shape (change 0050
  # unified it; the pre-0050 top-level-map agents.yaml is auto-migrated on the next sync).
  # <repo>/.docket.local.yml uses the SAME agents: wrapper shape too — gitignored,
  # this machine only, and the highest-precedence layer of the four.
  # A non-`claude` harness with no harness-specific model gets a non-fatal warning: unpinned when
  # nothing resolves, or a likely-wrong-ID note when the value came from agents.default. The
  # shipped layer is harness-indexed, so it never lends one harness's ID to another.
  # A harness block not in `agent_harnesses`, or a bare pre-0046 agent key, is warned + ignored.
```

`agent_harnesses` (which harness directories get generated files at all) is **orthogonal** to
`agents.<harness>` (which values those files carry) — a harness can appear in one list without
appearing in the other, and each falls back independently — and a pair the shipped layer does not
map ships **unpinned**, never carrying another harness's model ID.

**The shipped layer.** `agents/harness-defaults.yml` is program data, not user config, and
`scripts/lib/harness-defaults.sh` validates it before any wrapper is written: every entry nests
under a **concrete** harness (a neutral `default:` block is forbidden — that is the cross-harness
leakage it exists to prevent), each entry supplies **both** `model` and `effort`, and `runner:` is
forbidden, since delegation is user policy and never a shipped default. `HD_SHIPPED_HARNESSES` names
which harnesses carry a shipped block, and every one of them is COMPLETE: sparseness is a property
of WHICH harnesses appear, never of how much of one appears.

User-level files are built-in ⊕ global; project-level files are built-in ⊕ local ⊕ committed ⊕ global — where the
harness-first resolution above runs first, inside each layer, to pick that layer's per-field value before folding
into the next. Claude Code applies **project-over-user precedence natively**, so a project-level file resolves
**repo-local > repo-committed > global > built-in** without the generator hand-merging the two directories onto
the same file. A harness/agent pair with no entry in any layer — user or shipped — omits the field: the wrapper
carries no `model` and no `effort`, and the harness applies its own default.

**`runner:` — cross-harness delegation (change 0079).** An agent entry may carry `runner: <name>`
naming a registered runner (shipped: `codex`, `cursor`, `opencode`); the generated wrapper body then becomes a shim that
makes one foreground `docket.sh runner-dispatch` call, delegating the whole run to that child
harness. `runner` resolves per-field through the same four layers and is global-able (a machine
preference, like `model`/`effort` — it writes no shared state). It is honored under the `claude`
harness key (or `default:` when generating claude's files); under any other harness key it is
reserved and warned-and-ignored. An unregistered name is a loud generation-time error. Per-runner
knobs live in a top-level `runners.<name>:` block (any layer); each adapter's knobs and
prerequisites are in `scripts/runners/<name>.md`, and the user-facing walkthrough is README's *Runner delegation*
subsection under *Customization*.

**Model and effort on a delegated agent (0168, 0205).** A shipped `harness-defaults.yml` value is
**never forwarded to a child harness**: the sidecar is harness-indexed and the runner path resolves
under the *parent*, so that value is a parent default, not evidence the string means anything to the
child. Only user-configured values cross. Two consequences, runner-wide, not per-adapter.
**`model:` is required** — a model-less (or `inherit`, the no-pin sentinel every adapter normalizes
away) `runner:` entry is a generation-time error (ADR-0067), reversing the old "omitted ⇒ child
default" posture. **`effort:` stays optional, but omitting ≠ opting out** — it defers to lower
*user* layers, whose value IS forwarded, while `effort: auto` suppresses the flag outright. With no
flag baked the child uses its own default for the chosen model; the parent's effort stays in the
wrapper frontmatter and never reaches the child.

## Generation scope: agent_harnesses

`agent_harnesses` does **not** gate which harness keys any block may carry; it gates only which
harness *directories* get generated files. The repo's own `agent_harnesses` — read from **either**
`.docket.local.yml` or `.docket.yml`, whichever declares the key first (local wins outright, not a
merge; a direct parse in `sync-agents.sh`, not `docket-config.sh`) — solely governs the
**per-repo** pass, never the global value: each listed harness `H` gets generated
`<repo>/.<H>/agents/docket-*.md`; **default `[claude]`**; a Cursor repo sets
`agent_harnesses: [claude, cursor]`. Explicit over present-directory auto-detection, so a stray
`.cursor/` never silently mints generated files; an unknown token is warned-and-ignored. The
user-level pass instead writes every harness `agents/` directory **present on disk** — unless the
global `config.yml` sets `agent_harnesses:`, which then governs the user-level target list:
creating listed dirs, skipping unlisted ones, and pruning docket-owned files from any de-listed
known harness (never rmdir'ing the harness root itself; change 0050). The `sync-agents.sh --check`
drift gate spans every generated per-harness file.

## Harness-portable model IDs

**Harness-portable model IDs (ADR-0015).** Agent `model:` values are **direct model IDs, harness-neutral and
passed through verbatim** — no tier layer. The running harness interprets the string (a Claude alias/ID under
Claude Code; a Cursor model ID like `gpt-5.5-medium-fast` under Cursor). This unvalidated **passthrough** is
exactly what lets docket drive non-Claude harnesses.

**Per-harness wrapper shapes.** The generated wrapper is **not one uniform document** — each harness gets its
target harness's documented shape, from its own named emitter in `sync-agents.sh`. A harness with no named
emitter falls to the generic `*)` branch, which emits **Claude's** shape: a best guess, not a supported mapping
(change 0135; the Cursor defect shipped that way).

| harness | file | model | effort | skills |
|---|---|---|---|---|
| claude | `.md` | `model:` | `effort:` | `skills:` frontmatter |
| cursor | `.md` | `model: <id>[effort=<e>]` | *(inside the model value)* | body preamble |
| codex | `.toml` | `model =` | `model_reasoning_effort =` | `developer_instructions` preamble |
| opencode | `.md` | `model:` (`openrouter/<vendor>/<id>`) | `reasoningEffort:` (a provider model option, not a first-class field) | body preamble |

Cursor's frontmatter is `name`, `description`, `model`, `readonly`, `is_background` — no standalone `effort:` key
and no `skills:` preload; docket emits the first three and leaves the rest at Cursor's defaults, which suit
every docket agent. Under `model: inherit` a resolved effort has nowhere to attach and is dropped with a
generation-time WARN.

## Always-full-set generation + the Cursor dispatch rule

The **per-repo pass writes the full built-in agent set** for every harness in `agent_harnesses` — the `agents:`
block is **override-only** (it tunes a model/effort; it never decides *which* agents exist, since the agents
compose and a harness needs all of them; an entry naming no built-in is a typo warning). Per-repo generation is
**opt-in**: by declaring an `agents:` block or a top-level `agent_harnesses:` key in **either** its committed
`.docket.yml` or its `.docket.local.yml`; with neither, no per-repo wrappers are generated and `--check` stays a
no-op. The generated files are **gitignored, never committed** — regenerated from each machine's resolved config,
not shared through git. `sync-agents.sh` maintains the marker-bounded `# docket:start` / `# docket:end` block in
the repo's `.gitignore` covering every docket-owned path (plus `.docket.local.yml` itself), writing or repairing
it the moment a repo opts in (or merely carries a `.docket.local.yml`) and printing a one-time notice to commit
the block; a repo with 0048-era committed copies gets a one-time migration on the next run (tracked copies
deleted, local set regenerated fresh, the single remedy commit printed). The `cursor` harness additionally gets a
generated **`docket-dispatch.mdc`** rule (`~/.cursor/rules/` user-level; `<repo>/.cursor/rules/` per-repo, also
gitignored) that forces a dispatch to the matching docket subagent — Cursor otherwise runs a directly-invoked
skill inline at the current model, defeating the pin. Claude Code fixes the same inline quirk natively: the four
headless-safe autonomous skills (`docket-status`, `docket-adr`, `docket-implement-next`, `docket-auto-groom`)
carry `context: fork` + `agent: docket-<name>` frontmatter directly in their `SKILL.md`, forking a
directly-invoked skill into the same pinned wrapper — no generated file to sync, inert in every other harness.
**Fork-exclusion principle:** only skills that never need the human mid-run are forked, since a forked subagent
has no channel back to the human (Claude Code withholds `AskUserQuestion` and similar); the two interactive
skills stay inline, and `docket-finalize-change` stays unforked — its headless merge is gated by a permission
classifier, a separate decision (see ADR-0043). That full set is generated into the harness too, so the Cursor
rule's dispatch targets resolve by construction. `sync-agents.sh` prunes orphaned `docket-*` files (a removed
built-in drops its wrapper; a de-listed harness drops its wrappers and dispatch rule), and `--check` spans the
`.gitignore` block, the tracked-file check, and (advisory) content staleness for both.

**Both invocation paths land on the same pinned wrapper.** A forked skill-invoke (`/docket-status`)
and an explicit agent dispatch (`@docket-status`, or a subagent dispatch naming the wrapper)
resolve to the *same* generated wrapper and run at the *same* resolved model/effort; they differ
only in **observability** (the dispatch is drillable in the TUI, the fork is not) and in **cost**
(the dispatch spends a turn). The trade-off table, the fork's on-disk transcript path, and the
restart-your-session caveat live in docket's README (*Tuning agent models & effort*) — not restated here.
Two mechanics do belong here, because they govern how the wrappers compose: **a wrapper whose `skills:` preloads
the very skill that forks into it does not recurse** (preload is content injection at startup; the fork fires on
invocation — verified on Claude Code 2.1.207, closing the question ADR-0024 left open), and **skills and agents
register at process start**, so after `sync-agents.sh` or a skill-frontmatter edit an already-open session still
runs the old definitions.

Generated files are machine-local: identical-on-every-clone pinning is retired — a deliberate trade-off; team defaults still live in the committed `.docket.yml` `agents:` block, without CI-enforced pinning of generated copies.

## sync-agents.sh runs + the --check gate

`sync-agents.sh` runs **on demand** (install time, and after editing any config layer) — the same mental model as
`link-skills.sh`; it does NOT hook session start (silently regenerating out of band mid-session would be
surprising, and per-repo files are gitignored, so there is no commit to race). The drift backstop is
**`sync-agents.sh --check`**, a CI gate with three legs: (1) the managed docket `.gitignore` block
is present and current, and (2) no generated agent or dispatch-rule file is tracked by git — both are
**CI-meaningful** (`rc != 0`); (3) whether the local files on disk match what the resolved config would generate
is reported as `advisory:` output only — it never fails the build, since every machine regenerates its own copy.
