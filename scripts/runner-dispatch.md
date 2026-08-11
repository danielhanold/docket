# runner-dispatch.sh — the runner delegation facade

## Purpose

The runner-neutral entry point of the cross-harness runner delegation framework (change 0079).
Generated shim wrappers make exactly one call to it (via `docket.sh runner-dispatch`); it
validates the request, anchors the repo root, resolves the per-runner config block, and **calls
and returns** — foreground — to the named per-runner adapter `scripts/runners/<name>.sh`, which
owns everything child-specific. Adding a future runner touches only the seams: a new adapter
script + contract in `scripts/runners/`, and a registry token in `sync-agents.sh`'s
`REGISTERED_RUNNERS` (generation-time) — it absorbed three adapters (`codex`, `cursor`,
`opencode`) without a line of change.

Change 0237 replaced the original `exec` with a call-and-return so the facade regains control
after the adapter, and hung a **run gate** on that seam: for an `implement-next` delegation only,
the facade verifies in git that the delegated run actually reached its PR.

## Usage

```
docket.sh runner-dispatch [--launch] --runner <name> --agent <agent> [--model <m>] [--effort <e>] [--worktree <path>] [--brief-file <path>] [--] [<args…>]
docket.sh runner-dispatch --observe <key> --runner <name> --agent <agent> [--worktree <path>]
```

- `--runner <name>` (required) — the runner. **Registration is the adapter file's existence**:
  `scripts/runners/<name>.sh` present ⇒ registered. Unknown ⇒ loud nonzero naming the
  registered set (abort-and-report; explicit config is never silently ignored).
- `--agent <agent>` (required) — the built-in docket agent to delegate (e.g. `status`).
- `--model` / `--effort` — forwarded to the adapter verbatim (model is ADR-0015 opaque
  passthrough end-to-end). The generated shim bakes these only from **user-configured** values;
  a value that came from docket's shipped `agents/harness-defaults.yml` is never forwarded. Since
  change 0205 a `runner:`-bearing agent with **no** user-configured model is a generation-time
  error, so the model-less case reaches this facade only on a direct hand invocation.
  **`--model inherit` is docket's own no-pin sentinel, never a vendor model ID** — this facade
  normalizes it to "no model" for **every** adapter, so a sentinel dispatch is byte-identical to
  omitting `--model` and no adapter re-decides it. Normalizing docket's own sentinel is **not
  model-ID validation**: the ADR-0015 boundary is unamended, no vendor value is inspected, and no
  allowlist of model IDs is introduced.
