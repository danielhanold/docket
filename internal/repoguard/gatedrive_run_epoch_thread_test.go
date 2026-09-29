package repoguard

// Change 0467: the run epoch the gated parent's arm prints must reach every call
// that mints a recovery scope or starts a build-owned drive, while a build-task
// worker's scoped starts never carry it — the driver hands a scoped start the
// epoch its scope pinned. The one worker exception is the integration-repair
// worker's build-owned post-fix full-suite re-run: docket-build hands it the
// epoch in the repair dispatch payload (0467 review fix-1). Prongs over maintained workflow markdown (isWorkflowMD, so the
// embedded mirrors are scanned too):
//   (A) every paragraph referencing gate.drive.prepare-scope carries --run-id;
//   (B) every build-owned gate.drive.start paragraph (--owner build) carries
//       --run-id;
//   (C) no scoped task-owned start paragraph (--owner task) carries --run-id —
//       the worker passes none; the scope supplies it;
//   (D) the repair worker's post-fix re-run is a build-owned start site in both
//       docket-build and docket-build-task (floor), so prong B binds it to
//       --run-id — the exception is pinned, and prong C stays unweakened.
// TestRunGateCopiesEpochIntoDispatchPrompt (below) binds the managed run-gate
// source to copying the epoch into the dispatch prompt.
// Site discovery is keyed on syntactic shape, never a per-file allowlist; the
// shared caller contract (sharedContractRel) is the operation reference, not a
// caller, and is excluded exactly as in TestGateDriveScopedStartIdentity.
// Residual risk, recorded not hidden: an instruction that names the operation
// without its `gate.drive.` prefix (a bare `prepare-scope`), or a build-owned
// start without the --owner build token in the same paragraph, is not a site;
// at run time the driver still fences such an epoch-less start against an
// epoch-owned worktree (stale-run-epoch).

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/harness"
)

const implementNextSkillRel = "skills/docket-implement-next/SKILL.md"

var (
	prepareScopeOpRe = regexp.MustCompile(`gate\.drive\.prepare-scope`)
	ownerBuildRe     = regexp.MustCompile(`--owner build(?:[^a-z-]|$)`)
	repairRerunRe    = regexp.MustCompile(`(?i)post-fix re-run`)
	runEpochFlagRe   = regexp.MustCompile(`--run-id(?:[^a-z-]|$)`)
)

// isPrepareScopeSite: a collapsed paragraph that references the
// gate.drive.prepare-scope operation.
func isPrepareScopeSite(p string) bool { return prepareScopeOpRe.MatchString(p) }

// isBuildOwnerStartSite: a collapsed paragraph that references gate.drive.start
// AND carries the --owner build token.
func isBuildOwnerStartSite(p string) bool {
	return startOpRe.MatchString(p) && ownerBuildRe.MatchString(p)
}

// isRepairRerunSite: a build-owned start paragraph that is about the
// integration-repair worker's post-fix re-run.
func isRepairRerunSite(p string) bool {
	return isBuildOwnerStartSite(p) && repairRerunRe.MatchString(p)
}

// carriesRunEpoch: the paragraph carries the --run-id flag token.
func carriesRunEpoch(p string) bool { return runEpochFlagRe.MatchString(p) }

