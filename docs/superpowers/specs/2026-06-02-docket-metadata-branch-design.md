# Design: the `docket` metadata branch — separate planning state from code history

**Status:** design (brainstormed 2026-06-02)
**Change:** 0002
**Supersedes the v1 rough edge:** closes the deliberately-deferred `metadata_branch: docket` gap documented in `docket-implement-next/SKILL.md` ("`docket` mode caveat") and `README.md` ("How metadata is stored").

---

## 1. Context

docket needs a durable, queryable source of truth for planning state — changes, statuses, ADRs, dependencies, board — shared across agents, machines, and time, with **git as the only persistence mechanism** (no database, no service).

The shipped v1 stores that state directly on the integration branch (`main`). This guarantees freshness (every feature branch is cut from a branch that already carries the latest planning state) but **pollutes code history**: a continuous stream of planning-only commits (`claim`, `reconcile`, `refresh board`, `archive`) interleaves with production code, mixing project-management churn into the branch people read to understand the software.

A second branch already exists in name: `metadata_branch: docket` is a config knob in every skill's convention block, and the README documents it — but it is shipped as an explicit **rough edge**. `docket-implement-next` tells implementers *not* to use it ("encounter silent failures"). The README's *How metadata is stored* names the three unsolved sub-problems verbatim:

1. spec files live on `docket` and must be read cross-tree during implementation;
2. reconcile pushes land on `docket` but the feature branch still cuts from `origin/main`;
3. there is **no automatic mirror or merge from `docket` into `main`**.

This change closes that rough edge and makes `docket`-mode the **default, supported, tested** path.

## 2. Goals / Non-goals

**Goals**
- A dedicated `docket` branch is the authoritative working surface for *all* planning state; `main` (the integration branch) stays code-only except for **published terminal records**.
- Clean code history; complete git-native audit trail; no database.
- Works for trunk-based (`main`) **and** GitFlow (`develop`) integration lines.
- `docket`-mode becomes the **literal default** (absent `.docket.yml` ⇒ `docket`), with a **safe** first-run bootstrap that never strands existing metadata.
- The whole backlog/board/specs/ADRs are browsable and reviewable **on the remote** (GitHub) at all times.

**Non-goals**
- Migrating *this* repo (docket itself) to `docket`-mode. We build + document the capability; dogfooding the migration is a **separate follow-up change**.
- Migrating other repos' historical metadata beyond what the one-shot migration script does.
- A living-spec layer or any CLI runtime dependency. (A one-shot setup *script* is allowed — see §9 — and matches the existing `sync-convention.sh` / `link-skills.sh` pattern.)
- Rewriting existing `main` history (the planning commits already on `main` stay; only the go-forward surface changes).

## 3. The two-branch model

```
  docket  (orphan, metadata-only, ALWAYS pushed)
    └─ docs/changes/{active,archive}, BOARD.md, docs/adrs/, docs/superpowers/specs/
       (NO docs/results/ and NO docs/superpowers/plans/ — those are feature-branch build artifacts)
       all churn: proposed → in-progress → implemented, board refreshes, reconcile edits, ADRs

  <integration_branch>  (main | develop)   — code + build artifacts + PUBLISHED TERMINAL RECORDS (no planning churn)
    └─ code + (via PR) plan + results ;  (via §7.7 terminal-publish) archived change + spec + ADRs

  feat/<slug>  — cut from origin/<integration_branch> ; carries ONLY plan + results + code ;
                 NEVER touches docket metadata
```

### Artifact-location table (the questions raised at brainstorm)

