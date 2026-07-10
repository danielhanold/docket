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
| Built-in | `agents/docket-*.md` shipped in docket (each ships its default model/effort) | — |
| Global | the `agents:` block in `~/.config/docket/config.yml` (optional, XDG; legacy `agents.yaml` auto-migrated) | user-level `~/.claude/agents/docket-*.md` |
| Repo-committed | `.docket.yml` `agents:` block (committed, every clone) | project-level `<repo>/.claude/agents/docket-*.md` (gitignored, machine-local — see below) |
| Repo-local | `.docket.local.yml` `agents:` block (gitignored, this machine only) | same project-level files, highest precedence |

## Harness-first agents: blocks

Every one of `config.yml`'s, the repo's committed `.docket.yml`'s, and the repo's `.docket.local.yml`'s
`agents:` blocks are **harness-first**: a reserved `default:` key holds the
harness-neutral fallback, and any harness name (e.g. `cursor`) can override just the fields that
differ for that harness — the harness key is just a map key in any of the three blocks.
Whatever keys any of the three blocks has, they resolve the same harness-first way (`~/.claude/agents`,
`~/.cursor/agents`, …):

```yaml
agents:                                 # harness-first: reserved `default:` + harness-name keys
  default:                              # neutral fallback for any harness without its own entry
    implement-next: { model: claude-opus-4-8, effort: xhigh }
    status:         { model: claude-haiku-4-5-20251001 }
  cursor:                               # per-harness override — only what differs
    implement-next: { model: gpt-5.1, effort: high }
    status:         { model: gpt-5.5-medium-fast }
  # Resolution is field-by-field, first non-empty wins: agents.<harness>.<agent> -> agents.default.<agent> -> shipped built-in (agents/docket-*.md).
  # effort: auto explicitly drops the effort line (inherit the model default); omitting the
  # effort: key instead keeps the built-in effort — auto and omitted are NOT equivalent.
  # The global ~/.config/docket/config.yml uses the SAME agents: wrapper shape (change 0050
  # unified it; the pre-0050 top-level-map agents.yaml is auto-migrated on the next sync).
  # <repo>/.docket.local.yml uses the SAME agents: wrapper shape too — gitignored,
  # this machine only, and the highest-precedence layer of the four.
  # A non-`claude` harness whose model falls to default/built-in gets a non-fatal warning
  # (likely-wrong ID; docket never validates model IDs).
  # A harness block not in `agent_harnesses`, or a bare pre-0046 agent key, is warned + ignored.
```

`agent_harnesses` (which harness directories get generated files at all) is **orthogonal** to
`agents.<harness>` (which values those files carry) — a harness can appear in one list without
appearing in the other, and each falls back independently: `agent_harnesses` defaults to `[claude]`;
an unlisted `agents.<harness>` falls to `agents.default`, then to the built-in.

User-level files are built-in ⊕ global; project-level files are built-in ⊕ local ⊕ committed ⊕ global — where the
harness-first resolution above runs first, inside each layer, to pick that layer's per-field value before folding
into the next. Claude Code applies **project-over-user precedence natively**, so the effective order for a
project-level file's own resolved value is **repo-local > repo-committed > global > built-in**, without the
generator hand-merging the two directories onto the same file. An agent with no entry in any layer defaults to
`model: inherit` with no `effort`.

## Generation scope: agent_harnesses

`agent_harnesses` does **not** gate which harness keys any block may carry; it gates only which
harness *directories* get generated files. The repo's own `agent_harnesses` — read from **either**
`.docket.local.yml` or `.docket.yml`, whichever declares the key first (local wins outright, not a
merge) — gates the **per-repo** generation pass (which `<repo>/.<H>/agents/` directories get
generated files). The user-level pass writes every harness `agents/` directory **present on disk**
— unless the global `config.yml` sets `agent_harnesses:`, which then governs the user-level target
list: creating listed dirs, skipping unlisted ones, and pruning docket-owned files from any
de-listed known harness (never rmdir'ing the harness root itself — it is the user's own config
directory, not a docket artifact; change 0050). The per-repo generation pass is governed solely by
the repo's own (local-then-committed) `agent_harnesses`, never the global value.

