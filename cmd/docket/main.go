// Command docket is the Docket executable. This is the only os.Exit site.
package main

import (
	"os"

	"github.com/danielhanold/docket/internal/buildinfo"
	"github.com/danielhanold/docket/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr,
		buildinfo.Current(), buildinfo.CurrentRuntime()))
}
