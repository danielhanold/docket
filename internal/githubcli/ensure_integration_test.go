//go:build integration

package githubcli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// EnsurePullRequest drives every idempotent probe→act→verify path through the
// protocol-faithful fake gh. Each test asserts BOTH the returned disposition and
// the witness log — which gh calls happened and which did not — so a green
// result can never hide an unexercised or an extra external mutation (learnings:
// green-suite-untested-branch). The authored body must travel only on stdin.
//
// The ens* fixture constants (ensRepoSpec, ensHead, …) live in the untagged
// fixtures_test.go: the untagged probePRJSONWithDecision helper (probe_test.go)
// consumes them, so the default-tag build must see them — change 0333's partition.

func ensRequest() EnsurePullRequestRequest {
	return EnsurePullRequestRequest{
		Repository:   Repository{Host: "github.com", Owner: "acme", Name: "widget"},
		HeadBranch:   ensHead,
		ExpectedHead: ensHeadOid,
		BaseBranch:   ensBase,
		Title:        ensTitle,
		Body:         ensBody,
	}
}

// ensPRJSON renders one PR object in the exact nested shape `gh --json` emits.
func ensPRJSON(number int, state string, draft bool, head, oid, base, title, body string) string {
	m := map[string]any{
		"number":      number,
		"url":         fmt.Sprintf("https://github.com/acme/widget/pull/%d", number),
		"state":       state,
		"isDraft":     draft,
		"headRefName": head,
		"headRefOid":  oid,
		"baseRefName": base,
		"title":       title,
		"body":        body,
	}
	b, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// ensList wraps zero or more PR objects into the JSON array `gh pr list` emits.
func ensList(objs ...string) string {
	return "[" + strings.Join(objs, ",") + "]"
}

// ensMatchPR is the PR object that exactly satisfies ensRequest (open, ready,
// head oid, base, title, body all equal to the request).
func ensMatchPR(number int) string {
	return ensPRJSON(number, "OPEN", false, ensHead, ensHeadOid, ensBase, ensTitle, ensBody)
}

// arms keyed on the discriminating argv prefix. The probe list carries
// `--state all`; the post-mutation verify list carries `--state open`, so the
// stateless prefix matcher can serve the two head-branch queries different
// scripted responses.
func ensListAllArm(stdout string, exit int, stderr string) fakeArm {
	return fakeArm{ArgvPrefix: []string{"pr", "list", "--repo", ensRepoSpec, "--head", ensHead, "--state", "all"}, Stdout: stdout, Exit: exit, Stderr: stderr}
}

func ensListOpenArm(stdout string) fakeArm {
	return fakeArm{ArgvPrefix: []string{"pr", "list", "--repo", ensRepoSpec, "--head", ensHead, "--state", "open"}, Stdout: stdout, Exit: 0}
}

func ensCreateArm(stdout string, exit int, delayMs int) fakeArm {
	return fakeArm{ArgvPrefix: []string{"pr", "create", "--repo", ensRepoSpec}, Stdout: stdout, Exit: exit, DelayMs: delayMs}
}

func ensEditArm(stdout string, exit int) fakeArm {
	return fakeArm{ArgvPrefix: []string{"pr", "edit"}, Stdout: stdout, Exit: exit}
}

func ensViewArm(stdout string) fakeArm {
	return fakeArm{ArgvPrefix: []string{"pr", "view"}, Stdout: stdout, Exit: 0}
}

// mustDecodeOne decodes a single PR object so a test can obtain its canonical
// Version for an ExpectedVersion CAS assertion.
func mustDecodeOne(t *testing.T, obj string) PullRequest {
	t.Helper()
	prs, err := decodePullRequestList("test", []byte(ensList(obj)))
	if err != nil {
		t.Fatalf("decoding fixture PR: %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("fixture decoded to %d PRs, want 1", len(prs))
	}
	return prs[0]
}

// countArgv returns how many witnessed invocations begin with the given argv
// prefix.
func countArgv(recs []invocationRecord, prefix ...string) int {
	n := 0
	for _, r := range recs {
		if hasArgvPrefix(r.Argv, prefix) {
			n++
		}
	}
	return n
}

// --- (a) no PR -> exactly one create, body on stdin, verify runs, created ---

func TestIntegrationEnsureCreatesWhenNoPR(t *testing.T) {
	created := ensMatchPR(42)
	c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
		ensListAllArm(ensList(), 0, ""),                                // probe: no PR
		ensCreateArm("https://github.com/acme/widget/pull/42\n", 0, 0), // create
		ensListOpenArm(ensList(created)),                               // verify by head
		ensViewArm(created),                                            // verify by number
	}})
	res, err := c.EnsurePullRequest(context.Background(), ensRequest())
	if err != nil {
		t.Fatalf("EnsurePullRequest: %v", err)
	}
	if res.Disposition != EnsureCreated {
		t.Fatalf("disposition = %q, want %q", res.Disposition, EnsureCreated)
	}
	if res.PR.Number != 42 || res.PR.State != StateOpen || res.PR.Body != ensBody {
		t.Fatalf("verified snapshot mismatch: %+v", res.PR)
	}
	recs := log.records(t)
	if got := countArgv(recs, "pr", "create"); got != 1 {
		t.Fatalf("pr create count = %d, want exactly 1", got)
	}
	// Post-create verify queries ran (by head + by number).
	if got := countArgv(recs, "pr", "list", "--repo", ensRepoSpec, "--head", ensHead, "--state", "open"); got != 1 {
		t.Fatalf("verify list count = %d, want 1", got)
	}
	if got := countArgv(recs, "pr", "view"); got != 1 {
		t.Fatalf("verify view count = %d, want 1", got)
	}
	// Locate the create invocation and prove the create argv shape + body only on stdin.
	var create *invocationRecord
	for i := range recs {
		if hasArgvPrefix(recs[i].Argv, []string{"pr", "create"}) {
			create = &recs[i]
		}
	}
	if create == nil {
		t.Fatal("no create invocation witnessed")
	}
	for _, want := range []string{"--repo", ensRepoSpec, "--head", ensHead, "--base", ensBase, "--title", ensTitle, "--body-file", "-"} {
		if !argvContains(create.Argv, want) {
			t.Fatalf("create argv missing %q: %v", want, create.Argv)
		}
	}
	if create.Stdin != ensBody {
		t.Fatalf("create stdin = %q, want body %q", create.Stdin, ensBody)
	}
	for _, a := range create.Argv {
		if strings.Contains(a, "Body line one") {
			t.Fatalf("body leaked into create argv: %v", create.Argv)
		}
	}
}

