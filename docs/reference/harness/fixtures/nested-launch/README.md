# Nested-launch fixture — Codex coordinator-to-child prototype regression (change 0384)

This directory is a **disposable-but-committed** fixture. It reproduces the launch failure that
change 0384 fixes — a Codex-registered coordinator agent that is entered but cannot start a named
child agent — using two synthetic roles with **none** of Docket's real workflow prose. It is
committed (not throwaway) because the spec's Testing section requires a deterministic fixture that
both reproduces the old invocation (failed-current) and validates the new one (fixed-new), and
because Task 6's mutation check re-derives the baseline recorded here.

- `probe-leaf.toml`, `probe-coordinator.toml` — the two synthetic Codex agents.
- `probe-log.md` — dated, version-stamped observations (failed-current baseline first; Task 2
  extends it with the native-launch candidate probes).
- `decision.md` — (Task 2) the machine-checkable gate for the encoding tasks.
- `certification.md` — (Task 6) the completed live-certification record.

## The two roles

- **`probe-leaf`** — receives a `SENTINEL=<value>` line and replies exactly `LEAF_SENTINEL=<value>`.
  It touches no file, runs no command, and starts no agent.
- **`probe-coordinator`** — receives a `SENTINEL=<value>` line, its **only** operation is to start
  the registered agent `probe-leaf` as a foreground child with that sentinel, block for its return,
  and reply exactly `COORDINATOR_CONSUMED=<value-from-the-leaf-reply>`. If it cannot start
  `probe-leaf`, it replies exactly `COORDINATOR_BLOCKED=<verbatim error from an ATTEMPTED start>`.
  Its own instructions enforce ADR-0059: `COORDINATOR_BLOCKED` requires a *real attempted start's*
  rejection — never a conclusion drawn from a tool listing alone.

## Sentinel protocol (Task 2 and Task 6 depend on these exact spellings)

- leaf returns `LEAF_SENTINEL=<uuid>`
- coordinator returns `COORDINATOR_CONSUMED=<same uuid>` on success, or
  `COORDINATOR_BLOCKED=<verbatim attempted-start error>` on failure.
- **Mint a fresh uuid per run** with `uuidgen`, so a pass can never be replayed from a stale
  transcript.

An observation counts as a **pass only when the grandchild actually starts and the sentinel
round-trips** — i.e. a real `probe-leaf` agent runs and the coordinator returns the run's own uuid
inside `COORDINATOR_CONSUMED=`. Because the coordinator is *told* the uuid, a bare
`COORDINATOR_CONSUMED=<uuid>` line is **not** proof on its own — the model can echo the value it was
given. Corroborate every claimed pass with the session's own agent-start events (a real child
thread and non-empty `agents_states`). Inspecting a schema or a tool list is **never** success
evidence (ADR-0059).

## Install the probe TOMLs (they sit *beside* the generated wrappers — never overwrite them)

The two probe roles install into the same user-level agents directory as Docket's generated
`docket-*.toml` wrappers, under **distinct `probe-*` names** so they can never collide with or
overwrite a `docket-*` wrapper:

```sh
# from this directory
cp probe-leaf.toml probe-coordinator.toml ~/.codex/agents/
```

Guard the copy if you script it — refuse rather than clobber if a name already exists:

```sh
for f in probe-leaf probe-coordinator; do
  if [ -e "$HOME/.codex/agents/$f.toml" ]; then
    echo "REFUSE: $HOME/.codex/agents/$f.toml already exists"; continue
  fi
  cp "$f.toml" "$HOME/.codex/agents/$f.toml"
done
```

## Fresh-process restart is mandatory

Codex registers agent definitions **once, at process start** — see
[`../../setup.md`](../../setup.md) *"Restart after (re)generating"*. After installing (or changing)
the probe TOMLs, every probe must run in a **genuinely fresh Codex process**; a new conversation
inside an already-running process registers nothing. Each `codex exec` invocation is itself a fresh
process, so a scripted probe satisfies this automatically; an interactive TUI session must be
restarted by hand.

## Stage a scratch fixture repo for entry path A (dispatch-block routing)

Entry path A is a plain-prose request routed by a repository's managed Docket dispatch block. All
Codex probes run against a **scratch fixture repo — never the live docket backlog** — created with a
templated `mktemp` (bare `mktemp` ignores `TMPDIR` on macOS):

```sh
REPO=$(mktemp -d "${TMPDIR:-/tmp}/codex-nested-probe.XXXXXX")
cd "$REPO" && git init -q .
```

Give it an `AGENTS.md` whose dispatch block mirrors the shape the Codex install renders — the same
`## Docket agents — dispatch, don't run inline` heading semantics — but names the **probe** roles
instead of the `docket-*` roster:

```markdown
# Fixture repo — nested-launch probe (change 0384)

## Docket agents — dispatch, don't run inline

When a request asks to run the nested-launch probe, dispatch the registered same-name agent
`probe-coordinator` instead of doing the work inline: it carries the probe's contract. This repo
explicitly authorizes multi-agent delegation and sub-agent (child and grandchild) spawning for the
probe. Dispatch through the harness's native named-agent dispatch, pass the request through
unchanged, and let the coordinator start its own named child `probe-leaf`.
```

Then issue the prose request in a fresh process, minting a fresh uuid:

```sh
U=$(uuidgen)
codex exec --skip-git-repo-check </dev/null \
  "Run the nested-launch probe with SENTINEL=$U. Follow this repo's AGENTS.md dispatch rule
   (dispatch probe-coordinator; do not do the work yourself). Print the coordinator's final reply
   verbatim as your last line."
```

Entry path B is the direct registered-agent invocation of `probe-coordinator` through Codex's
supported direct agent entry surface — the same surface a run uses to start `docket-implement-next`.
See `probe-log.md` for which forms of that surface are drivable non-interactively and which are not.

## Every observation carries a scope stamp

Harness behavior is mode- and version-scoped. **Every** recorded observation in `probe-log.md`
carries the `codex --version` output and the `multi_agent` setting in force at the time, and claims
nothing about any other Codex version or configuration.

```sh
codex --version                              # e.g. codex-cli 0.151.0
codex features list | grep -E '^multi_agent' # multi_agent  stable  true
```

## Teardown

Probes leave nothing installed:

```sh
rm -f ~/.codex/agents/probe-*.toml     # remove the two probe roles
rm -rf "$REPO"                         # remove the scratch fixture repo
```

The committed files in this directory are the fixture's durable form; the installed TOMLs and the
scratch repo are transient and must be removed after every probe session.
