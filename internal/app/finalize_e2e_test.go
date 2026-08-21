//go:build e2e

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/repository/transaction"
	"github.com/danielhanold/docket/internal/workspace"
)

// This file is Task 17: the hermetic, end-to-end finalize matrix. Unlike the
// unit and real-git integration tests for Tasks 6-16 — which drive the app
// entry points in-process — these tests build the real `./cmd/docket` binary to
// a temporary path and drive the WHOLE terminal half of the workflow purely
// through CLI argv against disposable bare-remote repositories with hermetically
// isolated configuration. They prove the spec's "End-to-end and mutation tests"
// section bullet-for-bullet: ordinary finalize to archive+cleanup in both
// repository modes, conflict/repair/sign-off, response-loss convergence, the
// stack outcomes, out-of-band merge recovery, halt/resume plus reclaim, the
// deferred-capability fence, and no dependence on a PATH `docket`.
//
// Isolation contract (per the Global Constraints and the spec):
//   - The binary is invoked by ABSOLUTE PATH; PATH never carries a `docket`.
//   - The global config layer is an empty XDG_CONFIG_HOME tempdir (no host
//     ~/.config leaks in); a repo-local `.docket.yml` supplies the rest.
//   - `DOCKET_SCRIPTS_DIR` is cleared so no legacy Bash facade can satisfy
//     anything behind the Go binary's back.
//   - The GitHub seam is a separately-compiled, stateful fake `gh` on the
//     subprocess PATH that speaks gh's documented `--json` nested shapes and
//     performs a REAL merge commit on the bare origin, so the merge/closeout
//     reachability proofs run against genuine Git objects.

// --- shared argv + fake-gh harness ---------------------------------------

// e2eState is one implemented-state repository plus everything an argv-driven
// finalize sequence needs: the invocation clone to operate on, the compiled
// docket binary, the subprocess environment (isolated global config + fake gh),
// and the verified facts the finalize operations certify against.
type e2eState struct {
	repo      *gitRepo
	mode      planRepoMode
	node      realNode
	wdeps     WorkspaceDeps
	id        int
	slug      string
	recPath   string
	planPath  string
	head      string
	prRef     string
	prNumber  int
	evidence  []byte
	wp        string
	docketBin string
	ghBin     string
	stateFile string
	xdgHome   string
	env       []string
}

// e2eXDGOnce isolates the in-process git client's global-config layer exactly
// once for the whole test process. planningDepsFor uses t.Setenv, which forbids
// t.Parallel; these hermetic e2e tests are independent and run in parallel, so
// they set XDG_CONFIG_HOME to an empty directory ONCE with os.Setenv instead.
// This is safe because the file is behind the `e2e` build tag and only the
// TestE2E* tests ever run under it.
var (
	e2eXDGOnce sync.Once
	e2eXDGDir  string
)

// e2eNode builds the production planning seams over dir without t.Setenv, so the
// calling test may run in parallel. It mirrors planningDepsFor otherwise.
func e2eNode(t *testing.T, dir string) realNode {
	t.Helper()
	e2eXDGOnce.Do(func() {
		d, err := os.MkdirTemp("", "docket-e2e-xdg-*")
		if err != nil {
			t.Fatalf("isolate global config: %v", err)
		}
		e2eXDGDir = d
		os.Setenv("XDG_CONFIG_HOME", d)
	})
	client, err := gitcli.NewClient()
	if err != nil {
		t.Fatalf("gitcli.NewClient: %v", err)
	}
	engine, err := transaction.NewEngine(client, testClock())
	if err != nil {
		t.Fatalf("transaction.NewEngine: %v", err)
	}
	return realNode{
		dir: dir,
		deps: PlanningDeps{
			Client: client,
			Engine: engine,
			Reader: NewGitStatusReader(client),
			Clock:  testClock(),
		},
	}
}

// dkResult is one CLI invocation's decoded protocol-v1 document plus the raw
// streams and exit code.
type dkResult struct {
	doc    map[string]any
	stdout string
	stderr string
	code   int
}

// result returns the protocol-v1 "result" token (e.g. "applied",
// "unsupported-config", "contended").
func (r dkResult) result() string {
	if r.doc == nil {
		return ""
	}
	s, _ := r.doc["result"].(string)
	return s
}

// str reads a top-level string field from the decoded document.
func (r dkResult) str(key string) string {
	s, _ := r.doc[key].(string)
	return s
}

// dk runs `docket --json <args...>` with the state's isolated environment and
// --repo-dir bound to the invocation clone, feeding stdin when provided, and
// decodes exactly one protocol-v1 JSON document from stdout.
func (s *e2eState) dk(t *testing.T, stdin string, args ...string) dkResult {
	t.Helper()
	full := append([]string{"--json"}, args...)
	full = append(full, "--repo-dir", s.repo.invocation)
	cmd := exec.Command(s.docketBin, full...)
	cmd.Env = s.env
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var out, errBuf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errBuf
	code := 0
	if err := cmd.Run(); err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("running docket %v: %v\nstderr: %s", args, err, errBuf.String())
		}
		code = ee.ExitCode()
	}
	res := dkResult{stdout: out.String(), stderr: errBuf.String(), code: code}
	if strings.TrimSpace(res.stdout) != "" {
		dec := json.NewDecoder(strings.NewReader(res.stdout))
		if err := dec.Decode(&res.doc); err != nil {
			t.Fatalf("docket %v: stdout not one JSON document: %v\nstdout: %q\nstderr: %q", args, err, res.stdout, res.stderr)
		}
		var second any
		if err := dec.Decode(&second); err != io.EOF {
			t.Fatalf("docket %v: stdout carries a second JSON value: %q", args, res.stdout)
		}
	}
	return res
}

// writeInput writes a request/report body into a temp file inside the invocation
// clone's parent and returns its absolute path (bounded request-file transport,
// never argv interpolation).
func (s *e2eState) writeInput(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write input %s: %v", name, err)
	}
	return p
}

// ver reads the change record's current entity version from the bare origin —
// the independent oracle each exact-version request submits.
func (s *e2eState) ver(t *testing.T) string {
	t.Helper()
	return runGit(t, s.repo.origin, "rev-parse", s.mode.branch+":"+s.recPath)
}

// sharedBinaries compiles ./cmd/docket and the fake gh EXACTLY ONCE per test
// process and hands every e2e test the same two absolute paths. Both binaries
// are pure functions of committed source, so building once is correct and keeps
// the suite inside its wall-clock budget (learning
// budget-headroom-is-spent-before-it-is-breached).
var (
	sharedBinOnce   sync.Once
	sharedDocketBin string
	sharedGHBin     string
	sharedBinErr    error
)

func sharedBinaries(t *testing.T) (docketBin, ghBin string) {
	t.Helper()
	sharedBinOnce.Do(func() {
		dir, err := os.MkdirTemp("", "docket-e2e-bin-*")
		if err != nil {
			sharedBinErr = err
			return
		}
		sharedDocketBin, sharedGHBin, sharedBinErr = buildE2EBinaries(dir)
	})
	if sharedBinErr != nil {
		t.Fatalf("build shared e2e binaries: %v", sharedBinErr)
	}
	return sharedDocketBin, sharedGHBin
}

// buildE2EBinaries compiles ./cmd/docket and the extended fake gh into dir.
func buildE2EBinaries(dir string) (docketBin, ghBin string, err error) {
	goBin := goExecutable()
	root, err := moduleRoot()
	if err != nil {
		return "", "", err
	}
	docketBin = filepath.Join(dir, "docket")
	cmd := exec.Command(goBin, "build", "-o", docketBin, "./cmd/docket")
	cmd.Dir = root
	if out, berr := cmd.CombinedOutput(); berr != nil {
		return "", "", fmt.Errorf("build ./cmd/docket: %v\n%s", berr, out)
	}
	ghDir := filepath.Join(dir, "fakegh")
	if err := os.MkdirAll(ghDir, 0o755); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(filepath.Join(ghDir, "go.mod"), []byte("module fakegh\n\ngo 1.26\n"), 0o644); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(filepath.Join(ghDir, "main.go"), []byte(finalizeFakeGHSource), 0o644); err != nil {
		return "", "", err
	}
	ghBin = filepath.Join(ghDir, "gh")
	cmd = exec.Command(goBin, "build", "-o", ghBin, ".")
	cmd.Dir = ghDir
	if out, berr := cmd.CombinedOutput(); berr != nil {
		return "", "", fmt.Errorf("build fake gh: %v\n%s", berr, out)
	}
	return docketBin, ghBin, nil
}

// moduleRoot returns the module root (two levels up from internal/app).
func moduleRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Clean(filepath.Join(wd, "..", "..")), nil
}

// goExecutable resolves the go toolchain path.
func goExecutable() string {
	if p, err := exec.LookPath("go"); err == nil {
		return p
	}
	return filepath.Join(runtime.GOROOT(), "bin", "go")
}

// reachImplemented drives a build-ready change to a verified `implemented` state
// through the app entry points (proven by TestClaimToImplementedWorkflow), then
// returns the state the argv finalize sequence resumes from. The GitHub seam is
// the SAME compiled fake gh the argv subprocess will use, sharing one state file,
// so the PR the setup creates is the PR finalize acts on.
func reachImplemented(t *testing.T, m planRepoMode, docketBin, ghBin string) *e2eState {
	t.Helper()
	const (
		id   = 3
		slug = "widget"
	)
	recPath := groomPath(id, slug)
	planPath := "docs/superpowers/plans/2026-08-17-widget-plan.md"
	env := newImplEnv(t, m, docketBin, ghBin, map[string]string{recPath: buildReadyChange(id, slug)})
	c := env.implement(t, id, slug, planPath, "Add the widget")
	return &e2eState{
		repo: env.repo, mode: m, node: env.node, wdeps: env.wdeps,
		id: id, slug: slug, recPath: recPath, planPath: planPath,
		head: c.head, prRef: c.prRef, prNumber: parsePRNum(c.prRef),
		evidence: c.evidence, wp: c.wp, docketBin: docketBin, ghBin: ghBin,
		stateFile: env.stateFile, xdgHome: env.xdgHome, env: env.env,
	}
}

// implEnv is the shared in-process setup — one bare-remote repo, the production
// planning/workspace/GitHub seams over the compiled fake gh, and the argv
// environment — that can carry one or several changes to `implemented` in the
// SAME repository (a stack needs more than one).
type implEnv struct {
	ctx       context.Context
	node      realNode
	wdeps     WorkspaceDeps
	gdeps     GitHubDeps
	repo      *gitRepo
	m         planRepoMode
	docketBin string
	ghBin     string
	stateFile string
	xdgHome   string
	env       []string
}

