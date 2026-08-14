package app

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/harness"
	"github.com/danielhanold/docket/internal/install"
)

// TestInstallResultClassification pins the whole reason → result table. The
// vocabulary is closed on both sides, so every row here is a protocol
// statement: a consumer keys on `result` for what to do and on `reason` for
// why, and moving a reason between results is a protocol change.
func TestInstallResultClassification(t *testing.T) {
	cases := []struct {
		reason string
		want   Result
	}{
		// Success has no reason at all; Applied splits applied from no-op.
		{"", ResultNoOp},

		// The world is not in a state the operation can act on.
		{install.ReasonNoHarnessDetected, ResultInvalidState},
		{install.ReasonOwnershipConflict, ResultInvalidState},
		{install.ReasonManagedBlockInvalid, ResultInvalidState},
		{install.ReasonInstallationRequired, ResultInvalidState},
		{install.ReasonInstallationDrift, ResultInvalidState},
		{install.ReasonTransactionRecoveryRequired, ResultInvalidState},
		{install.ReasonAssetProtocolMismatch, ResultInvalidState},
		{install.ReasonSourceAssetsDrifted, ResultInvalidState},
		{install.ReasonStateInvalid, ResultInvalidState},
		{install.ReasonAssetManifestInvalid, ResultInvalidState},
		// Another docket process holds the installation lock. Nothing is wrong
		// with the user's input or their filesystem — the world is momentarily
		// not one this operation may act on, and the answer is to run again.
		{install.ReasonInstallInProgress, ResultInvalidState},

		// The configuration asks for behavior docket does not ship.
		{install.ReasonDeferredCapability, ResultUnsupportedConfig},

		// The caller said something docket cannot act on.
		{install.ReasonUnknownHarness, ResultInvalidInput},
		{install.ReasonInvalidOptions, ResultInvalidInput},
		{install.ReasonInvalidSourceRoot, ResultInvalidInput},
		{ReasonInvalidConfig, ResultInvalidInput},

		// Something outside docket failed.
		{install.ReasonBuildFailed, ResultExternalFailed},
		{install.ReasonFilesystemFailed, ResultExternalFailed},

		// Docket's own defect, and the fail-closed default for a reason no
		// classifier row names.
		{install.ReasonInternal, ResultInternalError},
		{"a-reason-nobody-classified", ResultInternalError},
	}

	for _, tc := range cases {
		name := tc.reason
		if name == "" {
			name = "(no reason)"
		}
		t.Run(name, func(t *testing.T) {
			out := install.Outcome{Reason: tc.reason}
			if tc.reason != "" {
				out.Err = errors.New("boom")
			}
			got := NewInstallResult("install", out)
			if got.Result != tc.want {
				t.Errorf("result = %q, want %q", got.Result, tc.want)
			}
			if got.Reason != tc.reason {
				t.Errorf("reason = %q, want %q", got.Reason, tc.reason)
			}
			if tc.reason != "" && got.Message != "boom" {
				t.Errorf("message = %q, want the error text", got.Message)
			}
		})
	}

	applied := NewInstallResult("install", install.Outcome{Applied: true})
	if applied.Result != ResultApplied || !applied.AppliedWork {
		t.Errorf("applied outcome = %q/%v, want applied/true", applied.Result, applied.AppliedWork)
	}
}

