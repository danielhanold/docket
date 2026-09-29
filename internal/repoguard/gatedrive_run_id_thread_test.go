package repoguard

// Change 0467: the run id that run.start prints must reach every call
// that mints a recovery scope or starts a build-owned drive, while a build-task
// worker's scoped starts never carry it — the driver hands a scoped start the
// run id its scope pinned. The one worker exception is the integration-repair
// worker's build-owned post-fix full-suite re-run: docket-build hands it the
// run id in the repair dispatch payload (0467 review fix-1). Prongs over maintained workflow markdown (isWorkflowMD, so the
// embedded mirrors are scanned too):
//   (A) every paragraph referencing gate.drive.prepare-scope carries --run-id;
//   (B) every build-owned gate.drive.start paragraph (--owner build) carries
//       --run-id;
//   (C) no scoped task-owned start paragraph (--owner task) carries --run-id —
//       the worker passes none; the scope supplies it;
//   (D) the repair worker's post-fix re-run is a build-owned start site in both
//       docket-build and docket-build-task (floor), so prong B binds it to
//       --run-id — the exception is pinned, and prong C stays unweakened.
// TestRunTrackerCopiesRunIDIntoDispatchPrompt (below) binds the managed run-tracker
// source to copying the run into the dispatch prompt.
// Site discovery is keyed on syntactic shape, never a per-file allowlist; the
// shared caller contract (sharedContractRel) is the operation reference, not a
// caller, and is excluded exactly as in TestGateDriveScopedStartIdentity.
// Residual risk, recorded not hidden: an instruction that names the operation
// without its `gate.drive.` prefix (a bare `prepare-scope`), or a build-owned
// start without the --owner build token in the same paragraph, is not a site;
// at run time the driver still fences such a no-run-record start against a
// run-owned worktree (stale-run-id).

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
	runIDFlagRe      = regexp.MustCompile(`--run-id(?:[^a-z-]|$)`)
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

// carriesRunID: the paragraph carries the --run-id flag token.
func carriesRunID(p string) bool { return runIDFlagRe.MatchString(p) }

func TestGateDriveRunIDThreaded(t *testing.T) {
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
				if !carriesRunID(p) {
					violations = append(violations, fmt.Sprintf(
						"%s: gate.drive.prepare-scope instruction lacks --run-id: %.160s", rel, p))
				}
			}
			if isBuildOwnerStartSite(p) {
				buildSites[rel]++
				if isRepairRerunSite(p) {
					repairSites[rel]++
				}
				if !carriesRunID(p) {
					violations = append(violations, fmt.Sprintf(
						"%s: build-owned gate.drive.start instruction lacks --run-id: %.160s", rel, p))
				}
			}
			if isScopedTaskStartSite(p) {
				taskSites[rel]++
				if carriesRunID(p) {
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
		// (D) The repair worker's build-owned re-run is handed the run id.
		if repairSites[rel] == 0 {
			t.Errorf("coverage floor: %s carries no build-owned repair re-run start with --run-id site (the repair worker has no run-id source)", rel)
		}
	}
	for _, rel := range mirror(buildTaskSkillRel) {
		if taskSites[rel] == 0 {
			t.Errorf("coverage floor: %s contributes no scoped task-start site (scan or corpus drifted)", rel)
		}
	}
	if len(violations) != 0 {
		t.Errorf("run-id threading violations (%d):\n%s", len(violations), strings.Join(violations, "\n"))
	}

	t.Run("non_vacuity", func(t *testing.T) {
		prep := "run the `gate.drive.prepare-scope` operation with `--change-id <id> --worktree <w> --run-context <g> --run-id <run-id> --json`"
		if !isPrepareScopeSite(prep) || !carriesRunID(prep) {
			t.Fatalf("a complete prepare-scope invocation was misclassified")
		}
		if carriesRunID(strings.Replace(prep, "--run-id <run-id> ", "", 1)) {
			t.Errorf("stripping --run-id from a prepare-scope invocation was not detected")
		}
		build := "the `gate.drive.start` operation with `--owner build --run-id <run-id> --json`"
		if !isBuildOwnerStartSite(build) || !carriesRunID(build) {
			t.Fatalf("a complete build-owned start was misclassified")
		}
		if carriesRunID(strings.Replace(build, "--run-id <run-id> ", "", 1)) {
			t.Errorf("stripping --run-id from a build-owned start was not detected")
		}
		if isBuildOwnerStartSite("the `gate.drive.start` operation with `--owner builder --json`") {
			t.Errorf("--owner build token boundary failed: 'builder' matched")
		}
		if isBuildOwnerStartSite("the `gate.drive.start` operation with `--owner task --json`") {
			t.Errorf("a task-owned start was classified as build-owned")
		}
		if carriesRunID("pass `--run-id-x <x>`") {
			t.Errorf("--run-id token boundary failed: '--run-id-x' matched")
		}
		repair := "the repair worker's post-fix re-run is the `gate.drive.start` operation with `--owner build --run-id <run-id> --json`"
		if !isRepairRerunSite(repair) || !carriesRunID(repair) {
			t.Fatalf("a complete repair re-run start was misclassified")
		}
		if carriesRunID(strings.Replace(repair, "--run-id <run-id> ", "", 1)) {
			t.Errorf("stripping --run-id from the repair re-run start was not detected")
		}
		if isRepairRerunSite("the final suite gate is the `gate.drive.start` operation with `--owner build --json`, no failure to repair") {
			t.Errorf("a non-repair build-owned start that merely mentions repair was classified as the repair re-run")
		}
		if isRepairRerunSite("the post-fix re-run: the `gate.drive.start` operation with `--owner task --json`") {
			t.Errorf("a task-owned start was classified as the build-owned repair re-run")
		}
		task := "the `gate.drive.start` operation with `--owner task --scope-id <s> --child-cap <c> --run-id <e> --json`"
		if !isScopedTaskStartSite(task) || !carriesRunID(task) {
			t.Errorf("a worker start that passes --run-id must be classified and flagged")
		}
		wrapped := "run `gate.drive.prepare-scope` again\nfor the same change (and `--run-id\n<run-id>`)"
		if got := paragraphs(wrapped); len(got) != 1 || !isPrepareScopeSite(got[0]) || !carriesRunID(got[0]) {
			t.Errorf("whitespace collapse failed: a wrapped prepare-scope site did not match as one paragraph")
		}
	})
}

