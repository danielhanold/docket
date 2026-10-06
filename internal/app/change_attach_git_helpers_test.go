package app

// Change 0465: helpers from change_attach_git_integration_test.go that default-build
// tests (change_implemented_test.go, claim_workflow_git_test.go) still use, kept
// untagged so every build compiles them.

import "fmt"

// attachBacklinkBlock renders the docket:backlink block the operation expects at
// the head of an artifact, targeting change id/title at recPath. It mirrors
// render.BacklinkContent's repo-relative shape exactly (no RepoWebURL is
// configured in these fixtures), so a happy plan round-trips through verification.
func attachBacklinkBlock(id int, title, recPath string) string {
	return "<!-- docket:backlink:start (generated — do not hand-edit) -->\n" +
		fmt.Sprintf("> ↩ **Change %04d — %s** — `%s`\n", id, title, recPath) +
		"<!-- docket:backlink:end -->\n"
}

// attachHappyPlan renders a well-formed plan artifact: the correct backlink plus
// an authored body whose sections merely MENTION planning tokens (change 0414
// acceptance — a plan that instructs about a token, and the human-approved
// ambiguous decision sentence, both attach; only a whole-slot bare-token filler
// refuses). Every slot here holds substantive content.
func attachHappyPlan(id int, title, recPath string) string {
	return attachBacklinkBlock(id, title, recPath) + "\n" + attachHappyPlanBody()
}

// attachHappyPlanBody is attachHappyPlan's authored body without the backlink
// block — the Markdown change.attach-plan receives; the operation renders and
// prepends the backlink itself.
func attachHappyPlanBody() string {
	return "# Implementation Plan\n\n## Task 1\n\nRemove the " + tok("todo") +
		" in retry.go and replace it with bounded retry logic.\n\n" +
		"## Error handling\n" + tok("todo") + ": decide whether failed requests should retry or stop.\n"
}