func argvContains(argv []string, want string) bool {
	for _, a := range argv {
		if a == want {
			return true
		}
	}
	return false
}

// --- (b1) create response lost, same call: create exits nonzero, requery adopts the effect ---

func TestIntegrationEnsureCreateResponseLostSameCall(t *testing.T) {
	created := ensMatchPR(42)
	c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
		ensListAllArm(ensList(), 0, ""),
		ensCreateArm("", 1, 0), // create reports failure AFTER the PR landed
		ensListOpenArm(ensList(created)),
		ensViewArm(created),
	}})
	res, err := c.EnsurePullRequest(context.Background(), ensRequest())
	if err != nil {
		t.Fatalf("EnsurePullRequest: %v", err)
	}
	if res.Disposition != EnsureCreated {
		t.Fatalf("disposition = %q, want %q (postcondition established by requery)", res.Disposition, EnsureCreated)
	}
	if got := countArgv(log.records(t), "pr", "create"); got != 1 {
		t.Fatalf("pr create count = %d, want exactly 1 (never a second create)", got)
	}
}

// --- (b2) create response lost, retry call: a fresh Ensure sees the exact PR -> adopted, no create ---

func TestIntegrationEnsureAdoptsExistingExactMatch(t *testing.T) {
	existing := ensMatchPR(42)
	c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
		ensListAllArm(ensList(existing), 0, ""),
	}})
	req := ensRequest() // ExpectedVersion empty -> create-or-adopt face
	res, err := c.EnsurePullRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("EnsurePullRequest: %v", err)
	}
	if res.Disposition != EnsureAdopted {
		t.Fatalf("disposition = %q, want %q", res.Disposition, EnsureAdopted)
	}
	if res.PR.Number != 42 {
		t.Fatalf("adopted PR number = %d, want 42", res.PR.Number)
	}
	recs := log.records(t)
	if len(recs) != 1 {
		t.Fatalf("witness invocations = %d, want exactly 1 (probe only)", len(recs))
	}
	if got := countArgv(recs, "pr", "create"); got != 0 {
		t.Fatalf("pr create count = %d, want 0 (never duplicate a lost create)", got)
	}
}

// --- (c) exact open PR + supplied matching version -> unchanged, no mutation ---

