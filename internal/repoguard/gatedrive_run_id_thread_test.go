package repoguard

// Change 0467 threaded run.start's run id into the build chain; change 0488
// retired its worker half (prepare-scope, task-owned starts, and the repair
// worker's re-run): build-task workers now run tests directly and call no gate
// operation, an absence TestNoTaskOwnedDriveInstructions guards. One prong
// remains over maintained workflow markdown (isWorkflowMD, so the embedded
// mirrors are scanned too): every build-owned gate.drive.start paragraph
// (--owner build) carries --run-id. run.cancel finds a run's suites by run id,
// so a build-owned start without it would survive a cancel (until change 0491
// retires the run id). Floors: docket-build carries both the final-gate start
// and the post-repair-attempt start the controller now makes itself, and
// docket-implement-next carries its own build-owned starts.
// TestRunTrackerCopiesRunIDIntoDispatchPrompt (below) binds the managed run-tracker
// source to copying the run into the dispatch prompt.
// Site discovery is keyed on syntactic shape, never a per-file allowlist; the
// shared caller contract (sharedContractRel) is the operation reference, not a
// caller, and is excluded.
// Residual risk, recorded not hidden: a build-owned start without the
// --owner build token in the same paragraph is not a site; at run time the
// driver still fences such a no-run-record start against a run-owned worktree
// (run-superseded).
// Change 0488 review: the same paragraph must also carry --change-id —
// GateDriveService.Start charges build.max_attempts only for a build-owned start
// with a non-empty ChangeID, and FindScopeDriveIDs matches drives on ChangeID, so
// a build-owned start without it is unbudgeted and invisible to run.verdict.

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/harness"
)

const implementNextSkillRel = "skills/docket-implement-next/SKILL.md"

var (
	ownerBuildRe = regexp.MustCompile(`--owner build(?:[^a-z-]|$)`)
	runIDFlagRe  = regexp.MustCompile(`--run-id(?:[^a-z-]|$)`)
	changeIDRe   = regexp.MustCompile(`--change-id(?:[^a-z-]|$)`)
)

// isBuildOwnerStartSite: a collapsed paragraph that references gate.drive.start
// AND carries the --owner build token.
func isBuildOwnerStartSite(p string) bool {
	return startOpRe.MatchString(p) && ownerBuildRe.MatchString(p)
}

// carriesRunID: the paragraph carries the --run-id flag token.
func carriesRunID(p string) bool { return runIDFlagRe.MatchString(p) }

func TestGateDriveRunIDThreaded(t *testing.T) {
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
			if !carriesRunID(p) {
				violations = append(violations, fmt.Sprintf(
					"%s: build-owned gate.drive.start instruction lacks --run-id: %.160s", rel, p))
			}
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
		t.Errorf("run-id threading violations (%d):\n%s", len(violations), strings.Join(violations, "\n"))
	}

	t.Run("non_vacuity", func(t *testing.T) {
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
		if changeIDRe.MatchString("`--owner build --change-idx 1`") || !changeIDRe.MatchString("`--change-id <id>`") {
			t.Errorf("--change-id token boundary failed")
		}
		if carriesRunID("pass `--run-id-x <x>`") {
			t.Errorf("--run-id token boundary failed: '--run-id-x' matched")
		}
		wrapped := "start the next attempt with the `gate.drive.start`\noperation with `--owner build\n--run-id <run-id>`"
		if got := paragraphs(wrapped); len(got) != 1 || !isBuildOwnerStartSite(got[0]) || !carriesRunID(got[0]) {
			t.Errorf("whitespace collapse failed: a wrapped build-owned start did not match as one paragraph")
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
// run id in the request file. The AGENTS.md rendering carries the Codex clause
// only when the committed agent_harnesses enables codex; the generator source
// is checked regardless.
func TestCodexRequestFileCarriesRunID(t *testing.T) {
	root := guardRoot(t)
	texts := map[string]string{
		"harness.CodexRootEntryClause": harness.CodexRootEntryClause,
	}
	if committedHarnesses(t)["codex"] {
		texts["AGENTS.md"] = readMaintained(t, root, "AGENTS.md")
	}
	for name, text := range texts {
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
