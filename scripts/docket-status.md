# scripts/docket-status.sh — deterministic docket-status orchestrator

## Purpose

One-invocation, deterministic orchestrator for the docket-status pass. It sequences the shared
docket scripts (`docket-config.sh`, `render-board.sh`, `github-mirror.sh`, `archive-change.sh`,
`render-change-links.sh`, `terminal-publish.sh`, `cleanup-feature-branch.sh`, `board-checks.sh`,
`reclaim-claims.sh`, `sync-integration-branch.sh`, `render-learnings-index.sh`) inside one process and emits a single
line-oriented report on stdout. It performs no mechanics of its own beyond sequencing and thin
glue — each shared script still owns its own contract. Change 0058; the learnings pass is change
0067.

## Usage

```
docket-status.sh [--board-only] [--digest-only] [--must-land] [--repo OWNER/REPO]
                  [--type TYPE|untyped|all] [--priority PRIORITY|all]
                  [--project OWNER/NUMBER] [--auto-create-project] [--project-owner OWNER]
docket-status.sh -h | --help
```

| Flag | Description |
|---|---|
| `--board-only` | Only run steps 1–4 (config/bootstrap, worktree sync, board pass, backlog pass) and exit; skip sweep detection/execution, health checks, judgment emission, and integration sync. |
| `--digest-only` | **Write-free read** (change 0094). Resolve config, enforce the bootstrap verdict fail-closed, confirm the metadata worktree's changes dir actually exists, and (only once all three succeed) emit the backlog digest — `backlog` rollups, `change` lines, and the trailing `ready` queue line — and exit 0. Runs **no** metadata-worktree sync (it does not call `docket_preflight`), no sweep, no health checks, no learnings pass, no board render, no commit and no push, and emits **no `board …` line** and no `pass ok`. Fails closed (exit 1, diagnostic on stderr) rather than emitting an empty-but-successful digest when config export fails, the bootstrap verdict is non-`PROCEED`, or the resolved changes dir does not exist as a directory — see Exit codes. Mutually exclusive with `--board-only` and with `--must-land` (exit 2, in either order for each pair): a selection read must not be a write, and `--must-land` has nothing to retry without a board pass. This is the entry point `docket-implement-next` Step 1 uses to acquire its ordered candidate set. |
| `--must-land` | With `--board-only`: run the board pass with an in-script bounded retry (3 attempts) on the sole retryable outcome `board inline changed push-failed`, re-syncing the metadata worktree between attempts, and map the result to the exit code. Exit 0 iff every board line is a terminal success — and that success set is exhaustive: `board inline changed pushed`, `board inline clean`, `board off`, `board github ok`, and nothing else. Any other terminal line or retry exhaustion exits non-zero; `board inline blocked-wedged-tree` (change 0247) is deliberately **outside** the success set and is **terminal**, not retryable, so it exits non-zero on its first appearance with no retry. Report-line vocabulary and flagless behavior are unchanged; `board_pass`'s fail-closed exit 2 propagates. Meaningless (and rejected) with `--digest-only`, which never runs a board pass. |
| `--type TYPE` | Report filter forwarded to the digest projection only (change 0127): `all` (default, ≡ omitted), `untyped`, or a well-formed `[a-z][a-z0-9-]*` token. Narrows the `change` lines and the `ready` queue. It is **not** forwarded to the board writer, so a filtered `--board-only` run still commits a COMPLETE `BOARD.md`; it likewise never narrows sweep/merge detection, harvesting, archiving, publishing, health checks, or reclaim — those report work the command actually performed. An invalid value is rejected UP FRONT — before any pass runs — so it fails closed identically in every mode (exit 2, nothing mutated) rather than being swallowed by the best-effort backlog pass. |
| `--priority PRIORITY` | Report filter, same projection-only semantics as `--type`: `all` (default) or one of `critical`/`high`/`medium`/`low`. Combined with `--type` by logical AND. |
| `--repo OWNER/REPO` | GitHub repo for PR-link resolution and sweep merge detection. Defaults to deriving from the `origin` remote (see `render-board.sh`) and, for sweep detection, from `gh repo view` when unset. |
| `--project OWNER/NUMBER` | GitHub Project to sync during the github board surface. Passed through to `github-mirror.sh`. |
| `--auto-create-project` | Create the GitHub Project if `--project` doesn't resolve. Passed through to `github-mirror.sh`. |
| `--project-owner OWNER` | Owner to create the project under when auto-creating. Passed through to `github-mirror.sh`. |
| `-h`, `--help` | Print the usage synopsis (the range of the leading header comment block that `usage()` prints) and exit 0. |

Any other argument is a hard error (`docket-status: unknown argument: <arg>`, exit 2).

Configuration (`DOCKET_MODE`, `METADATA_WORKTREE`, `METADATA_BRANCH`, `INTEGRATION_BRANCH`,
`CHANGES_DIR`, `ADRS_DIR`, `BOARD_SURFACES`, `TERMINAL_PUBLISH`, `RECLAIM_LEASE_TTL`, `RECLAIM_AUTO`,
`BOOTSTRAP`, …) comes entirely from
`docket-config.sh --export`, evaluated with `eval` into `main()`'s scope by the shared
`docket_preflight` call at the top of `main` (see Behavior, steps 1–2). The script defines no
config of its own.

## Behavior

The pass runs as a fixed 9-step sequence:

**1–2. Config, bootstrap gate, and metadata worktree ensure + sync — delegated.** Step-0 sync is
delegated to the shared `scripts/lib/docket-preflight.sh` (`docket_preflight`), the single sync
implementation shared with the `docket.sh` facade. `main()` calls `docket_preflight "$SCRIPTS_DIR"`,
which: (1) runs config export (normally `docket-config.sh --export`, overridable via the
`CONFIG_EXPORT_CMD` mock seam) and `eval`s the output into `main()`'s scope — a non-zero exit from
config export is a hard error; gates the resulting `BOOTSTRAP` verdict (`PROCEED` continues;
`STOP_MIGRATE` and `CREATE_ORPHAN` each print a remedy to stderr and return non-zero; any other
value is an unknown-verdict hard error); then (2) in `DOCKET_MODE=docket`, ensures the metadata
worktree (`METADATA_WORKTREE`, default `.docket`) exists — creating it from `METADATA_BRANCH` or
`origin/METADATA_BRANCH` if missing — then fetches and rebase-pulls `METADATA_BRANCH` inside it; in
non-docket mode, syncs the current checkout by the identical path against
`origin/INTEGRATION_BRANCH` — which the checkout must already **be** on (preflight aborts rather
than syncing a topic branch), and which is required, never defaulted to `METADATA_BRANCH` (the mode
keyword `main` there, not a branch name). A non-zero return from
`docket_preflight` (config export failure, bootstrap gate, or an unusable metadata worktree) is a
hard error and this script exits 1 immediately — with **one carve-out** (change 0247).