// implementedChange is one change carried to implemented.
type implementedChange struct {
	head     string
	prRef    string
	evidence []byte
	wp       string // the feature worktree path
}

// newImplEnv builds the repo with the given metadata records and wires the shared
// seams. The fake gh reads every PR head live from the origin, so the argv
// environment needs no per-change head pin.
func newImplEnv(t *testing.T, m planRepoMode, docketBin, ghBin string, records map[string]string) *implEnv {
	t.Helper()
	requireRealGit(t)
	repo := buildE2ERepo(t, m, records)
	node := e2eNode(t, repo.invocation)
	svc, err := workspace.NewService(node.deps.Client)
	if err != nil {
		t.Fatalf("workspace.NewService: %v", err)
	}
	stateFile := filepath.Join(t.TempDir(), "gh-state.json")
	xdgHome := t.TempDir()
	env := e2eEnv(stateFile, xdgHome, ghBin, repo.origin, "unused", "0000000000000000000000000000000000000000")
	ghClient, err := githubcli.NewClient(githubcli.WithExecutable(ghBin), githubcli.WithBaseEnvironment(env))
	if err != nil {
		t.Fatalf("githubcli.NewClient over fake gh: %v", err)
	}
	return &implEnv{
		ctx: context.Background(), node: node, wdeps: WorkspaceDeps{Service: svc},
		gdeps: GitHubDeps{Service: ghClient}, repo: repo, m: m,
		docketBin: docketBin, ghBin: ghBin, stateFile: stateFile, xdgHome: xdgHome, env: env,
	}
}

// implement carries one already-present build-ready record (which may declare a
// stacked_on parent) through claim, reconcile, prepare, plan, implement, gate,
// evidence, publish, PR create, and mark-implemented. A stacked child's workspace
// bases on its parent's branch and its PR base resolves to that branch, exactly
// as the effective-base resolver dictates.
func (e *implEnv) implement(t *testing.T, id int, slug, planPath, title string) implementedChange {
	t.Helper()
	recPath := groomPath(id, slug)
	ver := func() string { return blobVersionAt(t, e.repo.origin, e.m.branch, recPath) }

	// A stacked child's effective base resolves only inside the claim transaction
	// (ContextImplementation reads with empty branch facts), so the version comes
	// from the origin oracle and eligibility is proven by the claim itself.
	claim := ChangeClaim(e.ctx, e.node.deps, e.node.dir, ChangeClaimRequest{ID: id, Version: ver()})
	if claim.Result != ResultApplied {
		t.Fatalf("claim id %d = %q (findings %v)", id, claim.Result, claim.Findings)
	}
	rec := ChangeReconcile(e.ctx, e.node.deps, e.node.dir, ChangeReconcileRequest{
		ID: id, Version: ver(), ReconcileLogEntry: "Reconciled against current reality.\n",
	})
	if rec.Result != ResultApplied {
		t.Fatalf("reconcile id %d = %q (findings %v)", id, rec.Result, rec.Findings)
	}
	prep := WorkspacePrepare(e.ctx, e.node.deps, e.wdeps, e.node.dir, WorkspaceIDRequest{ID: id, Version: ver()})
	if prep.Result != ResultApplied {
		t.Fatalf("workspace prepare id %d = %q (reason %q msg %q)", id, prep.Result, prep.Reason, prep.Message)
	}
	wp := prep.Path

	writeRepoFile(t, wp, planPath, "# Implementation Plan\n\nConcrete steps here.\n")
	bl := ArtifactBacklink(e.ctx, e.node.deps, wp, ArtifactBacklinkRequest{ArtifactPath: planPath, ChangePath: recPath})
	if bl.Result != ResultApplied {
		t.Fatalf("artifact backlink id %d = %q (reason %q)", id, bl.Result, bl.Reason)
	}
	runGit(t, wp, "add", "-A")
	runGit(t, wp, "commit", "-q", "-m", "write plan", "--trailer", "Docket-Plan-Path: "+planPath)
	planHead := runGit(t, wp, "rev-parse", "HEAD")

	attach := ChangeAttachPlan(e.ctx, e.node.deps, e.wdeps, e.node.dir,
		ChangeAttachRequest{ID: id, Version: ver(), Path: planPath, Commit: planHead})
	if attach.Result != ResultApplied {
		t.Fatalf("attach plan id %d = %q (reason %q findings %v)", id, attach.Result, attach.Reason, attach.Findings)
	}

	writeRepoFile(t, wp, slug+".go", "package "+slug+"\n")
	runGit(t, wp, "add", "-A")
	runGit(t, wp, "commit", "-q", "-m", "implement "+slug)
	head := runGit(t, wp, "rev-parse", "HEAD")

	gateRoot := t.TempDir()
	launch := GateLaunch(gateRoot, wp, []string{passingGateScript(t)})
	if launch.Result != ResultApplied || launch.RunDir == "" {
		t.Fatalf("gate launch id %d = %q (reason %q)", id, launch.Result, launch.Reason)
	}
	pollGatePassed(t, launch.RunDir)

	evd := EvidenceRecord(e.ctx, e.node.deps, e.wdeps, e.node.dir, EvidenceRecordRequest{ID: id, RunDir: launch.RunDir, Head: head})
	if evd.Result != ResultApplied || evd.Block == "" {
		t.Fatalf("evidence record id %d = %q (reason %q)", id, evd.Result, evd.Reason)
	}
	evidenceBytes := []byte(evd.Block)

	pub := WorkspacePublish(e.ctx, e.node.deps, e.wdeps, e.node.dir, WorkspacePublishRequest{ID: id, Head: head})
	if pub.Result != ResultApplied {
		t.Fatalf("workspace publish id %d = %q (reason %q)", id, pub.Result, pub.Reason)
	}

	pr := PRPublish(e.ctx, e.node.deps, e.wdeps, e.gdeps, e.node.dir, PRPublishRequest{
		ID: id, Head: head, Title: title,
		Body: "Authored PR prose for " + slug + ".\n", EvidenceRecord: evidenceBytes,
	})
	if pr.Result != ResultApplied {
		t.Fatalf("pr publish id %d = %q (disposition %q reason %q)", id, pr.Result, pr.Disposition, pr.Reason)
	}

	mi := ChangeMarkImplemented(e.ctx, e.node.deps, e.wdeps, e.gdeps, e.node.dir, MarkImplementedRequest{
		ID: id, Version: ver(), Head: head, PR: pr.Reference, EvidenceRecord: evidenceBytes,
	})
	if mi.Result != ResultApplied {
		t.Fatalf("mark implemented id %d = %q (findings %v)", id, mi.Result, mi.Findings)
	}
	return implementedChange{head: head, prRef: pr.Reference, evidence: evidenceBytes, wp: wp}
}

// buildE2ERepo builds the mode's bare-remote topology with a trivially-passing
// resolved finalize.test_command (so a real rebase's local gate genuinely passes
// on the disposable fixture), plus the one change record on the metadata branch.
// It mirrors the workflow harness's buildConfiguredRepo but owns its own config so
// the terminal-path gate is deterministic.
func buildE2ERepo(t *testing.T, m planRepoMode, records map[string]string) *gitRepo {
	t.Helper()
	switch m.name {
	case "main":
		return newMainModeRepo(t, mergeMaps(map[string]string{
			".docket.yml": "metadata_branch: main\nfinalize:\n  test_command: 'exit 0'\n",
		}, records))
	case "docket":
		return newDocketModeRepo(t,
			map[string]string{".docket.yml": "metadata_branch: docket\nintegration_branch: main\nfinalize:\n  test_command: 'exit 0'\n"},
			records)
	default:
		t.Fatalf("unknown metadata mode %q", m.name)
		return nil
	}
}

// e2eEnv builds the isolated subprocess environment: an empty global-config XDG
// home, the fake gh's directory prepended to PATH (and its state/config env), and
// a cleared legacy-facade variable.
func e2eEnv(stateFile, xdgHome, ghBin, origin, headBranch, head string) []string {
	env := append([]string{}, os.Environ()...)
	env = scrub(env, "XDG_CONFIG_HOME", "DOCKET_SCRIPTS_DIR", "PATH", "GH_TOKEN", "GITHUB_TOKEN")
	env = append(env,
		"XDG_CONFIG_HOME="+xdgHome,
		"DOCKET_SCRIPTS_DIR=",
		"PATH="+filepath.Dir(ghBin)+string(os.PathListSeparator)+os.Getenv("PATH"),
		"FAKE_GH_STATE="+stateFile,
		"FAKE_GH_ORIGIN="+origin,
		"FAKE_GH_REPO_URL=https://github.com/acme/widget",
		"FAKE_GH_OWNER=acme",
		"FAKE_GH_NAME=widget",
		"FAKE_GH_HEAD_BRANCH="+headBranch,
		"FAKE_GH_HEAD="+head,
	)
	return env
}

// scrub removes any assignment to the named keys from env.
func scrub(env []string, keys ...string) []string {
	drop := map[string]bool{}
	for _, k := range keys {
		drop[k] = true
	}
	out := env[:0]
	for _, kv := range env {
		i := strings.IndexByte(kv, '=')
		if i > 0 && drop[kv[:i]] {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// featureBranchForHead finds the origin branch ref (other than the metadata /
// integration branches) whose tip equals head.
func featureBranchForHead(t *testing.T, origin, head string) string {
	t.Helper()
	out := runGit(t, origin, "for-each-ref", "--format=%(refname:short) %(objectname)", "refs/heads/")
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == head {
			return fields[0]
		}
	}
	t.Fatalf("no origin branch points at head %s:\n%s", head, out)
	return ""
}

// parsePRNum extracts the trailing "#<n>" number from a canonical PR reference.
func parsePRNum(ref string) int {
	i := strings.LastIndex(ref, "#")
	if i < 0 {
		return 0
	}
	n, _ := strconv.Atoi(ref[i+1:])
	return n
}

// --- TestE2EOrdinaryFinalize ----------------------------------------------

// TestE2EOrdinaryFinalize proves the local-gate finalize path in BOTH repository
// modes, driven purely through CLI argv: context finalize -> rebase (no-op, gate
// skipped by exact-head evidence) -> publish -> merge -> closeout -> cleanup, and
// asserts the change lands `done` (archived) with the merge verified against a
// real merge commit on the origin.
func TestE2EOrdinaryFinalize(t *testing.T) {
	t.Parallel()
	requireRealGit(t)
	docketBin, ghBin := sharedBinaries(t)

	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			s := reachImplemented(t, m, docketBin, ghBin)
			runOrdinaryFinalize(t, s)
		})
	}
}

