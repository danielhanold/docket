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

All four scripts this phase mentions — `sync-agents.sh`, `link-skills.sh`, `install.sh`, and
`migrate-to-docket.sh` — live at the **repo root** of the docket clone. There is **no**
`scripts/` prefix. Citing them at a fabricated `scripts/`-prefixed path is the exact error this
change's own spec shipped; every command below invokes them as `/path/to/docket/<script>.sh`,
not `/path/to/docket/scripts/<script>.sh`.

**Working directory matters.** `sync-agents.sh` resolves the target repo as `REPO="$PWD"` — it
configures *whatever repo you are standing in*, not the docket clone the script lives in. Each
command below states its cwd explicitly; run them from that cwd.

**`migrate-to-docket.sh` is not part of this phase.** It is the **migration** path — it moves an
**existing** repo that already has a live planning surface (`docs/changes/`, a board) onto the
docket branch model, by seeding the metadata branch *from those existing dirs*. On a fresh
fixture there is nothing to seed and it exits 1 by design. The fresh-repo path is
`docket.sh bootstrap` (step 2).

- [ ] 1. **Machine setup**, once per machine — not a per-repo step:
  ```sh
  bash /path/to/docket/install.sh
  ```
  `install.sh` sets up *this machine*: it links the docket skills into each present harness and
  exports `DOCKET_SCRIPTS_DIR` into your shell profile. It does not touch any project repo.

  Then **open a fresh shell** and confirm the export took:
  ```sh
  exec "$SHELL" -l
  echo "${DOCKET_SCRIPTS_DIR:-UNSET}"   # expect an absolute path ending in /docket/scripts
  ```
  This matters for Phase 2: `install.sh` writes the export to your **shell profile** (and to
  Claude Code's `settings.json`) — **neither of which Codex reads**. The variable reaches a Codex
  session only by ordinary process-environment inheritance from the shell that launches it. If
  you skip the fresh shell, every docket skill inside Codex will fail at its first command with
  `DOCKET_SCRIPTS_DIR: run docket/install.sh`, which is an env-propagation problem and **not** a
  sandbox problem. Phase 2 step 0 tells the two apart.

- [ ] 2. **Create a disposable fixture repo with an origin, then bootstrap it onto docket.**
  The fixture's `origin` **must be an absolute URL.** A relative one (`../origin.git`) survives
  bootstrap but breaks `preflight`: the metadata-worktree sync fetches from *inside* `.docket`,
  where the relative path no longer resolves, and git fails with
  `fatal: '../origin.git' does not appear to be a git repository`.
  ```sh
  cd /path/to/fixture
  git remote get-url origin     # confirm it is ABSOLUTE (/abs/path/origin.git or a URL)
  "${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh bootstrap
  ```
  Expected: the bootstrap guard sees a fresh repo (no `docket` branch, no live planning surface),
  creates the metadata branch, and prints:
  ```
  docket-config: seeded the managed .gitignore block in <fixture>/.gitignore — COMMIT THIS …
  BOOTSTRAP=PROCEED
  ```
  Concretely, this creates exactly two things: an **empty orphan `docket` branch on `origin`**,
  and the **managed block in the fixture's `.gitignore`** (commit that). It does **not** write
  `.docket.yml` — that file is optional, and no docket script ever creates it — and it does
  **not** create a `docs/changes/` tree. Those arrive later: `docs/changes/` appears with the
  Phase 6 stub. An empty `git ls-tree -r origin/docket` here is correct, not a failure.

- [ ] 3. **Discover real Codex model slugs.** Never assume one; `config.yml.example`'s Codex IDs
  are explicitly labeled unvalidated examples:
  ```sh
  codex debug models | jq -r '.models[] | .slug'
  ```
  Pick one distinctive slug from this list for step 4. If this command fails or returns nothing,
  **stop** — every Codex-side phase below depends on the wrappers naming a model Codex can
  actually run.

- [ ] 4. **Opt the fixture in to the Codex harness AND pin every Codex agent**, in one
  machine-local file so the fixture's committed config stays clean:
  ```sh
  cd /path/to/fixture
  cat > .docket.local.yml <<'YAML'
  agent_harnesses: [claude, codex]
  agents:
    codex:
      status:                { model: <slug>, effort: xhigh }
      adr:                   { model: <slug>, effort: xhigh }
      auto-groom:            { model: <slug>, effort: xhigh }
      auto-groom-critic:     { model: <slug>, effort: xhigh }
      brainstorm-consultant: { model: <slug>, effort: xhigh }
      finalize-change:       { model: <slug>, effort: xhigh }
      implement-next:        { model: <slug>, effort: xhigh }
      integration-repair:    { model: <slug>, effort: xhigh }
      rebase-resolver:       { model: <slug>, effort: xhigh }
  YAML
  ```
  Substitute the slug from step 3 for `<slug>`. Agent keys are **bare and un-prefixed**, nested
  under the harness; each generates `.codex/agents/docket-<key>.toml`.

  **Why the opt-in line is required.** `agent_harnesses:` is what targets the codex harness.
  Note the two keys do *different* jobs and are read *separately*: an `agents:` block alone
  already counts as per-repo opt-in, but harness targeting is resolved from `agent_harnesses:`
  on its own — with `agents:` present and `agent_harnesses:` **unset**, `sync-agents.sh` does not
  early-return; it generates for **`claude` only** (the default), silently producing nothing for
  Codex. Only with *neither* key present does it early-return and generate nothing at all — and
  in that case `sync-agents.sh --check` is a **vacuous exit 0**, so a green `--check` alone proves
  nothing here. Assert artifacts directly (step 6).

  **Why the pin must be here, not later.** With no `agents.codex.*` override reaching these
  wrappers, they carry whatever the lower config layers supply, and **neither default is runnable
  under Codex**:
  - If your `~/.config/docket/config.yml` has **no** `agents.codex` block, the wrappers fall back
    to the built-in **Claude** model IDs (`claude-opus-4-8`, …), which Codex cannot run.
    `sync-agents.sh` warns loudly — one `WARN … may not be a valid model ID for harness 'codex'`
    per (non-claude harness × agent).
  - If it **does** have one (the shipped `config.yml.example` block, copied), the wrappers carry
    those slugs — which that file's own comment labels **UNVALIDATED examples** — and you get
    **no warning at all**. This is the more dangerous case: silently plausible, possibly fake.

  Either way, an unpinned Phase 3 or Phase 4 would be observing agents Codex cannot spawn, and a
  refusal there would be **indistinguishable** from "Codex does not honor the dispatch block" —
  manufacturing a false answer to the very question ADR-0036 deferred to this runbook. Pinning
  all nine here, from `.docket.local.yml` (which overrides both the repo and global layers),
  makes every later phase observe a runnable wrapper.

  If you later edit this file, **do not overwrite it** and do not drop the `agent_harnesses:`
  line: losing it untargets codex, and the next regenerate treats every
  `.codex/agents/docket-*.toml` as an orphan and deletes it.

- [ ] 5. Generate:
  ```sh
  cd /path/to/fixture
  bash /path/to/docket/link-skills.sh     # global skill symlinks; cwd-independent
  bash /path/to/docket/sync-agents.sh     # reads .docket.local.yml from $PWD — cwd MUST be the fixture
  ```
  Expected: both scripts exit 0 and report writing Codex artifacts (skills symlinks, `.toml`
  wrappers, the `AGENTS.md` dispatch block).

  **No `WARN … may not be a valid model ID for harness 'codex'` line should appear for the
  project-level pass.** If one does, your step-4 pin did not land (check the YAML shape and that
  cwd is the fixture) — fix it now rather than recording it as a finding; every Codex-side phase
  below is invalid until it is clean.

- [ ] 6. Assert on disk:
  ```sh
  cd /path/to/fixture
  ls .codex/agents/docket-*.toml | wc -l           # expect the full built-in set
  grep -h '^model = ' .codex/agents/docket-*.toml | sort -u   # expect ONLY your pinned <slug>
  grep -c 'docket:dispatch:start' AGENTS.md        # expect 1
  ls ~/.codex/skills                               # expect docket skill symlinks
  bash /path/to/docket/sync-agents.sh --check; echo "exit=$?"   # expect exit=0
  ```
  The full built-in set, as of this runbook, is nine agents: `docket-adr`,
  `docket-auto-groom`, `docket-auto-groom-critic`, `docket-brainstorm-consultant`,
  `docket-finalize-change`, `docket-implement-next`, `docket-integration-repair`,
  `docket-rebase-resolver`, `docket-status`. **Don't trust that printed list as a ceiling** —
  the set grows; compare the count against `ls agents/docket-*.md` in the docket clone itself
  before treating a mismatch as a bug. If the set has grown, add the new keys to step 4's pin.

  Note: `~/.codex/skills` is only populated if `~/.codex` already exists —
  `link-skills.sh` only links into a harness whose parent directory is present.

  **Pass when:** every `.toml` in the built-in set exists, parses, and carries your pinned slug;
  `AGENTS.md` carries exactly one dispatch block; `~/.codex/skills` holds the docket skill links;
  and `--check` exits 0.

## Phase 2 — Skills load and scripts run

This is where you first open the fixture in an actual Codex CLI session. Launch Codex **from the
fresh shell of Phase 1 step 1** — that is the only way `DOCKET_SCRIPTS_DIR` reaches it.

- [ ] 0. **First, establish whether the facade's env var even arrived.** From inside Codex, ask
  it to run:
  ```sh
  echo "${DOCKET_SCRIPTS_DIR:-UNSET}"
  ```
  This is a three-way fork, and the outcomes are **not** interchangeable — record which one you
  got:
  - **An absolute path** → env propagation works; continue to step 1.
  - **`UNSET`** → an **env-propagation** outcome, *not* a sandbox denial. Either Codex was
    launched from a shell that predates `install.sh`, or Codex's sandbox scrubs the process
    environment. Distinguish: relaunch Codex from a fresh login shell where
    `echo "$DOCKET_SCRIPTS_DIR"` prints the path. If it is *still* `UNSET` inside Codex, the
    sandbox is scrubbing it — record that, and it becomes a follow-up stub about how Codex should
    receive the var.
  - **A denial/approval prompt on `echo` itself** → a sandbox outcome; record it per step 3.

- [ ] 1. Ask Codex to list its available skills.
  Expected: the docket skills (`docket-status`, `docket-adr`, `docket-implement-next`, etc.)
  are listed.

- [ ] 2. Ask Codex to run `docket-status`.
  Expected: the skill's convention step loads, and its bash reaches the helper scripts through
  the facade — `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh preflight` — executing
  under **Codex's own sandbox** (not Claude Code's). This is the bash-compatibility smoke test:
  Cursor needed a dedicated sandbox/permissions guide at exactly this step — see
  `docs/cursor/permissions.md` for the shape that guide took there.