// TestEveryInstallReasonClassified derives the reason vocabulary from the
// service's own source rather than restating it: a reason added there and not
// classified here would otherwise fall into the internal-error default and
// report docket's bug for the user's situation. The scan is deliberately
// shape-keyed (a `Reason<Name> = "<literal>"` constant), so a new spelling is
// picked up without editing this test.
func TestEveryInstallReasonClassified(t *testing.T) {
	re := regexp.MustCompile(`(?m)^\s*Reason[A-Za-z]+\s*=\s*"([^"]+)"`)
	found := map[string]bool{}
	for _, name := range []string{"service.go", "target.go", "devmode.go"} {
		body, err := os.ReadFile(filepath.Join("..", "install", name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		for _, m := range re.FindAllStringSubmatch(string(body), -1) {
			found[m[1]] = true
		}
	}
	if len(found) < 10 {
		// The scan matching nothing would make this guard vacuous.
		t.Fatalf("scan found only %d reasons, expected the service's full vocabulary", len(found))
	}
	for reason := range found {
		if reason == install.ReasonInternal {
			continue // the one reason whose classification IS internal-error
		}
		got := NewInstallResult("install", install.Outcome{Reason: reason, Err: errors.New("x")})
		if got.Result == ResultInternalError {
			t.Errorf("reason %q classifies as internal-error; add it to the classifier", reason)
		}
	}
}

func TestInstallHumanText(t *testing.T) {
	out := install.Outcome{
		Applied:       true,
		Mode:          install.ModeRelease,
		Harnesses:     []string{"claude", "codex"},
		AssetProtocol: 1,
		AssetSetID:    "sha256:abc",
		StatePath:     "/home/u/state/install.json",
		Actions: []install.Action{
			{Op: install.OpCreate, Path: "/home/u/.claude/agents/docket-adr.md", Detail: "claude"},
		},
	}
	text := NewInstallResult("install", out).HumanText()
	for _, want := range []string{
		"install: applied",
		"mode: release",
		"harnesses: claude, codex",
		"sha256:abc",
		"/home/u/state/install.json",
		"create",
		"/home/u/.claude/agents/docket-adr.md",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("human text missing %q:\n%s", want, text)
		}
	}
	if strings.HasSuffix(text, "\n") {
		t.Errorf("human text ends with a newline; the presenter adds it:\n%q", text)
	}

	failed := NewInstallResult("install.check", install.Outcome{
		Reason: install.ReasonInstallationRequired,
		Err:    errors.New("install: no installation record"),
	}).HumanText()
	if !strings.Contains(failed, "reason: installation-required") ||
		!strings.Contains(failed, "install.check: invalid-state") {
		t.Errorf("failure text = %q", failed)
	}
}

// TestInstallHumanTextConflictCarriesRemedy pins the one output a conflicted
// install leaves a person with. There is no --force, so the human text is the
// whole way forward: the path, the stable reason, and the target-specific
// instruction the service composed into the action's detail.
func TestInstallHumanTextConflictCarriesRemedy(t *testing.T) {
	const path = "/home/u/.claude/agents/docket-status.md"
	text := NewInstallResult("install", install.Outcome{
		Reason: install.ReasonOwnershipConflict,
		Err:    errors.New("install: 1 target(s) are not provably docket's"),
		Actions: []install.Action{{
			Op:     install.OpConflict,
			Path:   path,
			Detail: install.ReasonOwnershipConflict + ": docket did not write this file; move or delete it, then re-run",
		}},
	}).HumanText()
	for _, want := range []string{
		"install: invalid-state",
		install.OpConflict,
		path,
		"move or delete it, then re-run",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("conflict human text missing %q:\n%s", want, text)
		}
	}
}

// TestInstallHumanTextLegacyNoteOnlyOnOwnershipConflict pins the one aggregate
// sentence about unadopted legacy installs to the outcome it explains. It is
// not a per-target remedy, so it must appear exactly once on an
// ownership-conflict outcome and never on any other.
func TestInstallHumanTextLegacyNoteOnlyOnOwnershipConflict(t *testing.T) {
	const fragment = "legacy Bash installer"

	conflict := NewInstallResult("install", install.Outcome{
		Reason: install.ReasonOwnershipConflict,
		Err:    errors.New("install: 1 target(s) are not provably docket's"),
	}).HumanText()
	if n := strings.Count(conflict, fragment); n != 1 {
		t.Errorf("legacy note appears %d time(s), want 1:\n%s", n, conflict)
	}
	if !strings.Contains(conflict, "not yet adopted automatically") ||
		!strings.Contains(conflict, "move them aside") {
		t.Errorf("legacy note text = %q", conflict)
	}

	for _, reason := range []string{
		"",
		install.ReasonInstallationRequired,
		install.ReasonManagedBlockInvalid,
	} {
		other := NewInstallResult("install", install.Outcome{
			Reason: reason,
			Err:    errors.New("install: boom"),
		}).HumanText()
		if strings.Contains(other, fragment) {
			t.Errorf("legacy note leaked into reason %q:\n%s", reason, other)
		}
	}
}

