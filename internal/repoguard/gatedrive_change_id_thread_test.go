package repoguard

// Every build-owned `gate.drive.start` paragraph (`--owner build`) in maintained
// workflow markdown carries `--change-id` — `GateDriveService.Start` charges
// `build.max_attempts` only for a build-owned start with a non-empty ChangeID,
// and `FindScopeDriveIDs` matches drives on ChangeID, so a build-owned start
// without it is unbudgeted and invisible to run.verdict (change 0488 review).
// Change 0491 retired the run id; the retired-vocabulary seal keeps `--run-id`
// out. The scan covers maintained workflow markdown (isWorkflowMD, so the
// embedded mirrors are scanned too). Floors: docket-build carries both the
// final-gate start and the post-repair-attempt start the controller makes
// itself, and docket-implement-next carries its own build-owned starts.
// Site discovery is keyed on syntactic shape, never a per-file allowlist; the
// shared caller contract (sharedContractRel) is the operation reference, not a
// caller, and is excluded.

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

const implementNextSkillRel = "skills/docket-implement-next/SKILL.md"

var (
	ownerBuildRe = regexp.MustCompile(`--owner build(?:[^a-z-]|$)`)
	changeIDRe   = regexp.MustCompile(`--change-id(?:[^a-z-]|$)`)
)

// isBuildOwnerStartSite: a collapsed paragraph that references gate.drive.start
// AND carries the --owner build token.
func isBuildOwnerStartSite(p string) bool {
	return startOpRe.MatchString(p) && ownerBuildRe.MatchString(p)
}

func TestGateDriveBuildStartCarriesChangeID(t *testing.T) {
	root := guardRoot(t)
	var violations []string
	buildSites := map[string]int{}
	for _, rel := range maintainedPop(t, root) {
		if !isWorkflowMD(rel) || strings.HasSuffix(rel, sharedContractRel) {
			continue
		}
		for _, p := range paragraphs(readMaintained(t, root, rel)) {
			if !isBuildOwnerStartSite(p) {
				continue
			}
			buildSites[rel]++
			if !changeIDRe.MatchString(p) {
				violations = append(violations, fmt.Sprintf(
					"%s: build-owned gate.drive.start instruction lacks --change-id: %.160s", rel, p))
			}
		}
	}

	// Population floors FIRST (a vacuous scan passes every negative).
	mirror := func(rel string) []string { return []string{rel, "internal/assets/embedded/tree/" + rel} }
	for _, rel := range mirror(implementNextSkillRel) {
		if buildSites[rel] == 0 {
			t.Errorf("coverage floor: %s contributes no build-owned gate.drive.start site (scan or corpus drifted)", rel)
		}
	}
	for _, rel := range mirror(buildSkillRel) {
		// The final suite gate AND the post-repair attempt the controller starts itself.
		if buildSites[rel] < 2 {
			t.Errorf("coverage floor: %s must carry both the final-gate and the post-repair-attempt build-owned start sites, found %d", rel, buildSites[rel])
		}
	}
	if len(violations) != 0 {
		t.Errorf("change-id threading violations (%d):\n%s", len(violations), strings.Join(violations, "\n"))
	}

	t.Run("non_vacuity", func(t *testing.T) {
		build := "the `gate.drive.start` operation with `--owner build --change-id <id> --json`"
		if !isBuildOwnerStartSite(build) || !changeIDRe.MatchString(build) {
			t.Fatalf("a complete build-owned start was misclassified")
		}
		if changeIDRe.MatchString(strings.Replace(build, "--change-id <id> ", "", 1)) {
			t.Errorf("stripping --change-id from a build-owned start was not detected")
		}
		if isBuildOwnerStartSite("the `gate.drive.start` operation with `--owner builder --json`") {
			t.Errorf("--owner build token boundary failed: 'builder' matched")
		}
		if isBuildOwnerStartSite("the `gate.drive.start` operation with `--owner task --json`") {
			t.Errorf("a task-owned start was classified as build-owned")
		}
		if changeIDRe.MatchString("`--owner build --change-idx 1`") || !changeIDRe.MatchString("`--change-id <id>`") {
			t.Errorf("--change-id token boundary failed")
		}
		wrapped := "start the next attempt with the `gate.drive.start`\noperation with `--owner build\n--change-id <id>`"
		if got := paragraphs(wrapped); len(got) != 1 || !isBuildOwnerStartSite(got[0]) || !changeIDRe.MatchString(got[0]) {
			t.Errorf("whitespace collapse failed: a wrapped build-owned start did not match as one paragraph")
		}
	})
}