| Artifact | Authored / lives on | How it reaches `<integration_branch>` | On `<integration_branch>` after a terminal change? |
|---|---|---|---|
| change file (manifest + body) | `docket` | **terminal-publish copy** (§7.7) † | yes (archived) |
| spec (`docs/superpowers/specs/…`) | `docket` | **terminal-publish copy** (§7.7) † | yes |
| ADR (`docs/adrs/…`) | `docket` | **terminal-publish copy** (§7.7), gated on **`Accepted`** — change-tied via its change, standalone/superseded via `docket-adr` | yes (the `Accepted` ADRs in the manifest's `adrs:`, plus standalone `Accepted` ADRs) |
| `BOARD.md` | `docket` | **never** (live planning view) | no — view it on `docket` |
| plan (`docs/superpowers/plans/…`) | feature branch | the **PR merge** | yes (`done` only) |
| results (`docs/results/…`) | feature branch | the **PR merge** | yes (`done` only) |
| code | feature branch | the **PR merge** | yes (`done` only) |
| `.docket.yml` | **default branch** (`origin/HEAD`) | n/a — lives on the default branch, not routed to the integration branch | only if default branch == integration branch (trunk mode); under GitFlow it's on `main`, not `develop` |

† The terminal-publish copy runs on **both** terminal transitions: `done` (owned by `docket-finalize-change`) and `killed` (owned by the killing skill — `docket-new-change` from `proposed`, `docket-implement-next` from `in-progress`). See §7.7.

**Why spec is copied but plan/results are not:** the copy step only moves files that *live on `docket`* and would otherwise be stranded there. The plan and results files live on the **feature branch** (they are build artifacts) and reach the integration branch **through the PR merge** — they were never on `docket`. So the integration branch ends up with all five artifacts + code; they simply arrive by two paths. A `killed` change usually has **no merged PR**, so plan/results may not exist — kill publishes only what is on `docket`: the change file (always), its `spec:` if set, and the ADRs in the manifest's `adrs:` (if any).

## 4. Configuration — `.docket.yml`

```yaml
# .docket.yml — committed on the repo's DEFAULT branch (origin/HEAD); read by every docket skill at startup
metadata_branch: docket       # docket (default) | main  — where PM commits land
integration_branch: auto      # auto (default → origin/HEAD, fallback main) | main | develop  — where code lands; feature branches cut from origin/<this>
changes_dir: docs/changes     # default
adrs_dir: docs/adrs           # default
results_dir: docs/results     # default
```

- **`metadata_branch`** default flips from `main` → **`docket`**.
- **`integration_branch`** is new. **The absent-key default is `auto`.** Resolution: an explicit `main`/`develop` is used verbatim; `auto` (and absent) resolves to the remote's default branch via `git symbolic-ref refs/remotes/origin/HEAD`, falling back to `main` if undetectable. (Note: `auto` follows the repo's *default* branch — a GitFlow repo whose default branch is `main` but whose integration line is `develop` must set `integration_branch: develop` explicitly.) Feature branches always cut from `origin/<integration_branch>`.
- **Backward-compatible opt-out:** pinning `metadata_branch: main` (and `integration_branch: main`) reproduces today's behavior exactly — single-branch mode, no `docket` branch, no `.docket/` worktree.
- **`.docket.yml` lives on the repo's default branch (`origin/HEAD`), NOT on the integration branch.** This resolves a bootstrap paradox: `integration_branch` is a *value read from* `.docket.yml`, so the file cannot be located *by* the integration branch. The default branch is discoverable with zero prior config — **but `origin/HEAD` is not reliably populated** (older git only sets it on `clone`; it can be unset after `remote add`+`fetch`, or dangle after a default-branch rename). So skills **repair it first**: `git remote set-head origin -a` (refreshes `origin/HEAD` from the remote), then resolve `git symbolic-ref refs/remotes/origin/HEAD`. Read config **authoritatively** via `git show origin/HEAD:.docket.yml` (after a fetch); the working-tree copy is trusted only on the default branch's *primary* checkout (a feature-branch or `.docket/` copy may be stale or absent). **Distinguish two "no value" cases:** (a) `origin/HEAD` *resolves* but the file is genuinely absent → **defaults apply** (`metadata_branch: docket`, `integration_branch: auto`); (b) `origin/HEAD` is *unresolvable* or `origin` is unreachable → **do not assume defaults** (silently flipping a `main`-mode repo to `docket`-mode on a transient network failure is dangerous) — abort with a clear error, or fall back to the working-tree copy if on the primary checkout. **The abort must key on the `set-head`/fetch return code, NOT on `git show` failing:** when `origin/HEAD` is already cached locally and `origin` later goes unreachable, `set-head` fails but `git show origin/HEAD:.docket.yml` still *succeeds from the local object store* with possibly-stale bytes — so a check that only watches `show` would not notice the unreachability. The file then *declares* `integration_branch`, which may differ from the default branch (e.g. default `main`, integration `develop`); the orphan `docket` branch does not carry `.docket.yml`, so skills operating in `.docket/` resolve config via `origin/HEAD` regardless.

## 5. The `docket` branch — orphan, always pushed

- Created as an **orphan branch** (`git checkout --orphan docket`): no shared history with the integration branch, metadata-only. Same well-trodden pattern as `gh-pages`. It carries `docs/changes/` (active + archive), `BOARD.md`, `docs/adrs/`, and `docs/superpowers/specs/` — **no code, and no `docs/results/` or `docs/superpowers/plans/`** (those are feature-branch build artifacts that land on the integration branch via the PR merge — see §3). The small static `README.md` blurbs under `changes_dir`/`adrs_dir` link to `BOARD.md`/the ADR index, which live on `docket`, so they ride along on `docket`; the migration seeds them (§9). It therefore never drifts against the integration branch and never needs a code-direction merge.
- **Always-push invariant:** every metadata commit is pushed to `origin/docket` **immediately**. A local-only orphan branch defeats the purpose; the whole backlog, board, specs, and ADRs must be browsable/reviewable on the remote (GitHub) at all times. This generalizes the producer's existing push discipline to *all* skills and *all* metadata commits.

