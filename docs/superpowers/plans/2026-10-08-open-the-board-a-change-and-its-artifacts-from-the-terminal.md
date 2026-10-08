<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0542 — Open the board, a change, and its artifacts from the terminal](../../changes/active/0542-open-the-board-a-change-and-its-artifacts-from-the-terminal.md)**
<!-- docket:backlink:end -->

# Open the board, a change, and its artifacts from the terminal: Implementation Plan

> **For agentic workers:** executed by `docket-build`, task by task, in order; each task ends in one
> commit. Supplied test code is an unverified draft: prove each assert can pass and mutation-test
> its key (learning `plan-supplied-test-code-is-unverified`). Write doc comments on every new
> exported symbol and file in the house style; the code below keeps comments short to fit.

**Goal:** Add `docket open <what> [id]`: open the board, a change record, or a change's spec, plan,
results, or PR, as a GitHub page or from the local metadata checkout, inferring the change from
the checked-out branch when no id is given.

**Architecture:** `app.Open` composes existing machinery (pinned corpus read, `linkContextOf(pin).BlobURL`, `RunRepositoryPrepare`, `WorktreeCheckoutState` + `recordedBranch`) behind an injected `Opener`; a new layered key `open.artifacts` picks GitHub vs local; `internal/cli/open.go` is a thin adapter. Tech: Go, Cobra.

**Spec:** `docs/superpowers/specs/2026-10-08-open-the-board-a-change-and-its-artifacts-from-the-terminal-design.md`
(metadata branch). Its section numbers are cited below; read it alongside this plan.

## Global Constraints

The spec is authoritative for every message and rule; these are the ones most easily missed.

- Grammar `docket open <what> [id] [--print] [--repo-dir <dir>] [--json]`; six targets; id decimal with leading zeros allowed, `#541` rejected; board takes no id.
- `OpenResult` JSON: `what`, `change_id` (omitted for board), `target`, `target_kind` (`url`|`file`), `launched`, `notes`.
- GitHub vs local (all but `pr`): private -> local, no note, key ignored; else `open.artifacts: local` -> local; else `linkContextOf(pin).BlobURL(path)`, whose `""` means local plus the not-GitHub note. No new visibility or host logic. `pr` always opens its `http(s)` URL.
- Local opens run `RunRepositoryPrepare` first and never block on it; path root is prepare's `metadata_worktree_path`; never assume `.docket` or a store path.
- Opener: `open` (darwin), `xdg-open` (linux), none elsewhere; one argv element, no shell; waits; tests inject fakes and never launch a real application.
- CLI annotation `capability("open", EffectRead, EffectLocalWrite, EffectProcessControl)`; binding `{ID: "open", Request: nil, Result: OpenResult{}}`.
- Living docs cite no change/PR numbers; `cli.md` lists no flags.
- Repo rules: `go test -count=1` for probes; restore mutations from a backup copy; gofmt via `"$(GOTOOLCHAIN="$(awk '$1=="toolchain"{print $2}' go.mod)" go env GOROOT)/bin/gofmt" -w <files>`; the default-tag `internal/app` corpus never starts real `git`.

## Review Focus

1. **`--print` in command substitution**: stdout must be exactly the target; notes go to stderr in human mode. -> Task 5 `TestOpenCLIPrintNotesGoToStderr`.
2. **Relative `--repo-dir` / subdirectory cwd**: `WorktreeCheckoutState` refuses non-absolute paths; Open must make the dir absolute first. -> Task 4 `TestOpenInferenceViaOpen`.
3. **A record path escaping the repository** (`../../etc/passwd`, `/etc/passwd`): refused, never opened, stat'ed, or linked. -> Task 4 `TestOpenRefusesArtifactPathOutsideRepository`.
4. **Opener chatter and spaces**: opener stdout must never reach docket's stdout (the JSON stream); a path with spaces is one argv element. -> Task 2 `TestRunOpenerProcessPassesOneArgAndReportsFailure`.
5. **Checkout probe error** must not read as "belongs to no change" (learning `probe-error-is-not-clean-absence`). -> Task 3 `TestCheckoutBranchProbeErrorIsNotAbsence`, Task 4 `TestOpenInferenceViaOpen`.

---

### Task 1: The `open.artifacts` config key, end to end

**Files:** Modify `internal/config/{schema,config,defaults,resolve}.go`; tests `internal/config/{decode,defaults,fixtures,resolve,schema,setting_paths}_test.go`; `internal/app/config.go` + `config_test.go`; `.docket.example.yml` (+ regenerated `internal/assets/embedded/tree/.docket.example.yml`, `internal/assets/embedded/manifest.json`); `docs/reference/config-keys.md`.

**Interfaces:** Produces `config.Effective.Open` (`json:"open"`) of type `config.Open{ Artifacts Value[string] \`json:"artifacts"\` }`; `"github"` (built-in default) or `"local"`.

Template: the `leak_check.match_word` key (commit `b57353aa1`); mirror each of its edits.

- [ ] **Step 1: Write the failing tests**

`internal/config/resolve_test.go`, in `effectiveLeaf`'s switch:

```go
	case "open.artifacts":
		return eff.Open.Artifacts.Value, eff.Open.Artifacts.Provenance, eff.Open.Artifacts.Explicit
```

and append:

```go
func TestOpenArtifactsIsAnOrdinaryLayeredKey(t *testing.T) {
	if v := mustResolve(t, nil, mainCtx).effective.Open.Artifacts; v.Value != "github" || v.Provenance.Layer != LayerBuiltIn || v.Explicit {
		t.Fatalf("default = %+v, want non-explicit built-in github", v)
	}
	g, r, l := srcG("open:\n  artifacts: local\n"), srcR("open:\n  artifacts: github\n"), srcL("open:\n  artifacts: local\n")
	for _, tc := range []struct {
		sources []Source
		want    string
		layer   LayerKind
	}{{[]Source{g}, "local", LayerGlobal}, {[]Source{g, r}, "github", LayerRepository}, {[]Source{r, l}, "local", LayerRepositoryLocal}} {
		res := mustResolve(t, tc.sources, mainCtx)
		if v := res.effective.Open.Artifacts; v.Value != tc.want || v.Provenance.Layer != tc.layer || !v.Explicit {
			t.Errorf("open.artifacts = %+v, want explicit %q from %q", v, tc.want, tc.layer)
		}
		for _, d := range res.diags {
			if d.Severity == SeverityWarning || d.Severity == SeverityError {
				t.Errorf("unexpected diagnostic %s/%s/%s", d.Severity, d.Code, d.Path)
			}
		}
	}
}
```

`decode_test.go`: in `decodeAcceptanceCases()` after the `leak_check.match_word` row add
`{row: "open.artifacts", path: "open.artifacts", block: "open:\n  artifacts: local\n", flow: "open: {artifacts: local}\n", value: "local"},`;
in `TestDecodeRejections` after `"bad enum"` add
`{"bad open.artifacts enum", "open:\n  artifacts: browser\n", CodeInvalidValue, "open.artifacts", SeverityError},`.

`defaults_test.go` (`TestBuiltinEffectiveMatchesRegistryDefaults`): `"open.artifacts": eff.Open.Artifacts.Value,`.
`fixtures_test.go` (`TestFixtureSparseDefaults`): `{"open.artifacts", eff.Open.Artifacts.Explicit, eff.Open.Artifacts.Provenance.Layer},`.
`schema_test.go`: `"open.artifacts",` right after `"leak_check.match_word",` in `TestRegistryPathSetMatchesV092`; `"open.artifacts": "github",` in `TestRegistryDefaults`; `"open.artifacts": {"github", "local"},` in `TestRegistryEnumRows`.
`setting_paths_test.go` (`TestSettingPathsSupportSplit`): `"open.artifacts",` right after `"leak_check.match_word",`.

`internal/app/config_test.go` (mirrors `TestConfigDiagnosticsRepairMaxAttemptsSurface`; asserts the resolved non-default, learning `defaulted-param-hides-caller-wiring`):

```go
func TestConfigDiagnosticsOpenArtifactsSurface(t *testing.T) {
	src := []config.Source{{Layer: config.LayerRepository, Name: ".docket.yml", Data: []byte("open:\n  artifacts: local\n")}}
	for _, tc := range []struct {
		sources    []config.Source
		want, line string
	}{{sparseSources(), "github", "open.artifacts = github  [built-in]"}, {src, "local", "open.artifacts = local"}} {
		r := DiagnosticConfig(tc.sources, mainCtx(), false)
		if r.Effective == nil || r.Effective.Open.Artifacts.Value != tc.want || !strings.Contains(r.HumanText(), tc.line) {
			t.Errorf("want %q and line %q:\n%s", tc.want, tc.line, r.HumanText())
		}
	}
}
```

- [ ] **Step 2: Verify they fail**

Run: `go test -count=1 ./internal/config/ ./internal/app/ -run 'TestOpenArtifacts|TestDecode|TestBuiltinEffective|TestFixtureSparse|TestRegistry|TestSettingPaths|TestConfigDiagnosticsOpenArtifacts'`
Expected: build failure, `Effective has no field or method Open`.

- [ ] **Step 3: Implement**