*The wedged-tree carve-out.* `docket_preflight` fails closed on a metadata worktree that already
has a **rebase or merge in progress**: it refuses to sync one, because a commit made into that
state lands on the rebase's detached HEAD and the next `rebase --abort` destroys it. That refusal
is Step 0, ahead of both write paths, so the passes that own the `blocked-wedged-tree` report line
never get to run — and a token this contract documents that no run can emit is worse than no token
at all. So when (and only when) the bootstrap verdict was `PROCEED` and the metadata worktree
re-probes as wedged, this script maps that one preflight failure onto its own vocabulary: it emits
`board inline blocked-wedged-tree` (when `inline` is among the configured surfaces — the only
surface that writes into the shared worktree) and, on the full path in a learnings-enabled repo
with a learnings dir, `learnings index blocked-wedged-tree`; then exits **non-zero under
`--must-land`** and **0 otherwise**, the best-effort posture those lines already promise. It prints
no `pass ok` — no pass ran to completion — and runs no board render, sweep, health, reclaim, or
backlog pass. Every other preflight failure keeps the bare exit 1, the bootstrap gate included: an
unmigrated repo that happens to be mid-rebase still exits non-zero with no token.

Note the cost: the sync classifies a wedge as *transient* (another agent mid-sync, worth waiting
out) and spends its full bounded retry budget — roughly 22 seconds — before returning, so this
report arrives after that delay rather than immediately. `DOCKET_PREFLIGHT_TEST_SLEEP_CMD`
(`scripts/lib/docket-preflight.sh`) is the seam that keeps fixtures from paying it.

The in-function probes in `commit_and_push_generated` and sweep step 6a are **not** made redundant
by this mapping, and neither layer can be dropped: preflight answers only for a wedge that already
existed when the pass started, while the probes cover the window in which another agent starts its
rebase *after* Step 0 returned, mid-pass. The report line is identical either way — which layer
answered is visible only in the stderr diagnostic.

**3. Board pass**, once per surface token in the space-separated `BOARD_SURFACES` config value.
The reserved token **`none`** is the deliberate off-state and emits a positive `board off` line
(change 0069) — never silence. An **empty** `BOARD_SURFACES` is a wiring bug, not a
configuration: the pass exits 2 with a diagnostic (change 0071), because `docket-config.sh` never
emits an empty value and an unresolved config must never masquerade as a disabled board.
- **inline** — Renders and writes the board through `board-refresh.sh` (change 0059), which owns
  the surface gate and the atomic, truncation-safe replace of `BOARD.md`; this script never calls
  `render-board.sh` to produce the board. A failed render leaves the existing `BOARD.md`
  untouched, logs to stderr, and is treated as success for sequencing purposes (best-effort) — but
  it emits the positive stdout line `board inline failed` (change 0071 review, finding 1), never
  just the stderr diagnostic: the report-line channel must never go silent on a path that still
  exits 0, or a must-land caller keying on the report line (never the exit code) would read the
  silence as "the board landed". This line is terminal, not retryable. If
  `BOARD.md` is unchanged, `board inline clean` requires TWO things to hold, not just a clean
  working tree (change 0071 review, finding 3): the render produced no diff, **and** the local
  metadata branch carries no commit touching `BOARD.md` that is unpushed relative to its upstream
  (`@{u}..HEAD`, count > 0; no upstream at all counts as nothing-to-push, not an error). A clean
  working tree alone is not evidence the board landed — a prior run may have committed it locally
  and then failed to push. When the tree is clean but such an unpushed commit exists, nothing new
  is committed; execution falls through into the same push/rebase retry loop as a changed render,
  reporting `board inline changed pushed` / `board inline changed push-failed` from its outcome.
  When the render actually changed `BOARD.md`, it is `git add`ed and committed with message
  `docket: board refresh`, then pushed with up to 5 retry attempts: on push failure it
  rebase-pulls; if the rebase conflicts only on `BOARD.md`, it regenerates through the same gated
  helper (never a raw redirect) and continues the rebase; a rebase conflict on anything else, or a
  failed regeneration mid-rebase, aborts the rebase and stops retrying.
- **github** — Runs `github-mirror.sh` (passing through `--repo`, `--project`,
  `--auto-create-project`, `--project-owner`), best-effort. Lines it emits of the shape
  `issue-minted <id> <n>` / `project-minted <id> <n>` are translated to `minted issue <id> <n>` /
  `minted project <id> <n>` on this script's stdout; the surface's own success/failure is
  reported as one final `board github ok|failed` line regardless of what was minted.
- Any other token is an unrecognized-surface warning on stderr (non-fatal) — and, alongside it, a
  positive `board <token> unknown` stdout line (change 0071 review, finding 1) so a typo can never
  silently vanish from the report the way it used to when the warning lived on stderr alone. This
  line is terminal, not retryable — a typo is a config problem, not a transient one.

**`--must-land` (change 0085).** With `--board-only`, `--must-land` wraps this step in
`board_pass_must_land` instead of calling `board_pass` directly: it classifies the board pass's own
report lines (`board_classify`) into `success` / `retryable` / `failed`, and on the sole retryable
outcome (`board inline changed push-failed`) re-syncs the metadata worktree (`git pull --rebase`)
and re-runs the board pass, up to 3 attempts total. Every attempt's report line(s) still reach
stdout — the vocabulary above is unchanged — and the wrapper only adds the retry and an exit-code
mapping on top. `board_pass`'s own fail-closed `exit 2` (unresolved `BOARD_SURFACES`) propagates
verbatim, uninvolved with the retry loop. Flagless callers (no `--must-land`) never reach this
wrapper — `main()` calls `board_pass` directly, byte for byte as before change 0085.

**4. Backlog pass — UNGATED, once per path.** Runs `render-board.sh --format digest` and passes
its lines through (`backlog <status> <count>` rollups, then one `change <id> <status> <readiness>
<slug>` line per active change, then the trailing `ready <id> …` build-ready-queue line). It runs
**regardless of `board_surfaces`**, because **the digest
is report output, not a board surface**: it persists nothing, commits nothing, pushes nothing, and
never touches `BOARD.md`. That boundary is exactly what lets `board_surfaces: []` keep meaning "no
board is rendered or committed" while backlog state still reaches the report. Best-effort: a
digest failure logs to stderr, emits no digest lines, and never aborts the pass. Resolution is
**not** reimplemented here — `render-board.sh` stays the single owner of readiness.

The digest is a snapshot of the change files **at the moment it runs**, so it is called **once per
path** and the placement is part of the contract:

- **Under `--board-only`** (no sweep runs) it fires **here**, right after the board pass: the
  **state as-is** projection. That is what makes the "just show me the backlog" path useful in a
  board-off repo, where it previously did nothing at all. The process then prints `pass ok` and
  exits 0 — no sweep, health checks, judgment, or integration sync.
- **On a full pass** it fires **after** steps 5–8, once the sweep, the check/judgment lines, and
  the learnings pass are done: the **state after the pass** projection. **A change swept to
  `done` during this very pass therefore appears in the digest as `done` — not as the
  `implemented` it was when the pass began** — and is counted in `backlog done <n>`, never in
  `backlog implemented <n>`. A pre-sweep snapshot would make the report contradict its own `swept`
  lines, and since the digest is the sole backlog channel, that staleness would have no corrective
  path.

Report line order on a full pass is therefore: board → sweep lines → check lines → reclaim lines
(step 7a) → judgment lines → learnings lines → backlog digest → `pass ok`.

**4a. Digest-only path (`--digest-only`, change 0094).** A separate, write-free entry point that
runs `digest_only_pass` and exits before anything else — including `docket_preflight`. It resolves
config, enforces the bootstrap verdict fail-closed, and runs the same step-4 backlog pass. It is
deliberately **not** built on `--board-only`: that flag commits and pushes `BOARD.md`, and a
selection read must not be a write.