## 6. The metadata worktree — `.docket/`

The constraint: git checks out one branch per folder. To *write* a file that lives on `docket` while the main folder sits on `main`/a feature branch, a skill needs a second folder parked on `docket`. That is a worktree.

- Each skill, at startup in `docket`-mode, **ensures a persistent worktree at `.docket/`** parked on the `docket` branch, then **syncs it to `origin/docket` before any read** — reads and copies must source the *remote* tip, never a stale local ref. The main working tree **never switches branches**.
  - **Ensure (idempotent):** if `git worktree list` already shows `.docket`, **skip creation** (every creation form below `fatal`s when the worktree/branch already exists, and skills run this at every startup). Otherwise create it — the command is **state-specific** (a single `git worktree add .docket docket` fails when the branch does not yet exist):
    - **branch on neither local nor origin** (fresh repo, §7.0): `git worktree add --orphan -b docket .docket`, then `git -C .docket push -u origin docket`.
    - **branch on origin, not local** (a second clone after migration): `git fetch origin docket && git worktree add .docket --track -b docket origin/docket`.
    - **branch already local**: `git worktree add .docket docket`.
  - **Sync:** `git -C .docket fetch origin docket && git -C .docket pull --rebase origin docket` — use the **explicit `origin docket` refspec**, since the "branch already local" form may have no tracking upstream and a bare `pull --rebase` would then abort ("no tracking information").
  All metadata reads and writes then happen in `.docket/`.
- **Location decision: `.docket/`, NOT `.worktrees/docket`.** Reasons:
  1. **Slug collision (a corruption bug):** feature worktrees are `.worktrees/<slug>`; a slug is arbitrary text from a change title, so a change slugged `docket` would collide on `.worktrees/docket`. `.docket/` makes that structurally impossible.
  2. **Lifecycle mismatch:** `.worktrees/` holds *ephemeral* per-change trees that get pruned; the metadata worktree is *permanent, singular infrastructure*.
  3. **Cleanup blast radius:** finalize/janitor logic and humans run worktree pruning over `.worktrees/`; keeping the metadata tree at `.docket/` puts it outside that blast radius.
- `.gitignore` (added by this change / migration) ignores **both** `.docket/` and `.worktrees/`.
- The compare-and-swap push-rebase-retry loops are **unchanged in algorithm** — in `docket`-mode they target `origin/docket` via the `.docket/` worktree, where single-branch mode targets the metadata branch from the primary working tree. **One caveat the relocation sharpens:** `BOARD.md` is a wholly *regenerated* file, so a `pull --rebase` during the loop can hit a hard content conflict when two agents both rewrote it. Resolve by **regeneration, never 3-way merge**: on a `BOARD.md` conflict the conflicted side is immaterial — discard the conflict markers (either side), re-run the Board pass to regenerate `BOARD.md` from scratch, `git add` it, then `git rebase --continue`. (Don't reason about ours/theirs — they invert under rebase vs merge anyway; regeneration overwrites whichever you picked.) (docket already commits `BOARD.md` as its own commit, separate from change-file commits, limiting the blast radius — but the regenerate-don't-merge rule must be explicit since `docket`-mode concentrates more multi-agent churn on one branch.)
- **Unifying abstraction for the skills:** "commit metadata in the *metadata working tree* on `metadata_branch`." In single-branch (`main`) mode the metadata working tree **is the primary working tree, checked out on the integration branch** (`metadata_branch == integration_branch` there — which may be `develop`, not literally `main`); in `docket`-mode it **is** the `.docket/` worktree. Every place the skills today say "in the main working tree on `metadata_branch`" is reworded to "in the metadata working tree" — where "main working tree" carried git's sense of *the primary checkout*, not the branch named `main`.

## 7. Per-skill behavior

### 7.0 Shared: the bootstrap guard (in the synced convention block)
At startup, after resolving config, with `metadata_branch == docket`, the guard fetches origin and then evaluates a 2×2 over two probes, **both stated over the same vocabulary** so the matrix is verifiable:
- **`DOCKET`** = the `docket` branch exists (on origin OR locally) — `git rev-parse --verify --quiet refs/remotes/origin/docket || git rev-parse --verify --quiet refs/heads/docket`.
- **`LIVE`** = the **live planning surface** is still present on the integration branch — probed with `git ls-tree origin/<integration_branch> -- <changes_dir>/active <changes_dir>/README.md BOARD.md` (non-empty output ⇒ present). **The probe set must equal the migration's _prune_ set (§9 step 3), NOT its seed set** — migration deliberately *keeps* `<changes_dir>/archive/`, `<adrs_dir>/`, and the pre-migration specs on the integration branch, so probing those would make a correctly-migrated repo read `LIVE` forever and STOP on every run. Only the pruned live surface (`active/`, the changes `README.md`, `BOARD.md`) signals an un-migrated repo. **Use `git ls-tree`, never bare `<ref>:<path>`** (the latter misreports when a path also exists in the working dir). **Distinguish exit≠0 from empty output:** if `origin/<integration_branch>` does not resolve (mis-set `integration_branch`, integration branch never pushed), `ls-tree` exits 128 with empty stdout — treat that as a **hard config error**, not `¬LIVE`.