`schema.go`, after the `leak_check.match_word` row:

```go
		// open.artifacts: where `docket open` sends a shared repository's
		// artifacts. A private repository always opens locally.
		{path: "open.artifacts", kind: kindString, enum: []string{"github", "local"}, def: "github",
			merge: mergeScalar, scope: scopeAny, disp: dispSupported,
			validate: enumLeaf("github", "local")},
```

`config.go`: in `Effective` after `LeakCheck`, add `Open Open \`json:"open"\``; after the `LeakCheck` type:

```go
// Open is the `docket open` policy.
type Open struct {
	Artifacts Value[string] `json:"artifacts"` // github|local
}
```

`defaults.go` (`builtinEffective`): `Open: Open{Artifacts: builtinValue("github")},`.
`resolve.go` (`assemble`, after leak_check): `set(assign(&eff.Open.Artifacts, r.declared, "open.artifacts"))`.
`internal/app/config.go` (`effectiveLines`, after leak_check): `leafLine("open.artifacts", textValue(eff.Open.Artifacts.Value), eff.Open.Artifacts.Provenance),`.
gofmt the edited files.

- [ ] **Step 4: Verify they pass** (same command). Expected: PASS.

- [ ] **Step 5: Document the key**

`go test -count=1 ./internal/config/ -run Example` now fails (Direction B: supported key undocumented). In `.docket.example.yml`, insert between the `leak_check` block and the `# ═══ learnings` header, keeping the two blank lines before the next header; the header is 92 characters like its neighbours:

```yaml
# ═══ open — where docket open takes you ═══════════════════════════════════════════════════

open:
  # artifacts — github (default): in a shared repository, `docket open` shows the board, a
  # change, and its spec, plan, and results as GitHub pages in your browser. local: open the
  # file from your metadata checkout in your system's default app for Markdown instead —
  # handy offline, or when you would rather read in an editor. A private repository always
  # opens the local file, and `docket open pr` always opens the pull request on GitHub.
  # scope: any layer
  artifacts: github
```

Then `go generate ./internal/assets/`. In `docs/reference/config-keys.md`, above the `gate_observation_budget` row:

```markdown
| `open.artifacts` | `github` | any layer | where `docket open` shows a shared repository's board and change artifacts: `github` pages, or the `local` metadata checkout's files |
```

Run: `go test -count=1 ./internal/config/ ./internal/assets/` -> PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/config/ internal/app/config.go internal/app/config_test.go .docket.example.yml \
  internal/assets/embedded/tree/.docket.example.yml internal/assets/embedded/manifest.json docs/reference/config-keys.md
git commit -m "feat(config): add open.artifacts"
```

---

### Task 2: The opener seam

**Files:** Create `internal/app/open_opener.go`, `internal/app/open_opener_test.go`.

**Interfaces (produces):** `type Opener interface { Open(ctx context.Context, target string) error }`; `type NoOpenerError struct{ GOOS string }` (`Error()` = `no opener available on <GOOS> — open it yourself, or use --print`); `func NewSystemOpener(goos string, lookPath func(string) (string, error), run func(ctx context.Context, bin, target string) error) Opener`; `func RunOpenerProcess(ctx context.Context, bin, target string) error`.

- [ ] **Step 1: Write the failing tests**

```go
package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

func TestSystemOpenerSelectsByPlatform(t *testing.T) {
	cases := []struct {
		goos, lookup string
		lookErr      error
		noOpener     bool
	}{
		{goos: "darwin", lookup: "open"},
		{goos: "linux", lookup: "xdg-open"},
		{goos: "linux", lookup: "xdg-open", lookErr: errors.New("not found"), noOpener: true},
		{goos: "windows", noOpener: true},
		{goos: "freebsd", noOpener: true},
	}
	for _, tc := range cases {
		var looked []string
		var ran [][2]string
		o := NewSystemOpener(tc.goos,
			func(n string) (string, error) { looked = append(looked, n); return "/bin/" + n, tc.lookErr },
			func(_ context.Context, bin, target string) error { ran = append(ran, [2]string{bin, target}); return nil })
		err := o.Open(context.Background(), "/tmp/a b/c.md")
		if tc.noOpener {
			var no *NoOpenerError
			want := "no opener available on " + tc.goos + " — open it yourself, or use --print"
			if !errors.As(err, &no) || err.Error() != want || len(ran) != 0 {
				t.Errorf("%s: err=%v ran=%v, want %q and no launch", tc.goos, err, ran, want)
			}
			if tc.lookup == "" && len(looked) != 0 {
				t.Errorf("%s: consulted PATH %v", tc.goos, looked)
			}
			continue
		}
		if err != nil || len(looked) != 1 || looked[0] != tc.lookup || len(ran) != 1 || ran[0] != [2]string{"/bin/" + tc.lookup, "/tmp/a b/c.md"} {
			t.Errorf("%s: err=%v looked=%v ran=%v", tc.goos, err, looked, ran)
		}
	}
}

// openerScript writes a stand-in opener recording "$#|$1", chattering on
// stdout and stderr, and exiting code.
func openerScript(t *testing.T, code int, errText string) (bin, argsFile string) {
	t.Helper()
	dir := testsupport.TempDir(t)
	argsFile, bin = filepath.Join(dir, "args"), filepath.Join(dir, "fake-opener")
	body := fmt.Sprintf("#!/bin/sh\nprintf '%%s|%%s' \"$#\" \"$1\" > '%s'\necho chatter\necho '%s' >&2\nexit %d\n", argsFile, errText, code)
	if err := os.WriteFile(bin, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	return bin, argsFile
}

func TestRunOpenerProcessPassesOneArgAndReportsFailure(t *testing.T) {
	bin, args := openerScript(t, 0, "")
	if err := RunOpenerProcess(context.Background(), bin, "/tmp/a b/c d.md"); err != nil {
		t.Fatalf("RunOpenerProcess: %v", err)
	}
	if got, _ := os.ReadFile(args); string(got) != "1|/tmp/a b/c d.md" {
		t.Errorf("opener saw %q, want one argument", got)
	}
	bin, _ = openerScript(t, 3, "cannot open display")
	err := RunOpenerProcess(context.Background(), bin, "https://example.test/x")
	if err == nil || !strings.Contains(err.Error(), "exit status 3") || !strings.Contains(err.Error(), "cannot open display") {
		t.Fatalf("err = %v, want exit status and stderr", err)
	}
}
```

- [ ] **Step 2: Verify they fail.** Run: `go test -count=1 ./internal/app/ -run 'TestSystemOpener|TestRunOpenerProcess'`. Expected: `undefined: NewSystemOpener`.

- [ ] **Step 3: Implement** `internal/app/open_opener.go`: the `Opener` interface; `NoOpenerError` (pointer receiver `Error`, message above); `openerCommand(goos)` returning `"open"`/`"xdg-open"`/`""`; an unexported `systemOpener{goos, lookPath, run}` whose `Open` returns `&NoOpenerError{GOOS}` when the command is `""` or `lookPath` errors, else `run(ctx, bin, target)`; and:

```go
// RunOpenerProcess runs bin with target as its only argument (no shell) and
// waits. Stdin/stdout stay unset (the null device), so opener chatter never
// reaches docket's protocol stdout; a non-zero exit carries the opener's stderr.
func RunOpenerProcess(ctx context.Context, bin, target string) error {
	cmd := exec.CommandContext(ctx, bin, target)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if detail := strings.TrimSpace(stderr.String()); detail != "" {
			return fmt.Errorf("%s: %w: %s", filepath.Base(bin), err, detail)
		}
		return fmt.Errorf("%s: %w", filepath.Base(bin), err)
	}
	return nil
}
```

- [ ] **Step 4: Verify they pass** (same command). Mutation: swap `"xdg-open"` for `"open"` -> `TestSystemOpenerSelectsByPlatform` reddens.

- [ ] **Step 5: Commit**

```bash
git add internal/app/open_opener.go internal/app/open_opener_test.go
git commit -m "feat(app): add the opener seam for docket open"
```

---

### Task 3: Argument grammar, result document, and branch inference

**Files:** Create `internal/app/open.go`, `internal/app/open_infer.go`, `internal/app/open_test.go`, `internal/app/open_infer_test.go`.

**Interfaces:**
- Consumes: `Opener` (Task 2); existing `RepositoryPrepareResult`, `StatusReader`, `gitcli.CheckoutState`, `recordedBranch`, `branchRefPrefix` (`"refs/heads/"`, `status_git.go`), `parseCorpus`, `repository.BuildSnapshot`.
- Produces: `OperationOpen = "open"`; `OpenTargets = []string{"board", "change", "spec", "plan", "results", "pr"}`; `OpenTargetKindURL = "url"`, `OpenTargetKindFile = "file"`; reason constants `ReasonOpenX = "<value>"` for X/value: InvalidTarget/invalid-target, UnknownChange/unknown-change, AmbiguousChange/ambiguous-change, ArtifactUnset/artifact-unset, TrivialNoSpec/trivial-no-spec, BoardDisabled/board-disabled, PRNotURL/pr-not-url, PathInvalid/artifact-path-invalid, HeadDetached/head-detached, BranchUnmatched/branch-unmatched, BranchAmbiguous/branch-ambiguous, CheckoutUnreadable/checkout-unreadable, FileMissing/file-missing, NoOpener/no-opener, OpenerFailed/opener-failed; `openNoteNotGitHub` = `origin is not a GitHub remote — opened the local file instead`; `openNoteStale(code string) string` = `metadata checkout not synced (<code>) — file may be stale`; `OpenOptions`, `OpenDeps`, `OpenResult` (below); `openFailure{result Result; reason, message string}`; `ParseOpenArgs`; `checkoutBranch(ctx, probe, dir) (string, *openFailure)`; `inferChangeFromBranch(snap domain.Snapshot, branch string) (domain.Change, *openFailure)`.

- [ ] **Step 1: Write the failing tests**

`internal/app/open_test.go` (grows in Task 4):

```go
package app

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestParseOpenArgs(t *testing.T) {
	ok := []struct {
		args []string
		what string
		id   int
	}{
		{[]string{"board"}, "board", 0}, {[]string{"spec"}, "spec", 0},
		{[]string{"change", "541"}, "change", 541}, {[]string{"plan", "0541"}, "plan", 541},
		{[]string{"pr", "7"}, "pr", 7}, {[]string{"results", "000012"}, "results", 12},
	}
	for _, tc := range ok {
		if what, id, err := ParseOpenArgs(tc.args); err != nil || what != tc.what || id != tc.id {
			t.Errorf("ParseOpenArgs(%q) = (%q, %d, %v)", tc.args, what, id, err)
		}
	}
	bad := []struct {
		args []string
		want string // exact message; "" = must list all six targets
	}{
		{nil, ""}, {[]string{"bogus"}, ""}, {[]string{"board", "5"}, ""}, {[]string{"spec", "5", "6"}, ""},
		{[]string{"spec", "#541"}, "invalid change id #541"}, {[]string{"spec", "+5"}, "invalid change id +5"},
		{[]string{"spec", "-5"}, "invalid change id -5"}, {[]string{"spec", "0"}, "invalid change id 0"},
		{[]string{"spec", "0x1F"}, "invalid change id 0x1F"},
		{[]string{"spec", "99999999999999999999999"}, "invalid change id 99999999999999999999999"},
	}
	for _, tc := range bad {
		_, _, err := ParseOpenArgs(tc.args)
		if err == nil || strings.Contains(err.Error(), "\n") {
			t.Errorf("ParseOpenArgs(%q) = %v, want a one-line usage error", tc.args, err)
			continue
		}
		if tc.want != "" && err.Error() != tc.want {
			t.Errorf("ParseOpenArgs(%q) = %q, want %q", tc.args, err, tc.want)
		}
		for _, w := range OpenTargets {
			if tc.want == "" && !strings.Contains(err.Error(), w) {
				t.Errorf("ParseOpenArgs(%q) = %q does not list %q", tc.args, err, w)
			}
		}
	}
}