func TestGateDriveRunEpochThreaded(t *testing.T) {
	root := guardRoot(t)
	var violations []string
	prepSites := map[string]int{}
	buildSites := map[string]int{}
	taskSites := map[string]int{}
	repairSites := map[string]int{}
	for _, rel := range maintainedPop(t, root) {
		if !isWorkflowMD(rel) || strings.HasSuffix(rel, sharedContractRel) {
			continue
		}
		for _, p := range paragraphs(readMaintained(t, root, rel)) {
			if isPrepareScopeSite(p) {
				prepSites[rel]++
				if !carriesRunEpoch(p) {
					violations = append(violations, fmt.Sprintf(
						"%s: gate.drive.prepare-scope instruction lacks --run-id: %.160s", rel, p))
				}
			}
			if isBuildOwnerStartSite(p) {
				buildSites[rel]++
				if isRepairRerunSite(p) {
					repairSites[rel]++
				}
				if !carriesRunEpoch(p) {
					violations = append(violations, fmt.Sprintf(
						"%s: build-owned gate.drive.start instruction lacks --run-id: %.160s", rel, p))
				}
			}
			if isScopedTaskStartSite(p) {
				taskSites[rel]++
				if carriesRunEpoch(p) {
					violations = append(violations, fmt.Sprintf(
						"%s: scoped task-owned start must not pass --run-id (the scope supplies it): %.160s", rel, p))
				}
			}
		}
	}

	// Population floors FIRST (a vacuous scan passes every negative).
	mirror := func(rel string) []string { return []string{rel, "internal/assets/embedded/tree/" + rel} }
	for _, rel := range append(mirror(buildSkillRel), mirror(implementNextSkillRel)...) {
		if prepSites[rel] == 0 {
			t.Errorf("coverage floor: %s contributes no gate.drive.prepare-scope site (scan or corpus drifted)", rel)
		}
		if buildSites[rel] == 0 {
			t.Errorf("coverage floor: %s contributes no build-owned gate.drive.start site (scan or corpus drifted)", rel)
		}
	}
	for _, rel := range mirror(buildSkillRel) {
		// The per-dispatch scope AND the WAITING-continuation re-prepare.
		if prepSites[rel] < 2 {
			t.Errorf("coverage floor: %s must carry both the per-dispatch and the continuation prepare-scope sites, found %d", rel, prepSites[rel])
		}
	}
	for _, rel := range append(mirror(buildSkillRel), mirror(buildTaskSkillRel)...) {
		// (D) The repair worker's build-owned re-run is handed the epoch.
		if repairSites[rel] == 0 {
			t.Errorf("coverage floor: %s carries no build-owned repair re-run start with --run-id site (the repair worker has no epoch source)", rel)
		}
	}
	for _, rel := range mirror(buildTaskSkillRel) {
		if taskSites[rel] == 0 {
			t.Errorf("coverage floor: %s contributes no scoped task-start site (scan or corpus drifted)", rel)
		}
	}
	if len(violations) != 0 {
		t.Errorf("run-epoch threading violations (%d):\n%s", len(violations), strings.Join(violations, "\n"))
	}

	t.Run("non_vacuity", func(t *testing.T) {
		prep := "run the `gate.drive.prepare-scope` operation with `--change-id <id> --worktree <w> --gate-context <g> --run-id <epoch> --json`"
		if !isPrepareScopeSite(prep) || !carriesRunEpoch(prep) {
			t.Fatalf("a complete prepare-scope invocation was misclassified")
		}
		if carriesRunEpoch(strings.Replace(prep, "--run-id <epoch> ", "", 1)) {
			t.Errorf("stripping --run-id from a prepare-scope invocation was not detected")
		}
		build := "the `gate.drive.start` operation with `--owner build --run-id <epoch> --json`"
		if !isBuildOwnerStartSite(build) || !carriesRunEpoch(build) {
			t.Fatalf("a complete build-owned start was misclassified")
		}
		if carriesRunEpoch(strings.Replace(build, "--run-id <epoch> ", "", 1)) {
			t.Errorf("stripping --run-id from a build-owned start was not detected")
		}
		if isBuildOwnerStartSite("the `gate.drive.start` operation with `--owner builder --json`") {
			t.Errorf("--owner build token boundary failed: 'builder' matched")
		}
		if isBuildOwnerStartSite("the `gate.drive.start` operation with `--owner task --json`") {
			t.Errorf("a task-owned start was classified as build-owned")
		}
		if carriesRunEpoch("pass `--run-id-x <x>`") {
			t.Errorf("--run-id token boundary failed: '--run-id-x' matched")
		}
		repair := "the repair worker's post-fix re-run is the `gate.drive.start` operation with `--owner build --run-id <epoch> --json`"
		if !isRepairRerunSite(repair) || !carriesRunEpoch(repair) {
			t.Fatalf("a complete repair re-run start was misclassified")
		}
		if carriesRunEpoch(strings.Replace(repair, "--run-id <epoch> ", "", 1)) {
			t.Errorf("stripping --run-id from the repair re-run start was not detected")
		}
		if isRepairRerunSite("the final suite gate is the `gate.drive.start` operation with `--owner build --json`, no failure to repair") {
			t.Errorf("a non-repair build-owned start that merely mentions repair was classified as the repair re-run")
		}
		if isRepairRerunSite("the post-fix re-run: the `gate.drive.start` operation with `--owner task --json`") {
			t.Errorf("a task-owned start was classified as the build-owned repair re-run")
		}
		task := "the `gate.drive.start` operation with `--owner task --scope-id <s> --child-cap <c> --run-id <e> --json`"
		if !isScopedTaskStartSite(task) || !carriesRunEpoch(task) {
			t.Errorf("a worker start that passes --run-id must be classified and flagged")
		}
		wrapped := "run `gate.drive.prepare-scope` again\nfor the same change (and `--run-id\n<epoch>`)"
		if got := paragraphs(wrapped); len(got) != 1 || !isPrepareScopeSite(got[0]) || !carriesRunEpoch(got[0]) {
			t.Errorf("whitespace collapse failed: a wrapped prepare-scope site did not match as one paragraph")
		}
	})
}