Skipping the preflight sync is what makes the read strict — preflight fetches and rebases the
metadata worktree, which can move `HEAD`. Nothing is lost: `docket-implement-next` runs this
**after** its own Step-0 preflight, so the tree is already synced. The digest is a snapshot of the
change files as of the moment it runs; taking it before Step 0's sweep would list already-merged
changes, which is why Step 1 orders it after.

Skipping preflight also means nothing else guarantees the metadata worktree exists — on every
other path, `docket_preflight` creates it if it's missing, but this path never calls it. So
`digest_only_pass` resolves the changes dir itself (`docket_metadata_worktree` + `$CHANGES_DIR`,
the same resolution `backlog_pass` uses) and checks it exists **before** calling `backlog_pass`.
A fresh clone of an already-migrated repo is the reachable failure shape: `origin/docket` exists
(`BOOTSTRAP=PROCEED`), but `.docket/` is gitignored and was never materialized in this clone.
Without the check, `backlog_pass`'s underlying `render-board.sh` call would fail, log to stderr,
and best-effort `return 0` — exit 0 with an empty digest and no `ready` line, indistinguishable
from "nothing is ready" to a caller keying on the exit code. The check instead fails the pass
closed (exit 1, naming the resolved path and pointing at `docket.sh preflight`) — see Exit codes.

**5. Batched sweep detection.** `detect_merged` scans `active/*.md` for `status: implemented`
changes, resolves each PR's merge state with one batched `gh api graphql` call keyed by change ID
(for changes with a known `pr:` number) plus a per-change `gh pr list --repo <repo> --head
feat/<slug> --state merged` fallback for changes without one, and emits merged changes as
TAB-separated `<id>\t<slug>\t<pr>\t<merged-date>` (merged-date is the UTC date portion of GitHub's
`mergedAt`, never derived from local time / `now()`), followed by a fifth field `<base-ref>` — the
PR's `baseRefName`, i.e. the branch the code actually landed on (change 0298). Both arms request it.
It is carried through detection rather than resolved during close-out because it is a property of
the PR and the close-out makes no further `gh` call; an empty fifth field (an older `gh`, a parse
miss, or a hand-fed four-field record) degrades to the pre-0298 behavior — the change closes out
normally. The fallback is repo-scoped with the same
resolved `<repo>` as the batched arm, so a `--repo`-scoped pass never queries the repository the
process CWD implies. Any `gh`/network/parse failure is swallowed and reported
as `sweep-skipped <reason>` (`gh-unavailable` or `repo-unresolved`); detection never aborts the
pass.

**6. Sweep execution**, one change at a time, chaining the ADR-0035 close-out scripts in order:
rebase-pull the metadata worktree, then (skipping silently if the change is already archived,
already `done`/`killed`, or already `stacked-merged` — idempotent no-ops) the **stacked-merge
gate** (see below) → `archive-change.sh` → locate the archived file →
`render-change-links.sh` → **artifacts refresh** (see below) → `terminal-publish.sh` (always
passed `--enabled "${TERMINAL_PUBLISH:-false}"`, so the headless sweep honors the repo's publish
policy — unset defaults to no publish since change 0084; a suppressed publish is a no-op that
exits 0 and is logged, never a failure) →
`cleanup-feature-branch.sh`. Each step's failure emits `sweep-failed <id> <step> <reason>` and
abandons the rest of that change's close-out, but the loop always continues to the next change;
the **artifacts refresh** and a `cleanup-feature-branch.sh` failure are the two exceptions — both
still emit the terminal `swept`/`harvest` lines for that change. Full success for a change emits
`swept <id> <merged-date>` followed by `harvest <id> <archived-path>`, and queues that change for
the **stack close-out** (step 6-closeout), which runs once the whole loop has finished. Self-heal is idempotent for
a failure at `sync` (rebase-pull) or `archive`, and for a `cleanup` failure (all retry cleanly next
pass) — but a `sweep-failed` at `render-change-links` (`skipped-publish`, i.e. the renderer itself
exited non-zero) or at `terminal-publish` leaves the change **archived but its terminal record
unpublished**, which no later sweep resumes (the sweep only scans `active/*.md`), and requires a
manual `terminal-publish.sh --id <id> --enabled true` follow-up. The `terminal-publish` case is no
longer *invisible*, though: before emitting its `sweep-failed` line the sweep calls
`mark-publish-deferred.sh --mode add --reason blocked` on the archived file and commits+pushes it
on `metadata_branch`, so `board-checks`' `publish-deferred` check surfaces the gap on every later
pass until the publish completes (change 0083). That mark is **strictly best-effort** — muted, its
outcome never read — so a failure to mark changes neither the control flow nor the report lines
above; the residual is an unmarked deferral, which is still invisible. The `render-change-links` case marks too (change
0118), under the same best-effort posture and one extra gate: `TERMINAL_PUBLISH=true` **and**
docket-mode. That gate is load-bearing on this leg and absent on the other, because both of the
publish's suppressions are exit-0 no-ops — so the `terminal-publish` branch is unreachable under
suppression, while a renderer failure fires regardless of the knob. The pre-0118 rationale
("nothing published means nothing was deferred *yet*") does not survive the code: once archived the
change leaves `active/`, the sweep scans `active/` only, and no later pass resumes it — so the gap
is permanent until a human acts, which is exactly what ADR-0051's marker exists to surface. Whether
the publish was deferred, blocked, or never reached is a distinction about *cause*, and cause
travels in the dated `--detail` line. Under suppression the leg stays unmarked, because a suppressed
publish is *success*, not a deferral (ADR-0051); the residual there is unchanged — the archived
change keeps a stale `## Artifacts` block on `metadata_branch` that no later sweep resumes, and the
follow-up is a manual re-render on the metadata branch, not a publish. Both marks share one writer,
`sweep_mark_publish_deferred`, which skips entirely when the shared worktree is mid-rebase/merge,
restores the path to `HEAD` if `add`/`commit` fails, and retains a committed-but-unpushed marker so
the next pass's `pull --rebase` carries it. One precondition is **not** shared: only the
`render-change-links` leg also skips when the archived path is already dirty. There the archive
commit has just landed and the renderer writes atomically, so a dirty path is another actor's
uncommitted state. The `terminal-publish` leg marks over a dirty path deliberately — that is where
`terminal-publish.sh` has already stripped the marker in the shared worktree and failed to commit
the removal, and the documented recovery for that window is exactly this re-mark; step 6a's
`commit-failed` leg (below) also reaches the publish with the path dirty.

**6-stack. The stacked-merge gate (change 0298).** Runs before `archive-change.sh` and can end the
close-out on the spot. It fires for a change that satisfies **all three** conjuncts: the change
carries a `stacked_on:` parent, that parent's change file exists and records a `branch:`, and the
merged PR's `<base-ref>` is **exactly** that branch. All three are required, and the third is the
discriminating one: a stacked change whose PR was retargeted onto and merged into the integration
branch has its code reachable from that branch, so the governing invariant makes it `done` and it
takes the ordinary close-out. When the gate fires, the sweep sets `status: stacked-merged` in place
— an edit anchored to the first `---…---` frontmatter block — commits and pushes it on
`metadata_branch` (`docket(<id>): stacked-merged — PR merged into #<parent>'s branch`), emits
`stacked-merged <id> <parent>`, and **returns without archiving, publishing, or cleaning up the
branch**. Each of those omissions is load-bearing: the change is not `done`, so it stays in
`active/`; `stacked-merged` is a non-terminal status, so there is no terminal record to publish; and
the feature branch still carries the only copy of code the root's PR needs, so deleting it would
lose work. The promotion to `done` belongs to the stack close-out, not to this pass. Failures follow
the sweep's log-and-continue posture (`sweep-failed <id> stacked-merged <reason>`); every one of them
self-heals, by one of two routes the report-line table spells out — three leave the change
`implemented` for the next pass to retry, and `push-failed` retains a commit that has already landed
locally and needs only its push.

