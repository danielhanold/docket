<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0154 — Remove stale Bash instructions and duplicated runtime contracts from Docket skills](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0154-audit-skill-bodies-for-the-stale-restatement-class-change-01.md)**
<!-- docket:backlink:end -->
# Remove stale Bash instructions and duplicated runtime contracts from Docket skills — Results

## Outcome

One documentation PR audited every git-tracked Markdown file under `skills/` (29 files at baseline `d7363492`) plus their references and templates, removed stale runtime instructions and duplicated runtime contracts, and replaced them with usable pointers to current Go owners and the capability/schema/config discovery channels. The build gate (`go run ./cmd/docket development test`) is green at the feature head; the embedded `internal/assets` bundle was regenerated through its owner and corresponds byte-for-byte to the authored tree.

Seed dispositions (all verified against the running binary and named Go owners, not sibling prose):
- **Status report contract** — `skills/docket-status/SKILL.md` rewritten to request and interpret the structured `status`/`maintenance.sweep` payloads (`internal/app/status_result.go`, `status_human.go`, schema op) instead of the retired `backlog`/`change`/`ready` line grammar, `pass ok`, `harvest`, and mirror-mint reports. The historical `health checks failed <exit>` line was deliberately not "repaired" — the report-model replacement covers it.
- **Sweep posture & deleted owners** — the Bash-era stage-by-stage retry narrative and the missing `scripts/docket-status.md` / `board-checks.md` / `board-refresh.md` / `board-refresh.sh` / `render-board.sh` / `github-mirror.sh` pointers were removed; the surviving obligations (read vs `maintenance.sweep`, terminal-envelope observation, per-problem-entry inspection, unknown-state preservation, findings-without-repair, sweep-recovery-distinct-from-finalize) are stated against the current operation. The sweep-log-and-continue vs finalize-abort-and-report caller variance was preserved.
- **Board / GitHub mirror** — the "disabled board must not exist" contradiction was replaced with the current property (disabling rendering never authorizes deleting a pre-existing `BOARD.md`; status is read-only); active mirror/write-back instructions were removed and `skills/docket-convention/github-board-mirror.md` was deleted; the `github` surface is described as classified unsupported/mutation-blocking, discoverable via `diagnostic.config`.
- **Configuration copies** — the copied coordination-key inventory and retired sketch/`board_surfaces` behavior in `skills/docket-convention/SKILL.md` were removed or corrected against `internal/config/schema.go`, `capability.go`, `.docket.example.yml`, and the resolved `diagnostic.config`. The one absorbed change-0257 item (the stale "superpowers default shown" `skills:` sketch comment) was corrected to name docket's own build/review defaults (change 0193).
- **Legacy explanations & counts** — the "Derived-view script family" paragraph now names the Go owners (metadata-transaction board render, `artifact.backlink`); main-mode operating clauses and the `## main-mode degradation` section were removed (0363/ADR-0099); redundant wrapper/state/surface cardinalities were dropped rather than restated.

## Verification performed

- Full configured suite green through the native gate driver at head `8d669d48` (`go run ./cmd/docket development test`); build-evidence recorded and verified.
- Guard reconciliation was mutation-tested where guards changed: `TestSkillSizeBudgets`, `TestCapabilitySurface`, `TestProseContracts` (change_0389 sentinel), the capability-surface migrate pin, and the config-read-channel reader-liveness floor were each reddened by a targeted mutation and restored green. A new `TestNoActiveMirrorWriteBack` guard was written RED-first (against the still-present mirror prose), then GREEN after removal, binding subject (`mirrorMintSubjectRe`) AND obligation (`mirrorWriteBackRe`) on one line with a non-vacuity companion.
- Embedded-bundle correspondence verified both ways after `go generate ./internal/assets`: authored `skills/**/*.md` ↔ embedded tree byte-identical; `github-board-mirror.md` absent from both trees and the manifest.
- Whole-population audit inventory (`docs/superpowers/plans/2026-09-08-audit-skill-bodies-inventory.md`) records a disposition for every one of the 29 files (hit-fixed / no-hit-verified-current), with a closing evidence appendix; the plan's zero-`pending` invariant holds.
- Whole-branch review (docket-review-standard) returned 2 minor findings, both about the same over-broad `internal/config/capability.go` citation; both were fixed in-branch (commit `8d669d48`) — the transaction-refusal owner re-attributed to `internal/app/planning.go` (`fenceBoardSurface`) and the shipped skill bodies reworded to lead with the observable `diagnostic.config` behavior for consuming-repo readers. 0 blockers.

## Findings and limitations

- The prose guards protect this file class by shape (mint-subject + write-back, budget, capability-surface pins), not by pinning every reworded phrase; a future divergence between skill prose and its Go owner is caught only where a guard's subject/obligation binding covers it. This is the deliberate design (no new generic prose-lint framework, per the spec) and is stated here rather than papered over.
- Human testing beyond the automated suite is not required: the change is documentation/contract prose plus guard reconciliation and generated-asset regeneration, all covered by the repoguard and assets correspondence guards and the full suite.

## Follow-ups

Out-of-scope work surfaced during the audit, recorded for deliberate human triage (nothing minted automatically):
- The shipped `.docket.example.yml` still carries live single-branch `metadata_branch` narrative describing the retired topology; it sits outside the `skills/` population and warrants a coordinated documentation sweep. No change id.
- The typed `learning.record` / `learning.update` operations are unsurfaced in `skills/` prose while the "edit `learnings/` files directly" framing omits them — a pre-existing documentation gap, not a stale-restatement defect. No change id.
- Deferred change 0257's remaining rationale-restoration and shell-guidance work stays deferred; only its single sketch-comment item was absorbed here. Change 0257.