| | `LIVE` (metadata on integration) | `¬LIVE` (none) |
|---|---|---|
| **`¬DOCKET`** | existing single-branch repo → **STOP**, point to migration (§9); do not auto-create `docket` or move data | fresh repo → create the empty orphan `docket`, push, **proceed** |
| **`DOCKET`** | *half-migrated* (interrupted run) → **STOP**, point back to the migration script to finish its prune (§9) | migrated → **proceed** |

The guard is terse (a few lines) so it does not inflate the convention block; the migration itself lives in a standalone script (§9).

### 7.1 `docket-new-change` (producer)
- Allocate / brainstorm / scan / draft / board exactly as today, but all reads/writes happen in `.docket/`, and the change + spec + refreshed `BOARD.md` are committed and pushed to `origin/docket`.
- The spec is written to `.docket/docs/superpowers/specs/…` (on `docket`).
- **Proposed-kill sub-path:** if the producer kills a `proposed` change, set `status: killed` (+ `## Why killed`) in `.docket/`, push `origin/docket`, then run the **shared terminal-publish procedure (§7.7)** with outcome `killed`. (This is the second kill origin §7.7 must serve.) **In `main`-mode** (no `docket`/§7.7): do the archive move + `status: killed` + `## Why killed` directly in the metadata working tree (= the integration branch) and push `origin/<integration_branch>` — exactly as the `done` archive degrades in §7.3.

### 7.2 `docket-implement-next` (autonomous implementer) — resolves gaps #1 + #2
- **Selection / claim / reconcile** happen in `.docket/` on `docket`; pushes target `origin/docket`. "Confirm the reconcile push landed" means **compare SHAs** — `git -C .docket rev-parse @ == git rev-parse origin/docket` after a fetch — not merely "the push exited 0," so a concurrent re-reconcile cannot leave the build reading bytes older than origin.
- **Feature branch:** `git fetch origin <integration_branch>` (freshness), then `git worktree add .worktrees/<slug> -b feat/<slug> origin/<integration_branch>` — generalized from hard-coded `origin/main`.
- **Spec read:** immediately before reading, **re-sync** (`git -C .docket pull --rebase`) so `.docket/` matches `origin/docket`, then read the reconciled spec from `.docket/docs/superpowers/specs/…`. `writing-plans` performs an intentional **cross-tree** step: it *reads* the spec from `.docket/` and *writes* the plan into `.worktrees/<slug>`. The feature tree never carries the spec — so gap #2 ("reconcile lands on docket but feature cuts from integration") is *intended*, not a bug.
- **Plan:** `writing-plans` writes `docs/superpowers/plans/…` **in the feature worktree** (build artifact, merges via the PR). The change file's `plan:` *field* is written in `.docket/` (metadata).
- **ADRs:** `docket-adr` writes to `.docket/docs/adrs/…` on `docket`; `adrs:` field updated in `.docket/`.
- **`status: implemented` + `pr:`** committed in `.docket/`, pushed to `origin/docket`.
- **Reconcile-kill sub-path:** if reconcile finds the change obsolete, set `status: killed` (+ `## Why killed`) in `.docket/`, push `origin/docket`, run the **shared terminal-publish procedure (§7.7)** for the `killed` outcome (any ADRs the change already wrote ride along — they are in its `adrs:`), and prune any feature worktree/branch already created. (Without this, a kill-from-`in-progress` would never publish its terminal record.) **In `main`-mode** (no `docket`/§7.7): do the archive move + `status: killed` + `## Why killed` directly in the metadata working tree (= the integration branch) and push `origin/<integration_branch>`.
- The "`docket` mode caveat (v1 rough edge)" subsection is **removed** and replaced with the now-specified mechanics.