func runOrdinaryFinalize(t *testing.T, s *e2eState) {
	t.Helper()
	head, version := rebaseAndPublish(t, s)

	// (5) Merge: the attended --id invocation supplies approval; the gate is
	// satisfied by the exact-head evidence now in the PR body. A REAL merge
	// commit lands on the origin base branch and is proven reachable.
	mg := s.dk(t, "", "finalize", "merge", "--id", strconv.Itoa(s.id), "--version", version, "--head", head)
	if mg.result() != "applied" {
		t.Fatalf("finalize merge = %q\n%s", mg.result(), mg.stdout)
	}

	// (6) Closeout: archive to done after the merge-commit reachability proof.
	co := s.dk(t, "", "finalize", "closeout", "--id", strconv.Itoa(s.id))
	if co.result() != "applied" {
		t.Fatalf("finalize closeout = %q\n%s", co.result(), co.stdout)
	}
	if disp := co.str("disposition"); disp != "done-archived" {
		t.Fatalf("closeout disposition = %q, want done-archived\n%s", disp, co.stdout)
	}

	// The change record is now under the dated archive tree, no longer active.
	if _, ok := originFile(t, s.repo.origin, s.mode.branch, s.recPath); ok {
		t.Errorf("archived change still present at the active path %q", s.recPath)
	}

	// (7) Cleanup: ownership-safe workspace + branch removal after terminal state.
	cl := s.dk(t, "", "finalize", "cleanup", "--id", strconv.Itoa(s.id))
	if cl.result() != "applied" {
		t.Fatalf("finalize cleanup = %q\n%s", cl.result(), cl.stdout)
	}
}

// rebaseAndPublish drives the shared context->rebase->publish preamble and
// returns the (possibly rewritten) head the merge must match and the current
// record version.
func rebaseAndPublish(t *testing.T, s *e2eState) (head, version string) {
	t.Helper()

	// (1) Authoritative finalize context: exactly one candidate, our change,
	// actionable (no skip reason), at the exact version the oracle reports.
	cx := s.dk(t, "", "context", "finalize", "--id", strconv.Itoa(s.id))
	if cx.result() != "applied" {
		t.Fatalf("context finalize = %q\n%s", cx.result(), cx.stdout)
	}
	cand := soleCandidate(t, cx)
	// require_pr_approval defaults on, and the finalize prober cannot read an open
	// PR's review decision, so the candidate bands as approval-required — but the
	// attended, explicitly-named finalize run overrides exactly that skip reason,
	// which the context surfaces as an override note. Any OTHER skip reason
	// (malformed, pr-closed, dependency-unmerged, ...) is a real blocker.
	if sr, _ := cand["skip_reason"].(string); sr != "" {
		if sr != "approval-required" {
			t.Fatalf("implemented change carried a blocking skip reason %q: %v", sr, cand)
		}
		if note, _ := cand["override_note"].(string); note == "" {
			t.Fatalf("approval-required candidate carried no explicit-id override note: %v", cand)
		}
	}
	version, _ = cand["version"].(string)
	if version != s.ver(t) {
		t.Fatalf("context version %q disagrees with origin oracle %q", version, s.ver(t))
	}

	// (2) No children to retarget for an ordinary change (descendants empty).

	// (3) Rebase onto the effective base. In docket mode the integration branch
	// never moved, so the rebase is a no-op and the gate is SKIPPED on the exact-
	// head PR evidence; in main mode the metadata transactions advanced the base,
	// so a real rewrite happens and the local gate genuinely runs and passes. Both
	// are valid ordinary outcomes; the subsequent steps thread the resulting head.
	rb := s.dk(t, "", "finalize", "rebase", "--id", strconv.Itoa(s.id), "--version", version, "--head", s.head)
	if rb.result() != "applied" && rb.result() != "no-op" {
		t.Fatalf("finalize rebase = %q\n%s", rb.result(), rb.stdout)
	}
	if disp := rb.str("disposition"); disp != "unchanged" && disp != "rebased" {
		t.Fatalf("ordinary rebase disposition = %q, want unchanged or rebased\n%s", disp, rb.stdout)
	}
	attempt := rb.str("attempt")
	if attempt == "" {
		t.Fatalf("rebase returned no attempt token: %s", rb.stdout)
	}
	head = rb.str("head")
	if head == "" {
		t.Fatalf("rebase returned no head: %s", rb.stdout)
	}
	// Evidence for publish is the gate's fresh block when the suite ran, else the
	// exact-head evidence already carried into the PR body on the skip path.
	evidence := string(s.evidence)
	if gate, _ := rb.doc["gate"].(map[string]any); gate != nil {
		if ev, _ := gate["evidence"].(string); ev != "" {
			evidence = ev
		}
	}

	// (4) Publish: push the (possibly rewritten) head under the receipt lease and
	// ensure the exact-head evidence block in the PR body.
	evPath := s.writeInput(t, "evidence.txt", evidence)
	pubRes := s.dk(t, "", "finalize", "publish", "--id", strconv.Itoa(s.id),
		"--attempt", attempt, "--head", head, "--evidence", evPath)
	if pubRes.result() != "applied" && pubRes.result() != "no-op" {
		t.Fatalf("finalize publish = %q\n%s", pubRes.result(), pubRes.stdout)
	}
	return head, s.ver(t)
}

// --- TestE2EConflictAndRepair ---------------------------------------------

// TestE2EConflictAndRepair drives the full conflict/repair terminal path through
// CLI argv: a base-conflicting rebase stops CONFLICTED; a verified resolver
// report continues it; the local suite is RED at the rebased head (repair work);
// the operator records a durable repair-needs-signoff halt; and after the repair
// makes the suite green, the retry re-gates, publishes, merges, and closes out.
func TestE2EConflictAndRepair(t *testing.T) {
	t.Parallel()
	requireRealGit(t)
	docketBin, ghBin := sharedBinaries(t)
	m := planRepoModes()[1] // docket mode: the integration branch is the code base.
	s := reachImplemented(t, m, docketBin, ghBin)

	// Reconfigure the local gate to a marker-driven suite (red until the repair
	// lands `.repaired`) and push a conflicting `widget.go` onto the integration
	// branch, so the rebase must stop on an add/add conflict.
	commitToOriginDefault(t, s.repo.origin, ".docket.yml",
		"metadata_branch: docket\nintegration_branch: main\nfinalize:\n  test_command: 'test -f .repaired'\n",
		"marker-driven gate")
	commitToOriginDefault(t, s.repo.origin, "widget.go", "package widget\n// upstream edit\n", "conflicting upstream change")

	version := s.ver(t)

	// (1) Rebase stops CONFLICTED on widget.go.
	rb := s.dk(t, "", "finalize", "rebase", "--id", strconv.Itoa(s.id), "--version", version, "--head", s.head)
	if rb.str("disposition") != "conflicted" {
		t.Fatalf("rebase disposition = %q, want conflicted\n%s", rb.str("disposition"), rb.stdout)
	}
	attempt := rb.str("attempt")
	unmerged := toStringSlice(rb.doc["unmerged_paths"])
	if len(unmerged) == 0 || !contains(unmerged, "widget.go") {
		t.Fatalf("rebase conflict did not report widget.go unmerged: %v", unmerged)
	}

	// (2) The resolver resolves the conflict in the workspace and reports it; the
	// continue stages exactly the reported paths and completes the rebase, whose
	// gate then runs RED (no `.repaired` yet) — repair work.
	writeRepoFile(t, s.wp, "widget.go", "package widget\n// resolved: upstream + feature\n")
	report := `{"change_id":` + strconv.Itoa(s.id) + `,"attempt":"` + attempt +
		`","disposition":"resolved","summary":"merged upstream and feature","touched_paths":["widget.go"],` +
		`"conflicted_paths":["widget.go"],"observed_head":"","observed_base":"","recommended_action":"continue"}`
	reportPath := s.writeInput(t, "resolver.json", report)
	cont := s.dk(t, "", "finalize", "rebase-continue", "--id", strconv.Itoa(s.id), "--attempt", attempt, "--input", reportPath)
	if cont.str("disposition") != "failed" {
		t.Fatalf("continue disposition = %q, want failed (red suite is repair work)\n%s", cont.str("disposition"), cont.stdout)
	}
	if cont.str("reason") != "gate-failed" {
		t.Fatalf("continue reason = %q, want gate-failed\n%s", cont.str("reason"), cont.stdout)
	}

	// (3) The operator records the authored repair behind a durable finalize-
	// blocked marker whose reason is repair-needs-signoff: an autonomous run stops
	// here for a human sign-off. The marker names the reason on the record.
	headAfterContinue := runGit(t, s.wp, "rev-parse", "HEAD")
	blockPath := s.writeInput(t, "block.json", `{"report":"the local suite is red at the rebased head; an authored repair awaits a human sign-off","remedy":"review the repair, then clear the block and merge"}`)
	blk := s.dk(t, "", "finalize", "block", "--id", strconv.Itoa(s.id), "--version", s.ver(t),
		"--pr-number", strconv.Itoa(s.prNumber), "--attempt", attempt, "--reason", "repair-needs-signoff",
		"--head", headAfterContinue, "--input", blockPath)
	if blk.result() != "applied" {
		t.Fatalf("repair sign-off block = %q\n%s", blk.result(), blk.stdout)
	}
	if rec, _ := originFile(t, s.repo.origin, s.mode.branch, s.recPath); !strings.Contains(rec, "repair-needs-signoff") {
		t.Errorf("the finalize-blocked marker did not record repair-needs-signoff:\n%s", rec)
	}

	// (4) Retry: the human signs off. The repair lands `.repaired` (making the
	// suite green) on the already-rebased head, then the local gate is re-run and
	// its exact-head green evidence recorded through the landed gate/evidence
	// seams (the skill's "gate -> evidence record" step).
	writeRepoFile(t, s.wp, ".repaired", "green\n")
	runGit(t, s.wp, "add", "-A")
	runGit(t, s.wp, "commit", "-q", "-m", "repair: make the suite green")
	repairHead := runGit(t, s.wp, "rev-parse", "HEAD")

	gateRoot := t.TempDir()
	launch := GateLaunch(gateRoot, s.wp, []string{"/bin/sh", "-c", "test -f .repaired"})
	if launch.Result != ResultApplied || launch.RunDir == "" {
		t.Fatalf("repair gate launch = %q (reason %q)", launch.Result, launch.Reason)
	}
	pollGatePassed(t, launch.RunDir)
	evd := EvidenceRecord(context.Background(), s.node.deps, s.wdeps, s.node.dir,
		EvidenceRecordRequest{ID: s.id, RunDir: launch.RunDir, Head: repairHead})
	if evd.Result != ResultApplied || evd.Block == "" {
		t.Fatalf("repair evidence record = %q (reason %q)", evd.Result, evd.Reason)
	}

	// (5) Publish the repaired head under the owned receipt, clear the block (the
	// sign-off), merge, and close out.
	evPath := s.writeInput(t, "repaired-ev.txt", evd.Block)
	pub := s.dk(t, "", "finalize", "publish", "--id", strconv.Itoa(s.id), "--attempt", attempt, "--head", repairHead, "--evidence", evPath)
	if pub.result() != "applied" && pub.result() != "no-op" {
		t.Fatalf("repair publish = %q\n%s", pub.result(), pub.stdout)
	}
	cb := s.dk(t, "", "finalize", "clear-block", "--id", strconv.Itoa(s.id), "--version", s.ver(t), "--head", repairHead, "--pr-number", strconv.Itoa(s.prNumber))
	if cb.result() != "applied" {
		t.Fatalf("clear-block after sign-off = %q\n%s", cb.result(), cb.stdout)
	}
	mg := s.dk(t, "", "finalize", "merge", "--id", strconv.Itoa(s.id), "--version", s.ver(t), "--head", repairHead)
	if mg.result() != "applied" {
		t.Fatalf("post-repair merge = %q\n%s", mg.result(), mg.stdout)
	}
	co := s.dk(t, "", "finalize", "closeout", "--id", strconv.Itoa(s.id))
	if co.str("disposition") != "done-archived" {
		t.Fatalf("post-repair closeout = %q\n%s", co.str("disposition"), co.stdout)
	}
}