- `--worktree <path>` (optional) — the run anchor. Resolved through `docket_anchor_path`, so an
  absolute path passes through and a **relative** one joins to the main worktree (never to the
  caller's cwd). Absent ⇒ the main worktree, byte-identical to pre-0206 behavior. **Required for
  feature-scoped agents** — those whose `agents/docket-<name>.md` source declares
  `worktree-scope: feature` (change 0208): the `docket-build-task` contract requires a build worker
  to run inside its feature worktree, on its branch, and the rebase resolver, the integration
  repair worker and the three review rungs are feature-scoped for the same reason — two of them
  commit — so a feature-scoped delegation without it is a loud abort rather than a silent run in
  the primary checkout on the integration branch. The requirement is checked **before the verb**,
  so an `--observe` of a feature-scoped dispatch needs the flag too; the generated shim bakes the
  slot onto both its launch line and its observe line.

  The feature-scoped population is whatever `grep -l 'worktree-scope: feature' agents/` returns
  (today: the four build profiles, the rebase resolver, the integration repair worker and the three
  review rungs); every other built-in agent declares `worktree-scope: metadata`. The **declaration**
  is what both delegation gates key on — never a name list, which would be a second copy of the
  same predicate drifting against the first. `sync-agents.sh` validates it at **generation**: a
  source declaring no valid scope fails the run before any wrapper is written, which is the seam at
  which an undeclared agent is still preventable. The facade reads it at **runtime** from
  `$DOCKET_AGENTS_SRC/docket-<agent>.md`, deliberately **tolerantly** — an
  off-shape agent name, an unreadable source, or a missing key is metadata scope, so an unknown
  agent keeps the adapter's more specific unknown-agent diagnostic instead of dying at the probe.
  Both readings run the SAME extraction: `agent_worktree_scope` in
  `scripts/lib/docket-agent-scope.sh`, sourced by the generator and by the facade. It is anchored
  to the first `---…---` block, so a source that lost its declaration reads as ABSENT rather than
  as whatever its body prose happens to say about worktree scope — which is what keeps generation's
  absence refusal and the facade's tolerant fallback describing the same file.

  That tolerance stops at the **file**. The sources **directory** is the probe's precondition and is
  **loud**: a `$DOCKET_AGENTS_SRC` holding no `docket-*.md` at all — missing, misdirected, or
  unreadable — refuses **every** dispatch, metadata-scoped ones included. A missing file costs one
  agent its declaration; a missing directory resolves *every* agent to metadata scope at once, which
  disarms both delegation gates together and hands a feature-scoped worker the primary checkout on
  the integration branch in silence. With no sources the facade cannot tell the two scopes apart, so
  there is no narrower refusal available, and the loud one costs only a metadata dispatch that never
  needed the read. Keyed on the directory's **shape** rather than on `[ -d ]`, which a misdirected
  path satisfies while every scope read inside it comes back empty.
- `--brief-file <path>` (optional, change 0277) — the caller's task brief, read from a file
  instead of shell argv. The caller writes it with a quoted-delimiter heredoc, so no part of the
  brief is quoted by a model and none of it is joined or reflowed on the way to the child. The
  file must exist, be readable, and carry actual content: emptiness is measured over the file's
  contents, not its byte count, so a whitespace-only brief is refused as loudly as a zero-byte one. **Mutually exclusive with trailing `--`
  arguments**: passing both is a loud refusal, because preferring either channel silently drops
  or duplicates the child's entire input and concatenating them invents an ordering. Under
  `--launch` the brief is spooled into the per-dispatch directory as `brief` and the adapter is
  handed that durable copy; on the legacy foreground verb the caller's file is passed through.
- **`build-*` agents require a payload.** A `build-*` dispatch carrying neither a brief file nor
  trailing arguments **that carry content** (`-- ""` is arguments-present and payload-empty, and
  is refused too) is refused at the same pre-verb validation point as the `--worktree` gate —
  so the rule holds for `--launch` and the legacy verb alike. A build worker with no task does not
  error; it improvises from whatever is in the worktree and the dispatch still looks successful.
  `--observe` is exempt, since it starts no child and reads a result the matching `--launch`
  already recorded. Non-`build-*` agents (status, adr, …) legitimately dispatch payload-free and
  are unaffected.
- `--launch` (optional, change 0271) — the **detached** verb. Same request, same validation, same
  gates, same `runners.<name>:` resolution; the difference is only in how the adapter is started
  and when the call returns. Instead of blocking for the child's whole run, the facade starts the
  adapter in its **own process group**, redirects every stream into a durable per-dispatch
  directory, prints a **dispatch key** on stdout, and exits `0`. Absent, the call is the legacy
  synchronous call-and-return described below, byte-identical to pre-0271 behavior.
- `--observe <key>` (change 0271) — the other half of the launch verb: **one short, idempotent
  look** at what the dispatch named by `<key>` has left behind, returning a synthesized exit code.
  It never waits for the child — an observation that blocked for the run would re-introduce the
  single-foreground-call ceiling the verb exists to remove. The key must be a mint (the same
  `[A-Za-z0-9._-]` shape `--runner` is held to), and an unknown one is a **usage error**, never a
  verdict.
- `-- <args…>` — forwarded to the adapter as caller task context.
- **A value-taking flag in final position is a loud refusal, never a hang.** Each of `--runner`,
  `--agent`, `--model`, `--effort`, `--worktree` guards `[ $# -ge 2 ]` before consuming its value
  and dies with `<flag> requires a value`. `--observe` and `--brief-file` reach the same outcome
  through a shift-then-conditional-shift arm, because `--observe` must keep its own "requires a
  dispatch key" refusal reachable. Unguarded, the arm's `shift 2` would *fail* rather than truncate
  at `$# = 1` — the loop has no trailing shift and the facade runs with no `set -e`, so the parse
  loop would spin forever instead of refusing.

Mock seams: `RUNNERS_DIR` (adapter directory), `DOCKET_AGENTS_SRC` (the built-in agent sources the
facade reads `worktree-scope:` from, default `$SELF_DIR/../agents`; `DOCKET_`-namespaced per
ADR-0014 because it decides whether the delegation gates are armed, and the bare name is one an
unrelated tool could hold), `GIT` (the git binary — read by `lib/docket-root.sh` and, since
change 0271, by the launch verb's dispatch-time SHA read), and —
for the run gate (change 0237) — `VERIFY_RUN` (the disposition reader, default
`scripts/verify-run.sh`) and `DOCKET_FACADE` (the facade used for the metadata re-syncs on both
sides of the handoff, default `scripts/docket.sh`).

## Behavior

1. **Validate** — both required flags present; adapter file exists.
2. **Anchor** — `DOCKET_REPO_ROOT` = `docket_anchor_path "${worktree:-.}"`
   (`scripts/lib/docket-root.sh`, ADR-0034). With no `--worktree` that is the repo's primary
   checkout, cwd-independent — correct even when invoked from `.docket/` or a `.worktrees/<slug>`
   feature worktree. With `--worktree` it is the named tree, and a relative value joins to the
   main worktree so it too resolves identically from any cwd. Not in a repo ⇒ abort. Four loud
   gates follow, all before the config read so an anchor failure never depends on config parsing:
   `--worktree` is **required** for a **feature-scoped** agent (one whose `agents/docket-<name>.md`
   source declares `worktree-scope: feature`); the resolved anchor must be a **directory**; it must
   be a **worktree top-level of this repository** — membership, not containment, so an ordinary
   subdirectory of the main worktree is refused; and for a feature-scoped agent it must not be the
   **main worktree** itself — the primary checkout, which is where the integration branch is
   normally sitting.

   That last gate measures **path identity only** (anchor vs. repo root), and its diagnostic says
   so rather than naming a branch. The residual is deliberate and worth stating: a **linked**
   worktree that happens to be checked out on the integration branch is **not** caught. A branch
   predicate is not available here — `rebase-resolver` is dispatched mid-rebase, where HEAD is
   detached and there is no branch to compare — so the gate proves the fact it can prove, and the
   branch stays the hazard the main worktree normally carries rather than a fact the facade
   establishes.

   Membership is read out of a single `git worktree list --porcelain` capture taken from the
   anchor, which yields both halves: same-repo is the **first** `worktree` line equalling the repo
   root (git lists the main worktree first), and membership is an exact `worktree <anchor>` line.
   The first-line comparison is deliberately not an anywhere-in-list match — `worktree list`
   retains stale records for deleted-and-recreated directories, so a **foreign** repo's list can
   carry a `worktree <this repo's root>` line for a path that is no longer its worktree, and an
   anywhere-match would hand a delegated run a tree docket does not own. A non-repo path yields
   empty output and fails that same first-line comparison, so the not-a-repo case still falls out
   of this one check. The anchor is `pwd -P`-normalized first, which is load-bearing on macOS:
   `/tmp` is a symlink to `/private/tmp` while git prints physical paths, so without it the exact
   line match would reject valid worktrees.

   The main-worktree gate is **exempt on the `--observe` anchor fallback**: an observation whose
   worktree has since been removed deliberately re-anchors at the main worktree so the durable
   record still reports (see *Observation*), and refusing there would turn a reported
   `task-unverifiable worktree-removed` into a failed observation.
3. **Resolve `runners.<name>:`** — per **key**, first layer that has the key wins, across the
   **main worktree's** `.docket.local.yml` > the **main worktree's** `.docket.yml` >
   `${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml`. The config tree is the main worktree —
   the `docket_main_worktree()` result the facade binds before any argument-dependent anchoring —
   and it is **independent of `--worktree`**: the machine-local layer is gitignored, so a feature
   worktree carries no copy of it, and an anchor-relative read would silently drop every
   machine-local runner grant on exactly the feature-scoped dispatches that *require* `--worktree`.
   Each `key: value` scalar is exported
   as `DOCKET_RUNNER_CFG_<KEY>` (uppercased; `.`/`-` → `_`). The facade knows no runner's key
   names — each adapter defines and defaults its own (see its contract). `runners:` is **not**
   coordination-fenced: it is a machine preference in the same class as `model`/`effort`
   (it writes no shared state), so all three layers are honored.

   A value is read as the **rest of the line** after `<key>:`, with a whitespace-preceded `#`
   comment stripped and surrounding whitespace trimmed — so paths and URLs (`/Users/x/p`,
   `https://host/v1`) survive intact. A value that parses to nothing is **skipped, not fatal**:
   this runs on a live dispatch path, so a cosmetic config typo must not convert into a failed
   dispatch. The key is still claimed for its layer before its value is parsed, so a malformed
   high-precedence value masks the same key in lower layers (precedence is per-key, not
   per-value).
4. **Handoff** — `"$DOCKET_BASH_PATH" scripts/runners/<name>.sh --agent <agent> [--model m]
   [--effort e] -- <args…>`, foreground, **call-and-return** (change 0237 — no longer `exec`).
   The adapter's stdout/stderr pass through and its exit code is propagated **verbatim** on every
   path where the run gate takes no action.
   `--model` is omitted whenever no model resolved — including when the caller passed the
   `inherit` sentinel, which is normalized to empty right after argument parsing, above every
   adapter.
5. **Run gate (change 0237), synchronous path** — engages **only** for `--agent implement-next`,
   and only on a call with no `--launch`; the detached path's half of the same gate is split across
   `--launch` (the before-snapshot) and `--observe` (the verdict), described under *Observation*.
   Both halves share one reader — `verify-run.sh` — and one attribution; only the re-dispatch
   policy differs. The facade re-syncs
   the metadata worktree and records the set of `in-progress` change ids
   (`verify-run --in-progress-ids`), then stamps the clock; after the handoff it re-syncs **again**
   and re-reads the set with `verify-run --in-progress-ids --with-claimed-at`. The re-sync is
   **symmetric** by necessity: both reads must be of fresh origin state, or a change that is
   `in-progress` on `origin/docket` but not yet in the local `.docket` worktree is absent from the
   before-set, present in the after-set, and attributed to a run that never touched it.

   **Attribution.** A candidate must clear three filters, because a set diff identifies a change of
   state and not a claimant: (1) not in the before-set; (2) a `claimed_at` that parses — an absent
   or unreadable stamp is no positive evidence of ownership; (3) `claimed_at` at or after the
   dispatch stamp — a claim made before this run started cannot be ours however it became visible,
   which is what excludes an **abandoned** in-progress change from an earlier session (the case
   `board-checks`' `aborted-run` leg exists for) even on the path where the pre-handoff re-sync
   itself failed. A timestamp cannot separate our claim from one a **concurrent** loop made during
   our run — `claimed_at` is re-stamped at every phase boundary, so a live foreign run looks fresh
   too — so ambiguity is resolved by counting instead: an `implement-next` run claims **at most
   one** change, so two or more surviving candidates means none can be attributed and the gate
   **stands down with a warning**. Each surviving id (at most one) is checked with
   `verify-run <id>`:

   - `run-complete` / `run-halted` / `run-unclaimed` → nothing; exit the adapter's code.
   - `run-incomplete` → **one** bounded re-dispatch of the same adapter, with the change id and the
     unmet conjuncts as task context. The **second** verdict then decides: still `run-incomplete` ⇒
     abort loudly with exit `1`, naming the change and the still-unmet conjuncts; `run-halted` ⇒
     exit `3`, the same terminal halt as a first-verdict halt (a re-dispatched run stopping
     deliberately is not a success); `run-complete` / `run-unclaimed` ⇒ exit **`0`**; anything
     else (empty, unparseable) ⇒ the adapter's code, since the gate acts only on a positive
     finding.

   `run-halted` never re-dispatches — a halt means a human is needed. A `build-*` delegation leaves
   its change `in-progress` by design, which is why the agent gate is load-bearing rather than an
   optimization; a build task's terminal state is a **commit on its feature branch**, not a change
   status, so its disposition lives on the `--observe` seam (see *Liveness vs correctness* below)
   rather than being bolted onto this synchronous gate. An unrecognised agent is a no-op, never a guess. A snapshot that cannot be read, a
   failed clock read, or an ambiguous claim set all **disable the gate with a warning** — none of
   them ever converts a healthy dispatch into a failure. A failed metadata re-sync (either side) is
   likewise best-effort: it warns and degrades the gate's freshness rather than failing a dispatch
   that may well have succeeded.

   When the dispatch carried its payload as a **brief file**, the re-dispatch does not append the
   retry context as an extra argument — that would present both payload channels at once, the shape
   the adapters refuse. It composes a **combined brief** instead — the original brief's content and
   line structure unchanged (its final line terminated first if it was not already, so the
   separating blank line is unconditional and the retry context can never be glued onto the brief's
   last line), then a blank line, then the retry context — in a temporary file, re-dispatches with
   `--brief-file` pointing at it, and removes it when the call returns. With no brief file the re-dispatch keeps
   its original trailing-argv shape.

   The re-dispatched run's own exit code is not propagated: the gate's verdict is read from git,
   not from the retry's status. By the same rule, once a re-dispatch has actually run, a **positive**
   second verdict of `run-complete` / `run-unclaimed` supersedes the **first** adapter's code and the
   facade exits `0` — that first code describes the attempt the gate just superseded, and a non-zero
   one is a common accompaniment to a run that stopped short, so propagating it would report a
   run the gate has just proved complete as a failure. The override is scoped to the re-dispatch
   path only: where the gate took **no** action, the adapter's code is still propagated verbatim,
   including when the first verdict alone was `run-complete` / `run-unclaimed`.

**Signals.** The facade installs no traps. A **group**-directed signal (a terminal's Ctrl-C) is
unchanged by the loss of `exec` — the adapter is in the same process group and receives it
directly. A **pid**-directed signal to the facade now behaves differently, because the facade is a
separate process: `INT` is deferred while it waits on its child and then discarded, and `TERM`
kills the facade and orphans the adapter. Nothing in docket signals the facade by pid; forwarding
traps are a deliberate deferral, not an oversight.

### Launch (change 0271)

`--launch` runs steps 1–3 above unchanged — same validation, same anchor gates (including the
feature-scoped `--worktree` requirement), same `runners.<name>:` resolution and `DOCKET_RUNNER_CFG_*`
exports — and then replaces step 4's blocking handoff. It reaches no *verdict*: the gate reads what
a *finished* run left in git, and at launch time nothing has run yet. For an `implement-next`
delegation it does record the gate's **attribution inputs**, which are only knowable before the
child runs — see *The before-snapshot* below.

**Where the result lives.** `<git-common-dir>/docket/dispatch/<key>/`, the same family as
`disable-worktree-hooks.sh`'s docket-owned directory inside the common git dir. Under `.git/`, so
it is never tracked, never leaks into a commit, and needs no `.gitignore` entry. Deliberately
**not** in the feature worktree: a dispatch result must outlive `git worktree remove`, since the
whole point of the verb is that the child's work can be inspected after the run was declared over.
The facade's own reader honors that: an `--observe` whose `--worktree` anchor **no longer exists**
resolves the root from the main worktree instead of refusing, because the root is repo-wide rather
than worktree-scoped (see *Observation*). Storing the result durably and then refusing to read it
would make the claim true for a human with a shell and false for everything else.

**Retention.** Every launch mints a directory holding a whole agent run's `stdout.log` and
`stderr.log`, so under the autonomous drainer `.git` would grow without bound. The top of `--launch`
therefore prunes, and the rule is deliberately conservative — a wrong prune destroys exactly the
evidence this verb exists to preserve:

- A dispatch with **no terminal file** — no `done` sentinel from the launcher wrapper and no
  `killed` marker from an observer's give-up — is **never** considered, whatever its age. A live
  child, a child whose observer has not yet given up, and a child nothing ever observed are all
  untouchable. Liveness is never inferred from a clock here; only the two terminal writes make a
  dispatch eligible at all.
- An eligible dispatch is removed only once its **terminal file** is older than **7 days**
  (`DOCKET_DISPATCH_RETENTION_DAYS`). The age is measured on the file written *last*, so the window
  starts at the end of the run rather than at its launch: a caller has the full window to observe a
  finished dispatch, and re-observation stays idempotent throughout it.

The accepted residual, stated rather than hidden: a dispatch that **never** goes terminal is
retained forever. That is the conservative direction, and such a dispatch stays visible under
`.git/docket/dispatch/` for a human to remove. Pruning is best-effort — one that cannot run never
fails the dispatch it precedes.

**The key.** `<agent>-<UTC timestamp>-<pid>`, minted per dispatch and printed on stdout as the
call's only output. Keyed on agent plus a mint rather than on change id or worktree, so two
concurrent dispatches **for the same change** never collide. A mint that would reuse an existing
directory refuses rather than clobbers — a silent reuse would overwrite a live dispatch's sentinel.

**Detachment, and why job control.** The child is started as a background job under `set -m`, which
makes it a **process-group leader**. That is what lets it survive the harness's teardown of the
initiating call's *process group*, not merely its parent's exit. Measured on macOS 2026-08-09 with
one variable changed between two arms of a single run: a launcher started two children, one under
`set -m` and one not, then the launcher's whole group received `TERM`; the `set -m` child survived
and the other did not. `setsid` is **absent on macOS** and docket takes no `perl` dependency, which
is why the mechanism is job control rather than a new session. Every stream is redirected into the
dispatch dir (`stdout.log`, `stderr.log`) and stdin is closed, so nothing remains attached to the
initiating call.

**The race precondition, and the fail-closed refusal.** A new process group is not established
instantaneously, and a key returned before it is established describes a child that a teardown
would still reap. The facade therefore polls briefly for the child's actual PGID and refuses rather
than reports: a child that never appears at all aborts, and a child still sharing the facade's
group is `TERM`-ed and the launch aborts naming the key. A child that has already **finished** is
established too — the sentinel proves it — and is not a refusal.

**The launch record** — `<dir>/launch`, flat `KEY=value`: `pgid`, `child_pid`, `child_lstart`,
`started_at`, `agent`, `runner`, `worktree`, `since_sha`, `branch`, `dispatch_epoch`. `pgid` is the
group an observer must signal to reach the whole detached tree, and `child_pid` plus `child_lstart`
are what let a later observation prove that group is still *that* tree: `child_lstart` is the OS's
own start time for the child (`ps -o lstart=`), recorded as an **opaque token** to be compared as an
exact string rather than parsed, since its rendering is platform- and locale-dependent and both
sides of the comparison come from the same `ps`. It is empty only for a child that had already
finished by the time the group was measured. `since_sha` is the repo's `HEAD` captured
**before** the child could commit anything — the direct analogue of the run gate's
`DISPATCH_EPOCH`, so a commit landing in the gap is excluded either way; empty on a repo with no
commits — and `verify-run --build` refuses an empty `--since`, so the observe leg gets no verdict
at all and reports the run **failed** rather than guessing it succeeded. `branch` is the
anchor's branch captured at the same instant and for the same reason: whether the child **ended
where it was sent** is only answerable against a value recorded before it could move `HEAD`. A
**detached** anchor records nothing rather than the literal `HEAD` that `--abbrev-ref` prints —
`HEAD` is not a branch name, and recording it would let the conjunct hold for any other detached
state.

**The before-snapshot** — `<dir>/gate-before` (one change id per line) plus the record's
`dispatch_epoch`, written for an **`implement-next`** launch only. They are the run gate's
attribution inputs, and under detachment the two halves of its set diff land in two different
processes, so the "before" half has to be durable. The discipline is the synchronous gate's,
unchanged: the metadata worktree is re-synced **first** so the before-read is of fresh origin
state, and the clock is stamped **after** that read, so a claim landing in the gap is either
already in the before-set or stamped before the window and is excluded either way. `--observe`
re-syncs again, which is what keeps the pair symmetric — an asymmetric pair attributes an
**abandoned** claim from an earlier session to this run.

The pair **is** the arming signal: the snapshot file is written even when empty (nothing claimed at
the handoff is a real answer), so with either half missing or a `dispatch_epoch` that does not
parse, `--observe` falls back to the sentinel-only disposition rather than guessing at an
attribution. A snapshot or clock read that fails at launch warns, leaves the gate unarmed, and
leaves the dispatch itself untouched — the same tolerant posture every other gate read takes. A
non-`implement-next` launch records an empty `dispatch_epoch` and no snapshot file, so it is never
armed.

**The brief** — `<dir>/brief`, the caller's task brief, spooled here at launch when `--brief-file`
was passed (change 0277). Written atomically (`brief.partial` + `mv -f`) and handed to the adapter
in place of the caller's own path, so a detached run never depends on a caller temp file outliving
the call that started it. Absent when the dispatch carried its payload as trailing argv. It is part
of the dispatch's audit record and is pruned with the rest of the directory — no separate
lifecycle. A spool that cannot be written **aborts the launch** rather than degrading to a
task-less dispatch: the brief is the child's only input — and the abort **removes the dispatch
directory it just minted**, because a dispatch that never goes terminal is retained forever by the
retention rule, so an unwritable dispatch area would otherwise leak one dir per attempt.

**The sentinel** — `<dir>/done`, flat `KEY=value`: `exit_code`, `started_at`, `finished_at`, `pid`,
`dispatch_key`. `pid` is the **launcher subshell's own** pid — the same process the launch record
names as `child_pid`, not the facade's. **The wrapper writes it, never the agent**: "done" must not be a claim by the party
being judged. It is written as the wrapper's last act, so its absence *is* "still running". The
write is atomic — a temp file **beside** its destination (the one licensed exception to templating
temp files into `TMPDIR`, because the rename must be same-filesystem) then `mv -f` — so a reader
never sees a half-written sentinel, and a failing adapter records its real code just as a clean one
does.

### Observation (change 0271)

`--observe <key>` is the only way to learn how a launched dispatch ended. It performs the same
validation and anchoring as every other call — with one scoped relaxation, for durability: when the
`--worktree` anchor **does not exist**, the observation says so on stderr and resolves the
repo-wide dispatch root from the **main worktree** rather than aborting on the anchor gate, so a
result still reports after `git worktree remove`. An anchor that *exists* but is not this
repository's worktree is refused exactly as before, and `--launch` keeps both gates in full. What
does not survive the removal is the *tree*: a `build-*` observation on that path reports
`task-unverifiable worktree-removed` instead of reading a different worktree's git state. It then
makes **one pass** over the dispatch dir:

1. a `done` sentinel ⇒ the child is finished. A sentinel that does not parse ⇒ **result
   unavailable** (`1`), because a malformed sentinel means the *launcher* did not finish cleanly
   and an exit code read out of garbage would be a fabricated verdict. Otherwise the agent's
   git-read disposition decides where it has one — for `implement-next` that is the run gate, whose
   verdict can synthesize `3`, `1` or `0` over either exit-code class (see *Liveness vs
   correctness*) — and failing that the sentinel alone does: `exit_code=0` ⇒ **complete** (`0`),
   *unless* a `build-*` git read disagrees; a non-zero code ⇒ **failed** (`1`), naming the code and
   the `stderr.log` to read;
2. a `killed` marker ⇒ terminal, and it re-reports identically forever, which is what makes the
   observation idempotent across a caller's retry loop. A give-up marker is **result unavailable**
   (`1`); a `cause=child-vanished` marker replays the verdict and the code it recorded (`0`, `1` or
   `3` — see *Liveness vs correctness*);
3. neither record, and the recorded process group is **no longer provably the launched child's**
   ⇒ terminal on this observation, `cause=child-vanished`, with git deciding the code (change
   0284);
4. otherwise ⇒ **still running** (`4`), unless the observation budget is spent — or has been
   **unenforceable** on three consecutive passes, which is terminal for the same reason (below).

**The order of the first two reads is load-bearing, and the sentinel wins.** The wrapper subshell is
**untrapped** and is the only writer of `done`, so a group-directed `TERM`/`KILL` reaches it and it
can never write a sentinel afterwards. A `done` sitting beside a `killed` marker therefore *proves*
the child completed **before** the signal — the give-up merely raced a run that had already
finished — so the completed disposition is the true one. Read the other way round, a sentinel that
landed anywhere between the give-up path's "no sentinel" read and its marker write was masked
**forever**: a completed run reported as result unavailable, sending a human to hunt for work that
is in fact committed.

**The relay — where the child's output surfaces.** `--launch` redirects the adapter's stdout into
`<dir>/stdout.log`, so `--observe` is the **only** channel by which a delegated agent's result
reaches its caller. On every path where the `done` sentinel exists — complete (`0`), a non-zero
child code, a git disagreement, a malformed sentinel — the observation writes that captured stdout
to **its own stdout**, byte-for-byte: never summarized, prefixed, or reformatted, because the
adapter contracts call the child's stdout the relay and a caller parses it. **Every diagnostic this
verb prints goes to stderr**, so the two streams never interleave and a caller can take stdout as
the child's words alone. A **still-running** (`4`) observation relays **nothing**: the shim observes
repeatedly, and a partial relay per pass would hand the caller the same prefix over and over. The
budget-kill and own-group-refusal paths relay nothing either — there is no finished run to relay,
and in the latter case the child is still writing.

**The budget.** `delegation_observation_budget` minutes (the environment wins, so a caller can hand
one down; otherwise it is resolved from config on this branch alone — a verb that needs no config
must never be failed by config). Elapsed time is measured from the launch record's `started_at`
against `verify-run.sh --iso-to-epoch`, which keeps the portable ISO→epoch parse in the one script
that already owns it. `0` is legal and buys exactly **one** observation, because the budget is
compared only *after* the sentinel read. Anything that is not positive evidence of exhaustion —
an unreadable clock, an unreadable `started_at`, a budget value that is not an integer — reports
**still running** and enforces nothing this pass, rather than killing a healthy child on a guess.

**When the budget cannot be enforced at all.** "Do not enforce" must not mean "never terminate".
`4` is the caller's loop condition, so a state that returns it unconditionally is a state the loop
can never leave — and an unreadable clock, an unreadable `started_at`, or a launch record that is
missing or unparseable (an empty `started_at` alone puts *every* later observation there) are each
such a state. So consecutive **budget-unenforceable** observations are counted in the dispatch dir,
and the **3rd** converts into the same terminal give-up a spent budget takes: the identity-checked
group kill, the `killed` marker, and **result unavailable** (`1`) with a diagnostic naming *why*
the budget could not be enforced. `3` is chosen against which members of that family are transient:
only a clock read failing under momentary load is, and a launch record that cannot be parsed never
repairs itself, so a larger N is pure grace on a state that is already permanent. The counter
**resets on any enforceable pass**, so one bad read in an otherwise healthy run never accumulates
toward termination. An unwritable counter is itself terminal — a bound that cannot be persisted
bounds nothing.

**Kill on giving up.** When the budget is exhausted the facade signals the **process group**
recorded at launch (`TERM`, then `KILL` after a short wait), never the launcher's pid alone: a
single-pid kill reaps the launcher shell and **orphans the adapter and its children**, which is the
half-dead state this change exists to eliminate (it honors change 0231 — no presumed-dead worker
wakes to race its replacement). It then records a `killed` marker and reports result unavailable.
Partial work is left in the worktree for a human. One state is refused rather than signalled: a
launch record naming the **observation's own process group**. `--launch` fails closed when the
child did not separate, so a record it wrote can only name a foreign group — but a record can be
wrong anyway, and a group-directed signal aimed at our own group takes down the harness that ran
it. That case reports result unavailable, writes no `killed` marker, and names the dispatch dir —
so a sentinel that arrives later is still read first by the next observation, which is what keeps
that refusal from masking a finished run.

**The sentinel is re-read twice inside the give-up, on both sides of the kill.** The path is
entered off a "no sentinel" read taken a `date`, a *subprocess* (`verify-run --iso-to-epoch`) and
two `ps` calls earlier — tens of milliseconds in which the child can finish. So the sentinel is
read **immediately before the signal**, where nothing but the `kill` separates the test from the
act, and **again after the kill and before the `killed` marker is written**; a sentinel found at
either point takes the completed disposition, no marker is recorded, and nothing is signalled. The
second read is what covers the no-signal path (where the first never runs) and the instant between
the first read and the signal landing. The correctness argument is the ordering's: the untrapped
wrapper is the only writer of `done` and the group signal reaches it, so a sentinel visible *after*
the kill was necessarily written *before* it, by a child that completed.

**The identity check — a pgid is a reusable name.** The own-group refusal defends the harness; it
defends nobody else. This path is reached **only** when no sentinel exists, which includes a child
killed externally or dead without a sentinel an hour earlier — by which time the OS may have handed
that group id to an unrelated tree. So the group is signalled only when the launch record proves it
is still the launched one, on two conjuncts: the recorded `child_pid` must **still lead** the
recorded `pgid` (the child is its group's leader by construction), and that pid's start time must
still equal the recorded `child_lstart` — the first conjunct alone is satisfied by a **recycled pid**
that happens to lead a group of the same id, which is an ordinary background job and not an exotic
state. A launch record carrying **no** token fails the conjunct closed (change 0284, adopting
`gate-run.sh`'s posture through the shared predicate `docket_group_alive_and_ours` in
`scripts/lib/docket-liveness.sh`). That is behaviour-preserving on every reachable input: `--launch`
records an empty `child_lstart` only when `ps` saw no process — i.e. the child had already
finished — in which case the wrapper writes `done` and the sentinel read disposes before either leg
is reached.

Failing either conjunct is not an error but the ordinary *that group is already gone* outcome: **no
signal is sent**, the terminal `killed` marker is still written with `reason=group-already-gone`
(so the dispatch stays terminal and a later observation re-reports it as unavailable, saying
plainly that nothing was signalled rather than claiming a kill), and the verdict is **result
unavailable** either way — the run outran its budget with no sentinel, so there is no result to
report whether or not a signal went out. The **accepted residual**: a group whose leader died while
processes it spawned keep running is not signalled, so those orphans outlive the budget; the same
holds when `ps` cannot be read. Killing them would mean signalling a name that cannot be proven
still theirs, and an unrelated process group dying is both the worse failure and the unrecoverable
one, while an orphan is visible and reapable. Since change 0284 this same residual also shapes a
**verdict** rather than only a kill decision: the liveness leg reaches it one lifecycle phase
earlier, on an observation that has not yet spent its budget.

**Liveness vs correctness (change 0284).** Three sources, in a fixed order: **the terminal record
first, process liveness second, git last.** A `done` sentinel or a `killed` marker outranks any
probe of the group the child used to lead — the wrapper is the only writer of the sentinel, so a
record that exists describes a child that reached the end. Only when neither exists does the facade
probe **liveness**, through the identity-checked predicate `docket_group_alive_and_ours` in
`scripts/lib/docket-liveness.sh`, shared with `gate-run.sh`: the recorded group must still exist
*and* the process leading it must still have started at the instant `--launch` recorded. Fail-closed
on every leg, because a false *dead* costs one wasted observation while a false *alive* costs the
caller its entire budget.

The half of the old rule that still holds, unchanged: **correctness never comes from liveness, and
liveness never comes from git.** A child that is running says nothing about whether its work is
sound, and git state says nothing about whether a process is alive. What changed is only that the
*sentinel* is no longer the sole liveness source — before change 0284 the predicate was "no sentinel
⇒ still running", so a child killed externally, crashed, or whose host was suspended read *still
running* for the whole 60-minute budget.

**The probe's own window is closed by a re-read.** The two record reads and the probe itself span a
`ps` call and a `kill -0`, so a child has every chance to finish *inside* that window — and without
the re-read a run that PASSED is disposed as dead. A dead verdict therefore re-reads the sentinel
immediately before disposing, and a `done` found there takes the completed disposition — the same
construction, and the same soundness argument, as the give-up path's pair of re-reads above.

**`cause=child-vanished`.** A child found dead with no sentinel is terminal on that observation. The
verdict is recorded in the **existing** `killed` marker with `cause=child-vanished` and
`reason=group-already-gone` — no second terminal file, because the `cause`/`reason` split already
carries the two axes (*why the facade gave up*, and *whether anything was signalled*). Nothing is
signalled: the probe has just established the group is not provably ours, which is the precondition
the give-up path already refuses to signal under.

**A dead child is not automatically "no result".** Git decides, on the same disagreement rule as the
sentinel path: a delegated run can commit its work, push its branch and open its PR and *then* be
killed before the wrapper's `mv -f` lands, and reporting *unavailable* over evidence sitting in git
sends a human hunting for work that is already committed. An `implement-next` dispatch takes the run
gate's verdict, read through the same single owner of the attribution ladder the sentinel path uses
(`implement_next_verdict`, split out of `observe_implement_next` so the read and the disposition are
separable); a `build-*` dispatch reads `verify-run --build`, and inherits both of that path's honest
non-verdicts — `task-unverifiable worktree-removed` when the anchor worktree is gone, and
`task-unverifiable launch-branch-missing` when the launch record names no branch. Every other agent
is *unavailable* with no git verdict read and none claimed. **No message on this path asserts an
exit code**: the child said nothing at all, and a code that was never read is a fabricated verdict.

**The marker carries the verdict AND the code, which is what makes this leg idempotent rather than
merely terminal.** Alongside the fields above the `child-vanished` marker records `git_verdict=`
(the verdict as read at the transition, newlines folded to spaces so the key=value record stays
line-oriented) and `disposition=` (the exit code that verdict decided). A later observation short-
circuits at the marker read and **replays both** — the same wording, the same code — asking git
**nothing**: a second `verify-run` call would be a differently-timed answer to a question already
answered, and could report `0` on one pass and `1` on the next in the one field a caller branches
on. It is the only `cause` that replays a code rather than falling through to the give-up path's
unconditional `1`; it also cannot borrow the give-up wording, which would tell a reader the budget
ran out on a dispatch that was terminal seconds after launch. An **absent or non-numeric**
`disposition` — an older marker, or one a failed write left short — reads as **`1`**: unavailable is
the fail-closed reading, never a success synthesized out of a missing field.

**A halt found this way exits `3`, not `1`, and that is a deliberate deviation from the design
spec's synthesized-exit table.** `3` is the code the sentinel path's halt already returns, the code
change 0271's table pins for a halt reached under detachment, and the code the run gate keys on for
*never re-dispatch a halt*. What a vanished child changes is how the facade **learned** the run
stopped; it never changes what the run's state **is**. Collapsing a stop-for-a-human into a generic
failure is exactly the prose-level failure change 0237 exists to prevent, so the halt keeps its own
code on this leg as on every other. The mapping is shape-keyed on the verdict's leading token
(`vanished_code`), never on an enumerated list of full verdict strings: `task-committed` /
`run-complete` / `run-unclaimed` ⇒ `0`, `run-halted` ⇒ `3`, everything else including an empty
verdict ⇒ `1`.

**The orphan residual now shapes a verdict, not only a kill decision.** A supervisor that died while
processes it spawned keep running is reported dead and those orphans are **not** reaped — the same
accepted residual the give-up path documents above, for the same reason. The diagnostic names the
dispatch dir so a human can find them.

**The disagreement rule (change 0271).** A sentinel claiming success (`exit_code=0`) with no
matching git evidence is a **failure** — correctness wins over liveness. The delegated run is the
party being judged, so its own exit code can never be the last word about the work it left behind:
change 0258 stranded +64 uncommitted lines and exited `0` at the adapter.

The observe seam therefore carries a git-read disposition for two agent families, and only those:

- **`implement-next`** — change 0237's run gate, carried onto this seam. It is **not** enough that
  the synchronous gate below still exists: the generated shim always launches, so that fence is
  unreachable for every *delegated* run, and without a disposition here a run that halted or that
  stopped before its PR exits `0` at the adapter and observes as `complete (child exited 0)` —
  precisely the prose-level failure change 0237 was built to eliminate. Detail below.
- **`build-*`** — new. On `exit_code=0` the facade reads `verify-run.sh --build --worktree <anchor>
  --branch <the launch record's branch> --since <the launch record's since_sha>`. Both inputs come
  from the **launch record**, never from the anchor now: a branch re-read at observation time
  compares `HEAD` to itself. `task-committed` ⇒ **complete** (`0`), with the verdict echoed on the
  diagnostic line. Every other answer — `task-incomplete`, `task-unverifiable`, or no verdict at
  all — ⇒ **failed** (`1`), naming the git verdict and the worktree the work was left in. A check
  that could not run is not evidence of success. A launch record carrying **no** branch is one of
  those answers: the facade reports `task-unverifiable launch-branch-missing` rather than falling
  back to the observation-time branch, because that fallback is the vacuity itself.
- every other agent (`status`, `adr`, `review-*`, `finalize-change`, `auto-groom`, an unrecognised
  name) keeps the **sentinel-only** disposition. No git verdict is read and none is claimed.

**The `implement-next` disposition, in full.** On the well-formed-sentinel path — the child is
finished and its exit code parses — an armed dispatch (see *The before-snapshot*) re-syncs the
metadata worktree, re-reads with `verify-run --in-progress-ids --with-claimed-at`, and applies the
**same three attribution filters** the synchronous gate applies: not in the before-set; a
`claimed_at` that parses; `claimed_at` at or after `dispatch_epoch`. Two or more surviving
candidates is the **same stand-down** — an `implement-next` run claims at most one change, so none
can be attributed — and no candidate at all is a no-op (drained, a lost claim race, or a run that
finished and left its change `implemented`, which is not `in-progress` and so never appears in the
after-set). The single surviving id is checked with `verify-run <id>`:

| Verdict | Exit |
|---|---|
| `run-halted` | **`3`** — stop and surface; the run stopped deliberately and needs a human |
| `run-complete` / `run-unclaimed` | **`0`** |
| `run-incomplete` | **`1`** — the run did not reach its PR |
| empty, unparseable, or the gate unarmed / stood down | the **sentinel-only** disposition (`0` on `exit_code=0`, `1` otherwise) |

`4` never arises here: this leg runs only where the sentinel already exists, so the run is over.

The verdict outranks the child's own exit code **in both directions**, which is why the leg runs
before the exit-code split rather than inside its zero branch: a halt is terminal whatever the
adapter returned, and a positively green verdict describes a run that reached its PR despite a
noisy adapter. It is the same rule the disagreement rule states — correctness outranks the
self-report of the party being judged — and it is why an unparseable verdict falls back rather than
guessing.

**`implement-next` is observe-only at this seam — no auto re-dispatch. This is a decision, not an
omission.** The synchronous gate's one bounded re-dispatch is deliberately **not** recreated here.
Re-launching a detached child out of an *observation* is a different lifecycle: an observation is a
short, idempotent read that a shim makes repeatedly, so a re-dispatch on this path would race the
very run being observed and could mint a fresh detached child on every pass. `run-incomplete`
therefore reports `1` and the caller decides; the change stays `in-progress` with its claim intact,
and `board-checks`' `aborted-run` leg remains the standing backstop. The synchronous gate keeps its
re-dispatch unchanged — a direct hand invocation without `--launch` still runs it.

**`build-*` is observe-only — never re-dispatched.** A build task may have left partial commits, and
re-running an adapter on top of them is `docket-build`'s "never escalate onto a stray commit"
hazard. The facade reports and stops; the partial work stays in the worktree for a human or for the
build role's own escalation to decide about.

**`task-committed` proves clean completion, not semantic success.** Its three conjuncts say the task
ran to its commit and stranded nothing. They do not certify that the commit implements the plan task
correctly — that judgment stays with `docket-build`'s suite gate and the review role. All three
conjuncts **bind** at this seam: `tree` and `tip` catch change 0258's stranded work, and `branch` —
compared against the value the launch record captured, not one read back afterwards — catches a
child that ended on another branch or on a detached `HEAD`.

## Delegation execution posture

A delegated run **may outlive the call that launched it**. That is the contract, not an
implementation detail: the parent harness's foreground-call ceiling does not bound a delegated
agent run, and nothing on this path may re-introduce a bound that it does.

The six required capabilities are the same ones the build gate needs, and they are defined once —
in `skills/docket-build/references/gate-execution.md`. **Read them there; this contract does not
restate them.** What is specific here is the division of labour:

- **The shim launches and observes, paced and bounded.** It makes one `--launch` call and then
  short `--observe` calls **spaced by a blocking wait** (about a minute), not back to back: an
  unpaced loop spends the parent's context re-reading the same "still running" answer. It never
  blocks for the child's duration and never yields between observations (ADR-0024 unamended: a
  dispatched child observes by *blocking* on short calls; only a top-level session agent may
  background-and-await). It carries its **own** cap on the number of passes and on consecutive
  `budget not enforced` diagnostics, independent of the facade's — the facade's bound is a
  guarantee about states it can measure, not a promise that every loop ends. Its result is the
  terminal observation's **stdout** — the relay above — which is what makes an in-context-report
  agent (`build-*`, `review-*`) able to answer its caller at all.
- **The facade owns detachment, observation, and disposition.** It starts the adapter in its own
  process group with every stream redirected to a durable per-dispatch directory, records a
  sentinel as the launcher's last act, bounds observation by `delegation_observation_budget` —
  and, where that budget cannot be measured at all, by the consecutive-unenforceable counter — and
  kills the whole detached group before reporting a run unavailable.
- **The agent owns none of it.** The delegated agent has no sentinel obligation and no knowledge of
  the result directory — a sentinel written by the party being judged would make "done" a claim
  rather than evidence.

ADR-0038's chokepoint property is unchanged: two verbs, still exactly **one** dispatch seam, still
no inline fallback and no silent retry.

Evidence for the **adapter** launch shape — a different shape from the gate launch, which is why it
does not inherit `gate-execution.md`'s verdicts — is in
`skills/docket-build/references/delegation-execution.md`. Every row there is `unverified` today: the
detachment *mechanism* was measured hermetically, no child CLI was.

## Exit codes

- `1` — validation failure (including a value-taking flag given in final position with no value),
  unknown runner, not inside a git repository, or a rejected `--worktree` (missing for a
  feature-scoped agent, not a directory, not a worktree top-level of this repo, or — for a
  feature-scoped agent — the main worktree itself).
- `1` — a `--launch` that could not be established: the dispatch root or key could not be created,
  the detached child never appeared, or it did not separate into its own process group (in which
  case the child is `TERM`-ed first — the facade never reports a dispatch a teardown would kill).
- `0` — a `--launch` that was established. Stdout is the dispatch key and nothing else. This says
  the child **started** detached; it says nothing about how it ends, which is the observation
  step's job.
- `1` — the run gate's two-strikes abort: a delegated `implement-next` run was still
  `run-incomplete` after one re-dispatch. The change stays `in-progress` with its claim intact.
- `3` — the run gate's **halt** stop: the delegated `implement-next` run wrote a `## Run halted`
  section, so it stopped deliberately and needs a human. Distinct from `1` because it is not a
  failure of the run — a driver that wants to tell "did not finish" from "stopped on purpose"
  can. Never re-dispatched, and it applies to a halt seen on either verdict — a halt after a
  re-dispatch is terminal too, never folded into the success below. The generated shim wrappers
  read any non-zero as abort-and-report-stderr, which is the correct handling for both. **The same
  `3` is returned by `--observe`**, which is where a *delegated* halt now surfaces.
- `0` — a re-dispatch ran and the **second** verdict was `run-complete` or `run-unclaimed`. The
  gate's git-read verdict outranks the first adapter's (possibly non-zero, now stale) code. Only
  on this path — a gate that took no action never overrides.
- otherwise — the adapter's exit code, propagated verbatim.

The full post-re-dispatch matrix, second verdict → exit: `run-complete` → `0`, `run-unclaimed` → `0`,
`run-halted` → `3`, `run-incomplete` → `1`, anything else → the adapter's code.

### `--observe` (change 0271)

| Exit | Meaning |
|---|---|
| `0` | terminal — the dispatch completed: a green git verdict, or a sentinel saying `exit_code=0` that no git read disagrees with. Since change 0284 a **child that vanished without a sentinel** also lands here when git says the work landed |
| `1` | terminal — failed, **or** the result is unavailable (a distinct stderr diagnostic tells them apart: a `FAILED` line names the child's code, the git verdict that contradicts it, or the `run-incomplete` conjuncts, and a `RESULT UNAVAILABLE` line says why no code could be trusted). A **vanished** child with no positive git evidence lands here, as does a `child-vanished` marker whose recorded `disposition` is absent or unreadable |
| `3` | terminal — **halted**: a delegated `implement-next` run stopped deliberately and needs a human (`run-halted`). Not a failure of the dispatch, which is why it is not folded into `1`. Reached from **either** liveness source — the sentinel path, or change 0284's liveness probe when the child vanished and git says `run-halted` |
| `4` | **not terminal — still running; observe again**. Since change 0284 this requires the recorded process group to be **provably still the launched child's**; a dead group is terminal on that pass instead of spinning out the budget |
| other | a usage error from the shared validation above (missing/invalid key, unknown key, rejected `--worktree`), which exits `1` like any other abort |

Stdout on a terminal observation is the **relay** — the child's captured stdout, verbatim and
alone (see *The relay* above); on `4` it is empty. Diagnostics are always on stderr.

**`4` is not a failure.** It is the loop condition: it means nothing has been decided yet and the
caller should observe again. Its **only** consumer is the generated shim wrapper, whose standing
rule is "any non-zero ⇒ abort and report" — a rule that would read a healthy in-flight run as a
failure, so that consumer is changed in this same change to **loop on `4`** and abort on every
other non-zero. No other caller reads this facade's code, and `4` remains the only code this verb
mints: the `3` it can return is the synchronous gate's own halt code, unchanged in meaning and now
reachable from the seam a delegated run actually returns through. A shim that aborts on any non-`4`
non-zero therefore handles a halt and a failure identically, as it already did — which is why the
new `3` needs no shim change, only the stderr diagnostic to tell them apart.

**Idempotence, refined.** Every **terminal** state re-reports identically forever: a dispatch that
completed, failed, halted or was given up on returns the same code and the same diagnostic on every
later observation, which is the guarantee a caller's retry loop actually rests on. The `killed`
marker is what makes that true across the give-up transition — once it exists, a later observation
reports the same unavailable verdict rather than re-signalling or discovering a different state. It
is also what makes it true across change 0284's **vanished** transition, and there the marker must
carry more than the fact of the death: its `git_verdict=` and `disposition=` fields let the replay
reproduce a `0` or a `3` without asking git a second, differently-timed time.

The sentinel's precedence over the marker does not weaken that. Neither file is ever removed, so a
sentinel visible on one observation is visible on every later one and the verdict read from it is
the same forever after — and where a kill actually went out, no sentinel can appear at all (the
untrapped wrapper cannot write one after the signal). The single transition this admits is
**unavailable → complete**, at most once, only where **nothing was signalled** (`reason=group-already-gone`,
or the own-group refusal), and only on the arrival of real evidence that the work finished.
Reporting a result the facade can now see is not an oscillation; masking it forever was the defect.

The **still-running-and-unenforceable** path is deliberately *not* covered by that guarantee, and
it is the only path that is not: it counts consecutive passes in the dispatch dir, so the 1st, 2nd
and 3rd observations of an unenforceable dispatch differ (`4`, `4`, then terminal `1`). That is the
whole point — an observation that could never differ from its predecessor is an observation the
caller can never stop making. The counter is read and written **only** there, after every terminal
disposition above has already been decided, so no terminal state can be perturbed by it.

## Invariants

- The anchor is **never** resolved from the caller's CWD; absent `--worktree` it is the main
  worktree (ADR-0034 unamended). A relative `--worktree` joins to the main worktree, so the
  argument inherits that cwd-independence rather than reintroducing the hazard.
- **One payload channel per dispatch.** A brief file and trailing argv are never both forwarded to
  an adapter — the facade refuses the shape up front, and every handoff site (synchronous,
  `--launch`, and the run gate's re-dispatch) constructs a single-channel invocation.
- Never runs a child harness itself; all child specifics live in the adapter.
- `inherit` is docket's own no-pin sentinel and is normalized to "no model" **here**, once, for
  every adapter — adapters keep a one-line defensive twin for their documented hand-invocation
  path, never as a second decision. Real model IDs are untouched (ADR-0015).
- The adapter's exit code is propagated verbatim whenever the run gate takes no action; the
  two-strikes abort (`1`) and the halt stop (`3`) are the only new non-zeros, and both are on paths
  that were previously silent.
- The adapter's code is overridden with `0` on exactly one path: a re-dispatch ran **and** the
  second verdict positively showed the run finished. No re-dispatch ⇒ no override, ever.
- A `run-halted` verdict is terminal at this seam whichever verdict surfaces it: never
  re-dispatched, never exit-0. Stop and surface, per `docket-implement-next`'s disposition table.
- The run gate is scoped to `--agent implement-next` and never writes docket state — it acts only
  by running an agent. On the **synchronous** path it re-dispatches an unfinished change **at most
  once**; on the **observe** seam it re-dispatches **never** (a re-dispatch out of a repeated short
  read would race the run it is observing), so the two seams share an attribution and differ in
  exactly that one respect.
- Every `implement-next` delegation gets the gate at whichever seam it returns through, and none
  gets it twice: `--launch` reaches no verdict, `--observe` reaches it once the sentinel exists, and
  the synchronous fence runs only when no `--launch` was asked for.
- The **observe** seam's git-read disposition covers `implement-next` and `build-*` and nothing
  else. A sentinel claiming success that git contradicts is a failure — correctness outranks the
  child's self-report — and for **both** families that failure is **observe-only**: reported, never
  re-dispatched.
- The gate acts on at most one change per dispatch, and only on one whose `claimed_at` falls inside
  this dispatch's window. It never re-dispatches onto, nor reports a disposition for, a claim it
  cannot attribute to itself; where it cannot attribute one it stands down to the code the seam
  would otherwise have returned.
- Never degrades a delegation request to a native run.
- Without `--launch`, foreground only — the shim (and any native caller) blocks until the child
  exits. This is still the default: the verb is opt-in, so every currently-shipped caller is
  unaffected.
- With `--launch`, the facade starts the adapter and returns; it never waits for it, and it never
  reports a dispatch whose process group it could not confirm. The dispatch dir is the only channel
  between the two — the launch record and the sentinel are written by the **facade**, never by the
  agent being judged.
- An observation is **short**, and it never waits for the child. It is **idempotent on every
  terminal state**; the sole exception is the still-running-and-unenforceable path, which counts
  consecutive passes so that state can end at all. It mutates the dispatch only through that
  counter and the terminal `killed` marker, which exists precisely so that the give-up is observed
  identically forever after rather than re-attempted.
- The `done` sentinel outranks the `killed` marker, and the give-up re-reads it on both sides of
  its kill. A signalled group cannot produce a sentinel afterwards, so a sentinel that exists
  alongside a marker records a child that finished **before** the signal — reported as completed,
  once and thereafter identically. No completed run is ever masked as unavailable.
- Every state the observation can report is **reachable-terminal**: no dispatch state returns `4`
  forever. The budget bounds a run whose clock is readable; the consecutive-unenforceable counter
  bounds the run whose clock is not.
- A terminal observation puts the child's captured stdout on **its own stdout**, verbatim, and every
  diagnostic on stderr. A non-terminal (`4`) observation puts **nothing** on stdout, so a polling
  caller never accumulates partial output.
- Giving up on a dispatch **kills its process group**, never the launcher's pid alone — an
  observation that gave up must not leave the adapter running unwatched. Two states are exceptions.
  A record naming the observer's **own** group is refused outright: the facade never signals the
  group it is running in. And a group the record cannot **prove** is still the launched child's —
  `child_pid` no longer leading it, or leading it with a start time other than the recorded
  `child_lstart` — is left alone: a pgid is a reusable name, so signalling an unconfirmed one can
  reach an unrelated process group. That dispatch is still recorded terminal and still reports
  result unavailable.
- `4` is a **non-failure** exit and the only new code this verb mints. Its sole consumer loops on
  it; nothing else in docket reads this facade's exit code.
- The `runners.<name>:` parse handles simple `key: value` scalars only — by design; a runner
  needing structured config gets it via its own adapter contract, not a richer facade parser.
