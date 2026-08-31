package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
)

// These tests pin the sweep session's one-observation-per-attempt contract: each
// Prepare is exactly one fresh metadata fetch (never a setup re-probe), it always
// observes the current remote tip (never a supplied stale one), a failed fetch is
// a classified error (never a stale fallback or inferred absence), the bound
// reader serves the observation without any network of its own, and the session
// refuses a repoDir naming a different repository.

// countingGitClient builds a production gitcli client whose executable is a
// wrapper script that appends every invocation's argument line to logPath before
// delegating to the real git. It is the counting probe the network-shape asserts
// read: `fetch`/`ls-remote` lines are the only network git processes docket runs.
func countingGitClient(t *testing.T, logPath string) *gitcli.Client {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	dir := t.TempDir()
	wrapper := filepath.Join(dir, "git")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '" + logPath + "'\nexec '" + realGit + "' \"$@\"\n"
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

// gitLogLines returns the recorded git argument lines, oldest first.
func gitLogLines(t *testing.T, logPath string) []string {
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

// countMatching counts recorded git lines containing every one of subs.
func countMatching(lines []string, subs ...string) int {
	n := 0
	for _, l := range lines {
		all := true
		for _, s := range subs {
			if !strings.Contains(l, s) {
				all = false
				break
			}
		}
		if all {
			n++
		}
	}
	return n
}

// sessionUnderTest builds a session bound to r.invocation over client, plus the
// live gitStatusReader (already pinned) the bound reader delegates to.
func sessionUnderTest(t *testing.T, client *gitcli.Client, invocation string) (*sweepSession, StatusReader) {
	t.Helper()
	ctx := context.Background()
	repo, err := client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: invocation})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	reader := NewGitStatusReader(client)
	base, err := reader.PinContext(ctx, invocation)
	if err != nil {
		t.Fatalf("PinContext: %v", err)
	}
	return newSweepSession(client, repo, base), reader
}