func TestIntegrationEnsureUnchangedWithMatchingVersion(t *testing.T) {
	existing := ensMatchPR(42)
	pr := mustDecodeOne(t, existing)
	c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
		ensListAllArm(ensList(existing), 0, ""),
	}})
	req := ensRequest()
	req.ExpectedVersion = pr.Version
	res, err := c.EnsurePullRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("EnsurePullRequest: %v", err)
	}
	if res.Disposition != EnsureUnchanged {
		t.Fatalf("disposition = %q, want %q", res.Disposition, EnsureUnchanged)
	}
	recs := log.records(t)
	if got := countArgv(recs, "pr", "create") + countArgv(recs, "pr", "edit"); got != 0 {
		t.Fatalf("mutation calls = %d, want 0", got)
	}
	if len(recs) != 1 {
		t.Fatalf("witness invocations = %d, want 1", len(recs))
	}
}

// --- (d) one differing open PR: version CAS gate ---

func ensDifferingPR(number int) string {
	// Same head oid (so the wrong-head refusal does not fire first); differs in
	// title and body from the request.
	return ensPRJSON(number, "OPEN", false, ensHead, ensHeadOid, ensBase, "Old title", "old body")
}

func TestIntegrationEnsureContendedEmptyVersionOnDifferingPR(t *testing.T) {
	c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
		ensListAllArm(ensList(ensDifferingPR(7)), 0, ""),
	}})
	req := ensRequest() // empty version cannot authorize an edit
	res, err := c.EnsurePullRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("EnsurePullRequest: %v", err)
	}
	if res.Disposition != EnsureContended {
		t.Fatalf("disposition = %q, want %q", res.Disposition, EnsureContended)
	}
	if got := countArgv(log.records(t), "pr", "edit"); got != 0 {
		t.Fatalf("pr edit count = %d, want 0 (untouched)", got)
	}
}

func TestIntegrationEnsureContendedMismatchedVersionOnDifferingPR(t *testing.T) {
	c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
		ensListAllArm(ensList(ensDifferingPR(7)), 0, ""),
	}})
	req := ensRequest()
	req.ExpectedVersion = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	res, err := c.EnsurePullRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("EnsurePullRequest: %v", err)
	}
	if res.Disposition != EnsureContended {
		t.Fatalf("disposition = %q, want %q", res.Disposition, EnsureContended)
	}
	if got := countArgv(log.records(t), "pr", "edit"); got != 0 {
		t.Fatalf("pr edit count = %d, want 0 (body preserved)", got)
	}
}

func TestIntegrationEnsureUpdatesWithMatchingVersion(t *testing.T) {
	differing := ensDifferingPR(7)
	updated := ensMatchPR(7)
	pr := mustDecodeOne(t, differing)
	c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
		ensListAllArm(ensList(differing), 0, ""),
		ensEditArm("", 0),
		ensListOpenArm(ensList(updated)),
		ensViewArm(updated),
	}})
	req := ensRequest()
	req.ExpectedVersion = pr.Version
	res, err := c.EnsurePullRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("EnsurePullRequest: %v", err)
	}
	if res.Disposition != EnsureUpdated {
		t.Fatalf("disposition = %q, want %q", res.Disposition, EnsureUpdated)
	}
	if res.PR.Title != ensTitle || res.PR.Body != ensBody {
		t.Fatalf("verified snapshot not converged: %+v", res.PR)
	}
	recs := log.records(t)
	if got := countArgv(recs, "pr", "edit"); got != 1 {
		t.Fatalf("pr edit count = %d, want exactly 1", got)
	}
	// The edit carries the PR number, --repo, --title, --body-file - and body on stdin only.
	var edit *invocationRecord
	for i := range recs {
		if hasArgvPrefix(recs[i].Argv, []string{"pr", "edit"}) {
			edit = &recs[i]
		}
	}
	if edit == nil {
		t.Fatal("no edit invocation witnessed")
	}
	for _, want := range []string{"7", "--repo", ensRepoSpec, "--title", ensTitle, "--body-file", "-"} {
		if !argvContains(edit.Argv, want) {
			t.Fatalf("edit argv missing %q: %v", want, edit.Argv)
		}
	}
	if edit.Stdin != ensBody {
		t.Fatalf("edit stdin = %q, want body", edit.Stdin)
	}
	for _, a := range edit.Argv {
		if strings.Contains(a, "Body line one") {
			t.Fatalf("body leaked into edit argv: %v", edit.Argv)
		}
	}
}

// --- (e) concurrent change: edit lands but a human raced a different body -> contended, no rollback ---

