<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0366 — Human-attended v1.0.0-rc1 acceptance and publication](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0366-human-attended-v1-0-0-rc1-acceptance-and-publication.md)**
<!-- docket:backlink:end -->

# Human-attended v1.0.0-beta1 acceptance and publication

**Change:** 0366 · **Type:** chore · **Priority:** critical · **Date:** 2026-09-03 · **Status:**
Approved design

## Purpose and boundary

This change takes one reviewed commit of `main` to a verified, public `v1.0.0-beta1` pre-release
of the Go-only Docket, under human attendance at every external-truth and irreversible boundary,
and closes the migration's metadata (backlog ledger, learnings, release evidence) through the
installed Go product.

The approved [Go migration program map](2026-08-12-go-migration-program-map.md) and
[architecture](2026-08-12-go-migration-architecture-design.md) are governing constraints and are
not reopened here: supported targets (Darwin/Linux × amd64/arm64), the four direct host harnesses,
the hard cutover with no Bash fallback, the deferred/dropped capability lists, and `v0.9.2` as the
frozen Bash baseline and documented rollback artifact all stand.

This spec resolves only 0366's independently deliverable scope. It builds no product code. A
source defect found anywhere in the protocol returns to a separate reviewed change, invalidates the
candidate, and restarts packaging from the new merged commit.

## What changed since the stub was written (2026-08-29)

The stub assumed the candidate would be cut from "the exact merged 0370 commit" and that the
release window would bracket 0370's merge. Reality on 2026-09-03:

- The source cutover chain is complete: 0318, 0369, 0371, 0372, and 0370 are all `done`
  (0370 archived 2026-08-31). Fourteen further changes merged after 0370: 0367, 0389, 0390, 0384,
  0374, 0394, 0364, 0396, 0397, 0359, 0373, 0395, 0399, 0400. A 0370-commit binary would reject
  Docket's own current `.docket.yml` (the `build:` block from 0374) and lack `repository
  configure-tests`, `maintenance preflight`, `capabilities`, and `schema`. **The candidate is
  therefore cut from the current tip of `origin/main`, not from 0370's merge commit.**
- The latest published tag is **`v0.9.3`** (2026-08-15, a Bash-era agent-registry release with
  no binary assets). `main` is 722 first-parent commits past it. `v0.9.2` remains the frozen Bash
  baseline and the documented rollback artifact (program map, Global Constraints and the 0324
  addendum; ADR-0096's frozen floor). Both facts must be stated in the notes; nothing may imply
  that fixture versions `v0.9.4`–`v0.9.7` under `testdata/repositories/` are tags.
- The pre-release is named **`v1.0.0-beta1`**, not `v1.0.0-rc1` (human decision at groom time).
- The candidate workflow (`.github/workflows/release-candidate.yml`) now runs its source gate on
  `macos-15` with Homebrew Bash and `go run ./cmd/docket development test` (0361, 0370). It is
  still non-publishing and read-only, with `workflow_dispatch` inputs `ref` and `version`.
- Every skill's Step 0 now runs `docket repository prepare` (0377) and resolves verbs from
  `docket capabilities` and payloads from `docket schema` (0394, 0399); implement-next's Step 0 is
  an inline `docket maintenance preflight` (0397). There is no board-refresh verb: the board is
  rendered inside owning transactions, drift is surfaced by `repository check`, and repaired only
  by an authorized `docket repository migrate` (0377).
- Only `docket learning record` and `docket learning update` survive of the learnings subsystem;
  harvest, index rendering, capacity, and promotion are retired fail-closed (0372).
- Native host dispatch is the only dispatch (ADR-0100). Codex coordinator entry through app-server
  root threads is decided (ADR-0103) but the implementing change 0393 is `implemented` with PR
  #265 open, not merged.
- The stub's "no maintenance sweep during the window" rule has already been overtaken: fourteen
  finalizes ran after 0370, and every implement-next start runs an implementation-scope sweep. The
  rule is re-specified below as a quiescence window bracketing the candidate commit.

## Current reality snapshot (2026-09-03)

| Fact | Value |
|---|---|
| `origin/main` head | `6348798a4991ab26c61ea983e104914d6aa296cb` (change 0400 close-out) |
| Latest release / tag | `v0.9.3` (no assets); older: `v0.9.2` (frozen Bash baseline) |
| Candidate workflow | `release-candidate.yml`: `permissions: contents: read`; `workflow_dispatch` inputs `ref`, `version`; jobs `source-gate` (macos-15) → `package` (`cmd/releasepkg`, once) → `smoke` (4 native runners, `scripts/release-smoke.sh`) → `summary` (`candidate-evidence` artifact, `evidence.json`) |
| Version grammar | `^v(0\|[1-9][0-9]*)\.(0\|[1-9][0-9]*)\.(0\|[1-9][0-9]*)(-[0-9A-Za-z][0-9A-Za-z.-]*)?$` (`internal/release/version.go`, mirrored in the workflow) — dots are mandatory |
| Bundle contents | `docket_<version>_{darwin,linux}_{amd64,arm64}.tar.gz`, `checksums.txt`, `install.sh` (rendered downloader) |
| Downloader | `internal/release/downloader/install.sh`; base `https://github.com/danielhanold/docket/releases/download/<version>`; `DOCKET_RELEASE_BASE_URL` is the one mirror seam; `--harness` forwarded verbatim to `docket install` |
| Retained POSIX products | repo-root `install.sh` (contributor bootstrap, 0322); the release downloader; `scripts/release-smoke.sh`; `scripts/runners/{codex,cursor,opencode}.sh` (kept by 0386's disposition). `link-skills.sh` at the repo root is undocumented — a release-notes watch item |
| Open at release time | 0393 (`implemented`, PR #265 open); build-ready 0392, 0401, 0402, 0403 |
| Operator machine | darwin/arm64 with a **development** (source-linked) install: `docket install check` reports `mode: development`, harnesses claude/codex/cursor/opencode. It must not leak into fresh-host evidence |
| Publishing automation | none — no workflow carries a write token; publication is human-typed `gh` |

## Naming and versioning

- **Git tag `v1.0.0-beta1`**, annotated, at the candidate commit. The human's instruction spelled
  it `v1-0-0-beta1`; that is the slug form. The packager, the workflow's version grammar, and Go
  module semver all require the dotted form, so the tag and every asset name use `v1.0.0-beta1`
  (`docket_v1.0.0-beta1_darwin_arm64.tar.gz`, …).
- **Why beta, not rc.** This is the first public build of the Go product: a feedback-seeking
  pre-release over an existing user base of Bash installs, with known gaps carried in the notes.
  `rc` is reserved for a feature-frozen candidate of stable `v1.0.0`; stable promotion stays out of
  scope.
- **GitHub Release**: title `v1.0.0-beta1 — Docket is a Go binary`, marked `--prerelease`, explicitly
  **not** marked latest (`--latest=false`), so `v0.9.3` remains "Latest" until stable `v1.0.0`.
  The downloader never resolves `latest`; it addresses the version path directly.
- **Release-note scope** is "since `v0.9.3`": the Go foundation (0304–0317, already teased in
  `v0.9.3`'s notes) summarized in one paragraph, and everything since 0318 in themes.
- **Change record naming.** The record's slug (`human-attended-v1-0-0-rc1-…`) is an identifier and
  stays; the minted branch will carry it. The `title:` still says rc1; retitling is a
  frontmatter edit outside this transaction's owned fields (human-typed edit plus
  `docket repository migrate` to re-render the board) and is optional.

## Roles and vocabulary

- **Operator** — the human plus the attended agent session on the operator machine, driving
  cataloged `docket` operations (resolved from `docket capabilities --json`, payloads from
  `docket schema --operation <id>`) and explicit `git`/`gh` argument vectors. The operator is the
  only actor; no autonomous Docket loop runs against the Docket repository inside the window.
- **Candidate** — the triple (candidate commit SHA, workflow run id, bundle `checksums.txt`),
  plus one immutable downloaded copy of the bundle that every later phase reads.
- **Window** — from the moment the candidate commit is recorded (Phase 1) to the closeout PR
  merge (Phase 10). Inside it, no PR merges into `main` and no autonomous Docket run touches the
  Docket repository.
- **STOP** — halt before the next effect, write the gate and reason to `decisions.md` in the
  evidence bundle, and wait for a human decision. Resumption always re-probes authoritative
  Git/GitHub state and the recorded checksums; local files, elapsed time, or a previously typed
  command never establish success.
- **Evidence bundle** — `docs/release/v1.0.0-beta1/` on `main`, delivered by this change's own
  PR (layout below), plus the results record.

## Protocol

Phases are gates: each names its preconditions, actions, evidence, and STOP conditions. Phase 2
touches only the metadata branch and may overlap Phases 3–4 (which are mostly workflow wall
clock), but must complete before Phase 8.

### Phase 0 — Pre-cut decisions (human agenda)

Decide, record in `decisions.md` (decision, who, when, merge SHA or "held"), and only then open
the window. Recommendations, not verdicts:

| Item | State | Recommendation |
|---|---|---|
| 0393 Codex coordinator root entry | `implemented`, PR #265 open | Verify what PR #265 proves (its record still carries a `## Disposition: HALTED` narrative from an earlier attempt). Merge, finalize, and live-certify before the cut: without it the Codex lifecycle row in Phase 5 cannot reach `docket-plan-writer`. If it cannot land, beta1 ships with Codex composition documented as unproven in the notes' known gaps, and the Codex row records the failure mode rather than a pass. |
| 0401 source-available license | build-ready | Land before the tag. This is the first public distribution of binaries; the `LICENSE` lives in the repository at the tagged commit and is linked from the notes. |
| 0392 installer-tolerant config read | build-ready | Land before the cut. Without it the first post-beta1 schema-extending release deadlocks `docket install` on every beta1 machine (the recorded 0374 bricking). |
| 0402 docs restructure, 0403 config diagnostics | build-ready | Optional. 0402 carries 0385's fix to `docs/cursor/permissions.md`, which still allowlists the deleted `scripts/docket.sh`; if 0402 is held, the notes carry a known-gaps line instead. |
| Stale prose the notes might cite | — | `.docket.example.yml` and `docs/guide/README.md` still describe the removed `metadata_branch: main` opt-out (0363). The notes never link stale prose as install guidance; the Install section of the notes is self-contained. Fixing the prose is a separate docs change. |

Exit condition: no change is `in-progress` (0366 itself excepted once claimed), every open PR has
a recorded decision (merged before the cut, or held until after Phase 10), and no `/loop` or
autonomous session is running against the Docket repository.

### Phase 1 — Establish the release source and open the window

1. `git fetch origin`; candidate SHA = `git rev-parse origin/main`; record
   `git log -1 --format='%H %ct %s' <sha>` (the `%ct` is the source epoch the packager stamps).
2. Quiescence proof, recorded verbatim: `docket status --repo-dir <docket clone> --json`
   (no `in-progress` entries other than 0366; `implemented` entries listed with their hold
   decision; `error_findings: 0`), `gh pr list --state open` output, and the operator's
   attestation that no autonomous loop is running.
3. Claim this change so the board shows the release in progress and a branch exists for
   evidence checkpoints: read `path` + `version` from `docket status --records --json`, then
   `docket change claim --id 366 --version <v>` and `docket workspace prepare --id 366 --version
   <v>`. Refresh the lease at every phase boundary with `docket change refresh-claim`. Commit each
   phase's evidence to the feature branch as it lands (pushed to `origin`, no PR yet) — the branch
   is the durable checkpoint that survives a crashed session.
4. **Freeze rule.** Every later phase re-probes `git ls-remote origin refs/heads/main` and
   requires it to equal the candidate SHA. A mismatch is a STOP: the human either re-cuts from the
   new tip (restart at Phase 3 with a fresh workflow run; the old candidate is discarded whole) or
   records each intervening commit in `decisions.md` as excluded from beta1 and continues. The tag
   always targets the recorded candidate SHA; a later commit is never silently substituted.

### Phase 2 — Close the migration ledger (metadata branch only)

Read every active record — `docket status --records --json` for paths and versions, then the
files — and apply the program map's five disposition rules item by item. Never bulk-edit by
filename or keyword. Mutations go through `docket change kill --request` (the request names the
successor Go owner where one exists) and `docket change defer --request`; both request shapes are
resolved from `docket schema`. A retained item whose `related:` should name its Go owner is
recorded in `ledger.md`; editing frontmatter links on a build-ready record has no typed operation
and is an optional human-typed edit plus `docket repository migrate`.

The audit performed on 2026-09-03 against the Go tree is the **agenda**, not the verdict. Every
"kill" below is a proposal for the human; rule-5 items stay `proposed` until the human answers.

| id | rule | proposed disposition | Go owner / evidence |
|---|---|---|---|
| 0007 recurring templates | 3 | keep deferred | its spawn primitive (`mint-stub.sh`, auto-capture) is gone; auto-capture is deferred from Go v1 |
| 0008 parallel drain | 5 | **human**: is N-way fan-out still a goal now that `/loop` drains serially over the Go claim CAS? (its revive condition 0110 was killed) | no Go fan-out exists |
| 0009 human escalation loop | 4 | keep deferred (independent product work) | `## Run halted` (`internal/app/change_halt.go`) is now the third presence-encoded marker |
| 0010 board analytics | 5 | **human**: its own `## Why deferred` says "kill on next review" | month digest in `internal/render/board.go` |
| 0154 stale-restatement audit | 2 (mixed) | link + re-groom as a Go guard | `skills/docket-status/SKILL.md` still cites deleted `board-refresh.sh`/`render-board.sh`/`github-mirror.sh`; guard home `internal/repoguard/prose_contracts_test.go` |
| 0166 advisory session-model | 2 | link + re-groom | drift is live (`claude-sonnet-5` in the interactive skills vs opus in `harness-defaults.yml`); the Bash pin test is gone |
| 0195 opencode defaults retune | 4 | retain, re-groom | pure data retune over 17 rows; Go pin `internal/harness/inventory_test.go` |
| 0248 role self-description guard | 2 | link + re-groom | violation live in `skills/docket-review/SKILL.md`; guard home `internal/repoguard/prose_contracts_test.go` |
| 0257 residual review findings | 2 | narrow to the prose legs (E2/E6/E7/E8); the test-file legs are gone | — |
| 0261 `## Run halted` board surface | 2 | retain as a Go feat, link | `internal/render/board.go` renders finalize-blocked and auto-groom-blocked cells, no Run-halted cell; no age-based health family exists |
| 0263 AGENTS.md shell-rule guards | 2 | narrow to the two missing legs (leading `--`, awk `[^ ]`) | `internal/repoguard/shellshape_test.go`; `depends_on: [172]` is a killed change |
| 0273 host-relative budgets | 2 | link + re-groom | mechanics gone; regime survives in `internal/suiterunner/budgets.go`; `tests/runtime-budgets.tsv` has 39 Go-wrapper rows |
| 0283 slim AGENTS.md | 4 | retain, re-groom | AGENTS.md is 131 lines; new blocks postdate the spec |
| 0291 gate-failure.md ordering | 2 | link + re-groom | ordering gap holds in `docket-finalize-change/SKILL.md` |
| 0292 mutation-probe harness | 5 | **human**: has the defect class recurred on any Go-native plan since 0318? | proposed home `scripts/mutation-probe.sh` is gone; no Go successor |
| 0301 lifecycle cardinalities guard | 2 (mixed) | link + re-groom | "eight states" prose vs 8 `Status` consts in `internal/domain/types.go`; `github-board-mirror.md` describes the dropped `github` surface |
| 0302 mint-stub dedup | 3 | keep deferred | already re-dispositioned 2026-09-02 |
| 0320 testdata gitignore guard | 4 | retain | Bash-independent, unenforced |
| 0323 uninstall / version-tree GC | 4 | retain | `internal/install/roots.go` `VersionsDir()`; no `uninstall` verb |
| 0327 stack close-out reachability | 4 | retain, re-groom | `internal/app/finalize_closeout.go` proves root reachability; descendants still via merged-PR destinations |
| 0345 slash-command attribution | 5 | **human**: is there any pre-fork interposition for slash-launched agents; if not, is "verify-and-report-only" a docs change? | — |
| 0346 finalize rebuild from unpulled tree | 4 | retain, re-groom | rule lives only in AGENTS.md; no ancestry assert in `internal/app/install.go` |
| 0349 configurable resolver cap | 4 | retain, re-groom | cap is a skill literal; `internal/config/config.go` `Finalize{}` has no field |
| 0350 swallowed validation failure | 4 | retain | `internal/app/planning.go` |
| 0354 duplicate Run-halted heading | 4 | retain | `internal/app/change_halt.go` / `internal/render/section.go` |
| 0360 coordination tax | 4 (partly 3) | retain, split at groom | schema legs done by 0399; the results-only-delta leg leans on a deferred capability |
| 0368, 0375, 0376, 0379, 0380, 0382, 0383, 0387, 0388, 0398 | 4 | retain (post-Go, Go-native) | — |
| 0392, 0401, 0402, 0403 | 4 | decided in Phase 0 | — |

Migration learnings are recorded manually with `docket learning record --request` (one lesson
per record; the request shape from `docket schema`). Candidate lessons the operator authors,
extending an existing finding where one fits rather than duplicating: a release stub written
before its dependencies merge is stale within days, so a release change re-anchors on the current
tip at groom time; the frozen rollback artifact and the latest published tag can diverge, and the
notes must say which one they mean; a `workflow_dispatch` run has no base bundle, so the
Bash-to-Go upgrade path needs its own probe; fresh-process harness proof is adjudicated from the
harness's own thread store, never a parent's item stream (0384).

Evidence: `ledger.md` — one row per active item (id, rule, disposition, operation result
`committed_revision`, successor link), the human's answers to the rule-5 questions, and the
learning record ids. Gate: every active item has a row; no rule-5 item was decided by anyone but
the human; the board re-rendered inside each transaction (no separate board pass exists).

### Phase 3 — Package once

Preconditions: Phase 1 recorded; `origin/main` equals the candidate SHA.

1. `gh workflow run release-candidate.yml --repo danielhanold/docket -f ref=<candidate SHA>
   -f version=v1.0.0-beta1`; capture the run id (`gh run list --workflow release-candidate.yml
   --limit 1 --json databaseId,headSha,status`), require `headSha` == candidate; `gh run watch`.
2. Require conclusion `success` with all four jobs green. Download `candidate-head` and
   `candidate-evidence` with `gh run download <id> -n <name> -D <dir>` into a fresh directory.
3. Verify `evidence.json`: `source_commit` == candidate SHA; `head_version` == `v1.0.0-beta1`;
   `smoke_result` == `success`; exactly four `tuples` entries each with `verdict: success`;
   `checksums_txt` byte-equal to the bundle's `checksums.txt`. Recompute the SHA-256 of every file
   in the bundle and require a matching manifest line (and no unmatched line).
4. Make the copy immutable (`chmod -R a-w`), record its absolute path and a `sha256` listing in
   `candidate/run.txt` together with the run URL, the source-gate log's `go version`, the runner
   images, and the suite summary lines (`SUITE …`, any `BUDGET WATCH:`/`PARALLEL-SENSITIVE:` lines
   as screening findings, any `OVER BUDGET:` line as an authoritative breach — the latter fails the
   job by design).
5. Every later phase reads **this copy** (decide and act on the same bytes). A second workflow
   run, for any reason, produces a new candidate: the old copy is discarded whole, and Phases 4–9
   restart. Bundles are never mixed.

A `workflow_dispatch` run has no base bundle, so the workflow's upgrade block does not run; the
Go-to-Go upgrade was proven on pull-request runs, and the release-relevant upgrade — a Bash install
adopted by the Go installer — is Phase 6.

STOP on: any job not `success`; any evidence field mismatch; any checksum mismatch.

### Phase 4 — Native tuple evidence

The four `smoke` legs of the Phase 3 run are the tuple evidence: native runners, the packaged
bytes, blocks A–F and H of `scripts/release-smoke.sh`. Record per tuple in `tuples.md`: runner
label, `os/arch`, verdict, the `SMOKE PASS <os>/<arch> v1.0.0-beta1` stdout line, and the block-C
identity (`version --json` and `diagnostic runtime --json` from the log).

The operator also runs `bash scripts/release-smoke.sh --bundle <immutable copy> --version
v1.0.0-beta1` from a checkout at the candidate SHA on the operator machine (darwin/arm64), as an
independent execution of the downloaded copy; its row joins the table. Per-target rebuilding is
never evidence.

### Phase 5 — Fresh-host self-host scenarios (four harnesses)

**Host.** A **fresh macOS user account** on the operator machine (darwin/arm64). It gives every
harness — including the Cursor IDE — a genuinely fresh `HOME`, and it cannot see the operator's
development install. If a fresh account is impossible, the CLI harnesses (Claude, Codex, OpenCode)
may run under an isolated `HOME`/`XDG_*`/`CODEX_HOME` root and Cursor under its real profile with
the Docket assets freshly installed from the candidate; the row records which host mode was used.
A Linux lifecycle run is not required: the harness boundary is not an OS boundary, and Linux bytes
are covered by the tuple smokes.

**Install.** In the fresh account: `sh <immutable copy>/install.sh --harness claude --harness
codex --harness cursor --harness opencode --bin-dir <dir on PATH>`; then `docket version --json`
(version `v1.0.0-beta1`, commit == candidate), `docket install check --json` (clean, `mode:
release`, four harnesses, the candidate asset set), `docket diagnostic runtime --json`
(`supported_target: true`). Log in to each harness and to `gh` inside the fresh account; copy no
other operator state.

**Remote.** One disposable **private** GitHub repository per harness:
`gh repo create danielhanold/docket-accept-v1-0-0-beta1-<harness> --private`, cloned inside the
fresh account. After the row is recorded, archive or delete the repository; the row keeps the PR
URL and the final `docket status --json`.

**Fixture** (identical for all four): `README.md`, an executable `test.sh` (`#!/bin/sh` / `exit
0`), pushed to the disposable remote; `docket repository init` in the clone, then confirm the
generated policy is `build.test_command: sh ./test.sh`, `finalize.test_command: sh ./test.sh`,
both gates `local` (use `docket repository configure-tests` if init left the policy pending);
commit and push; `docket repository check --json` clean.

**Lifecycle** (the human types every invocation through that harness's native surface — a named
`@docket-…` agent or the skill by name — never through another harness or a runner shim):

1. Create one stub with the cataloged `docket change create --request` (type `feat`, "Add a
   greeting line to README"); the stub is needs-brainstorm.
2. `docket-groom-next <id>` in the harness; exit with a short spec (the spec path lands via the
   groom transaction).
3. `docket-implement-next <id>` with the explicit id, bracketed by the run-gate facade exactly as
   the repository's `AGENTS.md` block prescribes (`docket run gate-before implement-next`, then
   `docket run gate-verdict <key>`).
4. **Restart/resume.** From a second shell, poll `docket status --records --json` until the record
   shows `in-progress` with `plan:` set. Terminate the harness process (SIGTERM to its PID). Start
   a fresh session of the same harness; run `docket run gate-before implement-next --resume <id>`
   and dispatch `docket-implement-next <id>` again. The run must resume that id — never claim a
   different change — reconcile the interrupted gate drive through the driver's own continuation or
   takeover path, and reach `implemented` with an open PR.
5. `docket-finalize-change <id>`: rebase gate (`sh ./test.sh`), merge, closeout, cleanup.
6. `docket-status`: sweep plus health; then the terminal predicate.

**Terminal predicate** (same for every row): the change record is under `archive/` with `status:
done`, a full-URL `pr:`, `plan:` and `results:` set; `gh pr view <pr> --json state,mergeCommit`
shows `MERGED`; the fixture's default branch carries the greeting line; `docket status --json`
reports `ready: []` and `error_findings: 0`; `docket repository check --json` is clean; the
metadata branch on the remote holds the archived record and the re-rendered board.

**Row fields** (`harness/<name>.md`): harness name, exact vendor version, mode (CLI
interactive / IDE), host mode (fresh account or isolated root), candidate commit and archive
SHA-256, proof a fresh process started (process start time versus install time), proof the named
children ran (the harness's own record of `docket-plan-writer`, a `docket-build-*` profile, and a
`docket-review-*` rung — for Codex, adjudicated from the thread store or app-server activity,
never the `codex exec` item stream), kill and resume timestamps and the resume verdict line, the
terminal predicate results, and the sanitized transcript location.

Gate: all four rows pass the same predicate. A failing row is a STOP. A Docket defect returns to a
source change and a new candidate. A vendor defect is diagnosed against the recorded version and
mode; the human may let beta1 ship with that harness listed under known gaps in the notes and the
row recording the exact failure — a latitude beta semantics allow and stable `v1.0.0` does not.

### Phase 6 — Legacy-install upgrade probes

Two probes in isolated roots (fresh account or isolated `HOME`), each recorded in
`upgrade-probes.md` with the exact installer output:

- **From `v0.9.2`.** Check out tag `v0.9.2`, run its `install.sh`, then run the beta1 downloader
  with the same harnesses. Expected: ownership-safe adoption of the legacy user-level artifacts
  (ADR-0096), `docket install check --json` clean.
- **From `v0.9.3`.** Same, from tag `v0.9.3` (the 17-agent inventory). ADR-0096 reproduces only
  the `v0.9.2` closed inventory, so an `ownership-conflict` on the `docket-plan-writer` wrapper is
  the plausible outcome. Record what actually happens. A conflict is **not** a beta1 source fix:
  the notes' "Upgrading from v0.9.3" section carries the installer's own remedy text, and a
  follow-up change (widen the frozen corpus to `v0.9.3`, ADR-0096's named mechanism) is reported
  for deliberate capture.

### Phase 7 — Rollback rehearsal (`v0.9.2`)

In a separate isolated root: check out tag `v0.9.2` (record its commit and the SHA-256 of its
`install.sh`), run its `install.sh`, bootstrap a fresh fixture with its own `docket.sh` flow, run
its status/board pass and one trivial metadata write; record outcomes in `rollback.md`.

Cross-compatibility probe, **read-only**: point `v0.9.2`'s status pass at a clone of a Phase 5
fixture that beta1 wrote (Go-written records, the six-group board, the `build:` config block) and
record whether it reads cleanly. `v0.9.2` never writes into it. The observed result becomes the
notes' rollback statement: machine-level rollback is independent of the candidate; repository-level
compatibility is exactly what was observed.

No-fallback proof: the candidate's suite (green in the source gate) carries 0370's shape-derived
absence guards over the retired Bash control plane; `tar -tzf` of each archive lists exactly
`docket`; the downloader's dependency set is `sh`, `curl`, `tar`, and one SHA-256 provider. Record
all three. `v0.9.3` is noted as the last Bash-era published tag and an equally valid machine-level
rollback for machines that ran it; `v0.9.2` remains the documented artifact.

### Phase 8 — Publish (the human irreversible boundary)

Preconditions: Phases 1–7 recorded and passing; `origin/main` re-probed equal to the candidate;
the notes (`release-notes.md`, outline below) authored and reviewed by the human; the human's
explicit "publish" recorded in `decisions.md`.

Each step is probe → act only if absent → verify → record, in `publication.md`:

1. **Tag.** `git ls-remote --tags origin refs/tags/v1.0.0-beta1 'refs/tags/v1.0.0-beta1^{}'`.
   Absent: `git tag -a v1.0.0-beta1 <candidate SHA> -m "docket v1.0.0-beta1"`, then
   `git push origin refs/tags/v1.0.0-beta1`; verify the peeled target equals the candidate.
   Present at the candidate SHA: continue (a prior partial attempt). Present elsewhere: STOP —
   never move or delete a tag; a re-cut ships as `v1.0.0-beta2`.
2. **Release.** `gh release view v1.0.0-beta1 --json isDraft,isPrerelease,tagName,assets`.
   Absent: `gh release create v1.0.0-beta1 --verify-tag --draft --prerelease --title "<title>"
   --notes-file release-notes.md`. Draft present: continue. Published present: skip to step 4.
3. **Assets.** For each of the six files in the immutable copy: absent on the release →
   `gh release upload v1.0.0-beta1 <file>` (never `--clobber`); present → download it with
   `gh release download v1.0.0-beta1 -p <name> -D <verify dir>` and compare its SHA-256 with the
   immutable copy; a mismatch is a STOP (the human may delete the offending draft asset by hand and
   re-run this step; the protocol deletes nothing).
4. **Verify.** `gh release view --json assets`: exactly six assets, expected names, sizes equal to
   the immutable copy, digests equal (GitHub's asset `digest` where exposed, else downloaded bytes).
5. **Publish.** `gh release edit v1.0.0-beta1 --draft=false --prerelease --latest=false`; verify
   `isDraft: false`, `isPrerelease: true`; re-probe the tag.

Retry rules after a partial publication:

| Observed state | Action |
|---|---|
| tag at candidate, no release | continue at step 2 |
| tag at candidate, draft with a subset of assets | upload only the missing assets; verify all |
| tag at candidate, draft with a mismatching asset | STOP; human deletes it explicitly; rerun step 3 |
| tag at candidate, published release with missing assets | STOP; human confirms; upload the missing assets (absence is not a conflict), verify, record |
| tag at candidate, published with all assets matching | continue at Phase 9 |
| tag elsewhere, or release targeting another commit | STOP; never move, delete, or overwrite; the next attempt is `v1.0.0-beta2` from a new candidate |

Never compensate a published effect automatically; probe each remote object and continue only
when its identity matches the recorded candidate.

### Phase 9 — Public installation verification

In a fresh account or isolated root with no Docket present, over the real network path (no
`DOCKET_RELEASE_BASE_URL`): download
`https://github.com/danielhanold/docket/releases/download/v1.0.0-beta1/install.sh` and
`checksums.txt`; verify the script's SHA-256 against its manifest line **before** executing it
(never pipe a downloaded script into `sh`); run `sh install.sh --harness claude` (any subset of
the four); verify `docket version --json` (version, commit, build date), `docket install check
--json` clean, `docket diagnostic runtime --json`; run the read-only CLI baseline from
`docs/release/four-harness-acceptance.md` against a fixture and record the protocol fields.
Record every URL, digest, and output in `public-install.md`. A public URL test verifies exposure of
the accepted bytes; it never authorizes a rebuild.

### Phase 10 — Close out through the installed Go product

1. Assemble the evidence bundle (layout below) and the results record
   `docs/results/<date>-<slug>-results.md` on the claimed feature branch; the results record's
   human-verify section is the gate table with one line per phase.
2. In the feature worktree: `docket gate drive start --owner build … -- go run ./cmd/docket
   development test` to `PASSED`; `docket evidence record`; `docket change attach-results`;
   `docket pr publish` (the PR body carries the gate table); `docket change mark-implemented`.
3. `docket-finalize-change 366`: rebase gate, merge, closeout with `verification_outcomes`
   naming each gate, cleanup. The window closes at this merge.
4. `docket-status`: sweep and health, expected clean. Held PRs from Phase 0 may now merge.

`docket-implement-next` is deliberately not used for 0366: there is no code to plan or build, and
the evidence already exists when the PR is opened.

## Evidence bundle

```text
docs/release/v1.0.0-beta1/
  README.md               index: candidate identity, gate table (phase → verdict → file)
  decisions.md            Phase 0 decisions; every STOP, its reason, and the resumption probe
  candidate/evidence.json verbatim workflow artifact
  candidate/checksums.txt verbatim
  candidate/run.txt       run id/URL, toolchain, runner images, suite summary, immutable-copy listing
  tuples.md               four workflow rows plus the operator-machine row
  ledger.md               per-item disposition table, rule-5 answers, learning record ids
  harness/<name>.md       one row per harness (fields above)
  harness/<name>-transcript.txt   sanitized; or a pointer to a durable private location if > 1 MB
  upgrade-probes.md       v0.9.2 and v0.9.3 adoption outcomes with installer output
  rollback.md             v0.9.2 rehearsal, read-only cross-compatibility probe, no-fallback proof
  publication.md          tag object and peeled SHA, release id/URL, asset ids and digests, timestamps
  public-install.md       URLs, digests, command outputs
  release-notes.md        the notes as published
```

Sanitization: no tokens or credentials; `$HOME` and account names replaced by `$FRESH_HOME`; only
Docket-related transcript content; disposable repository URLs are fine (they name nothing
private). A transcript that cannot be sanitized to that standard is summarized instead, with the
summary saying so.

Frozen-record rule: once the closeout PR merges, `results:` and the bundle are frozen build
records; corrections go in a new change.

## Failure and retry boundary

- STOP before publication on any failed source, candidate, tuple, harness, lifecycle, upgrade,
  rollback, ledger, or evidence gate. Missing or ambiguous evidence fails the gate.
- Resume only from authoritative probes (`git ls-remote`, `gh run view`, `gh release view`) and
  the recorded checksums. Local files, elapsed time, or a previously typed command do not establish
  success. A crashed operator session resumes from the feature branch's last pushed evidence commit
  and the probes.
- Never repair source inline. Any source change gets its own reviewed change, invalidates the
  candidate, and restarts at Phase 3 from the new merged commit.
- Never automatically compensate a published external effect (Phase 8 table).
- A vendor harness failure never widens this change into a compatibility wrapper, runner fallback,
  or harness redesign.

## Release notes outline

Title: `v1.0.0-beta1 — Docket is a Go binary`. Sections, in order:

1. **What this release is.** The first public build of the Go product; a hard replacement of the
   Bash implementation with no Bash fallback; a beta seeking feedback from existing users. Stable
   `v1.0.0` follows after the known gaps close.
2. **Install.** The downloader command with `--harness` flags, the four supported targets
   (darwin/linux × amd64/arm64), the four harnesses (Claude Code, Codex, Cursor, OpenCode), the
   `checksums.txt` verification step, and the runtime dependency set (`sh`, `curl`, `tar`, one
   SHA-256 provider; no Bash, Python, or Perl).
3. **Upgrading from v0.9.x.** What the Go installer adopts from a Bash install (ADR-0096) and the
   observed `v0.9.3` outcome from Phase 6 with its remedy; `metadata_branch: main` is removed —
   run `docket repository migrate`; the deferred capabilities that now fail closed before a
   mutation (auto-capture, terminal publishing, results-only gate skipping, build checkpoints,
   CI finalize gates, autonomous grooming, dummy mode, runner delegation, per-repo model routing,
   skill rebinding); the GitHub Issues/Projects mirror is dropped; restart every harness process
   after installing.
4. **What changed since v0.9.3** — themes, each naming its changes: the Go foundation (0304–0317,
   one paragraph); Bash control plane deleted (0322, 0326, 0338, 0339, 0342, 0369–0372, 0377);
   the CLI surface (`repository init|migrate|check|prepare|configure-tests`, `run gate-*`, `gate
   drive *`, `maintenance sweep --scope`, `maintenance preflight`, `capabilities`, `schema`,
   `status --json --records`, `finalize closeout` notes, `--repo-dir` defaulting to cwd, the
   `failure` envelope field); configuration (`build.gate`/`build.test_command` and the removal of
   the `auto` sentinel, `board.section_order`/`board.sorting`, `branch_prefix:`, `metadata_branch`
   tombstone); harness dispatch (native-only dispatch, compact dispatch block with recursion guard,
   catalog-resolved skills, model-pinned plan-writer, Cursor unquoted names, Codex direct dispatch
   and coordinator entry with its 0393 status); build gate (`docket development test`, gate driver
   continuation and takeover, integration-tag partitions, runner concurrency cap); finalize (policy-
   aware merge method, exact-PR identity, WAITING re-entry, closeout notes, scoped sweeps); metadata
   and board (GitHub blob links, six-group board, one topology); docs (goal-first README).
5. **Retained POSIX products.** The contributor `install.sh` bootstrap, the release downloader,
   `scripts/release-smoke.sh`, and `scripts/runners/*` — the complete list, and the disposition of
   `link-skills.sh`.
6. **Rollback.** `v0.9.2` is the frozen Bash baseline and documented rollback artifact; the Phase 7
   rehearsal result and the observed repository-level compatibility; `v0.9.3` as the last Bash-era
   tag.
7. **Known gaps.** Homebrew, Windows, signing/notarization, SBOM, uninstall and version-tree
   collection (0323), and whatever Phases 5–6 recorded (a harness listed as unproven, the
   `v0.9.3` adoption remedy, stale prose named in Phase 0). Every gap names its tracking change.
8. **Evidence.** A link to `docs/release/v1.0.0-beta1/` on `main` and the ADRs the release rests on
   (0095, 0096, 0099, 0100, 0102, 0103, 0104).

## Alternatives considered

- **`v1.0.0-rc1` as written in the stub.** Rejected by the human at groom time: the first public
  Go build is a feedback candidate over Bash-era installs, not a feature-frozen release candidate.
- **Cut from the exact 0370 merge commit.** Rejected: fourteen changes merged after it, including
  the `build:` config block Docket's own repository now requires; the tip of `main` is the only
  commit the source gate and the self-host proof can both stand on.
- **A tag-triggered publishing workflow.** Rejected for beta1: it is a source change carrying a
  write token, and the protocol's value is a human at the irreversible boundary. Reported as a
  follow-up for a later release.
- **Reuse 0317's read-only status scenario as the four-harness gate.** Rejected: the program's
  success criterion is the complete retained lifecycle from all four hosts, and 0317's live rows
  were never recorded (it has no results file).
- **Isolated `HOME` only, no fresh account.** Kept as the fallback; a fresh macOS account is the
  only way to give the Cursor IDE a genuinely fresh profile and to exclude the operator's
  development install by construction.
- **One shared disposable remote for all harnesses.** Rejected in favor of one per harness:
  isolation, parallel runs, and clean deletion.
- **Tag a commit behind a moved `main` silently.** Rejected: the freeze rule makes a moved `main`
  a STOP with a recorded human choice.
- **Run the lifecycle on Linux as well.** Not required: the harness boundary is not an OS boundary;
  the tuple smokes execute the Linux bytes natively.

## Explicit exclusions

Source changes of any kind (a defect returns to a reviewed change); changing accepted bytes after
packaging; rebuilding or substituting bytes mid-protocol; a Bash fallback or compatibility launcher
in the candidate; stable `v1.0.0` promotion; Homebrew, Windows, signing/notarization, SBOM or
provenance signing; uninstall or version-tree collection (0323); public-install documentation in
the README or `docs/guide/` (a separate docs change — the notes are self-contained until it lands);
a publishing workflow; widening ADR-0096's frozen corpus to `v0.9.3`; any redesign of storage, the
JSON protocol, harness topology, or the Git/GitHub adapters.

## Acceptance boundary

This change is **designed** when this spec is linked from the record. It is **implemented** when
the closeout PR is open with every phase's evidence in the bundle and every gate green (or a
recorded human downgrade in the notes' known gaps), the tag and Release exist at the candidate
with six verified assets, and a public installation from the release URL verified clean. It is
**done** when `docket-finalize-change` archives it and the sweep is clean.

## Follow-ups to report for deliberate capture (never minted by this change)

- Public-install instructions in the README and `docs/guide/` (0402 may absorb them).
- A tag-triggered publishing workflow with a scoped write token, for a later release.
- Widening ADR-0096's frozen corpus to `v0.9.3`, if Phase 6 records a conflict.
- Disposition of the undocumented repo-root `link-skills.sh`.
- Retiring the `metadata_branch: main` opt-out text from `.docket.example.yml` and
  `docs/guide/README.md` (0363 left it behind).
- Retitling this record from rc1 to beta1 (human-typed frontmatter edit plus
  `docket repository migrate`).