// toStringSlice converts a decoded JSON array to a []string.
func toStringSlice(v any) []string {
	arr, _ := v.([]any)
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// --- TestE2EStack ---------------------------------------------------------

// TestE2EStack proves the stacked-change terminal outcomes through CLI argv: a
// root's `finalize merge` REFUSES while an unauthorized child PR is still open
// (open-children gate) and retains the child branch and its open PR; a child
// merged into its live parent branch closes out to `stacked-merged` (retained,
// not archived); and once the child is stacked-merged, the root's merge to the
// integration branch closes out carrying every descendant to `done`.
func TestE2EStack(t *testing.T) {
	t.Parallel()
	requireRealGit(t)
	docketBin, ghBin := sharedBinaries(t)
	m := planRepoModes()[1] // docket mode: integration branch never moves for the setup.

	const (
		rootID, rootSlug   = 3, "widget"
		childID, childSlug = 4, "gadget"
	)
	env := newImplEnv(t, m, docketBin, ghBin, map[string]string{
		groomPath(rootID, rootSlug):   buildReadyChange(rootID, rootSlug),
		groomPath(childID, childSlug): stackedBuildReadyChange(childID, childSlug, rootID),
	})
	root := env.implement(t, rootID, rootSlug, "docs/superpowers/plans/2026-08-17-widget-plan.md", "Add the widget")
	child := env.implement(t, childID, childSlug, "docs/superpowers/plans/2026-08-17-gadget-plan.md", "Add the gadget")

	s := &e2eState{
		repo: env.repo, mode: m, node: env.node, wdeps: env.wdeps,
		id: rootID, slug: rootSlug, recPath: groomPath(rootID, rootSlug),
		head: root.head, prRef: root.prRef, prNumber: parsePRNum(root.prRef),
		docketBin: docketBin, ghBin: ghBin, stateFile: env.stateFile, xdgHome: env.xdgHome, env: env.env,
	}
	childPRNum := parsePRNum(child.prRef)
	rootBranch := featureBranchForHead(t, s.repo.origin, root.head)
	childBranch := featureBranchForHead(t, s.repo.origin, child.head)

	// The root's finalize context surfaces the child as a stack descendant. (The
	// open-child PR set is populated only over an unbounded probe — an explicit
	// --id bounds probing to that record alone — so the authoritative open-child
	// gate is proven at merge time below.)
	cx := s.dk(t, "", "context", "finalize", "--id", strconv.Itoa(rootID))
	cand := soleCandidate(t, cx)
	descs, _ := cand["descendants"].([]any)
	if len(descs) != 1 {
		t.Fatalf("root context did not surface the child descendant: %v", cand["descendants"])
	}

	// (c) The root merge REFUSES while the child PR is open, closes no child PR,
	// and retains the child branch.
	rootVer := verStack(t, s, rootID, rootSlug)
	mgBlocked := s.dk(t, "", "finalize", "merge", "--id", strconv.Itoa(rootID), "--version", rootVer, "--head", root.head)
	if mgBlocked.result() == "applied" {
		t.Fatalf("root merge succeeded with an open unauthorized child\n%s", mgBlocked.stdout)
	}
	if r := mgBlocked.str("reason"); r != "open-children" {
		t.Fatalf("root-with-open-child merge reason = %q, want open-children\n%s", r, mgBlocked.stdout)
	}
	if st := fakeGHPRStateOf(t, s.stateFile, childPRNum); st != "OPEN" {
		t.Errorf("the blocked root merge changed the child PR state to %q; it must stay OPEN", st)
	}
	if !originHasBranch(t, s.repo.origin, childBranch) {
		t.Errorf("the blocked root merge dropped the child branch %q", childBranch)
	}

	// (a) The child merges into its live parent branch and closes out to
	// stacked-merged (retained, not archived).
	childVer := verStack(t, s, childID, childSlug)
	cm := s.dk(t, "", "finalize", "merge", "--id", strconv.Itoa(childID), "--version", childVer, "--head", child.head)
	if cm.result() != "applied" {
		t.Fatalf("child merge into parent branch = %q\n%s", cm.result(), cm.stdout)
	}
	cco := s.dk(t, "", "finalize", "closeout", "--id", strconv.Itoa(childID))
	if cco.str("disposition") != "stacked-merged" {
		t.Fatalf("child closeout disposition = %q, want stacked-merged\n%s", cco.str("disposition"), cco.stdout)
	}
	if _, ok := originFile(t, s.repo.origin, s.mode.branch, groomPath(childID, childSlug)); !ok {
		t.Errorf("a stacked-merged child was archived; it must be retained until the root closes")
	}
	if !originHasBranch(t, s.repo.origin, childBranch) {
		t.Errorf("a stacked-merged child dropped its branch %q", childBranch)
	}

	// (b) Root carry: the child's merge advanced the root branch, so sync the
	// root worktree to it, rebase/merge the root PR to the integration branch, and
	// close out — carrying every proven descendant to done in one archive.
	syncWorktree(t, root.wp, rootBranch, s.repo.origin)
	rootVer = verStack(t, s, rootID, rootSlug)
	rootHead := runGit(t, root.wp, "rev-parse", "HEAD")
	rbRoot := s.dk(t, "", "finalize", "rebase", "--id", strconv.Itoa(rootID), "--version", rootVer, "--head", rootHead)
	if rbRoot.result() != "applied" && rbRoot.result() != "no-op" {
		t.Fatalf("root rebase = %q\n%s", rbRoot.result(), rbRoot.stdout)
	}
	rootHead = rbRoot.str("head")
	evPath := s.writeInput(t, "root-ev.txt", rootEvidence(rbRoot, root.evidence))
	pubRoot := s.dk(t, "", "finalize", "publish", "--id", strconv.Itoa(rootID),
		"--attempt", rbRoot.str("attempt"), "--head", rootHead, "--evidence", evPath)
	if pubRoot.result() != "applied" && pubRoot.result() != "no-op" {
		t.Fatalf("root publish = %q\n%s", pubRoot.result(), pubRoot.stdout)
	}
	mgRoot := s.dk(t, "", "finalize", "merge", "--id", strconv.Itoa(rootID), "--version", verStack(t, s, rootID, rootSlug), "--head", rootHead)
	if mgRoot.result() != "applied" {
		t.Fatalf("root merge to integration = %q\n%s", mgRoot.result(), mgRoot.stdout)
	}
	coRoot := s.dk(t, "", "finalize", "closeout", "--id", strconv.Itoa(rootID))
	if d := coRoot.str("disposition"); d != "root-archived" && d != "done-archived" {
		t.Fatalf("root closeout disposition = %q, want root-archived\n%s", d, coRoot.stdout)
	}
	// Both root and child are archived (carried to done); neither remains active.
	if _, ok := originFile(t, s.repo.origin, s.mode.branch, groomPath(rootID, rootSlug)); ok {
		t.Errorf("root still active after root-carry closeout")
	}
	if _, ok := originFile(t, s.repo.origin, s.mode.branch, groomPath(childID, childSlug)); ok {
		t.Errorf("carried descendant still active after root-carry closeout")
	}
}

// stackedBuildReadyChange is a build-ready change stacked on parent.
func stackedBuildReadyChange(id int, slug string, parent int) string {
	return strings.Replace(buildReadyChange(id, slug), "stacked_on:\n", "stacked_on: "+strconv.Itoa(parent)+"\n", 1)
}

// verStack reads the current record version for a stack member.
func verStack(t *testing.T, s *e2eState, id int, slug string) string {
	t.Helper()
	return blobVersionAt(t, s.repo.origin, s.mode.branch, groomPath(id, slug))
}

// rootEvidence returns the gate's fresh evidence when the root rebase ran, else
// the original implemented-state evidence.
func rootEvidence(rb dkResult, fallback []byte) string {
	if gate, _ := rb.doc["gate"].(map[string]any); gate != nil {
		if ev, _ := gate["evidence"].(string); ev != "" {
			return ev
		}
	}
	return string(fallback)
}

// fakeGHPRStateOf reads the state of one PR (by number) from the fake gh state.
func fakeGHPRStateOf(t *testing.T, stateFile string, number int) string {
	t.Helper()
	raw, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatalf("read fake gh state: %v", err)
	}
	var list []struct {
		Number int    `json:"number"`
		State  string `json:"state"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatalf("decode fake gh state: %v", err)
	}
	for _, p := range list {
		if p.Number == number {
			return p.State
		}
	}
	return ""
}

// originHasBranch reports whether the origin carries the named branch ref.
func originHasBranch(t *testing.T, origin, branch string) bool {
	t.Helper()
	_, err := tryGit(origin, "rev-parse", "--verify", "refs/heads/"+branch)
	return err == nil
}

// syncWorktree fast-forwards a feature worktree to the current origin tip of its
// branch, simulating the operator picking up a child merge that advanced it.
func syncWorktree(t *testing.T, wp, branch, origin string) {
	t.Helper()
	runGit(t, wp, "fetch", "-q", "origin", branch)
	runGit(t, wp, "reset", "--hard", "-q", "FETCH_HEAD")
}

// --- TestE2EOutOfBandMergeRecovered ---------------------------------------

// TestE2EOutOfBandMergeRecovered proves a PR merged out of band (by a human, not
// by `finalize merge`) is recovered by `maintenance sweep`: the sweep finds the
// merged `implemented` change, closes it out, and archives it to done — without a
// second merge and without a false state.
func TestE2EOutOfBandMergeRecovered(t *testing.T) {
	t.Parallel()
	requireRealGit(t)
	docketBin, ghBin := sharedBinaries(t)
	m := planRepoModes()[1] // docket mode: no base move, so no rebase is required.
	s := reachImplemented(t, m, docketBin, ghBin)

	// A human merges the PR out of band: the fake gh lands a real merge commit on
	// the integration branch and records the merged facts.
	humanMergePR(t, s)
	if st := fakeGHPRState(t, s.stateFile); st != "MERGED" {
		t.Fatalf("out-of-band merge did not land: PR state = %q", st)
	}

	// The maintenance sweep recovers it: closeout to done-archived.
	sw := s.dk(t, "", "maintenance", "sweep")
	if sw.result() != "applied" {
		t.Fatalf("maintenance sweep = %q\n%s", sw.result(), sw.stdout)
	}
	if _, ok := originFile(t, s.repo.origin, s.mode.branch, s.recPath); ok {
		t.Errorf("out-of-band-merged change not archived by the sweep: still at %q", s.recPath)
	}
	if mergeCommits := countMergeCommits(t, s.repo.origin, "main"); mergeCommits != 1 {
		t.Errorf("origin integration branch carries %d merge commits, want exactly 1 (no duplicate merge)", mergeCommits)
	}
}

// humanMergePR runs the fake gh `pr merge` directly (out of band), simulating a
// human merging the PR through the GitHub UI.
func humanMergePR(t *testing.T, s *e2eState) {
	t.Helper()
	cmd := exec.Command(s.ghBin, "pr", "merge", strconv.Itoa(s.prNumber), "--merge")
	cmd.Env = s.env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("out-of-band merge via fake gh: %v\n%s", err, out)
	}
}

// countMergeCommits counts merge commits (two-parent) on branch at the origin.
func countMergeCommits(t *testing.T, origin, branch string) int {
	t.Helper()
	out := strings.TrimSpace(runGit(t, origin, "rev-list", "--merges", "--count", branch))
	n, _ := strconv.Atoi(out)
	return n
}

// --- TestE2EResponseLossConvergence ---------------------------------------

// TestE2EResponseLossConvergence injects a lost response at the single
// irreversible external boundary — the PR merge — where the effect LANDS on the
// origin but gh returns no usable response, then reruns the terminal sequence and
// asserts it converges: the merge is adopted (verified by an authoritative
// reprobe), never issued twice, and closeout still archives to a single true
// done state.
func TestE2EResponseLossConvergence(t *testing.T) {
	t.Parallel()
	requireRealGit(t)
	docketBin, ghBin := sharedBinaries(t)
	m := planRepoModes()[1] // docket mode: deterministic no-op rebase preamble.
	s := reachImplemented(t, m, docketBin, ghBin)

	head, version := rebaseAndPublish(t, s)

	// Merge with a lost response: the fake gh lands the merge commit on the origin
	// and then exits non-zero as if the response were lost. The adapter's
	// authoritative reprobe discovers the landed merge and converges to a verified
	// success rather than fabricating a failure.
	s.env = withFault(s.env, "merge", "loss")
	mg1 := s.dk(t, "", "finalize", "merge", "--id", strconv.Itoa(s.id), "--version", version, "--head", head)
	if mg1.result() != "applied" {
		t.Fatalf("merge under lost response = %q (want applied via reprobe convergence)\n%s", mg1.result(), mg1.stdout)
	}
	// The reprobe converged on a real, reachable merge commit. Capture the base
	// tip it landed on: the default all-enabled policy selects rebase, a
	// single-parent chain with ZERO merge commits, so "no duplicate merge" is
	// proven graph-shape-independently by the destination tip standing still on
	// the rerun — not by counting merge commits.
	landedMerge := assertMergeReachable(t, s, mg1)
	landedBaseTip := runGit(t, s.repo.origin, "rev-parse", "refs/heads/main")

	// A rerun with the fault cleared must NOT merge again: it converges to a
	// verified already-merged no-op.
	s.env = withFault(s.env, "", "")
	mg2 := s.dk(t, "", "finalize", "merge", "--id", strconv.Itoa(s.id), "--version", s.ver(t), "--head", head)
	if mg2.str("disposition") != "already-merged" {
		t.Fatalf("merge rerun disposition = %q (want already-merged)\n%s", mg2.str("disposition"), mg2.stdout)
	}
	if mc := mergeCommitField(t, mg2); mc != landedMerge {
		t.Fatalf("response-loss rerun reported a different merge commit %s, want the landed %s", mc, landedMerge)
	}
	if after := runGit(t, s.repo.origin, "rev-parse", "refs/heads/main"); after != landedBaseTip {
		t.Fatalf("response-loss rerun advanced the base tip %s -> %s: the merge was issued twice", landedBaseTip, after)
	}

	// Closeout converges to a single true done state.
	co := s.dk(t, "", "finalize", "closeout", "--id", strconv.Itoa(s.id))
	if co.result() != "applied" || co.str("disposition") != "done-archived" {
		t.Fatalf("closeout after response-loss = %q/%q\n%s", co.result(), co.str("disposition"), co.stdout)
	}
	// A closeout replay is an idempotent no-op keyed on the promised archive state.
	co2 := s.dk(t, "", "finalize", "closeout", "--id", strconv.Itoa(s.id))
	if co2.result() != "no-op" && co2.str("disposition") != "already" {
		t.Fatalf("closeout replay = %q/%q, want an idempotent no-op\n%s", co2.result(), co2.str("disposition"), co2.stdout)
	}
}

// withFault returns a copy of env with the fake gh fault verb/mode set (or
// cleared when verb is "").
func withFault(env []string, verb, mode string) []string {
	env = scrub(env, "FAKE_GH_FAULT_VERB", "FAKE_GH_FAULT")
	return append(env, "FAKE_GH_FAULT_VERB="+verb, "FAKE_GH_FAULT="+mode)
}

// --- TestE2EMergeMethodShapes ---------------------------------------------

// These four tests drive the full argv finalize preamble and then `finalize
// merge` under each merge-method policy the fake gh exposes through
// FAKE_GH_REPO_SETTINGS, proving that finalize selects the best permitted
// method (rebase -> merge commit -> squash), lands the REAL corresponding graph
// shape on the origin base, and verifies the merge WITHOUT any two-parent
// assumption: every reachability proof is `merge-base --is-ancestor
// <merge_commit> <freshly-read base tip>`, which holds for a single-parent
// rebase chain, a single-parent squash commit, and a two-parent merge commit
// alike. The all-false policy is refused `blocked / merge-method-unavailable`
// with the origin base tip and the PR state provably unmoved.

// withRepoSettings returns a copy of env with the fake gh repository merge-method
// settings JSON knob set (or cleared when body is ""). Absent, the fake enables
// all three methods, so the default finalize path selects rebase.
func withRepoSettings(env []string, body string) []string {
	env = scrub(env, "FAKE_GH_REPO_SETTINGS")
	if body == "" {
		return env
	}
	return append(env, "FAKE_GH_REPO_SETTINGS="+body)
}

// mergeCommitField reads the merge document's verified merge_commit oid, failing
// if the applied merge carried no reachable merge object.
func mergeCommitField(t *testing.T, mg dkResult) string {
	t.Helper()
	merge, _ := mg.doc["merge"].(map[string]any)
	if merge == nil {
		t.Fatalf("merge document carries no merge object:\n%s", mg.stdout)
	}
	mc, _ := merge["merge_commit"].(string)
	if mc == "" {
		t.Fatalf("merge document carries no merge_commit oid:\n%s", mg.stdout)
	}
	return mc
}

// assertMergeReachable proves the reported merge commit is reachable from the
// freshly-read origin base tip — the graph-shape-independent verification the
// spec mandates (no two-parent or head-equality requirement).
func assertMergeReachable(t *testing.T, s *e2eState, mg dkResult) string {
	t.Helper()
	mc := mergeCommitField(t, mg)
	baseTip := runGit(t, s.repo.origin, "rev-parse", "refs/heads/main")
	if _, err := tryGit(s.repo.origin, "merge-base", "--is-ancestor", mc, baseTip); err != nil {
		t.Errorf("merge commit %s not reachable from origin base tip %s: %v", mc, baseTip, err)
	}
	return mc
}

// TestE2EMergeSelectsRebaseShape: the default all-enabled policy selects rebase.
// The document reports method "rebase", the merge commit is reachable, and the
// origin base carries ZERO new merge commits — a single-parent chain — proving
// the shape POSITIVELY without the verification depending on it.
func TestE2EMergeSelectsRebaseShape(t *testing.T) {
	t.Parallel()
	requireRealGit(t)
	docketBin, ghBin := sharedBinaries(t)
	m := planRepoModes()[1] // docket mode: deterministic no-op rebase preamble.
	s := reachImplemented(t, m, docketBin, ghBin)

	head, version := rebaseAndPublish(t, s)
	mg := s.dk(t, "", "finalize", "merge", "--id", strconv.Itoa(s.id), "--version", version, "--head", head)
	if mg.result() != "applied" {
		t.Fatalf("finalize merge = %q\n%s", mg.result(), mg.stdout)
	}
	if method := mg.str("method"); method != "rebase" {
		t.Fatalf("selected method = %q, want rebase\n%s", method, mg.stdout)
	}
	assertMergeReachable(t, s, mg)
	if n := countMergeCommits(t, s.repo.origin, "main"); n != 0 {
		t.Errorf("rebase selection produced %d merge commits on the base, want 0 (single-parent chain)", n)
	}
}

// TestE2EMergeCommitShape: rebase disabled, merge+squash enabled selects the
// merge commit. The document reports method "merge", exactly one merge commit
// lands on the base, and it is reachable.
func TestE2EMergeCommitShape(t *testing.T) {
	t.Parallel()
	requireRealGit(t)
	docketBin, ghBin := sharedBinaries(t)
	m := planRepoModes()[1]
	s := reachImplemented(t, m, docketBin, ghBin)
	s.env = withRepoSettings(s.env, `{"allow_rebase_merge":false,"allow_merge_commit":true,"allow_squash_merge":true}`)

	head, version := rebaseAndPublish(t, s)
	mg := s.dk(t, "", "finalize", "merge", "--id", strconv.Itoa(s.id), "--version", version, "--head", head)
	if mg.result() != "applied" {
		t.Fatalf("finalize merge = %q\n%s", mg.result(), mg.stdout)
	}
	if method := mg.str("method"); method != "merge" {
		t.Fatalf("selected method = %q, want merge\n%s", method, mg.stdout)
	}
	assertMergeReachable(t, s, mg)
	if n := countMergeCommits(t, s.repo.origin, "main"); n != 1 {
		t.Errorf("merge-commit selection produced %d merge commits on the base, want exactly 1", n)
	}
}

// TestE2ESquashOnlyShape: squash-only policy selects the last-priority fallback.
// The document reports method "squash", the base tip is a single-parent commit
// (zero merge commits), it is NOT the original PR head, and it is reachable.
func TestE2ESquashOnlyShape(t *testing.T) {
	t.Parallel()
	requireRealGit(t)
	docketBin, ghBin := sharedBinaries(t)
	m := planRepoModes()[1]
	s := reachImplemented(t, m, docketBin, ghBin)
	s.env = withRepoSettings(s.env, `{"allow_rebase_merge":false,"allow_merge_commit":false,"allow_squash_merge":true}`)

	head, version := rebaseAndPublish(t, s)
	mg := s.dk(t, "", "finalize", "merge", "--id", strconv.Itoa(s.id), "--version", version, "--head", head)
	if mg.result() != "applied" {
		t.Fatalf("finalize merge = %q\n%s", mg.result(), mg.stdout)
	}
	if method := mg.str("method"); method != "squash" {
		t.Fatalf("selected method = %q, want squash\n%s", method, mg.stdout)
	}
	mc := assertMergeReachable(t, s, mg)
	if n := countMergeCommits(t, s.repo.origin, "main"); n != 0 {
		t.Errorf("squash selection produced %d merge commits on the base, want 0 (single-parent commit)", n)
	}
	if mc == head {
		t.Errorf("squash merge commit equals the original PR head %s; it must be a distinct commit", head)
	}
}

// TestE2EMergeMethodUnavailable: an all-false policy leaves no permitted method.
// finalize refuses `blocked / merge-method-unavailable` BEFORE any merge — the
// document carries no `method` key, the PR is still OPEN in the fake state, and
// the origin base tip is unmoved (zero merge commands proven by effect, not by
// the absence of logging).
func TestE2EMergeMethodUnavailable(t *testing.T) {
	t.Parallel()
	requireRealGit(t)
	docketBin, ghBin := sharedBinaries(t)
	m := planRepoModes()[1]
	s := reachImplemented(t, m, docketBin, ghBin)
	s.env = withRepoSettings(s.env, `{"allow_rebase_merge":false,"allow_merge_commit":false,"allow_squash_merge":false}`)

	head, version := rebaseAndPublish(t, s)
	baseBefore := runGit(t, s.repo.origin, "rev-parse", "refs/heads/main")

	mg := s.dk(t, "", "finalize", "merge", "--id", strconv.Itoa(s.id), "--version", version, "--head", head)
	if mg.result() != "blocked" {
		t.Fatalf("finalize merge under empty policy = %q, want blocked\n%s", mg.result(), mg.stdout)
	}
	if r := mg.str("reason"); r != "merge-method-unavailable" {
		t.Fatalf("blocked reason = %q, want merge-method-unavailable\n%s", r, mg.stdout)
	}
	if _, ok := mg.doc["method"]; ok {
		t.Errorf("a pre-effect refusal reported a merge method; the document must omit it:\n%s", mg.stdout)
	}
	if st := fakeGHPRState(t, s.stateFile); st != "OPEN" {
		t.Errorf("the refused merge changed the PR state to %q; it must stay OPEN", st)
	}
	if after := runGit(t, s.repo.origin, "rev-parse", "refs/heads/main"); after != baseBefore {
		t.Errorf("the refused merge moved the origin base tip: %s -> %s (a merge command ran)", baseBefore, after)
	}
}

// --- TestE2EHaltResumeAndReclaim ------------------------------------------

// TestE2EHaltResumeAndReclaim proves the two implementation-side recovery paths
// through CLI argv: a persistent `change halt` records a durable run-halted
// marker on an in-progress change and `change resume-halted --acknowledge-
// quiescent` clears it and refreshes the claim; and an expired, no-work in-
// progress change (claim past its lease TTL, no branch or workspace) is reclaimed
// back to proposed.
func TestE2EHaltResumeAndReclaim(t *testing.T) {
	t.Parallel()
	requireRealGit(t)
	docketBin, ghBin := sharedBinaries(t)

	// Two claimed, in-progress changes: id 3 exercises halt/resume; id 4 is left
	// to expire and be reclaimed. lease_ttl is 1h so the testClock claim stamp is
	// unambiguously expired against the binary's real system clock.
	s := reachInProgress(t, docketBin, ghBin)

	// --- halt then resume (id 3) ---
	reportPath := s.writeInput(t, "halt.json", `{"report":"paused: awaiting an upstream decision"}`)
	h := s.dk(t, "", "change", "halt", "--id", "3", "--version", verOf(t, s, 3), "--input", reportPath)
	if h.result() != "applied" {
		t.Fatalf("change halt = %q\n%s", h.result(), h.stdout)
	}
	// The durable run-halted marker is on the record and `run verify` reads it.
	rv := s.dk(t, "", "run", "verify", "--id", "3")
	if verdict, _ := rv.doc["verdict"].(string); verdict != "run-halted" {
		t.Fatalf("run verify after halt = %q, want run-halted\n%s", verdict, rv.stdout)
	}
	res := s.dk(t, "", "change", "resume-halted", "--id", "3", "--version", verOf(t, s, 3), "--acknowledge-quiescent")
	if res.result() != "applied" {
		t.Fatalf("change resume-halted = %q\n%s", res.result(), res.stdout)
	}
	// The marker is gone: a second resume finds nothing to resume.
	res2 := s.dk(t, "", "change", "resume-halted", "--id", "3", "--version", verOf(t, s, 3), "--acknowledge-quiescent")
	if res2.result() == "applied" {
		t.Fatalf("resume-halted replayed as a success though the marker was cleared\n%s", res2.stdout)
	}

	// --- expired no-work reclaim (id 4) ---
	rc := s.dk(t, "", "change", "reclaim", "--id", "4", "--version", verOf(t, s, 4))
	if rc.result() != "applied" {
		t.Fatalf("change reclaim of an expired no-work change = %q\n%s", rc.result(), rc.stdout)
	}
	// The reclaimed change is back to proposed (its claim released). The status
	// scalar may be quoted, so match either spelling.
	body, ok := originFile(t, s.repo.origin, s.mode.branch, groomPath(4, "gadget"))
	if !ok || (!strings.Contains(body, "status: proposed") && !strings.Contains(body, "status: 'proposed'")) {
		t.Errorf("reclaimed change 4 is not back to proposed:\n%s", body)
	}
}

// verOf reads the current record version for one id from the origin oracle.
func verOf(t *testing.T, s *e2eState, id int) string {
	t.Helper()
	slug := map[int]string{3: "widget", 4: "gadget"}[id]
	return blobVersionAt(t, s.repo.origin, s.mode.branch, groomPath(id, slug))
}

// reachInProgress builds a main-mode repo with two claimed (in-progress) changes
// and a short reclaim lease TTL, returning the argv harness state. No workspace
// is prepared and no feature branch is pushed, so the changes present as no-work
// for the reclaim path.
func reachInProgress(t *testing.T, docketBin, ghBin string) *e2eState {
	t.Helper()
	requireRealGit(t)
	m := planRepoModes()[0]
	records := map[string]string{
		groomPath(3, "widget"): buildReadyChange(3, "widget"),
		groomPath(4, "gadget"): buildReadyChange(4, "gadget"),
	}
	repo := newMainModeRepo(t, mergeMaps(map[string]string{
		".docket.yml": "metadata_branch: main\nreclaim:\n  lease_ttl: 1\n  auto: false\nfinalize:\n  test_command: 'exit 0'\n",
	}, records))

	node := e2eNode(t, repo.invocation)
	svc, err := workspace.NewService(node.deps.Client)
	if err != nil {
		t.Fatalf("workspace.NewService: %v", err)
	}
	wdeps := WorkspaceDeps{Service: svc}
	ctx := context.Background()
	ver := func(id int, slug string) string { return blobVersionAt(t, repo.origin, m.branch, groomPath(id, slug)) }
	for _, id := range []int{3, 4} {
		cx := ContextImplementation(ctx, node.deps, node.dir, ImplementationContextRequest{ID: id})
		if cx.Result != ResultApplied || cx.Context == nil {
			t.Fatalf("context implementation id %d = %q", id, cx.Result)
		}
		cl := ChangeClaim(ctx, node.deps, node.dir, ChangeClaimRequest{ID: id, Version: cx.Context.Change.Version})
		if cl.Result != ResultApplied {
			t.Fatalf("claim id %d = %q (findings %v)", id, cl.Result, cl.Findings)
		}
	}
	// id 3 gets an owned workspace so resume-halted has a matching-ownership,
	// quiescent workspace to reprobe; id 4 is left no-work for the reclaim path.
	prep := WorkspacePrepare(ctx, node.deps, wdeps, node.dir, WorkspaceIDRequest{ID: 3, Version: ver(3, "widget")})
	if prep.Result != ResultApplied {
		t.Fatalf("workspace prepare id 3 = %q (reason %q)", prep.Result, prep.Reason)
	}

	stateFile := filepath.Join(t.TempDir(), "gh-state.json")
	xdgHome := t.TempDir()
	env := e2eEnv(stateFile, xdgHome, ghBin, repo.origin, "unused", "0000000000000000000000000000000000000000")
	return &e2eState{
		repo: repo, mode: m, node: node, id: 3,
		docketBin: docketBin, ghBin: ghBin, stateFile: stateFile, xdgHome: xdgHome, env: env,
	}
}

// mergeMaps returns a new map with b's entries overlaid on a copy of a.
func mergeMaps(a, b map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

// --- TestE2EUnsupportedConfigFence ----------------------------------------

// TestE2EUnsupportedConfigFence loads the capability requests that fenced Docket
// off before 0326 — repository-local `agents.*`, `auto_capture.enabled`,
// `build.checkpoint`, `finalize.skip_results_only_delta`, and `terminal_publish`
// — into the invocation clone's OWN `.docket.yml` (a tempdir file, never a frozen
// fixture tree), and proves every mutating 0316 operation that reruns the
// capability preflight returns `unsupported-config` naming the blockers, with the
// metadata remote and the GitHub PR left byte-for-byte untouched. Global
// model/effort pins are placed in the isolated XDG global layer and proven to
// remain supported (they never block).
func TestE2EUnsupportedConfigFence(t *testing.T) {
	t.Parallel()
	requireRealGit(t)
	docketBin, ghBin := sharedBinaries(t)
	m := planRepoModes()[0] // mode-invariant: the fence is a config-layer property.
	s := reachImplemented(t, m, docketBin, ghBin)

	// Global layer: model + effort pins remain supported (never a blocker).
	writeGlobalConfig(t, s.xdgHome, "agents:\n  claude:\n    adr:\n      model: m1\n      effort: high\n")
	version := s.ver(t)

	// Repository layer: request five deferred capabilities in the .docket.yml the
	// pin reads — the origin default branch's blob (config is read from git, never
	// the working tree). The file body is authored here in the test's own space,
	// never copied into a frozen fixture tree.
	deferred := "metadata_branch: " + m.branch + "\n" +
		"finalize:\n  test_command: 'exit 0'\n  skip_results_only_delta: true\n" +
		"auto_capture:\n  enabled: true\n" +
		"build:\n  checkpoint: true\n" +
		"terminal_publish: true\n" +
		"agents:\n  claude:\n    adr:\n      model: claude-opus-5\n"
	commitToOriginDefault(t, s.repo.origin, ".docket.yml", deferred, "request deferred capabilities")

	// A read-only op still applies under deferred caps (global pins are supported
	// and reads never fence), so context finalize is unaffected.
	cx := s.dk(t, "", "context", "finalize", "--id", strconv.Itoa(s.id))
	if cx.result() != "applied" {
		t.Fatalf("read-only context finalize under deferred caps = %q\n%s", cx.result(), cx.stdout)
	}

	beforeTip := originTip(t, s.repo.origin, m.branch)
	beforeBase := originTip(t, s.repo.origin, "main")
	evPath := s.writeInput(t, "evidence.txt", string(s.evidence))
	reportPath := s.writeInput(t, "report.json", `{"report":"fenced"}`)

	// Every mutating 0316 operation that reruns the capability preflight.
	fenced := []struct {
		name string
		args []string
	}{
		{"rebase", []string{"finalize", "rebase", "--id", strconv.Itoa(s.id), "--version", version, "--head", s.head}},
		{"publish", []string{"finalize", "publish", "--id", strconv.Itoa(s.id), "--attempt", "x", "--head", s.head, "--evidence", evPath}},
		{"block", []string{"finalize", "block", "--id", strconv.Itoa(s.id), "--version", version, "--pr-number", strconv.Itoa(s.prNumber), "--attempt", "x", "--reason", "wedged", "--head", s.head, "--input", reportPath}},
		{"clear-block", []string{"finalize", "clear-block", "--id", strconv.Itoa(s.id), "--version", version, "--head", s.head, "--pr-number", strconv.Itoa(s.prNumber)}},
		{"merge", []string{"finalize", "merge", "--id", strconv.Itoa(s.id), "--version", version, "--head", s.head}},
		{"closeout", []string{"finalize", "closeout", "--id", strconv.Itoa(s.id)}},
		{"halt", []string{"change", "halt", "--id", strconv.Itoa(s.id), "--version", version, "--input", reportPath}},
		{"resume-halted", []string{"change", "resume-halted", "--id", strconv.Itoa(s.id), "--version", version, "--acknowledge-quiescent"}},
		{"reclaim", []string{"change", "reclaim", "--id", strconv.Itoa(s.id), "--version", version}},
		{"maintenance-sweep", []string{"maintenance", "sweep"}},
	}
	for _, op := range fenced {
		op := op
		t.Run(op.name, func(t *testing.T) {
			r := s.dk(t, "", op.args...)
			if r.result() != "unsupported-config" {
				t.Fatalf("%s under deferred caps = %q, want unsupported-config\n%s", op.name, r.result(), r.stdout)
			}
			if !strings.Contains(r.stdout, "terminal_publish") && !strings.Contains(r.stdout, "build.checkpoint") &&
				!strings.Contains(r.stdout, "auto_capture") && !strings.Contains(r.stdout, "skip_results_only_delta") &&
				!strings.Contains(r.stdout, "agents.claude.adr.model") {
				t.Errorf("%s refusal named no blocker path:\n%s", op.name, r.stdout)
			}
		})
	}

	// Zero effect: the metadata remote, the integration branch, and the PR are
	// exactly where they were before the fenced attempts.
	if after := originTip(t, s.repo.origin, m.branch); after != beforeTip {
		t.Errorf("a fenced operation moved the metadata remote: %s -> %s", beforeTip, after)
	}
	if after := originTip(t, s.repo.origin, "main"); after != beforeBase {
		t.Errorf("a fenced operation moved the integration branch: %s -> %s", beforeBase, after)
	}
	if st := fakeGHPRState(t, s.stateFile); st != "OPEN" {
		t.Errorf("a fenced operation changed the PR state to %q; want OPEN (no GitHub effect)", st)
	}
}

// commitToOriginDefault commits one file onto the origin's default branch
// through a throwaway clone (fast-forward push), so a config read from the pinned
// default-branch blob observes it.
func commitToOriginDefault(t *testing.T, origin, path, content, msg string) {
	t.Helper()
	dir := t.TempDir()
	clone := filepath.Join(dir, "cfg")
	runGit(t, dir, "clone", "-q", origin, clone)
	gitIdentity(t, clone)
	def := strings.TrimSpace(runGit(t, clone, "rev-parse", "--abbrev-ref", "HEAD"))
	writeRepoFile(t, clone, path, content)
	runGit(t, clone, "add", "-A")
	runGit(t, clone, "commit", "-q", "-m", msg)
	runGit(t, clone, "push", "-q", "origin", def)
}

// writeGlobalConfig writes the isolated global-layer config.yml under an
// XDG_CONFIG_HOME tempdir.
func writeGlobalConfig(t *testing.T, xdgHome, body string) {
	t.Helper()
	dir := filepath.Join(xdgHome, "docket")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir global config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write global config: %v", err)
	}
}

// fakeGHPRState reads the current PR state token from the fake gh state file.
func fakeGHPRState(t *testing.T, stateFile string) string {
	t.Helper()
	raw, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatalf("read fake gh state: %v", err)
	}
	var list []struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatalf("decode fake gh state: %v", err)
	}
	if len(list) == 0 {
		return ""
	}
	return list[0].State
}

