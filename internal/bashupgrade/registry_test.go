package bashupgrade

// The step registry: one action per `<!-- upgrade-step: <name> -->` block in
// docs/release/upgrading-from-bash.md. runGuide drives the guide's marked blocks in
// guide order through these actions against a restored saved case.
//
// Three kinds of action keep the guide honest:
//   - run actions execute only the block's own text (`/bin/sh -e -c <body>`) and
//     never add a command the guide does not show;
//   - observe actions run the block's command with --json to read its findings,
//     and check that the guide's tables explain every one;
//   - mirror actions do by hand exactly what the block and its prose tell the reader
//     to do (download, edit a file, delete paths), after asserting the guide text
//     names the object they act on.
//
// Each action also asserts the claims the guide's prose makes about that step, so a
// change in the binary's behavior turns the test red instead of leaving the guide
// silently wrong.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// runState carries what earlier steps observed to later ones. InstallAttempts holds
// every install hand-off in order; Conflicts the paths the first failed attempt
// named; Observed the finding codes each observe step saw.
type runState struct {
	InstallAttempts []cmdResult
	Conflicts       []string
	Observed        map[string][]findingRef

	Guide            string   // the full guide text, for prose and table checks
	Cwd              string   // the reader's terminal directory after the last block
	DownloadDir      string   // where install-binary downloaded install.sh
	InstallLine      string   // install-binary's installer invocation, re-run by takeover-remedy
	InstallHarnesses []string // the --harness names of InstallLine, in line order
	CheckJSON        string   // repo-check's --json stdout
	Settings         []string // settings-table patterns config-cleanup matched to a finding
	Removed          []string // settings config-cleanup removed from .docket.yml

	DispatchBlockRemoved bool // dispatch-block found and removed the CLAUDE.md block
	RepoAgentFilesShadow int  // repository agent files that hid an installed agent before repo-agent-files
	RepoSettingsSeen     bool // the repository carried migrate's .claude/settings.local.json
}

type stepAction func(t *testing.T, c *upgradeCase, st *runState, body string)

var stepRegistry = map[string]stepAction{
	"install-binary":         mirrorInstallBinary,
	"cursor-takeover-remedy": mirrorCursorTakeoverRemedy,
	"takeover-remedy":        mirrorTakeoverRemedy,
	"global-config-cleanup":  mirrorGlobalConfigCleanup,
	"confirm-install":        runConfirmInstall,
	"repo-prepare":           runRepoPrepare,
	"repo-check":             observeRepoCheck,
	"fix-gitignore":          mirrorFixGitignore,
	"config-cleanup":         mirrorConfigCleanup,
	"commit-fixes":           runCommitFixes,
	"configure-tests":        runPlain,
	"configure-harnesses":    runPlain,
	"commit-config":          runCommit,
	"repair-preview":         observeRepairPreview,
	"repair-apply":           runRepairApply,
	"repo-confirm":           runRepoConfirm,
	"dispatch-block":         mirrorDispatchBlock,
	"choose-harnesses":       runChooseHarnesses,
	"leftovers":              mirrorLeftovers,
	"repo-agent-files":       runRepoAgentFiles,
}

// Guide headings the table checks anchor on.
const (
	findingTableHeading = "## 5. Upgrade each repository"
	settingTableHeading = "## 6. Settings that changed"
)

// settingCodes are the finding codes the settings table must explain.
var settingCodes = map[string]bool{
	"deferred-capability-requested": true,
	"obsolete-setting":              true,
	"inert-setting":                 true,
}

// guidePath is the guide under test: $DOCKET_BASH_UPGRADE_GUIDE when set (the
// by-hand mutation-check seam), else the committed guide.
func guidePath(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("DOCKET_BASH_UPGRADE_GUIDE"); p != "" {
		return p
	}
	return filepath.Join(repoRoot(t), "docs", "release", "upgrading-from-bash.md")
}

func readGuide(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(guidePath(t))
	if err != nil {
		t.Fatalf("read guide: %v", err)
	}
	return string(b)
}

// runGuide runs every marked step of the guide, in guide order, against c.
func runGuide(t *testing.T, c *upgradeCase) *runState {
	t.Helper()
	src := readGuide(t)
	g, err := parseGuide(src)
	if err != nil {
		t.Fatalf("parse guide: %v", err)
	}
	st := &runState{Observed: map[string][]findingRef{}, Guide: src, Cwd: c.Home}
	for _, s := range g.Steps {
		act, ok := stepRegistry[s.Name]
		if !ok {
			t.Fatalf("guide step %q (line %d) has no registry action", s.Name, s.Line)
		}
		body, err := substitutePlaceholders(s.Body, map[string]string{"repo": c.Clone})
		if err != nil {
			t.Fatalf("guide step %q (line %d): %v", s.Name, s.Line, err)
		}
		t.Logf("guide step %s (line %d)", s.Name, s.Line)
		act(t, c, st, body)
	}
	return st
}

// ---- shared helpers ---------------------------------------------------------------

func blockLines(body string) []string {
	var out []string
	for _, l := range strings.Split(body, "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, strings.TrimRight(l, " \t"))
		}
	}
	return out
}

// expandHome expands a leading ~ the way the reader's shell does.
func (c *upgradeCase) expandHome(p string) string {
	if p == "~" {
		return c.Home
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(c.Home, p[2:])
	}
	return p
}

// trackCd follows every `cd <dir>` segment of a block so the next block starts where
// the reader's terminal is.
func (c *upgradeCase) trackCd(st *runState, body string) {
	for _, line := range blockLines(body) {
		for _, seg := range strings.Split(line, "&&") {
			f := strings.Fields(seg)
			if len(f) == 2 && f[0] == "cd" {
				st.Cwd = c.expandHome(f[1])
			}
		}
	}
}

// runBlock executes the block's own text with /bin/sh -e in the reader's current
// directory and fails the test on a non-zero exit.
func runBlock(t *testing.T, c *upgradeCase, st *runState, body string) cmdResult {
	t.Helper()
	r := c.run(t, st.Cwd, "/bin/sh", "-e", "-c", body)
	if r.Code != 0 {
		t.Fatalf("guide block exited %d (in %s)\nblock:\n%s\nstdout:\n%s\nstderr:\n%s", r.Code, st.Cwd, body, r.Stdout, r.Stderr)
	}
	c.trackCd(st, body)
	return r
}

func mustContain(t *testing.T, what, text, want string) {
	t.Helper()
	if !strings.Contains(text, want) {
		t.Fatalf("%s does not contain %q:\n%s", what, want, text)
	}
}

// guideHarnesses are the harnesses the guide's combined path covers; the test runs
// every saved case once, through all of them.
var guideHarnesses = []string{"claude", "cursor"}

// installLineHarnesses parses the guide's installer invocation: exactly `sh install.sh`
// followed by one or more `--harness <name>` pairs, each name one the downloader
// accepts and none repeated. It returns the names in line order.
func installLineHarnesses(line string) ([]string, bool) {
	f := strings.Fields(line)
	if len(f) < 4 || len(f)%2 != 0 || f[0] != "sh" || f[1] != "install.sh" {
		return nil, false
	}
	known := map[string]bool{"claude": true, "codex": true, "cursor": true, "opencode": true}
	seen := map[string]bool{}
	var out []string
	for i := 2; i < len(f); i += 2 {
		if f[i] != "--harness" || !known[f[i+1]] || seen[f[i+1]] {
			return nil, false
		}
		seen[f[i+1]] = true
		out = append(out, f[i+1])
	}
	return out, true
}

