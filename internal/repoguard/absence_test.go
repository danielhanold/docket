package repoguard

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/harness"
	"github.com/danielhanold/docket/internal/harness/claude"
	"github.com/danielhanold/docket/internal/harness/codex"
	"github.com/danielhanold/docket/internal/harness/cursor"
	"github.com/danielhanold/docket/internal/harness/opencode"
	"github.com/danielhanold/docket/internal/install"
)

// This file is the FINAL absence seal for change 0370 (Gate 6; acceptance 17,
// 18, 19): after the frozen Bash control plane is physically deleted, no
// maintained file may still INVOKE, SOURCE, DEPEND ON, or variable-compose a
// call to it. It is the forward-looking replacement for four Bash seals deleted
// in Task 8 — test_facade_consumer_seal, test_go_consumer_migration_guard,
// test_deferred_surface_seal, and test_configured_bash_finalize — and it closes
// the ban-token GAP the pre-existing Go harness seal left: native_dispatch_test.go's
// runnerEraTokens bans only {docket.sh, runner-dispatch, scripts/runners}; this
// guard adds run-tests.sh, scripts/lib, docket-runtime.sh / runtime.bash, and the
// DOCKET_SCRIPTS_DIR / DOCKET_BASH_PATH env seams that the harness ban does not
// reach.
//
// # What is a violation — EXECUTABLE / DEPENDENCE shapes, not passive mentions
//
// The seal keys on syntactic shapes that make the retired control plane a live
// dependency of the checkout, never on the mere appearance of a name (AGENTS.md:
// "Key a guard on syntactic shape, never an enumerated list of spellings", and
// learning byte-pattern-guard-matches-a-spelling). The shape classes, each
// bounded on BOTH sides:
//
//   - facade/runner invocation: a path/command token ending in the retired
//     basenames docket.sh or run-tests.sh (retiredBasenameRe). Because the token
//     itself is matched, a variable-PREFIXED composition whose literal tail is
//     the basename ("$X/docket.sh", ${Y}run-tests.sh) and an assignment that
//     names the basename (f=docket.sh) are both caught.
//   - runtime.bash as a PATH SEGMENT (runtimeBashPathRe, slash-anchored) — which
//     is deliberately distinct from the obsolete `runtime.bash` CONFIG KEY that
//     internal/config correctly warns-and-ignores (a permanent, correct-end-state
//     false positive that MUST stay green).
//   - scripts/lib sourcing/reference: any executable-position reference to the
//     deleted helper tree (scriptsLibRe) — sourcing (`. scripts/lib/…`) or
//     invocation (`bash scripts/lib/…`).
//   - docket-runtime.sh sourcing/invocation (runtimeFileRe): the retired runtime
//     library by basename, even reached from outside scripts/lib.
//   - retired env dependence (envDependenceRe): a shell parameter EXPANSION of,
//     or an assignment/export TO, DOCKET_SCRIPTS_DIR / DOCKET_BASH_PATH. A bare
//     mention with no `$` and no `=` — a comment, or a Go string literal that
//     CLEARS the var to prove independence — is not a dependence and stays green.
//
// # Population — the executable surface, with categorical exclusions
//
// The seal scans repoguard.ExecutableSurface (shell scripts, executable-bit
// files, and the scripts/ + skills/ command-markdown a harness runs verbatim),
// which already prunes the immutable-history and frozen-corpus corpora
// categorically (docs/, every testdata/ tree, internal/install/legacydata,
// tests/fixtures — see repoguard.go). That population choice is itself the
// categorical explanation for the ACCEPTED survivor idioms this guard must NOT
// red, by LOCATION/OWNERSHIP rather than a per-file allowlist:
//
//   - Go source (*.go) is compiled program text, not an executed script, so it
//     is not executable surface. The retired tokens legitimately live there as
//     DATA — the ban-lists in internal/harness/{dispatch,native_dispatch}_test.go
//     and internal/reposeed/plan_test.go, the "the seam was retired" removal
//     comments in internal/app/repository_prepare.go and
//     internal/cli/development_test_cmd.go, and the test fixtures in this
//     package's own repoguard_test.go / test_source_hygiene_test.go. None are in
//     population; none can red this seal.
//   - Config YAML (.yml) is not executable surface either. The two frozen-pinned
//     comment residuals live there and are byte-pinned to versioned fixtures that
//     a re-cut would be needed to move (out of scope): root .docket.yml naming
//     run-tests.sh in a comment (pinned by internal/config's self fixture) and
//     agents/harness-defaults.yml naming scripts/lib/… in a comment (pinned to a
//     v0.9.3 fixture). Both are out of population and stay green trivially.
//
// # Markdown — fenced code is executable, prose is descriptive
//
// In a command-markdown file only a fenced code block is content a harness runs
// verbatim; prose (including inline `code` spans) is descriptive. The seal scans
// inside ``` / ~~~ fences and ignores prose, so a documented-removal sentence and
// the surviving scripts/runners/*.md prose that names `runner-dispatch.sh`
// descriptively are permitted, while a fenced `scripts/docket.sh` recipe is not.
//
// # Structural blind spots (stated, per byte-pattern-guard-matches-a-spelling)
//
//   - A command word assembled ENTIRELY from variables with no literal basename
//     substring anywhere on the reachable text (b=docket; e=sh; "$X/$b.$e") is
//     invisible to a byte pattern. The variable-PREFIXED form with a literal tail
//     IS caught; the fully-decomposed form is not.
//   - .github/workflows/*.yml is not in ExecutableSurface, so a re-introduced
//     `DOCKET_BASH_PATH=` export in a workflow step would not be seen here (that
//     seam's producer was cleaned in Task 8 and is now a comment). The shell and
//     command-markdown surface — where these shapes actually execute — is fully
//     covered.
//
// # Fail-closed and the population floor
//
// A walk error, an unreadable file, or any scan error is a test FAILURE naming
// the path (acceptance 17): ExecutableSurface fails closed on an unreadable
// directory, and readMaintained fails closed on an unreadable file. The seal
// separately asserts its scanned population is non-empty — an empty walk is an
// error, not a vacuous pass (learning marker-scoped-guard-needs-a-population-floor).