func TestOpenResultRendering(t *testing.T) {
	r := OpenResult{What: "board", Target: "/m/BOARD.md", Notes: []string{openNoteNotGitHub}}
	if r.HumanText() != "/m/BOARD.md" || !reflect.DeepEqual(r.HumanNotes(), r.Notes) {
		t.Errorf("success renders %q / %q", r.HumanText(), r.HumanNotes())
	}
	if r.Message = "boom"; r.HumanText() != "/m/BOARD.md\nboom" { // resolved target, then the failure
		t.Errorf("failure with target renders %q", r.HumanText())
	}
	if r.Target = ""; r.HumanText() != "boom" {
		t.Errorf("failure renders %q", r.HumanText())
	}
	raw, _ := json.Marshal(OpenResult{What: "board", Notes: []string{}})
	if strings.Contains(string(raw), "change_id") || !strings.Contains(string(raw), `"notes":[]`) || !strings.Contains(string(raw), `"launched":false`) {
		t.Errorf("board document = %s, want no change_id, notes [], launched false", raw)
	}
}
```

`internal/app/open_infer_test.go`:

```go
package app

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/repository"
)

// openChange builds a change blob: active = in-progress under active/,
// archived = done under archive/ with a date prefix. extra is raw frontmatter.
func openChange(id int, slug string, archived bool, extra string) StatusBlob {
	status, loc := "in-progress", repository.LocationActive
	path := fmt.Sprintf("docs/changes/active/%04d-%s.md", id, slug)
	if archived {
		status, loc = "done", repository.LocationArchive
		path = fmt.Sprintf("docs/changes/archive/2026-01-03-%04d-%s.md", id, slug)
	}
	fm := fmt.Sprintf("---\nid: %d\nslug: %s\ntitle: Change %d\nstatus: %s\npriority: medium\ntype: feat\ncreated: 2026-01-02\n%s---\n\nBody.\n",
		id, slug, id, status, extra)
	return StatusBlob{Kind: repository.KindChange, Location: loc, Path: path, Revision: fmt.Sprintf("blob%04d%s", id, slug), Data: []byte(fm)}
}

func openSnapshot(t *testing.T, blobs ...StatusBlob) domain.Snapshot {
	t.Helper()
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: testConfig(t).Effective, Documents: inputs})
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	return build.Snapshot
}

func TestInferChangeFromBranch(t *testing.T) {
	w := openChange(7, "widget", false, "branch: feat/widget\n")
	none := func(b string) string { return "branch " + b + " belongs to no change — pass a change id" }
	cases := []struct {
		branch      string
		blobs       []StatusBlob
		wantID      int
		reason, msg string
	}{
		{"feat/widget", []StatusBlob{w}, 7, "", ""},
		{"feat/old", []StatusBlob{w, openChange(3, "old", true, "branch: feat/old\n")}, 3, "", ""},                // archived fallback
		{"feat/widget", []StatusBlob{openChange(4, "widget-old", true, "branch: feat/widget\n"), w}, 7, "", ""}, // active wins over archived
		{"main", []StatusBlob{w}, 0, ReasonOpenBranchUnmatched, none("main")},
		{"docket", []StatusBlob{w}, 0, ReasonOpenBranchUnmatched, none("docket")},                               // metadata checkout
		{"feat/fresh", []StatusBlob{openChange(9, "fresh", false, "")}, 0, ReasonOpenBranchUnmatched, none("feat/fresh")}, // no recorded branch
		{"feat/widget", []StatusBlob{openChange(12, "widget-two", false, "branch: feat/widget\n"), w}, 0,
			ReasonOpenBranchAmbiguous, "branch feat/widget is recorded by changes 7, 12 — pass a change id"},
	}
	for i, tc := range cases {
		c, f := inferChangeFromBranch(openSnapshot(t, tc.blobs...), tc.branch)
		if tc.reason == "" && (f != nil || int(c.ID()) != tc.wantID) {
			t.Errorf("case %d: got (%d, %+v), want change %d", i, c.ID(), f, tc.wantID)
		}
		if tc.reason != "" && (f == nil || f.reason != tc.reason || f.message != tc.msg || f.result != ResultInvalidInput) {
			t.Errorf("case %d: failure = %+v, want %s %q", i, f, tc.reason, tc.msg)
		}
	}
}

func probeReturning(st gitcli.CheckoutState, err error) func(context.Context, string) (gitcli.CheckoutState, error) {
	return func(context.Context, string) (gitcli.CheckoutState, error) { return st, err }
}

func TestCheckoutBranch(t *testing.T) {
	if b, f := checkoutBranch(context.Background(), probeReturning(gitcli.CheckoutState{Branch: "refs/heads/feat/widget"}, nil), "/r"); f != nil || b != "feat/widget" {
		t.Errorf("branch = (%q, %+v)", b, f)
	}
	if _, f := checkoutBranch(context.Background(), probeReturning(gitcli.CheckoutState{Detached: true}, nil), "/r"); f == nil ||
		f.reason != ReasonOpenHeadDetached || f.message != "HEAD is detached — pass a change id" {
		t.Errorf("detached = %+v", f)
	}
}

// TestCheckoutBranchProbeErrorIsNotAbsence (Review Focus 5).
func TestCheckoutBranchProbeErrorIsNotAbsence(t *testing.T) {
	_, f := checkoutBranch(context.Background(), probeReturning(gitcli.CheckoutState{}, errors.New("not a git repository")), "/r")
	if f == nil || f.reason != ReasonOpenCheckoutUnreadable || f.result != ResultExternalFailed {
		t.Fatalf("failure = %+v, want checkout-unreadable external-failed", f)
	}
}
```

- [ ] **Step 2: Verify they fail.** Run: `go test -count=1 ./internal/app/ -run 'TestParseOpenArgs|TestOpenResultRendering|TestInferChangeFromBranch|TestCheckoutBranch'`. Expected: `undefined: ParseOpenArgs`.

- [ ] **Step 3: Implement**

`internal/app/open.go`:

```go
package app

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/danielhanold/docket/internal/gitcli"
)

// (file doc comment; then OperationOpen, OpenTargets, the target-kind and
// reason constants, openNoteNotGitHub, and openNoteStale from Interfaces)

type OpenOptions struct {
	RepoDir string
	What    string
	ID      int // 0 = infer from the checked-out branch
	Print   bool
}

