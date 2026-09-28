package repoguard

// Change 0467: the run epoch the gated parent's arm prints must reach every call
// that mints a recovery scope or starts a build-owned drive, while build-task
// workers never handle it — the driver hands a scoped start the epoch its scope
// pinned. Prongs over maintained workflow markdown (isWorkflowMD, so the
// embedded mirrors are scanned too):
//   (A) every paragraph referencing gate.drive.prepare-scope carries --run-epoch;
//   (B) every build-owned gate.drive.start paragraph (--owner build) carries
//       --run-epoch;
//   (C) no scoped task-owned start paragraph (--owner task) carries --run-epoch —
//       the worker passes none; the scope supplies it.
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
)

const implementNextSkillRel = "skills/docket-implement-next/SKILL.md"

var (
	prepareScopeOpRe = regexp.MustCompile(`gate\.drive\.prepare-scope`)
	ownerBuildRe     = regexp.MustCompile(`--owner build(?:[^a-z-]|$)`)
	runEpochFlagRe   = regexp.MustCompile(`--run-epoch(?:[^a-z-]|$)`)
)

// isPrepareScopeSite: a collapsed paragraph that references the
// gate.drive.prepare-scope operation.
func isPrepareScopeSite(p string) bool { return prepareScopeOpRe.MatchString(p) }

// isBuildOwnerStartSite: a collapsed paragraph that references gate.drive.start
// AND carries the --owner build token.
func isBuildOwnerStartSite(p string) bool {
	return startOpRe.MatchString(p) && ownerBuildRe.MatchString(p)
}

// carriesRunEpoch: the paragraph carries the --run-epoch flag token.
func carriesRunEpoch(p string) bool { return runEpochFlagRe.MatchString(p) }

func TestGateDriveRunEpochThreaded(t *testing.T) {
	root := guardRoot(t)
	var violations []string
	prepSites := map[string]int{}
	buildSites := map[string]int{}
	taskSites := map[string]int{}
	for _, rel := range maintainedPop(t, root) {
		if !isWorkflowMD(rel) || strings.HasSuffix(rel, sharedContractRel) {
			continue
		}
		for _, p := range paragraphs(readMaintained(t, root, rel)) {
			if isPrepareScopeSite(p) {
				prepSites[rel]++
				if !carriesRunEpoch(p) {
					violations = append(violations, fmt.Sprintf(
						"%s: gate.drive.prepare-scope instruction lacks --run-epoch: %.160s", rel, p))
				}
			}
			if isBuildOwnerStartSite(p) {
				buildSites[rel]++
				if !carriesRunEpoch(p) {
					violations = append(violations, fmt.Sprintf(
						"%s: build-owned gate.drive.start instruction lacks --run-epoch: %.160s", rel, p))
				}
			}
			if isScopedTaskStartSite(p) {
				taskSites[rel]++
				if carriesRunEpoch(p) {
					violations = append(violations, fmt.Sprintf(
						"%s: scoped task-owned start must not pass --run-epoch (the scope supplies it): %.160s", rel, p))
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
	for _, rel := range mirror(buildTaskSkillRel) {
		if taskSites[rel] == 0 {
			t.Errorf("coverage floor: %s contributes no scoped task-start site (scan or corpus drifted)", rel)
		}
	}
	if len(violations) != 0 {
		t.Errorf("run-epoch threading violations (%d):\n%s", len(violations), strings.Join(violations, "\n"))
	}

	t.Run("non_vacuity", func(t *testing.T) {
		prep := "run the `gate.drive.prepare-scope` operation with `--change-id <id> --worktree <w> --gate-context <g> --run-epoch <epoch> --json`"
		if !isPrepareScopeSite(prep) || !carriesRunEpoch(prep) {
			t.Fatalf("a complete prepare-scope invocation was misclassified")
		}
		if carriesRunEpoch(strings.Replace(prep, "--run-epoch <epoch> ", "", 1)) {
			t.Errorf("stripping --run-epoch from a prepare-scope invocation was not detected")
		}
		build := "the `gate.drive.start` operation with `--owner build --run-epoch <epoch> --json`"
		if !isBuildOwnerStartSite(build) || !carriesRunEpoch(build) {
			t.Fatalf("a complete build-owned start was misclassified")
		}
		if carriesRunEpoch(strings.Replace(build, "--run-epoch <epoch> ", "", 1)) {
			t.Errorf("stripping --run-epoch from a build-owned start was not detected")
		}
		if isBuildOwnerStartSite("the `gate.drive.start` operation with `--owner builder --json`") {
			t.Errorf("--owner build token boundary failed: 'builder' matched")
		}
		if isBuildOwnerStartSite("the `gate.drive.start` operation with `--owner task --json`") {
			t.Errorf("a task-owned start was classified as build-owned")
		}
		if carriesRunEpoch("pass `--run-epoch-id <x>`") {
			t.Errorf("--run-epoch token boundary failed: '--run-epoch-id' matched")
		}
		task := "the `gate.drive.start` operation with `--owner task --scope-id <s> --child-cap <c> --run-epoch <e> --json`"
		if !isScopedTaskStartSite(task) || !carriesRunEpoch(task) {
			t.Errorf("a worker start that passes --run-epoch must be classified and flagged")
		}
		wrapped := "run `gate.drive.prepare-scope` again\nfor the same change (and `--run-epoch\n<epoch>`)"
		if got := paragraphs(wrapped); len(got) != 1 || !isPrepareScopeSite(got[0]) || !carriesRunEpoch(got[0]) {
			t.Errorf("whitespace collapse failed: a wrapped prepare-scope site did not match as one paragraph")
		}
	})
}