// retiredBasenameRe matches docket.sh or run-tests.sh as a path/command token,
// bounded on both sides so a longer identifier (mydocket.shx, foo-run-tests.sh)
// does not match. The left boundary excludes `.` and `-` as well as word chars;
// the right excludes word chars and `-`.
var retiredBasenameRe = regexp.MustCompile(`(^|[^[:alnum:]_.-])(docket\.sh|run-tests\.sh)([^[:alnum:]_-]|$)`)

// runtimeBashPathRe matches runtime.bash ONLY as a slash-anchored path segment
// (…/runtime.bash), which is what a retired-runtime path or invocation looks
// like — never the dotted config key `runtime.bash`, which has no leading slash.
var runtimeBashPathRe = regexp.MustCompile(`/runtime\.bash([^[:alnum:]_-]|$)`)

// scriptsLibRe matches a reference to the deleted scripts/lib/ helper tree,
// left-bounded so xscripts/lib/ does not match.
var scriptsLibRe = regexp.MustCompile(`(^|[^[:alnum:]_.-])scripts/lib/`)

// runtimeFileRe matches the retired runtime library docket-runtime.sh as a
// path/command token (any prefix up to the basename), left-bounded.
var runtimeFileRe = regexp.MustCompile(`(^|[^[:alnum:]_.-])[^[:space:]"';|&]*docket-runtime\.sh`)

// envDependenceRe matches a shell parameter EXPANSION of, or an
// assignment/export TO, the two retired env seams — never a bare mention. The
// `$`-expansion arm catches $DOCKET_BASH_PATH and ${DOCKET_SCRIPTS_DIR:?}; the
// assignment arm catches (export) DOCKET_BASH_PATH=…. A quoted string literal
// naming the token with neither `$` nor `=` (os.Getenv("DOCKET_SCRIPTS_DIR"),
// t.Setenv("DOCKET_SCRIPTS_DIR", "")) is not matched.
var envDependenceRe = regexp.MustCompile(
	`(\$\{?(DOCKET_SCRIPTS_DIR|DOCKET_BASH_PATH)([^[:alnum:]_]|$))` +
		`|((^|[^[:alnum:]_])(export[[:space:]]+)?(DOCKET_SCRIPTS_DIR|DOCKET_BASH_PATH)=)`)