type OpenDeps struct {
	Reader   StatusReader
	Checkout func(ctx context.Context, dir string) (gitcli.CheckoutState, error)
	Prepare  func(ctx context.Context, repoDir string) RepositoryPrepareResult
	Opener   Opener
}

type OpenResult struct {
	Envelope
	What       string   `json:"what"`
	ChangeID   int      `json:"change_id,omitempty"`
	Target     string   `json:"target,omitempty"`
	TargetKind string   `json:"target_kind,omitempty"`
	Launched   bool     `json:"launched"`
	Notes      []string `json:"notes"`
	Reason     string   `json:"reason,omitempty"`
	Message    string   `json:"message,omitempty"`
}

func (r OpenResult) HumanText() string {
	if r.Message == "" {
		return r.Target
	}
	if r.Target != "" {
		return r.Target + "\n" + r.Message
	}
	return r.Message
}

func (r OpenResult) HumanNotes() []string { return r.Notes }

type openFailure struct {
	result  Result
	reason  string
	message string
}

var openIDPattern = regexp.MustCompile(`^[0-9]+$`)

func ParseOpenArgs(args []string) (string, int, error) {
	targets := strings.Join(OpenTargets, ", ")
	switch {
	case len(args) == 0:
		return "", 0, fmt.Errorf("missing target — choose one of %s", targets)
	case len(args) > 2:
		return "", 0, fmt.Errorf("too many arguments — usage: open <target> [id], where <target> is one of %s", targets)
	}
	what := args[0]
	if !slices.Contains(OpenTargets, what) {
		return "", 0, fmt.Errorf("unknown target %q — choose one of %s", what, targets)
	}
	if len(args) == 1 {
		return what, 0, nil
	}
	if what == "board" {
		return "", 0, fmt.Errorf("board takes no change id — the targets are %s, and only the last five take an id", targets)
	}
	raw := args[1]
	n, err := strconv.Atoi(raw)
	if !openIDPattern.MatchString(raw) || err != nil || n <= 0 {
		return "", 0, fmt.Errorf("invalid change id %s", raw)
	}
	return what, n, nil
}
```

`internal/app/open_infer.go`:

```go
package app

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
)

func checkoutBranch(ctx context.Context, probe func(context.Context, string) (gitcli.CheckoutState, error), dir string) (string, *openFailure) {
	st, err := probe(ctx, dir)
	if err != nil {
		return "", &openFailure{ResultExternalFailed, ReasonOpenCheckoutUnreadable,
			fmt.Sprintf("could not read the branch checked out at %s: %v — pass a change id", dir, err)}
	}
	if st.Detached || st.Branch == "" {
		return "", &openFailure{ResultInvalidInput, ReasonOpenHeadDetached, "HEAD is detached — pass a change id"}
	}
	return strings.TrimPrefix(string(st.Branch), branchRefPrefix), nil
}

// inferChangeFromBranch: see the bullets below.
```

`inferChangeFromBranch`: loop `snap.Changes()`; skip any record whose `recordedBranch(c)` errors or differs from `branch`; collect `c.Location() == domain.LocationArchive` into `archived`, the rest into `active`; `matches := active`, falling back to `archived` only when `active` is empty; one match -> it; none -> `ResultInvalidInput`/`ReasonOpenBranchUnmatched` with `branch <b> belongs to no change — pass a change id`; several -> `ResultInvalidInput`/`ReasonOpenBranchAmbiguous` with `branch <b> is recorded by changes <ids ascending, ", "-joined> — pass a change id`.

- [ ] **Step 4: Verify they pass** (same command). Mutation probes (must redden): delete the archived fallback (case 1); merge the lists as `matches := append(active, archived...)` (case 2, the spec's active-before-archived check); fold the probe-error branch into the detached one (`TestCheckoutBranchProbeErrorIsNotAbsence`).

- [ ] **Step 5: Commit**

```bash
git add internal/app/open.go internal/app/open_infer.go internal/app/open_test.go internal/app/open_infer_test.go
git commit -m "feat(app): add docket open's argument grammar, result document, and branch inference"
```

---

### Task 4: The Open operation

**Files:** Modify `internal/app/open.go`; append to `internal/app/open_test.go`.

**Interfaces:**
- Consumes: Tasks 1-3; existing `classifyStatusError`, `parseCorpus`, `repository.BuildSnapshot`, `boardCorpusPath`, `boardSurfaceInline`, `linkContextOf`, `layout.Private`.
- Produces: `func Open(ctx context.Context, deps OpenDeps, o OpenOptions) OpenResult`.

- [ ] **Step 1: Write the failing tests**

Append to `open_test.go` (add imports `context`, `errors`, `fmt`, `os`, `path/filepath`, `config`, `gitcli`, `layout`, `reposetup`, `testsupport`):

```go
const (
	openWeb       = "https://github.com/acme/widget"
	widgetSpec    = "docs/superpowers/specs/2026-01-02-widget-design.md"
	widgetPlan    = "docs/superpowers/plans/2026-01-02-widget.md"
	widgetResults = "docs/results/2026-01-03-widget-results.md"
	widgetPR      = "https://github.com/acme/widget/pull/12"
	oldSpec       = "docs/superpowers/specs/2025-12-01-old-design.md"
)

// 7 fully linked, 3 archived, 8 bare, 9 trivial, 10 shorthand pr.
func openCorpus() []StatusBlob {
	return []StatusBlob{
		openChange(7, "widget", false, "branch: feat/widget\nspec: "+widgetSpec+"\nplan: "+widgetPlan+"\nresults: "+widgetResults+"\npr: '"+widgetPR+"'\n"),
		openChange(3, "old", true, "branch: feat/old\nspec: "+oldSpec+"\npr: 'https://github.com/acme/widget/pull/2'\n"),
		openChange(8, "bare", false, "branch: feat/bare\n"),
		openChange(9, "tiny", false, "branch: feat/tiny\ntrivial: true\n"),
		openChange(10, "short", false, "branch: feat/short\npr: 'acme/widget#12'\n"),
	}
}

func openConfig(t *testing.T, yml string) config.Snapshot {
	t.Helper()
	var src []config.Source
	if yml != "" {
		src = []config.Source{{Layer: config.LayerRepository, Name: ".docket.yml", Data: []byte(yml)}}
	}
	snap, _, err := config.Resolve(src, config.ResolveContext{DefaultBranch: "main"})
	if err != nil {
		t.Fatalf("resolve %q: %v", yml, err)
	}
	return *snap
}

type fakeOpener struct {
	targets []string
	err     error
}

func (o *fakeOpener) Open(_ context.Context, target string) error {
	o.targets = append(o.targets, target)
	return o.err
}

type openFixture struct {
	repoDir, root          string
	reader                 *fakeReader
	prepare                RepositoryPrepareResult
	prepared, checkoutDirs []string
	checkout               gitcli.CheckoutState
	checkoutErr            error
	opener                 *fakeOpener
}

// newOpenFixture: a fake-reader harness whose metadata checkout (the layout's
// MetadataWorktree) is a real temp directory.
func newOpenFixture(t *testing.T, cfgYAML string, private bool, web string, blobs ...StatusBlob) *openFixture {
	t.Helper()
	base := testsupport.TempDir(t)
	lay := layout.SharedLayout(filepath.Join(base, ".git"), base)
	if private {
		lay = layout.PrivateLayout(filepath.Join(base, ".git"), base, filepath.Join(base, "data"), "acme-widget")
	}
	pin := docketPin(t)
	pin.Config, pin.Layout, pin.RepoWebURL = openConfig(t, cfgYAML), lay, web
	return &openFixture{
		repoDir: base, root: lay.MetadataWorktree, reader: &fakeReader{pin: pin, corpus: blobs}, opener: &fakeOpener{},
		prepare: RepositoryPrepareResult{Envelope: NewEnvelope(OperationRepositoryPrepare, ResultNoOp),
			Disposition: PrepareDispositionNoOp, Context: &PrepareContext{MetadataWorktreePath: lay.MetadataWorktree}},
	}
}

func (f *openFixture) deps() OpenDeps {
	return OpenDeps{
		Reader: f.reader,
		Checkout: func(_ context.Context, dir string) (gitcli.CheckoutState, error) {
			f.checkoutDirs = append(f.checkoutDirs, dir)
			return f.checkout, f.checkoutErr
		},
		Prepare: func(_ context.Context, dir string) RepositoryPrepareResult {
			f.prepared = append(f.prepared, dir)
			return f.prepare
		},
		Opener: f.opener,
	}
}

