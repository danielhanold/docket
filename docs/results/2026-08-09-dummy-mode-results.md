<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0276 — Dummy mode — persona-calibrated human-facing language simplification](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0276-dummy-mode.md)**
<!-- docket:backlink:end -->

# Dummy mode — persona-calibrated human-facing language simplification — results

Change: #0276 · Branch: feat/dummy-mode · PR: (opened at close of this run) · Plan: docs/superpowers/plans/2026-08-09-dummy-mode-plan.md · ADRs: none

## Verify (human)

- [ ] Set `dummy_mode.enabled: true` with a persona of your own in `.docket.local.yml`, then run an
      interactive groom or new-change and judge whether the dialogue actually reads for that reader.
      Nothing automated can check this: the simplification is LLM-authored prose, and the suite
      deliberately asserts only that the rule and its pointers exist, never how well a run applies them.
- [ ] Read the five-persona gallery in `README.md` as a first-time user and decide whether you could
      write your own persona from it without reading the spec.

## Findings

**The build gate went red twice on a defect that predates this branch.** Both reds were the
`producer | early-exiting-consumer` shape under `set -o pipefail` that AGENTS.md forbids: the
producer takes SIGPIPE, and pipefail promotes the 141 into an intermittent test failure. The
first red was `tests/test_docket_example_yml.sh` (`git show … | grep -q`), the second
`tests/test_docket_status.sh` (`printf … | grep -qF`) — a different file each time, which is what
established it as a class rather than a bug.

The measured mechanism, worth recording because it explains why the suite looked healthy for so
long: `.docket.example.yml` is about 40KB, larger than a macOS pipe's 16KB default. Unloaded, the
kernel grows the pipe to 64KB, the whole payload fits, and the producer finishes before the
consumer closes the read end — 0 failures in 70 runs. Under parallel suite load the kernel declines
to grow the pipe and the producer blocks mid-write — 70 failures in 70 runs. The defect has been
latent since the example crossed 16KB; change 0276 only shifted the load profile enough to lose the
race. The initial hypothesis that this branch's own additions to the example tipped it over was
tested against the pre-branch file and **refuted** — it fails identically at 38682 bytes.

The repair swept the class rather than the two files that happened to fail: **415 executable sites
across 49 shell files**, derived from a whole-repo shape grep, remedied as 361 here-string
rewrites, 84 de-quieted terminal stages, and 47 `head -N` → `sed -n 1,Np`. All 92 negated asserts
were preserved verbatim and none went red afterward, so no previously-vacuous guard was masking a
real defect. `tests/test_pipe_shapes.sh` was added to stop the class returning.

**The negated-assert inversion is the part worth remembering.** In `! producer | grep -q pattern`,
SIGPIPE fires only when the consumer exits early — which happens only when it *matches*. The 141
then inverts through the leading `!` into a **green** assert at exactly the moment the guard should
have fired. AGENTS.md already names this; seeing it live in four guards is the reason to keep
taking it seriously.

**Review (deep rung) returned 8 findings — 0 blockers, 4 important, 4 minor — and all 8 were fixed
in-branch.** Two are worth carrying forward as design notes rather than as history:

- The persona `#` refusal originally keyed on the *residue* of truncation (an unbalanced leading
  quote) rather than on truncation itself. It therefore missed the unquoted form entirely — a
  fragment was exported silently while three documents promised a hard error — and it aborted on a
  legal persona whose text simply began with a quote character. Since `docket-config.sh` runs at
  every skill's Step 0, that second limb would have bricked the tool for such a repo. It now reads
  the raw leaf before normalization can eat the `#`.
- The repo-wide pipe-shape guard first enumerated `grep` and `head`. `awk … exit` closes stdin the
  same way, and three live sites survived the sweep — two of them in the very file that had gone red
  twice. The predicate is now keyed on shape across `grep`, `head`, `awk … exit`, `sed … q`, and
  `read`, with an optional path prefix, and its `KNOWN IMPRECISION` block was widened to match what
  it actually delivers.

**No ADRs were minted.** Every non-obvious decision this build made was already owned by an existing
record: the single-line-scalar persona constraint and the block-scalar refusal by the reconciled
spec, the literal (never expanded) `all` export by `scripts/docket-config.md`, and the
keep-it-fatal choice for a `#`-bearing persona by the spec's own "detected and refused rather than
exported as a fragment". None of them decides anything at the altitude the ADR ledger is for.

## Follow-ups

- **Change 0280** (minted by this run) — shard or re-budget the test files the suite runner reports
  `OVER BUDGET`. Ten to twelve files breach on every run, led by the `sync_agents` family
  (`test_sync_agents_runners` at 227s against a 60s ceiling). The advisory does not fail the run, so
  nothing forces the shard; the suite's tail is dominated by these files.
- **A residual gap in the persona `#` refusal, deliberately left.** `persona:#foo` with no space
  after the colon resolves to empty and falls through to the shipped default rather than aborting.
  YAML itself is ambiguous there and no documented example produces the shape, so the reader cannot
  honestly distinguish it. Recorded rather than papered over.
- **The `#`-refusal's blast radius is scoped to the winning layer.** A `#`-broken persona in a layer
  that a higher layer overrides is intentionally not fatal, so a repo that has already fixed its
  config is never held hostage by a stale global one. Two asserts pin that scoping.
- **Roughly 84 same-shape pipe sites remain outside the swept set** in the original count, over
  bounded payloads (heaviest historically `test_docket_status`, `test_closeout`, `test_render_board`).
  The final sweep and the widened guard closed the executable ones; the guard now fails the suite if
  a new one lands, so no separate change is proposed.