// containsProse reports whether text contains want once every whitespace run in both
// is collapsed to one space, so a re-flowed guide paragraph still matches the claim.
func containsProse(text, want string) bool {
	return strings.Contains(strings.Join(strings.Fields(text), " "), strings.Join(strings.Fields(want), " "))
}

func mustContainProse(t *testing.T, what, text, want string) {
	t.Helper()
	if !containsProse(text, want) {
		t.Fatalf("%s does not contain %q (whitespace collapsed):\n%s", what, want, text)
	}
}

// pendingPathsLead introduces the paths `docket repository configure-harnesses` asks
// the reader to review and commit, in its human output.
const pendingPathsLead = "review and commit the pending paths: "

// pendingPathsFrom returns, sorted, the paths after pendingPathsLead up to the end of
// that line, or nil when the output carries no such list.
func pendingPathsFrom(out string) []string {
	_, rest, ok := strings.Cut(out, pendingPathsLead)
	if !ok {
		return nil
	}
	line, _, _ := strings.Cut(rest, "\n")
	var paths []string
	for _, p := range strings.Split(line, ",") {
		if p = strings.TrimSpace(p); p != "" {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	return paths
}

// sameSet reports whether a and b hold the same strings, ignoring order.
func sameSet(a, b []string) bool {
	x, y := append([]string(nil), a...), append([]string(nil), b...)
	sort.Strings(x)
	sort.Strings(y)
	return strings.Join(x, "\x00") == strings.Join(y, "\x00") && len(x) == len(y)
}

// findingObjects decodes one JSON document and returns, in document order, every
// object carrying string code and severity fields (the shape findingCodes reads).
func findingObjects(t *testing.T, stdout string) []map[string]any {
	t.Helper()
	var doc any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("decode JSON output: %v\n%s", err, stdout)
	}
	var out []map[string]any
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			_, okCode := x["code"].(string)
			_, okSev := x["severity"].(string)
			if okCode && okSev {
				out = append(out, x)
			}
			keys := make([]string, 0, len(x))
			for k := range x {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				walk(x[k])
			}
		case []any:
			for _, e := range x {
				walk(e)
			}
		}
	}
	walk(doc)
	return out
}

func str(m map[string]any, k string) string {
	s, _ := m[k].(string)
	return s
}

// guideTable returns the body rows (cells trimmed) of the first table after heading,
// stopping at the next "## " heading.
func guideTable(t *testing.T, src, heading string) [][]string {
	t.Helper()
	lines := strings.Split(src, "\n")
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == heading {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatalf("guide has no %q heading", heading)
	}
	var rows [][]string
	inTable := false
	for _, l := range lines[start+1:] {
		if strings.HasPrefix(l, "## ") {
			break
		}
		if !strings.HasPrefix(l, "|") {
			if inTable {
				break
			}
			continue
		}
		if !inTable {
			inTable = true
			continue // header row
		}
		cells := strings.Split(strings.Trim(strings.TrimSpace(l), "|"), "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		if strings.HasPrefix(cells[0], "---") {
			continue
		}
		rows = append(rows, cells)
	}
	if len(rows) == 0 {
		t.Fatalf("guide has no table under %q", heading)
	}
	return rows
}

func backticked(s string) []string {
	var out []string
	for _, m := range regexp.MustCompile("`([^`]+)`").FindAllStringSubmatch(s, -1) {
		out = append(out, m[1])
	}
	return out
}

// findingTable maps each finding code in the section-5 table to its row.
func findingTable(t *testing.T, src string) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	for _, row := range guideTable(t, src, findingTableHeading) {
		codes := backticked(row[0])
		if len(codes) != 1 || len(row) != 3 {
			t.Fatalf("finding table row %q must name exactly one `code` and have 3 cells", strings.Join(row, " | "))
		}
		out[codes[0]] = row
	}
	return out
}

type settingRow struct {
	Patterns       []string
	Severity       string // error | warning | notice
	Does, Fix, Raw string
}

func settingTable(t *testing.T, src string) []settingRow {
	t.Helper()
	var out []settingRow
	for _, row := range guideTable(t, src, settingTableHeading) {
		if len(row) != 3 {
			t.Fatalf("settings table row %q must have 3 cells", strings.Join(row, " | "))
		}
		pats := backticked(row[0])
		word := strings.ToLower(strings.TrimRight(strings.Fields(row[1])[0], ":."))
		if len(pats) == 0 || (word != "error" && word != "warning" && word != "notice") {
			t.Fatalf("settings table row %q needs `setting` names and a Error/Warning/Notice first word", strings.Join(row, " | "))
		}
		out = append(out, settingRow{Patterns: pats, Severity: word, Does: row[1], Fix: row[2], Raw: strings.Join(row, " | ")})
	}
	return out
}

// settingMatches reports whether a dotted setting name matches a table pattern,
// where a <name> segment stands for any one segment.
func settingMatches(pattern, field string) bool {
	p, f := strings.Split(pattern, "."), strings.Split(field, ".")
	if len(p) != len(f) {
		return false
	}
	for i := range p {
		if strings.HasPrefix(p[i], "<") && strings.HasSuffix(p[i], ">") {
			continue
		}
		if p[i] != f[i] {
			return false
		}
	}
	return true
}

// snapshotTree records every path under dir with its link target or content hash.
func snapshotTree(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			target, err := os.Readlink(p)
			if err != nil {
				return err
			}
			out[p] = "link:" + target
		case d.Type().IsRegular():
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(b)
			out[p] = "file:" + hex.EncodeToString(sum[:])
		default:
			out[p] = "dir"
		}
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", dir, err)
	}
	return out
}

// harnessHomesSnapshot snapshots every harness folder the guide's install writes.
func harnessHomesSnapshot(t *testing.T, c *upgradeCase) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, d := range []string{".claude", ".cursor"} {
		for k, v := range snapshotTree(t, filepath.Join(c.Home, d)) {
			out[k] = v
		}
	}
	return out
}

func gitRev(t *testing.T, c *upgradeCase, ref string) string {
	t.Helper()
	return strings.TrimSpace(c.mustGit(t, c.Origin, "rev-parse", ref))
}

// ---- section 3 and 4: the machine install ------------------------------------------

// repairPrepareOptionalProse is the guide's statement that the prepare step repair
// mentions is optional.
const repairPrepareOptionalProse = "That step is optional: every docket workflow runs `docket repository prepare` first, which brings the folder up to date."

// releaseVerifiedProse is the guide's statement that the download and checksum
// lines rest on the release verification, not on this test.
const releaseVerifiedProse = "The download and checksum lines are checked by the release's own verification, not by\nthis guide's test."

// mirrorInstallBinary mirrors the release download block. The download and the
// checksum check need the published release, which the release verification proves
// (and the guide says so); here every line must be one of those known shapes, and the
// installer invocation is mirrored by installHandoff.
func mirrorInstallBinary(t *testing.T, c *upgradeCase, st *runState, body string) {
	t.Helper()
	mustContain(t, "guide", st.Guide, releaseVerifiedProse)
	var sawInstallSh, sawChecksums, sawVerify bool
	for _, line := range blockLines(body) {
		f := strings.Fields(line)
		switch {
		case len(f) == 6 && f[0] == "mkdir" && f[1] == "-p" && f[3] == "&&" && f[4] == "cd" && f[2] == f[5]:
			dir := c.expandHome(f[2])
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			st.Cwd = dir
			st.DownloadDir = dir
		case strings.HasPrefix(line, "VERSION="):
		case f[0] == "curl" && strings.HasSuffix(line, `/install.sh"`):
			sawInstallSh = true
		case f[0] == "curl" && strings.HasSuffix(line, `/checksums.txt"`):
			sawChecksums = true
		case strings.Contains(line, "checksums.txt") && strings.Contains(line, "shasum -a 256 -c"):
			sawVerify = true
		case f[0] == "sh":
			hs, ok := installLineHarnesses(line)
			if !ok || !sameSet(hs, guideHarnesses) {
				t.Fatalf("install-binary: installer line %q must be `sh install.sh` with one --harness per covered harness %v", line, guideHarnesses)
			}
			st.InstallLine, st.InstallHarnesses = line, hs
		default:
			t.Fatalf("install-binary: line %q is not a shape the test mirrors", line)
		}
	}
	if st.DownloadDir == "" || !sawInstallSh || !sawChecksums || !sawVerify || st.InstallLine == "" {
		t.Fatalf("install-binary block must make a download folder, fetch install.sh and checksums.txt, verify with shasum -a 256 -c, and run the installer for every covered harness:\n%s", body)
	}
	installHandoff(t, c, st)
}

