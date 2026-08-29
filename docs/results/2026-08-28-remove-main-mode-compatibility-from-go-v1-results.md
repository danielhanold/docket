<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0363 — Remove main-mode compatibility from Go v1](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-29-0363-remove-main-mode-compatibility-from-go-v1.md)**
<!-- docket:backlink:end -->
# Remove main-mode compatibility from Go v1 — results

Change: #363 · Branch: refactor/remove-main-mode-compatibility-from-go-v1 · Plan: docs/superpowers/plans/2026-08-28-remove-main-mode-compatibility-from-go-v1.md · ADRs: 1, 2, 52, 69, 99

## Verify (human)

- [ ] After merge, per the repo's standing rule, rebuild the installed binary so the tool matches source:
      `docket development install --source /Users/homer/dev/docket`. (The `metadata_branch` effective row
      and the mode-shaped protocol fields are gone in source; a stale installed binary still shows them.)

## Findings

- **ADR-0099 recorded** — "One metadata topology for Go v1 (main-mode removed)". Supersedes ADR-0002
  (now `Superseded by ADR-99`); relates to ADR-0001 (which received a dated `## Update`) and ADR-0052.
- **Operational predicate pinned to `legacy` only.** The shared operational-repository gate
  (`internal/app/operational_context.go`) refuses exactly `reposetup.StateLegacy`; every non-legacy state
  `PinContext` accepted before (fresh/partial/needs-review/healthy) stays accepted, preserving the existing
  single-pin contract. `repository check`/`init`/`migrate` keep their own classifier sub-gate below the
  operational gate. This resolved a spec under-specification ("operational/healthy only") that, read
  literally, would have over-refused normal repos and forced a per-fixture `init` (Task 11 budget risk).
- **Review (docket-review-deep): 7 findings, 0 blocker / 0 important / 7 minor.** Five minor findings
  (stale "main mode" prose in `finalize_closeout.go` ×2, `status.go`, `render/link.go`, and a dead test
  var) were fixed in-branch. Two were acknowledged as no-fix-needed and are recorded here:
  - *Structural guard reach* — `mode_shape_guard_test.go` keys on the removed json-tag keys
    (`metadata_mode`, `repo_mode`) and the removed constants (`metadataModeMain`/`metadataModeDocket`),
    mutation-tested per arm plus a visited-count floor. It does not (and by design cannot cheaply) catch a
    reintroduced mode selector under a brand-new name, e.g. a `.Mode == "main"` field access — guarding
    bare `.Mode` would false-positive against `os.FileMode`/`install.Mode`. Accepted limitation.
  - *Config source divergence* — the operational gate reads config from the pinned default-branch
    `.docket.yml` blob, while `repository check` reads the working tree; a pathological uncommitted
    `.docket.yml` edit could classify differently between the two. By design (the gate's pinned-
    reproducibility contract); no defect.
- **Layer-stranding check (Task 9).** No `metadata_branch` in this machine's `~/.config/docket/config.yml`
  or `.docket.local.yml`. A stale outer-layer `metadata_branch` is degraded to a warning (coordination-key
  fence / obsolete-setting tombstone), never a hard failure, so no outer-layer config is stranded by this
  change.

## Follow-ups

- **Notable plan deviations** (not new work, recorded for audit):
  - *Task 3* (shared operational gate) escalated `premium → max`: introducing the gate into the shared
    `PinContext` caused suite-wide behavioral refusal across the real-git integration fixtures (all
    classified legacy/fresh/partial). The `max` worker was authorized to fold in **Task 6's real-git
    fixture contraction** (`newMainModeRepo` → `newLegacyRepo`/`newWorkingRepo`; `planRepoModes` collapsed
    to its docket row) atomically with the gate, so Task 6's later scope shrank to fake-pin constructors,
    the CLI/gate-facade fixtures, and prose.
  - *Task 8 documentation scope* — the shell/bash config layer (`docket-config.sh`) still supports
    `metadata_branch` (removing Bash production is change 0318's scope, spec exclusion). Per that
    constraint, `.docket.example.yml` and the README's still-live bash main-mode narrative were left as
    accurate descriptions of currently-supported behavior; only docket's own `.docket.yml` active setting
    was removed and the `v0.9.5` self-fixture cut. Frozen `v0.9.2`–`v0.9.4` trees are byte-untouched.
- **Downstream:** change 0318 (Go-only cutover) remains the later step and was `waiting-on-363-unbuilt`;
  it does not absorb this contraction.
