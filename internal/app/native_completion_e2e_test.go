//go:build e2e

package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/codexcontract"
	"github.com/danielhanold/docket/internal/testsupport"
)

// Rehearsal uses the existing local Git/GitHub fixture. Only GitHub effects and
// model dispatch/review are simulated; gates, receipts, input checks, evidence,
// metadata transitions and publication verification execute the actual CLI.
func TestNativeCompletionE2E(t *testing.T) {
	_, gh := sharedBinaries(t)
	root, err := filepath.EvalSymlinks(testsupport.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	source, err := moduleRoot()
	if err != nil {
		t.Fatal(err)
	}
	commit := runGit(t, source, "rev-parse", "HEAD")
	binary := filepath.Join(root, "docket")
	cmd := exec.Command(goExecutable(), "build", "-ldflags", "-X github.com/danielhanold/docket/internal/buildinfo.Commit="+commit, "-o", binary, "./cmd/docket")
	cmd.Dir = source
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build candidate: %v %s", err, b)
	}
	m := planRepoModes()[0]
	const id = 4
	const slug = "native-child"
	recPath := groomPath(id, slug)
	child := strings.Replace(buildReadyChange(id, slug), "stacked_on:\n", "stacked_on: 3\n", 1)
	e := newImplEnv(t, m, binary, gh, map[string]string{groomPath(3, "parent"): buildReadyChange(3, "parent"), recPath: child})
	parent := e.implement(t, 3, "parent", "docs/superpowers/plans/parent.md", "Parent fixture")
	s := &e2eState{repo: e.repo, mode: m, node: e.node, wdeps: e.wdeps, id: id, slug: slug, recPath: recPath, docketBin: binary, env: e.env}
	call := func(args ...string) dkResult { return nativeCLI(t, s, args...) }
	repoCall := func(args ...string) dkResult { return call(append(args, "--repo-dir", s.repo.invocation)...) }
	ok := func(r dkResult) dkResult {
		t.Helper()
		if r.code != 0 || (r.result() != "applied" && r.result() != "no-op") {
			t.Fatalf("CLI refused: %s %s", r.stdout, r.stderr)
		}
		return r
	}
	pin := func(name string, v any) (string, string) {
		t.Helper()
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(root, name)
		if err := os.WriteFile(p, b, 0600); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(b)
		return p, hex.EncodeToString(sum[:])
	}
	arm := ok(repoCall("run", "gate-before", "implement-next"))
	if arm.doc["armed"] != true {
		t.Fatalf("arm: %s", arm.stdout)
	}
	key, epoch, contextToken := arm.str("key"), arm.str("epoch"), arm.str("dispatch_context")
	ok(repoCall("change", "claim", "--id", "4", "--version", s.ver(t), "--gate-context", contextToken))
	reconcile, _ := pin("reconcile.json", ChangeReconcileRequest{ID: id, Version: s.ver(t), ReconcileLogEntry: "Native deterministic rehearsal.\n"})
	ok(repoCall("change", "reconcile", "--input", reconcile))
	prepared := ok(repoCall("workspace", "prepare", "--id", "4", "--version", s.ver(t)))
	s.wp = prepared.str("path")
	branch := strings.TrimPrefix(prepared.str("feature_ref"), "refs/heads/")
	featureCall := func(args ...string) dkResult { return call(append(args, "--repo-dir", s.wp)...) }
	const plan = "docs/superpowers/plans/native-rehearsal.md"
	writeRepoFile(t, s.wp, plan, "# Native rehearsal plan\n\nAdd the child and retain its completed implementation during recovery.\n")
	ok(featureCall("artifact", "backlink", "--artifact", plan, "--change", recPath))
	runGit(t, s.wp, "add", plan)
	runGit(t, s.wp, "commit", "-m", "plan", "--trailer", "Docket-Plan-Path: "+plan)
	planHead := runGit(t, s.wp, "rev-parse", "HEAD")
	ok(repoCall("change", "attach-plan", "--id", "4", "--version", s.ver(t), "--path", plan, "--commit", planHead))
	// The retained worker checkpoint is a genuine Git commit. Native host behavior
	// is simulated; the same CLI input validator used at entry and active is real.
	runRoot := filepath.Join(root, "worker-runs")
	if err := os.Mkdir(runRoot, 0700); err != nil {
		t.Fatal(err)
	}
	grant := ok(featureCall("gate", "drive", "prepare-scope", "--worktree", s.wp, "--change-id", "4", "--task-id", "worker", "--phase", "build", "--branch", branch, "--run-root", runRoot, "--run-epoch", epoch, "--gate-context", contextToken))
	primary, err := filepath.EvalSymlinks(s.repo.invocation)
	if err != nil {
		t.Fatal(err)
	}
	a := codexcontract.Assignment{SchemaVersion: 1, ChangeID: id, Role: "docket-build-standard", Phase: "build", TaskID: "worker", Mode: "fresh", Primary: primary, Feature: s.wp, CommonDir: runGit(t, s.wp, "rev-parse", "--path-format=absolute", "--git-common-dir"), Branch: branch, EntryHEAD: planHead, MetadataRevision: runGit(t, s.repo.origin, "rev-parse", "docket"), ChangePath: recPath, DocketExecutable: binary, DocketCommit: commit, ReadRoots: []string{root, s.wp}, WritePaths: []string{"native.go"}, RunRoot: runRoot}
	ap, ah := pin("worker.json", a)
	witness := ok(repoCall("agent", "check-inputs", "--assignment", ap, "--sha256", ah, "--stage", "prepare"))
	var checked CheckInputsResult
	if err := json.Unmarshal([]byte(witness.stdout), &checked); err != nil {
		t.Fatal(err)
	}
	a.RootIdentity = &checked.Observation.RootIdentity
	ap, ah = pin("worker.json", a)
	payload := codexcontract.WorkerPayload{SchemaVersion: 1, Kind: "worker", AssignmentPath: ap, AssignmentSHA256: ah, EntryArgv: codexcontract.EntryCheckerArgv(a, ap, ah), TaskText: "Add native.go", ScopeID: grant.str("scope_id"), ChildCapability: grant.str("child_capability"), RunEpochID: epoch, GateContext: contextToken}
	pp, ph := pin("worker-payload.json", payload)
	input := func(stage string) dkResult {
		return repoCall("agent", "check-inputs", "--assignment", ap, "--sha256", ah, "--payload", pp, "--payload-sha256", ph, "--stage", stage)
	}
	ok(input("dispatch"))
	ok(input("entry"))
	writeRepoFile(t, s.wp, "native.go", "package native\n")
	worker := ok(featureCall("gate", "drive", "start", "--owner", "build", "--run-root", runRoot, "--cwd", s.wp, "--change-id", "4", "--task-id", "worker", "--phase", "build", "--branch", branch, "--scope-id", grant.str("scope_id"), "--child-cap", grant.str("child_capability"), "--run-epoch", epoch, "--gate-context", contextToken))
	wd := nativeDrive(t, worker)
	runGit(t, s.wp, "add", "native.go")
	runGit(t, s.wp, "commit", "-m", "worker checkpoint")
	workerHead := runGit(t, s.wp, "rev-parse", "HEAD")
	ok(input("active"))
	ok(featureCall("gate", "drive", "acknowledge", "--scope-id", grant.str("scope_id"), "--child-cap", grant.str("child_capability"), "--drive-id", wd["drive_id"].(string), "--owner-gen", wd["generation"].(string)))
	if r := input("active"); r.result() == "applied" {
		t.Fatal("closed worker scope remained active")
	}
	// Certification uses fresh scopes at current HEAD; one recovery path below
	// traverses the actual keyed verdict and facade claim without seeded records.
	certify := func(name string, recover bool) string {
		t.Helper()
		rr := filepath.Join(root, name+"-runs")
		if err := os.Mkdir(rr, 0700); err != nil {
			t.Fatal(err)
		}
		g := ok(featureCall("gate", "drive", "prepare-scope", "--worktree", s.wp, "--change-id", "4", "--task-id", name, "--phase", "final", "--branch", branch, "--run-root", rr, "--run-epoch", epoch, "--gate-context", contextToken))
		started := ok(featureCall("gate", "drive", "start", "--owner", "build", "--run-root", rr, "--cwd", s.wp, "--change-id", "4", "--task-id", name, "--phase", "final", "--branch", branch, "--scope-id", g.str("scope_id"), "--child-cap", g.str("child_capability"), "--run-epoch", epoch, "--gate-context", contextToken))
		d := nativeDrive(t, started)
		drive, owner := d["drive_id"].(string), d["generation"].(string)
		if recover {
			handoff := ok(featureCall("gate", "drive", "handoff", "--drive-id", drive, "--owner-gen", owner))
			nativeCheckReceipt(t, s, root, rr, "gate.drive.handoff", handoff, drive, "final")
			verdict := ok(repoCall("run", "gate-verdict", key))
			if verdict.str("decision") != "gate-continue" {
				t.Fatalf("not continuation: %s", verdict.stdout)
			}
			claimed := ok(repoCall("run", "gate-claim", key, verdict.str("continuation_id")))
			receipt := nativeCheckReceipt(t, s, root, rr, "run.gate-claim", claimed, drive, "final")
			if receipt["authority"] != "owner" || receipt["next_operation"] != "gate.drive.advance" {
				t.Fatal("claim did not select advance")
			}
			owner = receipt["generation"].(string)
			advanced := ok(featureCall("gate", "drive", "advance", "--drive-id", drive, "--owner-gen", owner))
			d = nativeDrive(t, advanced)
			if d["raw_run_dir"] != nativeDrive(t, started)["raw_run_dir"] {
				t.Fatal("recovered terminal relaunched")
			}
			closed := featureCall("gate", "drive", "acknowledge", "--scope-id", g.str("scope_id"), "--child-cap", g.str("child_capability"), "--drive-id", drive, "--owner-gen", owner)
			if closed.code == 0 {
				t.Fatal("recovered scope accepted normal acknowledgement")
			}
		} else {
			ok(featureCall("gate", "drive", "acknowledge", "--scope-id", g.str("scope_id"), "--child-cap", g.str("child_capability"), "--drive-id", drive, "--owner-gen", owner))
		}
		head := runGit(t, s.wp, "rev-parse", "HEAD")
		path := filepath.Join(root, name+"-evidence.md")
		ev := ok(repoCall("evidence", "record", "--id", "4", "--head", head, "--run", d["raw_run_dir"].(string), "--output", path))
		if ev.str("record_path") != path {
			t.Fatal("evidence not durable")
		}
		ok(call("evidence", "verify", "--record", path, "--head", head))
		return path
	}
	oldEvidence := certify("before-results", false)
	const results = "docs/results/native-rehearsal.md"
	writeRepoFile(t, s.wp, results, "# Native rehearsal — Results\n\n## Outcome\n\nRetained the planner and worker checkpoints; native host review is simulated in this deterministic fixture.\n")
	ok(featureCall("artifact", "backlink", "--artifact", results, "--change", recPath))
	runGit(t, s.wp, "add", results)
	runGit(t, s.wp, "commit", "-m", "results checkpoint")
	head := runGit(t, s.wp, "rev-parse", "HEAD")
	ok(repoCall("change", "attach-results", "--id", "4", "--version", s.ver(t), "--path", results, "--commit", head))
	if call("evidence", "verify", "--record", oldEvidence, "--head", head).result() == "applied" {
		t.Fatal("results commit did not invalidate evidence")
	}
	freshEvidence := certify("review", true)
	// Real review entry validates current head and durable evidence. The reviewer
	// model's no-findings disposition is simulated and never called native proof.
	a.Role, a.Phase, a.Mode = "docket-review-standard", "review", "review"
	a.TaskID = ""
	a.EntryHEAD = head
	a.MetadataRevision = runGit(t, s.repo.origin, "rev-parse", "docket")
	a.WritePaths = nil
	a.ReviewBase = parent.head
	a.ReviewHEAD = head
	a.BuildEvidence = "evidence"
	reviewEntry := func(evidencePath string) dkResult {
		// The review workflow verifies evidence content; check-inputs separately pins its bytes.
		verified := call("evidence", "verify", "--record", evidencePath, "--head", head)
		if verified.result() != "applied" {
			return verified
		}
		b, err := os.ReadFile(evidencePath)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(b)
		a.Resources = []codexcontract.Resource{{LogicalID: "evidence", Path: evidencePath, SHA256: hex.EncodeToString(sum[:]), Source: "evidence.record"}}
		ap, ah = pin("review.json", a)
		pp, ph = pin("review-payload.json", codexcontract.WorkerPayload{SchemaVersion: 1, Kind: "review", AssignmentPath: ap, AssignmentSHA256: ah, EntryArgv: codexcontract.EntryCheckerArgv(a, ap, ah), TaskText: "Review current head"})
		return input("entry")
	}
	if reviewEntry(oldEvidence).result() == "applied" {
		t.Fatal("native review entry accepted stale evidence")
	}
	ok(reviewEntry(freshEvidence))
	f, err := os.OpenFile(filepath.Join(s.wp, results), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString("\n## Review\n\nSimulated host reviewer returned no findings after successful real input validation.\n")
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	runGit(t, s.wp, "add", results)
	runGit(t, s.wp, "commit", "-m", "consolidate review results")
	finalEvidence := certify("final", false)
	head = runGit(t, s.wp, "rev-parse", "HEAD")
	if call("evidence", "verify", "--record", freshEvidence, "--head", head).result() == "applied" {
		t.Fatal("final results failed to invalidate review evidence")
	}
	ok(repoCall("workspace", "publish", "--id", "4", "--head", head))
	body, _ := pin("pr.json", map[string]string{"title": "Native completion rehearsal", "body": "Deterministic native completion rehearsal; host review simulated.\n"})
	pr := ok(repoCall("pr", "publish", "--id", "4", "--head", head, "--body", body, "--evidence", finalEvidence))
	if pr.str("base") != "feat/parent" || pr.str("head") != head {
		t.Fatalf("incorrect stacked PR: %s", pr.stdout)
	}
	ok(repoCall("change", "mark-implemented", "--id", "4", "--version", s.ver(t), "--head", head, "--pr", pr.str("reference"), "--evidence", finalEvidence))
	verified := ok(repoCall("run", "verify", "--id", "4"))
	if verified.str("verdict") != "run-complete" {
		t.Fatalf("incomplete recovered workflow: %s", verified.stdout)
	}
	terminal := ok(repoCall("run", "gate-verdict", key))
	if terminal.str("outcome") != "run-complete" {
		t.Fatalf("outer run incomplete: %s", terminal.stdout)
	}
	record, err := LoadGateRecord(s.repo.invocation, key)
	if err != nil {
		t.Fatal(err)
	}
	if record.Retry != RetryUnused {
		t.Fatalf("continuation consumed retry: %s", terminal.stdout)
	}
	if runGit(t, s.wp, "merge-base", planHead, head) != planHead || runGit(t, s.wp, "merge-base", workerHead, head) != workerHead {
		t.Fatal("completed checkpoints were discarded")
	}
}

func nativeCLI(t *testing.T, s *e2eState, args ...string) dkResult {
	t.Helper()
	cmd := exec.Command(s.docketBin, append([]string{"--json"}, args...)...)
	cmd.Env = s.env
	var out, diagnostic bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &diagnostic
	r := dkResult{}
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			r.code = ee.ExitCode()
		} else {
			t.Fatal(err)
		}
	}
	r.stdout, r.stderr = out.String(), diagnostic.String()
	if err := json.Unmarshal(out.Bytes(), &r.doc); err != nil {
		t.Fatalf("invalid CLI result: %v %s %s", err, r.stdout, r.stderr)
	}
	return r
}
func nativeDrive(t *testing.T, r dkResult) map[string]any {
	t.Helper()
	d, ok := r.doc["drive"].(map[string]any)
	if !ok {
		t.Fatalf("missing drive: %s", r.stdout)
	}
	if d["outcome"] != "PASSED" {
		t.Fatalf("fixture gate not passed: %s", r.stdout)
	}
	return d
}
func nativeCheckReceipt(t *testing.T, s *e2eState, control, root, operation string, r dkResult, drive, phase string) map[string]any {
	t.Helper()
	out := filepath.Join(control, "capture-out.json")
	stderr := filepath.Join(control, "capture-err")
	if err := os.WriteFile(out, []byte(r.stdout), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stderr, []byte(r.stderr), 0600); err != nil {
		t.Fatal(err)
	}
	checked := nativeCLI(t, s, "agent", "check-receipt", "--operation", operation, "--run-root", root, "--expected-drive-id", drive, "--expected-phase", phase, "--stdout", out, "--stderr", stderr, "--exit-code", strconv.Itoa(r.code))
	if checked.result() != "applied" {
		t.Fatalf("checked consumer: %s", checked.stdout)
	}
	receipt, ok := checked.doc["receipt"].(map[string]any)
	if !ok {
		t.Fatal("missing receipt")
	}
	return receipt
}