**6a. The artifacts refresh (change 0075).** After `render-change-links.sh` rewrites the archived
change's `## Artifacts` block in the metadata worktree, the sweep **commits and pushes** that file
on `metadata_branch` (`docket(<id>): refresh artifacts links`) — but only when the render actually
changed bytes; an unchanged file is a silent no-op. `$mw` (the metadata worktree, and therefore the
`$archived` pathspec this step tests) is **absolute** — anchored to the repo's MAIN worktree by
`lib/docket-root.sh`, so the step means the same thing from every CWD, including a linked worktree.
Both the `add` and the `commit` carry a `--` pathspec, and the step is gated on a rebase/merge
already being in progress in that shared tree — the same two rules as `commit_and_push_generated`,
for the same reason (change 0247): the metadata worktree is shared, so an unscoped commit sweeps up
whatever another agent had staged, and a commit into a mid-rebase tree writes onto that rebase's
detached HEAD. The two sites order the wedged probe differently, deliberately: this step probes
**inside** its `status --porcelain -- "$archived"` gate, so a wedge is reported only when there is
actually something to commit, whereas `commit_and_push_generated` probes **before** its
nothing-to-commit check and so reports a wedge even on a clean tree. **This step never aborts the close-out.** A failure emits `sweep-failed <id>
render-change-links commit-failed`, `… push-failed`, or `… blocked-wedged-tree` on the report
channel and the sweep **continues** to `terminal-publish.sh` and `cleanup-feature-branch.sh`. That posture is
deliberate: a stale link block is cosmetic and self-heals on a manual re-render, whereas an aborted
close-out leaves the change archived-but-unpublished (invisible to every future sweep) plus an
orphaned worktree and remote branch — a strictly worse, non-self-healing state. Callers key on the
report **line**, never on the exit code.

**6-closeout. The stack close-out (change 0298).** Runs **after the whole per-change loop**, once
per change this pass swept to `done`, and only when that change actually has `stacked_on:`
descendants — a change with no stack pays nothing, not even a subprocess. It shells
`stack-closeout.sh --root-id <id> --date <merged-date>` with the pass's resolved
`--integration-branch`, `--metadata-branch`, `--adrs-dir` and `--terminal-publish`, and relays its
report lines verbatim: `promoted <id>`, `promote-skipped <id> <reason>`,
`promote-failed <id> <reason>`, `stack-carried <root> <count>`, `stack-carried-failed <root>
<reason>`. The root's code has just reached the integration branch, so every descendant parked at
`stacked-merged` became reachable from it too and is `done` by the governing invariant — and nothing
else would ever tell them so, because the sweep only detects a MERGED PR on an `active/` change and
a descendant's PR merged passes ago.

The after-the-loop placement is **load-bearing, not incidental**. `stack-closeout.sh` snapshots the
descendant graph at call time, so a child THIS pass flipped to `stacked-merged` (step 6-stack) is in
the snapshot and is promoted in the same pass. Invoked from inside the loop it would read the graph
as it stood before that flip: detection emits in ascending id order and a parent is normally the
lower id, so the root would be swept first and a child merged in the same window would wait a full
pass. Only the FULL-SUCCESS close-out path queues a root — an abandoned close-out has not put the
root's code anywhere, and promoting its stack off the back of it would falsify the invariant.

Failure posture is the sweep's own: a close-out that could not run emits
`sweep-failed <id> stack-closeout script-error` and the pass **continues to the next root**. It
never aborts — the root is already `done` and archived by then — and it self-heals, because every
step of the close-out is idempotent and the next pass re-runs it.

**7. Health checks.** Runs `board-checks.sh` over the current changes-dir and metadata/integration
branches — forwarding `--lease-ttl-hours "${RECLAIM_LEASE_TTL:-72}"` (change 0089) so the
claim-lease staleness signal keys on the repo's configured `reclaim.lease_ttl` — and prefixes each
of its TSV findings as `check <check-id> <change-id> <message>` on this script's stdout. Also emits
one `judgment blocked <id> <blocked_by-text>` line per `active` change with `status: blocked`,
leaving the actual re-examination judgment to the caller/skill. Both are best-effort/warn-only: a
clean tree produces no extra output, and a `board-checks.sh` failure (change 0144) emits
`health checks failed <exit>` on stdout — its findings, if any, are still printed above that line —
but either way the pass still continues and never aborts.
Also forwards `--adrs-dir` and `--terminal-publish` (change 0117) to arm the `adr-unpublished`
check, but only when its ADR directory actually **exists** under the metadata worktree — `ADRS_DIR`
always has a value (`docket-config.sh` defaults it to `docs/adrs`), so a fresh repo without one
would otherwise pass a nonexistent path and lose every health check at once, not just this one.
`--terminal-publish` is gated further, on both caller-side legs required by spec §4.4: the
`terminal_publish: true` config knob AND docket-mode (`metadata_branch: docket`) — in main-mode the
metadata and integration refs coincide, so the comparison would be vacuous.

### The `aborted-run` GitHub enrichment leg (change 0219)

Full path only — never under `--board-only`. After the git-only health findings of step 7 are
printed and before the reclaim gate of step 7a, `detect_orphan_pr` resolves the ambiguity
`board-checks.sh`'s `aborted-run` **leg C** leaves behind.

**Gate — leg C's own, mirrored predicate for predicate rather than re-tuned.** A change under
`active/` with `status: in-progress`, an empty `pr:`, and a branch that is **both** idle past
**2 hours** (`ORPHAN_PR_IDLE_SECS`, the same value as `board-checks.sh`'s `ABORTED_RUN_IDLE_SECS`,
kept in sync by value — the two scripts share no library and `board-checks.sh` must stay
independently runnable) **and ahead of at least one integration base**. The branch is `branch:` when
set, else `feat/<slug>`, resolved as `refs/heads/<branch>` then `refs/remotes/origin/<branch>`; an
unresolvable branch is **silence**, never a finding.

The ahead-of-bases half is not optional decoration on the floor — it is what makes "reuse leg C's
gate" true. Bases are `refs/heads/<integration_branch>` and `refs/remotes/origin/<integration_branch>`,
each `show-ref`-verified, and **no base resolving at all is silence**, never "ahead of nothing".
Without it, a run that died before its first commit leaves a branch whose tip *is* the base commit,
whose date is almost always past the floor — so the leg would fire on the nothing-built signature
that belongs to **leg B** and that leg C deliberately stays silent about. `INTEGRATION_BRANCH` comes
from the config resolved by `docket_preflight`.