// installHandoff mirrors internal/release/downloader/install.sh from the point it
// has a verified binary: stage it beside the destination, run the staged binary's
// `install` with one `--harness` per harness on the guide's installer line, and only on success move it into place, place the
// relative dckt alias beside it, write the ownership record (naming the alias), and
// run `docket install check`.
func installHandoff(t *testing.T, c *upgradeCase, st *runState) {
	t.Helper()
	dest := filepath.Join(c.BinDir, "docket")
	alias := filepath.Join(c.BinDir, "dckt")
	for _, p := range []string{dest, alias} {
		if _, err := os.Lstat(p); err == nil {
			t.Fatalf("%s already exists; the mirror covers only the downloader's fresh-install path", p)
		}
	}
	if err := os.MkdirAll(c.BinDir, 0o755); err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(c.BinDir, ".docket-stage")
	bin, err := os.ReadFile(c.Docket)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stage, bin, 0o755); err != nil {
		t.Fatal(err)
	}
	args := []string{"install"}
	for _, h := range st.InstallHarnesses {
		args = append(args, "--harness", h)
	}
	before := harnessHomesSnapshot(t, c)
	// The downloader runs the staged binary in human mode, so the reader sees these
	// lines; the guide quotes them.
	r := c.run(t, st.DownloadDir, stage, args...)
	st.InstallAttempts = append(st.InstallAttempts, r)
	human := r.Stdout + r.Stderr
	if r.Code != 0 {
		// Guide: "this first run stops with `install: invalid-state` and a list of
		// `conflict` lines".
		mustContain(t, "guide", st.Guide, "`install: invalid-state`")
		mustContain(t, "install output", human, "install: invalid-state")
		// A failed run changes nothing, so a --json re-run reads the same conflict
		// set in a form the test can parse; the human output must list each path on
		// a conflict line.
		j := c.run(t, st.DownloadDir, stage, append([]string{"--json"}, args...)...)
		_ = os.Remove(stage)
		var conflicts []string
		var doc struct {
			Result  string `json:"result"`
			Actions []struct {
				Op, Path, Detail string
			} `json:"actions"`
		}
		if err := json.Unmarshal([]byte(j.Stdout), &doc); err != nil {
			t.Fatalf("install --json output: %v\nstdout:\n%s\nstderr:\n%s", err, j.Stdout, j.Stderr)
		}
		for _, a := range doc.Actions {
			if a.Op == "conflict" {
				conflicts = append(conflicts, a.Path)
			}
		}
		if j.Code == 0 || len(conflicts) == 0 || doc.Result != "invalid-state" {
			t.Fatalf("install exited %d (json %d) with result %q and %d conflict paths\nhuman:\n%s\njson:\n%s", r.Code, j.Code, doc.Result, len(conflicts), human, j.Stdout)
		}
		for _, p := range conflicts {
			if !humanConflictLine(human, p) {
				t.Errorf("install's human output has no `conflict` line naming %s:\n%s", p, human)
			}
		}
		if t.Failed() {
			t.FailNow()
		}
		if len(st.InstallAttempts) > 1 {
			t.Fatalf("install still conflicts after the guide's remedy: %v", conflicts)
		}
		// Guide: "The run changes nothing and leaves no docket command behind."
		if _, err := os.Lstat(dest); err == nil {
			t.Fatalf("a failed install left %s behind", dest)
		}
		if after := harnessHomesSnapshot(t, c); !mapsEqual(before, after) {
			t.Fatalf("a failed install changed ~/.claude or ~/.cursor")
		}
		st.Conflicts = conflicts
		return
	}
	// Guide: "This time the installer reports `install: applied`."
	mustContain(t, "guide", st.Guide, "`install: applied`")
	mustContain(t, "install output", human, "install: applied")
	if err := os.Rename(stage, dest); err != nil {
		t.Fatal(err)
	}
	// The downloader's fresh-install alias step: `ln -s docket "$bin_dir/dckt"`.
	if err := os.Symlink("docket", alias); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(bin)
	recDir := filepath.Join(c.Home, ".local", "state", "docket")
	if err := os.MkdirAll(recDir, 0o755); err != nil {
		t.Fatal(err)
	}
	rec := "path=" + dest + "\nversion=development\nalias=" + alias + "\nsha256=" + hex.EncodeToString(sum[:]) + "\n"
	if err := os.WriteFile(filepath.Join(recDir, "release-binary.record"), []byte(rec), 0o644); err != nil {
		t.Fatal(err)
	}
	if chk := c.run(t, st.DownloadDir, dest, "install", "check"); chk.Code != 0 {
		t.Fatalf("install check after install exited %d\nstdout:\n%s\nstderr:\n%s", chk.Code, chk.Stdout, chk.Stderr)
	}
}

// humanConflictLine reports whether human-mode install output has a line whose first
// word is "conflict" and which names path.
func humanConflictLine(out, path string) bool {
	for _, l := range strings.Split(out, "\n") {
		f := strings.Fields(l)
		if len(f) > 0 && strings.TrimRight(f[0], ":") == "conflict" && strings.Contains(l, path) {
			return true
		}
	}
	return false
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// mirrorTakeoverRemedy runs the block's delete commands, proves they removed every
// path the installer named, and re-runs the installer the way the block's last line
// does.
func mirrorTakeoverRemedy(t *testing.T, c *upgradeCase, st *runState, body string) {
	t.Helper()
	lines := blockLines(body)
	if len(lines) < 2 || lines[len(lines)-1] != st.InstallLine {
		t.Fatalf("takeover-remedy must end by re-running %q:\n%s", st.InstallLine, body)
	}
	mustContain(t, "takeover-remedy block", body, "~/.claude/skills/docket-*")
	for _, l := range lines[:len(lines)-1] {
		if !strings.HasPrefix(l, "rm ") {
			t.Fatalf("takeover-remedy line %q is not a delete the test mirrors", l)
		}
	}
	if len(st.Conflicts) == 0 {
		t.Logf("takeover-remedy: the first install had no conflict; nothing to do")
		return
	}
	// Guide: every docket-* link under ~/.claude/skills/ is a conflict; every agent
	// file except v0.9.3's docket-plan-writer.md is taken over.
	conflict := map[string]bool{}
	for _, p := range st.Conflicts {
		conflict[p] = true
	}
	links, _ := filepath.Glob(filepath.Join(c.Home, ".claude", "skills", "docket-*"))
	for _, l := range links {
		if !conflict[l] {
			t.Errorf("guide says every ~/.claude/skills/docket-* link conflicts, but %s did not", l)
		}
	}
	planWriter := filepath.Join(c.Home, ".claude", "agents", "docket-plan-writer.md")
	for _, p := range st.Conflicts {
		if strings.HasPrefix(p, filepath.Join(c.Home, ".claude", "agents")+"/") && p != planWriter {
			t.Errorf("guide says agent files are taken over, but %s conflicted", p)
		}
	}
	if conflict[planWriter] != (c.Tag == "v0.9.3") {
		t.Errorf("guide says docket-plan-writer.md conflicts on v0.9.3 only; %s conflict = %v", c.Tag, conflict[planWriter])
	}
	r := c.run(t, st.DownloadDir, "/bin/sh", "-e", "-c", strings.Join(lines[:len(lines)-1], "\n"))
	if r.Code != 0 {
		t.Fatalf("takeover-remedy deletes exited %d\nstdout:\n%s\nstderr:\n%s", r.Code, r.Stdout, r.Stderr)
	}
	for _, p := range st.Conflicts {
		if _, err := os.Lstat(p); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("takeover-remedy left the conflict path %s in place", p)
		}
	}
	if t.Failed() {
		t.FailNow()
	}
	installHandoff(t, c, st)
	if last := st.InstallAttempts[len(st.InstallAttempts)-1]; last.Code != 0 {
		t.Fatalf("the install after takeover-remedy exited %d", last.Code)
	}
}