func (f *openFixture) writeFile(t *testing.T, rel string) string {
	t.Helper()
	p := filepath.Join(f.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func (f *openFixture) refusePrepare(code string) {
	f.prepare = RepositoryPrepareResult{Envelope: NewEnvelope(OperationRepositoryPrepare, ResultInvalidState),
		Disposition: PrepareDispositionRefused, Findings: []reposetup.Finding{{Code: code}}}
}

func (f *openFixture) open(what string, id int) OpenResult {
	return Open(context.Background(), f.deps(), OpenOptions{RepoDir: f.repoDir, What: what, ID: id})
}

// TestOpenTargetTable: {shared+github, shared+local, shared non-GitHub origin,
// private with open.artifacts: github} x {board, change active/archived, spec,
// plan, results, pr}.
func TestOpenTargetTable(t *testing.T) {
	targets := []struct {
		what  string
		id    int
		rel   string // repo-relative path, or the PR URL
		isURL bool
	}{
		{"board", 0, "docs/changes/BOARD.md", false},
		{"change", 7, "docs/changes/active/0007-widget.md", false},
		{"change", 3, "docs/changes/archive/2026-01-03-0003-old.md", false},
		{"spec", 7, widgetSpec, false}, {"spec", 3, oldSpec, false},
		{"plan", 7, widgetPlan, false}, {"results", 7, widgetResults, false},
		{"pr", 7, widgetPR, true},
	}
	modes := []struct {
		name, cfg, web string
		private, github bool
		note            string
	}{
		{"shared github", "", openWeb, false, true, ""},
		{"shared local", "open:\n  artifacts: local\n", openWeb, false, false, ""},
		{"shared non-github origin", "", "", false, false, openNoteNotGitHub},
		{"private ignores github", "open:\n  artifacts: github\n", openWeb, true, false, ""},
	}
	for _, m := range modes {
		for _, tg := range targets {
			t.Run(fmt.Sprintf("%s/%s-%d", m.name, tg.what, tg.id), func(t *testing.T) {
				f := newOpenFixture(t, m.cfg, m.private, m.web, openCorpus()...)
				kind, target, notes := OpenTargetKindURL, "", []string{}
				switch {
				case tg.isURL:
					target = tg.rel
				case m.github:
					target = openWeb + "/blob/" + layout.SharedName + "/" + tg.rel
				default:
					kind, target = OpenTargetKindFile, f.writeFile(t, tg.rel)
					if m.note != "" {
						notes = []string{m.note}
					}
				}
				res := f.open(tg.what, tg.id)
				if res.Result != ResultApplied || res.Target != target || res.TargetKind != kind || !reflect.DeepEqual(res.Notes, notes) {
					t.Fatalf("Open = (%s, %q, %s, %q, %q), want (applied, %q, %s, %q)", res.Result, res.Target, res.TargetKind, res.Notes, res.Message, target, kind, notes)
				}
				if !res.Launched || !reflect.DeepEqual(f.opener.targets, []string{target}) {
					t.Errorf("launched=%v opener=%v", res.Launched, f.opener.targets)
				}
				if want := map[bool]int{true: 1, false: 0}[kind == OpenTargetKindFile]; len(f.prepared) != want {
					t.Errorf("prepare ran %d times, want %d", len(f.prepared), want)
				}
				if (tg.what != "board" && res.ChangeID != tg.id) || len(f.checkoutDirs) != 0 {
					t.Errorf("change_id %d, checkout probes %v", res.ChangeID, f.checkoutDirs)
				}
			})
		}
	}
}

func TestOpenErrors(t *testing.T) {
	dup := append(openCorpus(), openChange(7, "widget-copy", false, "branch: feat/widget-copy\n"))
	inv, st := ResultInvalidInput, ResultInvalidState
	cases := []struct {
		what   string
		id     int
		result Result
		reason string
		msg    string // "" = not pinned here
	}{
		{"change", 99, inv, ReasonOpenUnknownChange, "no change 99"},
		{"change", 7, st, ReasonOpenAmbiguousChange, "more than one record claims change 7 — refusing to choose"},
		{"spec", 8, st, ReasonOpenArtifactUnset, "change 8 has no spec yet"},
		{"plan", 8, st, ReasonOpenArtifactUnset, "change 8 has no plan yet"},
		{"results", 8, st, ReasonOpenArtifactUnset, "change 8 has no results yet"},
		{"pr", 8, st, ReasonOpenArtifactUnset, "change 8 has no PR yet"},
		{"spec", 9, st, ReasonOpenTrivialNoSpec, "change 9 is trivial — it has no spec"},
		{"board", 0, st, ReasonOpenBoardDisabled, "this repository has no board (board_surfaces has no inline surface)"},
		{"pr", 10, st, ReasonOpenPRNotURL, "change 10 pr: acme/widget#12 is not a URL"},
		{"bogus", 7, inv, ReasonOpenInvalidTarget, ""},
		{"board", 7, inv, ReasonOpenInvalidTarget, ""},
	}
	for i, tc := range cases {
		cfg, blobs := "", openCorpus()
		if tc.reason == ReasonOpenBoardDisabled {
			cfg = "board_surfaces: []\n"
		}
		if tc.reason == ReasonOpenAmbiguousChange {
			blobs = dup
		}
		f := newOpenFixture(t, cfg, false, openWeb, blobs...)
		f.writeFile(t, "docs/changes/BOARD.md") // a stale board must not rescue a disabled one
		res := f.open(tc.what, tc.id)
		if res.Result != tc.result || res.Reason != tc.reason || (tc.msg != "" && res.Message != tc.msg) ||
			strings.Contains(res.HumanText(), "\n") || len(f.opener.targets) != 0 {
			t.Errorf("case %d: (%s, %s, %q) opener %v, want (%s, %s, %q)", i, res.Result, res.Reason, res.Message, f.opener.targets, tc.result, tc.reason, tc.msg)
		}
	}
}

// TestOpenRefusesArtifactPathOutsideRepository (Review Focus 3). If the domain
// decoder already marks such a value malformed, make changeArtifactPath report
// a malformed field as ReasonOpenPathInvalid; do not loosen this assert.
func TestOpenRefusesArtifactPathOutsideRepository(t *testing.T) {
	for _, bad := range []string{"../../etc/passwd", "/etc/passwd"} {
		for _, cfg := range []string{"", "open:\n  artifacts: local\n"} {
			f := newOpenFixture(t, cfg, false, openWeb, openChange(11, "escape", false, "branch: feat/escape\nplan: '"+bad+"'\n"))
			res := f.open("plan", 11)
			if res.Result != ResultInvalidState || res.Reason != ReasonOpenPathInvalid || len(f.opener.targets) != 0 || len(f.prepared) != 0 {
				t.Errorf("plan %q cfg %q: (%s, %s, %q) opener %v prepare %v", bad, cfg, res.Result, res.Reason, res.Message, f.opener.targets, f.prepared)
			}
		}
	}
}

func TestOpenLocalFreshness(t *testing.T) {
	local := "open:\n  artifacts: local\n"

	f := newOpenFixture(t, local, false, openWeb, openCorpus()...)
	f.root = testsupport.TempDir(t) // prepare's own path wins over the layout's
	f.prepare.Disposition, f.prepare.Context = PrepareDispositionApplied, &PrepareContext{MetadataWorktreePath: f.root}
	want := f.writeFile(t, widgetPlan)
	if res := f.open("plan", 7); res.Result != ResultApplied || res.Target != want || len(res.Notes) != 0 {
		t.Errorf("context path: (%s, %q, %q), want %q", res.Result, res.Target, res.Notes, want)
	}

	f = newOpenFixture(t, local, false, openWeb, openCorpus()...)
	f.refusePrepare("metadata-worktree-dirty")
	want = f.writeFile(t, widgetPlan)
	res := f.open("plan", 7)
	if res.Result != ResultApplied || res.Target != want ||
		!reflect.DeepEqual(res.Notes, []string{"metadata checkout not synced (metadata-worktree-dirty) — file may be stale"}) {
		t.Errorf("refused prepare: (%s, %q, %q)", res.Result, res.Target, res.Notes)
	}

	f = newOpenFixture(t, local, false, openWeb, openCorpus()...)
	f.refusePrepare("metadata-worktree-dirty")
	res = f.open("plan", 7)
	path := filepath.Join(f.root, filepath.FromSlash(widgetPlan))
	if res.Result != ResultInvalidState || res.Reason != ReasonOpenFileMissing || !strings.Contains(res.Message, path) ||
		!strings.Contains(res.Message, "metadata-worktree-dirty") || len(f.opener.targets) != 0 {
		t.Errorf("missing file: (%s, %s, %q)", res.Result, res.Reason, res.Message)
	}

	f = newOpenFixture(t, local, false, openWeb, openCorpus()...)
	if res := f.open("plan", 7); res.Reason != ReasonOpenFileMissing {
		t.Errorf("missing after clean prepare: (%s, %s, %q)", res.Result, res.Reason, res.Message)
	}
}

func TestOpenPrintAndLaunchOutcomes(t *testing.T) {
	f := newOpenFixture(t, "", false, openWeb, openCorpus()...)
	res := Open(context.Background(), f.deps(), OpenOptions{RepoDir: f.repoDir, What: "pr", ID: 7, Print: true})
	if res.Result != ResultApplied || res.Launched || len(f.opener.targets) != 0 || res.Target != widgetPR {
		t.Errorf("--print: (%s, %v, %v, %q)", res.Result, res.Launched, f.opener.targets, res.Target)
	}
	for _, tc := range []struct {
		err    error
		reason string
	}{{&NoOpenerError{GOOS: "plan9"}, ReasonOpenNoOpener}, {errors.New("exit status 4"), ReasonOpenOpenerFailed}} {
		f := newOpenFixture(t, "", false, openWeb, openCorpus()...)
		f.opener.err = tc.err
		res := f.open("pr", 7)
		if res.Result != ResultExternalFailed || res.Reason != tc.reason || res.Launched || !strings.HasPrefix(res.HumanText(), widgetPR+"\n") {
			t.Errorf("%v: (%s, %s, %q)", tc.err, res.Result, res.Reason, res.HumanText())
		}
	}
}

// TestOpenInferenceViaOpen: no id -> the checked-out branch's change, from a
// worktree path and from a relative dir that every seam receives absolute
// (Review Focus 2); detached / probe error fail before any pin.
func TestOpenInferenceViaOpen(t *testing.T) {
	for _, rel := range []bool{false, true} {
		f := newOpenFixture(t, "open:\n  artifacts: local\n", false, openWeb, openCorpus()...)
		f.checkout = gitcli.CheckoutState{Branch: "refs/heads/feat/widget"}
		want, dir := f.writeFile(t, widgetPlan), filepath.Join(f.repoDir, ".worktrees", "widget")
		if rel {
			dir = "."
		}
		res := Open(context.Background(), f.deps(), OpenOptions{RepoDir: dir, What: "plan"})
		if res.Result != ResultApplied || res.ChangeID != 7 || res.Target != want {
			t.Errorf("%s: (%s, %d, %q, %q)", dir, res.Result, res.ChangeID, res.Target, res.Message)
		}
		if len(f.checkoutDirs) != 1 || len(f.prepared) != 1 || !filepath.IsAbs(f.checkoutDirs[0]) || !filepath.IsAbs(f.prepared[0]) {
			t.Errorf("%s: probe %v prepare %v, want one absolute dir each", dir, f.checkoutDirs, f.prepared)
		}
	}
	for _, tc := range []struct {
		state  gitcli.CheckoutState
		err    error
		reason string
	}{
		{gitcli.CheckoutState{Detached: true}, nil, ReasonOpenHeadDetached},
		{gitcli.CheckoutState{}, errors.New("not a git repository"), ReasonOpenCheckoutUnreadable},
	} {
		f := newOpenFixture(t, "", false, openWeb, openCorpus()...)
		f.checkout, f.checkoutErr = tc.state, tc.err
		if res := f.open("plan", 0); res.Reason != tc.reason || f.reader.pinCount != 0 {
			t.Errorf("reason %s pins %d, want %s and no pin", res.Reason, f.reader.pinCount, tc.reason)
		}
	}
}
```

- [ ] **Step 2: Verify they fail.** Run: `go test -count=1 ./internal/app/ -run 'TestOpen'`. Expected: `undefined: Open`.

- [ ] **Step 3: Implement.** Append to `open.go` (add imports `errors`, `io/fs`, `net/url`, `os`, `path/filepath`, `runtime`, `domain`, `layout`, `repository`):

```go
func Open(ctx context.Context, deps OpenDeps, o OpenOptions) OpenResult {
	res := OpenResult{What: o.What, ChangeID: o.ID, Notes: []string{}}
	fail := func(f openFailure) OpenResult {
		res.Envelope = NewEnvelope(OperationOpen, f.result)
		res.Reason, res.Message = f.reason, f.message
		return res
	}
	if !slices.Contains(OpenTargets, o.What) || (o.What == "board" && o.ID != 0) {
		args := []string{o.What}
		if o.ID != 0 {
			args = append(args, strconv.Itoa(o.ID))
		}
		_, _, err := ParseOpenArgs(args)
		return fail(openFailure{ResultInvalidInput, ReasonOpenInvalidTarget, err.Error()})
	}
	if o.RepoDir == "" {
		return fail(openFailure{ResultInvalidInput, ReasonOpenInvalidTarget, "a repository directory is required"})
	}
	dir, err := filepath.Abs(o.RepoDir)
	if err != nil {
		return fail(openFailure{ResultInvalidInput, ReasonOpenInvalidTarget, fmt.Sprintf("cannot resolve %s: %v", o.RepoDir, err)})
	}
	branch := ""
	if o.What != "board" && o.ID == 0 { // before any fetch
		b, f := checkoutBranch(ctx, deps.Checkout, dir)
		if f != nil {
			return fail(*f)
		}
		branch = b
	}
	pin, err := deps.Reader.PinContext(ctx, dir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return fail(openFailure{result, reason, err.Error()})
	}
	eff := pin.Config.Effective
	var rel string
	if o.What == "board" {
		if !slices.Contains(eff.BoardSurfaces.Value, boardSurfaceInline) {
			return fail(openFailure{ResultInvalidState, ReasonOpenBoardDisabled, "this repository has no board (board_surfaces has no inline surface)"})
		}
		rel = boardCorpusPath(eff)
	} else {
		c, f := resolveOpenChange(ctx, deps.Reader, pin, o.ID, branch)
		if f != nil {
			return fail(*f)
		}
		res.ChangeID = int(c.ID())
		if o.What == "pr" {
			u, f := changePRURL(c)
			if f != nil {
				return fail(*f)
			}
			res.Target, res.TargetKind = u, OpenTargetKindURL
			return finishOpen(ctx, deps, o, res)
		}
		if rel, f = changeArtifactPath(c, o.What); f != nil {
			return fail(*f)
		}
	}
	if !filepath.IsLocal(filepath.FromSlash(rel)) {
		return fail(openFailure{ResultInvalidState, ReasonOpenPathInvalid, fmt.Sprintf("%s path %q is not a path inside the repository", o.What, rel)})
	}
	if pin.Layout.Mode != layout.Private && eff.Open.Artifacts.Value != "local" {
		if u := linkContextOf(pin).BlobURL(rel); u != "" {
			res.Target, res.TargetKind = u, OpenTargetKindURL
			return finishOpen(ctx, deps, o, res)
		}
		res.Notes = append(res.Notes, openNoteNotGitHub)
	}
	path, notes, f := localArtifact(ctx, deps, dir, pin, rel)
	res.Notes = append(res.Notes, notes...)
	if f != nil {
		return fail(*f)
	}
	res.Target, res.TargetKind = path, OpenTargetKindFile
	return finishOpen(ctx, deps, o, res)
}

func localArtifact(ctx context.Context, deps OpenDeps, dir string, pin StatusPin, rel string) (string, []string, *openFailure) {
	pr := deps.Prepare(ctx, dir)
	root, code := pin.Layout.MetadataWorktree, ""
	var notes []string
	if pr.Disposition == PrepareDispositionApplied || pr.Disposition == PrepareDispositionNoOp {
		if pr.Context != nil && pr.Context.MetadataWorktreePath != "" {
			root = pr.Context.MetadataWorktreePath
		}
	} else {
		code = prepareFindingCode(pr)
		notes = append(notes, openNoteStale(code))
	}
	path := filepath.Join(root, filepath.FromSlash(rel))
	if _, err := os.Stat(path); err != nil {
		msg := fmt.Sprintf("%s is not in the metadata checkout", path)
		if !errors.Is(err, fs.ErrNotExist) {
			msg = fmt.Sprintf("could not read %s: %v", path, err)
		}
		if code != "" {
			msg += fmt.Sprintf(" (metadata checkout not synced: %s)", code)
		}
		return "", notes, &openFailure{ResultInvalidState, ReasonOpenFileMissing, msg}
	}
	return path, notes, nil
}
```

The remaining helpers, each pinned by Step 1's exact messages:

- `resolveOpenChange(ctx, reader, pin, id, branch) (domain.Change, *openFailure)`: `ReadCorpus` (error -> `classifyStatusError`), `parseCorpus`, `repository.BuildSnapshot` (error -> `ResultInternalError`/`ReasonStatusInternalError`); `id == 0` -> `inferChangeFromBranch(build.Snapshot, branch)`; else `Snapshot.Change(domain.ChangeID(id))`: found -> it; `LookupAmbiguous` -> invalid-state `ReasonOpenAmbiguousChange` `more than one record claims change <id> — refusing to choose`; otherwise invalid-input `ReasonOpenUnknownChange` `no change <id>`. Mirrors `resolvePRChange`.
- `changeArtifactPath(c, what) (string, *openFailure)`: `change` -> `c.Path()`; `spec` -> trivial first (`ReasonOpenTrivialNoSpec`, `change <id> is trivial — it has no spec`), then `c.Spec()`; `plan` -> `c.Plan()`; `results` -> `c.Results()`; a field not `domain.FieldPresent` or blank -> invalid-state `ReasonOpenArtifactUnset` `change <id> has no <what> yet`.
- `changePRURL(c) (string, *openFailure)`: unset -> `ReasonOpenArtifactUnset` `change <id> has no PR yet`; `url.Parse` error, scheme not `http`/`https`, or empty host -> invalid-state `ReasonOpenPRNotURL` `change <id> pr: <value> is not a URL`.
- `prepareFindingCode(pr) string`: the first finding's `Code`, else `pr.RepositoryState`, else `pr.Disposition`.
- `finishOpen(ctx, deps, o, res) OpenResult`: `o.Print` -> applied, not launched; else open through `deps.Opener` (nil -> `&NoOpenerError{GOOS: runtime.GOOS}`); a `*NoOpenerError` -> external-failed `ReasonOpenNoOpener` with its message; any other error -> external-failed `ReasonOpenOpenerFailed` `could not open <target>: <err>`; success -> applied, `Launched = true`. The target stays set on failure so `HumanText` prints it first.

- [ ] **Step 4: Verify they pass.** Run: `go test -count=1 ./internal/app/ -run 'Open|Opener|CheckoutBranch'`. Mutation probes, each must redden: drop `pin.Layout.Mode != layout.Private &&` (private rows gain the note); make the BlobURL branch unconditional (non-github rows); delete the note append; delete the `filepath.IsLocal` check; use the raw dir instead of `filepath.Abs`.

- [ ] **Step 5: Commit**

```bash
git add internal/app/open.go internal/app/open_test.go
git commit -m "feat(app): resolve and open docket artifacts on GitHub or from the metadata checkout"
```

---

### Task 5: The `docket open` command, catalog, schema, presenter notes, reference line

**Files:** Modify `internal/cli/presenter.go` (+ `presenter_test.go`), `internal/cli/root.go`, `internal/cli/install.go`, `internal/cli/capability_production_test.go`, `internal/app/schema_registry.go`, `docs/reference/cli.md`; create `internal/cli/open.go`, `internal/cli/open_test.go`.

**Interfaces:** Consumes the Task 2-4 `app` names plus `app.NewGitStatusReader`, `app.RunRepositoryPrepare`. Produces `newOpenCommand(setResult func(app.OperationResult)) *cobra.Command`; test seams `openRunner`, `openDepsFor`.

- [ ] **Step 1: Write the failing tests**

`presenter_test.go` (add `bytes` import if absent):

```go
type notedResult struct{ app.CLIErrorResult }

func (notedResult) HumanText() string    { return "https://example.test/x" }
func (notedResult) HumanNotes() []string { return []string{"first", "second"} }

func TestPresentHumanNotesGoToStderr(t *testing.T) {
	r := notedResult{app.CLIErrorResult{Envelope: app.NewEnvelope("open", app.ResultApplied)}}
	var out, errBuf bytes.Buffer
	Presenter{Stdout: &out, Stderr: &errBuf}.Present(r)
	if out.String() != "https://example.test/x\n" || errBuf.String() != "note: first\nnote: second\n" {
		t.Errorf("stdout %q stderr %q", out.String(), errBuf.String())
	}
	out.Reset()
	errBuf.Reset()
	if Presenter{Stdout: &out, Stderr: &errBuf, JSON: true}.Present(r); errBuf.Len() != 0 {
		t.Errorf("JSON mode wrote stderr %q", errBuf.String())
	}
}
```

`internal/cli/open_test.go`:

```go
package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/app"
)

func stubOpen(t *testing.T, res app.OpenResult) *[]app.OpenOptions {
	t.Helper()
	var got []app.OpenOptions
	prevRunner, prevDeps := openRunner, openDepsFor
	t.Cleanup(func() { openRunner, openDepsFor = prevRunner, prevDeps })
	openDepsFor = func() (app.OpenDeps, error) { return app.OpenDeps{}, nil }
	openRunner = func(_ context.Context, _ app.OpenDeps, o app.OpenOptions) app.OperationResult {
		got = append(got, o)
		return res
	}
	return &got
}

func openOK(target string, notes ...string) app.OpenResult {
	if notes == nil {
		notes = []string{}
	}
	return app.OpenResult{Envelope: app.NewEnvelope(app.OperationOpen, app.ResultApplied), What: "spec", ChangeID: 541,
		Target: target, TargetKind: app.OpenTargetKindURL, Notes: notes}
}

func TestOpenCLIPassesParsedOptions(t *testing.T) {
	got := stubOpen(t, openOK("https://example.test/s"))
	if _, errS, code := runCLI(t, "open", "spec", "0541", "--print", "--repo-dir", "/tmp/repo"); code != 0 {
		t.Fatalf("code %d stderr %q", code, errS)
	}
	if _, _, code := runCLI(t, "open", "plan", "--repo-dir", "/tmp/repo"); code != 0 {
		t.Fatalf("inference code %d", code)
	}
	want := []app.OpenOptions{{RepoDir: "/tmp/repo", What: "spec", ID: 541, Print: true}, {RepoDir: "/tmp/repo", What: "plan"}}
	if len(*got) != 2 || (*got)[0] != want[0] || (*got)[1] != want[1] {
		t.Fatalf("options = %+v, want %+v", *got, want)
	}
}

// TestOpenCLIPrintNotesGoToStderr (Review Focus 1).
func TestOpenCLIPrintNotesGoToStderr(t *testing.T) {
	stubOpen(t, openOK("/m/docs/spec.md", "origin is not a GitHub remote — opened the local file instead"))
	out, errS, code := runCLI(t, "open", "spec", "7", "--print", "--repo-dir", "/tmp/repo")
	if code != 0 || out != "/m/docs/spec.md\n" || errS != "note: origin is not a GitHub remote — opened the local file instead\n" {
		t.Fatalf("code %d stdout %q stderr %q", code, out, errS)
	}
}

func TestOpenCLIUsageErrors(t *testing.T) {
	got := stubOpen(t, openOK("x"))
	for _, args := range [][]string{{"open"}, {"open", "bogus"}, {"open", "board", "5"}, {"open", "spec", "#541"}, {"open", "spec", "0"}, {"open", "spec", "1", "2"}} {
		if _, errS, code := runCLI(t, args...); code != 2 || strings.Contains(strings.TrimRight(errS, "\n"), "\n") {
			t.Errorf("%q: code %d stderr %q, want exit 2 and one line", args, code, errS)
		}
	}
	_, errS, _ := runCLI(t, "open", "bogus")
	for _, w := range app.OpenTargets {
		if !strings.Contains(errS, w) {
			t.Errorf("unknown-target error %q does not list %q", errS, w)
		}
	}
	if _, errS, _ := runCLI(t, "open", "spec", "#541"); !strings.Contains(errS, "invalid change id #541") {
		t.Errorf("stderr = %q", errS)
	}
	if len(*got) != 0 {
		t.Errorf("usage errors reached the operation: %+v", *got)
	}
}
```

`capability_production_test.go`, `TestRepresentativeSignatures`: add `"open": "<what> [<id>] [--print] [--repo-dir <dir>]",` to `want`, and after the `run.start` effects check:

```go
	if e, ok := entryByID(entries, "open"); !ok || strings.Join(e.Effects, " ") != "local-write process-control read" {
		t.Errorf("open effects = %v, want [local-write process-control read]", e.Effects)
	}
```

- [ ] **Step 2: Verify they fail.** Run: `go test -count=1 ./internal/cli/ -run 'TestPresentHumanNotes|TestOpenCLI|TestRepresentativeSignatures'`. Expected: `undefined: openRunner`.

- [ ] **Step 3: Implement**

`presenter.go`, `Present`'s human branch becomes:

```go
	fmt.Fprintln(p.Stdout, r.HumanText())
	// Notes go to stderr in human mode so stdout stays exactly the text line
	// (`docket open --print` inside command substitution). JSON carries them.
	if n, ok := r.(interface{ HumanNotes() []string }); ok {
		for _, note := range n.HumanNotes() {
			fmt.Fprintf(p.Stderr, "note: %s\n", note)
		}
	}
	return app.ExitCode(r.Env().Result)
```

`internal/cli/open.go`:

```go
package cli

import (
	"context"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
	"github.com/danielhanold/docket/internal/gitcli"
)

var (
	openRunner = func(ctx context.Context, d app.OpenDeps, o app.OpenOptions) app.OperationResult {
		return app.Open(ctx, d, o)
	}
	openDepsFor = func() (app.OpenDeps, error) {
		client, err := gitcli.NewClient()
		if err != nil {
			return app.OpenDeps{}, err
		}
		return app.OpenDeps{
			Reader:   app.NewGitStatusReader(client),
			Checkout: client.WorktreeCheckoutState,
			Prepare: func(ctx context.Context, dir string) app.RepositoryPrepareResult {
				return app.RunRepositoryPrepare(ctx, app.SetupDeps{Git: client, RepoDir: dir}, app.PrepareOptions{})
			},
			Opener: app.NewSystemOpener(runtime.GOOS, exec.LookPath, app.RunOpenerProcess),
		}, nil
	}
)

func newOpenCommand(setResult func(app.OperationResult)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "open <what> [<id>]",
		Short: "Open the board, or a change's record, spec, plan, results, or pull request",
		Long: `Without an id, opens the change whose recorded branch is checked out. Shared
repositories open GitHub pages unless open.artifacts is local; private ones open
the metadata checkout's file; pr opens the pull request.`,
		Args: func(_ *cobra.Command, args []string) error {
			_, _, err := app.ParseOpenArgs(args)
			return err
		},
		Annotations: capability("open", EffectRead, EffectLocalWrite, EffectProcessControl),
		RunE: func(c *cobra.Command, args []string) error {
			what, id, err := app.ParseOpenArgs(args)
			if err != nil {
				return err
			}
			repoDir, err := resolveRepoDir(c)
			if err != nil {
				return err
			}
			printOnly, _ := c.Flags().GetBool("print")
			deps, err := openDepsFor()
			if err != nil {
				return err
			}
			setResult(openRunner(c.Context(), deps, app.OpenOptions{RepoDir: repoDir, What: what, ID: id, Print: printOnly}))
			return nil
		},
	}
	cmd.Flags().Bool("print", false, "print the URL or path without opening it")
	cmd.Flags().String("repo-dir", "", "repository `dir` to operate on (default: current directory)")
	return cmd
}
```

`root.go`: `openCmd := newOpenCommand(func(r app.OperationResult) { result = r })` beside the other families; add `openCmd` to `root.AddCommand(...)` after `maintenanceCmd`.
`install.go` `assetIndependent`: `"open": true, // reads the corpus and launches the system opener; no installed assets`.
`schema_registry.go` `operationBindings`, between `maintenance.sweep` and `pr.publish`: `{ID: "open", Request: nil, Result: OpenResult{}}, // Open`.
`docs/reference/cli.md`, between the `docket maintenance` and `docket pr` lines:

```markdown
- **`docket open`** — open the board, or a change's record, spec, plan, results, or pull request,
  as a GitHub page or from the local metadata checkout. Without an id it opens the change whose
  branch is checked out. Details: `docket open --help`.
```

gofmt the edited Go files.

- [ ] **Step 4: Verify they pass.** Run: `go test -count=1 ./internal/cli/ ./internal/app/ -run 'Present|OpenCLI|Signatures|Correspondence|AssetIndependent|PayloadWithinByteBudget|OperationBindings'` (catalog stays under 16 KB), then `go test -count=1 ./internal/repoguard/` (capability surface, living docs). Mutation: delete the `HumanNotes` loop -> both notes tests redden.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/presenter.go internal/cli/presenter_test.go internal/cli/open.go internal/cli/open_test.go \
  internal/cli/root.go internal/cli/install.go internal/cli/capability_production_test.go \
  internal/app/schema_registry.go docs/reference/cli.md
git commit -m "feat(cli): add docket open"
```

---

### Task 6: Integration: local opens against a real private repository

**Files:** Create `internal/app/open_integration_test.go`, `tests/test_go_integration_app_open.sh`; modify `tests/runtime-budgets.tsv`.

**Interfaces:** Consumes `Open`, `OpenDeps`, `fakeOpener` (untagged `open_test.go`, compiled under `-tags integration` too); helpers `newPrivateInitRepo`, `(*initRepo).runInitWith`, `planningDepsFor`, `seedBuildReadyPrivate`, `resolvedPrivateLayout`, `runGit`; `RunRepositoryPrepare`.

- [ ] **Step 1: Write the tests**

`internal/app/open_integration_test.go` (line 1 the tag, line 2 blank; the integration contract checks both):

```go
//go:build integration