The query is **one batched call for the whole candidate set** —
`gh pr list --repo <repo> --state open --json number,headRefName --limit 200` — whose result is
matched to each candidate locally by `headRefName`. This leg shares the full-path pass with
`detect_merged`, which is batched for exactly this reason, so its network cost is **O(1) per pass**
and not O(candidates): a per-candidate query made a backlog drain with several in-progress changes
the slowest thing in the pass. `<repo>` is `--repo` when given and `gh repo view`'s `owner/name`
otherwise — the same resolution `detect_merged` performs. Passing it explicitly is what keeps this
leg on the **same repository as the rest of the pass**: without `--repo`, `gh` infers the repository
from the process CWD, which a `--repo` invocation (forwarded by `board_pass` and `github-mirror.sh`)
is precisely saying not to trust.

`--limit 200` because `gh`'s default of 30 would silently truncate a busy repository, and a
truncated listing does not read as "no PR" — it reads as the *wrong message arm* below. 200 is two
100-item API pages inside the one invocation, against an open-PR count bounded by a repo's in-flight
changes. A listing that comes back **at** the ceiling is treated as possibly truncated and skips the
leg (see `pr-list-truncated`) rather than guessing.

**Three outcomes, three remedies**, all rendered as `check aborted-run <id> <message>` — the same
shape `health_checks` prints, so consumers read one vocabulary:

- an open PR exists on the branch → `PR #<n> is open on <branch> but pr: is unset — record it`
- no open PR, and the branch **is** on the remote (`refs/remotes/origin/<branch>` exists, the same
  probe leg C splits its own two messages on) → `<branch> is pushed (last commit Nh ago) but no PR
  on GitHub — the run stopped before opening one`
- no open PR and no remote-tracking ref → `<branch> was never pushed (last commit Nh ago) and has no
  PR on GitHub — the run stopped before pushing it`. "Pushed" is a claim about the remote and is
  never asserted for a branch resolved purely locally; the missing push names the earlier seam a
  human has to act on first.

Unlike leg C's messages these do **not** hedge about the PR's existence — this leg has asked GitHub,
so it states what it found. The remedy stays a bookkeeping act on the manifest and never a push or a
merge: acting on the branch would race a run that is merely between commits. Advisory like every
`aborted-run` leg — it flips no status, releases no claim, and writes no file.

**Best-effort, verbatim `detect_merged`'s posture — but not its token.** Any gh/network/parse failure
emits `orphan-pr-skipped <reason>` and returns 0; it never aborts the pass. The prefix is this leg's
own on purpose: `sweep-skipped` is `detect_merged`'s machine contract with `sweep_execute` and means
"the merge sweep did not run", so reusing it here would have made an advisory enrichment skip read
as a sweep skip in the same pass log. The reasons are `gh-unavailable`
(a `gh` that exits non-zero, and equally a `gh` that is not installed at all — the common offline
case), `repo-unresolved` (`--repo` unset *and* `gh repo view` returning something that is not
`owner/name` — validated by the same owner/name split `detect_merged` uses), `gh-unparseable`
(a `gh` that exits 0 and prints something `jq` cannot parse), and `pr-list-truncated` (the listing
came back at the `--limit` ceiling, so a candidate with no match can no longer be distinguished from
one whose PR fell off the page). Because the listing is **one batched response for the whole
candidate set**, both of the latter are *global*: the one response was all the evidence there was,
so the leg goes quiet entirely rather than skipping a single change. A repo with no candidate change
pays nothing — not even a
`gh repo view`. This is what keeps `board-checks.sh`'s offline guarantee intact: offline, the
git-only check keeps emitting leg C's finding and only the enrichment goes quiet.

**Deliberately not folded into `health_checks`' output blob.** `reclaim_pass` keys a *mutating* gate
on that blob (`RECLAIMABLE_LINE_RE`); widening what feeds it with network-derived lines would put a
remote service inside a local mutation's trigger.

**7a. Reclaim pass — opt-in mutation OR a state-valid remedy (change 0089). Full path only.** After
the health-check lines are captured and printed, `reclaim_pass` keys on the stable `[reclaimable]`
marker `board-checks.sh` stamps on the expired-lease-**and**-no-branch finding — the one case
reclaim is provably collision- and orphan-free. The **mutation is gated behind BOTH conditions**: at
least one `[reclaimable]` finding exists **and** `reclaim.auto` (`RECLAIM_AUTO`) is `true`. If no
`[reclaimable]` finding is present, this pass is a total no-op regardless of `reclaim.auto`.
- **`reclaim.auto: true`** — invokes `reclaim-claims.sh --changes-dir <metadata-worktree>/<changes-dir>
  --lease-ttl-hours "${RECLAIM_LEASE_TTL:-72}"` (the mutating sweep: it flips expired, branch-less
  in-progress changes back to build-ready `proposed`, commits, and **pushes to origin**) and passes
  each of its report lines through prefixed `reclaim `. The metadata worktree is resolved by the
  **same** `docket_metadata_worktree` helper the health checks use, so reclaim runs against exactly
  the worktree the findings came from; single-clone safety comes from the guard's LOCAL
  `refs/heads/feat/<slug>` arm (always present in this clone) — `docket_preflight` fetches only
  `origin/<metadata_branch>`, never `origin/feat/*`. The genuine cross-machine unfetched-remote-ref
  case is the documented §7-H residual (`reclaim-claims.md`), contained by lease expiry plus
  `reclaim.auto`'s default-off.
- **`reclaim.auto: false` (the default)** — prints **one** state-valid remedy line,
  `reclaim: <n> expired-lease change(s) can self-heal — run: docket.sh reclaim-claims`, where `<n>`
  is the `[reclaimable]` finding count. **printed-remedy-state-validity:** the remedy is keyed on the
  SAME condition that gates the write, so the command it names is valid in exactly the state that
  produced it; it is **never** printed under `reclaim.auto: true` (reclaim just ran).

Neither the mutation nor the remedy can fire under `--board-only`: that path early-exits after the
board and backlog passes (step 4), long before health checks or `reclaim_pass` are ever reached.
The `[reclaimable]` test is a capture-then-grep on a here-string (never `health_checks | grep -q`),
honoring the no-pipefail-SIGPIPE rule.

