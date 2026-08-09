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
  `build-*` agents**: the `docket-build-task` contract requires a build worker to run inside its
  feature worktree, on its branch, so a `build-*` delegation without it is a loud abort rather
  than a silent run in the primary checkout on the integration branch.
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
exports — and then replaces step 4's blocking handoff. It reaches no *verdict*: the gate reads what
a *finished* run left in git, and at launch time nothing has run yet. For an `implement-next`
delegation it does record the gate's **attribution inputs**, which are only knowable before the
child runs — see *The before-snapshot* below.

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
commits, which a later git-read verdict reports as unknown rather than guessing. `branch` is the
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

**The sentinel** — `<dir>/done`, flat `KEY=value`: `exit_code`, `started_at`, `finished_at`, `pid`,
`dispatch_key`. **The wrapper writes it, never the agent**: "done" must not be a claim by the party
being judged. It is written as the wrapper's last act, so its absence *is* "still running". The
write is atomic — a temp file **beside** its destination (the one licensed exception to templating
temp files into `TMPDIR`, because the rename must be same-filesystem) then `mv -f` — so a reader
never sees a half-written sentinel, and a failing adapter records its real code just as a clean one
does.

### Observation (change 0271)

`--observe <key>` is the only way to learn how a launched dispatch ended. It performs the same
validation and anchoring as every other call, then makes **one pass** over the dispatch dir:

1. a `killed` marker ⇒ terminal, **result unavailable** (`1`) — and it re-reports identically
   forever, which is what makes the observation idempotent across a caller's retry loop;
2. a `done` sentinel ⇒ the child is finished. A sentinel that does not parse ⇒ **result
   unavailable** (`1`), because a malformed sentinel means the *launcher* did not finish cleanly
   and an exit code read out of garbage would be a fabricated verdict. Otherwise the agent's
   git-read disposition decides where it has one — for `implement-next` that is the run gate, whose
   verdict can synthesize `3`, `1` or `0` over either exit-code class (see *Liveness vs
   correctness*) — and failing that the sentinel alone does: `exit_code=0` ⇒ **complete** (`0`),
   *unless* a `build-*` git read disagrees; a non-zero code ⇒ **failed** (`1`), naming the code and
   the `stderr.log` to read;
3. no sentinel ⇒ **still running** (`4`), unless the observation budget is spent.

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

**Kill on giving up.** When the budget is exhausted the facade signals the **process group**
recorded at launch (`TERM`, then `KILL` after a short wait), never the launcher's pid alone: a
single-pid kill reaps the launcher shell and **orphans the adapter and its children**, which is the
half-dead state this change exists to eliminate (it honors change 0231 — no presumed-dead worker
wakes to race its replacement). It then records a `killed` marker and reports result unavailable.
Partial work is left in the worktree for a human. One state is refused rather than signalled: a
launch record naming the **observation's own process group**. `--launch` fails closed when the
child did not separate, so a record it wrote can only name a foreign group — but a record can be
wrong anyway, and a group-directed signal aimed at our own group takes down the harness that ran
it. That case reports result unavailable, writes no `killed` marker, and names the dispatch dir.

**The identity check — a pgid is a reusable name.** The own-group refusal defends the harness; it
defends nobody else. This path is reached **only** when no sentinel exists, which includes a child
killed externally or dead without a sentinel an hour earlier — by which time the OS may have handed
that group id to an unrelated tree. So the group is signalled only when the launch record proves it
is still the launched one, on two conjuncts: the recorded `child_pid` must **still lead** the
recorded `pgid` (the child is its group's leader by construction), and that pid's start time must
still equal the recorded `child_lstart` — the first conjunct alone is satisfied by a **recycled pid**
that happens to lead a group of the same id, which is an ordinary background job and not an exotic
state. The token conjunct is skipped only when the launch recorded none.

Failing either conjunct is not an error but the ordinary *that group is already gone* outcome: **no
signal is sent**, the terminal `killed` marker is still written with `reason=group-already-gone`
(so the dispatch stays terminal and a later observation re-reports it as unavailable, saying
plainly that nothing was signalled rather than claiming a kill), and the verdict is **result
unavailable** either way — the run outran its budget with no sentinel, so there is no result to
report whether or not a signal went out. The **accepted residual**: a group whose leader died while
processes it spawned keep running is not signalled, so those orphans outlive the budget; the same
holds when `ps` cannot be read. Killing them would mean signalling a name that cannot be proven
still theirs, and an unrelated process group dying is both the worse failure and the unrecoverable
one, while an orphan is visible and reapable.

**Liveness vs correctness.** The sentinel is the *only* source of liveness — the facade never
infers "still running" from git state, and never infers "finished" from anything but the wrapper's
own sentinel. **Correctness** is a separate judgment, made from git rather than from the child's
self-reported code. Keeping the two sources apart is what lets the facade observe a *live* child
without ever reading liveness out of git state.

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

- **The shim launches and observes.** It makes one `--launch` call and then bounded, short
  `--observe` calls. It never blocks for the child's duration and never yields between
  observations (ADR-0024 unamended: a dispatched child observes by *blocking* on short calls; only
  a top-level session agent may background-and-await). Its result is the terminal observation's
  **stdout** — the relay above — which is what makes an in-context-report agent (`build-*`,
  `review-*`) able to answer its caller at all.
- **The facade owns detachment, observation, and disposition.** It starts the adapter in its own
  process group with every stream redirected to a durable per-dispatch directory, records a
  sentinel as the launcher's last act, bounds observation by `delegation_observation_budget`, and
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
| `0` | terminal — the dispatch completed: a green git verdict, or a sentinel saying `exit_code=0` that no git read disagrees with |
| `1` | terminal — failed, **or** the result is unavailable (a distinct stderr diagnostic tells them apart: a `FAILED` line names the child's code, the git verdict that contradicts it, or the `run-incomplete` conjuncts, and a `RESULT UNAVAILABLE` line says why no code could be trusted) |
| `3` | terminal — **halted**: a delegated `implement-next` run stopped deliberately and needs a human (`run-halted`). Not a failure of the dispatch, which is why it is not folded into `1` |
| `4` | **not terminal — still running; observe again** |
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

**Idempotence.** Same key, same dispatch state ⇒ same code and same diagnostic, every time. The
`killed` marker is what makes that true across the one transition that is not a pure read: once the
budget kill has fired, every later observation reports the same unavailable verdict rather than
re-signalling or discovering a different state.

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
- An observation is **short and idempotent**: it never waits for the child, and it never mutates the
  dispatch except for the one terminal `killed` marker, which exists precisely so that the kill is
  observed identically forever after rather than re-attempted.
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
