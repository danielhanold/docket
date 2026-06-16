# Design: GitHub board mirror — selectable board surfaces, one-way Issues + Projects mirror

**Status:** design (brainstormed 2026-06-14)
**Change:** 0011
**Related:** 0004 (board-refresh-on-status-transition — established the "refresh the board on every status write" invariant this change generalizes), 0010 (board-analytics — a future stats block on the same Board pass; this design keeps the surface hook composable so the two don't fight), `docket-status` (owns the Board pass the mirror rides in), `docket-convention` (gains the `board_surfaces`/`github_project` config, the `issue:` field, and the status→issue mapping), `docket-implement-next` (best-effort PR→issue reference at PR-open)

## 1. Context / problem

The board is docket's pure-visibility surface. The one status it matters most to surface — `implemented — needs your merge`, the human's single job — is the one markdown renders worst and GitHub renders best (an open issue with a linked PR in an "awaiting merge" view). This change mirrors each docket change onto GitHub as one issue (and one Projects v2 item), **strictly one-way**: change files on the metadata branch stay the source of truth, and the mirror is a derived view that is never read back.

In the course of the brainstorm the feature grew a second axis the original stub did not have: the board is not "BOARD.md, optionally also mirrored to GitHub" — it is a **derived view that can be rendered on zero or more selectable surfaces**. A repo may want the offline-safe `inline` board, the `github` mirror, both, or neither. That selection (`board_surfaces`) is the spine of this design; the Issues/Projects mirror is the new `github` surface hung off it.

**Invariant that survives all of this:** the change files (+ git history) are *always* the source of truth. Every board surface — `inline` included — is derived output, regenerated wholesale from the change files, never hand-edited, never read back. Rendering zero surfaces loses no authoritative data; it just means no derived convenience view.

## 2. Configuration — `board_surfaces` and the GitHub knobs

New keys in `.docket.yml` (committed on the default branch, read by every docket skill at startup, like all existing config):

```yaml
# board_surfaces — which derived board view(s) to render. A list; [] disables the board entirely.
#   inline  → BOARD.md on the metadata branch (offline-safe, committed)
#   github  → GitHub Issues + Projects v2 mirror (one-way)
board_surfaces: [inline]      # DEFAULT. [inline, github] = both; [github] = GitHub-only; [] = no board at all

# github_project — identity of the auto-managed Projects v2 board (only consulted when `github` is enabled).
# Minted on first sync if absent and written back here (one-time config commit on the default branch).
github_project:               # e.g. {owner: danielhanold, number: 7}  — unset ⇒ auto-create on first github sync
```

Rules:

- **Closed token set.** `board_surfaces` members are `inline` and `github`. An unknown token is **warned-and-ignored** (best-effort spirit — a typo must never abort a build); the warning surfaces in `docket-status`'s health output.
- **Default `[inline]`.** Backward-compatible: every existing repo keeps exactly today's behavior and does *not* start minting GitHub issues. `github` is strictly opt-in.
- **`[]` is valid and means "no board."** No `BOARD.md`, no GitHub mirror. The change files + git remain the only (and fully authoritative) record. This is the explicit answer to "can the board be completely disabled?" — yes.
- **A non-GitHub remote silently drops `github`** even if listed (the same "repos on other remotes skip the mirror" rule from the stub), degrading to whatever else is in the list.
- Backward-compat shorthand is unnecessary — absent key ⇒ `[inline]` covers every pre-existing repo.

## 3. The board as N surfaces — generalizing the existing invariants

`docket-status`'s **Board pass** becomes a **render-each-enabled-surface** pass. The convention's existing rules generalize cleanly:

| today (single implicit surface) | generalized |
|---|---|
| "Any skill that writes `status:` regenerates `BOARD.md`" | "…refreshes **each enabled board surface**" — `inline` rewrites `BOARD.md`; `github` runs the mirror upsert; `[]` ⇒ no-op |
| "`BOARD.md` is the offline-safe canonical view" | offline-safety is a property **of the `inline` surface**; dropping `inline` forfeits it (documented tradeoff); `[]` ⇒ git history is the only record |
| health check: board/source **drift compare** against committed `BOARD.md` | runs **per enabled surface** (inline: compare `BOARD.md`; github: best-effort reconcile; skipped entirely when the surface is off) |
| Board pass on a `pull --rebase` conflict: regenerate, never 3-way merge | unchanged for `inline`; `github` is idempotent upsert, no merge concept |

The `inline` surface is exactly today's `BOARD.md` generation — unchanged code, now gated on `inline ∈ board_surfaces`. All the new machinery is the `github` surface (§4–§7).

## 4. The `github` surface — one-way Issues + Projects mirror

Runs inside the Board pass whenever `github ∈ board_surfaces`. **Best-effort, exactly like the existing board rule:** it needs network + `gh` auth; it must **never abort a build**; it self-heals on the next pass. It is purely additive — it never replaces any other surface.

### 4.1 Issues (always part of `github`)

- **One issue per change**, upserted idempotently. A new per-change frontmatter field **`issue:`** (same shape/lifecycle as `pr:`) is minted on first sync and stored in the change file; subsequent syncs update the existing issue. `issue:` set ⇒ update; unset ⇒ create-then-record.
- **State + close reason** (the sync is the *sole writer* of issue open/closed state — see §6):
  - `proposed`, `in-progress`, `blocked`, `deferred`, `implemented` → **open** issue.
  - `done` → **closed as completed**.
  - `killed` → **closed as not planned**.
- **Labels — `docket:` namespace only** (§5).
- **Body** (§4.3): banner + digest + `## Why` distillation + hrefs to every relevant artifact.

### 4.2 Projects v2 (part of `github`; auto-created; degrades gracefully)

- **Auto-create if not configured.** When `github_project` is unset, first sync **mints a Projects v2 board** under the **integration repo's owner**, **private** by default (a public board would leak the backlog), with a **Status single-select field** whose options are the active statuses (proposed / in-progress / blocked / deferred / implemented — the two terminal states are expressed by closing the issue, not a column). The minted `{owner, number}` (plus resolved field/option ids, cached) is **written back into `.docket.yml` on the default branch** — a one-time config commit that makes every subsequent run idempotent (no duplicate projects) and visible to every clone. This is the same persistence philosophy as the per-change `issue:` field, lifted to the single per-repo project.
- **Each synced issue is added as a project item** and its Status field set from the change's status.
- **Graceful degradation — Projects is the optional half of `github`.** If the token lacks `project` scope, or any GraphQL call fails, the sync **skips Projects entirely and still mirrors Issues + labels**, logs, and continues. Projects never blocks Issues; `github` never blocks a build.

### 4.3 Issue body

A visibility *pointer*, never a second home for the content (mirroring the full body would duplicate the source of truth and re-push a fat body every pass). The body carries:

1. **One-way banner** — "Generated mirror of `docs/changes/…` on the `docket` branch. Edits and comments here are not read back." (the stub's required banner).
2. **Frontmatter digest** — one line: status · priority · readiness · deps.
3. **`## Why` distilled** to one or two sentences.
4. **Hrefs to every relevant artifact** (the brainstorm's explicit addition — not just the change file): a deep link to the **change file on `docket`**, to the **`spec:`**, to **each ADR in `adrs:`**, and to **`plan:`/`results:`** once those fields are populated. Each link is emitted only when its field resolves, so the link set grows as the change moves through its lifecycle.

### 4.4 Implementation — a deterministic sync script, not agent prose

**Decided in the brainstorm (2026-06-14):** the `github` surface is implemented as a **deterministic script** that `docket-status`'s Board pass *invokes*, NOT as agent-constructed `gh`/GraphQL calls written inline in skill prose.

This is a deliberate departure from the rest of docket. Every other board operation is local, idempotent-by-regeneration, and side-effect-free (`inline` rewrites a file from the change files), so agent-executed prose is fine there. The mirror is the opposite: **idempotent, side-effectful writes to an external API** (create-or-update one issue per change, reconcile a label set, add/move project items). LLM variance on that surface is a real hazard — a non-deterministic agent could mint duplicate issues, drift a label name, set the wrong close reason, or spawn a second project — and the writes are invisible to the test suite (which only sees the integration-branch checkout, per the LEARNINGS note on metadata-branch artifacts). A script makes the external behavior reproducible and lets tests **assert command construction** against mocked `gh`/GraphQL, without a live GitHub.

Shape (settle exact form at build time):

- A single entry point — e.g. `scripts/github-mirror.sh` (or a small program if shell + GraphQL proves unwieldy) — that takes the parsed backlog (or reads the change files directly) and performs the full upsert: issues, labels, Projects items, close states.
- **Pure-ish and idempotent:** given the same change files + the same GitHub state, it converges to the same result and is safe to re-run. It owns the `issue:`/`github_project` reads but the **change-file writes** (minting `issue:`, the `.docket.yml` project write-back) stay with the Board pass / git discipline — the script emits what to persist; the skill commits it. (Reconcile may instead let the script do the metadata git writes; flag the boundary at build time.)
- **Best-effort contract preserved:** the script exits non-zero / no-ops cleanly on missing network, auth, or `project` scope; the Board pass treats any failure as best-effort (log, continue, never abort a build) exactly like today's board rule.
- **Testable:** a `tests/test_github_mirror.sh` asserts command construction against a mocked `gh` (and GraphQL), covering create-vs-update, the seven status→state/reason mappings, the `docket:` label set, and the Projects-skipped-when-no-scope degradation. This is the build-time verification the LEARNINGS ledger calls for in place of suite assertions on live GitHub state.

The `inline` surface stays agent-prose (unchanged); only the `github` surface gets the script. `docket-status` documents the invocation; the script is the single source of the external-write mechanics.

## 5. Labels — `docket:` namespace, owns only its own

Every mirror label is prefixed `docket:` and docket **only ever creates or updates labels inside that namespace** — it never touches a label it did not mint, so a repo already using bare `status:`-style labels is untouched and collision-proof.

- `docket:status/<proposed|in-progress|blocked|deferred|implemented>` (terminal states are the closed-issue reason, not a label).
- `docket:priority/<low|medium|high|critical>`.
- Readiness annotations: `docket:readiness/needs-brainstorm`, `docket:readiness/auto-groom-blocked`, `docket:waiting/needs-your-merge`, `docket:waiting/not-yet-built` — the same derived annotations the inline board renders, lifted to labels.

docket creates each label (with a managed color) on first use and reconciles its set per change each pass.

## 6. Closing & PR linkage — sync owns close, PR only references

The headline win (open issue + linked PR in an awaiting-merge view) needs a PR→issue link, but the one-way invariant says the sync is the **single writer** of issue state. Resolution:

- The PR **references** the issue (a plain `#N` mention / Development link) so GitHub renders the linked-PR relationship — but **never `Closes #N`**, which would make GitHub a second writer that auto-closes on merge with reason *completed* and cannot express `killed → not planned`.
- The **Board-pass sync remains the sole writer** of open/closed state and reason (`done → completed`, `killed → not planned`, §4.1).
- PR linkage is **best-effort**: `docket-implement-next` adds the reference at PR-open **only if `issue:` is already set** (it usually is — the issue is minted on the proposed/in-progress Board passes that precede PR-open per 0004 — but the link is skipped, not blocked, when a prior sync hasn't run, e.g. offline). The next sync does not need to repair anything; the reference is a one-time write on the PR body.

## 7. Touch-ups to existing skills and docs

- **`docket-convention`** — the single source for all new contract: the `board_surfaces` list (members, default `[inline]`, `[]` = no board, unknown-token warn-and-ignore), the `github_project` knob (auto-create, private, write-back), the per-change `issue:` field added to the manifest, the status→issue-state + close-reason mapping, the `docket:` label namespace, and the one-way rule. The "regenerate BOARD.md on status write" and "offline-safe canonical view" sentences are generalized to "each enabled surface" / "property of the `inline` surface" per §3.
- **`docket-status`** — the Board pass becomes render-each-enabled-surface; for the `github` surface it **invokes the deterministic mirror script** (§4.4) best-effort, and gates `inline` on membership. The drift-compare health check generalizes per §3; add a check for an `issue:` set but unreachable. Merge-sweep/finalize close the mirror issue via the sync's normal pass (no new write path — closing is just the `done`/`killed` status mapping).
- **`scripts/github-mirror.sh`** (new, §4.4) — the deterministic sync engine for the `github` surface; the single source of the external-write mechanics. Wired into `link`/plumbing only if needed (it is invoked by `docket-status`, not auto-discovered like skills).
- **`docket-implement-next`** — at PR-open, best-effort add the `#N` reference to the PR body when `issue:` is set (§6). No `Closes`.
- **`docket-new-change` / `docket-finalize-change`** — no mechanics change; finalize's terminal transition already drives a Board pass, which now also closes the mirror issue with the right reason.
- **Repo plumbing / tests** — config-parsing for `.docket.yml` gains the two keys; any test enumerating config keys or skills' Board behavior updates accordingly. The mirror's `gh`/GraphQL calls are exercised through the **deterministic script** (§4.4), so tests assert the script's command construction against a mocked `gh` (the suite runs against the integration-branch checkout and has no live GitHub — assert command construction, not live effects; verify live behavior at build time and record in the results file, per the LEARNINGS note about metadata-branch artifacts).

## 8. Residuals for the implementer (reconcile / build time)

- **Cross-branch config write.** The `github_project` write-back lands on the **default/integration branch** (where `.docket.yml` lives), not `docket` — the *one* time the mirror writes outside the metadata branch. The Board pass runs in the `.docket/` worktree on `docket`; minting the project therefore needs a separate, guarded commit to the default branch (fetch → edit `.docket.yml` → commit → push, best-effort, race-tolerant). Reconcile should confirm this is the least-surprising mechanism, or revisit the persistence choice (a `docket`-branch state file was the considered alternative).
- **Candidate ADR.** The boundary rule — *git-native agent mechanics live in docket; human-facing visibility lives on GitHub's surface; the mirror is strictly one-way* — is a genuine architecture decision worth an ADR. ADR creation is build-time (`docket-adr`), so it is left for the implementer to mint and back-link via `adrs:`.
- **Projects field schema drift.** If a human renames/reorders the auto-created Status options, the cached option ids go stale. Reconcile/build should decide the repair posture (re-resolve by option name each pass vs. cache + heal on mismatch) — kept out of this proposal as build-detail.
- **Token-scope detection.** Distinguishing "no `project` scope" (skip Projects, keep Issues) from a transient GraphQL failure (retry next pass) is best-effort; settle the exact signal at build time.

## 9. Out of scope (unchanged from the stub, plus)

- **Two-way sync of any kind** — never read comments, assignments, or label edits back into change files.
- **Mirroring the Mermaid dependency graph** onto native sub-issue/dependency relationships (write-heavy, least-consulted; a possible later change).
- **Per-day timeseries / charts** on the GitHub surface (that is 0010's analytics territory, and even there markdown-only).
- **Auto-creating the project as public, or under a non-owner account** — private under the repo owner only; anything else is a human decision.