func TestIntegrationEnsureConcurrentChangeContended(t *testing.T) {
	differing := ensDifferingPR(7)
	pr := mustDecodeOne(t, differing)
	// The post-edit verify observes a body a human raced in.
	raced := ensPRJSON(7, "OPEN", false, ensHead, ensHeadOid, ensBase, ensTitle, "human raced this body in")
	c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
		ensListAllArm(ensList(differing), 0, ""),
		ensEditArm("", 0),
		ensListOpenArm(ensList(raced)),
		ensViewArm(raced),
	}})
	req := ensRequest()
	req.ExpectedVersion = pr.Version
	res, err := c.EnsurePullRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("EnsurePullRequest: %v", err)
	}
	if res.Disposition != EnsureContended {
		t.Fatalf("disposition = %q, want %q (race reported honestly)", res.Disposition, EnsureContended)
	}
	recs := log.records(t)
	if got := countArgv(recs, "pr", "edit"); got != 1 {
		t.Fatalf("pr edit count = %d, want exactly 1 (no compensating rollback edit)", got)
	}
}

// --- (f) edit response lost: edit exits nonzero, requery shows it landed -> updated ---

func TestIntegrationEnsureEditResponseLostRecovered(t *testing.T) {
	differing := ensDifferingPR(7)
	updated := ensMatchPR(7)
	pr := mustDecodeOne(t, differing)
	c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
		ensListAllArm(ensList(differing), 0, ""),
		ensEditArm("", 1), // edit reports failure after the change landed
		ensListOpenArm(ensList(updated)),
		ensViewArm(updated),
	}})
	req := ensRequest()
	req.ExpectedVersion = pr.Version
	res, err := c.EnsurePullRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("EnsurePullRequest: %v", err)
	}
	if res.Disposition != EnsureUpdated {
		t.Fatalf("disposition = %q, want %q", res.Disposition, EnsureUpdated)
	}
	if got := countArgv(log.records(t), "pr", "edit"); got != 1 {
		t.Fatalf("pr edit count = %d, want exactly 1", got)
	}
}

// --- (g) blocks ---

func TestIntegrationEnsureBlocksTwoOpenPRs(t *testing.T) {
	c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
		ensListAllArm(ensList(ensMatchPR(7), ensMatchPR(8)), 0, ""),
	}})
	_, err := c.EnsurePullRequest(context.Background(), ensRequest())
	assertFailureKind(t, err, KindInvalidState)
	assertNoMutation(t, log)
}

func TestIntegrationEnsureBlocksTerminalSameHead(t *testing.T) {
	closed := ensPRJSON(7, "CLOSED", false, ensHead, ensHeadOid, ensBase, ensTitle, ensBody)
	c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
		ensListAllArm(ensList(closed), 0, ""),
	}})
	_, err := c.EnsurePullRequest(context.Background(), ensRequest())
	assertFailureKind(t, err, KindInvalidState)
	assertNoMutation(t, log)
}

func TestIntegrationEnsureBlocksDraft(t *testing.T) {
	draft := ensPRJSON(7, "OPEN", true, ensHead, ensHeadOid, ensBase, ensTitle, ensBody)
	c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
		ensListAllArm(ensList(draft), 0, ""),
	}})
	_, err := c.EnsurePullRequest(context.Background(), ensRequest())
	assertFailureKind(t, err, KindInvalidState)
	assertNoMutation(t, log)
}

func TestIntegrationEnsureBlocksWrongHead(t *testing.T) {
	wrong := ensPRJSON(7, "OPEN", false, ensHead, ensOtherOid, ensBase, ensTitle, ensBody)
	c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
		ensListAllArm(ensList(wrong), 0, ""),
	}})
	_, err := c.EnsurePullRequest(context.Background(), ensRequest())
	assertFailureKind(t, err, KindInvalidState)
	assertNoMutation(t, log)
}

// --- (h) decode hazards, auth failure redaction, timeout ---

func TestIntegrationEnsureBlocksMalformedJSON(t *testing.T) {
	c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
		ensListAllArm("{not json", 0, ""),
	}})
	_, err := c.EnsurePullRequest(context.Background(), ensRequest())
	assertFailureKind(t, err, KindInvalidOutput)
	assertNoMutation(t, log)
}

func TestIntegrationEnsureBlocksMissingField(t *testing.T) {
	// A PR object missing headRefOid is invalid-output, never zero-value data.
	missing := ensPRJSON(7, "OPEN", false, ensHead, "", ensBase, ensTitle, ensBody)
	c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
		ensListAllArm(ensList(missing), 0, ""),
	}})
	_, err := c.EnsurePullRequest(context.Background(), ensRequest())
	assertFailureKind(t, err, KindInvalidOutput)
	assertNoMutation(t, log)
}

