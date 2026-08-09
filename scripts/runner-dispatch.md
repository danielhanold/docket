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
docket.sh runner-dispatch [--launch] --runner <name> --agent <agent> [--model <m>] [--effort <e>] [--worktree <path>] [--] [<args…>]
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
  `build-*` agents**: the `docket-build-task` contract requires a build worker to run inside its
  feature worktree, on its branch, so a `build-*` delegation without it is a loud abort rather
  than a silent run in the primary checkout on the integration branch.
- `--launch` (optional, change 0271) — the **detached** verb. Same request, same validation, same
  gates, same `runners.<name>:` resolution; the difference is only in how the adapter is started
  and when the call returns. Instead of blocking for the child's whole run, the facade starts the
  adapter in its **own process group**, redirects every stream into a durable per-dispatch
  directory, prints a **dispatch key** on stdout, and exits `0`. Absent, the call is the legacy
  synchronous call-and-return described below, byte-identical to pre-0271 behavior.
- `-- <args…>` — forwarded to the adapter as caller task context.

Mock seams: `RUNNERS_DIR` (adapter directory), `GIT` (the git binary — read by
`lib/docket-root.sh` and, since change 0271, by the launch verb's dispatch-time SHA read), and —
for the run gate (change 0237) — `VERIFY_RUN` (the disposition reader, default
`scripts/verify-run.sh`) and `DOCKET_FACADE` (the facade used for the metadata re-syncs on both
sides of the handoff, default `scripts/docket.sh`).

## Behavior

1. **Validate** — both required flags present; adapter file exists.
2. **Anchor** — `DOCKET_REPO_ROOT` = `docket_anchor_path "${worktree:-.}"`
   (`scripts/lib/docket-root.sh`, ADR-0034). With no `--worktree` that is the repo's primary
   checkout, cwd-independent — correct even when invoked from `.docket/` or a `.worktrees/<slug>`
   feature worktree. With `--worktree` it is the named tree, and a relative value joins to the
   main worktree so it too resolves identically from any cwd. Not in a repo ⇒ abort. Three loud
   gates follow, all before the config read so an anchor failure never depends on config parsing:
   `--worktree` is **required** for a `build-*` agent; the resolved anchor must be a **directory**;
   and it must be a **worktree of this repository** (compared via `docket_main_worktree "$anchor"`,
   whose empty result for a non-repo path fails the same comparison).
3. **Resolve `runners.<name>:`** — per **key**, first layer that has the key wins, across
   `<repo>/.docket.local.yml` > `<repo>/.docket.yml` >
   `${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml`. Each `key: value` scalar is exported
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
5. **Run gate (change 0237)** — engages **only** for `--agent implement-next`. The facade re-syncs
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
   optimization. An unrecognised agent is a no-op, never a guess. A snapshot that cannot be read, a
   failed clock read, or an ambiguous claim set all **disable the gate with a warning** — none of
   them ever converts a healthy dispatch into a failure. A failed metadata re-sync (either side) is
   likewise best-effort: it warns and degrades the gate's freshness rather than failing a dispatch
   that may well have succeeded.

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
`build-*` `--worktree` requirement), same `runners.<name>:` resolution and `DOCKET_RUNNER_CFG_*`
exports — and then replaces step 4's blocking handoff. It never engages the run gate: the gate
reads what a *finished* run left in git, and at launch time nothing has run yet.

**Where the result lives.** `<git-common-dir>/docket/dispatch/<key>/`, the same family as
`disable-worktree-hooks.sh`'s docket-owned directory inside the common git dir. Under `.git/`, so
it is never tracked, never leaks into a commit, and needs no `.gitignore` entry. Deliberately
**not** in the feature worktree: a dispatch result must outlive `git worktree remove`, since the
whole point of the verb is that the child's work can be inspected after the run was declared over.

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

**The launch record** — `<dir>/launch`, flat `KEY=value`: `pgid`, `child_pid`, `started_at`,
`agent`, `runner`, `worktree`, `since_sha`. `pgid` is the group an observer must signal to reach
the whole detached tree. `since_sha` is the repo's `HEAD` captured **before** the child could
commit anything — the direct analogue of the run gate's `DISPATCH_EPOCH`, so a commit landing in
the gap is excluded either way; empty on a repo with no commits, which a later git-read verdict
reports as unknown rather than guessing.

**The sentinel** — `<dir>/done`, flat `KEY=value`: `exit_code`, `started_at`, `finished_at`, `pid`,
`dispatch_key`. **The wrapper writes it, never the agent**: "done" must not be a claim by the party
being judged. It is written as the wrapper's last act, so its absence *is* "still running". The
write is atomic — a temp file **beside** its destination (the one licensed exception to templating
temp files into `TMPDIR`, because the rename must be same-filesystem) then `mv -f` — so a reader
never sees a half-written sentinel, and a failing adapter records its real code just as a clean one
does.

## Exit codes

- `1` — validation failure, unknown runner, not inside a git repository, or a rejected
  `--worktree` (missing for a `build-*` agent, not a directory, or not a worktree of this repo).
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
  read any non-zero as abort-and-report-stderr, which is the correct handling for both.
- `0` — a re-dispatch ran and the **second** verdict was `run-complete` or `run-unclaimed`. The
  gate's git-read verdict outranks the first adapter's (possibly non-zero, now stale) code. Only
  on this path — a gate that took no action never overrides.
- otherwise — the adapter's exit code, propagated verbatim.

The full post-re-dispatch matrix, second verdict → exit: `run-complete` → `0`, `run-unclaimed` → `0`,
`run-halted` → `3`, `run-incomplete` → `1`, anything else → the adapter's code.

## Invariants

- The anchor is **never** resolved from the caller's CWD; absent `--worktree` it is the main
  worktree (ADR-0034 unamended). A relative `--worktree` joins to the main worktree, so the
  argument inherits that cwd-independence rather than reintroducing the hazard.
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
  by running an agent. It re-dispatches an unfinished change **at most once**.
- The gate acts on at most one change per dispatch, and only on one whose `claimed_at` falls inside
  this dispatch's window. It never re-dispatches onto a claim it cannot attribute to itself.
- Never degrades a delegation request to a native run.
- Without `--launch`, foreground only — the shim (and any native caller) blocks until the child
  exits. This is still the default: the verb is opt-in, so every currently-shipped caller is
  unaffected.
- With `--launch`, the facade starts the adapter and returns; it never waits for it, and it never
  reports a dispatch whose process group it could not confirm. The dispatch dir is the only channel
  between the two — the launch record and the sentinel are written by the **facade**, never by the
  agent being judged.
- The `runners.<name>:` parse handles simple `key: value` scalars only — by design; a runner
  needing structured config gets it via its own adapter contract, not a richer facade parser.
