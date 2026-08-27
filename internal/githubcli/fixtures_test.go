package githubcli

// Shared PR fixtures for both the fast default-tag tests and the tagged
// integration corpus. They live in this UNTAGGED file (change 0333's partition)
// because untagged tests consume them: pr_test.go's decode tests use
// samplePRJSON, and probe_test.go's probePRJSONWithDecision helper — reached from
// the untagged review-decision tests — uses the ens* constants. A symbol declared
// untagged is visible to the tagged build too, so the integration tests
// (ensure/merge/harness) that also read these keep seeing them unchanged.

// samplePRJSON is the canonical nested shape `gh pr view --json ...` documents.
const samplePRJSON = `{
  "number": 7,
  "url": "https://github.com/acme/widget/pull/7",
  "state": "OPEN",
  "isDraft": false,
  "headRefName": "feat/x",
  "headRefOid": "1111111111111111111111111111111111111111",
  "baseRefName": "main",
  "title": "Add widget",
  "body": "Body text"
}`

const (
	ensRepoSpec = "github.com/acme/widget"
	ensHead     = "feat/x"
	ensBase     = "main"
	ensTitle    = "Add widget"
	ensBody     = "Body line one\nBody line two\n"
	ensHeadOid  = "1111111111111111111111111111111111111111"
	ensOtherOid = "2222222222222222222222222222222222222222"
)