// --- TestE2ENoPathDocketDependency ----------------------------------------

// TestE2ENoPathDocketDependency runs the whole ordinary finalize sequence with a
// PATH scrubbed of any `docket` executable, proving nothing depends on a PATH
// `docket` (or on Go verbs in `docket.sh`): the binary is reached only by its
// absolute temporary path.
func TestE2ENoPathDocketDependency(t *testing.T) {
	t.Parallel()
	requireRealGit(t)
	docketBin, ghBin := sharedBinaries(t)
	m := planRepoModes()[0]
	s := reachImplemented(t, m, docketBin, ghBin)

	// Scrub every PATH element that carries a `docket` binary, then prove none
	// resolves under the subprocess PATH.
	s.env = scrubDocketFromPath(t, s.env)
	if p := lookPathIn(subprocessPath(s.env), "docket"); p != "" {
		t.Fatalf("PATH still resolves a docket binary at %q", p)
	}
	runOrdinaryFinalize(t, s)
}

// scrubDocketFromPath removes every PATH directory that contains a `docket`
// executable from env's PATH assignment.
func scrubDocketFromPath(t *testing.T, env []string) []string {
	t.Helper()
	pathVal := subprocessPath(env)
	var kept []string
	for _, dir := range filepath.SplitList(pathVal) {
		if dir == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, "docket")); err == nil {
			continue // drops any directory that offers a `docket`
		}
		kept = append(kept, dir)
	}
	newPath := strings.Join(kept, string(os.PathListSeparator))
	env = scrub(env, "PATH")
	return append(env, "PATH="+newPath)
}