// The Cursor paths the guide's cursor-takeover-remedy block deletes, and the guide's
// claims about them.
const (
	cursorSkillsGlob           = "~/.cursor/skills/docket-*"
	cursorPlanWriter           = "~/.cursor/agents/docket-plan-writer.md"
	cursorUserRule             = "~/.cursor/rules/docket-dispatch.mdc"
	cursorRuleSelfRemovedProse = "On `v0.9.2` the installer removes that rule itself."
	cursorRemedySkipProse      = "If you don't use Cursor, skip this block."
	cursorSandboxLink          = "](../install/cursor.md)"
)

// mirrorCursorTakeoverRemedy runs the block's Cursor deletes after proving the first
// install's Cursor conflicts are exactly the ones the guide lists: every
// ~/.cursor/skills/docket-* link, plus, on v0.9.3 only, docket-plan-writer.md and Bash's
// user-level rule. On v0.9.2 the rule is present but no conflict, which is the
// installer's side of the guide's "removes that rule itself" (the block's rm -f gets
// there first; internal/install's TestRetireCursorFileLegacyBytes proves the removal).
// The installer re-run stays in takeover-remedy, so this block must come first.
func mirrorCursorTakeoverRemedy(t *testing.T, c *upgradeCase, st *runState, body string) {
	t.Helper()
	lines := blockLines(body)
	if len(lines) == 0 {
		t.Fatalf("cursor-takeover-remedy block is empty")
	}
	for _, l := range lines {
		if !strings.HasPrefix(l, "rm ") {
			t.Fatalf("cursor-takeover-remedy line %q is not a delete the test mirrors; the installer re-run belongs to takeover-remedy", l)
		}
	}
	for _, p := range []string{cursorSkillsGlob, cursorPlanWriter, cursorUserRule} {
		mustContain(t, "cursor-takeover-remedy block", body, p)
	}
	mustContainProse(t, "guide", st.Guide, cursorRuleSelfRemovedProse)
	mustContainProse(t, "guide", st.Guide, cursorRemedySkipProse)
	if len(st.Conflicts) == 0 {
		t.Fatalf("cursor-takeover-remedy: the first install reported no conflict; the guide says every %s link conflicts", cursorSkillsGlob)
	}
	links, _ := filepath.Glob(c.expandHome(cursorSkillsGlob))
	if len(links) == 0 {
		t.Fatalf("population floor: the saved %s home has no %s link", c.Tag, cursorSkillsGlob)
	}
	planWriter, rule := c.expandHome(cursorPlanWriter), c.expandHome(cursorUserRule)
	want := map[string]bool{}
	for _, l := range links {
		want[l] = true
	}
	if c.Tag == "v0.9.3" {
		want[planWriter], want[rule] = true, true
	} else if _, err := os.Lstat(rule); err != nil {
		t.Errorf("guide says the installer removes Bash's rule itself on %s, but the saved home has no %s: %v", c.Tag, rule, err)
	}
	got := map[string]bool{}
	for _, p := range st.Conflicts {
		if strings.HasPrefix(p, filepath.Join(c.Home, ".cursor")+"/") {
			got[p] = true
		}
	}
	for p := range want {
		if !got[p] {
			t.Errorf("guide says %s conflicts on %s; the installer did not report it", p, c.Tag)
		}
	}
	for p := range got {
		if !want[p] {
			t.Errorf("the installer reported %s as a conflict on %s, which the guide does not list", p, c.Tag)
		}
	}
	if t.Failed() {
		t.FailNow()
	}
	runBlock(t, c, st, body)
	for p := range want {
		if _, err := os.Lstat(p); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("cursor-takeover-remedy left the conflict path %s in place", p)
		}
	}
}

