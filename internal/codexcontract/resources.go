package codexcontract

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var localMarkdownLink = regexp.MustCompile(`\[[^]]*\]\(([^)#]+)(?:#[^)]*)?\)`)

func ValidateDocketExecutable(a Assignment) error {
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
	return nil
}

func ValidateResources(a Assignment) error {
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
		b, err := os.ReadFile(r.Path)
		if err != nil {
			return fmt.Errorf("resource %q: %w", r.LogicalID, err)
		}
		sum := sha256.Sum256(b)
		if hex.EncodeToString(sum[:]) != strings.ToLower(r.SHA256) {
			return fmt.Errorf("resource %q sha256 mismatch", r.LogicalID)
		}
		byPath[r.Path] = r
	}
	for p := range byPath {
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for _, m := range localMarkdownLink.FindAllSubmatch(b, -1) {
			target := string(m[1])
			if strings.HasPrefix(target, "/") || strings.Contains(target, ":") {
				continue
			}
			resolved := filepath.Clean(filepath.Join(filepath.Dir(p), filepath.FromSlash(target)))
			if _, ok := byPath[resolved]; !ok {
				return fmt.Errorf("resource %q links to undeclared local dependency %q", byPath[p].LogicalID, target)
			}
		}
	}
	for _, id := range []string{a.PlanSkill, a.BuildSkill} {
		if id != "" && !logical[id] {
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
