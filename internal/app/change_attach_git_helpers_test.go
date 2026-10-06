package app

// Change 0465: helpers from change_attach_git_integration_test.go that default-build
// tests (change_implemented_test.go, claim_workflow_git_test.go) still use, kept
// untagged so every build compiles them.

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/repository"
)

// changeByPath rebuilds the snapshot from corpus and returns the change at path,
// so a test's golden ties to render.BacklinkContent rather than a hand-copied
// string.
func changeByPath(t *testing.T, pin StatusPin, corpus []StatusBlob, path string) domain.Change {
	t.Helper()
	inputs, _ := parseCorpus(corpus)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: pin.Config.Effective, Documents: inputs})
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}
	for _, c := range build.Snapshot.Changes() {
		if c.Path() == path {
			return c
		}
	}
	t.Fatalf("no change at %q in corpus", path)
	return domain.Change{}
}

// attachBacklinkBlock renders the docket:backlink block the operation expects at
// the head of the metadata-branch artifact at artifactPath, targeting change
// id/title at recPath: the link is recPath relative to the artifact's own
// directory (render.ArtifactBacklinkContent's shape). The relative path comes
// from the standard library's filepath.Rel, an oracle independent of the
// renderer, so a happy plan round-trips through verification.
func attachBacklinkBlock(id int, title, recPath, artifactPath string) string {
	rel, err := filepath.Rel(filepath.Dir(artifactPath), recPath)
	if err != nil {
		panic(fmt.Sprintf("attachBacklinkBlock: %v", err))
	}
	return "<!-- docket:backlink:start (generated — do not hand-edit) -->\n" +
		fmt.Sprintf("> ↩ **[Change %04d — %s](%s)**\n", id, title, filepath.ToSlash(rel)) +
		"<!-- docket:backlink:end -->\n"
}

// attachHappyPlan renders a well-formed plan artifact: the correct backlink plus
// an authored body whose sections merely MENTION planning tokens (change 0414
// acceptance — a plan that instructs about a token, and the human-approved
// ambiguous decision sentence, both attach; only a whole-slot bare-token filler
// refuses). Every slot here holds substantive content.
func attachHappyPlan(id int, title, recPath, planPath string) string {
	return attachBacklinkBlock(id, title, recPath, planPath) + "\n" + attachHappyPlanBody()
}

// attachHappyPlanBody is attachHappyPlan's authored body without the backlink
// block — the Markdown change.attach-plan receives; the operation renders and
// prepends the backlink itself.
func attachHappyPlanBody() string {
	return "# Implementation Plan\n\n## Task 1\n\nRemove the " + tok("todo") +
		" in retry.go and replace it with bounded retry logic.\n\n" +
		"## Error handling\n" + tok("todo") + ": decide whether failed requests should retry or stop.\n"
}