// absenceHit is one shape-class match at a location.
type absenceHit struct {
	class string
	rel   string
	line  int
	text  string
}

// scanExecutableLine applies every shape class to a single already-decommented
// executable line and returns the matches.
func scanExecutableLine(rel string, lineNo int, line string) []absenceHit {
	var hits []absenceHit
	add := func(class string) {
		hits = append(hits, absenceHit{class, rel, lineNo, strings.TrimSpace(line)})
	}
	if retiredBasenameRe.MatchString(line) {
		add("facade/runner invocation (docket.sh/run-tests.sh)")
	}
	if runtimeBashPathRe.MatchString(line) {
		add("runtime.bash path segment")
	}
	if scriptsLibRe.MatchString(line) {
		add("scripts/lib sourcing/reference")
	}
	if runtimeFileRe.MatchString(line) {
		add("docket-runtime.sh sourcing/invocation")
	}
	if envDependenceRe.MatchString(line) {
		add("retired env dependence (DOCKET_SCRIPTS_DIR/DOCKET_BASH_PATH)")
	}
	return hits
}

// isMarkdownSurface reports whether rel is command-markdown, where only fenced
// code blocks are executable content.
func isMarkdownSurface(rel string) bool { return hasExt(rel, ".md", ".mdc", ".markdown") }

// stripHashComment removes an unquoted `#` comment (shell + YAML + TOML). Quote
// state is tracked so a `#` inside a quoted string is preserved; a `#` opens a
// comment only at a word start (line-start or after whitespace).
func stripHashComment(line string) string {
	var sq, dq bool
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case c == '\'' && !dq:
			sq = !sq
		case c == '"' && !sq:
			dq = !dq
		case c == '#' && !sq && !dq:
			if i == 0 || line[i-1] == ' ' || line[i-1] == '\t' {
				return line[:i]
			}
		}
	}
	return line
}

// scanGeneratedForRetired scans GENERATOR OUTPUT strictly: every line (with its
// `#` comment stripped) is checked, prose included. The on-disk markdown rule
// (fenced code is executable, prose is descriptive) does not apply to machine
// emission — a generator that learns to write the retired route into an
// agent-INSTRUCTION block (which a harness executes as prose, learning
// agent-executed-markdown-is-code) is exactly the regression this scan catches,
// so a forbidden token anywhere in the emitted bytes is a violation. Comment
// residuals the generator copies verbatim into config YAML (.docket.example.yml,
// agents/harness-defaults.yml) are still stripped, so the two frozen-pinned
// comment residuals stay green here too.
func scanGeneratedForRetired(rel, content string) []absenceHit {
	var hits []absenceHit
	for i, raw := range strings.Split(content, "\n") {
		hits = append(hits, scanExecutableLine(rel, i+1, stripHashComment(raw))...)
	}
	return hits
}

// scanContentForRetired extracts the executable lines of one file and scans them
// for every retired-control-plane shape. Shell/exec-bit/config files scan every
// line with its `#` comment stripped; command-markdown scans only lines inside a
// ``` / ~~~ fence (prose, including inline code spans, is descriptive and not
// scanned), likewise with the fenced shell's own `#` comments stripped.
func scanContentForRetired(rel, content string) []absenceHit {
	var hits []absenceHit
	md := isMarkdownSurface(rel)
	inFence := false
	for i, raw := range strings.Split(content, "\n") {
		if md {
			trimmed := strings.TrimSpace(raw)
			if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
				inFence = !inFence
				continue
			}
			if !inFence {
				continue
			}
		}
		hits = append(hits, scanExecutableLine(rel, i+1, stripHashComment(raw))...)
	}
	return hits
}

