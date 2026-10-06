package repoguard

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Ports two size-direction guards off the retired Bash suite:
//
//   tests/test_skill_size_budgets.sh   -> TestSkillSizeBudgets
//   tests/test_dispatch_block_budget.sh -> TestDispatchBlockBudget
//
// Both make a compaction DIRECTION durable (size-target-is-direction): a file
// that regrows past its recorded ceiling reddens, and a ceiling can only be
// moved DOWN in the same diff that slims the file.

// ---------------------------------------------------------------------------
// skill size budgets
// ---------------------------------------------------------------------------

type skillBudget struct {
	rel      string // relative to skills/
	maxLines int
	maxWords int
}

// skillBudgets is the per-file line/word ceiling table for skills/**/*.md,
// carried verbatim from the retired tests/test_skill_size_budgets.sh. To slim a
// file, lower its numbers in the same diff; to add a skill file, add its row
// (the completeness direction below reddens an unbudgeted file).
//
// The word ceilings for docket-convention/SKILL.md, docket-implement-next/SKILL.md,
// and docket-convention/references/stacked-changes.md were re-baselined upward once
// (change 0394) to hold the capability-catalog contract prose: the new startup-check
// capability bootstrap and the catalog-resolved semantic-operation idiom that
// replaced hard-coded `docket <argv>` spellings. Change 0399 re-baselined
// docket-convention/SKILL.md upward once more (7750 -> 7800) to hold the
// authoritative machine-readable request/result schema-surface contract prose — the
// catalog-resolved `schema` operation and its fail-closed descriptor-driven body
// construction; the mandated content cannot fit under 7750 even at minimum-faithful
// phrasing. The ratchet stays in force at the new baselines — they catch any further
// regrowth.
//
// Change 0388 re-baselined docket-convention/SKILL.md (7800 -> 7807),
// docket-finalize-change/SKILL.md (4150 -> 4344), and docket-status/SKILL.md
// (3050 -> 3065) upward once to hold the native post-merge integration-branch-sync
// contract prose it wired into these three skills: the once-per-scope
// repository.sync-integration operation, its safety ladder and closed disposition
// vocabulary, and where the maintenance sweep runs it. This is authored contract
// documentation, not slack — the ceilings are pinned at the exact new word counts,
// so the ratchet still reddens on any further regrowth.
//
// Change 0393's structured feature-worktree payload lines and balanced dispatch
// markers land atop the 0410/0349 baselines in this rebase: docket-build/SKILL.md
// reaches 406 lines, docket-implement-next/SKILL.md 210 lines, and
// docket-finalize-change/SKILL.md 236 lines. That machine-checked syntax cannot
// be compacted into prose without losing the supplied input shape; the ratchet
// remains exact.
//
// Change 0346 re-baselined docket-finalize-change/SKILL.md upward once to hold
// the verified post-merge rebuild contract (step 12): the sync-disposition
// gate, the source ancestry proof, the post-install identity check, and the
// separate binary-rebuild-incomplete report. Authored contract documentation,
// not slack — the ceiling is pinned at the exact new counts and the ratchet
// stays in force.
//
// Change 0354 re-baselined docket-implement-next/SKILL.md (6900 -> 6956) upward once
// to hold the halt-report body-only contract prose in the Step-3 "FUNDAMENTALLY
// invalidated" escape hatch: change.halt owns the `## Run halted` heading and dated
// `###` sub-heading, so the caller-authored report is the section body only, and a
// body with its own column-zero `## ` heading or an unterminated code fence is
// refused invalid-input (invalid-section-markdown, field report). Minimum-faithful
// phrasing exceeds the prior 6900 ceiling; the new baseline is pinned at the exact
// word count so the ratchet still reddens on any further regrowth.
//
// Change 0376 re-baselined docket-build/SKILL.md (385/3850 -> 390/3907),
// docket-build-task/SKILL.md (155/1550 -> 160/1605), and
// docket-implement-next/SKILL.md (word ceiling 6956 -> 6991; line ceiling
// unchanged) upward once to hold the required gate.drive JSON-capture caller
// contract: every credential-consuming invocation must pass --json and capture
// the first response before acting. This is authored contract documentation, not
// slack — the ceilings are pinned at the exact new counts (after trimming
// per-caller repetition of what the shared contract already states), so the
// ratchet still reddens on any further regrowth. The docket-implement-next word
// ceiling carries both re-baselines: 0354's halt-report prose and 0376's
// JSON-capture prose now coexist in the file.
//
// Change 0327 re-baselined docket-convention/references/stacked-changes.md and
// docket-finalize-change/SKILL.md upward once to hold the carry-preservation
// contract prose it wired into stacked close-out: a merged PR destination
// establishes the carry RELATIONSHIP, while the descendant's merged work must be
// separately proven still present in Git (ancestry in the pinned integration
// history, or exact-content at the root's merge result) before any archive. This is
// authored contract documentation, not slack. The finalize ceiling here is the
// exact count of the file that now carries 0327's carry-preservation prose on top of
// main's 0346/0388 re-baselines, so the ratchet still reddens on any further
// regrowth.
//
// Change 0410 re-baselined the required-results workflow surfaces upward once to
// hold the durable-results contract prose (results are now a REQUIRED close-out
// artifact for every implemented change, trivial included; the mandatory Step 6.5
// checkpoint lifecycle; and the build/review capture-ownership boundaries):
// docket-build/SKILL.md (390/3907 -> 404/4054), docket-convention/SKILL.md (word
// 7807 -> 7969), docket-implement-next/SKILL.md (180/6991 -> 201/7530),
// docket-implement-next/references/edge-paths.md (58/800 -> 78/1091),
// docket-implement-next/references/fix-loop.md (185/1900 -> 190/1958),
// docket-implement-next/results-template.md (25/250 -> 51/257 — the five-section
// canonical template replaced the terse optional stub), and
// docket-review/SKILL.md (word 900 -> 913). Authored contract documentation, not
// slack — the ceilings are pinned at the exact new counts, so the ratchet still
// reddens on any further regrowth.
//
// Change 0440 re-baselined the same surfaces once more for the reader-first
// results shape (required Human action statement; Human actions and testing;
// Known issues and follow-ups): results-template.md and the Step 6.5 /
// convention prose that names the sections. Authored contract documentation,
// not slack — ceilings stay pinned at the exact new counts.
//
// Change 0349's reserve-before-dispatch prose follows the 0410 baseline. Together
// they bring docket-finalize-change/SKILL.md to 226 lines and 5179 words, so its
// rounded ceilings remain a durable ratchet rather than silently dropping either
// change's contract.
//
// Change 0416 re-baselined docket-build/SKILL.md and docket-build-task/SKILL.md
// upward once to hold the complete start-ready scope-bundle contract: the
// controller's dispatch payload hands the worker every identity value
// prepare-scope pinned, and the worker passes the bundle through to its scoped
// task-owned gate.drive.start unchanged — closing the identity omission that
// made the driver reject every scoped build-task start before its first test.
// Authored contract documentation, not slack — the ceilings are pinned at the
// exact new counts, so the ratchet still reddens on any further regrowth.
//
// Change 0405 added the sequential-drive contract: one recovery scope carries a
// sequence of task-owned drives, every successor start presents the predecessor
// receipt, and terminal acknowledgement consumes the final PASSED/FAILED result.
// Changes 0420 and 0421 then made build-gate capture shell-safe and bounded the
// repair cycle with `build_max_attempts` (default 4). These ceilings include all
// three changes and are pinned at the exact new counts, so the ratchet still
// reddens on any further regrowth.
//
// Change 0375 re-baselined the caller-contract surfaces that document the new
// worktree-admission and Stop/cancel/resume contract: gate-caller-loop.md gained
// the one-live-gate-per-worktree admission section, docket-build-task/SKILL.md the
// worktree-busy-is-not-a-retry rule, gate-failure.md the shared-admission note for
// the scopeless finalize gate, and both docket-implement-next surfaces the run-tracker
// resume refusals. Authored contract, not slack — pinned at the exact new counts.
//
// Change 0448 re-baselined docket-implement-next/SKILL.md (210/7716 -> 214/8223):
// Step 0 gained the named-invocation branch (a single explicit id skips the
// maintenance preflight and never falls back to selection) and the bounded
// own-dependency closeout paragraph, both pinned verbatim by the
// change_0448_named_preflight_skip prose-contract row. Authored contract, not
// slack — pinned at the exact new counts. A review fix then narrowed that closeout
// to `implemented` depends_on only (8223 -> 8233): a stack-base refusal never
// triggers an ancestor closeout, because that closeout cannot prove the carry of a
// not-yet-stacked-merged named change and returns children-retarget-required.
// A second review fix named the real status field (8233 -> 8241): the closeout
// set is read off `unmet_dependencies` (app.StatusChange), since the status
// projection exposes no `depends_on` field. A third review fix keyed the closeout mapping on
// the envelope `result` and reason `pr-not-merged` rather than `disposition`
// tokens, and named the still-refusing re-read (8241 -> 8270).
// Progressive disclosure then moved that whole closeout paragraph into
// references/edge-paths.md behind a blocking pointer at its trigger, slimming
// SKILL.md (214/8270 -> 214/8025) and growing edge-paths.md (93/1261 -> 118/1554)
// by the relocated contract; the prose-contract row follows it there.
//
// Change 0419 (repair-attempts configurable, built-in default 3 -> 10) added the
// required repair-attempt budget payload line and rewired the repair contract in
// the finalize surfaces: docket-finalize-change/SKILL.md (236 -> 238 lines) and
// docket-finalize-change/references/gate-failure.md (1450 -> 1465 words). Authored
// contract, not slack — pinned at the exact new counts.
//
// Change 0488 lowered docket-build/SKILL.md, docket-build-task/SKILL.md,
// gate-caller-loop.md, gate-execution.md, and docket-implement-next/SKILL.md to
// their new sizes: build-task workers run focused tests directly under
// `timeout --kill-after=10s 10m` and call no gate operation, so the 0405/0416/
// 0459/0467 worker-scope contract the notes above re-baselined for is gone.
// Pinned at the exact new counts — the ratchet reddens on any regrowth.
//
// Change 0490 rewrote the gate-caller-loop.md admission section and the
// gate-failure.md finalize note around the supervisor-held worktree lock that
// replaced the durable slot: a busy worktree means a live supervisor holds the
// lock, and launch-unconfirmed is no longer an admission refusal. Pinned at the
// exact new counts.
var skillBudgets = []skillBudget{
	{"docket-adr/SKILL.md", 86, 1143},        // 0531: the metadata worktree and remote come from the repository.prepare context (86/1129 -> 86/1143); 0502: one metadata layout — the docket branch and the .docket/ worktree (86/1123 -> 86/1129); 0502: the adr-unpublished health-check text is deleted (86/1148 -> 86/1123); 0502: the deferred-publication section and narration are deleted (110/1600 -> 86/1148)
	{"docket-auto-groom/SKILL.md", 64, 1638}, // 0532: the private-repository writing rule and visibility hand-off (64/1606 -> 64/1638); 0531: the metadata worktree and remote come from the repository.prepare context (64/1597 -> 64/1606); 0502: "loop" means /loop only (64/1596 -> 64/1597); 0502: one metadata layout — the docket branch and the .docket/ worktree (64/1595 -> 64/1596); 0502: the spec draft stays out of .docket/ (64/1572 -> 64/1595); 0502: auto-groom is per-change only (64/1582 -> 64/1572); 0502: the dummy-mode pointer is deleted (66/1625 -> 64/1582); 0382: typed change.groom abstain replaces the plain-git abstain commit prose (word ceiling 1750 -> 1627)
	{"docket-brainstorm/SKILL.md", 83, 722},  // 0532: the private-repository writing rule and visibility hand-off (81/690 -> 83/722); 0502 review: the scope stop names new-change Steps 3–5 accurately (81/685 -> 81/690); 0502: the role runs on a human request, not a binding (80/671 -> 81/680); 0502: drop change citations and Go v1 narration (81/680 -> 81/679); 0502: description names the human request, no binding or change citation (81/679 -> 81/685)
	{"docket-build/SKILL.md", 386, 3878},     // 0532: the private-repository writing rule and visibility hand-off (384/3852 -> 386/3878); 0531: the metadata worktree and remote come from the repository.prepare context (384/3851 -> 384/3852); 0502: the install operation replaces install.sh; the wrapper skills list (385/3850 -> 385/3856); 0502: the observation-budget and build-checkpoint export reads are deleted (395/3942 -> 385/3850); 0502: fixed build role; undispatchable tier halts (398/3974 -> 395/3942); 0502: the gate-execution pointer paragraph is deleted with the reference (403/4007 -> 398/3974); 0493: the gate relaunch is retired (404/4023 -> 403/4007); 0491: the run id is retired (406/4037 -> 404/4023); 0488 review fix: +the dispatch payload names the branch (406/4034 -> 406/4037); 0488 review: build-owned starts carry --change-id/--run-context (404/4018 -> 406/4034); 0488: workers run tests directly; the dispatch payload carries no worker scope identity, the task-level WAITING/takeover section is gone, the controller runs every post-repair attempt, +time-limit audit (439/4479 -> 404/4018); 0467 review fix: +the repair dispatch payload carries the run id for the build-owned post-fix re-run (436/4443 -> 439/4479); 0467: +run-id threading on prepare-scope and the build-owned start; the bundle carries no run id (432/4391 -> 436/4443); 0459: +continuation carries the claimed verdict, closed-scope statement, and fresh prepare-scope bundle (424/4284 -> 432/4391); 0405: sequential-drive contract; 0420: shell-safe capture; 0421: budgeted repair cycle (see note above); 0502: drop change citations and Go v1 narration (385/3856 -> 384/3851)
	// 0154: docket-build/references/delegation-execution.md removed — it was the
	// evidence record for the Bash delegation facade that change 0370 deleted; its
	// budget row is deleted with it.
	{"docket-build/references/gate-driver.md", 138, 1552}, // 0502 review: the raw-verb section names finalize's repaired-head re-gate (137/1546 -> 138/1552); 0502: "loop" means /loop only (137/1550 -> 137/1546); 0502: finalize's repaired-head re-gate is a direct caller (136/1523 -> 137/1550); 0502: the gate-execution intro reference is deleted (138/1555 -> 136/1523); 0502: renamed from gate-caller-loop.md; 0497: +halted-gate tree-survives finding relay (134/1510 -> 138/1555); 0491: the run id is retired (134/1513 -> 134/1510); 0490: the admission section describes the supervisor-held worktree lock; launch-unconfirmed is no longer a refusal (136/1511 -> 134/1513); 0488 review fix: finalize reaches the driver only through finalize.rebase; task-era handoff clauses dropped (word ceiling 1504 -> 1511); 0488 review: start row carries --change-id (word ceiling 1491 -> 1504); 0488: worker-scope/takeover/acknowledge rows and the parent-takeover section removed; callers are the full-suite gates (175/1872 -> 136/1491); 0467: +prepare-scope --run-id and scope-inherited start run id (word ceiling 1826 -> 1872); 0375: +worktree-admission section (word ceiling 1750 -> 1826)
	{"docket-build/references/task-routing.md", 50, 500},
	{"docket-build-task/SKILL.md", 173, 1695}, // 0532: the private-repository writing rule and visibility hand-off (168/1640 -> 173/1695); 0531: the metadata worktree and remote come from the repository.prepare context (168/1637 -> 168/1640); 0488: scoped drive protocol replaced by direct foreground tests under timeout; three outcomes (211/2235 -> 168/1637); 0467 review fix: +the repair re-run run-id exception (206/2183 -> 211/2235); 0467: +the run id rides on the scope, never the bundle (204/2151 -> 206/2183); 0459: +post-handoff continuation never acknowledges or reuses the transferred scope (188/1964 -> 204/2151); 0405: sequential-drive receipt and acknowledgement; 0420: shell-safe capture; 0375: worktree-busy-not-a-retry rule (179/1842 -> 188/1964)
	{"docket-convention/SKILL.md", 361, 7263}, // 0532: the private-repository writing rule and visibility hand-off (359/7160 -> 361/7263); 0531 review: skills say "the metadata branch", naming `docket` once for shared mode (359/7155 -> 359/7160); 0531: the metadata worktree and remote come from the repository.prepare context (357/7062 -> 359/7155); 0530: plan, results, and build evidence live on the docket branch — the plan-writer's one metadata operation, the attach operations as file-backlink writers, relative same-branch links, and the new ## Build evidence record section (357/7026 -> 357/7062); 0529 review: the backlink block names its PR-body writers beside artifact.backlink, one shared renderer (357/7004 -> 357/7026); 0515: the repair sign-off is retired; a green repair merges (357/7007 -> 357/7004); 0502 rebase onto 0510: kept the backlog-match follow-up clause in the results-artifact paragraph (357/7004 -> 357/7007); 0502 review: the review-tier dispatch is the controller's own (357/6995 -> 357/7004); 0502: configuration read paths corrected — operations read .docket.yml from origin's default-branch tip, the repository setup operations from the primary worktree (357/6959 -> 357/6995); 0502: "loop" means /loop only (357/7036 -> 357/7037); 0502: one metadata layout — the docket branch and the .docket/ worktree (357/7035 -> 357/7036); 0502: agent layer names user-level wrappers and global-only pins (357/7024 -> 357/7035); 0502: typed operations replace the Bash-era script names; ## Run halted facts corrected (359/7092 -> 357/7024); 0502: terminal publish, the learnings cap, and the repository auto_groom default are deleted (362/7256 -> 359/7092); 0502: the auto-capture export paragraph is deleted (370/7334 -> 362/7256); 0502: the configuration contract rewritten from the schema (371/7488 -> 370/7334); 0502: fixed role skills; halt posture (372/7660 -> 371/7488); 0502: record blocks mirror the renderers (367/7619 -> 372/7660); 0502: the dummy-mode shared definition is deleted (390/7848 -> 367/7619); 0410: +required-results lifecycle prose; 0399: +schema request/result contract prose; 0388: +sync-integration prose (see note above); 0502: drop change citations and Go v1 narration (357/7037 -> 357/6958); 0502: the harness adapter is named without a docket source path (357/6958 -> 357/6959)
	// 0154: docket-convention/github-board-mirror.md removed — the GitHub mirror is
	// retired (unsupported, mutation-blocking); its budget row is deleted with it.
	{"docket-convention/references/agent-layer.md", 164, 1553},       // 0535: a private repository's instructions file and its user-level triggers (148/1419 -> 164/1553); 0502: rewritten around user-level wrappers and global-only pins (202/2346 -> 148/1419); 0502: the legacy agents-file migration clause is deleted (202/2349 -> 202/2346)
	{"docket-convention/references/close-out.md", 141, 1347},         // 0531: the metadata worktree and remote come from the repository.prepare context (141/1349 -> 141/1347); 0502: "loop" means /loop only (144/1375 -> 144/1374); 0502: one metadata layout — the docket branch and the .docket/ worktree (144/1376 -> 144/1375); 0502: the scripts/<name>.md contract pointer becomes the schema operation (144/1374 -> 144/1376); 0502: the terminal-publication step is deleted (240/2150 -> 144/1374); 0474: reference renamed (ADR-0129 row 59a); ceilings unchanged; 0502: drop change citations and Go v1 narration (144/1374 -> 144/1356); 0502: the docket-source test citations are deleted (144/1356 -> 141/1349)
	{"docket-convention/references/learnings.md", 69, 487},           // 0535: promotion lands in the private instructions file in a private repository (66/457 -> 69/487); 0502: one metadata layout — the docket branch and the .docket/ worktree (66/455 -> 66/457); 0502: learning.record/learning.update are the writers; the capacity section is deleted (84/580 -> 66/455)
	{"docket-convention/references/stacked-changes.md", 214, 2149},   // 0502: typed operations and binary-emitted findings replace the Bash-era names (213/2125 -> 214/2149); 0502: the deferred descendant-publication sentence is deleted (215/2140 -> 213/2125); 0327: +carry-preservation contract prose (see note above)
	{"docket-finalize-change/SKILL.md", 241, 6033},                   // 0532 review: both finalize gate dispatches carry the Visibility payload line (239/6020 -> 241/6033); 0532: the private-repository writing rule and visibility hand-off (239/6000 -> 239/6020); 0529 review: the PR-backlink repoint is a best-effort follow-up after the archive push, for root-archived descendants too (239/5984 -> 239/6000); 0529: close-out repoints the merged PR description's backlink; pr-backlink-pending is a retryable warning (239/5936 -> 239/5984); 0525 review: workspace-remnant rides any finished-workspace disposition and is named in the final report; branch-rules-unavailable comes only from the probing run (239/5910 -> 239/5936); 0525: plan-gated merges carry branch-rules-unavailable into closeout notes; cleanup reports workspace-remnant and names what blocked a workspace (239/5820 -> 239/5910); 0515 review: the refused-notes fallback names every dropped note in the final report (238/5809 -> 239/5820); 0515 review: a repair whose run halts before closeout is named in finalize.block and carried into a later closeout (238/5708 -> 238/5809); 0515: the repair sign-off is retired; a green repair merges (238/5808 -> 238/5708); 0517: the repaired-head evidence.record carries --owner finalize and records finalize.test_command (238/5790 -> 238/5808); 0502 review: clear-block re-reads the revision after the block lands (238/5771 -> 238/5790); 0502: repair sign-off records finalize.block before publishing the repaired head (238/5733 -> 238/5771); 0502: "loop" means /loop only (238/5729 -> 238/5733); 0502: repair sign-off is the human running finalize.clear-block (238/5666 -> 238/5729); 0502: the repaired head is re-gated through the gate driver (236/5568 -> 238/5666); 0502: the dummy-mode paragraph is deleted (238/5641 -> 236/5568); 0455: +record-invalid refusal (structural scope, findings remedy, merged-outside-docket precedence) in step 8 (word ceiling 5520 -> 5647); 0442: +post-publication base-advance guidance (word ceiling 5421 -> 5520); de-duplicated the shared forward-rebase mechanic against the 0438 unpublished-case paragraph (reclaimed 57 words), but the distinct published-refresh facts plus the retained 0438 guidance cannot fit the old ceiling without deleting required guidance; 0411: +reconciliation-write recovery exception paragraph in the resolver loop (ceilings 238/5232 -> 239/5421); 0413: +generated-bundle mixed-conflict handoff sentence in the resolver-loop block (word ceiling 5200 -> 5232); 0419: +repair-attempt budget payload line and rewired repair contract (line ceiling 236 -> 238); 0393: +exact payload, marker, and direct-dispatch lines atop 0349/0410 (see note above)
	{"docket-finalize-change/references/gate-failure.md", 162, 2111}, // 0532 review: the gate-agent payload names Visibility (161/2109 -> 162/2111); 0515 review: the refused-notes fallback names every dropped note in the final report (160/2098 -> 161/2109); 0515 review: a repair whose run halts before closeout is named in finalize.block and carried into a later closeout (153/2005 -> 160/2098); 0515: the repair sign-off is retired; a green repair merges (160/2086 -> 153/2005); 0517: the repaired-head evidence.record carries --owner finalize (159/2081 -> 160/2086); 0502 review: clear-block re-reads the revision after the block lands (157/2062 -> 159/2081); 0502: repair sign-off records finalize.block before publishing the repaired head (155/2028 -> 157/2062); 0502: the embedded-bundle rule is conditioned on docket's own repository (155/2024 -> 155/2028); 0502: "loop" means /loop only (155/2027 -> 155/2024); 0502: repair sign-off is the human running finalize.clear-block; the marker lifecycle names what removes it (154/2013 -> 155/2027); 0502: the repaired head is re-gated through the gate driver (148/1937 -> 154/2013); 0497: +wait-for-the-leftover-group sentence (146/1901 -> 148/1937); 0493: the gate relaunch is retired (145/1892 -> 146/1901); 0491: the run id is retired (145/1894 -> 145/1892); 0490: the shared-slot note became the shared worktree-lock note (147/1901 -> 145/1894); 0411: +reconciliation-write exception section and abort-set carve-out (ceilings 135/1472 -> 147/1901); 0413: +conflicted_paths-lists-authored-only rule in the resolver-report section (line ceiling 133 -> 135, word ceiling 1465 -> 1472); 0419: +repair-attempt budget payload and rewired repair contract prose (word ceiling 1450 -> 1465); 0349: +reserve-before-dispatch resolver protocol prose; 0375: +worktree-slot note for the scopeless finalize gate (120/1300 -> 133/1450)
	{"docket-groom-next/SKILL.md", 76, 2141},                         // 0532: the private-repository writing rule and visibility hand-off (76/2109 -> 76/2141); 0531: the metadata worktree and remote come from the repository.prepare context (76/2102 -> 76/2109); 0502 rebase onto 0509: carried the edit-a-stub revise route and groom-exit-scoped STOPs (76/2012 -> 76/2102); 0502: "loop" means /loop only (76/2011 -> 76/2012); 0502: one metadata layout — the docket branch and the .docket/ worktree (76/2010 -> 76/2011); 0502: selection bands key on auto_groomable itself (76/2009 -> 76/2010); 0502: invoke superpowers:brainstorming by name (76/2037 -> 76/2009); 0502: the dummy-mode pointer is deleted (78/2080 -> 76/2037); 0461: +Step-4 retitle paragraph (title on change.groom; lines 77 -> 78, words 1996 -> 2081); +not-retitleable refusal code (2081 -> 2082); 0382: +typed rearm exit (word ceiling 1889 -> 1996); 0445: +revise route for already-groomed explicit ids (Step 1) and the fifth Step-4 exit (word ceiling 1650 -> 1813); +spec_version pin for a spec-body revise (1813 -> 1849); +revise spec_markdown excludes the backlink block (1849 -> 1850); +revise in the description and the revise contended/board clauses (1850 -> 1889)
	{"docket-implement-next/SKILL.md", 215, 8411},                    // 0532: the private-repository writing rule and visibility hand-off (212/8282 -> 215/8411); 0531: the metadata worktree and remote come from the repository.prepare context (216/8443 -> 212/8282); 0517: evidence.record reads the gate command from build configuration (216/8438 -> 216/8443); 0502 rebase onto 0510: carried the Step 6.5 Backlog match paragraph and verdict pointers (214/8039 -> 216/8438); 0502 review: the controller dispatches the review tier wrapper; the skill dispatches nothing (214/8023 -> 214/8039); 0502: "loop" means /loop only (214/8058 -> 214/8057); 0502: one metadata layout — the docket branch and the .docket/ worktree (214/8042 -> 214/8058); 0502: the review payload names the resolved build.gate value; build_gate spellings become build.gate (214/8020 -> 214/8042); 0502: change.reclaim replaces reclaim-claims; the claim removes a leftover ## Run halted (214/8009 -> 214/8020); 0502: terminal-publication narration is deleted (214/8037 -> 214/8009); 0502: the severity floor is read as review.min_fix_severity through diagnostic.config (214/8034 -> 214/8037); 0502: the fixed role skills replace the $SKILL_* sites (214/8257 -> 214/8034); 0502: the dummy-mode paragraph and PR-body clause are deleted (216/8356 -> 214/8257); 0497: +Step 5 halted-build evidence kept verbatim in ## Run halted (216/8325 -> 216/8356); 0498 review: checkpoint (ii) persist wording (8320 -> 8325); 0498: +Step 6.5 review-findings homes, condensation exception, Human-actions class rule (214/8161 -> 216/8320); 0491: the run id is retired (214/8183 -> 214/8161); 0488 review: build-owned starts carry --change-id/--run-context (word ceiling 8162 -> 8183); 0488: run context and run id name only build-owned starts (word ceiling 8175 -> 8162); 0467 review fix: +the repair worker's post-fix re-run is a build-owned start (word ceiling 8165 -> 8175); 0467: +run id threaded to prepare-scope and build-owned starts (word ceiling 8080 -> 8165); 0455: +pr.publish record-invalid refusal clause (word ceiling 8025 -> 8080); 0448: +named-invocation branch; bounded own-dependency closeout moved to edge-paths.md (ceilings 210/7716 -> 214/8270 -> 214/8025); 0393: +exact payload, marker, and direct-dispatch lines atop 0410/0354/0376; 0375: +run-tracker resume pointer (word ceiling 7530 -> 7547); 0440: reader-first results prose; 0502: drop change citations and Go v1 narration (214/8057 -> 214/8023)
	{"docket-implement-next/references/edge-paths.md", 109, 1265},    // 0532: the private-repository writing rule and visibility hand-off (109/1226 -> 109/1265); 0531: the metadata worktree and remote come from the repository.prepare context (118/1499 -> 109/1226); 0502: one metadata layout — the docket branch and the .docket/ worktree (118/1507 -> 118/1509); 0502: run.verify replaces verify-run (118/1505 -> 118/1507); 0502: terminal-publication narration is deleted (118/1513 -> 118/1505); 0502: the deferred results-only permit clause is deleted (118/1546 -> 118/1513); 0502: no custom plan location (118/1547 -> 118/1546); 0491: the run id is retired (118/1554 -> 118/1547); 0410: +resume/recovery + required-results reconciliation; 0375: +run-tracker resume refusals (78/1091 -> 93/1261); 0448: +named own-dependency closeout moved from SKILL.md (93/1261 -> 118/1554); 0502: drop change citations and Go v1 narration (118/1509 -> 118/1499)
	{"docket-implement-next/references/fix-pass.md", 189, 1963},      // 0532: the private-repository writing rule and visibility hand-off (187/1943 -> 189/1963); 0502 rebase onto 0510: carried the backlog-verdict pointer for reported findings (186/1929 -> 187/1943); 0502: "loop" means /loop only (190/1962 -> 190/1967); 0502: undispatchable fix worker halts (192/1991 -> 190/1962); 0502: renamed from fix-loop.md; 0498 review: results-file antecedent (1989 -> 1991); 0498: +review-findings condensation at final consolidation (190/1958 -> 192/1989); 0410: +findings-to-results checkpoint linkage (see note above); 0502: drop change citations and Go v1 narration (190/1967 -> 186/1929)
	{"docket-implement-next/results-template.md", 70, 545},           // 0502 rebase onto 0510: carried the backlog verdict and next action in the Known issues placeholder (68/506 -> 70/545); 0502: inline review replaces the custom review skill (68/511 -> 68/510); 0498: +whole-branch review summary line in Verification performed (64/446 -> 68/511); 0440: reader-first template — action statement + merged Known issues (see note above); 0502: drop change citations and Go v1 narration (68/510 -> 68/506)
	{"docket-review/SKILL.md", 114, 966},                             // 0531: the metadata worktree and remote come from the repository.prepare context (114/965 -> 114/966); 0502: the evidence rule keys on the payload's build.gate value; gate-off skipped records are expected (110/913 -> 114/965); 0410: +findings-return capture contract (see note above)
	{"docket-new-change/SKILL.md", 57, 1635},                         // 0532: the private-repository writing rule and visibility hand-off (57/1603 -> 57/1635); 0531: the metadata worktree and remote come from the repository.prepare context (57/1596 -> 57/1603); 0502 review: omitted draft-time scalars are described exactly (57/1591 -> 57/1596); 0502: "loop" means /loop only (57/1593 -> 57/1591); 0502: one metadata layout — the docket branch and the .docket/ worktree (57/1590 -> 57/1593); 0502: terminal-publication narration and the repository auto_groom default are deleted (57/1609 -> 57/1590); 0502: invoke superpowers:brainstorming by name (57/1636 -> 57/1609); 0502: the dummy-mode pointer is deleted (59/1674 -> 57/1636); 0445: +pointer to the docket-groom-next revise path after landing (word ceiling 1700 -> 1706); 0382: draft-time scalars moved into change.create (ceiling 1706 -> 1675)
	{"docket-status/SKILL.md", 126, 2923},                            // 0531: the metadata worktree and remote come from the repository.prepare context (126/2921 -> 126/2923); 0530: artifact-missing names where a plan or results file may live — the docket branch, or the integration branch for a record closed before the move (126/2904 -> 126/2921); 0529 review: full-scope cleanup retries also repoint stale merged-PR description backlinks; repair --pr-backlinks previews them (126/2880 -> 126/2904); 0502: one metadata layout — the docket branch and the .docket/ worktree (126/2876 -> 126/2880); 0502: findings named as the binary emits them (126/2852 -> 126/2876); 0502: deferred learnings and publication narration is deleted (127/2956 -> 126/2852); 0502: the dummy-mode pointer is deleted (129/2985 -> 127/2956); 0388: +sync-integration prose (see note above)
}