func TestPrepareIsOneMetadataFetchZeroSetupProbes(t *testing.T) {
	requireRealGit(t)
	records := map[string]string{
		"docs/changes/active/0001-alpha.md": changeRecord(1, "alpha", "Alpha"),
	}
	r := newDocketModeRepo(t, nil, records)
	logPath := filepath.Join(t.TempDir(), "git.log")
	client := countingGitClient(t, logPath)
	session, _ := sessionUnderTest(t, client, r.invocation)

	// Discard every setup git process; only Prepare's own network shape is asserted.
	if err := os.WriteFile(logPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	obs, err := session.Prepare(context.Background())
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if obs == nil {
		t.Fatal("Prepare returned a nil observation")
	}

	lines := gitLogLines(t, logPath)
	if got := countMatching(lines, "fetch", "refs/heads/docket"); got != 1 {
		t.Fatalf("Prepare ran %d docket fetches, want exactly 1\nlog:\n%s", got, strings.Join(lines, "\n"))
	}
	if got := countMatching(lines, "fetch"); got != 1 {
		t.Fatalf("Prepare ran %d fetches total, want exactly 1 (no default/integration re-fetch)\nlog:\n%s", got, strings.Join(lines, "\n"))
	}
	if got := countMatching(lines, "ls-remote"); got != 0 {
		t.Fatalf("Prepare ran %d ls-remote probes, want 0 (no setup/topology re-probe)\nlog:\n%s", got, strings.Join(lines, "\n"))
	}
}

func TestPrepareObservesFreshMetadataTip(t *testing.T) {
	requireRealGit(t)
	records := map[string]string{
		"docs/changes/active/0001-alpha.md": changeRecord(1, "alpha", "Alpha"),
	}
	r := newDocketModeRepo(t, nil, records)
	client := newGitClient(t)
	session, _ := sessionUnderTest(t, client, r.invocation)

	first, err := session.Prepare(context.Background())
	if err != nil {
		t.Fatalf("first Prepare: %v", err)
	}

	// An independent writer advances the docket branch between attempts.
	newTip := r.writerAdvance(t, "docket", map[string]string{
		"docs/changes/active/0002-beta.md": changeRecord(2, "beta", "Beta"),
	})

	second, err := session.Prepare(context.Background())
	if err != nil {
		t.Fatalf("second Prepare: %v", err)
	}

	if second.pin.MetadataRevision == first.pin.MetadataRevision {
		t.Fatal("second observation reused the first attempt's metadata revision; a supplied tip stood in for a fresh fetch")
	}
	if second.pin.MetadataRevision != newTip {
		t.Fatalf("second observation revision = %q, want the advanced origin tip %q", second.pin.MetadataRevision, newTip)
	}
	if blobsHavePath(first.blobs, "docs/changes/active/0002-beta.md") {
		t.Fatal("first observation already carried the not-yet-written record")
	}
	if !blobsHavePath(second.blobs, "docs/changes/active/0002-beta.md") {
		t.Fatalf("second observation missing the freshly written record\npaths: %v", pathsOf(second.blobs))
	}
}

func TestPrepareFailedFetchIsErrorNeverStaleFallback(t *testing.T) {
	requireRealGit(t)
	records := map[string]string{
		"docs/changes/active/0001-alpha.md": changeRecord(1, "alpha", "Alpha"),
	}
	r := newDocketModeRepo(t, nil, records)
	client := newGitClient(t)
	session, _ := sessionUnderTest(t, client, r.invocation)

	// Delete the remote metadata branch: the fetch can no longer resolve a tip.
	runGit(t, r.origin, "update-ref", "-d", "refs/heads/docket")

	obs, err := session.Prepare(context.Background())
	if err == nil {
		t.Fatalf("Prepare succeeded after the remote docket branch was deleted; obs=%+v (stale fallback or inferred absence)", obs)
	}
	if obs != nil {
		t.Fatalf("Prepare returned a non-nil observation alongside its error: %+v", obs)
	}
}

func TestBoundReaderNeverFetches(t *testing.T) {
	requireRealGit(t)
	records := map[string]string{
		"docs/changes/active/0001-alpha.md": changeRecord(1, "alpha", "Alpha"),
	}
	r := newDocketModeRepo(t, nil, records)
	logPath := filepath.Join(t.TempDir(), "git.log")
	client := countingGitClient(t, logPath)
	session, live := sessionUnderTest(t, client, r.invocation)

	obs, err := session.Prepare(context.Background())
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	bound := newBoundStatusReader(obs, live)

	// Discard setup + Prepare git processes; the bound PinContext's only git is
	// the same-repository guard's local discovery, which is never network.
	if err := os.WriteFile(logPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	gotPin, err := bound.PinContext(context.Background(), r.invocation)
	if err != nil {
		t.Fatalf("bound PinContext: %v", err)
	}
	if !reflect.DeepEqual(gotPin, obs.pin) {
		t.Fatalf("bound PinContext returned %+v, want the observation pin %+v", gotPin, obs.pin)
	}
	pinLines := gitLogLines(t, logPath)
	if got := countMatching(pinLines, "fetch"); got != 0 {
		t.Fatalf("bound PinContext ran %d fetches, want 0\nlog:\n%s", got, strings.Join(pinLines, "\n"))
	}
	if got := countMatching(pinLines, "ls-remote"); got != 0 {
		t.Fatalf("bound PinContext ran %d ls-remote probes, want 0\nlog:\n%s", got, strings.Join(pinLines, "\n"))
	}

	// The bound corpus read serves from the observation: it must spawn NO git
	// process of any kind, not merely no network one — a local re-read at the
	// same immutable revision would return identical bytes and slip a
	// network-only count.
	if err := os.WriteFile(logPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	gotBlobs, err := bound.ReadCorpus(context.Background(), gotPin)
	if err != nil {
		t.Fatalf("bound ReadCorpus: %v", err)
	}
	if !reflect.DeepEqual(gotBlobs, obs.blobs) {
		t.Fatalf("bound ReadCorpus returned a different corpus than the observation")
	}
	if corpusLines := gitLogLines(t, logPath); len(corpusLines) != 0 {
		t.Fatalf("bound ReadCorpus ran %d git processes, want 0 (it must serve the observation from memory)\nlog:\n%s",
			len(corpusLines), strings.Join(corpusLines, "\n"))
	}
}

func TestSessionRefusesDifferentRepository(t *testing.T) {
	requireRealGit(t)
	records := map[string]string{
		"docs/changes/active/0001-alpha.md": changeRecord(1, "alpha", "Alpha"),
	}
	repoA := newDocketModeRepo(t, nil, records)
	repoB := newDocketModeRepo(t, nil, records)

	client := newGitClient(t)
	session, live := sessionUnderTest(t, client, repoA.invocation)
	obs, err := session.Prepare(context.Background())
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	bound := newBoundStatusReader(obs, live)

	// The bound repository still resolves the observation pin.
	if _, err := bound.PinContext(context.Background(), repoA.invocation); err != nil {
		t.Fatalf("bound PinContext on the original repository errored: %v", err)
	}
	// A repoDir naming a different repository must error, never silently reuse
	// the captured facts.
	if _, err := bound.PinContext(context.Background(), repoB.invocation); err == nil {
		t.Fatal("bound PinContext accepted a repoDir naming a different repository")
	}
}

// blobsHavePath reports whether blobs carry a record at the given repo-relative
// path.
func blobsHavePath(blobs []StatusBlob, p string) bool {
	for _, b := range blobs {
		if b.Path == p {
			return true
		}
	}
	return false
}
