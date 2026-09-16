package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// verifyRuntimePins checks global files too: project-local files alone do not
// prove which instructions a native child loads. Symlinks are followed because
// linked skills are a supported installation shape. No installation is changed.
func verifyRuntimePins(manifestPath, runtimeHome string) error {
	if !filepath.IsAbs(manifestPath) || !filepath.IsAbs(runtimeHome) {
		return fmt.Errorf("runtime manifest and home must be absolute")
	}
	body, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	var m struct {
		Files map[string]string `json:"files"`
	}
	if err := json.Unmarshal(body, &m); err != nil {
		return err
	}
	paths := make([]string, 0, len(m.Files))
	for p := range m.Files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	roles, skills := 0, 0
	for _, p := range paths {
		if filepath.IsAbs(p) || filepath.ToSlash(filepath.Clean(p)) != p || strings.HasPrefix(p, "../") {
			return fmt.Errorf("unsafe manifest path: %s", p)
		}
		targets := []string{}
		if strings.HasPrefix(p, ".codex/agents/docket-") && strings.HasSuffix(p, ".toml") && filepath.Dir(p) == ".codex/agents" {
			roles++
			targets = append(targets, p)
		} else if strings.HasPrefix(p, ".agents/skills/docket-") {
			if strings.HasSuffix(p, "/SKILL.md") {
				skills++
			}
			targets = append(targets, p, strings.Replace(p, ".agents/skills/", ".codex/skills/", 1))
		}
		for _, target := range targets {
			digest, err := hex.DecodeString(m.Files[p])
			if err != nil || len(digest) != 32 {
				return fmt.Errorf("invalid runtime digest: %s", p)
			}
			actual, err := os.ReadFile(filepath.Join(runtimeHome, filepath.FromSlash(target)))
			if err != nil {
				return fmt.Errorf("runtime resource unavailable %s: %w", target, err)
			}
			if hash(actual) != m.Files[p] {
				return fmt.Errorf("runtime resource drift: %s", target)
			}
		}
	}
	if roles == 0 || skills == 0 {
		return fmt.Errorf("runtime manifest must pin Docket roles and skills")
	}
	return nil
}