// TestNoRetiredBashControlPlane is the final absence seal (change 0370, Gate 6).
func TestNoRetiredBashControlPlane(t *testing.T) {
	root := guardRoot(t)
	pop := execPop(t, root)

	// Population floor FIRST (marker-scoped-guard-needs-a-population-floor): a
	// broken or empty walk passes every "no violations" assert vacuously. The
	// surviving executable surface (skills/**/*.md, scripts/*, the retained POSIX
	// suites and the go-wrapper suites) is comfortably above this floor.
	const floor = 40
	if len(pop) < floor {
		t.Fatalf("population floor: executable surface collapsed to %d files (expected >= %d) — a broken walk passes every absence assert vacuously", len(pop), floor)
	}

	var violations []string
	for _, rel := range pop {
		// readMaintained fails closed on an unreadable file (acceptance 17).
		for _, h := range scanContentForRetired(rel, readMaintained(t, root, rel)) {
			violations = append(violations, fmt.Sprintf("%s:%d: %s: %s", h.rel, h.line, h.class, h.text))
		}
	}
	if len(violations) != 0 {
		t.Errorf("retired Bash control-plane shapes survive in the executable surface (%d):\n%s",
			len(violations), strings.Join(violations, "\n"))
	}

	t.Run("generator_output", testGeneratorOutputAbsence)
	t.Run("non_vacuity", testAbsenceNonVacuity)
	t.Run("negative_controls", testAbsenceNegativeControls)
}

// testGeneratorOutputAbsence runs each surviving canonical generator into a temp
// dir and scans its OUTPUT with the same shape classes: a generator that emits a
// forbidden command is caught even while every committed file is clean.
func testGeneratorOutputAbsence(t *testing.T) {
	root := guardRoot(t)

	cat, err := assets.EmbeddedCatalog()
	if err != nil {
		t.Fatalf("EmbeddedCatalog: %v", err)
	}

	// Generator 1: the harness dispatch / agent-wrapper renderer — the generator
	// that historically emitted the facade shim ${DOCKET_SCRIPTS_DIR}/docket.sh
	// runner-dispatch, so a regression here is exactly what this seal exists to
	// catch. Every adapter's rendered targets plus the two shared dispatch-surface
	// producers are materialized to a temp tree.
	in := harness.PlanInput{
		Assets:    cat,
		Mode:      harness.ModeRelease,
		AssetsDir: "/data/versions/sha256-x/assets",
		Roots: install.UserRoots{
			Home:       "/home/u",
			DataRoot:   "/home/u/.local/share/docket",
			ConfigHome: "/home/u/.config",
			BinDir:     "/home/u/.local/bin",
		},
		Agents: crossAgentsTable(),
	}
	out := t.TempDir()
	written := 0
	writeGen := func(relParts []string, content []byte) {
		p := filepath.Join(append([]string{out}, relParts...)...)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir generated: %v", err)
		}
		if err := os.WriteFile(p, content, 0o644); err != nil {
			t.Fatalf("write generated: %v", err)
		}
		written++
	}
	adapters := map[string]harness.Adapter{
		"claude":   claude.New(),
		"codex":    codex.New(),
		"cursor":   cursor.New(),
		"opencode": opencode.New(),
	}
	for _, name := range harness.Order {
		targets, err := adapters[name].Plan(in)
		if err != nil {
			t.Fatalf("%s Plan: %v", name, err)
		}
		if len(targets) == 0 {
			t.Fatalf("%s rendered no targets — the generator scan would be vacuous", name)
		}
		for i, tg := range targets {
			writeGen([]string{name, fmt.Sprintf("%03d-%s", i, filepath.Base(tg.Path))}, tg.Content)
		}
	}
	rg, err := harness.RunGate(cat)
	if err != nil {
		t.Fatalf("RunGate: %v", err)
	}
	writeGen([]string{"dispatch", "interior.md"}, []byte(harness.DispatchInterior(rg)))
	writeGen([]string{"dispatch", "cursor-rule.mdc"}, cursor.DispatchRuleContent(rg))

	// Generator 2: the embedded-asset generator (cmd/genassets), materialized into
	// a temp tree exactly as the tool does.
	m, payload, err := assets.Generate(root, assets.DefaultAllowedRoots())
	if err != nil {
		t.Fatalf("assets.Generate: %v", err)
	}
	tree := filepath.Join(t.TempDir(), "embedded")
	if err := assets.WriteTree(tree, m, payload); err != nil {
		t.Fatalf("assets.WriteTree: %v", err)
	}

	var violations []string
	scanned := 0
	scanTree := func(dir string) {
		walkErr := filepath.WalkDir(dir, func(p string, d fs.DirEntry, werr error) error {
			if werr != nil {
				return werr // fail closed
			}
			if d.IsDir() || !d.Type().IsRegular() {
				return nil
			}
			b, rerr := os.ReadFile(p)
			if rerr != nil {
				return rerr // fail closed
			}
			rel, rerr := filepath.Rel(dir, p)
			if rerr != nil {
				return rerr
			}
			rel = filepath.ToSlash(rel)
			scanned++
			for _, h := range scanGeneratedForRetired(rel, string(b)) {
				violations = append(violations, fmt.Sprintf("[generator-output] %s:%d: %s: %s", h.rel, h.line, h.class, h.text))
			}
			return nil
		})
		if walkErr != nil {
			t.Fatalf("scan generated tree %s failed closed: %v", dir, walkErr)
		}
	}
	scanTree(out)
	scanTree(tree)

	if written == 0 {
		t.Fatalf("generator population floor: the harness renderer produced no output")
	}
	if scanned < 20 {
		t.Fatalf("generator population floor: only %d generated files scanned (expected >= 20)", scanned)
	}
	if len(violations) != 0 {
		t.Errorf("retired Bash control-plane shapes in GENERATOR OUTPUT (%d):\n%s",
			len(violations), strings.Join(violations, "\n"))
	}
}