// A public start really yields WAITING before the process is released. Recovery
// then consumes the original execution; no fake driver or fabricated receipt is used.
func TestNativeCompletionE2EWaitingAndAbsentOwner(t *testing.T) {
	binary, gh := sharedBinaries(t)
	for _, outcome := range []string{"PASSED", "FAILED", "HALTED"} {
		t.Run(outcome, func(t *testing.T) {
			t.Parallel()
			nativeWaitingRecovery(t, binary, gh, outcome)
		})
	}
}

func nativeWaitingRecovery(t *testing.T, binary, gh, outcome string) {
	root, err := filepath.EvalSymlinks(testsupport.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	release, launches := filepath.Join(root, "release"), filepath.Join(root, "launches")
	// External markers do not alter the gate's checked worktree fingerprint.
	command := "printf x >> " + launches + "; while [ ! -f " + release + " ]; do sleep 0.05; done; touch " + filepath.Join(root, "completed")
	if outcome == "FAILED" {
		command += "; exit 7"
	}
	repo := newDocketModeRepo(t, map[string]string{".docket.yml": "integration_branch: main\nbuild:\n  test_command: '" + command + "'\n"}, nil)
	s := &e2eState{repo: repo, docketBin: binary, env: e2eEnv(filepath.Join(root, "gh-state"), root, gh, repo.origin, "unused", strings.Repeat("0", 40))}
	t.Cleanup(func() { _ = os.WriteFile(release, nil, 0600) })
	call := func(args ...string) dkResult {
		return nativeCLI(t, s, append(args, "--repo-dir", s.repo.invocation)...)
	}
	start := call("gate", "drive", "start", "--owner", "build", "--run-root", root, "--phase", "final")
	checked := nativeCheckReceipt(t, s, root, root, "gate.drive.start", start, "", "final")
	d := checked["drive"].(map[string]any)
	if d["outcome"] != "WAITING" || checked["next_operation"] != "gate.drive.advance" {
		t.Fatalf("real start did not yield WAITING: %s", start.stdout)
	}
	id, owner := d["drive_id"].(string), d["generation"].(string)
	handoff := call("gate", "drive", "handoff", "--drive-id", id, "--owner-gen", owner)
	h := nativeCheckReceipt(t, s, root, root, "gate.drive.handoff", handoff, id, "final")
	token := h["drive"].(map[string]any)["generation"].(string)
	if h["authority"] != "handoff" || token == owner {
		t.Fatal("handoff did not rotate authority")
	}
	if err := os.WriteFile(release, nil, 0600); err != nil {
		t.Fatal(err)
	}
	completed := false
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		if _, err := os.Stat(filepath.Join(root, "completed")); err == nil {
			completed = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !completed {
		t.Fatal("original process did not finish while handed off")
	}
	claim := call("gate", "drive", "claim", "--drive-id", id, "--handoff-id", token)
	c := nativeCheckReceipt(t, s, root, root, "gate.drive.claim", claim, id, "final")
	fresh := c["drive"].(map[string]any)["generation"].(string)
	if fresh == token || c["authority"] != "owner" || c["next_operation"] != "gate.drive.advance" {
		t.Fatal("claim lost fresh owner")
	}
	if outcome == "HALTED" {
		writeRepoFile(t, s.repo.invocation, "drift.txt", "intentional fingerprint mismatch\n")
	}
	advance := call("gate", "drive", "advance", "--drive-id", id, "--owner-gen", fresh)
	terminal := nativeCheckReceipt(t, s, root, root, "gate.drive.advance", advance, id, "final")
	ended := terminal["drive"].(map[string]any)
	if terminal["classification"] != outcome || ended["attempt"] != d["attempt"] || ended["deadline"] != d["deadline"] || (outcome == "PASSED" && ended["raw_run_dir"] == "") {
		t.Fatal("recovery lost original execution/budget")
	}
	for _, args := range [][]string{
		{"gate", "drive", "advance", "--drive-id", id, "--owner-gen", owner},
		{"gate", "drive", "claim", "--drive-id", id, "--handoff-id", token},
	} {
		r := call(args...)
		if r.code == 0 {
			t.Fatal("backend accepted stale authority")
		}
		refusal := nativeCheckReceipt(t, s, root, root, "gate.drive."+args[2], r, "", "final")
		if refusal["classification"] != "halt" || refusal["next_operation"] != nil {
			t.Fatal("refusal permits continuation")
		}
	}
	b, err := os.ReadFile(launches)
	if err != nil || string(b) != "x" {
		t.Fatalf("suite relaunched: %q %v", b, err)
	}
}