### 7.3 `docket-finalize-change` (close-out) — resolves gap #3 (the missing `docket`→integration publish)
- A **terminal transition** is reaching `done` (PR merged) or `killed` (abandoned) — docket's two terminal states. The actual publish-to-integration is the **shared procedure in §7.7**, invoked by whichever skill drives the transition.
- `docket-finalize-change` owns the **`done`** path: merge the approved PR (code) into the integration branch first (existing behavior — `gh pr merge` against `<integration_branch>`, not hard-coded `main`), then invoke **§7.7** with outcome `done`.
- **`main`-mode:** §7.7 is skipped entirely (it is guarded on `metadata_branch == docket`); the archive move alone lands the terminal record, because there the metadata working tree *is* the integration branch. The **archive-move contract is identical in both modes** — the dated `archive/<DATE>-<id>-<slug>.md` filename (UTC merge/kill-commit date, per the convention) and the reuse-existing-archive-file idempotency rule from §7.7 step 1 apply here too; only the tree it runs in differs. The `main`-mode kill clauses (§7.1/§7.2) inherit this contract.

### 7.4 `docket-status` (board + janitor)
- Board pass regenerates `BOARD.md` in `.docket/` on `docket` and pushes to `origin/docket`. The board is the **live planning view and stays on `docket`** — it is never published to the integration branch.
- **Sweep:** the safety-net sweep that moves a merged change to `done` is a **terminal-transition driver** — it MUST invoke the shared terminal-publish (§7.7) for that change, exactly as finalize does. (Without this, a swept change would be archived on `docket` but its terminal record would never reach the integration branch.) §7.7 step 1's idempotency means a sweep that races finalize on the same change is a safe no-op.
- Sweep / health checks operate against `.docket/`. Broken-link checks resolve `spec:` against `docket` (it lives there). **`plan:`/`results:` are resolved against `origin/<integration_branch>`, NOT `docket`** — those files never exist on `docket` (§3), so the old "tolerate until merge" rule is replaced by "validate on the integration branch": before merge they legitimately don't resolve anywhere (tolerated); after merge they resolve on the integration branch. Resolving them against `docket` would flag every `done` change as a permanent broken link.

### 7.5 `docket-adr` (decision ledger)
- Reads/writes `docs/adrs/` in `.docket/` on `docket`; regenerates the ADR index there; pushes to `origin/docket`.
- **How an ADR reaches the integration branch.** The rule is **an `Accepted` ADR publishes to the integration branch** (the decision ledger is a durable record that belongs with code), via the §7.7 copy mechanism for ADR files:
  - **Change-tied ADR** (the common case): rides its change's terminal publish — it is in the change manifest's `adrs:` field, so §7.7 copies it on that change's `done` (or `killed`) transition.
  - **Standalone ADR** (`docket-adr` invoked directly, not tied to an in-flight change): `docket-adr` publishes it itself — on acceptance it runs §7.7's **ADR-only** entry (copy-set = that single ADR file, no archive step) and pushes. Without this, a change-less ADR would be stranded on `docket` and the integration-branch ledger would be silently incomplete.
- **Status change to an already-published ADR** (`Superseded by`/`Reversed by`/`Deprecated`) — whether or not the ADR was originally change-tied — is published by **`docket-adr`'s own §7.7 ADR-only invocation** for that one file (the producing change is long since `done`, so it cannot drive the re-publish). The new ADR that supersedes/reverses is itself published the same way (standalone) or via its own change's terminal publish if it is change-tied.

### 7.6 `main`-mode (the opt-out) — no behavior change
Every §7 description above is the **`docket`-mode delta**. In `main`-mode (`metadata_branch: main`), all of these reduce to **today's single-branch behavior, unchanged**, because the abstractions degrade cleanly:
- "metadata working tree" = the **primary working tree on the integration branch** (`metadata_branch == integration_branch`; no `.docket/` worktree is created — §6).
- "push to `origin/docket`" = push to `origin/<metadata_branch>`, which in single-branch mode equals `origin/<integration_branch>` (the metadata branch *is* the integration branch — `main` for a trunk repo, `develop` for a single-branch GitFlow repo).
- "feature branch from `origin/<integration_branch>`" — unchanged (it was already integration-branch-relative in both modes).
- The **terminal-publish procedure (§7.7) does not run** (guarded on `metadata_branch == docket`); the archive move on the integration branch is the terminal record.
- The bootstrap guard (§7.0) is a no-op (`DOCKET`/`LIVE` are only evaluated when `metadata_branch == docket`).

A reviewer can verify backward-compat by asserting that with `metadata_branch: main` pinned, no skill path touches a `docket` branch, a `.docket/` worktree, the §7.7 publish procedure, or a `git checkout origin/docket -- …` / `git show origin/docket:…` invocation. (A `tests/` assertion should encode exactly this.)

