<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0305 — Configuration and capability envelope](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-13-0305-configuration-and-capability-envelope.md)**
<!-- docket:backlink:end -->

# Configuration and capability envelope — results
Change: #0305 · Branch: feat/configuration-and-capability-envelope · PR: (opened at step 7) · Plan: docs/superpowers/plans/2026-08-13-configuration-and-capability-envelope.md · ADRs: 15, 16, 19, 20, 52 (cited; none produced)

## Verify (human)

- [ ] **Vendor model IDs are outside-truth by construction (no certification run needed — a ratification).** The Go built-in agent registry ships all sixteen×four model/effort pairs as byte-parity copies of the already-shipped `agents/harness-defaults.yml` (frozen at commit `096c48de` into `testdata/repositories/v0.9.2/agents-harness-defaults.yml`; parity plus frozen-copy↔live-file drift asserts are in `internal/config/defaults_test.go`). This change introduces **no new vendor ID**, so no in-repo test — and no new certifying run — can or needs to prove a vendor accepts them; confirm you accept that the parity chain (Go table ↔ frozen sidecar ↔ live sidecar) is the intended oracle, per the learnings finding `external-truth-needs-a-human-checkpoint`.
- [ ] **Optional smoke on a real repository:** `go build -o /tmp/docket ./cmd/docket && /tmp/docket diagnostic config --repo-dir . --default-branch main --for-mutation` from the docket checkout should return `unsupported-config` with exactly four blockers (`auto_capture.enabled` — if your global config arms it, `build.checkpoint`, `finalize.skip_results_only_delta`, `terminal_publish`); with a clean global config, three. The suite already proves this against the frozen `docket-self` fixture; this item exists only if you want to see it live.

## Findings

- **Review (docket-review-deep, 8 findings: 0 blocker / 3 important / 5 minor).** All eight fixed in-branch; dispositions with commit SHAs are in the PR body's table. The one behavioral repair worth naming: a nonexistent `--repo-dir` previously resolved as "all layers absent" and certified a nonexistent repository as mutation-allowed (`f11cc46a`).
- **Frozen-fixture drift was undetectable as first built** — both frozen copies (`docket-self`, the agents sidecar) now carry byte-equality asserts against their live originals, so a live-file edit reddens the suite and forces a deliberate new versioned fixture tree (`e492a751`). The classic one-way correspondence gap.
- **`.gitignore` vs fixtures:** the repo-wide `.docket.local.yml` ignore silently swallowed repository-local fixture files (only tracked via a one-time `git add -f`). Durable fix is a nested `testdata/repositories/.gitignore` negation, deliberately outside the managed docket gitignore block so a block rewrite cannot hoist and neuter it (`551c5bd8`, `0aabf00a`). Residual: no automated guard reproves the negation's effectiveness if the ignore layout changes; the two `git check-ignore` probes were manual.
- **Suite budget advisory (pre-existing, untouched by this branch):** both full-suite gate runs printed `OVER BUDGET: test_sync_agents_runners` (197s vs 60s ceiling) under parallel load; that file's solo budget story belongs to changes 0251/0273, not here. `test_go_toolchain.sh` ran well inside its 20s row (4.6s solo) despite the two new packages.
- **Plan-contract friction recorded for the record:** the plan's `capabilities` JSON field prescribed both `omitempty` and an always-present `[]` — contradictory; resolved as omit-when-empty (never `null`), with the CLI tests asserting the seven-key sparse shape. The plan's illustrative `auto→main` human-output annotation is unrenderable from the resolved snapshot (the raw `auto` is not retained) and was dropped.

## Follow-ups

- None minted. Auto-capture ran at both sites (reconcile, review) with zero admissible discoveries; every review finding was in-branch work.
