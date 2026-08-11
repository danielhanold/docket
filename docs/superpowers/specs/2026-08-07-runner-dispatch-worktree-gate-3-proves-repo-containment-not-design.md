<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0208 — Harden runner-dispatch — worktree membership gate, feature-scoped coverage, flag-parse guards](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0208-runner-dispatch-worktree-gate-3-proves-repo-containment-not.md)**
<!-- docket:backlink:end -->

# Harden runner-dispatch — worktree membership gate, feature-scoped coverage, flag-parse guards

**Change:** 0208 · **Date:** 2026-08-07 · **Type:** fix
**Files:** `scripts/runner-dispatch.sh`, `scripts/runner-dispatch.md`, `sync-agents.sh`,
`agents/docket-*.md` (frontmatter only), `skills/docket-implement-next/SKILL.md` and
`skills/docket-finalize-change/{SKILL.md,references/gate-failure.md}` (one dispatch sentence
each), adapter contracts (`scripts/runners/*.md`, wording only), `tests/test_runner_dispatch.sh`,
`tests/test_sync_agents_runners.sh`, one new ADR + an `## Update` on ADR-0068.

Three hardenings of the same script's input gates, consolidated from 0206's whole-branch review
(this stub + killed 0209/0210). All three share `tests/test_runner_dispatch.sh`.

## 1. Gate 3 becomes a membership test (a)

Today's gate — `[ "$(docket_main_worktree "$ANCHOR")" = "$REPO_ROOT" ]` — proves *containment in
the repo*: it passes for the main worktree itself and for every ordinary subdirectory of it, so a
`build-*` delegation handed the repo root clears all gates and anchors the build worker in the
primary checkout on the integration branch.

Replacement (after the existing `-d` gate, same position in the gate order):

```sh
ANCHOR="$(cd "$ANCHOR" && pwd -P)"   # physical path; git prints physical paths in worktree list
wt_list="$("${GIT:-git}" -C "$ANCHOR" worktree list --porcelain 2>/dev/null)"
[ "$(sed -n '1s/^worktree //p' <<<"$wt_list")" = "$REPO_ROOT" ] || die "--worktree $ANCHOR is not a worktree of this repository"
grep -qxF -- "worktree $ANCHOR" <<<"$wt_list" || die "--worktree $ANCHOR is not a worktree of this repository (it is inside one, but a run anchor must be a worktree top-level)"
```

- One `git worktree list` call from `$ANCHOR` yields both facts. Same-repo is proven by the
  **first** `worktree` line — git lists the main worktree first (the exact property
  `docket_main_worktree` already rests on) — never by an anywhere-in-list match: `worktree list`
  retains stale records for deleted-and-recreated directories, so a foreign repo's list can
  contain a `worktree $REPO_ROOT` line for a path that is no longer its worktree, and an
  anywhere-match would hand a delegated run a tree docket does not own (regressing the very
  guarantee today's gate 3 provides). Membership is the exact `worktree $ANCHOR` line (top-level,
  not merely contained). A non-repo path yields empty output and fails the first-line comparison —
  the not-a-repo case still falls out of the same check, as today.
- Captured into a variable, never piped into `grep -q` under `pipefail` (the stub's own
  requirement; grep's early exit would otherwise race git's SIGPIPE status).
- `pwd -P` normalization is load-bearing on macOS: `mktemp -d` and user-supplied `/tmp/...` paths
  are symlinked (`/tmp` → `/private/tmp`) while git prints physical paths; without it the new
  exact-line match would falsely reject valid worktrees the old containment check accepted. It
  runs after the `-d` gate, so the `cd` cannot fail.
- Additionally, for **feature-scoped agents** (§2): `[ "$ANCHOR" != "$REPO_ROOT" ] || die` with a
  diagnostic naming the hazard — *"--worktree resolves to the main worktree; a feature-scoped
  agent must not run in the primary checkout on the integration branch"*. This is the wrong value
  the gate most needs to reject; membership alone still admits it.

Diagnostic wording now matches what is verified. The existing test-(g) phrase *"not a worktree of
this repository"* is retained (it becomes true), minimizing assert churn.

## 2. The gate keys on a declared agent scope (b)

