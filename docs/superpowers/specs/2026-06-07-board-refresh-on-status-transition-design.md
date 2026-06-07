# Design: refresh BOARD.md on status transitions (not only at Step 0)

**Status:** design (brainstormed 2026-06-07)
**Change:** 0004
**Related:** change 0002 (metadata-branch model; introduced the `.docket/` worktree, terminal-publish, and the wholesale Board pass); ADR-0001 (metadata branch model)

## 1. Context / problem

`BOARD.md` is a derived view of the change files, regenerated wholesale by `docket-status`'s **Board pass**. The change files' `status:` field is the source of truth and transitions correctly; the board only refreshes when *someone* runs the Board pass. Today that responsibility is implicit and unevenly applied, so the board trails reality for the entire duration of an autonomous build.

Audited every site that writes a change's `status:`:

| Site | Skill | Transition | Board today |
|---|---|---|---|
| New change / scan stubs | `docket-new-change` | →`proposed` | ✅ refreshes (step 5) |
| Proposed-kill | `docket-new-change` | →`killed` | ❌ **none** |
| Claim | `docket-implement-next` | `proposed`→`in-progress` | ❌ **none** |
| Reconcile-kill | `docket-implement-next` | `in-progress`→`killed` | ❌ **none** |
| PR open | `docket-implement-next` | `in-progress`→`implemented` | ❌ **none** |
| Finalize | `docket-finalize-change` | `implemented`→`done` | ✅ refreshes (step 5) |
| Merge-sweep | `docket-status` | `implemented`→`done` | ✅ refreshes (Board pass) |
| `blocked` / `deferred` / revive | *(no driving skill — manual frontmatter edit)* | — | n/a |

**Four** sites are gaps, not the two named in the change file. The change file assumed "terminal transitions are already covered" — true only for `done`. The two `killed` origins invoke only the shared **terminal-publish** procedure, which copies records to the integration branch and explicitly *never* touches `BOARD.md` ("BOARD.md is never published"). For `done`, the board refresh lives in the *driving* skill (finalize step 5 / sweep), not in terminal-publish; the kill origins have no such driving-skill refresh, so they leave the board stale too.

This is a **visibility gap, not a correctness bug.** Claiming is a compare-and-swap on the change *file* (`pull --rebase` → re-read `status:` → proceed only if still `proposed`), never on the board, so a stale board cannot cause a double-claim. But the board — docket's at-a-glance "what's happening now?" — is wrong for the whole build window: claimed/in-flight/PR-open work keeps showing as `proposed` / build-ready.

## 2. Decision

Make the board reflect status transitions *as they happen*, by establishing one explicit invariant and closing the whole class of gaps — not just the two named sites.

1. **Invariant (shared contract).** Add one terse sentence to the canonical `## Convention` block: *any* skill that writes a change's `status:` regenerates `BOARD.md` (the Board pass) in a separate commit immediately after.
2. **Fix the four gap sites** by appending a Board-pass call to each (the two `killed` origins + claim + `implemented`).
3. **Best-effort disposition** for the refreshes `docket-implement-next` performs inline during an autonomous drain (claim, `implemented`, reconcile-kill): bounded retry, then log-and-continue — never abort the build for a derived view.
4. **Drift tripwire** in `docket-status`'s health checks: warn (don't fail) when the committed board disagrees with a fresh render, then regenerate as usual.

**Bloat is a first-class constraint** (see §3.6). The renderer is never duplicated (single source in `docket-status`); the convention carries only the one-sentence *rule*; all *mechanism* (retry semantics, best-effort) lives once, in the single skill that needs it, and is cross-referenced. Net synced (×5) text added: one sentence.

Rejected:
- **Bundling the board regen into the `status:` commit.** The change-file commit must stay byte-identical across concurrent agents so the claim CAS's loser rebases cleanly; folding in a per-agent board render reintroduces conflicts on the hot path. Board stays a **separate** commit.
- **Sites-only, no convention invariant.** Loses the "future skills inherit the rule" benefit; the one-sentence cost is trivial.
- **Live/streaming board** (file-watcher, per-substep push, freshness timestamps) — out of scope.

## 3. Design

### 3.1 The invariant — one sentence, in the canonical Convention block

The `## Convention` block is synced byte-identical into all five skills by `sync-convention.sh` (canonical source `docket-new-change/SKILL.md`). Add the rule to the lifecycle **Rules** paragraph (prose that already discusses transitions and what the board shows), as a single sentence:

> **Board refresh on status writes.** Any skill that writes a change's `status:` regenerates `BOARD.md` (the Board pass) in a separate commit immediately after — the board is a derived view and must never trail the change files.

That is the *entire* convention-level addition. The "terminal-publish never touches the board, so the driving skill owns the refresh" nuance is **not** added to the convention (×5); it is already true in terminal-publish ("BOARD.md is never published") and is realized at the kill sites in §3.3.

### 3.2 The renderer stays single-sourced; board stays a separate commit

No second renderer. Every site "runs the Board pass" = invokes the existing canonical procedure in `docket-status` (wholesale regen from the change files; on a `pull --rebase` collision in `BOARD.md`, regenerate from the change files, never 3-way merge). The board commit is always separate from the `status:` commit, preserving the determinism that lets concurrent writers' CAS rebase cleanly.

### 3.3 The four gap sites — append one clause each

Each fix appends a Board-pass call to the existing step; no new sections.