// subprocessPath extracts the PATH assignment value from an environment slice.
func subprocessPath(env []string) string {
	for i := len(env) - 1; i >= 0; i-- {
		if strings.HasPrefix(env[i], "PATH=") {
			return env[i][len("PATH="):]
		}
	}
	return ""
}

// lookPathIn reports the first directory in pathVal that offers an executable
// named name, or "".
func lookPathIn(pathVal, name string) string {
	for _, dir := range filepath.SplitList(pathVal) {
		p := filepath.Join(dir, name)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	return ""
}

// soleCandidate decodes the finalize context document and returns its single
// candidate report, failing if there is not exactly one.
func soleCandidate(t *testing.T, r dkResult) map[string]any {
	t.Helper()
	raw, ok := r.doc["candidates"].([]any)
	if !ok || len(raw) != 1 {
		t.Fatalf("want exactly one finalize candidate, got %v", r.doc["candidates"])
	}
	c, _ := raw[0].(map[string]any)
	if c == nil {
		t.Fatalf("candidate is not an object: %v", raw[0])
	}
	return c
}

// finalizeFakeGHSource is the extended stateful fake `gh`. It tracks a LIST of
// pull requests (a stack needs several open at once) in one JSON state file, and
// speaks gh's documented `--json` shapes for the whole finalize vocabulary:
// create/view/list, `pr edit --base` (retarget) and `--body-file` (evidence),
// `pr comment` (idempotent marker comment), and `pr merge --match-head-commit`
// (a REAL merge commit pushed to the bare origin's base branch, recording
// mergedAt/mergeCommit). headRefOid and baseRef are read LIVE from the origin so
// a rewrite push or a base move is observed exactly as GitHub reports it. Fault
// modes (response loss, denial) are driven by FAKE_GH_FAULT{,_VERB}.
const finalizeFakeGHSource = `package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type pr struct {
	Number      int       ` + "`json:\"number\"`" + `
	URL         string    ` + "`json:\"url\"`" + `
	State       string    ` + "`json:\"state\"`" + `
	IsDraft     bool      ` + "`json:\"isDraft\"`" + `
	HeadRefName string    ` + "`json:\"headRefName\"`" + `
	BaseRefName string    ` + "`json:\"baseRefName\"`" + `
	Title       string    ` + "`json:\"title\"`" + `
	Body        string    ` + "`json:\"body\"`" + `
	Mergeable   string    ` + "`json:\"mergeable\"`" + `
	MergedAt    string    ` + "`json:\"mergedAt\"`" + `
	MergeCommit string    ` + "`json:\"mergeCommit\"`" + `
	Comments    []comment ` + "`json:\"comments\"`" + `
}

type comment struct {
	Body string ` + "`json:\"body\"`" + `
	URL  string ` + "`json:\"url\"`" + `
}

func flagVal(args []string, name string) string {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == name {
			return args[i+1]
		}
	}
	return ""
}

func hasFlag(args []string, name string) bool {
	for _, a := range args {
		if a == name {
			return true
		}
	}
	return false
}

func statePath() string { return os.Getenv("FAKE_GH_STATE") }

func load() []pr {
	raw, err := os.ReadFile(statePath())
	if err != nil {
		return nil
	}
	var list []pr
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil
	}
	return list
}

func save(list []pr) {
	raw, _ := json.Marshal(list)
	if err := os.WriteFile(statePath(), raw, 0o644); err != nil {
		os.Exit(65)
	}
}

func find(list []pr, number int) int {
	for i := range list {
		if list[i].Number == number {
			return i
		}
	}
	return -1
}

func git(args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", os.Getenv("FAKE_GH_ORIGIN")}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=docket-fakegh", "GIT_AUTHOR_EMAIL=fakegh@docket.test",
		"GIT_COMMITTER_NAME=docket-fakegh", "GIT_COMMITTER_EMAIL=fakegh@docket.test",
	)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

func liveHead(p pr) string {
	if oid, err := git("rev-parse", "refs/heads/"+p.HeadRefName); err == nil {
		return oid
	}
	return ""
}

func snapshot(p pr) map[string]interface{} {
	m := map[string]interface{}{
		"number":      p.Number,
		"url":         p.URL,
		"state":       p.State,
		"isDraft":     p.IsDraft,
		"headRefName": p.HeadRefName,
		"headRefOid":  liveHead(p),
		"baseRefName": p.BaseRefName,
		"title":       p.Title,
		"body":        p.Body,
		"mergeable":   p.Mergeable,
		"mergedAt":    p.MergedAt,
		"comments":    p.Comments,
	}
	if p.MergeCommit != "" {
		m["mergeCommit"] = map[string]interface{}{"oid": p.MergeCommit}
	} else {
		m["mergeCommit"] = nil
	}
	return m
}

func faultMode(verb string) string {
	if os.Getenv("FAKE_GH_FAULT_VERB") == verb {
		return os.Getenv("FAKE_GH_FAULT")
	}
	return ""
}

func argNumber(args []string) int {
	if len(args) >= 3 {
		if n, err := strconv.Atoi(args[2]); err == nil {
			return n
		}
	}
	return 0
}

func main() {
	args := os.Args[1:]
	if len(args) >= 2 && args[0] == "repo" && args[1] == "view" {
		owner := os.Getenv("FAKE_GH_OWNER")
		name := os.Getenv("FAKE_GH_NAME")
		_ = json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
			"nameWithOwner": owner + "/" + name,
			"owner":         map[string]interface{}{"login": owner},
			"name":          name,
			"url":           os.Getenv("FAKE_GH_REPO_URL"),
		})
		os.Exit(0)
	}
	if len(args) >= 1 && args[0] == "api" {
		owner, name := os.Getenv("FAKE_GH_OWNER"), os.Getenv("FAKE_GH_NAME")
		path := args[len(args)-1] // last arg is the endpoint path; --hostname rides earlier
		if path == "repos/"+owner+"/"+name {
			body := os.Getenv("FAKE_GH_REPO_SETTINGS")
			if body == "" {
				body = "{\"allow_rebase_merge\":true,\"allow_merge_commit\":true,\"allow_squash_merge\":true}"
			}
			fmt.Fprintln(os.Stdout, body)
			os.Exit(0)
		}
		if strings.HasPrefix(path, "repos/"+owner+"/"+name+"/rules/branches/") {
			body := os.Getenv("FAKE_GH_BRANCH_RULES")
			if body == "" {
				body = "[]"
			}
			fmt.Fprintln(os.Stdout, body)
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "fake gh: unmatched api path %q\n", path)
		os.Exit(64)
	}
	if len(args) < 2 || args[0] != "pr" {
		fmt.Fprintf(os.Stderr, "fake gh: unmatched invocation %v\n", args)
		os.Exit(64)
	}
	list := load()
	switch args[1] {
	case "list":
		wantState := flagVal(args, "--state")
		head := flagVal(args, "--head")
		out := []map[string]interface{}{}
		for _, p := range list {
			matchState := wantState == "all" || (wantState == "open" && p.State == "OPEN")
			matchHead := head == "" || head == p.HeadRefName
			if matchState && matchHead {
				out = append(out, snapshot(p))
			}
		}
		_ = json.NewEncoder(os.Stdout).Encode(out)
		os.Exit(0)
	case "view":
		if faultMode("view") == "loss" {
			fmt.Fprintln(os.Stderr, "fake gh: injected view transport loss")
			os.Exit(1)
		}
		i := find(list, argNumber(args))
		if i < 0 {
			fmt.Fprintln(os.Stderr, "no such pull request")
			os.Exit(1)
		}
		_ = json.NewEncoder(os.Stdout).Encode(snapshot(list[i]))
		os.Exit(0)
	case "create":
		body, _ := io.ReadAll(os.Stdin)
		number := 1
		for _, p := range list {
			if p.Number >= number {
				number = p.Number + 1
			}
		}
		p := pr{
			Number: number, URL: os.Getenv("FAKE_GH_REPO_URL") + "/pull/" + strconv.Itoa(number),
			State: "OPEN", HeadRefName: flagVal(args, "--head"), BaseRefName: flagVal(args, "--base"),
			Title: flagVal(args, "--title"), Body: string(body), Mergeable: "MERGEABLE",
		}
		list = append(list, p)
		save(list)
		fmt.Fprintln(os.Stdout, p.URL)
		os.Exit(0)
	case "edit":
		i := find(list, argNumber(args))
		if i < 0 {
			fmt.Fprintln(os.Stderr, "no such pull request")
			os.Exit(1)
		}
		if base := flagVal(args, "--base"); base != "" {
			list[i].BaseRefName = base
		}
		if hasFlag(args, "--body-file") {
			src := flagVal(args, "--body-file")
			var body []byte
			if src == "-" {
				body, _ = io.ReadAll(os.Stdin)
			} else {
				body, _ = os.ReadFile(src)
			}
			list[i].Body = string(body)
		}
		save(list)
		os.Exit(0)
	case "comment":
		if faultMode("comment") == "loss" {
			fmt.Fprintln(os.Stderr, "fake gh: injected comment transport loss")
			os.Exit(1)
		}
		i := find(list, argNumber(args))
		if i < 0 {
			fmt.Fprintln(os.Stderr, "no such pull request")
			os.Exit(1)
		}
		src := flagVal(args, "--body-file")
		var body []byte
		if src == "-" {
			body, _ = io.ReadAll(os.Stdin)
		} else if src != "" {
			body, _ = os.ReadFile(src)
		}
		url := list[i].URL + "#issuecomment-" + strconv.Itoa(len(list[i].Comments)+1)
		list[i].Comments = append(list[i].Comments, comment{Body: string(body), URL: url})
		save(list)
		fmt.Fprintln(os.Stdout, url)
		os.Exit(0)
	case "merge":
		if faultMode("merge") == "denied" {
			fmt.Fprintln(os.Stderr, "fake gh: merge denied by branch protection")
			os.Exit(1)
		}
		// Exactly one of the three method flags must be present: finalize selects
		// one method and attempts only it, so a merge with none or several is a
		// protocol error, not a permissive default.
		method := ""
		for _, f := range []string{"--rebase", "--merge", "--squash"} {
			if hasFlag(args, f) {
				if method != "" {
					fmt.Fprintln(os.Stderr, "fake gh: pr merge carries multiple method flags")
					os.Exit(64)
				}
				method = f
			}
		}
		if method == "" {
			fmt.Fprintln(os.Stderr, "fake gh: pr merge without a method flag")
			os.Exit(64)
		}
		doMerge(argNumber(args), flagVal(args, "--match-head-commit"), method)
		if faultMode("merge") == "loss" {
			fmt.Fprintln(os.Stderr, "fake gh: injected merge transport loss")
			os.Exit(1)
		}
		os.Exit(0)
	}
	fmt.Fprintf(os.Stderr, "fake gh: unmatched pr subcommand %v\n", args)
	os.Exit(64)
}

// doMerge lands the REAL graph shape for the selected method on the origin base
// branch when the PR is open at the expected head, records the merged facts, and
// is idempotent. The method flag governs the shape: "--merge" writes a two-parent
// merge commit, "--squash" a single-parent commit carrying the head tree, and
// "--rebase" a single-parent chain of one commit per feature commit not on the
// base. Whichever shape lands is recorded as MergeCommit; the destination base
// ref is fast-forwarded to it, so the adapter's reachability proof runs against
// genuine Git objects regardless of parent count.
func doMerge(number int, matchHead, method string) {
	list := load()
	i := find(list, number)
	if i < 0 {
		os.Exit(1)
	}
	if list[i].State == "MERGED" {
		return
	}
	head := liveHead(list[i])
	if matchHead != "" && matchHead != head {
		os.Exit(1)
	}
	tree, err := git("rev-parse", head+"^{tree}")
	if err != nil {
		os.Exit(1)
	}
	baseTip, err := git("rev-parse", "refs/heads/"+list[i].BaseRefName)
	if err != nil {
		os.Exit(1)
	}
	var mc string
	switch method {
	case "--merge":
		mc, err = git("commit-tree", tree, "-p", baseTip, "-p", head, "-m", "Merge pull request #"+strconv.Itoa(number))
	case "--squash":
		mc, err = git("commit-tree", tree, "-p", baseTip, "-m", "squash merge PR #"+strconv.Itoa(number))
	case "--rebase":
		// One rebased commit per feature commit not on base; single-parent chain.
		revs, rerr := git("rev-list", "--reverse", baseTip+".."+head)
		if rerr != nil {
			os.Exit(1)
		}
		tip := baseTip
		for _, rev := range strings.Fields(revs) {
			rtree, terr := git("rev-parse", rev+"^{tree}")
			if terr != nil {
				os.Exit(1)
			}
			tip, terr = git("commit-tree", rtree, "-p", tip, "-m", "rebased "+rev)
			if terr != nil {
				os.Exit(1)
			}
		}
		mc = tip
		if mc == baseTip {
			os.Exit(1) // nothing to rebase: refuse rather than fake
		}
	default:
		os.Exit(64)
	}
	if err != nil {
		os.Exit(1)
	}
	if _, err := git("update-ref", "refs/heads/"+list[i].BaseRefName, mc); err != nil {
		os.Exit(1)
	}
	list[i].State = "MERGED"
	list[i].MergeCommit = mc
	list[i].MergedAt = time.Now().UTC().Format(time.RFC3339)
	save(list)
}
`