// runGateEpochCopyRe binds the copy instruction to the epoch AND to its
// destination with one bounded, sentence-local gap: a rewrite that keeps the
// word <epoch> elsewhere but drops "copy it into the dispatch prompt" reddens.
var runGateEpochCopyRe = regexp.MustCompile("copy the [^.]{0,60}`<epoch>` into the dispatch prompt")

// TestRunGateCopiesEpochIntoDispatchPrompt: the managed run-gate source, its
// embedded mirror, and the committed AGENTS.md rendering all tell the parent to
// copy the arm's <epoch> into the implement-next dispatch prompt (change 0467) —
// otherwise the epoch never reaches the build chain.
func TestRunGateCopiesEpochIntoDispatchPrompt(t *testing.T) {
	root := guardRoot(t)
	for _, rel := range []string{
		"cursor-rules/run-gate.md",
		"internal/assets/embedded/tree/cursor-rules/run-gate.md",
		"AGENTS.md",
	} {
		if !runGateEpochCopyRe.MatchString(collapseWS(readMaintained(t, root, rel))) {
			t.Errorf("%s: run-gate step 1 does not copy the `<epoch>` into the dispatch prompt", rel)
		}
	}

	t.Run("non_vacuity", func(t *testing.T) {
		good := "keep all three and copy the `<dispatch-context>` and the `<epoch>`\n   into the dispatch prompt."
		if !runGateEpochCopyRe.MatchString(collapseWS(good)) {
			t.Fatalf("the intended (wrapped) wording did not match")
		}
		old := "copy the `<dispatch-context>` into the dispatch prompt. The `<epoch>` is the run epoch id"
		if runGateEpochCopyRe.MatchString(collapseWS(old)) {
			t.Errorf("the pre-0467 wording (epoch not copied) matched")
		}
		if runGateEpochCopyRe.MatchString("copy the `<dispatch-context>`. Later, `<epoch>` into the dispatch prompt") {
			t.Errorf("bounded gap failed: the binding must not span sentences")
		}
	})
}

// codexRequestEpochRe binds the Codex agent.enter request-file sentence to the
// run epoch and its --run-id destination, sentence-local (a dot inside a token like change.claim is not a sentence end): agent.enter's own
// --run-id is lifecycle registration only and is never forwarded, so the
// request file is the epoch's only path to implement-next on that route (0467
// review fix-3).
var codexRequestEpochRe = regexp.MustCompile("Write a request file (?:[^.]|\\.\\S){0,400}run epoch(?:[^.]|\\.\\S){0,80}`--run-id`")

// TestCodexRequestFileCarriesRunEpoch: the generator source and the committed
// AGENTS.md rendering both tell the Codex agent.enter route to carry the run
// epoch in the request file.
func TestCodexRequestFileCarriesRunEpoch(t *testing.T) {
	root := guardRoot(t)
	for name, text := range map[string]string{
		"harness.CodexRootEntryClause": harness.CodexRootEntryClause,
		"AGENTS.md":                    readMaintained(t, root, "AGENTS.md"),
	} {
		if !codexRequestEpochRe.MatchString(collapseWS(text)) {
			t.Errorf("%s: the Codex agent.enter request file does not carry the run epoch for `--run-id`", name)
		}
	}

	t.Run("non_vacuity", func(t *testing.T) {
		good := "Write a request file containing the request unchanged; for implement-next include the unchanged gate dispatch-context token, labeled for `change.claim --run-context` and gate-drive `--gate-context`, and the unchanged run epoch, labeled for `--run-id`."
		if !codexRequestEpochRe.MatchString(good) {
			t.Fatalf("the intended wording did not match")
		}
		old := "Write a request file containing the request unchanged; for implement-next include the unchanged gate dispatch-context token, labeled for `change.claim --run-context` and gate-drive `--gate-context`. Preserve ids. Pass the run epoch to `--run-id`."
		if codexRequestEpochRe.MatchString(old) {
			t.Errorf("the pre-fix wording (epoch outside the request-file sentence) matched")
		}
	})
}
