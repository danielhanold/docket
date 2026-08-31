package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/repository/transaction"
	"github.com/danielhanold/docket/internal/workspace"
)

// This file is Task 10: traffic accounting for `maintenance sweep` driven through
// the PRODUCTION entry point app.MaintenanceSweep over REAL git and gh processes,
// counted at the executable boundary. Unlike maintenance_test.go (which proves the
// orchestration over recording seams) and sweep_session_test.go (which proves the
// one-metadata-fetch-per-attempt / bound-reader contract at the session seam),
// these tests build the whole FinalizeDeps the CLI wires — a transaction engine,
// status reader, workspace service, PR prober, batched PR reader, gate, and
// CleanupGit, all over one policy-carrying gitcli.Client and one githubcli.Client
// — and then classify every logged git/gh invocation by PURPOSE:
//
//   - `fetch … refs/heads/docket`         metadata preparation (setup pin, or an op)
//   - `ls-remote --symref … HEAD`         the setup default-branch probe
//   - `ls-remote … refs/heads/docket`     the setup metadata-ref discovery
//   - `ls-remote --heads`                 the ONE shared inventory advertisement
//   - `ls-remote … refs/heads/feat/…`     a FORBIDDEN per-item remote-ref probe
//   - gh `repo view`                      the single GitHub identity resolution
//   - gh `api graphql`                    one batched exact-number PR read (≤25)
//
// The point is real process accounting: no argv spelling is banned globally — the
// setup pin legitimately issues one `ls-remote refs/heads/docket`, while a per-
// HISTORY feature-ref probe is the defect these tests forbid.
//
// Realism note (Task 10 realism clause): the "exactly one metadata fetch per
// dispatched operation, shared by helper+operation+nested readers" property is
// proved cleanly at the session seam by sweep_session_test.go
// (TestPrepareIsOneMetadataFetchZeroSetupProbes, TestBoundReaderNeverFetches) with
// the same real-process counting. It is NOT re-asserted as a bare fetch count
// around a live mutation here, because a dispatched reclaim's transaction engine
// legitimately re-fetches the metadata branch for its own fresh-origin CAS commit
// (learning cas-re-read-fresh-origin) — that fetch is required proof, not a
// duplicate reader read, so a naive "one fetch per op" count around a real
// mutation would be counting two different, both-correct things.
//
// Gaps deliberately left to their existing owners / noted precisely:
//   - A successful closeout+cleanup SUFFIX over a fake `gh` executable would need a
//     scripted gh speaking the full merged-facts reprobe/merge surface; that live
//     end-to-end suffix is exercised by the CLI e2e matrix (finalize_e2e_test.go),
//     not re-built here. The dispatched-operation traffic proof here uses reclaim,
//     which needs no gh.
//   - Mid-sweep source movement between prepare points (Step 4's two-phase hook) is
//     proved at the session seam by sweep_session_test.go's TestPrepareObservesFresh
//     MetadataTip; the movement asserted end-to-end here is the metadata-fetch
//     failure/deletion path (no stale fallback).

// --- harness --------------------------------------------------------------

// trafficGit builds a production gitcli.Client whose executable is a shell
// wrapper that appends every invocation's argument line to logPath and then, if
// fault is non-empty, evaluates fault BEFORE delegating to the real git — so a
// scripted fault can fail-fast (an immediate non-zero exit, never a hang) a
// forbidden invocation. An empty fault is a pure counting wrapper.
func trafficGit(t *testing.T, logPath, fault string) *gitcli.Client {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	dir := t.TempDir()
	wrapper := filepath.Join(dir, "git")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '" + logPath + "'\n"
	if strings.TrimSpace(fault) != "" {
		script += fault + "\n"
	}
	script += "exec '" + realGit + "' \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	// Isolate the global docket config layer, exactly as newGitClient does.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	client, err := gitcli.NewClient(gitcli.WithExecutable(wrapper))
	if err != nil {
		t.Fatalf("gitcli.NewClient: %v", err)
	}
	return client
}