// wcLines counts lines the way `wc -l` does: the number of newline bytes.
func wcLines(content string) int { return strings.Count(content, "\n") }

// wcWords counts words the way `wc -w` does: whitespace-separated tokens.
func wcWords(content string) int { return len(strings.Fields(content)) }

func TestSkillSizeBudgets(t *testing.T) {
	root := guardRoot(t)

	// Forward: every budgeted file exists and is within both ceilings.
	for _, b := range skillBudgets {
		rel := "skills/" + b.rel
		p := filepath.Join(root, filepath.FromSlash(rel))
		data, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("budgeted skill file missing/unreadable %s: %v", rel, err)
			continue
		}
		content := string(data)
		if l := wcLines(content); l > b.maxLines {
			t.Errorf("%s is %d lines, over its %d-line budget — slim it or lower the ceiling in-diff", rel, l, b.maxLines)
		}
		if w := wcWords(content); w > b.maxWords {
			t.Errorf("%s is %d words, over its %d-word budget — slim it or lower the ceiling in-diff", rel, w, b.maxWords)
		}
	}

	// Reverse (correspondence): every skills/**/*.md carries a budget row. An
	// unbudgeted new skill file reddens rather than shipping unmeasured.
	budgeted := make(map[string]bool, len(skillBudgets))
	for _, b := range skillBudgets {
		budgeted[b.rel] = true
	}
	var found, missing []string
	for _, rel := range maintainedPop(t, root) {
		if !strings.HasPrefix(rel, "skills/") || !strings.HasSuffix(rel, ".md") {
			continue
		}
		sub := strings.TrimPrefix(rel, "skills/")
		found = append(found, sub)
		if !budgeted[sub] {
			missing = append(missing, rel)
		}
	}
	if len(found) < 20 {
		t.Fatalf("population floor: only %d skills/**/*.md found (expected >= 20)", len(found))
	}
	if len(missing) != 0 {
		slices.Sort(missing)
		t.Errorf("skills/**/*.md files with no budget row (add one):\n%s", strings.Join(missing, "\n"))
	}

	t.Run("non_vacuity", func(t *testing.T) {
		// A 2-line/2-word body must exceed a 1-line/1-word budget.
		body := "a b\nc d\n"
		if wcLines(body) <= 1 {
			t.Errorf("line counter is vacuous: %q counted <= 1 line", body)
		}
		if wcWords(body) <= 1 {
			t.Errorf("word counter is vacuous: %q counted <= 1 word", body)
		}
	})
}