**8. Learnings pass (change 0067).** Runs `learnings_pass` — the learnings-index self-heal +
two needs-you advisories — **only on the full path, never under `--board-only`** (the board's own
dedicated entry point; adding unrelated learnings work to it would be wrong). It runs after step 7
(health checks/judgment) and before the full-pass backlog digest (step 4's post-sweep firing), so
report line order on a full pass is board → sweep lines → check lines → reclaim lines → judgment
lines → **learnings lines** → backlog digest → `pass ok`.

- **Gate, checked FIRST, before anything else in this step:** `learnings.enabled` (`LEARNINGS_ENABLED`,
  from `docket-config.sh`). When not `true`, the pass emits the single positive line `learnings
  disabled` and returns — **the renderer is never invoked**, `learnings/` is never read or
  written, and no advisories are computed. Deliberate positive evidence, not silence (the same
  ADR-0028 lesson as `board off`/the backlog digest).
- **No learnings directory.** When enabled but `<changes-dir>/learnings` does not exist, emits
  `learnings index skipped (no learnings dir)` and returns — there are no finding files to render
  or advise on, so skipping advisories here too is correct (this is the one early-return where
  that's true).
- **Index self-heal render.** Otherwise renders `<learnings-dir>/README.md` in place via
  `render-learnings-index.sh` through the same atomic-write discipline as `board-refresh.sh`
  (temp file on the same filesystem, non-empty check, `chmod 644`, rename) — the last-known-good
  index is never truncated by a failed render. A render failure emits `learnings index failed`
  and does **not** abort the pass. On success, commit + push uses the **same shared write-decision
  helper as the board pass** (`commit_and_push_generated`, factored out in change 0067): commit
  only if the render actually changed bytes, then push with the same bounded rebase-retry loop,
  regenerating through `render-learnings-index.sh` (never a raw redirect or hand-merge) if a
  rebase conflict touches the index. Reports one of `learnings index clean` / `learnings index
  changed pushed` / `learnings index changed push-failed`, with the identical "clean tree is not
  evidence of a push" discipline as `board inline clean` (change 0071 review, finding 3): a prior
  run's locally-committed-but-unpushed index still falls through into the push/retry loop rather
  than being reported clean.
- **Advisories — fire independently of the render outcome (change 0067 review, finding 3).**
  After the render step (success **or** failure — everywhere except the "no learnings dir" and
  "disabled" returns above), `learnings_advisories` scans every `<learnings-dir>/*.md` file
  (excluding `README.md`) and reads each one's `promotion_state` through the frontmatter lib
  (never a bare grep, which cannot distinguish a `promotion_state: candidate` line from war-story
  prose that happens to contain the word). A finding with no `promotion_state` defaults to
  `retained`. **Cap counts ACTIVE findings only** — `retained` + `candidate`, never `promoted` (a
  promoted finding is exactly what the shrink valve removes from the count):
  - `learnings over-cap — needs curation (<n> active, cap <n>)` when active count exceeds
    `LEARNINGS_CAP`.
  - `learnings promotion-pending <n> — needs you` when at least one active finding has
    `promotion_state: candidate`.
  A broken renderer must not also mute these two needs-you channels — that would silence the
  escalation precisely when something is already wrong — so they are computed from the finding
  files, not gated on the render's own success.

**9. Integration sync.** If step 6 swept at least one change (`swept ` line count ≥ 1), runs
`sync-integration-branch.sh --integration-branch "$INTEGRATION_BRANCH"` once, best-effort
(failures are swallowed). Skipped entirely when nothing was swept. Runs after the full-pass
backlog digest and emits nothing on stdout, so it does not affect the report's line order.

### Failure postures (summary)

- **Board pass: best-effort, but never silent.** A failed inline render or failed github mirror
  never aborts the pass; it degrades to a diagnostic on stderr and (for inline) leaves the
  last-known-good `BOARD.md` in place — but every path, including a failed render and an unknown
  surface token, also emits a positive `board …` stdout line (`board inline failed`,
  `board <token> unknown`), never just the stderr diagnostic (change 0071 review, finding 1). A
  caller that sees the pass exit 0 with **no** `board …` line at all has found a bug in this
  script, not evidence the board landed.
- **Sweep: per-change log-and-continue.** A failed step for one change emits `sweep-failed` and
  abandons only the rest of *that* change's close-out (except a cleanup failure and an
  artifacts-refresh `commit-failed`/`push-failed`/`blocked-wedged-tree`, which report and continue, still emitting
  `swept`/`harvest`); the sweep loop proceeds to the next change regardless. The close-out is never
  abandoned for a cosmetic reason: publishing the terminal record and tearing down the branch +
  worktree outrank a stale link block (change 0075).
- **Health checks: warn-only.** Findings and judgments are reported, never enforced; the script
  never modifies a change file or blocks the pass because of a finding.
- **Reclaim pass (step 7a): opt-in and doubly-gated.** The only step that mutates a change file and
  pushes to origin on a health finding — and it does so **only** when both a `[reclaimable]` finding
  exists **and** `reclaim.auto: true`. With `reclaim.auto: false` (the default) it prints a
  remedy and touches nothing; with no `[reclaimable]` finding it is a total no-op regardless of the
  knob; and it never runs under `--board-only`. `reclaim-claims.sh` owns the actual reclaim
  mechanics (eligibility, the CAS push loop) — this script only gates and forwards.
- **Learnings pass: best-effort, but never silent, and never blind to a broken renderer.** Gated
  FIRST on `learnings.enabled` — disabled emits exactly one `learnings disabled` line and touches
  nothing. Enabled, a failed index render emits `learnings index failed` and never aborts the pass
  (the last-known-good `README.md`, if any, is left untouched) — but the two needs-you advisories
  (`learnings over-cap`, `learnings promotion-pending`) still fire on that path, since they are
  computed from the finding files, not from the render outcome (change 0067 review, finding 3): a
  broken renderer must not also mute the escalation channels precisely when something is already
  wrong. Only the `disabled` and `no learnings dir` returns skip the advisories — the former
  because the gate short-circuits everything, the latter because there are no finding files to
  advise on.

## Output contract

All report lines are stdout, one shape per line, diagnostics go to stderr:

| Shape | Meaning |
|---|---|
| `board inline clean` | Inline render matched the existing `BOARD.md` AND there is nothing unpushed touching it — no local commit on `BOARD.md` sits ahead of its upstream. Attests the board is caught up on the remote, not merely that the working tree is clean. |
| `board inline changed pushed` | `BOARD.md` changed and the commit was pushed successfully. |
| `board inline changed push-failed` | `BOARD.md` changed and committed locally, but push retries were exhausted or a rebase conflict outside `BOARD.md` forced an abort. Unchanged by change 0247, and still the **sole retryable** board outcome — the new `blocked-wedged-tree` token below was added beside it, never split out of it. |
| `board inline blocked-wedged-tree` | The shared metadata worktree has a rebase or merge in progress, so the board pass pushed **nothing** — and committed nothing either when the pre-commit probe fired, though the in-loop probe (a wedge opening after that first probe) may leave an unpushed local commit behind (change 0247). Distinct from `changed push-failed` and deliberately **not** retryable: committing into a mid-rebase tree writes onto that rebase's detached HEAD, and the push-retry loop's own `rebase --abort` would destroy another agent's in-flight work. `--must-land` treats it as **not landed** (non-zero exit → the autonomous caller STOPs and abort-reports); a flagless best-effort caller logs it and continues. Clearing it is a human act — finish or abort the in-progress operation. Emitted from **either** layer: `commit_and_push_generated`'s own probe when the wedge appeared mid-pass, or the Step-0 wedged-tree carve-out (see Behavior, steps 1–2) when it was already there — same line, and on the Step-0 path no other pass runs and no `pass ok` follows. |
| `board github ok` | `github-mirror.sh` exited 0. |
| `board github failed` | `github-mirror.sh` exited non-zero. |
| `board off` | `BOARD_SURFACES` is the reserved token `none` — the board is deliberately disabled (`board_surfaces: []`); no surface was rendered and nothing was committed. Positive evidence of a deliberate skip, never silence. |
| `board inline failed` | The `inline` render failed; the existing `BOARD.md` was left untouched (best-effort — the pass still continues to `pass ok`). Terminal, not retryable (change 0071 review, finding 1). |
| `board <token> unknown` | `<token>` in `BOARD_SURFACES` matched neither `inline`, `github`, nor `none`; warned on stderr and ignored. Terminal, not retryable (change 0071 review, finding 1). |
| `minted issue <id> <n>` | Passthrough of `github-mirror.sh`'s `issue-minted <id> <n>`. |
| `minted project <id> <n>` | Passthrough of `github-mirror.sh`'s `project-minted <id> <n>`. |
| `backlog <status> <count>` | One rollup per non-zero status across the active + archived change files (from the ungated backlog pass). On a full pass these are **post-sweep** counts: a change swept this pass is counted under `done`, not `implemented`. |
| `change <id> <status> <readiness> <slug>` | One line per **active** change, as of the moment the backlog pass ran (post-sweep on a full pass, so a change swept this pass has no `change` line at all — it is archived). `<readiness>` is `build-ready`, `needs-brainstorm`, `auto-groom-blocked`, `stack-base-unresolved` (a `stacked_on:` change whose effective base does not resolve — change 0298), `waiting-on-<N>-unbuilt`, or `waiting-on-<N>-needs-merge` for a `proposed` change; `finalize-blocked` for an `implemented` change carrying the `## Finalize blocked` section (this pass pipes render-board's digest through unmodified, so the token reaches the report); and `-` for everything else — an `implemented` change *without* the marker, plus every change in any other status (where readiness does not apply). |
| `ready [<id> …]` | The **build-ready queue in selection order** (`priority` → `created` → `id`), from `render-board.sh`'s digest (change 0094). Emitted on every path that runs the backlog pass — the full report, `--board-only`, and `--digest-only` — **when that pass succeeds**. Present and bare when nothing is build-ready; its absence means the pass did not reach the backlog digest at all (config export failure, a fail-closed bootstrap/changes-dir gate, an older `render-board.sh`, or a failed render), never "nothing is ready". A caller must check the exit code before treating a missing `ready` line as "no candidates" — see Exit codes. Membership always equals the `change` lines reporting `proposed build-ready`. |
| `swept <id> <date>` | Change `<id>` fully closed out (archived, links refreshed, terminal record published only if the repo opted in with `terminal_publish: true`, branch cleaned up) as of `<date>` (UTC, from merge). |
| `stacked-merged <id> <parent>` | Change `<id>`'s PR merged into change `<parent>`'s branch rather than into the integration branch, so its code is **not** reachable from that branch and it is not `done` (change 0298). Its `status:` was flipped to `stacked-merged` in place and committed on `metadata_branch`; the change stays in `active/`, nothing was archived or published, and its feature branch was **not** deleted. Deliberately distinct from `swept <id> <date>` — a caller must be able to tell a close-out from a stack-parent merge, and this pass emits no `harvest` line for it. |
| `promoted <id>` | Relayed verbatim from `stack-closeout.sh` (step 6-closeout, change 0298): descendant `<id>` of a root swept this pass was at `stacked-merged`, so the root's merge made its code reachable from the integration branch and it went through the full terminal close-out to `done`, archived under the **root's** merge date. |
| `promote-skipped <id> <reason>` | Relayed from `stack-closeout.sh`: descendant `<id>` needed no promotion this pass — `already-archived`, `not-stacked-merged`, or `change-file-missing`. An ordinary, silent-by-design outcome of an idempotent re-run, not a failure. |
| `promote-failed <id> <reason>` | Relayed from `stack-closeout.sh`: a step of descendant `<id>`'s close-out failed (`archive`, `archived-file-not-found`, `render-change-links`, `terminal-publish`). Its siblings still ran — each promotion is independently re-runnable — and the next pass resumes this one. |
| `stack-carried <root> <count>` | Relayed from `stack-closeout.sh`: the root's archived record's marker-bounded **Stack carried** table was regenerated over `<count>` descendants (committed only if it changed). |
| `stack-carried-failed <root> <reason>` | Relayed from `stack-closeout.sh`: the table was not written — `root-not-archived`, `markers-unbalanced`, `render-failed`, `commit-failed`, or `push-failed`. The promotions above it stand; only the table is missing, and it is regenerated on the next pass. |
| `sweep-failed <id> stack-closeout script-error` | The stack close-out for root `<id>` could not run at all (step 6-closeout). Log-and-continue, like every other sweep failure: the pass moves to the next root and the rest of the run is unaffected. The root itself is already `done` and archived by then, and the close-out is idempotent, so the next pass re-runs it unchanged. |
| `harvest <id> <path>` | The archived file path for a swept change — a hook for the caller to harvest learnings. `<path>` is absolute (since change 0075, anchored to the main worktree via `lib/docket-root.sh`) — previously relative to the process CWD. |
| `sweep-failed <id> stacked-merged <reason>` | The stacked-merge gate fired but could not land the flip. All four reasons self-heal, by two different routes. `blocked-wedged-tree` (the shared metadata worktree is mid-rebase/merge, so nothing was written), `write-failed` (the frontmatter edit failed), and `commit-failed` (the path was restored to HEAD, leaving the shared worktree clean) all leave the change `implemented`, so the next pass re-detects the same merged PR and retries the whole gate. `push-failed` does **not**: the local commit is deliberately **retained**, so the flip has already landed locally and the next pass's `pull --rebase` carries it to the remote. Never reset that commit — doing so re-opens the flip and re-reports it forever. |
| `sweep-failed <id> <step> <reason>` | Step `<step>` (`sync`, `archive`, `render-change-links`, `terminal-publish`, `stacked-merged`, `stack-closeout`, or `cleanup`) failed for change `<id>` with `<reason>`; that change's remaining close-out steps were abandoned — **except** for `cleanup` and for the artifacts-refresh reasons `commit-failed` / `push-failed` / `blocked-wedged-tree` (step 6a), after which the close-out continues and the change still reports `swept`/`harvest`. |
| `sweep-failed <id> render-change-links commit-failed\|push-failed` | The refreshed `## Artifacts` block could not be committed/pushed on `metadata_branch` (step 6a). Cosmetic and non-terminal: `terminal-publish.sh` and `cleanup-feature-branch.sh` **still ran**, and the change is still reported `swept`. The archived record on `metadata_branch` keeps its previous link block until a manual re-render. |
| `sweep-failed <id> render-change-links blocked-wedged-tree` | Step 6a found the shared metadata worktree mid-rebase/merge and committed **nothing** (change 0247). **Report-and-continue**, exactly like `commit-failed`: `terminal-publish.sh` and `cleanup-feature-branch.sh` still ran and the change is still reported `swept`. The `## Artifacts` block self-heals on the next pass once a human has cleared the operation. |
| `sweep-skipped <reason>` | Batched **merge** detection itself was skipped (`gh-unavailable` or `repo-unresolved`); no changes were evaluated this pass. Emitted only by `detect_merged` (and passed through `sweep_execute`) — never by the `aborted-run` enrichment, which has its own token below. |
| `orphan-pr-skipped <reason>` | The `aborted-run` GitHub enrichment (`detect_orphan_pr`, full path only) was skipped: `gh-unavailable`, `repo-unresolved`, `gh-unparseable`, or `pr-list-truncated`. Advisory and global — no candidate was enriched this pass, the git-only `aborted-run` findings are unaffected, and the pass continues normally. |
| `check <check-id> <change-id> <message>` | One `board-checks.sh` finding, passed through with the `check` prefix. `<check-id>` ∈ {aborted-run, adr-unpublished, board-row-dropped, broken-spec, broken-plan-results, dep-cycle, field-domain, scalar-form, stack-invalid, stack-parent-killed, publish-deferred, stale-in-progress, stale-finalize-blocked, merge-gate-stall, merged-orphan, unknown-commit-ref, malformed-id}. |
| `judgment blocked <id> <text>` | Change `<id>` is `status: blocked`, with its `blocked_by:` text, for the caller to re-judge. |
| `reclaim: <n> expired-lease change(s) can self-heal — run: docket.sh reclaim-claims` | Step 7a, `reclaim.auto: false`: `<n>` `[reclaimable]` findings (expired lease, no branch) can be reclaimed. A state-valid remedy — printed only when at least one such finding exists, never under `reclaim.auto: true`, never under `--board-only`. |
| `reclaim <line>` | Step 7a, `reclaim.auto: true`: a passthrough of one `reclaim-claims.sh` report line (`reclaimed <id> <slug> …` / `skipped <id> raced`) after the mutating reclaim sweep ran and **pushed to origin**. Emitted only on the full path when a `[reclaimable]` finding exists. |
| `learnings disabled` | `learnings.enabled` resolved `false` — the pass is a total no-op: no render, no advisories, no read or write of `learnings/` at all. |
| `learnings index skipped (no learnings dir)` | Learnings enabled, but `<changes-dir>/learnings` does not exist — nothing to render and no finding files to advise on. |
| `health checks failed <exit>` | The health pass's `board-checks.sh` invocation exited non-zero (`<exit>` is its status); its findings, if any, are still printed above this line. A **health-pass** line, deliberately outside the `board ` family — the cause stays on stderr. Warn-only: the pass still continues to `pass ok`. |
| `learnings index failed` | The learnings index render failed; the existing `README.md` (if any) was left untouched (best-effort — the pass still continues). The two advisory lines below still fire on this path (change 0067 review, finding 3). |
| `learnings index clean` | The rendered index matched the existing `README.md` AND there is nothing unpushed touching it — the same two-part attestation as `board inline clean`. |
| `learnings index changed pushed` | The learnings index changed and the commit was pushed successfully. |
| `learnings index changed push-failed` | The learnings index changed and committed locally, but push retries were exhausted or a rebase conflict outside the index forced an abort. Unchanged by change 0247 — still the sole retryable learnings outcome. |
| `learnings index blocked-wedged-tree` | As `board inline blocked-wedged-tree`, for the learnings-index pass: the shared metadata worktree was mid-rebase/merge, so nothing was pushed — and nothing committed when the pre-commit probe fired, while the in-loop probe may leave an unpushed local commit. Not retryable; the pass continues best-effort and the index self-heals on the next pass once a human has cleared the operation. Emitted from either layer, as the board line above — and never under `--board-only`, which runs no learnings pass at all. |
| `learnings over-cap — needs curation (<n> active, cap <n>)` | Active findings (`retained` + `candidate`, `promoted` excluded) exceed `learnings.cap` — needs human curation. Emitted whenever the render succeeded, failed, or was clean — never gated on the render outcome. |
| `learnings promotion-pending <n> — needs you` | `<n>` active findings carry `promotion_state: candidate` — needs a human promotion decision. Same independence from the render outcome as the over-cap line above. |
| `pass ok` | The orchestrator ran to completion. Always the last line of a successful pass; **stdout is never empty**. A hard error exits non-zero and never prints it, so it is a reliable completion signal — read it as the completion marker, not the exit code, since the two paths that exit 0 without completing a pass (`--digest-only` and the wedged-tree carve-out, both under Exit codes) are told apart from a real pass by this line's absence and nothing else. |

## Exit codes

- `0` — the pass completed (and printed `pass ok` as its last line). Findings, `sweep-failed`,
  `sweep-skipped`, `orphan-pr-skipped`, `board *-failed`, `board off`, and `judgment` lines on
  stdout are all normal,
  expected pass outcomes, not errors — **a thin report is the success case.** Two exceptions print
  **no** `pass ok` and still exit 0: `--digest-only` (change 0094), which exits 0 **only when the
  digest actually reaches stdout** — it is a read, not a pass, so that completion marker does not
  apply to it — and the flagless wedged-tree carve-out (change 0247), whose stdout is the
  `blocked-wedged-tree` line alone because no pass ran. On both, `pass ok`'s absence is the signal:
  exit 0 means "no hard error", never "the pass completed".
- non-zero — a hard error only: config export failure, an unrecognized `BOOTSTRAP` verdict,
  `STOP_MIGRATE`/`CREATE_ORPHAN` bootstrap gate (exit 1 — including on a wedged tree, which never
  softens it), an unusable metadata worktree (create or sync failure, exit 1, **except** the
  wedged-tree carve-out above, which exits 1 only under `--must-land`), an unknown CLI argument
  (exit 2), or `BOARD_SURFACES` was empty (or
  whitespace-only — defence-in-depth, change 0071 review finding 6) / `none` was combined with
  another surface (a wiring bug — change 0071). Under `--digest-only` specifically (exit 1, no
  digest on stdout, diagnostic on stderr): config export failure, a non-`PROCEED` bootstrap
  verdict, or a metadata worktree whose changes dir does not exist (the fresh-clone gap — nothing
  else on this path guarantees the worktree was ever materialized, since it skips
  `docket_preflight`); also exit 2 (before any of the above run) when combined with `--board-only`
  or with `--must-land`, in either order.
- non-zero — under `--must-land`, the board pass ended on a non-success terminal line or exhausted
  its 3 retries. `board inline blocked-wedged-tree` is one such **terminal** line (change 0247): it
  is classified `failed`, never `retryable`, so it exits non-zero immediately rather than spending
  the retry budget. Retrying it would be pure latency — a rebase or merge in progress in the shared
  metadata worktree clears only when a human finishes or aborts it. `changed push-failed` keeps its
  exact prior meaning as the one retryable outcome; the new token was added beside it, not split
  out of it.

## Invariants

- **Determinism.** Archive commits touch only the change file being archived. All dates are UTC,
  taken from GitHub's `mergedAt`, never `now()` or local time. Two runs over the same change files
  produce byte-identical `BOARD.md` output (inherited from `render-board.sh`'s determinism).
  Concurrent runs converge: a losing writer's push failure triggers a rebase-and-regenerate retry
  rather than a hand merge, and idempotent no-ops (already-archived, already-terminal changes)
  make repeated sweeps safe.
- **No duplication of shared-script internals.** This script only sequences the shared scripts and
  translates/prefixes their output lines; it does not reimplement rendering, archiving, health
  checks, reclaim, or publishing logic that already lives in `render-board.sh`, `archive-change.sh`,
  `render-change-links.sh`, `terminal-publish.sh`, `cleanup-feature-branch.sh`,
  `board-checks.sh`, `reclaim-claims.sh`, or `render-learnings-index.sh`.
- **Surface-specific failure postures.** See Failure postures above: board best-effort, sweep
  per-change log-and-continue, health checks warn-only, learnings best-effort. No single surface's failure aborts another
  surface's work within the same pass.
- **Mock seams.** `GIT="${GIT:-git}"`, `GH="${GH:-gh}"`, `NOW="${NOW:-$(date +%s)}"` (the staleness
  clock, spelled exactly as `board-checks.sh`'s so both suites drive their clocks the same way; read
  by `detect_orphan_pr`'s idle floor), `SCRIPTS_DIR="${SCRIPTS_DIR:-$SELF_DIR}"`
  (where the chained scripts are looked up), and `CONFIG_EXPORT_CMD` (overrides the
  `docket-config.sh --export` call) — all overridable in tests for hermetic runs. The shared
  `docket_preflight` (`scripts/lib/docket-preflight.sh`) honors the same `GIT` and
  `CONFIG_EXPORT_CMD` seams.