// TestPlannersCoverHarnessOrder ties the adapters this layer wires to the
// package that owns the order, so a fifth harness cannot be shipped in
// internal/harness and silently never installed.
func TestPlannersCoverHarnessOrder(t *testing.T) {
	planners := Planners(install.UserRoots{Home: "/home/u"}, nil)
	var names []string
	for _, p := range planners {
		names = append(names, p.Name)
		if p.Detect == nil || p.Plan == nil {
			t.Errorf("planner %q is missing a seam", p.Name)
		}
	}
	if strings.Join(names, ",") != strings.Join(harness.Order, ",") {
		t.Fatalf("planner names = %v, want %v", names, harness.Order)
	}
}

func TestPlannersPlanAndDetect(t *testing.T) {
	catalog, err := assets.EmbeddedCatalog()
	if err != nil {
		t.Fatalf("embedded catalog: %v", err)
	}
	// filepath.Clean, and not a bare t.TempDir(): t.TempDir() hands back
	// $TMPDIR's spelling verbatim (os.TempDir strips trailing slashes and
	// nothing else), so under a $TMPDIR carrying an interior "//" — which
	// scripts/run-tests.sh produces, since macOS's default TMPDIR ends in "/"
	// and the runner appends "/run-tests.XXXXXX" to it — the fake home is not
	// lexically clean. Every planned path comes out of filepath.Join, which
	// cleans, so the containment check below would compare a cleaned target
	// against an uncleaned root and report every path as an escape. Cleaning
	// puts this fixture on production's footing: UserRoots is only ever built
	// by install.ResolveRoots, whose Home is cleaned.
	//
	// NOT filepath.EvalSymlinks: on macOS /var is a symlink to /private/var, so
	// resolving would move the root to a spelling the planners never produce.
	home := filepath.Clean(t.TempDir())
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	roots := install.UserRoots{Home: home, ConfigHome: filepath.Join(home, ".config")}

	for _, p := range Planners(roots, nil) {
		targets, err := p.Plan(install.ModeRelease, "/assets", catalog)
		if err != nil {
			t.Fatalf("%s: plan: %v", p.Name, err)
		}
		if len(targets) == 0 {
			t.Fatalf("%s planned nothing", p.Name)
		}
		for _, target := range targets {
			// The separator is part of the prefix: without it a sibling root
			// whose name merely starts with the fake home's ("<home>-other")
			// would read as contained.
			if !strings.HasPrefix(target.Path, home+string(filepath.Separator)) {
				t.Errorf("%s planned %s, outside the fake home", p.Name, target.Path)
			}
		}
		present, _ := p.Detect(roots)
		if want := p.Name == "claude"; present != want {
			t.Errorf("%s detect = %v, want %v", p.Name, present, want)
		}
	}
}

// TestAgentDigestIdentifiesTheTable proves the digest is a function of the
// resolved settings the plan was rendered from — stable across calls, and
// sensitive to a pin change in a selected harness.
func TestAgentDigestIdentifiesTheTable(t *testing.T) {
	table := func(model string) config.AgentsTable {
		return config.AgentsTable{
			"claude": {"build-standard": {
				Model:  config.Value[string]{Value: model},
				Effort: config.Value[string]{Value: "high"},
			}},
			"codex": {"build-standard": {Model: config.Value[string]{Value: "gpt"}}},
		}
	}
	base, err := AgentDigest(table("opus"), []string{"claude", "codex"})
	if err != nil {
		t.Fatal(err)
	}
	again, err := AgentDigest(table("opus"), []string{"claude", "codex"})
	if err != nil || again != base {
		t.Fatalf("digest is not stable: %q vs %q (%v)", base, again, err)
	}
	changed, err := AgentDigest(table("sonnet"), []string{"claude", "codex"})
	if err != nil || changed == base {
		t.Fatalf("digest ignored a model change: %q (%v)", changed, err)
	}
	scoped, err := AgentDigest(table("opus"), []string{"codex"})
	if err != nil || scoped == base {
		t.Fatalf("digest ignored the harness scope: %q (%v)", scoped, err)
	}
	if !strings.HasPrefix(base, "sha256:") {
		t.Errorf("digest = %q, want a sha256: prefix", base)
	}
}