package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/layout"
)

// docket open's local opens over a real private repository (prefix
// TestIntegrationOpenArtifact). Not parallel: the helpers use t.Setenv.

type openPrivateFixture struct {
	node realNode
	lay  layout.Layout
	id   int
}

// newOpenPrivateFixture: init --private, then one change created and groomed
// with a spec (the groom commit, the metadata tip, adds the spec).
func newOpenPrivateFixture(t *testing.T) *openPrivateFixture {
	t.Helper()
	r, _ := newPrivateInitRepo(t, nil)
	if res := r.runInitWith(t, InitOptions{Private: true}); res.Result != ResultApplied {
		t.Fatalf("init --private = %q (%s)", res.Result, res.HumanText())
	}
	node := planningDepsFor(t, r.invocation)
	id, _ := seedBuildReadyPrivate(t, node)
	return &openPrivateFixture{node: node, lay: resolvedPrivateLayout(t, r.invocation), id: id}
}

func (f *openPrivateFixture) open(t *testing.T) OpenResult {
	t.Helper()
	client := f.node.deps.Client
	deps := OpenDeps{
		Reader:   f.node.deps.Reader,
		Checkout: client.WorktreeCheckoutState,
		Prepare: func(ctx context.Context, dir string) RepositoryPrepareResult {
			return RunRepositoryPrepare(ctx, SetupDeps{Git: client, RepoDir: dir}, PrepareOptions{})
		},
		Opener: &fakeOpener{},
	}
	return Open(context.Background(), deps, OpenOptions{RepoDir: f.node.dir, What: "spec", ID: f.id, Print: true})
}