// mirrorGlobalConfigCleanup deletes the marked runtime.bash block from the global
// config, after checking it is the block the guide shows.
func mirrorGlobalConfigCleanup(t *testing.T, c *upgradeCase, st *runState, body string) {
	t.Helper()
	want := blockLines(body)
	if len(want) != 4 || want[0] != "# >>> docket (runtime.bash) >>>" || want[1] != "runtime:" ||
		!strings.HasPrefix(want[2], "  bash: ") || want[3] != "# <<< docket (runtime.bash) <<<" {
		t.Fatalf("global-config-cleanup block is not the runtime.bash block:\n%s", body)
	}
	mustContain(t, "guide", st.Guide, "`~/.config/docket/config.yml`")
	path := filepath.Join(c.Home, ".config", "docket", "config.yml")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("guide says Bash docket left runtime.bash in %s: %v", path, err)
	}
	lines := strings.Split(string(b), "\n")
	start, end := -1, -1
	for i, l := range lines {
		switch l {
		case want[0]:
			start = i
		case want[3]:
			end = i
		}
	}
	if start < 0 || end != start+3 || lines[start+1] != want[1] || !strings.HasPrefix(lines[start+2], "  bash: ") {
		t.Fatalf("%s holds no runtime.bash block shaped like the guide's:\n%s", path, b)
	}
	out := append(append([]string{}, lines[:start]...), lines[end+1:]...)
	if err := os.WriteFile(path, []byte(strings.Join(out, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
}

// runConfirmInstall runs the block; the guide promises both commands succeed and
// install check prints no warning.
func runConfirmInstall(t *testing.T, c *upgradeCase, st *runState, body string) {
	t.Helper()
	r := runBlock(t, c, st, body)
	for _, l := range strings.Split(r.Stdout+r.Stderr, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "warning:") {
			t.Fatalf("confirm-install printed a warning the guide says it does not:\n%s%s", r.Stdout, r.Stderr)
		}
	}
}

// ---- section 5: each repository ----------------------------------------------------

func runPlain(t *testing.T, c *upgradeCase, st *runState, body string) {
	t.Helper()
	runBlock(t, c, st, body)
}

func runRepoPrepare(t *testing.T, c *upgradeCase, st *runState, body string) {
	t.Helper()
	r := runBlock(t, c, st, body)
	if st.Cwd != c.Clone {
		t.Fatalf("repo-prepare must leave the reader in the repository; cwd %s", st.Cwd)
	}
	mustContain(t, "repository prepare output", r.Stdout+r.Stderr, "healthy")
}

// observeRepoCheck runs the check with --json, records its findings, and requires
// the guide's finding table to explain each one.
func observeRepoCheck(t *testing.T, c *upgradeCase, st *runState, body string) {
	t.Helper()
	if strings.TrimSpace(body) != "docket repository check" {
		t.Fatalf("repo-check block must be exactly `docket repository check`:\n%s", body)
	}
	r := c.run(t, st.Cwd, "docket", "--json", "repository", "check")
	if r.Code == 0 {
		t.Fatalf("guide says the first repository check exits with an error; it exited 0\n%s", r.Stdout)
	}
	st.CheckJSON = r.Stdout
	st.Observed["repo-check"] = findingCodes(t, r.Stdout)
	table := findingTable(t, st.Guide)
	for _, f := range st.Observed["repo-check"] {
		if _, ok := table[f.Code]; !ok {
			t.Errorf("repository check reported %s (%s), which the guide's finding table does not explain", f.Code, f.Severity)
		}
	}
}

// mirrorFixGitignore replaces the .gitignore docket block with the guide's lines,
// after checking they are exactly the lines the check's remedy printed.
func mirrorFixGitignore(t *testing.T, c *upgradeCase, st *runState, body string) {
	t.Helper()
	want := blockLines(body)
	var remedy string
	for _, f := range findingObjects(t, st.CheckJSON) {
		if str(f, "code") == "committed-ignore-invalid" {
			remedy = str(f, "remedy")
		}
	}
	if remedy == "" {
		t.Logf("fix-gitignore: repository check did not report committed-ignore-invalid; nothing to do")
		return
	}
	_, printed, ok := strings.Cut(remedy, ":\n")
	if !ok || strings.Join(blockLines(printed), "\n") != strings.Join(want, "\n") {
		t.Fatalf("the guide's .gitignore block differs from the lines the check printed\nguide:\n%s\ncheck:\n%s", strings.Join(want, "\n"), printed)
	}
	path := filepath.Join(c.Clone, ".gitignore")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(b), "\n")
	start, end := -1, -1
	for i, l := range lines {
		switch {
		case strings.HasPrefix(l, "# docket:start"):
			if start >= 0 {
				t.Fatalf(".gitignore has two docket:start lines")
			}
			start = i
		case l == "# docket:end":
			if end >= 0 {
				t.Fatalf(".gitignore has two docket:end lines")
			}
			end = i
		}
	}
	if start < 0 || end <= start {
		t.Fatalf(".gitignore has no well-formed docket block:\n%s", b)
	}
	out := append(append(append([]string{}, lines[:start]...), want...), lines[end+1:]...)
	if err := os.WriteFile(path, []byte(strings.Join(out, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
}

// mirrorConfigCleanup reads `docket status`, requires the settings table to explain
// every setting finding with the same severity, and removes each error and warning
// setting from .docket.yml the way the table's Fix column says.
func mirrorConfigCleanup(t *testing.T, c *upgradeCase, st *runState, body string) {
	t.Helper()
	if strings.TrimSpace(body) != "docket status" {
		t.Fatalf("config-cleanup block must be exactly `docket status`:\n%s", body)
	}
	rows := settingTable(t, st.Guide)
	r := c.run(t, st.Cwd, "docket", "--json", "status")
	var remove []string
	for _, f := range findingObjects(t, r.Stdout) {
		if !settingCodes[str(f, "code")] {
			continue
		}
		field := str(f, "field")
		if field == "" {
			field = str(f, "path")
		}
		var row *settingRow
		for i := range rows {
			for _, p := range rows[i].Patterns {
				if settingMatches(p, field) {
					row = &rows[i]
					st.Settings = append(st.Settings, p)
				}
			}
		}
		if row == nil {
			t.Errorf("docket status reports %s (%s) for %s, which the guide's settings table does not list", str(f, "code"), str(f, "severity"), field)
			continue
		}
		if row.Severity != str(f, "severity") {
			t.Errorf("settings table says %s is %s; docket status reports %s", field, row.Severity, str(f, "severity"))
		}
		checkSettingFix(t, *row, f, field)
		if row.Severity == "notice" {
			continue
		}
		if !strings.HasPrefix(str(f, "message"), ".docket.yml:") {
			t.Errorf("%s is reported outside .docket.yml (%s); the guide only tells the reader to edit .docket.yml here", field, str(f, "message"))
		}
		remove = append(remove, field)
	}
	if t.Failed() {
		t.FailNow()
	}
	if len(remove) == 0 {
		t.Logf("config-cleanup: no setting to remove")
		return
	}
	path := filepath.Join(c.Clone, ".docket.yml")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("settings to remove %v but no .docket.yml: %v", remove, err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil || len(doc.Content) != 1 {
		t.Fatalf("parse .docket.yml: %v", err)
	}
	for _, field := range remove {
		if !removeYAMLKey(doc.Content[0], strings.Split(field, ".")) {
			t.Fatalf("config-cleanup: %s is reported but not found in .docket.yml", field)
		}
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	st.Removed = remove
	// Guide: status reads the pushed .docket.yml, so the edit alone clears nothing.
	still := map[string]bool{}
	for _, f := range findingObjects(t, c.run(t, st.Cwd, "docket", "--json", "status").Stdout) {
		still[str(f, "field")+str(f, "path")] = true
	}
	for _, field := range remove {
		if !still[field] {
			t.Errorf("guide says docket status reads the pushed .docket.yml, but the uncommitted edit already cleared %s", field)
		}
	}
}

// runCommitFixes commits and pushes the .gitignore and settings fixes, then checks
// the guide's claim that the pushed edit clears every setting it removed.
func runCommitFixes(t *testing.T, c *upgradeCase, st *runState, body string) {
	t.Helper()
	runCommit(t, c, st, body)
	if len(st.Removed) == 0 {
		return
	}
	after := c.run(t, st.Cwd, "docket", "--json", "status")
	for _, f := range findingObjects(t, after.Stdout) {
		code, sev := str(f, "code"), str(f, "severity")
		if (settingCodes[code] && sev != "notice") || strings.Contains(code, "config") {
			t.Errorf("after pushing the settings fix, docket status still reports %s (%s): %s", code, sev, str(f, "message"))
		}
	}
}

// checkSettingFix binds the table's Fix and What-docket-does cells to the binary's
// own words: every alternative value the Fix cell offers appears in the binary's
// remedy, and each notice's description matches the binary's message.
func checkSettingFix(t *testing.T, row settingRow, f map[string]any, field string) {
	t.Helper()
	remedy, msg := str(f, "remedy"), str(f, "message")
	if row.Severity != "notice" && !strings.HasPrefix(row.Fix, "Remove it") {
		t.Errorf("settings row %q: the Fix the test mirrors is removal, so the cell must start with \"Remove it\"", row.Raw)
	}
	for _, tok := range backticked(row.Fix) {
		switch {
		case tok == "~/.config/docket/config.yml":
			mustContain(t, "remedy for "+field, remedy, "global docket configuration")
		case !strings.Contains(tok, "/"):
			mustContain(t, "remedy for "+field, remedy, tok)
		}
	}
	if strings.Contains(row.Does, "no effect") {
		mustContain(t, "message for "+field, msg, "no effect")
	}
	if strings.Contains(row.Does, "read only once") {
		mustContain(t, "message for "+field, msg, "read only once")
	}
}

// removeYAMLKey deletes path from mapping m, then drops every mapping the deletion
// left empty. It reports whether the key was found.
func removeYAMLKey(m *yaml.Node, path []string) bool {
	if m.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value != path[0] {
			continue
		}
		if len(path) == 1 {
			m.Content = append(m.Content[:i], m.Content[i+2:]...)
			return true
		}
		child := m.Content[i+1]
		if !removeYAMLKey(child, path[1:]) {
			return false
		}
		if child.Kind == yaml.MappingNode && len(child.Content) == 0 {
			m.Content = append(m.Content[:i], m.Content[i+2:]...)
		}
		return true
	}
	return false
}

// runCommit runs a commit block. The guide shows these blocks unconditionally, so a
// clean tree means the steps before it did nothing the guide says they do: fatal.
// A step the guide says to skip when there is nothing to commit handles that case
// itself and never reaches here with a clean tree.
func runCommit(t *testing.T, c *upgradeCase, st *runState, body string) {
	t.Helper()
	if strings.TrimSpace(c.mustGit(t, st.Cwd, "status", "--porcelain")) == "" {
		t.Fatalf("commit block has nothing to commit, but the guide runs it unconditionally:\n%s", body)
	}
	before := gitRev(t, c, "main")
	runBlock(t, c, st, body)
	if gitRev(t, c, "main") == before {
		t.Fatalf("commit block did not push a new commit to main")
	}
	if left := strings.TrimSpace(c.mustGit(t, st.Cwd, "status", "--porcelain")); left != "" {
		t.Fatalf("commit block left changes behind:\n%s", left)
	}
}

// observeRepairPreview runs the preview with no terminal; the guide says it lists the
// repairs and asks for confirmation, and answering no (end of input) writes nothing.
func observeRepairPreview(t *testing.T, c *upgradeCase, st *runState, body string) {
	t.Helper()
	if strings.TrimSpace(body) != "docket repository repair" {
		t.Fatalf("repair-preview block must be exactly `docket repository repair`:\n%s", body)
	}
	before := gitRev(t, c, "docket")
	r := c.run(t, st.Cwd, "/bin/sh", "-c", body)
	out := r.Stdout + r.Stderr
	mustContain(t, "repair preview", out, "preview")
	mustContain(t, "repair preview", out, "repair? [y/N]")
	if r.Code != 0 {
		mustContain(t, "repair preview", out, "confirmation required")
	}
	if gitRev(t, c, "docket") != before {
		t.Fatalf("the repair preview changed the docket branch")
	}
}

// runRepairApply runs the block and checks the guide's claims: the repair is pushed
// to the docket branch, the output names `docket repository prepare`, which the guide
// calls optional, and the local .docket copy is left behind the remote (so the
// healthy check that follows proves a behind-only copy is healthy).
func runRepairApply(t *testing.T, c *upgradeCase, st *runState, body string) {
	t.Helper()
	before := gitRev(t, c, "docket")
	r := runBlock(t, c, st, body)
	after := gitRev(t, c, "docket")
	if after == before {
		t.Fatalf("guide says repair pushes to the docket branch; origin/docket did not move")
	}
	mustContain(t, "repair output", r.Stdout+r.Stderr, "docket repository prepare")
	mustContain(t, "guide", st.Guide, repairPrepareOptionalProse)
	local := strings.TrimSpace(c.mustGit(t, filepath.Join(st.Cwd, ".docket"), "rev-parse", "HEAD"))
	if local == after {
		t.Fatalf("the local .docket copy is already at the repaired tip; the guide's claim that prepare is optional is untested")
	}
}

// runRepoConfirm checks the guide's claim that `docket repository check` alone
// reports healthy right after the repair, with no `prepare` step in between.
func runRepoConfirm(t *testing.T, c *upgradeCase, st *runState, body string) {
	t.Helper()
	if strings.TrimSpace(body) != "docket repository check" {
		t.Fatalf("repo-confirm block must be exactly `docket repository check`:\n%s", body)
	}
	r := runBlock(t, c, st, body)
	mustContain(t, "repository check output", r.Stdout+r.Stderr, "repository check: no-op (healthy)")
}

// The lines that open and close the dispatch block Bash docket wrote into a
// repository's CLAUDE.md. The opening line carries a trailing note after the stem.
const (
	dispatchBlockStart = "<!-- docket:dispatch:start"
	dispatchBlockEnd   = "<!-- docket:dispatch:end -->"
	// dispatchBlockSkip and dispatchBlockSkipWhy are the guide's instruction for a
	// repository without the block, which mirrorDispatchBlock proves.
	dispatchBlockSkip    = "If there is no such block, skip this step."
	dispatchBlockSkipWhy = "There is nothing to commit, and `git commit` stops with an error."
	// bashDispatchSignature is what marks a dispatch block as Bash docket's: its body
	// runs docket.sh. docket's own block shares the marker lines but never this.
	bashDispatchSignature = "/docket.sh "
)

// dispatchBlockSpan validates the dispatch block's markers in lines and returns the
// index of its opening and closing line, or -1, -1 when neither marker is present.
// A dangling, duplicated or out-of-order marker is an error, so the caller refuses
// before writing anything and the removal never runs to the end of the file.
func dispatchBlockSpan(lines []string) (int, int, error) {
	start, end := -1, -1
	for i, l := range lines {
		switch {
		case strings.HasPrefix(l, dispatchBlockStart):
			if start >= 0 {
				return -1, -1, fmt.Errorf("two %q lines", dispatchBlockStart)
			}
			start = i
		case l == dispatchBlockEnd:
			if end >= 0 {
				return -1, -1, fmt.Errorf("two %q lines", dispatchBlockEnd)
			}
			end = i
		}
	}
	if (start < 0) != (end < 0) || end < start {
		return -1, -1, fmt.Errorf("dispatch block markers are unbalanced or out of order (start line %d, end line %d)", start+1, end+1)
	}
	return start, end, nil
}

// mirrorDispatchBlock deletes the Bash dispatch block from the repository's CLAUDE.md
// the way the prose says (the file too, when nothing else is left), then runs the
// block to commit and push it. On a case without the block it asserts the guide's
// skip instruction holds: nothing to commit, and the block as written fails.
func mirrorDispatchBlock(t *testing.T, c *upgradeCase, st *runState, body string) {
	t.Helper()
	mustContain(t, "guide", st.Guide, "`"+dispatchBlockStart+"`")
	mustContain(t, "guide", st.Guide, "`"+dispatchBlockEnd+"`")
	mustContain(t, "guide", st.Guide, "If that leaves `CLAUDE.md` empty")
	if st.Cwd != c.Clone {
		t.Fatalf("dispatch-block runs in the repository; the reader is in %s", st.Cwd)
	}
	path := filepath.Join(c.Clone, "CLAUDE.md")
	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	lines := strings.Split(string(b), "\n")
	start, end, err := dispatchBlockSpan(lines)
	if err != nil {
		t.Fatalf("CLAUDE.md: %v; leaving it untouched", err)
	}
	if start < 0 {
		// Guide: with no block there is nothing to commit, so the reader skips the
		// step, because the block's `git commit -am` stops with an error on a clean
		// tree. Prove both halves: the tree is clean, and the block as written fails
		// and pushes nothing.
		mustContain(t, "guide", st.Guide, dispatchBlockSkip)
		mustContain(t, "guide", st.Guide, dispatchBlockSkipWhy)
		if left := strings.TrimSpace(c.mustGit(t, st.Cwd, "status", "--porcelain")); left != "" {
			t.Fatalf("dispatch-block found no block but the repository has changes:\n%s", left)
		}
		before := gitRev(t, c, "main")
		if r := c.run(t, st.Cwd, "/bin/sh", "-e", "-c", body); r.Code == 0 {
			t.Fatalf("guide says git commit stops with an error on a clean tree; the dispatch-block block exited 0\nstdout:\n%s\nstderr:\n%s", r.Stdout, r.Stderr)
		}
		if gitRev(t, c, "main") != before {
			t.Fatalf("the skipped dispatch-block block moved origin main")
		}
		t.Logf("dispatch-block: the repository has no dispatch block; the step is skipped")
		return
	}
	// Guide: the upgrade leaves the block in place and `docket repository check`
	// does not report it. repo-confirm already saw `healthy` with the block present.
	mustContain(t, "guide", st.Guide, "`docket.sh`")
	if !strings.Contains(strings.Join(lines[start:end+1], "\n"), bashDispatchSignature) {
		t.Errorf("the guide says the block tells Claude to run Bash docket's docket.sh; it does not")
	}
	out := append(append([]string{}, lines[:start]...), lines[end+1:]...)
	rest := strings.Join(out, "\n")
	if strings.TrimSpace(rest) == "" {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	} else if err := os.WriteFile(path, []byte(rest), 0o644); err != nil {
		t.Fatal(err)
	}
	runCommit(t, c, st, body)
	// The block is gone from the working copy and from the pushed default branch.
	after, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	if s, _, err := dispatchBlockSpan(strings.Split(string(after), "\n")); err != nil || s >= 0 {
		t.Fatalf("CLAUDE.md still carries the dispatch block after dispatch-block")
	}
	if r := c.run(t, c.Origin, "git", "show", "main:CLAUDE.md"); r.Code == 0 && strings.Contains(r.Stdout, "docket:dispatch:") {
		t.Fatalf("origin main:CLAUDE.md still carries the dispatch block after dispatch-block")
	}
	st.DispatchBlockRemoved = true
}

// The guide's final section-5 step: choose the agents, then commit what the command
// lists. The Cursor rule it writes is ignored, so only the listed paths are committed.
const (
	chooseHarnessesLine  = "docket repository configure-harnesses --harnesses claude,cursor"
	cursorRepoRule       = ".cursor/rules/docket-dispatch.mdc"
	cursorRepoRuleProse  = "For Cursor, it writes `.cursor/rules/docket-dispatch.mdc` into the repository, which the `.gitignore` block keeps out of commits."
	claudeRepoBlockProse = "For Claude Code, it writes docket's own block into `CLAUDE.md`."
)

// runChooseHarnesses runs the block and checks the guide's claims: the git add line
// names exactly the paths configure-harnesses lists, the commit reaches origin main and
// leaves nothing behind, .docket.yml there records both harnesses, CLAUDE.md there
// carries docket's dispatch block (not Bash's), and the repository Cursor rule exists
// and is ignored by git.
func runChooseHarnesses(t *testing.T, c *upgradeCase, st *runState, body string) {
	t.Helper()
	lines := blockLines(body)
	if len(lines) != 4 || lines[0] != chooseHarnessesLine || !strings.HasPrefix(lines[1], "git add ") ||
		!strings.HasPrefix(lines[2], "git commit -m ") || lines[3] != "git push" {
		t.Fatalf("choose-harnesses block must be `%s`, `git add <paths>`, `git commit -m …` and `git push`:\n%s", chooseHarnessesLine, body)
	}
	mustContainProse(t, "guide", st.Guide, cursorRepoRuleProse)
	mustContainProse(t, "guide", st.Guide, claudeRepoBlockProse)
	if st.Cwd != c.Clone {
		t.Fatalf("choose-harnesses runs in the repository; the reader is in %s", st.Cwd)
	}
	if left := strings.TrimSpace(c.mustGit(t, st.Cwd, "status", "--porcelain")); left != "" {
		t.Fatalf("choose-harnesses starts from a clean repository; git status shows:\n%s", left)
	}
	before := gitRev(t, c, "main")
	r := runBlock(t, c, st, body)
	pending := pendingPathsFrom(r.Stdout + r.Stderr)
	added := strings.Fields(strings.TrimPrefix(lines[1], "git add "))
	if len(pending) == 0 || !sameSet(pending, added) {
		t.Fatalf("the guide's git add line names %v; configure-harnesses asked to commit %v\nstdout:\n%s\nstderr:\n%s", added, pending, r.Stdout, r.Stderr)
	}
	if gitRev(t, c, "main") == before {
		t.Fatalf("choose-harnesses did not push a new commit to main")
	}
	if left := strings.TrimSpace(c.mustGit(t, st.Cwd, "status", "--porcelain")); left != "" {
		t.Fatalf("choose-harnesses left changes behind:\n%s", left)
	}
	if fi, err := os.Lstat(filepath.Join(c.Clone, filepath.FromSlash(cursorRepoRule))); err != nil || !fi.Mode().IsRegular() {
		t.Fatalf("configure-harnesses did not write %s into the repository (%v)", cursorRepoRule, err)
	}
	if ig := c.run(t, c.Clone, "git", "check-ignore", "-q", "--", cursorRepoRule); ig.Code != 0 {
		t.Errorf("guide says the .gitignore block keeps %s out of commits; git does not ignore it", cursorRepoRule)
	}
	var cfg struct {
		AgentHarnesses []string `yaml:"agent_harnesses"`
	}
	if err := yaml.Unmarshal([]byte(c.mustGit(t, c.Origin, "show", "main:.docket.yml")), &cfg); err != nil {
		t.Fatalf("parse main:.docket.yml: %v", err)
	}
	if !sameSet(cfg.AgentHarnesses, guideHarnesses) {
		t.Errorf("origin main:.docket.yml records agent_harnesses %v; the guide chooses %v", cfg.AgentHarnesses, guideHarnesses)
	}
	claudeLines := strings.Split(c.mustGit(t, c.Origin, "show", "main:CLAUDE.md"), "\n")
	start, end, err := dispatchBlockSpan(claudeLines)
	switch {
	case err != nil || start < 0:
		t.Errorf("origin main:CLAUDE.md has no docket dispatch block after choose-harnesses (%v)", err)
	case strings.Contains(strings.Join(claudeLines[start:end+1], "\n"), bashDispatchSignature):
		t.Errorf("origin main:CLAUDE.md carries Bash docket's dispatch block after choose-harnesses, not docket's")
	}
}

// ---- section 7: leftovers ----------------------------------------------------------

// mirrorLeftovers deletes each leftover the block lists, after proving it exists and
// that `docket install check` does not mention it. It also checks the guide's claims
// about the old Bash checkout.
func mirrorLeftovers(t *testing.T, c *upgradeCase, st *runState, body string) {
	t.Helper()
	chk := c.run(t, c.Home, "docket", "--json", "install", "check")
	for _, line := range blockLines(body) {
		path, desc, _ := strings.Cut(line, "  ")
		desc = strings.TrimSpace(desc)
		full := c.expandHome(path)
		if strings.Contains(chk.Stdout+chk.Stderr, full) {
			t.Errorf("docket install check mentions %s, which the guide calls an unused leftover", path)
		}
		switch path {
		case "~/.claude/settings.json":
			removeSettingsEnv(t, full, desc)
		case "~/.zshenv":
			removeMarkedLines(t, full, desc)
		default:
			t.Fatalf("leftovers line %q names a path the test does not mirror", line)
		}
	}
	// Guide: nothing under ~/.claude or ~/.cursor points into the old checkout; the
	// other tools' links under ~/.codex and ~/.agents still do. The guide states that
	// using docket from those tools on an upgraded repository is not supported, rather
	// than advising to keep using Bash docket there: the test observes only the links.
	mustContain(t, "guide", st.Guide, "`~/dev/docket`")
	mustContainProse(t, "guide", st.Guide, "on an upgraded repository is not supported")
	checkout := filepath.Join(c.Home, "dev", "docket")
	for _, d := range []string{".claude", ".cursor"} {
		mustContain(t, "guide", st.Guide, "`~/"+d+"`")
		if n := linksInto(t, filepath.Join(c.Home, d), checkout); n != 0 {
			t.Errorf("guide says nothing under ~/%s points into the old checkout; %d links do", d, n)
		}
	}
	for _, d := range []string{".codex", ".agents"} {
		mustContain(t, "guide", st.Guide, "`~/"+d+"`")
		if linksInto(t, filepath.Join(c.Home, d), checkout) == 0 {
			t.Errorf("guide says links under ~/%s still point into the old checkout; none do", d)
		}
	}
	// Guide: docket must run outside Cursor's sandbox, with a link to the Cursor page.
	mustContain(t, "guide", st.Guide, cursorSandboxLink)
	if _, err := os.Stat(filepath.Join(repoRoot(t), "docs", "install", "cursor.md")); err != nil {
		t.Errorf("the guide links %s, which does not exist: %v", cursorSandboxLink, err)
	}
}

// The per-repository leftovers the guide names. repoAgentGlob is the exact block line
// the test requires; repoSettings is the file the guide says it does not cover.
const (
	repoAgentGlob           = "rm -f .claude/agents/docket-*.md"
	repoSettings            = ".claude/settings.local.json"
	repoCursorAgentGlob     = "rm -f .cursor/agents/docket-*.md"
	repoCursorAgentUnproven = "so the test does not prove the `.cursor/agents` line"
)

// runRepoAgentFiles runs the block that deletes the repository's Bash agent files.
// Before it runs, every such file must be ignored by git (the guide says there is
// nothing to commit) and at least one must share its name with an agent the installer
// wrote under ~/.claude/agents (the guide says the old files hide those). Afterwards
// none may be left, the repository has nothing to commit, and the repository's
// .claude/settings.local.json, which the guide leaves alone, is unchanged.
func runRepoAgentFiles(t *testing.T, c *upgradeCase, st *runState, body string) {
	t.Helper()
	lines := blockLines(body)
	if len(lines) != 3 || lines[0] != "cd "+c.Clone || lines[1] != repoAgentGlob || lines[2] != repoCursorAgentGlob {
		t.Fatalf("repo-agent-files block must be exactly `cd <repo>`, `%s` and `%s`:\n%s", repoAgentGlob, repoCursorAgentGlob, body)
	}
	mustContainProse(t, "guide", st.Guide, "these old files hide the agents the installer just wrote")
	mustContainProse(t, "guide", st.Guide, "`"+repoSettings+"`")
	mustContainProse(t, "guide", st.Guide, "the test leaves it in place")
	mustContainProse(t, "guide", st.Guide, repoCursorAgentUnproven)

	glob := filepath.Join(c.Clone, ".claude", "agents", "docket-*.md")
	cursorGlob := filepath.Join(c.Clone, ".cursor", "agents", "docket-*.md")
	if found, _ := filepath.Glob(cursorGlob); len(found) > 0 {
		t.Errorf("the saved %s clone carries repository Cursor agent files %v; the guide says the test does not prove the .cursor/agents line, so prove it and drop that sentence", c.Tag, found)
	}
	before, _ := filepath.Glob(glob)
	for _, p := range before {
		rel := strings.TrimPrefix(p, c.Clone+"/")
		if r := c.run(t, c.Clone, "git", "check-ignore", "-q", "--", rel); r.Code != 0 {
			t.Errorf("guide says the .gitignore block keeps %s out of commits; git does not ignore it", rel)
		}
		if _, err := os.Stat(filepath.Join(c.Home, ".claude", "agents", filepath.Base(p))); err == nil {
			st.RepoAgentFilesShadow++
		}
	}
	if len(before) > 0 && st.RepoAgentFilesShadow == 0 {
		t.Errorf("guide says the repository's agent files hide installed agents; none of %d shares a name with ~/.claude/agents", len(before))
	}
	settingsPath := filepath.Join(c.Clone, filepath.FromSlash(repoSettings))
	settingsBefore, settingsErr := os.ReadFile(settingsPath)
	if settingsErr == nil {
		st.RepoSettingsSeen = true
		mustContain(t, repoSettings, string(settingsBefore), "push origin HEAD:")
	}

	runBlock(t, c, st, body)

	for _, g := range []string{glob, cursorGlob} {
		if left, _ := filepath.Glob(g); len(left) > 0 {
			t.Errorf("repo-agent-files left %v behind", left)
		}
	}
	if left := strings.TrimSpace(c.mustGit(t, c.Clone, "status", "--porcelain")); left != "" {
		t.Errorf("guide says there is nothing to commit after repo-agent-files; git status shows:\n%s", left)
	}
	if settingsErr == nil {
		if after, err := os.ReadFile(settingsPath); err != nil || !bytes.Equal(after, settingsBefore) {
			t.Errorf("guide says the test leaves %s in place; it changed or is gone (%v)", repoSettings, err)
		}
	}
}

func removeSettingsEnv(t *testing.T, path, desc string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("guide lists %s as a leftover: %v", path, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	env, _ := doc["env"].(map[string]any)
	names := regexp.MustCompile(`"([A-Z_]+)"`).FindAllStringSubmatch(desc, -1)
	if len(names) == 0 || !strings.Contains(desc, `under "env"`) {
		t.Fatalf("leftovers line for %s must name the \"env\" entries to delete: %q", path, desc)
	}
	for _, m := range names {
		if _, ok := env[m[1]]; !ok {
			t.Fatalf("guide lists %s in %s, but it is not there", m[1], path)
		}
		delete(env, m[1])
	}
	for k := range env {
		if strings.HasPrefix(k, "DOCKET_") {
			t.Errorf("%s still has %s after the guide's deletions", path, k)
		}
	}
	if len(env) == 0 {
		delete(doc, "env")
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func removeMarkedLines(t *testing.T, path, desc string) {
	t.Helper()
	m := regexp.MustCompile(`^the lines from "([^"]+)" to "([^"]+)"$`).FindStringSubmatch(desc)
	if m == nil {
		t.Fatalf("leftovers line for %s must read `the lines from \"<open>\" to \"<close>\"`: %q", path, desc)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("guide lists %s as a leftover: %v", path, err)
	}
	lines := strings.Split(string(b), "\n")
	start, end := -1, -1
	for i, l := range lines {
		if l == m[1] && start < 0 {
			start = i
		}
		if l == m[2] && start >= 0 && end < 0 {
			end = i
		}
	}
	if start < 0 || end < start {
		t.Fatalf("%s has no lines from %q to %q:\n%s", path, m[1], m[2], b)
	}
	out := append(append([]string{}, lines[:start]...), lines[end+1:]...)
	if err := os.WriteFile(path, []byte(strings.Join(out, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
}

// linksInto counts the symlinks under dir whose target lies inside root.
func linksInto(t *testing.T, dir, root string) int {
	t.Helper()
	n := 0
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&fs.ModeSymlink == 0 {
			return nil
		}
		target, err := os.Readlink(p)
		if err != nil {
			return err
		}
		if target == root || strings.HasPrefix(target, root+"/") {
			n++
		}
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("walk %s: %v", dir, err)
	}
	return n
}

// changesDir is the case's changes directory: changes_dir from .docket.yml on main
// when set, else the default.
func changesDir(t *testing.T, c *upgradeCase) string {
	t.Helper()
	dir := "docs/changes"
	if r := c.run(t, c.Origin, "git", "show", "main:.docket.yml"); r.Code == 0 {
		var cfg struct {
			ChangesDir string `yaml:"changes_dir"`
		}
		if err := yaml.Unmarshal([]byte(r.Stdout), &cfg); err != nil {
			t.Fatalf("parse main:.docket.yml: %v", err)
		}
		if cfg.ChangesDir != "" {
			dir = strings.TrimSuffix(cfg.ChangesDir, "/")
		}
	}
	return dir
}