// runTrackerRunIDCopyRe binds the copy instruction to the run id AND to its
// destination with one bounded, sentence-local gap: a rewrite that keeps the
// word <run-id> elsewhere but drops "copy it into the dispatch prompt" reddens.
var runTrackerRunIDCopyRe = regexp.MustCompile("copy the [^.]{0,60}`<run-id>` into the dispatch prompt")

// TestRunTrackerCopiesRunIDIntoDispatchPrompt: the managed run-tracker source, its
// embedded mirror, and the committed AGENTS.md rendering all tell the parent to
// copy run.start's <run-id> into the implement-next dispatch prompt (change 0467) —
// otherwise the run id never reaches the build chain.
func TestRunTrackerCopiesRunIDIntoDispatchPrompt(t *testing.T) {
	root := guardRoot(t)
	for _, rel := range []string{
		"cursor-rules/run-tracker.md",
		"internal/assets/embedded/tree/cursor-rules/run-tracker.md",
		"AGENTS.md",
	} {
		if !runTrackerRunIDCopyRe.MatchString(collapseWS(readMaintained(t, root, rel))) {
			t.Errorf("%s: run-tracker step 1 does not copy the `<run-id>` into the dispatch prompt", rel)
		}
	}

	t.Run("non_vacuity", func(t *testing.T) {
		good := "keep all three and copy the `<run-context>` and the `<run-id>`\n   into the dispatch prompt."
		if !runTrackerRunIDCopyRe.MatchString(collapseWS(good)) {
			t.Fatalf("the intended (wrapped) wording did not match")
		}
		old := "copy the `<run-context>` into the dispatch prompt. The `<run-id>` is the run id"
		if runTrackerRunIDCopyRe.MatchString(collapseWS(old)) {
			t.Errorf("the pre-0467 wording (run id not copied) matched")
		}
		if runTrackerRunIDCopyRe.MatchString("copy the `<run-context>`. Later, `<run-id>` into the dispatch prompt") {
			t.Errorf("bounded gap failed: the binding must not span sentences")
		}
	})
}

// codexRequestRunIDRe binds the Codex agent.enter request-file sentence to the
// run id and its --run-id destination, sentence-local (a dot inside a token like change.claim is not a sentence end): agent.enter's own
// --run-id is lifecycle registration only and is never forwarded, so the
// request file is the run id's only path to implement-next on that route (0467
// review fix-3).
var codexRequestRunIDRe = regexp.MustCompile("Write a request file (?:[^.]|\\.\\S){0,400}run id(?:[^.]|\\.\\S){0,80}`--run-id`")

// TestCodexRequestFileCarriesRunID: the generator source and the committed
// AGENTS.md rendering both tell the Codex agent.enter route to carry the run
// run id in the request file.
func TestCodexRequestFileCarriesRunID(t *testing.T) {
	root := guardRoot(t)
	for name, text := range map[string]string{
		"harness.CodexRootEntryClause": harness.CodexRootEntryClause,
		"AGENTS.md":                    readMaintained(t, root, "AGENTS.md"),
	} {
		if !codexRequestRunIDRe.MatchString(collapseWS(text)) {
			t.Errorf("%s: the Codex agent.enter request file does not carry the run id for `--run-id`", name)
		}
	}

	t.Run("non_vacuity", func(t *testing.T) {
		good := "Write a request file containing the request unchanged; for implement-next include the unchanged run-context token, labeled for `--run-context` on `change.claim` and the gate drive, and the unchanged run id, labeled for `--run-id`."
		if !codexRequestRunIDRe.MatchString(good) {
			t.Fatalf("the intended wording did not match")
		}
		old := "Write a request file containing the request unchanged; for implement-next include the unchanged run-context token, labeled for `--run-context` on `change.claim` and the gate drive. Preserve ids. Pass the run id to `--run-id`."
		if codexRequestRunIDRe.MatchString(old) {
			t.Errorf("the pre-fix wording (run id outside the request-file sentence) matched")
		}
	})
}