0206's decision is general — *"feature-scoped agents must name their tree"* — but the
implementation enumerated one name shape. The equally feature-scoped `rebase-resolver`,
`integration-repair`, and `review-{lean,standard,deep}` stay ungated, two of which commit.

Mechanism — a declared fact, not a second name list:

- Every built-in agent source `agents/docket-*.md` gains a frontmatter key
  `worktree-scope: feature` or `worktree-scope: metadata` (16 files, one line each).
  Feature: the four `build-*` profiles, `rebase-resolver`, `integration-repair`, the three
  `review-*` rungs. Metadata: everything else (status, adr, implement-next, finalize-change,
  auto-groom + critic, brainstorm-consultant).
- **`sync-agents.sh`** validates the key is present and valid on every agent source it processes
  and fails generation loudly when absent — the enforcement seam sits where new agents get wired,
  so a future feature-scoped agent cannot ship undeclared. `emit_shim` keys its `--worktree`
  required slot on `worktree-scope: feature` instead of `case build-*`.
- **`runner-dispatch.sh`** reads the same key at runtime from
  `"${AGENTS_SRC:-$SELF_DIR/../agents}"/docket-$AGENT.md` with a one-line `sed -n` frontmatter
  probe. The *resolution path* mirrors the adapters' hardcoded `AGENTS_SRC="$SELF_DIR/../../agents"`
  (`codex.sh` line 13; depth adjusted — the facade sits one level shallower); the `${AGENTS_SRC:-}`
  env override is a **new** mock seam this change introduces, added to the facade's header
  `# Mock seams:` line (currently `RUNNERS_DIR, GIT`). `feature` ⇒ `--worktree` required + the
  main-tree rejection (§1). Missing file or key ⇒
  treated as metadata-scope (tolerant): generation is the loud seam, and the facade must keep its
  current behavior of letting the adapter report an unknown agent with the more specific
  diagnostic.
