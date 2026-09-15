package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/testsupport"
)

func TestPrepareRefusesExistingDestinationBeforeCandidateActions(t *testing.T) {
	root := testsupport.TempDir(t)
	dest := filepath.Join(root, "existing")
	if err := os.Mkdir(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	err := prepare(options{Source: filepath.Join(root, "source"), Binary: filepath.Join(root, "docket"), Destination: dest, Pins: filepath.Join(root, "pins.yml")})
	if err == nil || !strings.Contains(err.Error(), "destination must not exist") {
		t.Fatalf("prepare error=%v", err)
	}
}

func TestVerifyCandidateIdentityRejectsMismatchedAssetSet(t *testing.T) {
	root := testsupport.TempDir(t)
	binary := filepath.Join(root, "docket")
	body := "#!/bin/sh\nprintf '%s\\n' '{\"commit\":\"abc123\",\"asset_set_id\":\"sha256:binary\"}'\n"
	if err := os.WriteFile(binary, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	err := verifyCandidateIdentity(binary, "abc123", "sha256:source")
	if err == nil || !strings.Contains(err.Error(), "asset set") {
		t.Fatalf("verify candidate identity error=%v", err)
	}
}

func TestWriteCatalogSkillsUsesVerifiedSnapshotAfterSourceMutation(t *testing.T) {
	root := testsupport.TempDir(t)
	const assetPath = "skills/docket-demo/SKILL.md"
	verified := []byte("verified snapshot\n")
	sourcePath := filepath.Join(root, "source", filepath.FromSlash(assetPath))
	if err := writeFile(sourcePath, verified, 0o644); err != nil {
		t.Fatal(err)
	}
	catalog := assets.NewCatalog(assets.Manifest{Entries: []assets.Entry{{Path: assetPath, Role: assets.RoleSkill, Mode: 0o644, Size: int64(len(verified)), SHA256: hash(verified)}}}, func(path string) ([]byte, error) {
		return append([]byte(nil), verified...), nil
	})
	if err := os.WriteFile(sourcePath, []byte("mutated after verification\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(root, "primary")
	files := map[string]string{}
	if err := writeCatalogSkills(catalog, primary, files); err != nil {
		t.Fatal(err)
	}
	installed := filepath.Join(primary, ".agents", filepath.FromSlash(assetPath))
	got, err := os.ReadFile(installed)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(verified) {
		t.Fatalf("installed skill=%q, want verified snapshot %q", got, verified)
	}
	if files[rel(primary, installed)] != hash(verified) {
		t.Fatalf("installed skill manifest=%q, want %q", files[rel(primary, installed)], hash(verified))
	}

	corrupt := assets.NewCatalog(catalog.Manifest, func(string) ([]byte, error) {
		return []byte("corrupt catalog bytes\n"), nil
	})
	if err := writeCatalogSkills(corrupt, filepath.Join(root, "corrupt-primary"), map[string]string{}); err == nil {
		t.Fatal("installed skill bytes that differ from the verified catalog manifest")
	}
}

func TestCompleteManifestFilesPreservesGeneratedFilesAndRejectsDrift(t *testing.T) {
	root := testsupport.TempDir(t)
	rel := ".codex/agents/docket-review.toml"
	original := []byte("model = \"test\"\n")
	if err := writeFile(filepath.Join(root, filepath.FromSlash(rel)), original, 0o644); err != nil {
		t.Fatal(err)
	}
	explicit := map[string]string{rel: hash(original)}
	got, err := completeManifestFiles(root, explicit, map[string]string{"tracked.txt": hash([]byte("tracked"))})
	if err != nil {
		t.Fatalf("complete manifest: %v", err)
	}
	if got[rel] != hash(original) || got["tracked.txt"] == "" {
		t.Fatalf("complete manifest omitted inputs: %#v", got)
	}

	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte("mutated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := completeManifestFiles(root, explicit, nil); err == nil {
		t.Fatal("complete manifest accepted a rendered agent mutated after hashing")
	}

	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), original, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := completeManifestFiles(root, explicit, map[string]string{rel: hash([]byte("different"))}); err == nil {
		t.Fatal("complete manifest accepted conflicting hashes for one path")
	}
}