// trafficGH builds a production githubcli.Client whose executable is a scripted
// fake `gh` that logs every invocation to logPath and answers the two verbs the
// sweep's discovery uses: `repo view` (the identity resolution) with a fixed
// acme/widgets identity, and `api graphql` (the batched PR read) with an empty
// repository object. An empty repository object resolves every aliased number to
// Found=false (UNKNOWN), which is exactly what a non-actionable population needs:
// the process is counted, no PR bands into closeout, and no mutation is dispatched.
func trafficGH(t *testing.T, logPath string) *githubcli.Client {
	t.Helper()
	dir := t.TempDir()
	wrapper := filepath.Join(dir, "gh")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '" + logPath + "'\n" +
		"case \"$1 $2\" in\n" +
		"'repo view') printf '%s' '{\"nameWithOwner\":\"acme/widgets\",\"owner\":{\"login\":\"acme\"},\"name\":\"widgets\",\"url\":\"https://github.com/acme/widgets\"}';;\n" +
		"'api graphql') printf '%s' '{\"data\":{\"repository\":{}}}';;\n" +
		"*) printf '%s' '{}';;\n" +
		"esac\n"
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	client, err := githubcli.NewClient(githubcli.WithExecutable(wrapper))
	if err != nil {
		t.Fatalf("githubcli.NewClient: %v", err)
	}
	return client
}

// trafficDeps assembles the exact FinalizeDeps shape the CLI's newSweepFinalizeDeps
// builds (newFinalizeDepsOver): every nested seam — engine, reader, workspace,
// prober, batch reader, gate, CleanupGit — over the SAME two counting clients, so
// every git/gh process the sweep spawns is recorded.
func trafficDeps(t *testing.T, git *gitcli.Client, gh *githubcli.Client) FinalizeDeps {
	t.Helper()
	engine, err := transaction.NewEngine(git, testClock())
	if err != nil {
		t.Fatalf("transaction.NewEngine: %v", err)
	}
	ws, err := workspace.NewService(git)
	if err != nil {
		t.Fatalf("workspace.NewService: %v", err)
	}
	planning := PlanningDeps{Client: git, Engine: engine, Reader: NewGitStatusReader(git), Clock: testClock()}
	return FinalizeDeps{
		Planning:   planning,
		GitHub:     gh,
		Workspace:  ws,
		PRProber:   NewGitHubFinalizeProber(gh),
		PRBatch:    NewSweepPRBatchReader(gh),
		Gate:       NewFinalizeGate(planning, WorkspaceDeps{Service: ws}),
		CleanupGit: git,
	}
}

