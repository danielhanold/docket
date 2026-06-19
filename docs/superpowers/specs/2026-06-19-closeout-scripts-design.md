# Design: extract the shared terminal-transition close-out mechanics into deterministic scripts (change 0025)

**Status:** design (brainstormed 2026-06-19)
**Change:** 0025
**Depends on:** — (foundational; consumes `scripts/lib/docket-frontmatter.sh` from 0022, already merged)
**Unblocks:** 0023 — its deferred "script the merge sweep" piece (§5b) needs exactly the shared close-out helper this change builds.
**Precedent:** 0011 (`github-mirror.sh`, ADR-0007) · 0022 (`render-board.sh`) — same extraction pattern.

---

## 1. Context

A single `docket-finalize-change` run costs real money (~$3.50 was the trigger for
this change). Instrumenting one such run showed the cause: of ~30 model turns, only
~4 needed judgment (the learnings harvest). The other ~26 were the model narrating
and sequencing **pure git/gh mechanics** step-by-step — and every one of those turns
re-sent the full context (the finalize SKILL ≈ 3.6k words + the convention ≈ 3.6k
words + a growing transcript + per-turn reasoning). The cost is paying a reasoning
model to drive a shell sequence it already knows.

The biggest mechanical block is the **terminal-transition close-out**: archive the
change on `metadata_branch`, copy its terminal records onto the integration branch
(terminal-publish), and remove the feature branch + worktree. This exact primitive
runs in **three** places — `docket-finalize-change`, `docket-status`'s merge sweep,
and the two kill paths (`docket-new-change` proposed-kill, `docket-implement-next`
reconcile-kill). Change **0023 explicitly deferred** scripting the sweep for this
reason ("its archive add is the *same* terminal-transition primitive
`docket-finalize-change` performs and must not diverge from; scripting it correctly
means routing **both** through one shared archive helper — out of scope [there]" —
0023 §5b). This change builds that shared helper.

It is the same extraction the project has already done twice: 0011 lifted the GitHub
mirror into `github-mirror.sh`; 0022 lifted the board render into `render-board.sh`.
The close-out mechanics are the next pure, judgment-free transformation to lift.

## 2. Guiding principle (ADR-0007, applied)

> Mechanical, judgment-free work that today runs on model tokens moves into a
> **deterministic, idempotent, seam-tested script**; the skill keeps owning *when*
> to run it and the human-legible commit messages, the script owns *how*.