- **`docket-implement-next` Step 2 (claim).** After the claim commit lands, run the Board pass (best-effort — §3.4) so the board moves the change to *in-progress*.
- **`docket-implement-next` Step 3 (reconcile-kill).** After the kill is archived (and, in `docket`-mode, terminal-published), run the Board pass (best-effort) so the board drops the killed change.
- **`docket-implement-next` Step 7 (`implemented`).** After the `implemented` + `pr:` commit, run the Board pass (best-effort) so the board shows *implemented — needs your merge*.
- **`docket-new-change` proposed-kill sub-path.** After the kill is archived (and, in `docket`-mode, terminal-published), refresh `BOARD.md` and push — the **must-land** pattern, same as the create path's step 5 (this skill is interactive; a clean board is its deliverable).

### 3.4 Best-effort — defined once, referenced thrice

`docket-implement-next` gets **one** definition (a short shared note, e.g. beside its existing *Metadata commits* note), cross-referenced from the three sites above:

> **Best-effort board refresh.** The Board pass this skill runs after its own status writes (claim, reconcile-kill, `implemented`) is best-effort: attempt the regen + push with bounded retries, then log and continue — never abort the build for it. The build's correctness rests on the change-file CAS, not the board; any residual staleness self-heals at the next must-land Board pass (the next change's Step 0 `docket-status`, a manual `docket-status`, or finalize).

Everywhere else keeps today's **must-land** behavior (rebase-regenerate until it lands): `docket-new-change`, `docket-finalize-change`, `docket-status` — the board-producing skills where the board is the deliverable.

### 3.5 Drift tripwire — `docket-status` health check (×1)

Added to `docket-status`'s health-check list, before the Board regen:

> **Board/source drift** — render the board in-memory from the change files and diff it against the committed `BOARD.md`; if any change's rendered status differs, emit a warning naming the change(s) ("a writer skipped the board-refresh invariant"), then regenerate as normal (which heals it).

A **warning, not a failure** — consistent with the other health checks and with best-effort refreshes being allowed to lose a race. Because `docket-implement-next` Step 0 *calls* `docket-status`, the drain self-reports at the start of each change whether the previous change's best-effort refresh actually landed.

### 3.6 Markdown budget — keeping skills effective

The whole point of the user constraint. Accounting of added prose:

| Where | Cost factor | Added |
|---|---|---|
| Convention invariant (§3.1) | ×5 (synced) | **1 sentence** |
| Best-effort definition (§3.4) | ×1 (`implement-next`) | ~1 short paragraph |
| 3 site clauses (§3.3) | ×1 (`implement-next`) | 1 clause each |
| Proposed-kill clause (§3.3) | ×1 (`new-change`) | 1 clause |
| Drift tripwire (§3.5) | ×1 (`docket-status`) | 1 health-check bullet |

Effectiveness rules this design follows:
- **Every word in the convention costs 5×** → the convention carries only the *rule*, never the *mechanism*.
- **Anchor to existing steps** (append a clause) rather than adding new top-level sections → skim structure unchanged.
- **Define mechanism once, cross-reference** (best-effort) → no repetition across the three implement-next sites.
- **Never duplicate the renderer** → "run the Board pass" is a reference, not a restatement.

## 4. What changes (touch-points)

- **`skills/docket-new-change/SKILL.md`** — (a) canonical Convention edit: the §3.1 invariant sentence; (b) proposed-kill sub-path: the must-land board-refresh clause (§3.3).
- **`skills/docket-implement-next/SKILL.md`** — best-effort definition (§3.4) + Board-pass clauses at Steps 2, 3 (reconcile-kill), and 7 (§3.3).
- **`skills/docket-status/SKILL.md`** — board/source drift health check (§3.5). (Plus the synced Convention block.)
- **`skills/docket-finalize-change/SKILL.md`, `skills/docket-adr/SKILL.md`** — receive only the synced Convention block (no behavioral edit).
- **`sync-convention.sh`** — run (not edited) to propagate the canonical block; `--check` must pass.
- **`tests/test_board_refresh_on_transition.sh`** — new (§6).

## 5. Out of scope

- **New statuses or lifecycle changes** — `in-progress` already spans plan/build/review; this only makes the board *reflect* existing states.
- **Board refresh on non-status steps** — reconcile (Step 3) doesn't move `status:`, so it gets no regen; no gratuitous refreshes.
- **`done` terminal paths** — finalize step 5 / merge-sweep already refresh; untouched.
- **Manual `blocked`/`deferred`/revive edits** — no driving skill carries the invariant; covered by the drift tripwire + a manual `docket-status`, not by new automation.
- **terminal-publish** — stays board-agnostic ("BOARD.md is never published"); the fix is at the kill *sites*, not in the shared copy procedure.
- **Live/streaming board** — no file-watcher, no per-substep push, no freshness timestamps.

## 6. Testing

House idiom: a bash assertion script that greps the `SKILL.md` files for the required wording (skills are agent instructions, so the contract is textual), plus the existing exec test for `sync-convention.sh`. New **`tests/test_board_refresh_on_transition.sh`**:

1. **Invariant synced** — the §3.1 sentence (keyed on a stable phrase, e.g. `Board refresh on status writes`) is present in all five skills; `bash sync-convention.sh --check` exits 0.
2. **Four sites wired** — `docket-implement-next` instructs a Board pass after claim, after reconcile-kill, and after `implemented`; `docket-new-change`'s proposed-kill instructs a board refresh.
3. **Best-effort marked** — the three implement-next refreshes are described best-effort / non-fatal; the others remain must-land.
4. **terminal-publish stays board-agnostic** — the "BOARD.md is never published" guarantee is still present (guards against "fixing" the kill gap by wrongly teaching terminal-publish to render).
5. **Tripwire present** — `docket-status` health checks include the board/source drift check as a warning.

The existing `test_sync_convention.sh` and `test_docket_metadata_branch.sh` (item A: convention in sync) stay green.

## 7. Open questions

None blocking. (Dogfood note: shipping this change is itself a `proposed → in-progress → implemented → done` walk; once merged, those transitions will refresh the board automatically — the change demonstrates itself.)