- Shim rule text generalizes: one feature-scoped wording ("this agent must run INSIDE the feature
  worktree it serves … if your caller named no worktree, abort-and-report") replacing the
  build-specific text; metadata shims stay byte-identical, preserving 0206's bidirectional guard
  (gated shims carry the slot, ungated shims carry none) now keyed on scope. Note: `emit()`
  passes source frontmatter through verbatim (only `model:`/`effort:` are rewritten), so
  `worktree-scope:` also appears in every generated Claude wrapper — harmless (Claude Code
  tolerates unknown frontmatter keys), but wrappers change bytes, so installs must re-run
  `sync-agents.sh` once for `--check` drift assertions to settle. Cursor/codex/opencode emitters
  build frontmatter from whitelists and are unaffected.
- **Dispatcher skills must supply the value the shims now demand.** `docket-build`'s dispatch
  prose already names the flag channel (SKILL.md "receives its worktree through the facade's
  `--worktree` flag") — but the callers of the five newly gated agents do not:
  `skills/docket-implement-next/SKILL.md` §6's review dispatch enumerates branch/base/title/
  learnings/evidence with no worktree, and `skills/docket-finalize-change`'s gate prose (SKILL.md
  + `references/gate-failure.md`) never names one for `rebase-resolver`/`integration-repair`.
  Without those sentences, every runner-delegated dispatch of the widened set deterministically
  aborts on the shim's "caller named no worktree" rule. Each gains one docket-build-shaped
  sentence naming the feature worktree as part of the dispatch payload. (A native — non-runner —
  dispatch is unaffected; the shim rule only exists in delegation shims.)
- `runner-dispatch.md` and the adapter contracts' affected sentences update from "build-* agents"
  to "feature-scoped agents (declared `worktree-scope: feature`)".

One new ADR records the rule — *an agent's worktree scope is a declared frontmatter fact; the
delegation gates (generation and runtime) key on the declaration, never on a name list* —
`relates_to: [34, 68]`, minted at build via docket-adr, id riding this change's `adrs:`.
ADR-0068 additionally receives a dated `## Update` note: its Consequences assert facts this
change falsifies ("the `build-*` gate is the one piece of agent-family knowledge the facade
gains"; "leaving every other shim byte-identical"), and the convention delivers a non-reversing
context change as an `## Update` pointing at the new ADR — with `68` also listed in this
change's `adrs:` so the body edit lands atomically (`adr-update-delivery` learning).

## 3. Flag-parse value guards (c)

Every flag currently parses as `--flag) VAR="${2:-}"; shift 2 ;;`. With the flag as the final
argument, `shift 2` at `$# = 1` shifts nothing and (no `-e`) the loop spins forever — a hang
where the facade's posture everywhere else is a loud `die`.

- Each of the five sites becomes: `--worktree) [ $# -ge 2 ] || die "--worktree requires a value"; WORKTREE="$2"; shift 2 ;;`
  (same shape for `--runner`, `--agent`, `--model`, `--effort`). The site list is derived at build
  time from `grep -n 'shift 2' scripts/runner-dispatch.sh`, not by hand.
- Non-goal: a valueless flag mid-argv still consumes the next `--flag` token as its value (e.g.
  `--model --effort high`). That is a wrong-value problem, not a hang, and detecting arg-shaped
  values is out of scope here.

## 4. Tests

`tests/test_runner_dispatch.sh` gains:

- **Success path (the review's paired gap):** `--agent build-economy --worktree
  <real feature worktree>` (created with `git -C "$SBX" worktree add`, not a bare `mkdir`) exits 0
  and the recorded argv's `-C` value is the feature worktree. Kills the mutant where `build-*`
  aborts unconditionally.
- **Membership:** `--worktree "$SBX"` (repo root) on a feature-scoped agent ⇒ nonzero, diagnostic
  names the main tree/integration branch; `--worktree "$SBX/docs"` (ordinary subdir) ⇒ nonzero
  with the membership diagnostic — the two values the old gate wrongly admitted. Existing legs
  (f) non-directory and (g) outside-repo stay green. The existing (b)/(c) fixture worktrees
  switch from `mkdir` to real `git worktree add` (they must now be actual members).
- **Scope coverage:** feature-scoped non-build agents (`rebase-resolver`, `review-lean`) without
  `--worktree` ⇒ nonzero naming the flag; `status` and `adr` without it still succeed.
- **Parse guards:** one leg per flag — trailing valueless flag exits nonzero with "requires a
  value". Bounded with a background-run + poll + kill helper (no `timeout(1)` dependency — macOS
  ships none and no existing test uses it), so a regression fails in ~5s instead of wedging the
  suite.

`tests/test_sync_agents_runners.sh`: the 0206 block re-asserts on scope — a feature-scoped
non-build shim (`review-lean` under a runner) bakes the slot; the missing-`worktree-scope` fixture
fails generation loudly; existing build-* and status asserts adapt to the generalized rule text.
Mechanism for the missing-key fixture: `sync-agents.sh`'s `AGENTS_SRC` is hardcoded
(`$SCRIPT_DIR/agents`, line 61, no seam), so the test copies the tree and strips the key from one
agent source in the copy — the mutation-fixture pattern `test_docket_status.sh` (~line 1258)
already uses — rather than adding a generator seam for one test.

## Out of scope

- Flag semantics/value resolution; arg-shaped flag values (§3 non-goal).
- 0207-review `sync-agents.sh` gate findings — change 0220's territory (the `worktree-scope`
  validation added here is a new gate, not one of 0220's findings).
- The exec→call-and-return handoff rewrite — change 0237's territory (see Assumptions).

## Assumptions

1. **Membership mechanism: one `worktree list --porcelain` capture from `$ANCHOR`; same-repo is
   the FIRST line equalling `$REPO_ROOT`, membership an exact `worktree $ANCHOR` line.**
   Rejected: an anywhere-in-list match for `$REPO_ROOT` (worktree lists retain stale records for
   deleted-and-recreated paths, so a foreign repo's list can name `$REPO_ROOT` — the critic
   reproduced this — and an anywhere-match would regress gate 3's foreign-tree guarantee);
   `git -C "$ANCHOR" rev-parse --show-toplevel` equality (a second git call, and the stub
   prescribes the list); piping into `grep -q` (pipefail hazard the stub names). Chosen shape
   does both proofs with one call, rests on the same list-main-first property as
   `docket_main_worktree`, and keeps the `GIT` mock seam.
2. **`pwd -P` physical normalization of `$ANCHOR` before matching.** Without it the exact match
   falsely rejects symlinked-but-valid paths (macOS `/tmp`); with it a symlink alias of a real
   member passes. Rejected: matching loosely (defeats the point); normalizing inside
   `docket_anchor_path` (would change a shared helper's contract for one caller).
3. **Main-tree rejection applies to the whole feature-scoped set, not `build-*` only.** The stub
   letter says build-*; the reason (never the primary checkout on the integration branch) holds
   identically for repair/resolver/review, and a scope-keyed gate makes the wider application one
   line. Rejected: build-only (re-creates the enumerated-floor gap inside the fix for it).
4. **Scope is a required frontmatter key on built-in agent sources, validated loudly at
   generation, read tolerantly at runtime.** Rejected: a hardcoded name list in facade +
   emit_shim (twin case statements that drift — `duplicated-gate-copies-the-whole-predicate`,
   and the stub itself calls a name list "an enumerated floor that ages into the gap");
   a separate scope-map file (second registry to drift); runtime-loud on a missing key (would
   shadow the adapter's more specific unknown-agent diagnostic and break non-built-in probes —
   the generation gate is where absence is preventable). Key spelled `worktree-scope` to avoid
   colliding with any harness's own frontmatter vocabulary.
5. **Facade reads the key from the docket checkout's `agents/`, via a NEW `${AGENTS_SRC:-}` mock
   seam.** The adapters already resolve sources from the same tree, but with a hardcoded
   `AGENTS_SRC="$SELF_DIR/../../agents"` and no env override — the facade's override is a seam
   this change introduces (documented in its `# Mock seams:` header line), not one it mirrors.
   Consumer repos run the facade from `DOCKET_SCRIPTS_DIR`, so `$SELF_DIR/../agents` exists
   wherever the facade runs. Rejected: reading the consumer repo's `.claude/agents/` (generated,
   harness-specific, absent for non-claude harnesses).
6. **Parse guard is the per-site `[ $# -ge 2 ] || die` shape.** Rejected: rewriting the loop into
   a shared consume-value helper (bigger diff, obscures the facade's deliberately flat style);
   `set -e` (changes the whole script's error posture, which is deliberately `-uo pipefail` only).
7. **Hang-regression tests bound with a background+poll+kill helper.** Rejected: `timeout(1)`
   (absent on stock macOS; no existing test depends on it); unbounded direct invocation (a
   regression wedges the suite — the failure mode under test).
8. **Diagnostic phrase "not a worktree of this repository" is retained.** It becomes accurate
   under the membership test; keeping it minimizes assert churn. The subdir case gets an
   additional clarifying clause.
9. **`depends_on: [237]` — 0237 is `in-progress` at groom time** (claimed 2026-08-07, branch
   `feat/prose-levers-...`, spec linked, reconciled; not yet merged). 0237 rewrites the facade's
   handoff tail (`exec` → call-and-return + verify-run); this change edits the parse loop, the
   gate block, and `emit_shim` — disjoint regions of the same file. Building 0208 after 0237
   merges avoids a same-file conflict and lets the build reconcile against the landed
   call-and-return shape; the design above does not otherwise depend on 0237's content. If 0237
   is abandoned, this design builds against the current `exec` tail unchanged.
10. **One new ADR for the declared-scope rule, plus a dated `## Update` on ADR-0068.** The
    declaration contract is a new decision (how scope is declared and enforced), not a reversal —
    but 0068's Consequences assert "the `build-*` gate is the one piece of agent-family knowledge
    the facade gains" and "every other shim byte-identical", both falsified here, so 0068 gets
    the convention's non-reversing `## Update` note pointing at the new ADR, with `68` in this
    change's `adrs:` for atomic delivery. Rejected: leaving 0068 untouched (knowingly stale
    prose); an `## Update` only, no new ADR (the contract is load-bearing for future agents and
    deserves its own searchable entry).
11. **Dispatcher skills gain the worktree sentence; shims stay strict.** Widening the shim rule
    without editing the callers would make every runner-delegated `review-*` /
    `rebase-resolver` / `integration-repair` dispatch deterministically abort — their dispatch
    prose (`docket-implement-next` §6, `docket-finalize-change` gate prose) names no worktree
    today. Chosen: one docket-build-shaped sentence per dispatch site. Rejected: softening the
    shim rule to "guess from context" (reopens the silent-wrong-tree hazard the rule exists to
    close); deferring the skill edits to a follow-up (ships a change that breaks its own
    delegations until the follow-up lands).
12. **Consolidation stands.** 0209 and 0210 are archived `killed` pointing here; their scope is
    carried verbatim above (checked against both archived stubs — nothing dropped).

## Reconcile 2026-08-11 — the post-0237/0270/0277 surface

Design intent is unchanged; five points where the file this change edits has moved since
2026-08-07, each one binding on the build. Where this section and the design above disagree about
a *fact*, this section is the current one.

1. **0237 merged (assumption 9 discharged).** The facade's tail is now call-and-return with the
   synchronous run gate, plus 0271's `--launch`/`--observe` verbs. The three regions this change
   edits — the parse loop, the gate block, `emit_shim` — remain disjoint from all of it.

2. **§1's main-tree rejection must exempt the observe anchor fallback.** `--observe` on a dispatch
   whose worktree has since been removed deliberately reassigns `ANCHOR="$REPO_ROOT"` and sets
   `ANCHOR_FALLBACK=1`, so the durable record stays readable; the build leg then reports
   `task-unverifiable worktree-removed`. A feature-scoped main-tree rejection applied blindly would
   `die` on exactly that path and convert a reported non-verdict into a failed observation. The
   rejection is therefore conditioned on `[ "$ANCHOR_FALLBACK" != 1 ]`. The membership test itself
   needs no exemption — `$REPO_ROOT` is a genuine member and passes it.

3. **§2 moves the `--worktree` requirement out of the `build-*` case, and nothing else.** That case
   block now carries a second, 0277-owned obligation: the empty-payload refusal (a `build-*`
   dispatch carrying neither `--brief-file` nor content-bearing trailing argv). That refusal stays
   keyed on `build-*`. Its reasoning is build-specific — a build worker with no task improvises in
   a worktree — and widening it to every feature-scoped agent would refuse dispatches that
   legitimately carry no payload. Only the `--worktree` requirement becomes scope-keyed.

4. **§2's runtime probe reads a path component, so it is shape-guarded.** `$AGENT` has no shape
   validation today (only `$RUNNER` does), and the probe turns it into a path under
   `${AGENTS_SRC:-$SELF_DIR/../agents}`. The probe therefore runs only for a name matching the same
   safe class `--runner` is held to (`[A-Za-z0-9._-]` with no `..`); any other name yields no
   declared scope and falls to the tolerant metadata default — the same outcome as a missing file
   or key, and the same reason: the adapter's unknown-agent diagnostic is the more specific one.

5. **§3's site list is still exactly the five `shift 2` flags.** 0271 (`--observe`) and 0277
   (`--brief-file`) each landed with their own last-argument guard in a different but equally
   non-hanging shape (`shift; [ $# -gt 0 ] && shift`), and both carry an in-file comment explaining
   it. The `grep -n 'shift 2'` derivation yields `--runner`, `--agent`, `--model`, `--effort`,
   `--worktree` and no others; the two already-guarded sites are left byte-identical rather than
   re-shaped, which keeps this change's diff to the sites that can actually hang.

6. **§4's success-path leg is already substantially on the branch — extend it, do not duplicate.**
   0270's config-locality section builds a real linked worktree with `git worktree add`, dispatches
   `--agent build-economy --worktree "$WT" -- "<task>"`, and asserts the anchor handed to the
   adapter is the linked worktree and is not the main worktree. That is the paired success path the
   review asked for, minus an explicit exit-code assert; this change adds the exit-code conjunct
   there rather than authoring a second near-identical fixture. The new membership and scope legs
   are still authored fresh. Separately, every `build-*` leg in this file must now carry a payload
   to reach the adapter at all (0277's gate fires first) — the existing 0206 legs (d) and (e) are
   refusal legs and are unaffected, but any new success-shaped `build-*` leg needs one.

7. **Budget.** `tests/test_runner_dispatch.sh` sits at a 20s row (raised from 10s by 0277), so the
   new legs have real margin. If the additions push the measured wall clock past it, re-measure and
   raise the row with the measured number rather than leaving a trailing `OVER BUDGET:` line.
