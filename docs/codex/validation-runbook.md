# Codex CLI live-validation runbook

This is the live-execution counterpart to `docs/codex/setup.md`. That doc covers *static*
Codex setup — opt-in scopes, model pinning, restart-after-regenerating. This runbook is the
checklist a human runs, by hand, inside a real Codex CLI session, to confirm the whole docket
loop actually works there: skills load, scripts run under Codex's own sandbox, generated
agents are visible, dispatch is honored, model/effort pins reach the spawned agent, and a
trivial metadata write lands end to end.

**Scope.** This validates docket running **natively inside Codex CLI** — a human opens Codex
in a docket-enabled repo and drives it directly. Change 0079's **`runner-dispatch`** (a Claude
Code parent process shelling out to `codex exec` as a child runner, via `scripts/runners/codex.sh`)
is the **opposite direction** and is **out of scope** here: that path never opens a Codex CLI
session at all, so none of the phases below exercise it. Do not treat a pass or fail in this
runbook as evidence about runner-dispatch, and vice versa.

## Environment

Fill this in before starting Phase 1 — every phase's observations are only meaningful pinned
to a specific build.

- **Codex CLI version:**
- **Model used:**
- **Date:**
- **OS:**

## Phase 1 — Setup (outside Codex)

This phase runs in an ordinary shell, not inside Codex — it prepares a disposable fixture repo
that Phase 2 onward will open in Codex CLI.

- [ ] 1. Create a fixture repo with an origin, then install docket into it:
  ```sh
  bash /path/to/docket/install.sh
  cd /path/to/fixture && bash /path/to/docket/migrate-to-docket.sh
  ```
  Expected: `install.sh` completes without error; `migrate-to-docket.sh` bootstraps the fixture
  onto docket (creates `.docket.yml`, the `docs/changes/` tree, and the `docket` metadata
  branch).

- [ ] 2. Opt the fixture in to the Codex harness — machine-local, so the fixture's committed
  config stays clean:
  ```sh
  printf 'agent_harnesses: [claude, codex]\n' > .docket.local.yml
  ```
  **This opt-in is required.** With `agent_harnesses` unset, `sync-agents.sh` early-returns and
  generates nothing for Codex, and `sync-agents.sh --check` is a **vacuous exit 0** in that
  case — a green `--check` alone proves nothing here. Assert artifacts directly (step 4 below).

- [ ] 3. Generate:
  ```sh
  bash /path/to/docket/link-skills.sh
  bash /path/to/docket/sync-agents.sh
  ```
  Expected: both scripts exit 0 and report writing Codex artifacts (skills symlinks, `.toml`
  wrappers, the `AGENTS.md` dispatch block).

- [ ] 4. Assert on disk:
  ```sh
  ls .codex/agents/docket-*.toml | wc -l          # expect the full built-in set
  grep -c 'docket:dispatch:start' AGENTS.md        # expect 1
  ls ~/.codex/skills                               # expect docket skill symlinks
  bash /path/to/docket/sync-agents.sh --check; echo "exit=$?"   # expect exit=0
  ```
  The full built-in set, as of this runbook, is nine agents: `docket-adr`,
  `docket-auto-groom`, `docket-auto-groom-critic`, `docket-brainstorm-consultant`,
  `docket-finalize-change`, `docket-implement-next`, `docket-integration-repair`,
  `docket-rebase-resolver`, `docket-status`. **Don't trust that printed list as a ceiling** —
  the set grows; compare the count against `ls agents/docket-*.md` in the docket clone itself
  before treating a mismatch as a bug.

  Note: `~/.codex/skills` is only populated if `~/.codex` already exists —
  `link-skills.sh` only links into a harness whose parent directory is present.

  **Pass when:** every `.toml` in the built-in set exists and parses, `AGENTS.md` carries
  exactly one dispatch block, `~/.codex/skills` holds the docket skill links, and `--check`
  exits 0.

## Phase 2 — Skills load and scripts run

This is where you first open the fixture in an actual Codex CLI session.

- [ ] 1. In Codex CLI, opened in the fixture repo, ask Codex to list its available skills.
  Expected: the docket skills (`docket-status`, `docket-adr`, `docket-implement-next`, etc.)
  are listed.

- [ ] 2. Ask Codex to run `docket-status`.
  Expected: the skill's convention step loads, and its bash reaches the helper scripts through
  the facade — `${DOCKET_SCRIPTS_DIR:?run docket/install.sh}/docket.sh preflight` — executing
  under **Codex's own sandbox** (not Claude Code's). This is the bash-compatibility smoke test:
  Cursor needed a dedicated sandbox/permissions guide at exactly this step — see
  `docs/cursor/permissions.md` for the shape that guide took there.

- [ ] 3. Record verbatim: every approval prompt, sandbox denial, or path/permission error, and
  what (if anything) had to be allowed to get past it. That transcript is the raw material for
  a future Codex analogue of the Cursor permissions guide, if one turns out to be needed.

