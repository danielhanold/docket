package repoguard

// Change 0416: gate.drive.prepare-scope pins repository, worktree, change,
// task, phase, and branch identity, and the driver's scopeIdentityMatch
// rejects a scope-bound start whose identity does not match — so a maintained
// instruction telling a worker to start a task-owned drive with only
// --scope-id/--child-cap/--run-root manufactures a guaranteed pre-test halt
// (invalid-request, no drive). Three prongs:
//   (A) corpus scan: every scoped task-start instruction paragraph (syntactic
//       shape: a gate.drive.start reference plus --owner task in one collapsed
//       paragraph) carries the complete flag bundle, including the
//       --gate-context pass-through;
//   (B) the build controller's feature-dispatch payload block hands the worker
//       the complete start-ready scope bundle (and only the child capability);
//   (C) the shared caller contract's start row documents the identity flags.
// Site discovery is keyed on syntactic shape, never a per-file allowlist; the
// required-flag list is the asserted PROPERTY, not the discovery key.
// Residual risk, recorded not hidden: an author who instructs a task-owned
// start without the --owner task token in the same paragraph dodges prong A;
// the driver itself still fails such a start closed at run time
// (scope-identity-mismatch), and prongs B/C carry the contract prose.

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

const (
	buildSkillRel     = "skills/docket-build/SKILL.md"
	buildTaskSkillRel = "skills/docket-build-task/SKILL.md"
)

// requiredScopedStartFlags is the complete bundle a scoped task-owned
// gate.drive.start instruction must carry: the identity flags the prepared
// scope pinned, the scope-binding pair, the gate-context pass-through, and
// the existing transport flags.
var requiredScopedStartFlags = []string{
	"--repo-dir", "--change-id", "--task-id", "--phase", "--branch",
	"--scope-id", "--child-cap", "--gate-context", "--run-root", "--json",
}

// requiredStartRowFlags is what the shared contract's start row must document
// for a scope-bound start (transport flags are documented elsewhere in it).
var requiredStartRowFlags = []string{
	"--repo-dir", "--change-id", "--task-id", "--phase", "--branch",
	"--scope-id", "--child-cap", "--gate-context",
}

// requiredBundleElems is what the controller's dispatch payload must name for
// the worker (matched case-insensitively against the collapsed block).
var requiredBundleElems = []string{
	"feature worktree", "change id", "task id", "phase", "branch",
	"scope id", "child capability", "dispatch context",
}

var (
	startOpRe   = regexp.MustCompile(`gate\.drive\.start`)
	ownerTaskRe = regexp.MustCompile(`--owner task(?:[^a-z-]|$)`)
	// One bounded gap each (two stacked gaps backtrack catastrophically on
	// non-matching input); flag/element presence uses strings.Contains.
	bundleBindRe = regexp.MustCompile(`(?i)complete start-ready scope bundle`)
	childOnlyRe  = regexp.MustCompile(`(?i)child capability only`)
	parentStayRe = regexp.MustCompile(`(?i)parent capability[^.]{0,120}never enters any prompt`)
)

// isScopedTaskStartSite: a collapsed paragraph is a SITE when it references
// gate.drive.start AND carries the --owner task token.
func isScopedTaskStartSite(p string) bool {
	return startOpRe.MatchString(p) && ownerTaskRe.MatchString(p)
}

// missingScopedStartFlags returns the required flags the paragraph omits.
func missingScopedStartFlags(p string) []string {
	var missing []string
	for _, f := range requiredScopedStartFlags {
		if !strings.Contains(p, f) {
			missing = append(missing, f)
		}
	}
	return missing
}

