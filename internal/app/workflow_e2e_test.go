package app

import (
	"context"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/workspace"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

// This is the hermetic, end-to-end claim→implemented workflow test: it drives
// every operation from Tasks 1–10 in spec order over real bare-remote temporary
// repositories, a real transaction.Engine, the landed native gate supervisor,
// and a real githubcli.Client backed by a fake `gh` binary on PATH — never a
// stubbed GitHubService. It proves acceptance criteria 1 and 6: one run takes a
// build-ready change through claim, reconcile, verified plan, passed gate,
// evidence, PR publication, and `implemented` without ever writing metadata
// directly, in BOTH repository modes; and a configuration that actively requests
// a deferred capability is refused (`unsupported-config`) before any mutation.
//
// The GitHub seam is exercised for real. The fake `gh` is a separately compiled,
// stateful program (built once per run) that speaks gh's documented `--json`
// nested shapes, so the real adapter's probe/act/verify sequence — list-all →
// create → list-open → view-by-number — round-trips exactly as it would against
// GitHub, with the PR body preserved byte-for-byte through stdin. The gate is a
// real trivially-passing fixture script launched through GateLaunch and observed
// to `passed`; evidence is derived from that terminal observation, never an
// agent-supplied boolean.
//
// The negative halves are load-bearing. Every commit that lands on the metadata
// remote is proven to carry the engine's Docket-Transaction-ID trailer, so no
// operation wrote metadata behind the transaction engine's back; the workflow
// runs with no `$DOCKET_SCRIPTS_DIR` on the environment, so no legacy Bash facade
// could have been consulted; and the unsupported-capability fixture is refused
// with the metadata remote left exactly where it started.

// runClaimToImplemented drives the full positive sequence for one metadata mode.
func runClaimToImplemented(t *testing.T, m planRepoMode, ghBin string) {
	t.Helper()
	const (
		id   = 3
		slug = "widget"
	)
	recPath := groomPath(id, slug)
	planPath := "docs/superpowers/plans/2026-08-17-widget-plan.md"

	// A resolved build.test_command is required so EvidenceRecord (build-owned
	// since change 0374) records a real observed gate command rather than
	// refusing with unconfigured-gate-command; finalize.test_command is set too
	// so the finalize path stays configured. Both are set in the config layer
	// each mode reads: on main in main mode, on main (the integration branch) in
	// docket mode.
	repo := buildConfiguredRepo(t, m, recPath, buildReadyChange(id, slug))
	ctx := context.Background()

	node := planningDepsFor(t, repo.invocation)
	svc, err := workspace.NewService(node.deps.Client)
	if err != nil {
		t.Fatalf("workspace.NewService: %v", err)
	}
	wdeps := WorkspaceDeps{Service: svc}

	// The metadata remote tip BEFORE the run: every commit past it must be an
	// engine transaction.
	baseTip := originTip(t, repo.origin, m.branch)

	// ver reads the change record's current entity version from the bare origin —
	// the independent oracle each exact-version request submits.
	ver := func() string { return blobVersionAt(t, repo.origin, m.branch, recPath) }

	// (1) Authoritative implementation context.
	ctxRes := ContextImplementation(ctx, node.deps, node.dir, ImplementationContextRequest{ID: id})
	if ctxRes.Result != ResultApplied || ctxRes.Context == nil {
		t.Fatalf("context implementation = %q (reason %q); want a bundle", ctxRes.Result, ctxRes.Reason)
	}
	if !ctxRes.Context.ClaimEligible {
		t.Fatalf("context reports the build-ready change not claim-eligible: %q", ctxRes.Context.ClaimRefusal)
	}
	v := ctxRes.Context.Change.Version
	if v != ver() {
		t.Fatalf("context version %q disagrees with the origin oracle %q", v, ver())
	}

	// (2) Claim.
	claim := ChangeClaim(ctx, node.deps, node.dir, ChangeClaimRequest{ID: id, Version: v})
	if claim.Result != ResultApplied || claim.Disposition != ClaimDispositionApplied {
		t.Fatalf("claim = (%q, %q), want applied/applied (findings %v)", claim.Result, claim.Disposition, claim.Findings)
	}

	// (3) Reconcile: sets reconciled:true and refreshes the claim.
	rec := ChangeReconcile(ctx, node.deps, node.dir, ChangeReconcileRequest{
		ID: id, Version: ver(),
		ReconcileLogEntry: "Reconciled against current reality.\n",
	})
	if rec.Result != ResultApplied {
		t.Fatalf("reconcile = %q (findings %v)", rec.Result, rec.Findings)
	}

	// (4) Prepare the feature workspace at the resolved base.
	prep := WorkspacePrepare(ctx, node.deps, wdeps, node.dir, WorkspaceIDRequest{ID: id, Version: ver()})
	if prep.Result != ResultApplied {
		t.Fatalf("workspace prepare = %q (reason %q msg %q)", prep.Result, prep.Reason, prep.Message)
	}
	wp := prep.Path

	// (5) The plan-writer half: author the plan body, stamp the deterministic
	// backlink through the artifact-backlink operation (writing a feature-tree
	// file, not a metadata transaction), then commit with the ADR-0094 single
	// artifact and plan-path trailer.
	writeRepoFile(t, wp, planPath, "# Implementation Plan\n\nConcrete steps here.\n")
	bl := ArtifactBacklink(ctx, node.deps, wp, ArtifactBacklinkRequest{ArtifactPath: planPath, ChangePath: recPath})
	if bl.Result != ResultApplied {
		t.Fatalf("artifact backlink = %q (reason %q msg %q)", bl.Result, bl.Reason, bl.Message)
	}
	runGit(t, wp, "add", "-A")
	runGit(t, wp, "commit", "-q", "-m", "write plan", "--trailer", "Docket-Plan-Path: "+planPath)
	planHead := runGit(t, wp, "rev-parse", "HEAD")

	// (6) Attach the verified plan.
	attach := ChangeAttachPlan(ctx, node.deps, wdeps, node.dir,
		ChangeAttachRequest{ID: id, Version: ver(), Path: planPath, Commit: planHead})
	if attach.Result != ResultApplied {
		t.Fatalf("attach plan = %q (reason %q msg %q findings %v)", attach.Result, attach.Reason, attach.Message, attach.Findings)
	}

	// (7) The implementation commit advances the feature head.
	writeRepoFile(t, wp, "widget.go", "package widget\n")
	runGit(t, wp, "add", "-A")
	runGit(t, wp, "commit", "-q", "-m", "implement the widget")
	head := runGit(t, wp, "rev-parse", "HEAD")

	// (8) Launch the real trivially-passing gate through the native supervisor and
	// observe it to a passed terminal.
	gateRoot := t.TempDir()
	launch := GateLaunch(gateRoot, wp, []string{passingGateScript(t)})
	if launch.Result != ResultApplied || launch.RunDir == "" {
		t.Fatalf("gate launch = %q (reason %q)", launch.Result, launch.Reason)
	}
	pollGatePassed(t, launch.RunDir)

	// (9) Evidence is derived from the passed terminal observation at the current
	// feature head — never an agent-supplied command or boolean.
	evd := EvidenceRecord(ctx, node.deps, wdeps, node.dir, EvidenceRecordRequest{ID: id, RunDir: launch.RunDir, Head: head})
	if evd.Result != ResultApplied || evd.Block == "" {
		t.Fatalf("evidence record = %q (reason %q msg %q)", evd.Result, evd.Reason, evd.Message)
	}
	evidenceBytes := []byte(evd.Block)

	// (10) Verify the record against the exact head (the invalidate-on-fix pin).
	if v := EvidenceVerify(EvidenceVerifyRequest{RecordFile: evidenceBytes, Head: head}); v.Result != ResultApplied {
		t.Fatalf("evidence verify = %q (verdict %q reason %q)", v.Result, v.Verdict, v.Reason)
	}

	// (11) Publish the feature head to the remote.
	pub := WorkspacePublish(ctx, node.deps, wdeps, node.dir, WorkspacePublishRequest{ID: id, Head: head})
	if pub.Result != ResultApplied {
		t.Fatalf("workspace publish = %q (reason %q msg %q)", pub.Result, pub.Reason, pub.Message)
	}

	// The GitHub seam: a real githubcli.Client over the fake gh, told the exact
	// published head so its created PR reports it as headRefOid.
	stateFile := filepath.Join(t.TempDir(), "gh-state.json")
	ghEnv := append(os.Environ(),
		"FAKE_GH_STATE="+stateFile,
		"FAKE_GH_REPO_URL=https://github.com/acme/widget",
		"FAKE_GH_OWNER=acme",
		"FAKE_GH_NAME=widget",
		"FAKE_GH_HEAD="+head,
	)
	ghClient, err := githubcli.NewClient(githubcli.WithExecutable(ghBin), githubcli.WithBaseEnvironment(ghEnv))
	if err != nil {
		t.Fatalf("githubcli.NewClient over fake gh: %v", err)
	}
	gdeps := GitHubDeps{Service: ghClient}

	// (12) Publish the pull request with authored prose; the operation weaves in
	// the backlink and evidence blocks and drives the real probe/act/verify path.
	pr := PRPublish(ctx, node.deps, wdeps, gdeps, node.dir, PRPublishRequest{
		ID:             id,
		Head:           head,
		Title:          "Add the widget",
		Body:           "Authored PR prose for the widget.\n",
		EvidenceRecord: evidenceBytes,
	})
	if pr.Result != ResultApplied || pr.Disposition != string(githubcli.EnsureCreated) {
		t.Fatalf("pr publish = %q (disposition %q reason %q msg %q)", pr.Result, pr.Disposition, pr.Reason, pr.Message)
	}
	if pr.Head != head || pr.Base == "" || pr.Reference == "" {
		t.Fatalf("pr snapshot did not round-trip: %+v", pr)
	}

	// (13) Mark implemented after reprobing every published effect.
	mi := ChangeMarkImplemented(ctx, node.deps, wdeps, gdeps, node.dir, MarkImplementedRequest{
		ID:             id,
		Version:        ver(),
		Head:           head,
		PR:             pr.Reference,
		EvidenceRecord: evidenceBytes,
	})
	if mi.Result != ResultApplied || mi.Status != string("implemented") {
		t.Fatalf("mark implemented = %q (status %q findings %v)", mi.Result, mi.Status, mi.Findings)
	}

	// (14) Read-only run verification: every postcondition holds ⇒ run-complete.
	rv := RunVerify(ctx, node.deps, wdeps, gdeps, node.dir, RunVerifyRequest{ID: id})
	if rv.Result != ResultApplied || rv.Verdict != VerdictRunComplete {
		t.Fatalf("run verify = %q verdict %q, want applied/run-complete (unmet %v)", rv.Result, rv.Verdict, rv.Unmet)
	}
	if len(rv.Unmet) != 0 {
		t.Errorf("run-complete carried unmet conjuncts: %v", rv.Unmet)
	}

	// Negative half: every metadata-remote commit past the fixture base is an
	// engine transaction (carries Docket-Transaction-ID), and the four durable
	// transitions are exactly the operations that ran — no direct skill-owned
	// metadata write slipped in.
	assertEngineOnlyMetadataCommits(t, repo.origin, m.branch, baseTip,
		[]string{"change.attach-plan", "change.claim", "change.mark-implemented", "change.reconcile"})
}

// buildConfiguredRepo builds the docket-topology bare remote with resolved
// build.test_command and finalize.test_command in the repository config layer,
// plus the one build-ready change record on the metadata branch.
func buildConfiguredRepo(t *testing.T, m planRepoMode, recPath, record string) *gitRepo {
	t.Helper()
	if m.name != "docket" {
		t.Fatalf("unknown repository topology %q", m.name)
	}
	return newDocketModeRepo(t,
		map[string]string{".docket.yml": "integration_branch: main\nbuild:\n  test_command: 'go test ./...'\nfinalize:\n  test_command: 'go test ./...'\n"},
		map[string]string{recPath: record})
}

// assertDeferredCapabilityBlocksClaim proves a `.docket.yml` that actively
// requests a deferred capability refuses the first mutation with
// `unsupported-config`, before the transaction engine is ever reached, leaving
// the metadata remote untouched.
func assertDeferredCapabilityBlocksClaim(t *testing.T) {
	t.Helper()
	requireRealGit(t)
	const (
		id   = 3
		slug = "widget"
	)
	recPath := groomPath(id, slug)
	repo := newWorkingRepo(t, map[string]string{
		// auto_groom: true is a deferred capability request — well-formed and
		// inspectable, but not mutable until withdrawn.
		".docket.yml": "auto_groom: true\n",
		recPath:       buildReadyChange(id, slug),
	})
	node := planningDepsFor(t, repo.invocation)
	before := originTip(t, repo.origin, "docket")
	version := blobVersionAt(t, repo.origin, "docket", recPath)

	res := ChangeClaim(context.Background(), node.deps, node.dir, ChangeClaimRequest{ID: id, Version: version})
	if res.Result != ResultUnsupportedConfig {
		t.Fatalf("claim under a deferred-capability request = %q, want unsupported-config (findings %v)", res.Result, res.Findings)
	}
	if !hasFindingCode(res.Findings, ReasonDeferredCapRequested) {
		t.Errorf("refusal did not name the deferred-capability reason: %v", res.Findings)
	}
	if after := originTip(t, repo.origin, "docket"); after != before {
		t.Errorf("a refused-before-mutation claim moved the metadata remote: %q -> %q", before, after)
	}
}

// assertEngineOnlyMetadataCommits proves every commit on branch past base carries
// the engine's Docket-Transaction-ID trailer (so nothing wrote the metadata
// remote outside the transaction engine) and that the set of Docket-Operation
// values equals wantOps.
func assertEngineOnlyMetadataCommits(t *testing.T, origin, branch, base string, wantOps []string) {
	t.Helper()
	shas := strings.Fields(runGit(t, origin, "rev-list", base+".."+branch))
	var ops []string
	for _, sha := range shas {
		txid := strings.TrimSpace(runGit(t, origin, "show", "-s",
			"--format=%(trailers:key=Docket-Transaction-ID,valueonly=true)", sha))
		if txid == "" {
			t.Errorf("metadata commit %s carries no engine transaction trailer — a direct skill-owned write", sha)
		}
		op := strings.TrimSpace(runGit(t, origin, "show", "-s",
			"--format=%(trailers:key=Docket-Operation,valueonly=true)", sha))
		ops = append(ops, op)
	}
	sort.Strings(ops)
	sorted := append([]string(nil), wantOps...)
	sort.Strings(sorted)
	if strings.Join(ops, ",") != strings.Join(sorted, ",") {
		t.Errorf("metadata engine operations = %v, want exactly %v", ops, sorted)
	}
}

// pollGatePassed observes runDir until it reports a passed terminal, failing the
// test if it never does within a generous bound.
func pollGatePassed(t *testing.T, runDir string) {
	t.Helper()
	for i := 0; i < 300; i++ {
		obs := GateObserve(runDir)
		if obs.Result == ResultApplied && obs.State == "passed" {
			return
		}
		if obs.State != "running" && obs.State != "passed" {
			t.Fatalf("gate reached a non-passed terminal: state=%q reason=%q", obs.State, obs.Reason)
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("gate never became passed")
}

// passingGateScript writes a trivially-passing gate command and returns its
// absolute path.
func passingGateScript(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "gate.sh")
	if err := os.WriteFile(p, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write gate script: %v", err)
	}
	return p
}

// buildFakeGH compiles the stateful fake `gh` program into a temp binary and
// returns its path. It is built once and reused across the mode table. The fake
// speaks gh's documented `--json` nested shapes so the real githubcli adapter's
// probe/act/verify sequence round-trips exactly as it would against GitHub.
func buildFakeGH(t *testing.T) string {
	t.Helper()
	goBin, err := exec.LookPath("go")
	if err != nil {
		goBin = filepath.Join(runtime.GOROOT(), "bin", "go")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module fakegh\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatalf("write fake gh go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(fakeGHSource), 0o644); err != nil {
		t.Fatalf("write fake gh source: %v", err)
	}
	out := filepath.Join(dir, "gh")
	cmd := exec.Command(goBin, "build", "-o", out, ".")
	cmd.Dir = dir
	if combined, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake gh: %v\n%s", err, combined)
	}
	return out
}

// fakeGHSource is the fake `gh` program. It is a stateful, protocol-faithful stand
// in for the pieces of gh the githubcli adapter drives: `repo view` (identity),
// `pr list` (probe + verify), `pr create` (act, body on stdin), and
// `pr view <n>` (verify by number). The single PR it tracks is persisted to
// FAKE_GH_STATE so the adapter's create-then-requery sequence observes the PR it
// just created, with the body preserved byte-for-byte.
const fakeGHSource = `package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type prJSON struct {
	Number      int    ` + "`json:\"number\"`" + `
	URL         string ` + "`json:\"url\"`" + `
	State       string ` + "`json:\"state\"`" + `
	IsDraft     bool   ` + "`json:\"isDraft\"`" + `
	HeadRefName string ` + "`json:\"headRefName\"`" + `
	HeadRefOid  string ` + "`json:\"headRefOid\"`" + `
	BaseRefName string ` + "`json:\"baseRefName\"`" + `
	Title       string ` + "`json:\"title\"`" + `
	Body        string ` + "`json:\"body\"`" + `
}

func flagVal(args []string, name string) string {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == name {
			return args[i+1]
		}
	}
	return ""
}

func loadPR() (prJSON, bool) {
	raw, err := os.ReadFile(os.Getenv("FAKE_GH_STATE"))
	if err != nil {
		return prJSON{}, false
	}
	var pr prJSON
	if err := json.Unmarshal(raw, &pr); err != nil {
		return prJSON{}, false
	}
	return pr, true
}

func savePR(pr prJSON) {
	raw, _ := json.Marshal(pr)
	if err := os.WriteFile(os.Getenv("FAKE_GH_STATE"), raw, 0o644); err != nil {
		os.Exit(65)
	}
}

func main() {
	args := os.Args[1:]
	if len(args) >= 2 && args[0] == "repo" && args[1] == "view" {
		owner := os.Getenv("FAKE_GH_OWNER")
		name := os.Getenv("FAKE_GH_NAME")
		out := map[string]interface{}{
			"nameWithOwner": owner + "/" + name,
			"owner":         map[string]interface{}{"login": owner},
			"name":          name,
			"url":           os.Getenv("FAKE_GH_REPO_URL"),
		}
		_ = json.NewEncoder(os.Stdout).Encode(out)
		os.Exit(0)
	}
	if len(args) >= 2 && args[0] == "pr" {
		switch args[1] {
		case "list":
			state := flagVal(args, "--state")
			pr, ok := loadPR()
			list := []prJSON{}
			if ok && (state == "all" || (state == "open" && pr.State == "OPEN")) {
				list = append(list, pr)
			}
			_ = json.NewEncoder(os.Stdout).Encode(list)
			os.Exit(0)
		case "view":
			pr, ok := loadPR()
			if !ok {
				fmt.Fprintln(os.Stderr, "no such pull request")
				os.Exit(1)
			}
			_ = json.NewEncoder(os.Stdout).Encode(pr)
			os.Exit(0)
		case "create":
			body, _ := io.ReadAll(os.Stdin)
			pr := prJSON{
				Number:      1,
				URL:         os.Getenv("FAKE_GH_REPO_URL") + "/pull/1",
				State:       "OPEN",
				IsDraft:     false,
				HeadRefName: flagVal(args, "--head"),
				HeadRefOid:  os.Getenv("FAKE_GH_HEAD"),
				BaseRefName: flagVal(args, "--base"),
				Title:       flagVal(args, "--title"),
				Body:        string(body),
			}
			savePR(pr)
			fmt.Fprintln(os.Stdout, pr.URL)
			os.Exit(0)
		case "edit":
			os.Exit(0)
		}
	}
	fmt.Fprintf(os.Stderr, "fake gh: unmatched invocation %v\n", args)
	os.Exit(64)
}
`