**Pass when:** docket skills load and `docket.sh preflight` runs to a `BOOTSTRAP=PROCEED`
block without an unrecoverable sandbox denial.

## Phase 3 — Agents load

- [ ] 1. Ask Codex to list its available agents (or inspect its `/agent` surface, if Codex
  exposes one).
  Expected: the `docket-*` agents from the `.toml` files generated in Phase 1 are visible by
  name.

**Pass when:** every agent generated in Phase 1 is listed by name.

## Phase 4 — Dispatch honored

- [ ] 1. **Restart the Codex session first.** Codex registers agents at process start, so a
  session that has been open since before Phase 1 holds stale definitions — see
  `docs/codex/setup.md`'s *Restart after (re)generating* section.

- [ ] 2. Directly invoke a skill with a pinned wrapper — `docket-status` — the same way you did
  in Phase 2.
  Expected: Codex delegates to the matching `docket-status` agent per the `AGENTS.md` dispatch
  block, instead of running the skill inline. (This is the Cursor inline-dispatch-quirk test,
  replayed for Codex.)

- [ ] 3. **Record the answer explicitly** — this phase exists to settle a decision ADR-0036
  deferred to this validation. Only the project-level `<repo>/AGENTS.md` dispatch block exists
  today; the **user-level `~/.codex/AGENTS.md`** dispatch question was left open. Capture both:
  - (a) Is delegation automatic, prompted, or refused?
  - (b) Does dispatch work for a skill invoked outside the fixture repo, or is a user-level
    `~/.codex/AGENTS.md` needed for globally-scoped agents?

  A "user-level dispatch is needed" answer becomes a follow-up `proposed` stub — **do not
  implement it as part of this runbook.**

**Pass when:** there is a definitive, recorded answer to both (a) and (b) — "Codex refuses to
delegate" is itself a passing outcome for this phase; the phase is about getting an answer, not
about the answer being yes.

## Phase 5 — Pin honored

- [ ] 1. Discover real Codex model slugs — never assume one; `config.yml.example`'s Codex IDs
  are explicitly labeled unvalidated examples:
  ```sh
  codex debug models | jq -r '.models[] | .slug'
  ```

- [ ] 2. Pin one agent to a distinctive slug + effort in the fixture's `.docket.local.yml`
  (agent keys are bare and un-prefixed, nested under the harness):
  ```yaml
  agents:
    codex:
      status: { model: <slug-from-codex-debug-models>, effort: xhigh }
  ```

- [ ] 3. Regenerate and confirm the values reached the wrapper, then restart Codex and dispatch
  the agent, asking it to report its own model identity:
  ```sh
  bash /path/to/docket/sync-agents.sh
  grep -E 'model|model_reasoning_effort' .codex/agents/docket-status.toml
  ```
  Expected: the `.toml` carries `model = "<slug>"` and `model_reasoning_effort = "xhigh"` —
  docket's `effort:` maps to that key **verbatim**; there is no `max`→`xhigh` remap at this
  layer — and the spawned agent, when asked, reports the pinned model.

  Note: with **no** `agents.codex.*` override, the wrappers carry the built-in **Claude**
  model IDs, which Codex cannot run — `sync-agents.sh` warns about exactly this. A pin is
  therefore mandatory for this phase to mean anything; skipping step 2 makes step 3 untestable.

**Pass when:** there is a definitive recorded answer to "does the spawned agent run the pinned
model/effort?" — including a negative.

## Phase 6 — End-to-end metadata write

- [ ] 1. From inside Codex, run a trivial `docket-new-change` stub through to completion.
  Expected: the change-file commits land on `origin/docket`, the Board pass reports
  `board inline changed pushed`, and the board renders the new stub.

- [ ] 2. Verify against the remote, not the local tree:
  ```sh
  git -C .docket log --oneline -3 origin/docket
  ```

This proves the whole producer loop — preflight, metadata worktree sync, must-land push, board
render — works end to end under a non-Claude harness.

**Pass when:** the stub's commits are on `origin/docket` and the Board pass reported
`board inline changed pushed`.

## Pass criteria

The runbook as a whole **passes** when:
- Phases 1, 2, 3, and 6 pass as stated above, **and**
- Phases 4 and 5 each have a **definitive observed answer**, whether that answer is yes or no.

A "no" on Phase 4 or Phase 5 is a valid, recordable outcome — it is not a runbook failure, it
is information. Every gap uncovered by any phase (a missing sandbox allowance, a dispatch that
doesn't fire, a pin that doesn't reach the agent, a user-level dispatch need) becomes a
follow-up `proposed` stub rather than something patched mid-run. Do not implement fixes while
executing this runbook — record, then propose.

## Recording results

Write up the executed run — environment block, phase-by-phase observations, verbatim
transcripts of any prompts/denials, and the answers to Phases 4 and 5 — as a close-out doc
under `docs/results/`, following the naming convention of the files already there. Link any
follow-up stubs the run surfaced from that doc.
