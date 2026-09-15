package codexcontract

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateDocketExecutableBindsReportedCommit(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "docket")
	want := strings.Repeat("a", 40)
	write := func(commit string) {
		t.Helper()
		body := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' '{\"commit\":\"%s\"}'\n", commit)
		if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write(want)
	a := Assignment{DocketExecutable: path, DocketCommit: want}
	if err := ValidateDocketExecutable(a); err != nil {
		t.Fatalf("matching executable: %v", err)
	}

	write(strings.Repeat("b", 40))
	if err := ValidateDocketExecutable(a); err == nil {
		t.Fatal("accepted a different binary at the assigned canonical path")
	}
}

func TestValidateDocketExecutableBoundsStalledVersionProcess(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "docket")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nsleep 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	err = ValidateDocketExecutable(Assignment{DocketExecutable: path, DocketCommit: strings.Repeat("a", 40)})
	if err == nil {
		t.Fatal("stalled version process was accepted")
	}
	if elapsed := time.Since(start); elapsed >= 2*time.Second {
		t.Fatalf("stalled version process was not bounded: %s", elapsed)
	}
}

func TestValidateResourcesRequiresNestedDeclaredClosureAndDigests(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(root, "skill")
	if err := os.MkdirAll(filepath.Join(skill, "references", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{"SKILL.md": "Read [edge](references/edge.md).", "references/edge.md": "Read [deep](deep/detail.md).", "references/deep/detail.md": "complete"}
	var resources []Resource
	for rel, body := range files {
		p := filepath.Join(skill, rel)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		s := sha256.Sum256([]byte(body))
		resources = append(resources, Resource{LogicalID: "plan/" + rel, Path: p, SHA256: hex.EncodeToString(s[:]), Source: "package:docket-plan@abc"})
	}
	a := Assignment{ReadRoots: []string{root}, Resources: resources, PlanSkill: "plan/SKILL.md"}
	if err := ValidateResources(a); err != nil {
		t.Fatalf("valid closure: %v", err)
	}
	a.Resources = a.Resources[:2]
	if err := ValidateResources(a); err == nil {
		t.Fatal("accepted package with undeclared nested reference")
	}
}

func TestValidateResourcesRejectsHashDriftAndSymlinkEscape(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(root, "SKILL.md")
	if err := os.WriteFile(p, []byte("body"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := Assignment{ReadRoots: []string{root}, Resources: []Resource{{LogicalID: "skill", Path: p, SHA256: "deadbeef", Source: "package:x"}}}
	if err := ValidateResources(a); err == nil {
		t.Fatal("accepted resource hash drift")
	}
	out := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(out, []byte("body"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(out, p); err != nil {
		t.Fatal(err)
	}
	s := sha256.Sum256([]byte("body"))
	a.Resources[0].SHA256 = hex.EncodeToString(s[:])
	if err := ValidateResources(a); err == nil {
		t.Fatal("accepted symlinked resource")
	}
}

func TestValidateResourcesRejectsIncompleteDependencyGraph(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(root, "SKILL.md")
	if err := os.WriteFile(p, []byte("skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("skill\n"))
	a := Assignment{ReadRoots: []string{root}, Resources: []Resource{{LogicalID: "skill", Path: p, SHA256: hex.EncodeToString(sum[:]), Source: "package:test"}}, ResourceDependencies: map[string][]string{"skill": {"missing"}}}
	if err := ValidateResources(a); err == nil {
		t.Fatal("accepted dependency on an undeclared resource")
	}
}
