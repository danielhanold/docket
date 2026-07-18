# Auto-create discovered stubs — results
Change: #0091 · Branch: feat/auto-create-discovered-stubs · PR: <url> · Plan: docs/superpowers/plans/2026-07-18-auto-create-discovered-stubs.md · ADRs: 45, 46

## Verify (human)

The hermetic suite sees only its fixtures and the integration-branch checkout — it cannot see the
`docket` metadata branch where stubs actually land. These are the checks that live outside it.

- [ ] **Enable the knob once, deliberately.** `auto_capture` ships `false`; this repo has not opted
      in. Setting `auto_capture: true` (repo, global, or `.docket.local.yml`) is the only way to see
      the feature run for real. Suggested first exposure: a machine-local `.docket.local.yml`, so a
      surprise is contained to one machine.
- [ ] **Watch the first real mint.** On the first autonomous run after opting in, confirm the minted
      stub carries `discovered_from: [<originating id>]`, lands as `needs-brainstorm` on the board,
      and that the run report names it. The mint path is covered by tests driving a real bare-origin
      repo, but no stub has yet been minted by a *skill* rather than by a test.
- [ ] **Judge the materiality bar in practice.** The bar ("would a human file this as a
      `docket-new-change`?") is model judgment, not a script. The thing to watch for over the first
      few runs is not failure — it is *noise*: stubs you would not have filed. That is the signal to
      tighten the bar's wording, and it can only be read from real output.

## Findings

**Two decisions became ADRs.**

- **ADR-0045 — auto-capture is best-effort.** A failed mint never aborts the change being built.
  This is a deliberate, narrow exception to docket's house abort-and-report exit-code posture, and
  the final review caught its absence as the branch's one Important finding: exit 1 is reachable in
  *ordinary* operation (contention in the shared `.docket` worktree; `main`-mode, where the
  `.docket/<changes_dir>` path does not exist; a body file missing its `## Why` opener), so without
  the exception stated where implementers read it, a routine capture failure would abort a build.
- **ADR-0046 — CAS `reset --hard` needs a tracked-files-only clean-tree precondition.** The change-0089
  learnings note predicted that a second user of the fresh-origin CAS reset should graduate the
  pattern to an ADR. `mint-stub.sh` is that second user, and it hit a hazard the first did not:
  `reset --hard` destroys *any* uncommitted work, and this script runs inside the metadata worktree
  that other agents share. Review reproduced real data loss (an unrelated uncommitted change file
  wiped, with the script still reporting success), then reproduced the *over*-correction too — gating
  on plain `git status --porcelain` counts untracked files, so a stray `.DS_Store` hard-failed the
  mint on exactly the contended path the feature exists for.

**Three review findings worth remembering beyond the ADRs.**

- **A lifecycle-gate bypass, caught pre-merge.** A multi-line `--title` could inject arbitrary
  frontmatter into the minted stub. Because `field()` returns the *first* match, an injected
  `trivial: true` landed ahead of the template's `trivial: false` — and a `trivial` stub reads as
  **build-ready**, so an ungroomed stub could have skipped the human grooming gate entirely. Fixed by
  rejecting control characters at argument validation; mutation-proved (17 assertions redden with the
  guard stripped).
- **Model-authored prose is not a script constant.** The first implementation interpolated the title
  straight into a `sed` replacement, copying `reclaim-claims.sh`'s `set_field`. That is safe there —
  it only ever writes generated constants. Here the value is English written by a model, where `&`
  ("Cleanup & dedupe") is unremarkable, and it silently produced titleless or mangled change files
  that were then *pushed*. Now written via `awk`'s `ENVIRON`, which does not reinterpret the
  replacement.
- **Green tests said nothing about either.** Both were invisible to a suite that was fully green at
  the time, because no fixture used a title with punctuation and none combined an empty `active/`
  with a forced retry. Every fix on this branch is now pinned by a fixture that was mutation-proved
  to redden — including a two-sided proof for the clean-tree gate, since a one-sided test would have
  accepted either the over-broad gate or no gate at all.

**Verified at build time, outside the suite:** no docket metadata (change file, `BOARD.md`, ADR)
leaked onto the feature branch; the default-off path resolves `AUTO_CAPTURE=false` in a repo with no
`.docket.yml` and writes nothing; the emitted `## Artifacts` markers are byte-identical to
`render-change-links.sh`'s, so a minted stub's block is refreshed rather than orphaned.

**Process note.** Concurrent autonomous loops share this repo. Twice during the build, work appeared
in the feature worktree that this run had not dispatched. It was never adopted blind: once reverted
and preserved outside the repo, once (the auto-capture semantics refinement) put through the final
whole-branch review as a diff and committed only after it was judged correct — with one correction
the reviewer caught, where an absolute "never reset per mint site" collided with `docket-status`'s
per-swept-change cap scope.

## Follow-ups

- **Two implementations of the mint routine now coexist.** The spec (§2) recommended that
  `docket-new-change` and the autonomous skills share one CAS-correct mint. This change built
  `mint-stub.sh` but left `docket-new-change`'s hand-rolled allocate/CAS prose in place, so the
  routine exists twice. Worth a change to fold new-change onto the helper — the risk is the usual
  one, that the two drift and only one gets a fix.
- **Deferred minors in `scripts/mint-stub.sh`**, all reviewed and consciously left: a duplicate-match
  message prints an empty id when the matched file lacks `id:`; `REL` degenerates if `--changes-dir`
  is the worktree root (unreachable from documented call sites); the control-character guard is
  exercised only with newlines, not tab/CR (the guard is shape-keyed, so this is test breadth, not a
  hole).
- **The per-invocation cap is a hardcoded 3.** Promoting it to config is deliberately out of scope
  (spec §6) and stays a follow-up if the constant proves too rigid once the knob sees real use.
- **`auto_capture` is inert in `main`-mode.** The documented invocation names a `.docket/…` path that
  only exists in `docket`-mode. The convention now says so, but making the feature actually work in
  `main`-mode is unbuilt.
