package codexcontract

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateResourcesRequiresNestedDeclaredClosureAndDigests(t *testing.T) {
	root := t.TempDir()
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
	root := t.TempDir()
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
