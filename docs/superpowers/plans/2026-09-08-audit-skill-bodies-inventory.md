<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0154 — Remove stale Bash instructions and duplicated runtime contracts from Docket skills](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0154-audit-skill-bodies-for-the-stale-restatement-class-change-01.md)**
<!-- docket:backlink:end -->
# Change 0154 — Skill-body audit inventory

The reviewable per-file audit record for change 0154. Every task appends dispositions to the
rows below; Task 9 verifies no `pending` row remains. One row per tracked Markdown file under
`skills/`.

## Header — baseline and discovery

- **Baseline HEAD (feature checkout):** `f0673b5e233a24caa41038a9a5afbc8d8576712f`
  (feature branch `docs/audit-skill-bodies-for-the-stale-restatement-class-change-01`; branched
  from the change-0154 baseline `d7363492` named in the plan's Global Constraints).
- **Discovery command:** `git ls-files 'skills/**/*.md' 'skills/*.md' | sort`
- **File count:** 29 (matches the plan's expected shape of 29 at baseline).
- **Reference-op population** (for later owner-verification, Task 4/etc.): captured from
  `docket capabilities | awk '{print $1}' | sort -u` — 72 operations. Snapshot lives in the
  build scratch dir (`cap-ops.txt`); re-derive from the running catalog when verifying, never from
  this note.

### Mechanical seed scans are seeds, not a parse

The columns below attach the raw hits from the Task 1 mechanical seed scans (removed script-tree
refs, count words, old report grammar, retired surfaces) plus their whitespace-collapsed variants.
**These scans are sampling, not a full read.** Per the plan, **every file still gets a full manual
read in Task 7**, and a `no-hit (mechanical)` row is not yet a cleared row — its disposition stays
`pending` until a task (2–6 for the seeded files, 7 for the rest) records a verdict. Anchors are
quoted clauses / section names, never line numbers (ADR-0054); parenthetical line numbers are
baseline reading aids only.

### Seed legend

- **Seed 1** — status report grammar (`backlog <status>`, `change <id> …`, `ready […]`, `pass ok`,
  `harvest <id>`).
- **Seed 2** — sweep posture.
- **Seed 3** — board / GitHub-mirror claims (`board-refresh.sh`, `render-board.sh`, `github-mirror.sh`,
  `issue-minted`/`project-minted`, `write-back` marker, `github-board-mirror.md`).
- **Seed 4** — convention config copies (`.docket.yml` sketch, config-layer key list, `board_surfaces`).
- **Seed 5** — legacy explanations / counts (`nine`/`eight` etc., derived-view script family, main-mode).

### Mechanical scan commands (recorded for reproducibility)

```bash
# A: removed script-tree references (retired Bash owners)
/usr/bin/grep -rn -E '(docket-status|board-checks|board-refresh|render-board|github-mirror|render-change-links|render-artifact-backlink)\.(md|sh)' skills/
# B: copied count words / enumerations
/usr/bin/grep -rn -E '\b(nine|eight|seven|six|five)\b' skills/
# C: old report grammar tokens
/usr/bin/grep -rn -E 'pass ok|harvest <id>|backlog <status>|swept <id>|issue-minted|project-minted' skills/
# D: retired surfaces
/usr/bin/grep -rn -E 'main-mode|main mode|github-mirror|Projects v2|write-back' skills/
# Whitespace-collapsed variant: per file, tr -s '[:space:]' ' ' then /usr/bin/grep -qF each phrase
# (surfaced the same file set as the line-based scans — no wrapped-prose-only hits were hidden)
```

## Inventory

| path | disposition | hits (section / quoted clause) | owner verified against | notes |
| --- | --- | --- | --- | --- |
| `skills/docket-status/SKILL.md` | seeds 1–2 hit: fixed (Task 2); seed 3 hit: fixed (Task 3) | **Seed 1/2/3.** Scan A: "The per-line shapes and failure postures stay documented in `scripts/docket-status.md`" (Overview ~l12); "see `scripts/docket-status.md` for the output-line shapes and failure postures" (~l63); "`board-refresh.sh` is its only writer" (~l87); "the orchestrator (contract: `scripts/docket-status.md`)" (~l107); "`board-refresh.sh` (contract: `scripts/board-refresh.md`) … the pure renderer `/render-board.sh` (contract: `scripts/render-board.md`)" (~l113); "`github-mirror.sh`, mechanics in `skills/docket-convention/github-board-mirror.md`" (~l115); "`scripts/board-checks.md`, and the `check <check-id>` report-line row in `scripts/docket-status.md`" (~l131). Scan C: "`backlog <status> <count>` and `change <id> <status> <readiness> <slug>` lines, plus a trailing `ready [<id> …]`" (~l12, ~l80); "**`pass ok`**" (~l82); "**`harvest <id> <path>` lines**" (~l93); "`issue-minted`/`project-minted` lines to record back" (~l115). Scan D: "`minted issue <id> <n>` / `minted project <owner> <n>` lines" (~l96); "one-way Issues + Projects v2 mirror" + `write-back` marker (~l115). | seeds 1–2: `internal/app/status_result.go` (`StatusResult`: summary/changes/ready/findings), `internal/app/status_human.go` (`HumanText`), `docket status --json`, `docket schema --operation status`/`maintenance.sweep` (entries: id/kind/disposition/operation/reason/message). Seed 3 owners deferred to Task 3. | **Task 2 (seeds 1–2) — fixed.** Report contract regrounded to the structured `StatusResult` payload (summary counts, `changes`, `ready`, `findings`); removed the `backlog <status>`/`change <id>`/`ready [<id> …]` line grammar, the `pass ok` completion line (→ envelope `result`), and the `harvest <id> <path>` line grammar (harvest deferred). Sweep posture rewritten to the surviving obligations against `maintenance.sweep` (read-vs-mutation, terminal envelope, per-entry `disposition`/`reason` under `applied`, preserve `unknown`, surface-don't-repair, sweep-recovery-distinct-from-finalize-merge; caller-variance sentence preserved). Removed non-existent script pointers `scripts/docket-status.md`, `scripts/board-checks.md`, `scripts/board-refresh.md`, `board-refresh.sh`, `render-board.sh`; board-writer narrative regrounded to the Go metadata transaction. Historical `health checks failed <exit>` line NOT touched (per plan). **Guards:** whole test surface (internal/, tests/, incl. whitespace-flattened) searched for every removed clause — **no dependent guard keyed on the retired copy** (board-checks/render-board hits are synthetic fixtures + anchor-scanner strings; `swept` in inline_role_stop is the guard's own vocabulary). No guard file changed. Surviving-property guards over this file all still green and non-vacuous: `change_0389_sweep_scope`/`change_0397_preflight_op` sentinel phrases sit outside edit regions (preserved verbatim); `TestSkillSizeBudgets` (3056 ≤ 3065 words) and `TestCapabilitySurface` migrate-pin (kept at 9; no new `docket <argv>` literal) reconciled by editing prose, not the pin. Mutation matrix: (a) budget — reddened at 3134w, green at 3056w; (b) capability-surface — reddened on injected `docket schema --operation` literal + migrate count 8, green after removing literal/restoring migrate; (c) `TestProseContracts` — stripped "never that every item succeeded" → RED, restored → green; (d) migrate pin — dropped occurrence → count 8 RED, restored → green. **Task 3 (seed 3) — fixed.** Board-off contradiction resolved: the two "it must not exist" lines now state the current property (disabled rendering commits nothing to `BOARD.md`, a pre-existing board is left untouched, disabled rendering never authorizes deleting a board, the skill stays read-only over `BOARD.md`), pointing at the convention's *Board refresh on status writes* owner. Removed the active mirror instructions: the `minted issue`/`minted project` write-back bullet (with its `docket:config-read-channel: write-back` marker), the `github` mirror-reachability judgment bullet (and its echo in the Health-checks section, now "one judgment check"), the Final-summary "GitHub mirror, if enabled" pointer, and the Board-section `github` one-way-mirror paragraph (`github-mirror.sh` / `issue-minted`/`project-minted` / write-back marker) — replaced by a compatibility pointer grounded in `internal/config/capability.go`. **Owners verified:** `internal/app/planning.go` (`fenceBoardSurface`: a `github` token is refused before any transaction, `ReasonUnsupportedBoardSurface`), `internal/app/derived_views.go` (`includeBoard`: renders/writes `BOARD.md` only when the enabled surfaces produce it, never deletes), `internal/config/capability.go` (`dispSupportedOrDropped`: `github` → Deferred + `blockerDiag`, mutation-blocking); classification discoverable via `diagnostic.config` (`docket diagnostic config --for-mutation`) and `docket capabilities`. **Guards:** removed the write-back marker's population — the config-read-channel scanned corpus (all skills minus the two excluded convention files) now names the config file nowhere; reconciled `internal/repoguard/config_read_channel_test.go` (retired the now-empty `classified >= 3`/`writeBacks >= 1` occurrence floors, added a reader-liveness floor probing the excluded convention body for >= 3 real occurrences, retargeted the deleted-file sibling check to `references/stacked-changes.md`); added a narrow negative guard `internal/repoguard/mirror_writeback_test.go` (`TestNoActiveMirrorWriteBack`: no skill line names a minted mirror identifier AND a record-it-back obligation on the same line; non_vacuity subtest exercises the classifier). Mutation matrix (all via the task gate, `/usr/bin/grep -cF` on whitespace-flattened copies): (a) new mirror guard — planted `issue-minted … record … back` → RED (1 violation), removed → green; (b) config-read-channel RULE — planted unmarked `.docket.yml` → RED, restored → green; (c) reader-liveness floor — probe pointed at a token-free file → RED (0 < 3), restored; (d) config sibling check — retargeted to a nonexistent sibling → RED, restored; (e) budget forward check — stale `github-board-mirror.md` row → RED (missing file), removed. `go test ./internal/repoguard/ -count=1` green. |
| `skills/docket-convention/SKILL.md` | seed 3 (mirror) hit: fixed (Task 3); seeds 4/5 pending | **Seed 3/4/5.** Scan A: "The one writer allowed to touch them afterward is `render-artifact-backlink.sh`" (~l211); "rendered by `render-change-links.sh` from frontmatter" + "written solely by `render-artifact-backlink.sh`" (`## Artifacts`, ~l220); mirror section "owned by the deterministic `github-mirror.sh`" (~l369); "**Derived-view script family**" listing `board-refresh.sh`/`render-board.sh`/`github-mirror.sh`/`render-change-links.sh`/`render-artifact-backlink.sh` (~l371). Scan B: "**Seven skills** get a wrapper"/"except **nine**"/"Those **eight** wrapper-bearing exceptions" (~l113); "superpowers for … docket's own for `build`/`review`" five-step (~l123); "### Lifecycle — eight states" (~l258); "author any of the five surfaces" (~l336); "status→issue mapping across all eight states" (~l369). Scan D: "`github_project:` … auto-create on first github sync" (~l43); mirror paragraph "Projects v2" (~l369); "surviving for frozen / main-mode paths" (~l371); "`PROCEED` (migrated or main-mode)" + "The guard is a no-op in `main`-mode" (~l385). | pending | **Task 3 (seed 3 mirror) — fixed:** the *GitHub board mirror (shared definition)* section (former ~l369) rewritten to one compatibility sentence grounded in `internal/config/capability.go` (`github` unsupported + mutation-blocking; historical `issue:` data preserved, never acted on), dropping the `github-mirror.sh` writer claim, the "all eight states" mapping restatement, and the `[github-board-mirror.md](…)` link (that file is deleted); the incoming *Script contracts* parenthetical link to `github-board-mirror.md` (former ~l63) removed as a dangling reference. Seeds 4/5 pending: Task 4 (config sketch/layers/`board_surfaces`), Task 5 (counts, "Derived-view script family" ~l371, main-mode ~l385). |
| `skills/docket-convention/github-board-mirror.md` | hit: fixed (Task 3 — file deleted) | **Seed 3/5.** Scan A/D: whole file is the active GitHub Issues + Projects v2 mirror recipe — "The `github` board surface mirrors each change to one GitHub issue (and one Projects v2 item)" (~l7); "`github-mirror.sh`" external-write owner (~l7); "**Projects v2.** … writes its `{owner, number}` back into `.docket.yml`" + `write-back` marker (~l17); "logs the skipped status (`scripts/github-mirror.md`)" (~l17). Scan B: "**Status → issue mapping (all eight).**" (~l11). | pending | **Task 3 — DELETED** (`git rm`; preferred retirement of an active recipe for a retired feature). The compatibility statement now lives as one sentence in the convention's *GitHub board mirror* section. Incoming maintained links fixed: convention *Script contracts* parenthetical (removed) and the mirror-section link (removed); `internal/repoguard/budgets_test.go` row deleted; `internal/repoguard/config_read_channel_test.go` sibling-probe retargeted to `references/stacked-changes.md`. Remaining `github-board-mirror` mentions are point-in-time `docs/results/*` records (untouched, per the never-modernize-history rule) and the generated `internal/assets/embedded` copy (Task 8 regenerates). Historical `issue:` data/fixtures NOT removed. |
| `skills/docket-convention/references/stacked-changes.md` | pending | **Seed 5.** Scan A: "parent-side **Stacked children** row is derived at render time by `render-change-links.sh`" (~l21). | pending | Task 5/6: `render-change-links.sh` is a retired Bash owner; current owner is the Go link-block renderer. |
| `skills/docket-convention/references/terminal-close-out.md` | pending | **Seed 5.** Scan D: TOC entry "· [main-mode degradation](#main-mode-degradation) ·" (~l12); "## main-mode degradation" section (~l124). | pending | Task 5 Step 2: delete the `## main-mode degradation` section + its TOC entry; no dangling anchors. |
| `skills/docket-convention/references/dummy-mode.md` | pending | **Seed 5 (count word).** Scan B: "space-separated subset of these **five**" (~l12). | pending | Task 5 Step 3: check whether the count merely restates an adjacent list (delete) or a Go guard floors it (keep + name guard). |
| `skills/docket-finalize-change/references/gate-failure.md` | pending | **Seed 5 (count word).** Scan B: "force every derived view to say **six** different things about one label" (~l95). | pending | Task 6 triage: likely prose describing label mapping, verify against current reality. |
| `skills/docket-build/references/delegation-execution.md` | pending | **Seed 5 (count word).** Scan B: "until change 0370 deletes it. The **six**…" (~l5). | pending | Task 6 triage. |
| `skills/docket-build/references/gate-execution.md` | pending | **Seed 5 (count word).** Scan B: "## The **six** required capabilities" (~l8); "### The **seven** probe scenarios" (~l133); "observed against all **seven**" (~l135). | pending | Task 6 triage: counts floored by a guard? verify. |
| `skills/docket-build/references/gate-execution-evidence.md` | pending | **Seed 5 (count word).** Scan B: "the **six** required capabilities, the mitigation, and each harness's verdict" (~l4). | pending | Task 6 triage. |
| `skills/docket-build/references/gate-caller-loop.md` | pending | **Seed 5 (count word).** Scan B: "The **five** raw verbs — `gate.launch`, `gate.observe`, `gate.stop`, …" (~l117). | pending | Task 6 triage: count restates an adjacent enumerated list. |
| `skills/docket-adr/adr-template.md` | pending | no-hit (mechanical) | pending | Full manual read — Task 7. |
| `skills/docket-adr/SKILL.md` | pending | no-hit (mechanical) | pending | Full manual read — Task 7. |
| `skills/docket-auto-groom/SKILL.md` | pending | no-hit (mechanical) | pending | Full manual read — Task 7. |
| `skills/docket-brainstorm/SKILL.md` | pending | no-hit (mechanical) | pending | Full manual read — Task 7. |
| `skills/docket-build-task/SKILL.md` | pending | no-hit (mechanical) | pending | Full manual read — Task 7. |
| `skills/docket-build/references/task-routing.md` | pending | no-hit (mechanical) | pending | Full manual read — Task 7. |
| `skills/docket-build/SKILL.md` | pending | no-hit (mechanical) | pending | Full manual read — Task 7. |
| `skills/docket-convention/references/agent-layer.md` | pending | no-hit (mechanical) | pending | Full manual read — Task 7. |
| `skills/docket-convention/references/learnings.md` | pending | no-hit (mechanical) | pending | Task 6 names "harvest is deferred from Go v1" wording (~l48) to verify as current guidance; else Task 7. |
| `skills/docket-finalize-change/SKILL.md` | pending | no-hit (mechanical) | pending | Full manual read — Task 7. |
| `skills/docket-groom-next/SKILL.md` | pending | no-hit (mechanical) | pending | Full manual read — Task 7. |
| `skills/docket-implement-next/references/edge-paths.md` | pending | no-hit (mechanical) | pending | Full manual read — Task 7. |
| `skills/docket-implement-next/references/fix-loop.md` | pending | no-hit (mechanical) | pending | Full manual read — Task 7. |
| `skills/docket-implement-next/results-template.md` | pending | no-hit (mechanical) | pending | Full manual read — Task 7. |
| `skills/docket-implement-next/SKILL.md` | pending | no-hit (mechanical) | pending | Full manual read — Task 7. |
| `skills/docket-new-change/change-template.md` | pending | no-hit (mechanical) | pending | Full manual read — Task 7. |
| `skills/docket-new-change/SKILL.md` | pending | no-hit (mechanical) | pending | Task 6 names "scan harvest" (~l51) as judgment vocabulary (likely no-hit); confirm in Task 6/7. |
| `skills/docket-review/SKILL.md` | pending | no-hit (mechanical) | pending | Full manual read — Task 7. |

## Disposition vocabulary (for tasks 2–7)

Each row's final disposition is one of:

- `hit: fixed` — a genuine stale/duplicated-contract defect of this change's class, corrected here
  (with the guard reconciliation recorded).
- `hit: reported (out of scope)` — a real defect outside this change's class; recorded with its
  existing change id when known, not fixed here.
- `no hit: verified current` — read in full and confirmed accurate against the named current owner
  (a true statement about a deferred feature counts as current guidance, not staleness).

No row may remain `pending` after Task 7 (Task 9 verifies).