// crossAgentsTable pins each harness on its own model row so the renderer is
// deterministic, mirroring internal/harness's cross-adapter fixture.
func crossAgentsTable() config.AgentsTable {
	agents := config.AgentsTable{}
	agents["claude"] = map[string]config.AgentSetting{
		"build-standard": {Model: config.Value[string]{Value: "claude-opus-5[1m]"}, Effort: config.Value[string]{Value: "high"}},
	}
	agents["codex"] = map[string]config.AgentSetting{
		"build-standard": {Model: config.Value[string]{Value: "gpt-5.5-codex"}, Effort: config.Value[string]{Value: "high"}},
	}
	agents["cursor"] = map[string]config.AgentSetting{
		"build-standard": {Model: config.Value[string]{Value: "gpt-5.5-cursor"}, Effort: config.Value[string]{Value: "high"}},
	}
	agents["opencode"] = map[string]config.AgentSetting{
		"build-standard": {Model: config.Value[string]{Value: "openrouter/anthropic/claude-opus-5"}, Effort: config.Value[string]{Value: "high"}},
	}
	return agents
}

// testAbsenceNonVacuity exercises every shape-class detector directly: each bad
// input must be caught, each ACCEPTED survivor idiom must stay green. This is the
// mutation the guard proves on itself — a detector that stopped matching would
// redden here before any planted-file mutation.
func testAbsenceNonVacuity(t *testing.T) {
	bt := "\x60" // backtick
	bad := map[string]string{
		"direct facade invocation":         "scripts/docket.sh preflight",
		"variable-prefixed facade":         `"$dir/scripts/docket.sh" preflight`,
		"braced-prefix runner":             "${SCRIPTS}/run-tests.sh --serial",
		"basename assignment":              "f=docket.sh",
		"backtick-composed invocation":     "out=" + bt + "$root/docket.sh status" + bt,
		"runtime sourcing via scripts/lib": ". scripts/lib/docket-runtime.sh",
		"scripts/lib invocation":           "bash scripts/lib/docket-frontmatter.sh",
		"runtime file sourced elsewhere":   `. "$DIR/vendor/docket-runtime.sh"`,
		"runtime.bash path segment":        `exec "$root/tools/runtime.bash"`,
		"env dependence required":          ": \"${DOCKET_SCRIPTS_DIR:?}\"",
		"env expansion":                    "echo \"$DOCKET_BASH_PATH/docket\"",
		"env export":                       "export DOCKET_BASH_PATH=/opt/bin/bash",
		"env assignment":                   "DOCKET_SCRIPTS_DIR=$PWD/scripts",
	}
	for name, b := range bad {
		if got := scanContentForRetired("probe.sh", b); len(got) == 0 {
			t.Errorf("detector missed a retired shape [%s]: %q", name, b)
		}
	}
	// A fenced facade recipe in command-markdown (the mutation-7 shape).
	fenced := "before prose\n```bash\nscripts/docket.sh preflight\n```\nafter prose\n"
	if got := scanContentForRetired("skills/x/SKILL.md", fenced); len(got) != 1 || got[0].line != 3 {
		t.Errorf("markdown fence scan missed the fenced facade command or misreported its line: %+v", got)
	}

	good := map[string]string{
		"comment mention in shell":        "# scripts/docket.sh is retired — never call it",
		"runner-dispatch prose token":     "`runner-dispatch.sh` owns that normalization",
		"obsolete config key literal":     `case "runtime.bash":`,
		"config-key remedy prose":         "remove runtime.bash from this file",
		"Go env read (string literal)":    `Bash: os.Getenv("DOCKET_BASH_PATH"),`,
		"Go env clear (proves independ.)": `t.Setenv("DOCKET_SCRIPTS_DIR", "")`,
		"retired-seam removal comment":    "# the DOCKET_BASH_PATH seam was retired with the frozen control plane",
		"markdown prose facade mention":   "the old `scripts/docket.sh` facade is gone",
	}
	for name, g := range good {
		// Shell files: prose-mention goods are comment/keyword shaped; the two
		// markdown-prose goods are checked as markdown so the non-fenced prose is
		// ignored.
		rel := "probe.sh"
		if strings.Contains(name, "markdown") {
			rel = "skills/x/SKILL.md"
		}
		if got := scanContentForRetired(rel, g); len(got) != 0 {
			t.Errorf("detector wrongly flagged an accepted survivor idiom [%s]: %q -> %+v", name, g, got)
		}
	}
}