- [ ] 3. Record verbatim: every approval prompt, sandbox denial, or path/permission error, and
  what (if anything) had to be allowed to get past it. That transcript is the raw material for
  a future Codex analogue of the Cursor permissions guide, if one turns out to be needed.

**Pass when:** `DOCKET_SCRIPTS_DIR` resolves inside Codex, docket skills load, and
`docket.sh preflight` runs to a `BOOTSTRAP=PROCEED` block without an unrecoverable sandbox
denial. A failure here must be recorded as **one of** env-propagation (step 0 `UNSET`) or
sandbox denial (step 3) — they are different findings and produce different follow-up stubs.

## Phase 3 — Agents load

Precondition: Phase 1 step 6 showed every wrapper carrying your pinned slug. If it did not, stop
— an agent Codex cannot spawn tells you nothing about whether Codex loads agents.

- [ ] 1. Ask Codex to list its available agents (or inspect its `/agent` surface, if Codex
  exposes one).
  Expected: the `docket-*` agents from the `.toml` files generated in Phase 1 are visible by
  name.

**Pass when:** every agent generated in Phase 1 is listed by name.

## Phase 4 — Dispatch honored

- [ ] 1. **Restart the Codex session first.** Codex registers agents at process start, so a
  session that has been open since before Phase 1 holds stale definitions — see
  `docs/codex/setup.md`'s *Restart after (re)generating* section. Restart from the same fresh
  shell, so `DOCKET_SCRIPTS_DIR` is still inherited.

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

  Before recording a **refusal** as the answer to (a), rule out the mundane causes: the wrapper
  names a slug Codex cannot run (re-check Phase 1 step 6), or the session predates the wrappers
  (step 1). A refusal caused by either is a *setup* fault, not evidence about dispatch, and
  recording it as the latter would settle ADR-0036 on a false finding.