func TestIntegrationEnsureBlocksUnknownEnum(t *testing.T) {
	weird := ensPRJSON(7, "WEIRD", false, ensHead, ensHeadOid, ensBase, ensTitle, ensBody)
	c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
		ensListAllArm(ensList(weird), 0, ""),
	}})
	_, err := c.EnsurePullRequest(context.Background(), ensRequest())
	assertFailureKind(t, err, KindInvalidState)
	assertNoMutation(t, log)
}

func TestIntegrationEnsureAuthFailureRedacted(t *testing.T) {
	secretStderr := "gh: authentication failed for host\n" +
		"token ghp_SECRETTOKENabc123 rejected\n" +
		"remote https://oauth2:ghp_SECRETTOKENabc123@github.example.com/acme/widget.git\n"
	c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
		ensListAllArm("", 4, secretStderr),
	}})
	_, err := c.EnsurePullRequest(context.Background(), ensRequest())
	f := assertFailureKind(t, err, KindExternal)
	if strings.Contains(f.Detail, "ghp_SECRETTOKENabc123") {
		t.Fatalf("token leaked into Detail: %q", f.Detail)
	}
	if strings.Contains(f.Detail, "github.example.com") {
		t.Fatalf("credentialed URL leaked into Detail: %q", f.Detail)
	}
}

func TestIntegrationEnsureTimeoutMidCreateUnknown(t *testing.T) {
	// Probe returns no PR; the create hangs past the network deadline; the
	// requery scenario is dead (no verify arm) so truth cannot be established.
	// The network timeout must comfortably exceed a race-instrumented subprocess
	// spawn so the FAST probe succeeds and only the delayed create times out.
	c, _ := newFakeClient(t,
		fakeScenario{Invocations: []fakeArm{
			ensListAllArm(ensList(), 0, ""),
			ensCreateArm("", 0, 5000), // delay far beyond the network timeout
		}},
		withNetworkTimeout(2*time.Second),
	)
	res, err := c.EnsurePullRequest(context.Background(), ensRequest())
	if err != nil {
		t.Fatalf("EnsurePullRequest returned error, want unknown disposition: %v", err)
	}
	if res.Disposition != EnsureUnknown {
		t.Fatalf("disposition = %q, want %q", res.Disposition, EnsureUnknown)
	}
}

// --- (i) every post-discovery invocation carries an explicit --repo ---

func TestIntegrationEnsureEveryPostDiscoveryCallHasRepo(t *testing.T) {
	// Drive the richest path (probe + edit + verify-by-head + verify-by-number)
	// so the log holds one of every gh subcommand this operation issues. The test
	// binary's own cwd is a different git checkout that would infer a different
	// repository, so an implicit-repo call would target the wrong repo — the
	// explicit --repo on every invocation is what prevents that.
	differing := ensDifferingPR(7)
	updated := ensMatchPR(7)
	pr := mustDecodeOne(t, differing)
	c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
		ensListAllArm(ensList(differing), 0, ""),
		ensEditArm("", 0),
		ensListOpenArm(ensList(updated)),
		ensViewArm(updated),
	}})
	req := ensRequest()
	req.ExpectedVersion = pr.Version
	if _, err := c.EnsurePullRequest(context.Background(), req); err != nil {
		t.Fatalf("EnsurePullRequest: %v", err)
	}
	recs := log.records(t)
	if len(recs) < 4 {
		t.Fatalf("expected at least 4 invocations (probe, edit, verify list, verify view), got %d", len(recs))
	}
	for i, r := range recs {
		if !argvContains(r.Argv, "--repo") {
			t.Fatalf("invocation %d lacks --repo: %v", i, r.Argv)
		}
		if !argvContains(r.Argv, ensRepoSpec) {
			t.Fatalf("invocation %d does not target %q: %v", i, ensRepoSpec, r.Argv)
		}
	}
}

// --- shared assertions ---

func assertFailureKind(t *testing.T, err error, want Kind) *Failure {
	t.Helper()
	if err == nil {
		t.Fatalf("expected a %q failure, got nil error", want)
	}
	f, ok := AsFailure(err)
	if !ok {
		t.Fatalf("error is not a *Failure: %v", err)
	}
	if f.Kind != want {
		t.Fatalf("failure kind = %q, want %q (detail %q)", f.Kind, want, f.Detail)
	}
	return f
}

func assertNoMutation(t *testing.T, log *witnessLog) {
	t.Helper()
	recs := log.records(t)
	if got := countArgv(recs, "pr", "create") + countArgv(recs, "pr", "edit"); got != 0 {
		t.Fatalf("mutation calls = %d, want 0", got)
	}
}
