# Agent layer — configuring model/effort-pinned subagents

> On-demand detail for the convention's *Agent layer* — read before configuring `agents:` / `agent_harnesses:` in any
> config layer, or running/debugging the agent-wrapper install. The runtime contract (which skills get wrappers, dispatch
> semantics, abort-and-report) stays in `SKILL.md`'s *Agent layer* stub; this file is the full configuration mechanics.
>
> **Install-time reconciliation is Go-owned (change 0351).** `docket development install` — reached through
> `install.sh`, a thin bootstrapper — builds a fresh binary and, in one journaled transaction, links the machine-global
> skills, reconciles the machine-global agent wrappers, reconciles each repository's parent-facing dispatch surfaces
> from that repo's **explicit** `agent_harnesses`, and **retires** the old global dispatch blocks earlier docket
> versions wrote into personal instruction files (`~/.claude/CLAUDE.md` and the other harnesses' globals). Retirement
> is **proof-gated**: a block is removed only while it still matches docket's exact ownership marker; **no `--force`**;
> a modified or foreign block is left untouched and reported to remedy-and-rerun. As a repo opt-in, `agent_harnesses`
> has three states — *absent* keeps the shipped default (Claude only), writing no other harness's repo surfaces; a
> *non-empty* list reconciles those harnesses; an *explicit empty* list (`agent_harnesses: []`) retires every
> docket-owned repo surface the repo had. `--repo-dir <path>` targets another repository, a repeatable `--harness
> <name>` scopes the run. The Go install is the wrapper generator and owns the transaction.
> Restart the harness process after any run that changed a wrapper or parent surface — read at process start.

Contents: [Layered config](#layered-config) · [Harness-first agents: blocks](#harness-first-agents-blocks) · [Generation scope: agent_harnesses](#generation-scope-agent_harnesses) · [Harness-portable model IDs](#harness-portable-model-ids) · [Launch posture](#launch-posture) · [Always-full-set generation + the Cursor dispatch rule](#always-full-set-generation--the-cursor-dispatch-rule) · [Wrapper generation and the drift-check gate](#wrapper-generation-and-the-drift-check-gate)

## Layered config

**Layered config (precedence: repo-local > repo-committed > global > built-in).** Frontmatter is static, so configurability is a **generator** — the Go install — resolving layers and writing agent files (generated copies it owns and overwrites, unlike `link-skills.sh`'s symlinks):

| Layer | Source | Generates |
|---|---|---|
| Built-in | `agents/harness-defaults.yml` shipped in docket (harness-indexed; claude/cursor/codex/opencode each complete) | — |
| Global | the `agents:` block in `~/.config/docket/config.yml` (optional, XDG; legacy `agents.yaml` auto-migrated) | user-level `~/.claude/agents/docket-*.md` |
| Repo-committed | `.docket.yml` `agents:` block (committed, every clone) | project-level `<repo>/.claude/agents/docket-*.md` (gitignored, machine-local — see below) |
| Repo-local | `.docket.local.yml` `agents:` block (gitignored, this machine only) | same project-level files, highest precedence |

## Harness-first agents: blocks

All three `agents:` blocks (`config.yml`'s, the repo's committed `.docket.yml`'s, and its `.docket.local.yml`'s) are
**harness-first**: a reserved `default:` key holds the harness-neutral fallback, and any harness name (e.g. `cursor`)
overrides just the fields that differ — the harness key is just a map key. All three resolve the same way
(`~/.claude/agents`, `~/.cursor/agents`, …):

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
  # Write model/effort values unquoted and space-free; `#` cannot appear inside the `{…}` flow map
  # — docket strips comments before parsing, so an in-map `#` truncates the value; both validators
  # refuse it, not ship a clipped pin.
  # Resolution is field-by-field, first non-empty wins: agents.<harness>.<agent> -> agents.default.<agent> -> that harness's shipped built-in.
  # effort: auto explicitly drops the effort line (inherit the model default); omitting the
  # effort: key instead keeps the built-in effort — auto and omitted are NOT equivalent.
  # A non-`claude` harness with no harness-specific model gets a non-fatal warning: unpinned when
  # nothing resolves, or a likely-wrong-ID note when the value came from agents.default.
  # A harness block not in `agent_harnesses`, or a bare pre-0046 agent key, is warned + ignored.
```

`agent_harnesses` (which harness directories get generated files) is **orthogonal** to `agents.<harness>` (which
values those files carry) — a harness can appear in one list without the other, each falling back independently — and
a pair the shipped layer does not map ships **unpinned**, never carrying another harness's model ID.

**The shipped layer.** `agents/harness-defaults.yml` is program data, not user config, and
the harness-defaults validator validates it before any wrapper is written: every entry nests under a **concrete**
harness (a neutral `default:` block is forbidden — the cross-harness leakage it exists to prevent), supplies **both**
`model` and `effort`, and forbids `runner:`, since delegation is user policy, never a shipped default.
`HD_SHIPPED_HARNESSES` names which harnesses carry a shipped block, and every one is COMPLETE: sparseness is which
harnesses appear, never how much of one appears.

User-level files are built-in ⊕ global; project-level files are built-in ⊕ local ⊕ committed ⊕ global — the
harness-first resolution running first inside each layer, picking that layer's per-field value before folding into the
next. Claude Code applies **project-over-user precedence natively**, so a project-level file resolves **repo-local >
repo-committed > global > built-in** without the generator hand-merging the two directories onto one file. A
harness/agent pair with no entry in any layer — user or shipped — omits the field: the wrapper carries no
`model`/`effort`, and the harness applies its own default.

**Cross-harness delegation is retired (change 0371).** An agent entry carries no `runner:` key on the
maintained surface: a parent invokes a registered `docket-*` agent through its own harness's native
named-agent dispatch — the generated `docket:dispatch` block is the contract — and a workflow with no
registration on the current host fails visibly rather than falling back to a shell runner, another
harness, or a generic agent.

## Generation scope: agent_harnesses

`agent_harnesses` does **not** gate which harness keys any block may carry; it gates only which harness *directories*
get generated files. The repo's own `agent_harnesses` — read from **either** `.docket.local.yml` or `.docket.yml`,
whichever declares the key first (local wins, not a merge; a direct parse in the install, not the config resolver)
— governs only the **per-repo** pass, never the global value: each listed harness `H` gets
generated `<repo>/.<H>/agents/docket-*.md`; **default `[claude]`**; a Cursor repo sets `agent_harnesses: [claude,
cursor]`. Explicit over present-directory auto-detection, so a stray `.cursor/` never silently mints generated files;
an unknown token is warned-and-ignored. The user-level pass instead writes every harness `agents/` directory
**present on disk** — unless the global `config.yml` sets `agent_harnesses:`, governing the user-level target list:
creating listed dirs, skipping unlisted, and pruning docket-owned files from any de-listed known harness (never
rmdir'ing the harness root; change 0050). The `docket install check` drift gate spans every generated per-harness
file.

## Harness-portable model IDs

**Harness-portable model IDs (ADR-0015).** Agent `model:` values are **direct model IDs, harness-neutral and passed
through verbatim** — no tier layer. The running harness interprets the string (a Claude alias/ID under Claude Code; a
Cursor model ID like `gpt-5.5-medium-fast` under Cursor). This unvalidated **passthrough** is what lets docket drive
non-Claude harnesses.

**Per-harness wrapper shapes.** The generated wrapper is **not one uniform document** — each harness gets its own
documented shape from its named emitter in the install. A harness with no named emitter falls to
the generic `*)` branch, which emits **Claude's** shape: a best guess, not a supported mapping (change 0135; the
Cursor defect shipped that way). Reaching it is not silent: generation prints a one-time WARN naming the harness as
unverified, and `docket install check` reports the same token as a non-failing advisory, not a check failure.

| harness | file | model | effort | skills |
|---|---|---|---|---|
| claude | `.md` | `model:` | `effort:` | `skills:` frontmatter |
| cursor | `.md` | `model: <id>[effort=<e>]` | *(inside the model value)* | body preamble |
| codex | `.toml` | `model =` | `model_reasoning_effort =` | `developer_instructions` preamble |
| opencode | `.md` | `model:` (`openrouter/<vendor>/<id>`) | `reasoningEffort:` (a provider model option, not a first-class field) | body preamble |

Cursor's frontmatter is `name`, `description`, `model`, `readonly`, `is_background` — no standalone `effort:` key and
no `skills:` preload; docket emits the first three and leaves the rest at Cursor's defaults, which suit every docket
agent. Under `model: inherit` a resolved effort has nowhere to attach and is dropped with a generation-time WARN.

## Launch posture

Agent-source frontmatter may declare `launch: root-coordinator`; absence means the closed default
`child`. The posture describes a role's required entry capability, not a model setting and not a
request to broaden every child's authority. Mark a role `root-coordinator` when its charter owns
multi-agent sequencing and therefore requires native collaboration controls at entry. The inventory
parser rejects unknown posture values, and correspondence tests derive the marked set from the
role's same-name skill contract rather than maintaining a filename allowlist.

Codex realizes `root-coordinator` through `docket agent enter`: the command resolves the same typed
role contract used to generate its TOML registration, launches `codex app-server --stdio` directly,
starts a root thread with the caller's absolute cwd, approval policy, and sandbox, and supplies the
unchanged request as the root turn. It never falls back to `codex exec`, another harness, a shell
relay, or an ordinary child launch. Other harnesses may ignore the posture until they implement a
corresponding native entry path; the source remains harness-neutral.

## Always-full-set generation + the Cursor dispatch rule

The **per-repo pass writes the full built-in agent set** for every harness in `agent_harnesses` — the `agents:`
block is **override-only** (it tunes a model/effort; it never decides *which* agents exist, since the agents
compose and a harness needs all of them; an entry naming no built-in is a typo warning). Per-repo generation is
**opt-in**, by declaring an `agents:` block or a top-level `agent_harnesses:` key in **either** the committed
`.docket.yml` or the `.docket.local.yml`; with neither, no per-repo wrappers generate and `--check` stays a no-op.
The generated files are **gitignored, never committed** — regenerated from each machine's resolved config.
The Go install maintains the marker-bounded `# docket:start` / `# docket:end` block in the repo's `.gitignore`
covering every docket-owned path (plus `.docket.local.yml`), writing or repairing it the moment a repo opts in
(or merely carries a `.docket.local.yml`) and printing a one-time notice to commit it; a repo with 0048-era
committed copies gets a one-time migration on the next run (tracked copies deleted, local set regenerated, the
single remedy commit printed). The `cursor` harness also gets a generated **`docket-dispatch.mdc`** rule
(`~/.cursor/rules/` user-level; `<repo>/.cursor/rules/` per-repo, also gitignored) dispatching to the matching docket
subagent — Cursor otherwise runs a directly-invoked skill inline at the current model, defeating
the pin. Claude Code fixes the same quirk natively: the four headless-safe autonomous skills (`docket-status`,
`docket-adr`, `docket-implement-next`, `docket-auto-groom`) carry `context: fork` + `agent: docket-<name>`
frontmatter in their `SKILL.md`, forking a directly-invoked skill into the same pinned wrapper — no generated
file to sync, inert in every other harness. **Fork-exclusion principle:** only skills that never need the human
mid-run are forked, since a forked subagent has no channel back to the human (Claude Code withholds
`AskUserQuestion` and similar); the two interactive skills stay inline, and `docket-finalize-change` stays
unforked — its headless merge is gated by a permission classifier, a separate decision (see ADR-0043). The full
set is generated into the harness, so the Cursor rule's dispatch targets resolve by construction. The install
prunes orphaned `docket-*` files (a removed built-in drops its wrapper; a de-listed harness drops its wrappers and
dispatch rule), and `--check` spans the `.gitignore` block, the tracked-file check, and (advisory) content staleness
for both.

**Both invocation paths land on the same pinned wrapper.** A forked skill-invoke (`/docket-status`) and an explicit
agent dispatch (`@docket-status`, or a subagent dispatch naming the wrapper) resolve to the *same* generated wrapper
at the *same* resolved model/effort; they differ only in **observability** (the dispatch is drillable in the TUI, the
fork is not) and **cost** (the dispatch spends a turn). The trade-off table and the fork's transcript path live in
docket's README (*Tuning agent models & effort*). Two mechanics belong here,
governing how the wrappers compose: **a wrapper whose `skills:` preloads the very skill that forks into it does not
recurse** (preload is content injection at startup; the fork fires on invocation — verified on Claude Code 2.1.207,
closing the question ADR-0024 left open), and **skills and agents register at process start**, so after
the install or a skill-frontmatter edit an already-open session still runs the old definitions.

Identical-on-every-clone pinning is retired (a deliberate trade-off); team defaults still live in the committed `.docket.yml` `agents:` block, without CI-enforced pinning of the machine-local generated copies.

## Wrapper generation and the drift-check gate

The Go install runs **on demand** (install time, and after editing any config layer) — the same mental model as
`link-skills.sh`; it does NOT hook session start (silently regenerating mid-session would be surprising, and per-repo
files are gitignored, so no commit to race). The drift backstop is **`docket install check`**,
a CI gate with three legs: (1) the managed docket `.gitignore` block is present and current, and (2) no generated
agent or dispatch-rule file is tracked by git — both **CI-meaningful** (`rc != 0`); (3) whether the local files match
what the resolved config would generate is `advisory:` output only — it never fails the build, since every machine
regenerates its own copy.
