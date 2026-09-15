package codexcontract

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var localMarkdownLink = regexp.MustCompile(`\[[^]]*\]\(([^)#]+)(?:#[^)]*)?\)`)

const docketExecutableVersionTimeout = 5 * time.Second

// ValidateDocketExecutable proves the assigned path is canonical and executable,
// then asks that exact program for its build identity. A path that is replaced
// after assignment cannot enter unless the program reports the pinned full
// source commit.
func ValidateDocketExecutable(a Assignment) error {
	return validateDocketExecutable(a, docketExecutableVersionTimeout)
}

func validateDocketExecutable(a Assignment, versionTimeout time.Duration) error {
	canon, err := filepath.EvalSymlinks(a.DocketExecutable)
	if err != nil {
		return fmt.Errorf("docket executable is unavailable: %w", err)
	}
	if canon != a.DocketExecutable {
		return fmt.Errorf("docket executable is not canonical")
	}
	info, err := os.Lstat(a.DocketExecutable)
	if err != nil {
		return fmt.Errorf("docket executable: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("docket executable is not an executable regular file")
	}
	ctx, cancel := context.WithTimeout(context.Background(), versionTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, a.DocketExecutable, "version", "--json")
	cmd.WaitDelay = 100 * time.Millisecond
	var stdout, stderr cappedBuffer
	stdout.max, stderr.max = 1<<20, 1<<20
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	if ctx.Err() != nil {
		return fmt.Errorf("docket executable version timed out")
	}
	if err != nil {
		return fmt.Errorf("docket executable version: %w", err)
	}
	if stdout.exceeded {
		return fmt.Errorf("docket executable version output exceeds 1 MiB")
	}
	out := stdout.Bytes()
	var version struct {
		Commit string `json:"commit"`
	}
	if err := json.Unmarshal(out, &version); err != nil {
		return fmt.Errorf("docket executable version output is invalid: %w", err)
	}
	if version.Commit != a.DocketCommit {
		return fmt.Errorf("docket executable commit mismatch")
	}
	return nil
}

func ValidateResources(a Assignment) error {
	if a.Role == "docket-plan-writer" && (a.PlanSkill == "" || a.BuildSkill == "" || a.ResultsTemplate == "" || a.ResultsTemplate == "auto") {
		return fmt.Errorf("planner resource selections and results template are required")
	}
	for _, root := range a.ReadRoots {
		canon, err := filepath.EvalSymlinks(root)
		if err != nil || canon != root {
			return fmt.Errorf("read root %q is unavailable or noncanonical", root)
		}
	}
	byPath := map[string]Resource{}
	logical := map[string]bool{}
	for _, r := range a.Resources {
		if r.LogicalID == "" || r.Source == "" || logical[r.LogicalID] {
			return fmt.Errorf("resource logical id %q is empty or repeated", r.LogicalID)
		}
		logical[r.LogicalID] = true
		if strings.Contains(r.Path, "://") || !filepath.IsAbs(r.Path) || filepath.Clean(r.Path) != r.Path {
			return fmt.Errorf("resource %q path is not canonical local absolute", r.LogicalID)
		}
		canon, err := filepath.EvalSymlinks(r.Path)
		if err != nil || canon != r.Path {
			return fmt.Errorf("resource %q path is unavailable or noncanonical", r.LogicalID)
		}
		if _, ok := byPath[r.Path]; ok {
			return fmt.Errorf("resource path %q is repeated", r.Path)
		}
		inside := false
		for _, root := range a.ReadRoots {
			if pathWithin(root, r.Path) {
				inside = true
				break
			}
		}
		if !inside {
			return fmt.Errorf("resource %q is outside declared read roots", r.LogicalID)
		}
		info, err := os.Lstat(r.Path)
		if err != nil {
			return fmt.Errorf("resource %q: %w", r.LogicalID, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("resource %q is not a regular file", r.LogicalID)
		}
		if info.Size() > 8<<20 {
			return fmt.Errorf("resource %q is too large", r.LogicalID)
		}
		b, err := readFileBounded(r.Path, 8<<20)
		if err != nil {
			return fmt.Errorf("resource %q: %w", r.LogicalID, err)
		}
		sum := sha256.Sum256(b)
		if hex.EncodeToString(sum[:]) != strings.ToLower(r.SHA256) {
			return fmt.Errorf("resource %q sha256 mismatch", r.LogicalID)
		}
		byPath[r.Path] = r
	}
	if a.Role == "docket-plan-writer" {
		var templatePath string
		for _, resource := range a.Resources {
			if resource.LogicalID == a.ResultsTemplate {
				templatePath = filepath.ToSlash(resource.Path)
				break
			}
		}
		if !strings.HasSuffix(templatePath, "/skills/docket-implement-next/results-template.md") {
			return fmt.Errorf("planner results template is not the packaged docket-implement-next template")
		}
	}
	for p := range byPath {
		b, err := readFileBounded(p, 8<<20)
		if err != nil {
			return err
		}
		for _, m := range localMarkdownLink.FindAllSubmatch(b, -1) {
			target := string(m[1])
			if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			resolved := filepath.Clean(filepath.FromSlash(target))
			if !filepath.IsAbs(resolved) {
				resolved = filepath.Clean(filepath.Join(filepath.Dir(p), resolved))
			}
			if _, ok := byPath[resolved]; !ok {
				return fmt.Errorf("resource %q links to undeclared local dependency %q", byPath[p].LogicalID, target)
			}
		}
	}
	for _, id := range []string{a.PlanSkill, a.BuildSkill, a.ResultsTemplate} {
		if id != "" && id != "auto" && !logical[id] {
			return fmt.Errorf("selected resource %q is not declared", id)
		}
	}
	for parent, children := range a.ResourceDependencies {
		if !logical[parent] {
			return fmt.Errorf("resource dependency parent %q is not declared", parent)
		}
		seen := map[string]bool{}
		for _, child := range children {
			if !logical[child] || seen[child] {
				return fmt.Errorf("resource dependency %q -> %q is missing or repeated", parent, child)
			}
			seen[child] = true
		}
	}
	return nil
}

type cappedBuffer struct {
	bytes    []byte
	max      int
	exceeded bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	remaining := b.max - len(b.bytes)
	if remaining > 0 {
		if remaining > len(p) {
			remaining = len(p)
		}
		b.bytes = append(b.bytes, p[:remaining]...)
	}
	if remaining < len(p) {
		b.exceeded = true
	}
	return len(p), nil
}

func (b *cappedBuffer) Bytes() []byte { return append([]byte(nil), b.bytes...) }