**Pass when:** there is a definitive, recorded answer to both (a) and (b) — "Codex refuses to
delegate" is itself a passing outcome for this phase; the phase is about getting an answer, not
about the answer being yes.

## Phase 5 — Pin honored

Phase 1 established the pin and confirmed it reached the `.toml` on disk. This phase asks the
one question that file cannot answer: **does the pin reach the running agent?**

- [ ] 1. Re-confirm the on-disk baseline (cwd = fixture):
  ```sh
  cd /path/to/fixture
  grep -E 'model|model_reasoning_effort' .codex/agents/docket-status.toml
  ```
  Expected: the `.toml` carries `model = "<slug>"` and `model_reasoning_effort = "xhigh"` —
  docket's `effort:` maps to that key **verbatim**; there is no `max`→`xhigh` remap at this
  layer. (The remap exists only in the runner adapter, `scripts/runners/codex.sh`, which is out
  of scope here.)

- [ ] 2. From inside Codex, dispatch `docket-status` and ask the spawned agent which model it is
  running.
  Expected: it reports the pinned `<slug>`, not Codex's session default.

- [ ] 3. Record the answer: does the spawned agent run the pinned model and effort? A **no** —
  e.g. Codex silently substitutes its house default — is a valid, recordable outcome and a
  follow-up stub. Note that a silent substitution is only detectable *because* the slug you
  pinned in Phase 1 was distinctive; if the agent reports Codex's default, say so explicitly.

**Pass when:** there is a definitive recorded answer to "does the spawned agent run the pinned
model/effort?" — including a negative.

## Phase 6 — End-to-end metadata write

- [ ] 1. From inside Codex, run a trivial `docket-new-change` stub through to completion.
  Expected: the change-file commits land on `origin/docket`, the Board pass reports
  `board inline changed pushed`, and the board renders the new stub. This is also where the
  fixture's `docs/changes/` tree first appears — Phase 1's bootstrap did not create it.

- [ ] 2. Verify against the remote, not the local tree:
  ```sh
  cd /path/to/fixture
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

The one exception is Phase 1: a fault there (a pin that did not land, an unset
`DOCKET_SCRIPTS_DIR`, a relative origin) is **setup**, not a finding. Fix it and restart the
phase — recording it as a result would attribute a setup mistake to Codex.

## Recording results

Write up the executed run — environment block, phase-by-phase observations, verbatim
transcripts of any prompts/denials, and the answers to Phases 4 and 5 — as a close-out doc
under `docs/results/`, following the naming convention of the files already there. Link any
follow-up stubs the run surfaced from that doc.