This is the same split `render-board.sh` has with `docket-status` ("the skill keeps
owning *when* to render … the script owns *how* to render"). It is governed by
ADR-0007 and touches the procedures of ADR-0001 (publish by copy, not merge) and
ADR-0002 (terminal-publish single-sourced in finalize). **No new ADR is required** —
this is a faithful extraction, as 0022 needed none. One review item for build time:
confirm whether ADR-0002's "single-sourced in finalize" wording wants a one-line
`## Update` clarifying that finalize remains the documented owner while delegating
the *mechanics* to the script (exactly as `docket-status` delegates to
`render-board.sh`). Not blocking.

## 3. The git-write boundary (the load-bearing decision)

The token cost and the fragility both live in the **multi-branch git dance**:
the archive commit + push on `docket`, then the terminal-publish copy + the
compare-and-swap (CAS) retry push onto the integration branch. Two precedents
bracket the choice — `render-board.sh` does **no** git writes (caller commits);
`github-mirror.sh` **owns** its external writes behind a mock seam.

**Decision: the scripts own the deterministic plumbing *and* the CAS loops; the
model authors each commit message and passes it in.**

- The model authors the commit message (contextual and human-legible, e.g.
  `docket(0022): done — PR #35 merged, archived (status done, merged 2026-06-19)`)
  and passes it as `--message`. Each script ships a sensible **default** message, so
  `--message` is an override, not a requirement.
- The script runs `git commit -m` + the push/CAS loop **internally**, so the
  retry-and-rebase-redo stays atomic and testable rather than being driven
  turn-by-turn by the model.

**Why this is the right risk trade.** The only alarming write is the push to the
integration branch — docket's most concurrency-exposed write. But the risk there is
**plumbing-correctness** (copyset construction, branch targeting, the CAS loop), not
**judgment** — and plumbing-correctness is exactly what tests pin down. The model's
"judgment" at the push step adds ~nothing today (the finalize run executed the
documented bash verbatim), while a test seam adds coverage you cannot get from "the
model will probably notice." The scripts do a **normal** CAS push to the integration
branch, never a force-push (only the feature branch is force-pushed, and that lives
in the merge gate — out of scope here). The archive commit stays **change-file-only
and tree-identical** across concurrent archivers (a property of the staged *tree*,
which the script controls deterministically — a differing message does not affect
the CAS race).

**Verification — the scripts self-confirm; the exit code *is* the confirmation.**
Today the model re-confirms every mechanical step by hand (this session: `git ls-tree
origin/main` after the merge, the archive probe, the re-fetch + diff after publish,
the idempotence re-render). That eyeballing is itself a slice of the token cost — and
it is *weaker* than a test, because "looks right" is not an invariant. So each script
**owns its own postcondition check and is fail-closed**: it asserts that reality
matches intent before exiting 0, and returns **non-zero with a diagnostic** on any
deviation —

- `archive-change.sh`: the dated file exists under `archive/`, `active/` no longer
  holds it, frontmatter `status`/`updated` are set, and the push was accepted;
- `terminal-publish.sh`: after the CAS push, **re-fetch and assert** the full copy-set
  is present on `origin/<integration>`, and `pub-N` is torn down;
- `cleanup-feature-branch.sh`: the worktree and the branch are both gone.

The skill then **trusts the exit code** — zero ⇒ proceed with **no re-confirmation
turn**; non-zero ⇒ **abort-and-report** (the existing autonomous contract: surface the
diagnostic, leave the change recoverable). Mechanical confirmation thus moves *into*
the script (deterministic, tested, one fewer model turn each); the model re-enters
only for the confirmations that were never mechanical — the merge decision, the
harvest, the gate sign-off. Net: **fewer** confirmation turns than today, and a
*stronger* guarantee on each. (This is why the §8 tests assert each script's
postcondition *and* its non-zero exit on an injected deviation — the fail-closed
behavior is itself covered.)

## 4. Components

Three small scripts under `scripts/`, each sourcing the existing
`scripts/lib/docket-frontmatter.sh` (`field`/`list_field`) — no new parser. `done`
and `killed` are **unified through one primitive**, which is what lets all three
call sites reuse it.

### 4a. `scripts/archive-change.sh`

```
archive-change.sh --changes-dir DIR --id N --outcome done|killed --date YYYY-MM-DD \
                  [--message M] [--results PATH]   # done: results link
                  [--message M] [--reason R]       # killed: ## Why killed text
```

In the metadata working tree (`--changes-dir` points at its `docs/changes`):

1. **Reuse-existing probe** (null-glob-safe): if `archive/*-N-<slug>.md` already
   exists, reuse that filename (ignore `--date`); if the change is already terminal,
   no-op and exit 0. This is what makes a sweep racing finalize a safe no-op, and
   what keeps an interrupted-then-resumed run from minting a second archive file
   across a day boundary.
2. `mkdir -p archive`; `git mv active/N-<slug>.md archive/<DATE>-N-<slug>.md`.
3. Set frontmatter: `status: <outcome>` + `updated: <DATE>`; for `done` write the
   `results:` link if a results file is named; for `killed` append a `## Why killed`
   section (text supplied via `--reason`/stdin — see §6).
4. Commit **change-file-only** with `-m "$MESSAGE"` (default if unset); push the
   current branch with `pull --rebase` retry on non-fast-forward.

`--date` is caller-supplied because the date *source* differs by outcome (merge date
for `done`, kill-commit date for `killed`) and must be UTC and stable — the caller
already computes it. The reuse probe makes `--date` matter only on the first archive.

### 4b. `scripts/terminal-publish.sh`

```
terminal-publish.sh --id N --outcome done|killed --integration-branch B --metadata-branch M [--message MSG]
```

The shared *Terminal publish (docket-mode)* procedure, scripted verbatim from its
current prose:

1. **Mode guard:** if `metadata-branch == integration-branch` (main-mode), no-op +
   log and exit 0 — there is no `docket` branch to copy from.
2. Provision a transient `pub-N` worktree in a temp dir on `origin/<integration>`
   (`git worktree prune` then `git worktree add -B`, so a leaked branch/registration
   is adopted — re-run safe).
3. Build the **copy-set as a list**: the archived change file (always); the `spec:`
   path iff non-empty; each `adrs:` entry **whose ADR file `status:` is `Accepted`**
   (the Accepted gate fires here, at the copy site — `Proposed`/draft ADRs skipped).
4. `git -C pub fetch origin docket` then `checkout origin/docket -- <copyset>`;
   guarded commit (`diff --cached --quiet ||`) with `-m "$MSG"`; **CAS-push**
   `HEAD:<integration>` with the re-copy-on-conflict retry loop.
5. Tear down (force-remove the temp worktree, delete `pub-N`, `rm -rf` the temp dir).

Idempotent: the guarded copy+commit is a no-op when bytes already match, and the push
loop completes an interrupted push.

### 4c. `scripts/cleanup-feature-branch.sh`

```
cleanup-feature-branch.sh --slug S [--worktrees-dir DIR]
```

Provenance-guarded removal: remove the worktree **only** if its path resolves under
`.worktrees/<slug>` (never the `.docket/` metadata worktree, never an out-of-tree
worktree); then delete the local and (if present) the remote `feat/<slug>` branch.
The guard is the safety invariant worth pinning in a tested script.

## 5. What the skills keep owning

The scripts move *how*, not *when* or *what*. Unchanged and still model-driven:

- **Sequencing** — finalize/sweep/kill decide which scripts to call, in what order.
- **Trusting the exit code** — proceed on 0, abort-and-report on non-zero; the skill
  does **not** re-confirm a mechanical step by hand (the script self-verified, §3).
- **The merge decision and the merge gate** (rebase-retest, conflict/red agents) —
  entirely out of scope (§7).
- **Harvest learnings** — judgment; stays model-driven, its own commit on `docket`.
- **Commit-message authorship** — the model writes each `--message`.
- **Board refresh** — already `render-board.sh`; unchanged.

`docket-finalize-change`, `docket-status`'s sweep, `docket-new-change`'s
proposed-kill, and `docket-implement-next`'s reconcile-kill are rewired to **invoke
the scripts** instead of restating the bash. Their SKILL prose shrinks to "author a
message → call the script," which is the per-turn input-token saving that compounds
across every future close-out.

## 6. Open design points (resolved)

- **`## Why killed` text:** passed via `--reason "…"` (the kill rationale the model
  already writes); the script inserts the section. Keeps the script pure-mechanical.
- **Test seam:** prefer **hermetic local git** over mocking `git` — a temp repo with
  a local *bare* origin carrying both `docket` and `<integration>` branches, so the
  archive → publish → CAS loop is exercised end-to-end with real push semantics and
  no network. (Close-out makes no `gh` calls, so no `MOCKGH` is needed here.) A
  `GIT=` indirection stays available as a thin seam but is not the primary mechanism.
- **Concurrency:** a test injects a competing push between provision and push to
  exercise the CAS retry; a second back-to-back run asserts the no-op idempotency
  (the racing-sweep guarantee).

## 7. Out of scope

- **The merge-gate spine** (rebase → run suite → force-push) — a separate, higher-
  risk decision; not in this change (deliberately deselected).
- **The config-resolution / bootstrap-guard helper** (`docket-config.sh`) — its own
  change; broad blast radius across all skills, distinct concern.
- **The harvest** — stays model-driven (judgment).
- **Any behavior change** — this moves work from model to script; the archive
  filename contract, terminal-publish copy-set rules, Accepted-ADR gate, and
  idempotency guarantees are reproduced exactly, not redesigned.
- **The health checks** and **the `github` surface** — 0023 / already scripted.

## 8. Testing

`tests/test_closeout.sh`, matching `tests/test_github_mirror.sh` /
`tests/test_render_board.sh`:

- **Fixtures:** a hermetic temp repo + bare origin with `docket` + `<integration>`,
  seeded change/spec/ADR files (one `Accepted` ADR, one `Proposed` ADR to prove the
  gate skips it).
- **archive-change.sh:** asserts the dated rename, the frontmatter delta for both
  `done` and `killed`, change-file-only commit, and the **reuse-existing-file** /
  already-terminal no-ops (run twice → identical, second run a no-op).
- **terminal-publish.sh:** asserts the copy-set (change file + spec + only the
  `Accepted` ADR), the guarded no-op re-run, the CAS retry under a competing push,
  the `pub-N` teardown, and the **main-mode no-op**.
- **cleanup-feature-branch.sh:** asserts removal under `.worktrees/<slug>` and that
  the provenance guard **refuses** a path outside it.
- **Fail-closed (§3):** each script asserts its postconditions and is verified to
  **exit non-zero with a diagnostic** when a deviation is injected — a pre-removed or
  pre-archived file, a copy-set path missing on the integration branch after push, a
  worktree that survives cleanup.

## 9. Risks & mitigations

| Risk | Mitigation |
|---|---|
| A scripted bug pushes bad/partial content to the integration branch | Hermetic bare-origin tests exercise the full copy + CAS loop; normal CAS push (never force); guarded commit is a no-op on byte-match |
| The three call sites drift from the scripts over time | Single shared scripts are the divergence fix 0023 §5b asked for; SKILLs invoke, never restate |
| Idempotency / racing-sweep guarantee lost in translation | Every existing guard (reuse-existing-archive, already-terminal no-op, `-B`+prune, guarded copy, push-loop resume) is ported and pinned by a test |
| `main`-mode regression | terminal-publish.sh mode-guard no-op + test; archive-change.sh operates in whatever tree `--changes-dir` names (mode-agnostic) |