// rewind resets the checkout to the commit before the metadata tip (before
// the spec existed) and returns the tip. Both preconditions are asserted so
// neither can pass vacuously.
func (f *openPrivateFixture) rewind(t *testing.T) string {
	t.Helper()
	wt := f.lay.MetadataWorktree
	runGit(t, wt, "fetch", "-q", f.lay.DefaultBareRemote, f.lay.MetadataBranch)
	tip := runGit(t, wt, "rev-parse", "FETCH_HEAD")
	if !strings.Contains(runGit(t, wt, "ls-tree", "-r", "--name-only", tip), "-design.md") {
		t.Fatalf("precondition: the metadata tip %s carries no spec", tip)
	}
	runGit(t, wt, "reset", "-q", "--hard", tip+"~1")
	if strings.Contains(runGit(t, wt, "ls-files"), "-design.md") {
		t.Fatalf("precondition: the rewound checkout still has the spec")
	}
	return tip
}

func (f *openPrivateFixture) dirty(t *testing.T) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(f.lay.MetadataWorktree, "scratch.txt"), []byte("edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// One fixture, three phases: a stale checkout missing the file fails naming
// the path and prepare's finding; once clean, a behind checkout is
// fast-forwarded before the open; a dirty one opens with the stale note.
func TestIntegrationOpenArtifactLocalOpenSyncsFirst(t *testing.T) {
	f := newOpenPrivateFixture(t)
	wt := f.lay.MetadataWorktree
	tip := f.rewind(t)
	f.dirty(t)
	res := f.open(t)
	if res.Result != ResultInvalidState || res.Reason != ReasonOpenFileMissing || !strings.Contains(res.Message, wt) ||
		!strings.Contains(res.Message, "-design.md") || !strings.Contains(res.Message, "metadata-worktree-dirty") {
		t.Fatalf("missing: (%s, %s, %q), want file-missing naming the spec path and the finding", res.Result, res.Reason, res.Message)
	}

	if err := os.Remove(filepath.Join(wt, "scratch.txt")); err != nil {
		t.Fatal(err)
	}
	res = f.open(t)
	if res.Result != ResultApplied || res.TargetKind != OpenTargetKindFile || len(res.Notes) != 0 ||
		!strings.HasPrefix(res.Target, wt+string(filepath.Separator)) || !strings.HasSuffix(res.Target, "-design.md") {
		t.Fatalf("behind: (%s, %q, %q, %q), want the spec inside %s, no note", res.Result, res.Target, res.Notes, res.Message, wt)
	}
	if _, err := os.Stat(res.Target); err != nil {
		t.Errorf("target missing: %v", err)
	}
	if head := runGit(t, wt, "rev-parse", "HEAD"); head != tip {
		t.Errorf("checkout HEAD = %s, want fast-forwarded to %s", head, tip)
	}

	f.dirty(t)
	res = f.open(t)
	want := "metadata checkout not synced (metadata-worktree-dirty) — file may be stale"
	if res.Result != ResultApplied || len(res.Notes) != 1 || res.Notes[0] != want {
		t.Fatalf("dirty: (%s, %q, %q), want applied with %q", res.Result, res.Notes, res.Message, want)
	}
}
```

If prepare does not read the rewound checkout as strictly behind, investigate the real topology and fix the fixture; never weaken the fast-forward assert.

`tests/test_go_integration_app_open.sh` (mode 0755): a copy of `tests/test_go_integration_app_runrecord.sh` with `SHARD_PREFIX="TestIntegrationOpenArtifact"` and its header comment describing this shard.

`tests/runtime-budgets.tsv`: add `tests/test_go_integration_app_open.sh<TAB>30<TAB>parallel` beside the other app shards; measure the shard serially (`time bash tests/test_go_integration_app_open.sh`) and set the ceiling to the measurement rounded up to the next multiple of 5 plus 5s (minimum 10s).

- [ ] **Step 2: Run.** `bash tests/test_go_integration_app_open.sh` -> `ok`. Then `bash tests/test_go_integration_contract.sh` and `go test -count=1 ./internal/repoguard/ -run TestRuntimeBudgetsCorrespondence` -> green. Mutation: empty `localArtifact`'s `else` branch -> `TestIntegrationOpenArtifactLocalOpenSyncsFirst` reddens (`go test -tags integration -count=1 -run '^TestIntegrationOpenArtifact' ./internal/app/`).

- [ ] **Step 3: Commit**

```bash
git add internal/app/open_integration_test.go tests/test_go_integration_app_open.sh tests/runtime-budgets.tsv
git commit -m "test(app): prove docket open's local opens over a real private repository"
```

---

## Build gate

The whole suite runs once at the gate; read the budget report even on green. Spec coverage: §1 T3/T5, §2 T4, §3 T3/T4, §4 T4/T6, §5 T1/T4, §6 T2-T4, §7 T1/T5, §8 T3/T4; mutation checks T3/T4.
