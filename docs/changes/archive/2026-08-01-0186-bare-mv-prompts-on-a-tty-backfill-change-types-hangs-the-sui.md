---
id: 186
slug: bare-mv-prompts-on-a-tty-backfill-change-types-hangs-the-sui
title: Bare mv prompts on a tty — backfill-change-types hangs the suite and can exit 0 without installing
status: done
priority: high
type: fix
created: 2026-08-01
updated: 2026-08-01
depends_on: []
related: [134, 150, 178]
discovered_from: [185]
adrs: []
spec: docs/superpowers/specs/2026-08-01-bare-mv-prompts-on-a-tty-design.md
plan: docs/superpowers/plans/2026-08-01-bare-mv-prompts-on-a-tty.md
results: docs/results/2026-08-01-bare-mv-prompts-on-a-tty-backfill-change-types-hangs-the-sui-results.md
trivial: false
auto_groomable: true
branch: feat/bare-mv-prompts-on-a-tty-backfill-change-types-hangs-the-sui
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/148
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-01-bare-mv-prompts-on-a-tty-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-01-bare-mv-prompts-on-a-tty-design.md) |
| Plan | [2026-08-01-bare-mv-prompts-on-a-tty.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-01-bare-mv-prompts-on-a-tty.md) |
| Results | [2026-08-01-bare-mv-prompts-on-a-tty-backfill-change-types-hangs-the-sui-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-08-01-bare-mv-prompts-on-a-tty-backfill-change-types-hangs-the-sui-results.md) |
<!-- docket:artifacts:end -->

## Why

`tests/test_backfill_change_types.sh` hangs forever when the suite is run from an interactive
terminal, which makes the full local suite unfinishable for a human — the exact environment a
maintainer runs the merge gate in. It passes green in every non-tty context (agent shells, the
finalize gate), so the suite has been reporting clean while being unrunnable by hand since
2026-07-23 (change 0127).

**The hang.** `scripts/backfill-change-types.sh:161` installs each staged file with a bare `mv`.
The test (line 201) makes one destination immutable via `chflags uchg` to force a mid-loop install
failure and exercise the rollback path. BSD `mv` handles an unwritable destination by *prompting*
— `override rw-r--r-- homer/wheel for dst.md? (y/n [n])` — but only when stdin is a terminal. In a
terminal it blocks on that read forever; with stdin at /dev/null or a pipe it skips the prompt,
fails `EPERM`, and the rollback assertions pass.

**The worse bug the hang is hiding.** Probed under a pty with stdin at EOF, `mv` declines the
overwrite and exits **0**:

```
override rw-r--r--  homer/wheel for dst.md? (y/n [n]) not overwritten
MV_RC=0
```

So there is an environment where the staged file is silently not installed, `if ! mv` never fires,
no rollback runs, and the script reports success — a half-migrated backlog with a zero exit. That
is precisely the outcome the test's own comment (lines 186–191) says the install's undo exists to
make impossible. The guard is sound only because the environments that run it never have a tty.

Diagnosed 2026-08-01 while investigating why `scripts/profile-one-test.sh
tests/test_backfill_change_types.sh` produced no output; the process tree showed the profiler
blocked 6m47s in `backfill-change-types.sh --map 1=fix,2=docs,4=chore` against the `rollback`
fixture.

## What changes

- **`mv -f` at `scripts/backfill-change-types.sh:162`** (the stub said 161; 162 is the verified
  line) — suppresses the prompt and returns non-zero on `EPERM`, preserving the test's intent rather
  than working around it. Probe-confirmed. This is the only part that unblocks the suite.
- **The `cp -p` twin at line 164** (rollback restore) and line 155 (backup stage) were **audited
  during grooming: no prompt exposure, no change** — `cp` prompts only under `-i`, and `-f` on the
  restore path would unlink the very destination the undo preserves. Recorded in the spec so the
  diff stays one line plus its guards.
- **Pin the property in both environments** with two layers: the existing rollback scenario re-run
  through a `script(1)` pty (gated on an exit-status-fidelity probe, `-e` on util-linux, stdin from
  `/dev/null`, stdout capture, CR normalization, and a widened `uchg` cleanup trap), plus a
  call-site-anchored source sentinel that always runs where the pty layer skips.
- **Profiler discoverability fixes** to `scripts/profile-one-test.sh` and
  `scripts/profile-asserts.sh`: print the artifact paths *before* launching the child, and add a
  per-test `running <t>` line inside `profile-asserts.sh`'s loop. Purely additive on the existing
  stream. The stub's stdout-buffering diagnosis was **disproved** during grooming and is not built
  on — see the spec's §4.
- **A learnings recommendation** (the close-out harvest decides, and it is a three-way choice among
  `shell-portability`, `green-suite-untested-branch`, and `agent-shell-noop-reads-as-success`): a
  guard that forces a failure via a filesystem flag is sound only if the tool under test does not
  prompt — non-tty stdin was concealing both a hang and a silent-success path.

## Out of scope

- No broader audit of BSD-vs-GNU prompting across the whole script tree. This change fixes the one
  proven site, its immediate twin, and records the rule; a sweep is its own change if the finding
  justifies one.
- No change to how the suite is invoked (no blanket `</dev/null` at the runner level) — that would
  re-hide this class of defect rather than fix it.

## Open questions

- ~~pty run vs. source sentinel for pinning the tty property?~~ **Resolved (2026-08-01, grooming):
  both.** The pty layer tests behavior but is skippable (and `script(1)` diverges across BSD and
  util-linux in argument order *and* exit-status propagation); the sentinel always runs but is text
  over source. Neither alone is sufficient. Mechanics in the spec's §3, rationale in A3.

## Reconcile log

### 2026-08-01 — reconcile at claim

Verified every call site and line number the spec cites against `origin/main` at `075fbb1b`. All
four hold exactly as written, no drift:

- `scripts/backfill-change-types.sh` — the bare `mv` install is at **162**; the rollback-restore
  `cp -p` at **164** and the backup-stage `cp -p` at **155**. The spec's correction of the stub's
  "line 161" stands.
- `tests/test_backfill_change_types.sh` — the `chflags uchg` guard opens at **201**, the
  single-fixture cleanup trap is at **203** (`chflags -R nouchg "$drb"` — confirmed scoped to
  `$drb` only, so the spec's §3 constraint 3 is a real gap, not a hypothetical), and the
  `2>&1 >/dev/null` capture idiom the pty layer must not inherit is at **204–205**.
- `scripts/profile-one-test.sh` — the `tracing …` line is at **77**, already pre-launch (the child
  runs at 83–86); the end-of-run `trace:`/`stdout:` summary is at **137–138**. So §4's premise
  correction is accurate and the work is purely the pre-launch path emission.
- `scripts/profile-asserts.sh` — `profiling %d test file(s)` at **108** is outside the per-test loop
  (**112–123**, run at **116**); the TSV path prints only at **150**. The verified
  per-command-flush comment §4 leans on is at **15–17**.

Scope unchanged. No work has landed elsewhere: the three `related` changes (#0134, #0150, #0178) are
all still `proposed`/unbuilt, so A6's "related, not depends_on, buildable today" holds. The only
commits touching these four files since the change was drafted are change 0185's profiler additions
(`0003608f`, `0d9efc7a`) — which are what §4 builds on, already present as described. No new ADRs
bear on the design. Nothing dropped, nothing added.