The per-repo generation fans out over the repo's
`agent_harnesses:` list (read from `.docket.local.yml` or `.docket.yml` — change 0051) —
**default `[claude]`** (byte-identical to before) — so each listed harness `H` gets generated
`<repo>/.<H>/agents/docket-*.md`; a Cursor repo sets `agent_harnesses: [claude, cursor]`. Explicit
over present-directory auto-detection, so a stray
`.cursor/` never silently mints generated files; an unknown harness token is warned-and-ignored. The
`sync-agents.sh --check` drift gate spans every generated per-harness file. (The user-level pass's
scope rule is stated once, above.) `agent_harnesses` is
read by a direct parse in `sync-agents.sh` (not `docket-config.sh`).

## Harness-portable model IDs

**Harness-portable model IDs (ADR-0015).** Agent `model:` values are **direct model
IDs, harness-neutral and passed through verbatim** — no tier layer. The running harness interprets the string (a Claude alias/ID under Claude Code; a Cursor
model ID like `gpt-5.5-medium-fast` under Cursor). This unvalidated **passthrough** is exactly what
lets docket drive non-Claude harnesses.

## Always-full-set generation + the Cursor dispatch rule

**Always-full-set generation, now machine-local, + the Cursor dispatch rule.** The **per-repo pass writes the full built-in agent set** for every harness in
`agent_harnesses` — the `agents:` block is **override-only** (it tunes a model/effort; it never
decides *which* agents exist, since the agents compose and a harness needs all of them). An
`agents:` entry naming no built-in is a typo warning. Per-repo generation is **opt-in**: a repo
opts in by declaring an `agents:` block or a top-level `agent_harnesses:` key, in **either** its
committed `.docket.yml` or its `.docket.local.yml`; a repo with neither key set in either file
generates no per-repo wrappers and its `--check` stays a no-op — pre-0048 behavior for
tracking-only repos. The generated files themselves are **gitignored, never committed** —
`<repo>/.<H>/agents/docket-*.md` (and the dispatch rule below) are regenerated from each machine's
own resolved config, not shared through git. `sync-agents.sh` maintains the marker-bounded
`# docket:start` / `# docket:end` block in the repo's `.gitignore`, covering every docket-owned
path, including every pattern it can generate for every harness (plus `.docket.local.yml` itself);
it writes or repairs that block the moment a repo
opts in, or merely carries a `.docket.local.yml`, and prints a one-time notice to commit the block.
A repo that predates this (0048-era committed copies) gets a one-time migration on the next run:
the tracked copies are deleted from the working tree, the local set is regenerated fresh, and the
single remedy commit (`git rm -r --cached … && git add .gitignore && git commit …`) is printed.
Additionally, the `cursor` harness gets a generated **`docket-dispatch.mdc`** rule
(`~/.cursor/rules/` user-level; `<repo>/.cursor/rules/` per-repo, also gitignored) that forces a
Task dispatch to the matching `subagent_type` — Cursor otherwise runs a directly-invoked skill
inline at the current model, defeating the pin. Because the per-repo pass generates that same full
set into the harness, the rule's dispatch targets resolve by construction. `sync-agents.sh` prunes
orphaned `docket-*` files (a removed built-in drops its wrapper; a de-listed harness drops its
wrappers and its dispatch rule) and `sync-agents.sh --check` spans the `.gitignore` block, the
tracked-file check, and (advisory) content staleness for both the agents and the dispatch rule.

Generated files are machine-local: per-repo wrappers were committed before the all-local model, so identical-on-every-clone pinning is retired — a deliberate trade-off; team defaults still live in the committed `.docket.yml` `agents:` block by convention, without CI-enforced pinning of generated copies.

## sync-agents.sh runs + the --check gate

`sync-agents.sh` runs **on demand** (install time, and after editing any config layer) — the same mental model as
`link-skills.sh`; it does NOT hook session start (silently regenerating out of band mid-session would be
surprising, and per-repo files are gitignored, so there is no commit to race). The drift backstop is
**`sync-agents.sh --check`**, a CI gate with three legs: (1) the managed docket `.gitignore` block
is present and current, and (2) no generated agent or dispatch-rule file is tracked by git — both are
**CI-meaningful** (`rc != 0`); (3) whether the local files on disk match what the resolved config would generate
is reported as `advisory:` output only — it never fails the build, since every machine regenerates its own copy.
