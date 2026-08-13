// Command genassets freezes the repo's authored asset roots into the embedded
// bundle that ships inside the docket binary: internal/assets/embedded's
// manifest.json plus its tree/ payload.
//
// It is a repo-side developer tool, not part of the docket CLI, so it reports
// through log rather than the protocol envelope — cmd/docket/main.go remains
// the CLI's single exit site.
//
//	genassets [-repo <dir>] [-check]
//
// Default mode regenerates the bundle into a temp directory beside the
// destination and moves it into place. -check regenerates in memory and
// byte-compares against the committed tree, exiting non-zero with one line per
// differing path — the shape the suite's drift gate consumes.
package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/danielhanold/docket/internal/assets"
)

// embeddedRel is the repo-relative home of the generated bundle.
const embeddedRel = "internal/assets/embedded"

func main() {
	log.SetFlags(0)
	log.SetPrefix("genassets: ")

	check := flag.Bool("check", false, "verify the committed bundle matches the authored roots; write nothing")
	repo := flag.String("repo", ".", "repo root to read the authored asset roots from")
	flag.Parse()

	repoDir, err := filepath.Abs(*repo)
	if err != nil {
		log.Fatalf("resolve -repo %q: %v", *repo, err)
	}
	outDir := filepath.Join(repoDir, filepath.FromSlash(embeddedRel))

	m, payload, err := assets.Generate(repoDir, assets.DefaultAllowedRoots())
	if err != nil {
		log.Fatalf("%v", err)
	}

	if *check {
		diffs, err := assets.DiffTree(outDir, m, payload)
		if err != nil {
			log.Fatalf("%v", err)
		}
		if len(diffs) > 0 {
			for _, d := range diffs {
				log.Printf("drift: %s", d)
			}
			log.Fatalf("%s is stale (%d differing paths) — run: go generate ./internal/assets/", embeddedRel, len(diffs))
		}
		log.Printf("%s matches the authored roots (%d entries, %s)", embeddedRel, len(m.Entries), m.AssetSetID)
		return
	}

	// Stage beside the destination so the publish is a same-filesystem rename.
	staging, err := os.MkdirTemp(filepath.Dir(outDir), ".embedded-staging-")
	if err != nil {
		log.Fatalf("create staging directory: %v", err)
	}
	defer os.RemoveAll(staging)

	staged := filepath.Join(staging, "embedded")
	if err := assets.WriteTree(staged, m, payload); err != nil {
		log.Fatalf("%v", err)
	}
	if err := os.RemoveAll(outDir); err != nil {
		log.Fatalf("remove previous %s: %v", embeddedRel, err)
	}
	if err := os.Rename(staged, outDir); err != nil {
		log.Fatalf("publish %s: %v", embeddedRel, err)
	}
	log.Printf("wrote %s (%d entries, %s)", embeddedRel, len(m.Entries), m.AssetSetID)
}
