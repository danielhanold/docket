package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/layout"
	"github.com/danielhanold/docket/internal/reposetup"
	"github.com/danielhanold/docket/internal/testsupport"
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
	full, _ := json.Marshal(OpenResult{What: "spec", ChangeID: 7, Target: "/m/spec.md", TargetKind: "file", Launched: true, Notes: []string{"n1"}})
	var doc map[string]any
	if err := json.Unmarshal(full, &doc); err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]any{"what": "spec", "change_id": float64(7), "target": "/m/spec.md", "target_kind": "file", "launched": true, "notes": []any{"n1"}} {
		if !reflect.DeepEqual(doc[k], want) {
			t.Errorf("success document %s = %#v, want %#v (doc %s)", k, doc[k], want, full)
		}
	}
}

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
		name, cfg, web  string
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
