package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/harness"
)

func main() {
	var repo string
	var check bool
	flag.StringVar(&repo, "repo", "", "absolute repository root")
	flag.BoolVar(&check, "check", false, "refuse when the generated block differs")
	flag.Parse()
	if !filepath.IsAbs(repo) {
		fatal("-repo must be absolute")
	}
	p := filepath.Join(repo, "AGENTS.md")
	src, err := os.ReadFile(p)
	if err != nil {
		fatal(err.Error())
	}
	doc, err := document.Parse(src)
	if err != nil {
		fatal(err.Error())
	}
	catalog, err := assets.EmbeddedCatalog()
	if err != nil {
		fatal(err.Error())
	}
	gate, err := harness.RunGate(catalog)
	if err != nil {
		fatal(err.Error())
	}
	var patches document.PatchSet
	patches.ReplaceBlock("dispatch", harness.CodexDispatchInterior(gate))
	out, err := doc.Apply(patches)
	if err != nil {
		fatal(err.Error())
	}
	if check {
		if !bytes.Equal(src, out) {
			fatal("AGENTS.md dispatch block is stale")
		}
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), "AGENTS.md.XXXXXX")
	if err != nil {
		fatal(err.Error())
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0o644); err != nil {
		fatal(err.Error())
	}
	if _, err := tmp.Write(out); err != nil {
		fatal(err.Error())
	}
	if err := tmp.Close(); err != nil {
		fatal(err.Error())
	}
	if err := os.Rename(name, p); err != nil {
		fatal(err.Error())
	}
}
func fatal(s string) { fmt.Fprintln(os.Stderr, s); os.Exit(1) }