// ghLogLines returns the recorded gh argument lines, oldest first.
func ghLogLines(t *testing.T, logPath string) []string {
	t.Helper()
	data, err := os.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	var out []string
	for _, l := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

// countPrefix counts recorded lines beginning with prefix (gh verbs are the first
// two argv tokens: "repo view", "api graphql").
func countPrefix(lines []string, prefix string) int {
	n := 0
	for _, l := range lines {
		if strings.HasPrefix(l, prefix) {
			n++
		}
	}
	return n
}

// featureRefProbes counts FORBIDDEN per-item remote-ref probes: an `ls-remote`
// that is neither the setup `--symref … HEAD` default-branch probe, nor the setup
// metadata-ref discovery (`ls-remote … refs/heads/docket`), nor the one shared
// `ls-remote --heads` inventory. Any remaining ls-remote is a per-ref probe the
// batched/shared design forbids (learning probe-error-is-not-clean-absence: the
// assessment reads the shared advertisement, never a fan-out of per-ref reads).
func featureRefProbes(lines []string) []string {
	var out []string
	for _, l := range lines {
		if !strings.Contains(l, "ls-remote") {
			continue
		}
		if strings.Contains(l, "--symref") || strings.Contains(l, "--heads") || strings.Contains(l, "refs/heads/docket") {
			continue
		}
		out = append(out, l)
	}
	return out
}

// trafficRepo builds a docket-mode repository whose main branch carries the given
// .docket.yml body (defaulting to the plain integration_branch config) and whose
// docket branch carries the given planning records plus a board view.
func trafficRepo(t *testing.T, docketYML string, records map[string]string) *gitRepo {
	t.Helper()
	if docketYML == "" {
		docketYML = "integration_branch: main\n"
	}
	recs := map[string]string{"docs/changes/BOARD.md": "# Board\n"}
	for k, v := range records {
		recs[k] = v
	}
	return newDocketModeRepo(t, map[string]string{".docket.yml": docketYML}, recs)
}

// trafficDoneRecord is a valid terminal (done) record: a coherent identity
// (recorded branch, canonical PR ref, resolvable base, workspace target) whose
// integration artifacts are absent and whose feature workspace has no manifest —
// so its every destructive leg resolves LOCALLY to a non-actionable verdict at the
// pinned inventory (snapshot-blocked: a missing manifest never certifies clean),
// with no per-item metadata, PR, or remote-ref read. It is the historical
// population the assessment reasons over from shared snapshots alone.
func trafficDoneRecord(id int, slug string) string {
	return closeoutRecord(id, slug, "done", fmt.Sprintf("acme/widgets#%d", id),
		fmt.Sprintf("docs/superpowers/specs/2026-08-16-%s.md", slug),
		fmt.Sprintf("docs/superpowers/plans/2026-08-16-%s.md", slug),
		fmt.Sprintf("docs/changes/results/%04d-%s-results.md", id, slug))
}

// trafficImplementedOpen is a non-terminal, PR-bearing record: it joins the
// finalize population the batched PR read covers, but its PR is UNKNOWN to the fake
// gh (empty repository object) so it bands to nothing and the sweep leaves it
// entirely alone — a population member that drives the batch count without
// triggering any mutation (mirrors TestSweepNeverEscalates).
func trafficImplementedOpen(id int, slug string) string {
	rec := lifecycleChange(id, slug, "in-progress")
	rec = strings.Replace(rec, "status: in-progress\n", "status: implemented\n", 1)
	rec = strings.Replace(rec, "blocked_by:\n", "pr: 'acme/widgets#"+itoaTest(id)+"'\nblocked_by:\n", 1)
	return rec
}

// nonActionable reports whether an entry is a pre-dispatch snapshot assessment —
// an EMPTY Operation and a non-applied disposition — i.e. a historical record the
// sweep resolved locally and did NOT dispatch a cleanup for.
func nonActionable(e MaintenanceEntry) bool {
	return e.Operation == "" && e.Disposition != SweepDispApplied
}

// --- Step 1: discovery + assessment traffic accounting --------------------

// TestIntegrationSweepDiscoveryTrafficAccounting drives one full-scope sweep over
// a mixed corpus — several non-terminal PR-bearing records (the batch population)
// and several terminal done records (the assessment population) — and accounts for
// every real git/gh process by purpose. The setup pin runs exactly once; the
// GitHub identity resolves at most once; PR requests are one batch per ≤25 unique
// numbers; the shared inventory is a single `ls-remote --heads`; and NO per-item
// remote-ref probe or metadata fetch is issued for the non-actionable history.
func TestIntegrationSweepDiscoveryTrafficAccounting(t *testing.T) {
	requireRealGit(t)
	records := map[string]string{}
	// 30 population members => ceil(30/25) = 2 batches.
	for i := 0; i < 30; i++ {
		id := 200 + i
		records[fmt.Sprintf("docs/changes/active/%04d-pop.md", id)] = trafficImplementedOpen(id, fmt.Sprintf("pop%02d", i))
	}
	// Three terminal done records the assessment resolves from shared snapshots.
	doneIDs := []int{41, 42, 43}
	for _, id := range doneIDs {
		records[fmt.Sprintf("docs/changes/active/%04d-hist.md", id)] = trafficDoneRecord(id, fmt.Sprintf("hist%02d", id))
	}
	r := trafficRepo(t, "", records)
	gitLog := filepath.Join(t.TempDir(), "git.log")
	ghLog := filepath.Join(t.TempDir(), "gh.log")
	deps := trafficDeps(t, trafficGit(t, gitLog, ""), trafficGH(t, ghLog))

	res := MaintenanceSweep(context.Background(), deps, r.invocation, SweepScopeFull)

	gl := gitLogLines(t, gitLog)
	ghl := ghLogLines(t, ghLog)

	// Full setup ran ONCE: exactly one default-branch probe.
	if got := countMatching(gl, "ls-remote", "--symref"); got != 1 {
		t.Errorf("setup default-branch probe ran %d times, want exactly 1\n%s", got, strings.Join(gl, "\n"))
	}
	// GitHub identity resolved at most once.
	if got := countPrefix(ghl, "repo view"); got > 1 {
		t.Errorf("GitHub identity resolved %d times, want at most 1", got)
	}
	// One batched PR read per 25 unique numbers.
	if got := countPrefix(ghl, "api graphql"); got != 2 {
		t.Errorf("api graphql invocations = %d, want 2 (ceil(30/25)); no per-PR fallback\n%s", got, strings.Join(ghl, "\n"))
	}
	// At most one shared inventory advertisement — never one per historical record.
	if got := countMatching(gl, "ls-remote", "--heads"); got != 1 {
		t.Errorf("ls-remote --heads ran %d times, want exactly 1 (one shared inventory, not one per history)", got)
	}
	// No forbidden per-item remote-ref probe for the non-actionable history.
	if probes := featureRefProbes(gl); len(probes) != 0 {
		t.Errorf("the sweep issued %d forbidden per-item remote-ref probe(s):\n%s", len(probes), strings.Join(probes, "\n"))
	}
	// No dispatched-operation metadata fetch beyond the two setup fetches
	// (default-branch + metadata branch): nothing was actionable, so no operation
	// prepared a fresh observation. A per-history refresh would inflate this.
	if got := countMatching(gl, "fetch"); got != 2 {
		t.Errorf("total fetches = %d, want exactly 2 (setup default-branch + metadata; no per-history refresh)\n%s", got, strings.Join(gl, "\n"))
	}

	// Pin the OUTCOME, not merely that it returned: one truthful non-actionable
	// entry per historical candidate, none dispatched.
	if res.Result != ResultNoOp {
		t.Fatalf("result = %q, want no-op (nothing actionable); entries=%+v", res.Result, res.Entries)
	}
	seen := map[int]MaintenanceEntry{}
	for _, e := range res.Entries {
		seen[e.ID] = e
	}
	for _, id := range doneIDs {
		e, ok := seen[id]
		if !ok {
			t.Errorf("historical record %d produced no entry", id)
			continue
		}
		if !nonActionable(e) {
			t.Errorf("historical record %d must be a non-actionable pre-dispatch entry, got %+v", id, e)
		}
	}
	// The population members are left entirely alone: no entry at all.
	for i := 0; i < 30; i++ {
		if _, ok := seen[200+i]; ok {
			t.Errorf("open-PR population member %d must produce no entry", 200+i)
		}
	}
}

// TestIntegrationSweepZeroPRsNoGitHubTraffic: a corpus whose only records are
// terminal (done) — hence outside the PR-bearing finalize population — issues ZERO
// gh processes. An empty number set never resolves the identity and never opens a
// batch.
func TestIntegrationSweepZeroPRsNoGitHubTraffic(t *testing.T) {
	requireRealGit(t)
	records := map[string]string{
		"docs/changes/active/0041-a.md": trafficDoneRecord(41, "a"),
		"docs/changes/active/0042-b.md": trafficDoneRecord(42, "b"),
	}
	r := trafficRepo(t, "", records)
	ghLog := filepath.Join(t.TempDir(), "gh.log")
	deps := trafficDeps(t, trafficGit(t, filepath.Join(t.TempDir(), "git.log"), ""), trafficGH(t, ghLog))

	res := MaintenanceSweep(context.Background(), deps, r.invocation, SweepScopeFull)

	if got := ghLogLines(t, ghLog); len(got) != 0 {
		t.Fatalf("zero PR numbers must spawn zero gh processes, got %d:\n%s", len(got), strings.Join(got, "\n"))
	}
	if len(res.Entries) != 2 {
		t.Fatalf("want two historical entries, got %d", len(res.Entries))
	}
}

// TestIntegrationSweepBatchesPRRequestsOncePer25 pins the batch transport shape at
// the real gh boundary across population sizes: the identity resolves exactly once
// and the PR reads are ceil(P/25) processes — never one per change.
func TestIntegrationSweepBatchesPRRequestsOncePer25(t *testing.T) {
	requireRealGit(t)
	for _, tc := range []struct {
		pop         int
		wantBatches int
	}{{25, 1}, {26, 2}, {60, 3}} {
		tc := tc
		t.Run(fmt.Sprintf("pop%d", tc.pop), func(t *testing.T) {
			records := map[string]string{}
			for i := 0; i < tc.pop; i++ {
				id := 300 + i
				records[fmt.Sprintf("docs/changes/active/%04d-p.md", id)] = trafficImplementedOpen(id, fmt.Sprintf("p%03d", i))
			}
			r := trafficRepo(t, "", records)
			ghLog := filepath.Join(t.TempDir(), "gh.log")
			deps := trafficDeps(t, trafficGit(t, filepath.Join(t.TempDir(), "git.log"), ""), trafficGH(t, ghLog))

			MaintenanceSweep(context.Background(), deps, r.invocation, SweepScopeFull)

			ghl := ghLogLines(t, ghLog)
			if got := countPrefix(ghl, "repo view"); got != 1 {
				t.Errorf("identity resolved %d times, want exactly 1", got)
			}
			if got := countPrefix(ghl, "api graphql"); got != tc.wantBatches {
				t.Errorf("api graphql invocations = %d, want %d (ceil(%d/25))\n%s", got, tc.wantBatches, tc.pop, strings.Join(ghl, "\n"))
			}
		})
	}
}

// --- Step 3: scaling + scope ----------------------------------------------

// remoteCounts is the categorized remote-call profile of one sweep, the invariant
// the scaling fixture holds constant across historical populations.
type remoteCounts struct {
	setupProbe  int // ls-remote --symref … HEAD
	sharedHeads int // ls-remote --heads
	fetches     int // fetch …
	featureRef  int // forbidden per-item remote-ref probes
	repoView    int // gh identity
	graphql     int // gh batches
}

func sweepRemoteCounts(t *testing.T, gitLog, ghLog string) remoteCounts {
	t.Helper()
	gl := gitLogLines(t, gitLog)
	ghl := ghLogLines(t, ghLog)
	return remoteCounts{
		setupProbe:  countMatching(gl, "ls-remote", "--symref"),
		sharedHeads: countMatching(gl, "ls-remote", "--heads"),
		fetches:     countMatching(gl, "fetch"),
		featureRef:  len(featureRefProbes(gl)),
		repoView:    countPrefix(ghl, "repo view"),
		graphql:     countPrefix(ghl, "api graphql"),
	}
}

// TestIntegrationSweepAssessmentTrafficConstantAcrossHistory is the scaling proof:
// growing the terminal (done) population 1 -> 25 -> 250 at fixed active/pending
// work must NOT change any remote-call count. The assessment reasons from ONE
// shared advertisement and ONE worktree list plus local per-record inspection, so
// its remote footprint is history-independent; a reintroduced per-history refresh
// or per-ref probe would make one of these counts grow with the population.
func TestIntegrationSweepAssessmentTrafficConstantAcrossHistory(t *testing.T) {
	requireRealGit(t)
	measure := func(t *testing.T, history int) (remoteCounts, MaintenanceResult) {
		t.Helper()
		records := map[string]string{}
		for i := 0; i < history; i++ {
			id := 1000 + i
			records[fmt.Sprintf("docs/changes/active/%04d-h.md", id)] = trafficDoneRecord(id, fmt.Sprintf("h%04d", i))
		}
		r := trafficRepo(t, "", records)
		gitLog := filepath.Join(t.TempDir(), "git.log")
		ghLog := filepath.Join(t.TempDir(), "gh.log")
		deps := trafficDeps(t, trafficGit(t, gitLog, ""), trafficGH(t, ghLog))
		res := MaintenanceSweep(context.Background(), deps, r.invocation, SweepScopeFull)
		return sweepRemoteCounts(t, gitLog, ghLog), res
	}

	base, baseRes := measure(t, 1)
	// One truthful entry per full-scope candidate at the base population.
	if len(baseRes.Entries) != 1 {
		t.Fatalf("history=1 produced %d entries, want exactly 1 per candidate", len(baseRes.Entries))
	}
	// Sanity: the base actually exercised the shared inventory (done candidates
	// present), else the invariant below would be vacuously true.
	if base.sharedHeads != 1 {
		t.Fatalf("history=1 shared-inventory count = %d, want 1", base.sharedHeads)
	}
	for _, history := range []int{25, 250} {
		got, res := measure(t, history)
		if got != base {
			t.Errorf("history=%d amplified remote work: got %+v, base %+v", history, got, base)
		}
		if len(res.Entries) != history {
			t.Errorf("history=%d produced %d entries, want one truthful entry per candidate", history, len(res.Entries))
		}
	}
}

// TestIntegrationSweepImplementationScopeInspectsNoDeferredResources: implementation
// scope defers every already-terminal record as an unprobed COUNT — it never gathers
// the shared inventory (no `ls-remote --heads`), fetches no deferred remote heads,
// and reads no per-item deferred resource — while full scope on the SAME corpus DOES
// gather the shared inventory and reports one entry per candidate. This proves the
// deferral is scope-keyed, not dead.
func TestIntegrationSweepImplementationScopeInspectsNoDeferredResources(t *testing.T) {
	requireRealGit(t)
	records := map[string]string{}
	for i := 0; i < 5; i++ {
		id := 400 + i
		records[fmt.Sprintf("docs/changes/active/%04d-h.md", id)] = trafficDoneRecord(id, fmt.Sprintf("h%02d", i))
	}
	build := func() (*gitRepo, string, string, FinalizeDeps) {
		r := trafficRepo(t, "", records)
		gitLog := filepath.Join(t.TempDir(), "git.log")
		ghLog := filepath.Join(t.TempDir(), "gh.log")
		return r, gitLog, ghLog, trafficDeps(t, trafficGit(t, gitLog, ""), trafficGH(t, ghLog))
	}

	// Implementation scope: no shared inventory, no per-item probe, all deferred.
	rI, giImpl, ghImpl, depsI := build()
	resI := MaintenanceSweep(context.Background(), depsI, rI.invocation, SweepScopeImplementation)
	ci := sweepRemoteCounts(t, giImpl, ghImpl)
	if ci.sharedHeads != 0 {
		t.Errorf("implementation scope gathered the shared inventory %d time(s), want 0 (nothing assessed)", ci.sharedHeads)
	}
	if ci.featureRef != 0 {
		t.Errorf("implementation scope issued %d per-item remote-ref probe(s), want 0", ci.featureRef)
	}
	if resI.DeferredHistoricalCleanups != 5 {
		t.Errorf("deferred_historical_cleanups = %d, want 5", resI.DeferredHistoricalCleanups)
	}
	for _, e := range resI.Entries {
		if e.ID >= 400 && e.ID < 500 {
			t.Errorf("implementation scope must render no per-item historical entry, got %+v", e)
		}
	}

	// Full scope on the same corpus DOES assess: one shared inventory, one entry
	// per candidate, zero deferred.
	rF, giFull, ghFull, depsF := build()
	resF := MaintenanceSweep(context.Background(), depsF, rF.invocation, SweepScopeFull)
	cf := sweepRemoteCounts(t, giFull, ghFull)
	if cf.sharedHeads != 1 {
		t.Errorf("full scope shared-inventory count = %d, want 1", cf.sharedHeads)
	}
	if resF.DeferredHistoricalCleanups != 0 {
		t.Errorf("full scope deferred = %d, want 0", resF.DeferredHistoricalCleanups)
	}
	if len(resF.Entries) != 5 {
		t.Errorf("full scope produced %d entries, want one per candidate (5)", len(resF.Entries))
	}
}

// --- Step 1 (dispatched op): reclaim keeps setup once ---------------------

// TestIntegrationSweepDispatchedReclaimKeepsSetupOnce drives a REAL dispatched
// mutation (an expired-lease reclaim, reclaim.auto on) end-to-end and proves the
// invocation still runs full setup exactly ONCE: one default-branch probe and no
// shared inventory (a reclaim is not a done candidate). The reclaim dispatch runs
// the workspace inspection and the transaction's own fresh-origin CAS re-read — so
// this also witnesses that a dispatched operation adds no setup re-probe. The
// bytes-level "one preparation fetch shared by nested readers" property is proved
// at the session seam (sweep_session_test.go); see this file's header note.
func TestIntegrationSweepDispatchedReclaimKeepsSetupOnce(t *testing.T) {
	requireRealGit(t)
	records := map[string]string{
		// An in-progress record whose claim (2026-08-02) is well before the fixed
		// test clock (2026-08-16); a 12-hour lease is strictly expired.
		"docs/changes/active/0050-stale.md": lifecycleChange(50, "stale", "in-progress"),
	}
	r := trafficRepo(t, "integration_branch: main\nreclaim:\n  lease_ttl: 12\n  auto: true\n", records)
	gitLog := filepath.Join(t.TempDir(), "git.log")
	deps := trafficDeps(t, trafficGit(t, gitLog, ""), trafficGH(t, filepath.Join(t.TempDir(), "gh.log")))

	res := MaintenanceSweep(context.Background(), deps, r.invocation, SweepScopeFull)

	// The reclaim was actually dispatched and applied (WorkspaceInspect ran).
	var reclaim *MaintenanceEntry
	for i := range res.Entries {
		if res.Entries[i].ID == 50 && res.Entries[i].Kind == sweepKindReclaim {
			reclaim = &res.Entries[i]
		}
	}
	if reclaim == nil {
		t.Fatalf("reclaim was not dispatched for the expired record; entries=%+v", res.Entries)
	}
	if reclaim.Disposition != SweepDispApplied || reclaim.Operation != OperationChangeReclaim {
		t.Fatalf("reclaim entry = %+v, want an applied change.reclaim dispatch", *reclaim)
	}
	gl := gitLogLines(t, gitLog)
	// Full setup ran once even across the dispatched mutation.
	if got := countMatching(gl, "ls-remote", "--symref"); got != 1 {
		t.Errorf("setup default-branch probe ran %d times, want exactly 1 across a dispatched op", got)
	}
	// A reclaim is not a done candidate: no snapshot assessment, no shared inventory.
	if got := countMatching(gl, "ls-remote", "--heads"); got != 0 {
		t.Errorf("a reclaim-only sweep must gather no shared inventory, got %d ls-remote --heads", got)
	}
	// NOTE: a dispatched reclaim's own feature-branch check (`ls-remote …
	// refs/heads/feat/stale`) is REQUIRED operation-specific proof, not a forbidden
	// per-HISTORY assessment probe — so featureRefProbes is deliberately NOT asserted
	// zero here. The forbidden-per-item-probe invariant is asserted on the
	// non-dispatched assessment path (discovery/scaling/unknown-advertisement tests),
	// where any feature-ref ls-remote would be a per-history fan-out.
}

// --- Step 2: blocked-probe / required-access regressions ------------------

// TestIntegrationSweepBlockedMetadataFetchRefusesNoMutation: blocking the ALLOWED
// metadata-branch fetch fails the initial pin, so the whole sweep refuses with the
// adapter's typed external failure and dispatches NO mutation — a bounded return
// (well under an independent watchdog), never a hang.
func TestIntegrationSweepBlockedMetadataFetchRefusesNoMutation(t *testing.T) {
	requireRealGit(t)
	records := map[string]string{"docs/changes/active/0041-a.md": trafficDoneRecord(41, "a")}
	r := trafficRepo(t, "", records)
	gitLog := filepath.Join(t.TempDir(), "git.log")
	// Fail-fast any fetch of the metadata branch.
	fault := `case "$*" in *fetch*refs/heads/docket*) echo "metadata fetch blocked" >&2; exit 1;; esac`
	deps := trafficDeps(t, trafficGit(t, gitLog, fault), trafficGH(t, filepath.Join(t.TempDir(), "gh.log")))

	done := make(chan MaintenanceResult, 1)
	go func() {
		done <- MaintenanceSweep(context.Background(), deps, r.invocation, SweepScopeFull)
	}()
	var res MaintenanceResult
	select {
	case res = <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("sweep did not return within the watchdog after a blocked metadata fetch (hang)")
	}

	if res.Result == ResultApplied || res.Result == ResultNoOp {
		t.Fatalf("a blocked metadata fetch must refuse the whole sweep, got %q", res.Result)
	}
	if len(res.Entries) != 0 {
		t.Fatalf("a whole-sweep refusal carries no partial entries, got %d", len(res.Entries))
	}
}

// TestIntegrationSweepFailFastGuardStaysGreenUnderProduction wires a fail-fast git
// that IMMEDIATELY exits non-zero on a SECOND setup default-branch probe — a
// redundant setup re-probe. Production issues exactly one such probe, so the sweep
// completes normally with its expected non-actionable entry and the guard never
// trips (zero forbidden-probe attempts). This is the phase-aware harness a mutation
// that reintroduces a setup re-probe would trip (fail fast, never hang: learning
// mutation-target-needs-a-forced-exit).
func TestIntegrationSweepFailFastGuardStaysGreenUnderProduction(t *testing.T) {
	requireRealGit(t)
	records := map[string]string{"docs/changes/active/0041-a.md": trafficDoneRecord(41, "a")}
	r := trafficRepo(t, "", records)
	gitLog := filepath.Join(t.TempDir(), "git.log")
	// A second `ls-remote --symref` (redundant setup probe) exits fast, non-zero.
	fault := `case "$*" in *"ls-remote --symref"*) n=$(grep -c -- "ls-remote --symref" '` + gitLog + `'); if [ "$n" -gt 1 ]; then echo "redundant setup probe forbidden" >&2; exit 47; fi;; esac`
	deps := trafficDeps(t, trafficGit(t, gitLog, fault), trafficGH(t, filepath.Join(t.TempDir(), "gh.log")))

	done := make(chan MaintenanceResult, 1)
	go func() {
		done <- MaintenanceSweep(context.Background(), deps, r.invocation, SweepScopeFull)
	}()
	var res MaintenanceResult
	select {
	case res = <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("sweep did not return within the watchdog (hang)")
	}

	// Production tripped nothing: exactly one setup probe, terminal success, and the
	// expected pinned entry.
	if got := countMatching(gitLogLines(t, gitLog), "ls-remote", "--symref"); got != 1 {
		t.Fatalf("setup default-branch probe ran %d times, want exactly 1 (the fail-fast guard should not have fired)", got)
	}
	if res.Result != ResultNoOp {
		t.Fatalf("result = %q, want no-op; a tripped guard would have surfaced a refusal", res.Result)
	}
	if len(res.Entries) != 1 || !nonActionable(res.Entries[0]) {
		t.Fatalf("want exactly one non-actionable historical entry, got %+v", res.Entries)
	}
}

// TestIntegrationSweepUnknownRemoteAdvertisementIsNotCleanAbsence: a failed shared
// `ls-remote --heads` makes the remote-ref leg UNKNOWN — a ref's absence is
// unprovable this invocation — so the done record is snapshot-unknown, never a
// clean no-op laundered from a failed advertisement (learning
// probe-error-is-not-clean-absence). No per-ref probe is issued to compensate.
func TestIntegrationSweepUnknownRemoteAdvertisementIsNotCleanAbsence(t *testing.T) {
	requireRealGit(t)
	records := map[string]string{"docs/changes/active/0041-a.md": trafficDoneRecord(41, "a")}
	r := trafficRepo(t, "", records)
	gitLog := filepath.Join(t.TempDir(), "git.log")
	fault := `case "$*" in *"ls-remote --heads"*) echo "advertisement unavailable" >&2; exit 3;; esac`
	deps := trafficDeps(t, trafficGit(t, gitLog, fault), trafficGH(t, filepath.Join(t.TempDir(), "gh.log")))

	res := MaintenanceSweep(context.Background(), deps, r.invocation, SweepScopeFull)

	if len(res.Entries) != 1 {
		t.Fatalf("want one historical entry, got %d: %+v", len(res.Entries), res.Entries)
	}
	e := res.Entries[0]
	if e.Disposition != SweepDispUnknown || e.Reason != ReasonSweepSnapshotUnknown {
		t.Fatalf("a failed shared advertisement must yield snapshot-unknown, got %+v", e)
	}
	// The failed advertisement did NOT fan out into a per-ref probe.
	if probes := featureRefProbes(gitLogLines(t, gitLog)); len(probes) != 0 {
		t.Errorf("a failed advertisement fanned out into %d per-ref probe(s):\n%s", len(probes), strings.Join(probes, "\n"))
	}
}

// --- Step 5: second invocation / second repository ------------------------

// TestIntegrationSweepSecondInvocationAndRepositoryReprobes: a fresh sweep is a
// fresh observation lifetime. Two sequential invocations over the same repository
// each run the full setup (the first invocation's captured setup never leaks into
// the second), and a sweep over a DIFFERENT repository resolves that repository's
// own identity — no observation reuse escapes its declared single-invocation
// lifetime.
func TestIntegrationSweepSecondInvocationAndRepositoryReprobes(t *testing.T) {
	requireRealGit(t)
	recordsA := map[string]string{"docs/changes/active/0041-a.md": trafficDoneRecord(41, "a")}
	rA := trafficRepo(t, "", recordsA)

	run := func(r *gitRepo) remoteCounts {
		gitLog := filepath.Join(t.TempDir(), "git.log")
		ghLog := filepath.Join(t.TempDir(), "gh.log")
		deps := trafficDeps(t, trafficGit(t, gitLog, ""), trafficGH(t, ghLog))
		MaintenanceSweep(context.Background(), deps, r.invocation, SweepScopeFull)
		return sweepRemoteCounts(t, gitLog, ghLog)
	}

	first := run(rA)
	second := run(rA)
	if first.setupProbe != 1 || second.setupProbe != 1 {
		t.Errorf("each invocation must run full setup once: first=%d second=%d probes", first.setupProbe, second.setupProbe)
	}
	if first.sharedHeads != 1 || second.sharedHeads != 1 {
		t.Errorf("each invocation gathers its own shared inventory: first=%d second=%d", first.sharedHeads, second.sharedHeads)
	}

	// A second, independent repository re-runs the whole setup for itself.
	rB := trafficRepo(t, "", map[string]string{"docs/changes/active/0060-b.md": trafficDoneRecord(60, "b")})
	other := run(rB)
	if other.setupProbe != 1 || other.sharedHeads != 1 {
		t.Errorf("a second repository must run its own full setup, got %+v", other)
	}
}