func TestGateDriveScopedStartIdentity(t *testing.T) {
	root := guardRoot(t)

	// Prong A — corpus scan over maintained workflow markdown (isWorkflowMD
	// already includes the internal/assets/embedded/tree mirrors). The shared
	// caller contract is the requirement's definition, not a caller: prong C
	// covers it directly, mirroring TestGateDriveJSONCapture's exclusion.
	var violations []string
	perFile := map[string]int{}
	siteCount := 0
	for _, rel := range maintainedPop(t, root) {
		if !isWorkflowMD(rel) || strings.HasSuffix(rel, "docket-build/references/gate-caller-loop.md") {
			continue
		}
		for _, p := range paragraphs(readMaintained(t, root, rel)) {
			if !isScopedTaskStartSite(p) {
				continue
			}
			siteCount++
			perFile[rel]++
			if missing := missingScopedStartFlags(p); len(missing) != 0 {
				violations = append(violations, fmt.Sprintf(
					"%s: scoped task-start instruction missing %s: %.160s",
					rel, strings.Join(missing, " "), p))
			}
		}
	}

	// Population floors FIRST (a vacuous scan passes every negative): the
	// worker skill and its embedded mirror each contribute a site.
	if siteCount < 2 {
		t.Fatalf("population floor: only %d scoped task-start sites discovered (expected >= 2)", siteCount)
	}
	for _, rel := range []string{
		buildTaskSkillRel,
		"internal/assets/embedded/tree/" + buildTaskSkillRel,
	} {
		if perFile[rel] == 0 {
			t.Errorf("coverage floor: %s contributes no scoped task-start site (scan or corpus drifted)", rel)
		}
	}
	if len(violations) != 0 {
		t.Errorf("scoped task-start identity violations (%d):\n%s", len(violations), strings.Join(violations, "\n"))
	}

	// Prong B — the controller's dispatch payload hands the worker the
	// complete start-ready bundle. The embedded mirror is byte-identical by
	// the assets DiffTree check, so the source copy is the one asserted here.
	// Marker discipline: refuse dangling/duplicate/out-of-order markers.
	lines := strings.Split(readMaintained(t, root, buildSkillRel), "\n")
	startIdx, endIdx := -1, -1
	for i, line := range lines {
		if strings.HasPrefix(line, "<!-- docket:feature-dispatch:start") {
			if startIdx != -1 {
				t.Fatalf("%s: second feature-dispatch start marker at line %d — refusing to guess the block", buildSkillRel, i+1)
			}
			startIdx = i
		}
		if strings.HasPrefix(line, "<!-- docket:feature-dispatch:end") {
			if endIdx != -1 {
				t.Fatalf("%s: second feature-dispatch end marker at line %d", buildSkillRel, i+1)
			}
			endIdx = i
		}
	}
	if startIdx == -1 || endIdx == -1 || endIdx < startIdx {
		t.Fatalf("%s: feature-dispatch markers missing or out of order", buildSkillRel)
	}
	block := strings.ToLower(strings.Join(strings.Fields(strings.Join(lines[startIdx:endIdx+1], " ")), " "))
	if !bundleBindRe.MatchString(block) {
		t.Errorf("%s: dispatch payload lost its 'complete start-ready scope bundle' claim", buildSkillRel)
	}
	for _, elem := range requiredBundleElems {
		if !strings.Contains(block, elem) {
			t.Errorf("%s: dispatch payload bundle lost element %q", buildSkillRel, elem)
		}
	}
	if !childOnlyRe.MatchString(block) {
		t.Errorf("%s: dispatch payload lost the child-capability-only restriction", buildSkillRel)
	}
	if !parentStayRe.MatchString(block) {
		t.Errorf("%s: dispatch payload lost the parent-capability containment clause", buildSkillRel)
	}

	// Prong C — the shared contract's start row documents the identity flags
	// for a scope-bound start. The reference carries two tables with a
	// `| `start` |` row: the operation table (which documents the op's flags)
	// and the returns-summary table (whose start row never carries a flag
	// token). Discriminate the operation row by shape — it is the start row
	// that documents flags — rather than by table position or prose spelling.
	var startRows []string
	for _, line := range strings.Split(readMaintained(t, root, sharedContractRel), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "| `start` |") && strings.Contains(trimmed, "--") {
			startRows = append(startRows, line)
		}
	}
	if len(startRows) != 1 {
		t.Fatalf("%s: expected exactly one flag-documenting '| `start` |' operation row, found %d", sharedContractRel, len(startRows))
	}
	for _, f := range requiredStartRowFlags {
		if !strings.Contains(startRows[0], f) {
			t.Errorf("%s: start operation row does not document %s", sharedContractRel, f)
		}
	}

	t.Run("non_vacuity", func(t *testing.T) {
		full := "run the `gate.drive.start` operation with `--owner task --repo-dir <w> --change-id <id> --task-id <t> --phase build --branch <b> --scope-id <s> --child-cap <c> --gate-context <g> --run-root <r> --json -- <cmd>`"
		if !isScopedTaskStartSite(full) {
			t.Fatalf("a complete scoped task-start invocation was not classified as a site")
		}
		if missing := missingScopedStartFlags(full); len(missing) != 0 {
			t.Errorf("a complete invocation reported missing flags: %v", missing)
		}
		// MUTATION per protected field: removing each required flag must be
		// detected as exactly that missing flag.
		for _, f := range requiredScopedStartFlags {
			mutated := strings.Replace(full, f, "", 1)
			miss := missingScopedStartFlags(mutated)
			if len(miss) != 1 || miss[0] != f {
				t.Errorf("removing %s was not detected as exactly that missing flag: %v", f, miss)
			}
		}
		buildOwned := "the `gate.drive.start` operation with `--owner build --json` — capture the drive id and owner generation"
		if isScopedTaskStartSite(buildOwned) {
			t.Errorf("a build-owned start was wrongly classified as a scoped task-start site")
		}
		mention := "a scoped `gate.drive.start` binds the drive into the prepared recovery scope"
		if isScopedTaskStartSite(mention) {
			t.Errorf("a prose mention without --owner task was wrongly classified as a site")
		}
		if isScopedTaskStartSite("the `gate.drive.start` operation for the --owner taskforce") {
			t.Errorf("--owner task token boundary failed: 'taskforce' matched")
		}
		wrapped := "the `gate.drive.start` operation\nwith `--owner task\n--scope-id <id>`"
		if got := paragraphs(wrapped); len(got) != 1 || !isScopedTaskStartSite(got[0]) {
			t.Errorf("whitespace collapse failed: a wrapped scoped start was not one matchable paragraph")
		}
		if parentStayRe.MatchString("the parent capability is held. it never enters any prompt") {
			t.Errorf("bounded gap failed: parent-capability clause bind must not span sentences")
		}
	})
}