// ---------------------------------------------------------------------------
// dispatch block budget
// ---------------------------------------------------------------------------

const (
	dispatchStart  = "docket:dispatch:start"
	dispatchEnd    = "docket:dispatch:end"
	dispatchBudget = 932  // 0501: run.start's stop note replaces the "honest owner-lifecycle caveat" wording, and coordinators are told not to relay it (was 897); 0491: the run id is retired from the run-tracker block; the 0443 operation-id wording is added (was 1154); 0477: the Codex request-file sentence labels the run context for `--run-context` on change.claim and the gate drive, since the gate drive now takes the same flag (was 1153); 0467 review fix-3: the Codex agent.enter request file also carries the unchanged run id for --run-id (was 1140); 0467: step 1 copies the <run-id> into the dispatch prompt alongside the run context (was 1137); 0375: step 1 now states the arm prints the run id and where it threads (run.cancel --run-id and every --run-id dispatch flag), so the documented human Stop path is followable; this rides atop the earlier 0375 Stop/cancel + resume-after-stop contract (was 1110). Re-baselined at the exact new count; still strictly below the retired roster (the anti-regrowth invariant below).
	dispatchOld    = 1156 // pre-0334 roster block; the ceiling must stay strictly below it.
)

// dispatchBlockWords returns the word count of the managed dispatch block
// between its markers (markers excluded), and whether both markers were found.
func dispatchBlockWords(content string) (int, bool) {
	var inBlock bool
	var b strings.Builder
	sawStart, sawEnd := false, false
	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(line, dispatchStart) {
			inBlock = true
			sawStart = true
			continue
		}
		if strings.Contains(line, dispatchEnd) {
			inBlock = false
			sawEnd = true
			continue
		}
		if inBlock {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return wcWords(b.String()), sawStart && sawEnd
}

func TestDispatchBlockBudget(t *testing.T) {
	root := guardRoot(t)

	// Direction, made durable: the ceiling is strictly below the pre-0334 actual,
	// so the block cannot regrow back toward the retired roster without reddening.
	if !(dispatchBudget < dispatchOld) {
		t.Fatalf("BUDGET (%d) must stay strictly below the recorded pre-0334 actual (%d)", dispatchBudget, dispatchOld)
	}

	// Measure the committed always-loaded surface (the enduring artifact that
	// rides every turn's context) rather than regenerating it: the Go emitter
	// internal/harness/dispatch.go now owns emission, and the committed AGENTS.md
	// is what a parent harness actually loads.
	content := readMaintained(t, root, "AGENTS.md")
	words, ok := dispatchBlockWords(content)
	if !ok {
		t.Fatalf("AGENTS.md is missing the docket dispatch block markers")
	}
	if words < 1 {
		t.Fatalf("the dispatch block is empty — the marker scan found no content")
	}
	if words > dispatchBudget {
		t.Errorf("the AGENTS.md dispatch block is %d words, over its %d-word budget", words, dispatchBudget)
	}

	t.Run("non_vacuity", func(t *testing.T) {
		fixture := "x\n" + dispatchStart + "\nalpha beta gamma\n" + dispatchEnd + "\ny\n"
		if got, ok := dispatchBlockWords(fixture); !ok || got != 3 {
			t.Errorf("dispatch block extractor miscounted a 3-word fixture: got=%d ok=%v", got, ok)
		}
		// The budget comparison is non-vacuous: 2 words exceeds a 1-word budget.
		if !(2 > 1) {
			t.Errorf("word-budget comparison is vacuous")
		}
	})
}