// testAbsenceNegativeControls (acceptance 19) points the guard at a fixture tree
// whose ONLY forbidden shapes sit inside categorically excluded corpora
// (docs/adrs, archived plans, every testdata tree, legacydata, tests/fixtures)
// and asserts both that those files are absent from the scanned population and
// that the guard is GREEN over what remains.
func testAbsenceNegativeControls(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module x\n")

	// Forbidden shapes, EVERY one inside an excluded corpus — must be ignored.
	excludedBad := map[string]string{
		"docs/adrs/0001.md":                        "```\nscripts/docket.sh preflight\n```\n",
		"docs/superpowers/plans/old.md":            "```\n. scripts/lib/docket-runtime.sh\n```\n",
		"internal/repository/testdata/corpus/f.sh": "scripts/docket.sh preflight\n",
		"testdata/repositories/v0.9.6/x.sh":        "export DOCKET_BASH_PATH=/bin/bash\n",
		"internal/install/legacydata/old.mdc":      "```\nscripts/docket.sh preflight\n```\n",
		"internal/install/testdata/legacy/o.sh":    ": \"${DOCKET_SCRIPTS_DIR:?}\"\n",
		"tests/fixtures/hygiene/bad.sh":            ". scripts/lib/x.sh\n",
	}
	for rel, content := range excludedBad {
		writeFile(t, root, rel, content)
	}
	// Clean INCLUDED files so the population is non-empty and the green assert is
	// not vacuous.
	writeFile(t, root, "install.sh", "#!/bin/sh\necho ok\n")
	writeFile(t, root, "scripts/runners/codex.sh", "#!/bin/sh\nexec codex \"$@\"\n")
	writeFile(t, root, "skills/x/SKILL.md", "# heading\n\nprose that says `docket.sh` descriptively\n")

	pop, err := ExecutableSurface(root)
	if err != nil {
		t.Fatalf("ExecutableSurface: %v", err)
	}
	for rel := range excludedBad {
		if slices.Contains(pop, rel) {
			t.Errorf("negative control: excluded-corpus file %q leaked into the scanned population", rel)
		}
	}
	if len(pop) == 0 {
		t.Fatalf("negative control: the included population is empty; the green assert would be vacuous")
	}
	var violations []string
	for _, rel := range pop {
		for _, h := range scanContentForRetired(rel, readMaintained(t, root, rel)) {
			violations = append(violations, fmt.Sprintf("%s:%d: %s", h.rel, h.line, h.class))
		}
	}
	if len(violations) != 0 {
		t.Errorf("negative control tree must be GREEN — every forbidden shape is in an excluded corpus — got:\n%s",
			strings.Join(violations, "\n"))
	}
}
