<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0270 — Fence runner-config locality at the main worktree (regression test + contract correction)](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-10-0270-fence-runner-config-locality-at-the-main-worktree.md)**
<!-- docket:backlink:end -->

# Fence runner-config locality at the main worktree — results

Change: #0270 · Branch: feat/fence-runner-config-locality-at-the-main-worktree · PR: (opened at close of this run) · Plan: docs/superpowers/plans/2026-08-10-fence-runner-config-locality-at-the-main-worktree.md · ADRs: none

## Verify (human)

- [ ] Confirm the reworded `scripts/runners/opencode.md` Prerequisites bullet matches how you
      actually set the grant on your own machine. It now says the grant is read at the **main
      worktree** and that the facade resolves its layers there regardless of `--worktree`. Your
      real `~/.config/docket/config.yml` carries `runners.opencode.permissions`, so the global-layer
      path is the one you exercise — the bullet should read correctly for it.
- [ ] `tests/test_runner_dispatch.sh` now measures **10s under parallel contention** against its
      **10s** ceiling in `tests/runtime-budgets.tsv` (6.0s serial, up from 5.79s). It did not trip
      the budget check — the runner's slack factor absorbed it — but it is now the closest file in
      the suite to its own ceiling. Decide whether to raise that row before the next change adds to
      this file; changes 0208 and 0277 are both queued against it.

## Findings

**The mutation probes the spec made mandatory — both reddened, both reverted.** `scripts/runner-dispatch.sh`
is byte-identical to `origin/main` on this branch, verified by SHA-256 across every probe and by
`git diff origin/main --exit-code`.

| Probe | Mutation | Result |
|---|---|---|
| 1 | config loop re-anchored: `for f in "$ANCHOR/.docket.local.yml" …` | RED on `0270: main-worktree grant reaches the child across a --worktree dispatch`; **no other assert in the file reddened** |
| 2 | `export DOCKET_REPO_ROOT="$REPO_ROOT"` | RED on both anchor asserts, plus three pre-existing `0206` anchor asserts — all legitimate dependents of the mutated export |

**The fence was latent decoration on any machine but this one — caught at review, and it is the
sharpest thing this change learned.** As first written, the new section did not pin
`DOCKET_HARNESS_ROOT` into its sandbox. The file `unset XDG_CONFIG_HOME`s at the top, and the facade
resolves `GLOBAL_CFG="${XDG_CONFIG_HOME:-${DOCKET_HARNESS_ROOT:-$HOME}/.config}/docket/config.yml"`,
so a direct `bash tests/test_runner_dispatch.sh` run read the developer's **real**
`~/.config/docket/config.yml`. On any machine whose global config sets the documented
`runners.codex.sandbox: danger-full-access` knob — spelled exactly as the fixture spells it — the
global layer alone satisfies the grant assert, no main-worktree read happens, and mutation probe 1
stays green. The fence would have been decoration while appearing to have survived its own probe.

It was load-bearing here only by accident: this machine's global config carries
`runners.opencode.permissions`, not `runners.codex.sandbox`. The fix (`deb74293`) pins
`DOCKET_HARNESS_ROOT="$SBX"`, matching the idiom three sibling sections in the same file already
use. The fix worker confirmed the finding empirically rather than by argument — it built a fake
`HOME` carrying the global grant and showed probe 1 staying **green** there before the fix and
reddening after it.

Two general lessons, both worth the learnings ledger at close-out:

1. **A hermetic-looking fixture inherits every config layer it does not explicitly pin.** The
   suite runner sandboxes `HOME` per job, so the gate was hermetic and green — the hole was only
   observable in the direct-invocation mode that the mutation probes themselves use. A guard whose
   non-vacuity depends on which mode you run it in is not yet a guard.
2. **A mutation that silently fails to apply prints an all-green run that reads as a surviving
   guard.** The premium fix worker's first probe re-run used a `perl -0pi` one-liner that died on a
   syntax error *before writing*; the unmutated run came back all-green. Read naively that is the
   worst available outcome — "the guard survived its mutation" — when in fact no mutation happened.
   It now gates every probe on `git diff` showing the changed line before believing the reading.
   This is the mutation-hygiene sibling of the existing `mutation-restore-needs-a-backup-copy`
   finding: restore correctness and *application* correctness are two separate failure modes of the
   same three-step procedure.

**Review disposition.** Four findings, zero blockers; all four fixed in-branch across three tasks.
Findings 2 and 4 were accuracy corrections to prose this change itself introduced — the "anti-vacuity
pair" comment overclaimed (mutation probe 2 showed both legs always redden together, and `grep -x`
makes the spec's prefix-match rationale inapplicable), and the new opencode bullet justified "never
inside a feature worktree" with the gitignore reason, which is false for the *committed*
`.docket.yml` it named in the same sentence.

**No production code changed**, which was the spec's central claim and is the one thing most worth
re-checking at merge: the branch's only `.sh` file is `tests/test_runner_dispatch.sh`.

## Follow-ups

- None minted. All four review findings were about this branch's own diff, so none was mintable;
  nothing surfaced during reconcile or review cleared the auto-capture admission gates.
- `tests/test_sync_agents_runners.sh` is over its 60s ceiling (184s) in both gate runs. Pre-existing
  and untouched by this change — noted only so the `OVER BUDGET` line in this run's evidence is not
  misread as belonging to it.