### 7.7 Shared procedure: terminal publish (`docket`-mode only)
Invoked on **every** terminal transition by the skill that drives it — `done` (finalize §7.3, **and the status sweep in §7.4**), `killed`-from-`proposed` (producer, §7.1), `killed`-from-`in-progress` (implementer reconcile, §7.2) — and reused by `docket-adr` (§7.5) to publish a standalone or status-changed ADR. **Skipped entirely in `main`-mode** (no `docket` branch; the metadata working tree *is* the integration branch, so the archive move there is itself the terminal record — see the `main`-mode kill note in §7.1/§7.2). Two entry shapes, distinguished by a **publish token** `T` used to name the throwaway branch:
- **change publish** (`done`/`killed`): `T = <id>`. Copy-set built from the change manifest (below). Archive-first ordering applies.
- **ADR-only publish** (standalone or supersession/reversal, from §7.5): `T = adr-<NN>`. Copy-set is the single ADR file; **step 1 is skipped** (no change file to archive).

1. **(change publish only) Archive on `docket` first.** In `.docket/` (synced to `origin/docket`): `mkdir -p <changes_dir>/archive` (git tracks no empty dirs, so a fresh repo has none), move `active/<id>-<slug>.md` → `archive/<DATE>-<id>-<slug>.md`, set the terminal `status`, and — for `done` — write the `results:` link into the manifest (the same way §7.2 writes `plan:` in `.docket/`; the results *file* arrived via the PR) or, for `killed`, add `## Why killed`; commit + push `origin/docket`. `<DATE>` = the UTC date of **this archive commit** (for `done`, equivalently the merge date). **Idempotent across re-runs and day boundaries:** first probe (null-glob-safe, e.g. `find <changes_dir>/archive -name '*-<id>-<slug>.md'`) for an existing archive file and reuse that filename rather than recomputing today's date — otherwise an interrupted-then-resumed kill could mint a second archive file. Ordering is load-bearing: step 3 copies the *archived* path, so it must exist on `origin/docket` first.
2. **Provision a clean integration checkout** without disturbing the main tree (which never switches branches): a **transient worktree in a temp dir** on a throwaway local branch `pub-<T>` so the push has a real ref to name. Use `-B` (reset-or-create) and prune leaks so the procedure is re-run safe even if a prior run died before teardown:
   ```bash
   pub="$(mktemp -d)/pub"
   git worktree prune                                   # clear any leaked registration
   git worktree add -B pub-<T> "$pub" origin/<integration_branch>   # -B: reset/adopt a leftover pub-<T>
   ```
   (Temp-dir location ⇒ no in-repo path, no `.gitignore` entry, no `.worktrees/` slug-collision or prune hazard.)
3. **Copy the terminal records from `origin/docket`** — the *remote* tip, never the stale local ref — then commit and push with a **fast-forward-or-retry** loop (the integration branch is the most concurrency-exposed write in the design; it gets the same CAS discipline as `origin/docket`). Assemble the copy-set **as a list, not a fixed command** — append the archived change file (always present, so the list is never empty), then `spec:` *iff* non-empty, then each `adrs:` entry **whose ADR is `Accepted`** (skip `Proposed`/draft ADRs — the `Accepted` gate fires here, at the copy site). For an ADR-only publish the list is the single ADR file.
   ```bash
   git -C "$pub" fetch origin docket
   git -C "$pub" checkout origin/docket -- "${copyset[@]}"     # copyset built per above; never empty
   git -C "$pub" diff --cached --quiet || \
     git -C "$pub" commit -m "docket(<T>): publish terminal record (<done|killed>)"   # or "publish ADR-<NN>"
   until git -C "$pub" push origin HEAD:<integration_branch>; do  # CAS retry on non-fast-forward
     git -C "$pub" pull --rebase origin <integration_branch> \
       || { git -C "$pub" checkout origin/docket -- "${copyset[@]}"; git -C "$pub" rebase --continue; }  # same-file race: re-copy authoritative bytes
   done
   ```
   **Push `HEAD:<integration_branch>` explicitly** — a bare `git push origin <integration_branch>` from this worktree resolves the *source* to the local `refs/heads/<integration_branch>` (a stale or absent local ref, never the publish commit on `pub-<T>`), silently dropping or rejecting it. The guarded commit (`diff --cached --quiet ||`) makes a no-op re-run safe under `set -e`. `BOARD.md` is never published; plan/results/code already arrived via the PR (`done`) or do not exist (`killed`).
4. **Tear down (force, since the temp tree is disposable):** `git -C "$pub" checkout --detach 2>/dev/null; git worktree remove --force "$pub"; git branch -D pub-<T> 2>/dev/null || true; rm -rf "$(dirname "$pub")"`. The whole procedure is re-run safe: step 1 reuses the existing archive filename; step 2's `-B` + `prune` adopt a leaked branch/registration; step 3's guarded copy+commit is a no-op when bytes already match, and the push loop completes an interrupted push.

## 8. Branch model section (synced convention) — rewrite

The one-line rule generalizes to:

> Metadata (change files, `BOARD.md`, ADRs, specs) commits to `metadata_branch` (default `docket`) via the **metadata working tree** — which is the primary working tree on the integration branch in single-branch (`main`) mode, and the persistent `.docket/` worktree in `docket`-mode — and is **always pushed to its remote immediately**. A change's `feat/<slug>` branch is **ALWAYS cut from `origin/<integration_branch>`** (default `main`; `develop` for GitFlow). The feature branch adds only plan + results + code and **never modifies** docket metadata. On a terminal transition (`done` *or* `killed`), the driving skill runs the shared terminal-publish (§7.7): it **copies** the change's terminal records (archived change + its `spec:` if set + the **`Accepted`** ADRs in `adrs:`) from `origin/docket` onto the integration branch in one dedicated commit and pushes — the only flow of metadata onto the code line.

## 9. Bootstrap & migration — `migrate-to-docket.sh`

Migration from single-branch (`main`) to `docket`-mode is **one-time per repo** and touches branches/files, so it lives in a **single committed one-shot script** (`migrate-to-docket.sh`, sibling to `sync-convention.sh` / `link-skills.sh`) — *not* inlined in the skills (which would bloat all five). The per-skill bootstrap guard (§7.0) only **detects + points to** it.

The script (idempotent; prints each action; no history rewrite):
1. Verify preconditions: clean tree; the live planning surface present on the integration branch; **and no `docket` branch already on `origin`** (if `origin/docket` exists, this repo is already migrated — fetch and adopt the existing branch via the §6 "branch on origin, not local" path, don't create a divergent orphan).
2. Create the orphan `docket` branch seeded from the current planning dirs **as whole directories** — `<changes_dir>/` (active + archive + its static `README.md` blurb), `<adrs_dir>/` (ADR files + its index `README.md`), `docs/superpowers/specs/`, and `BOARD.md`. **Not** `<results_dir>/` or `docs/superpowers/plans/` (those stay on the integration branch — they are build artifacts, not planning state). Commit, **push** `origin/docket`.
3. On the integration branch, **prune the live planning surface** that now lives on `docket`: `<changes_dir>/active/`, `<changes_dir>/README.md` (it links to the now-absent board), and `BOARD.md`. **Keep** what the integration branch should hold going forward: terminal records (`<changes_dir>/archive/`, `<adrs_dir>/` — *including* its index `README.md`, which legitimately indexes the published ADRs that remain on integration) and the build artifacts (`<results_dir>/`, `docs/superpowers/plans/`). (The two README blurbs are treated asymmetrically *on purpose*: the `changes/` blurb links to `BOARD.md`, which is now `docket`-only, so it is pruned; the `adrs/` index points at ADR files that *stay* on integration, so it is kept. Both also exist on `docket` per step 2.) Commit + push.
4. Add `.docket/` and `.worktrees/` to `.gitignore` on the integration branch (and ensure they are absent from `docket`). (The §7.7 publish worktree lives in a `mktemp` dir outside the repo, so it needs no ignore entry.)
5. Print next steps (skills now operate in `docket`-mode automatically).

**Idempotency / interrupted-migration safety:** re-running the script must converge from any partial state. Each step is split into a **mutation** and a **separately-guarded push**, probed against *different* refs:
- **Mutation postcondition → probe the LOCAL tree** (create-branch only if the local branch is absent; seed only paths not already on local `docket`; prune only paths still present on the *local* integration branch). Probing the mutation against `origin` would be wrong: after "pruned-but-not-pushed", a re-run that probed `origin` would see the path still there and re-`rm` it — a hard error. The prune `rm` is also written tolerantly (`git rm -r --ignore-unmatch …`) so a redundant run is a no-op, not a failure.
- **Push guard → probe `origin/<branch>`** (push only if `git rev-parse origin/<branch>` ≠ local). A mutation that already ran leaves nothing to commit, but its push may still be pending — guarding the push on "did the mutation run" would strand an unpushed `docket` branch or an unpushed prune commit.

All presence probes use `git ls-tree <ref> -- <paths>` (never bare `<ref>:<path>`, which misreports against the working dir), and treat exit≠0 (ref absent) as a config error, not "absent" (§7.0). The §7.0 guard's `DOCKET`/`LIVE` matrix detects the **half-migrated** state (`DOCKET ∧ LIVE`) and points back here to finish the prune rather than proceeding with stale duplicates.

Edge: a repo whose metadata is purely terminal (like docket itself: only `archive/0001`) migrates near-trivially — those records already belong on the integration branch; the script just creates `docket` for future churn.

## 10. README update (first-class deliverable)

Replace *How metadata is stored* with a full **"docket-mode"** section that lets a reader understand the model without reading the skills:
- The two-branch model + the **artifact-location table** (§3) — answering "which artifacts live where, and how do they reach the integration branch."
- The `integration_branch` knob + GitFlow (`develop`) support.
- The `.docket/` metadata worktree (what it is, why, that it is gitignored).
- The finalize → selective-publish flow (and that the live board stays on `docket`).
- The migration story (`migrate-to-docket.sh`) and the refuse-to-migrate bootstrap guard.
- The `main`-mode **opt-out** (pin both knobs) for teams that want everything on one branch.
- Update *Status* and *Install* accordingly: `docket`-mode is the supported default; remove the "rough edge / not recommended" language.

## 11. ADRs this change should produce (at build time)

The cross-branch architecture is decision-heavy. Expect ~1–2 ADRs, back-linked to change 0002:
- **The docket metadata-branch model** — orphan branch + persistent `.docket/` worktree + selective terminal publish; why a branch merge is the wrong tool and `checkout -- <paths>` is right.
- **docket-mode as the default + refuse-and-migrate bootstrap** — why the literal default flips and why first-run refuses rather than auto-migrating.

## 12. Touch-points (scope of the one PR)

- **Synced convention block** (canonical in `docket-new-change/SKILL.md`, propagated by `sync-convention.sh`): `.docket.yml` example (`metadata_branch` default + `integration_branch`), the Branch-model section rewrite (§8), the bootstrap-guard rule (§7.0), the "metadata working tree" wording.
- **All five skills:** `.docket/` worktree usage (state-specific creation + sync-before-read), always-push invariant, integration-branch generalization, and the per-skill specifics in §7 — the implementer's sync-then-read spec + SHA-compare reconcile-confirm + reconcile-kill sub-path and removal of the v1 caveat; the producer's proposed-kill sub-path; finalize's `done` path; the **shared terminal-publish procedure §7.7** (invoked by finalize + both kill origins); `docket-adr`'s Accepted-ADR publish; status's board-stays-on-`docket` and `plan:`/`results:` validated against the integration branch.
- **`migrate-to-docket.sh`** (new, committed): orphan-seed + prune + ignore, with split mutation/push idempotency and `ls-tree` probes (§9).
- **`.gitignore`** (new): `.docket/`, `.worktrees/`.
- **README.md** (§10 rewrite).
- **`tests/`**: content/sync **assertions** (the TDD-for-docs style used by change 0001) — e.g. "convention blocks in sync after edits", "`integration_branch` knob present across all five skills", "no skill still contains the v1 `docket` caveat", "terminal-publish copy-set = {change, spec, manifest `adrs:`} sourced from `origin/docket`", "kill-publish wired in `docket-new-change` *and* `docket-implement-next` (not just finalize)", "`.gitignore` ignores `.docket/`+`.worktrees/`", and a **`main`-mode backward-compat** assertion (§7.6: with `metadata_branch: main`, no skill path references a `docket` branch, a `.docket/` worktree, the §7.7 publish, or a `git checkout origin/docket -- …` copy)."

## 13. Decided details (resolved at brainstorm 2026-06-02)

- Terminal records are published via a **selective file copy** from `origin/docket` (`git checkout origin/docket -- <paths>`, §7.7), **never** a `git merge docket` (which would drag all churn onto the integration branch).
- Metadata surface = **persistent `.docket/` worktree** (not `.worktrees/docket`, not transient checkouts, not plumbing).
- `docket`-mode is the **literal default**; first-run on an existing main-mode repo **refuses + points to the migration** rather than auto-migrating.
- `docket` is an **orphan** branch, **always pushed**.
- Copy-set = {archived change (always), its `spec:` if set, the **`Accepted`** ADRs in the manifest's `adrs:`}, sourced from `origin/docket` (§7.7); plan/results/code arrive via the PR; `BOARD.md` never leaves `docket`. Both terminal transitions (`done` + `killed`) publish, via finalize / the killing skill / the §7.4 sweep.
- **One change** (id 0002), `high` priority, `related: [1]`. This repo's own migration is a **separate follow-up**.

## 14. Rollout note (consequence of flipping the default)

Flipping the literal default to `docket` means **every existing `main`-mode repo with no `.docket.yml` — including docket itself — will hit the bootstrap guard (§7.0) and refuse** on the next skill run (metadata exists on the integration branch, no `docket` branch yet). That is the intended safety, but it means: to keep a repo running in `main`-mode until it deliberately migrates, it must **pin `metadata_branch: main` in `.docket.yml`**. Because this repo's own migration is a separate follow-up (§2), shipping this change should include adding a `.docket.yml` to docket itself that pins `main`-mode, so docket's skills keep working on docket until the follow-up migrates it. (This is a rollout step, not new mechanism.)

## 15. Open questions

None outstanding. `integration_branch` auto-detection fallback (`origin/HEAD` → `main`), the exact bootstrap-guard wording, and the migration script's precise git invocations are implementation details for the plan, not open design questions.
